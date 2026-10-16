package library

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/yetone/magpie/internal/agent"
	"github.com/yetone/magpie/internal/proc"
	"github.com/yetone/magpie/internal/settings"
)

// Linux resolves its own symlinks; Windows UNC cannot reliably follow absolute
// Linux symlinks. File contents travel on stdin/stdout, never in command arguments.
const nativeWSLListScript = `set -eu
p=$1
if [ ! -e "$p" ]; then
  if [ -L "$p" ]; then printf 'unreadable native symlink\n' >&2; exit 1; fi
  exit 0
fi
if [ "$2" = file ]; then
  [ -f "$p" ] || { printf 'native path is not a regular file\n' >&2; exit 1; }
  printf '%s\000' "${p##*/}"
  exit 0
fi
[ -d "$p" ] || { printf 'native location is not a directory\n' >&2; exit 1; }
cd "$p"
find -L . -maxdepth 9 \( -name '.*' ! -name . -o -name node_modules -o -name blobs -o -name sessions -o -name cache -o -name backups -o -name __pycache__ -o -name vendor -o -name dist \) -prune -o -type f -size -1025k -print0
`

const nativeWSLReadScript = `set -eu
p=$1
if [ ! -e "$p" ]; then
  if [ -L "$p" ]; then printf 'unreadable native symlink\n' >&2; exit 1; fi
  printf 'missing\n'; exit 0
fi
[ -f "$p" ] || { printf 'native resource is not a regular file\n' >&2; exit 1; }
n=$(wc -c < "$p")
[ "$n" -le 1048576 ] || { printf 'native resource exceeds 1 MiB\n' >&2; exit 1; }
printf 'exists\n'
cat -- "$p"
`

const nativeWSLWriteScript = `set -eu
original=$1
p=$(readlink -m -- "$original")
IFS= read -r expected
version() {
  if [ -e "$p" ]; then
    [ -f "$p" ] || { printf 'native resource is not a regular file\n' >&2; exit 1; }
    sum=$(sha256sum -- "$p"); printf '%s' "${sum%% *}"
  else
    printf missing
  fi
}
if [ "$(version)" != "$expected" ]; then printf 'conflict\n'; exit 0; fi
if [ "$2" = remove ]; then
  rm -- "$original"
  printf 'ok\n'; exit 0
fi
dir=${p%/*}
mkdir -p -- "$dir"
tmp=$(mktemp "$dir/.agentdock-native.XXXXXX")
trap 'rm -f -- "$tmp"' EXIT HUP INT TERM
if [ -f "$p" ]; then cp -p -- "$p" "$tmp"; fi
cat > "$tmp"
if [ "$(version)" != "$expected" ]; then printf 'conflict\n'; exit 0; fi
mv -f -- "$tmp" "$p"
printf 'ok\n'
`

type nativeOutput struct{ bytes.Buffer }

func (b *nativeOutput) Write(p []byte) (int, error) {
	if len(p)+b.Len() > 2*nativeMaxBytes {
		return 0, errors.New("native WSL output exceeds limit")
	}
	return b.Buffer.Write(p)
}

var nativeWSLRun = func(r NativeRoot, script string, input io.Reader, args ...string) ([]byte, error) {
	if settings.Load().NoWSLAgents || settings.Load().WSLDistroOff(r.distro) || !agent.WSLRunning(r.distro) {
		return nil, errors.New("distribution is stopped or disabled; no file was opened")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	argv := []string{"-d", r.distro, "--exec", "sh", "-c", script, "sh"}
	cmd := proc.CommandContext(ctx, "wsl.exe", append(argv, args...)...)
	cmd.Env = append(os.Environ(), "WSL_UTF8=1")
	cmd.Stdin = input
	var out, stderr nativeOutput
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, errors.New("native WSL file operation timed out")
		}
		detail := strings.TrimSpace(stderr.String())
		if len(detail) > 512 {
			detail = detail[:512]
		}
		if detail == "" {
			detail = "native WSL file operation failed"
		}
		return out.Bytes(), fmt.Errorf("WSL %s: %s", r.distro, detail)
	}
	return out.Bytes(), nil
}

func nativeLinuxPath(p string) (string, error) {
	if linux := linuxHome(p); linux != "" {
		return linux, nil
	}
	if strings.HasPrefix(p, "/") {
		return p, nil
	}
	return "", errors.New("native WSL location is not a distro path")
}

func nativeWSLList(r NativeRoot) ([]string, error) {
	p, err := nativeLinuxPath(r.Path)
	if err != nil {
		return nil, err
	}
	kind := "directory"
	if !r.directory {
		kind = "file"
	}
	out, err := nativeWSLRun(r, nativeWSLListScript, nil, p, kind)
	var names []string
	for _, name := range bytes.Split(out, []byte{0}) {
		if len(name) == 0 {
			continue
		}
		n := strings.TrimPrefix(string(name), "./")
		if nativeTextFile(n) {
			names = append(names, n)
		}
		if len(names) == nativeMaxItems {
			return names, errors.New("native WSL location has too many files")
		}
	}
	return names, err
}

func nativeWSLRead(r NativeRoot, p string) ([]byte, bool, error) {
	linux, err := nativeLinuxPath(p)
	if err != nil {
		return nil, false, err
	}
	out, err := nativeWSLRun(r, nativeWSLReadScript, nil, linux)
	if err != nil {
		return nil, false, err
	}
	state, b, ok := bytes.Cut(out, []byte{'\n'})
	if !ok {
		return nil, false, errors.New("invalid native WSL file response")
	}
	if string(state) == "missing" {
		return nil, false, nil
	}
	if string(state) != "exists" || len(b) > nativeMaxBytes {
		return nil, false, errors.New("invalid or oversized native WSL file response")
	}
	return b, true, nil
}

func nativeWSLWrite(r NativeRoot, p string, b []byte, version string, remove bool) error {
	linux, err := nativeLinuxPath(p)
	if err != nil {
		return err
	}
	action := "write"
	if remove {
		action = "remove"
	}
	input := io.MultiReader(strings.NewReader(version+"\n"), bytes.NewReader(b))
	out, err := nativeWSLRun(r, nativeWSLWriteScript, input, linux, action)
	if err != nil {
		return err
	}
	if string(out) == "conflict\n" {
		return ErrNativeConflict
	}
	if string(out) != "ok\n" {
		return errors.New("native WSL write did not report success")
	}
	got, exists, err := nativeWSLRead(r, p)
	if err != nil {
		return err
	}
	if remove {
		if exists {
			return errors.New("removed native WSL resource is still present")
		}
		return nil
	}
	if !exists || !bytes.Equal(got, b) {
		return errors.New("native WSL file changed before readback completed")
	}
	return nil
}
