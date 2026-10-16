package library

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/tidwall/jsonc"
	"gopkg.in/yaml.v3"

	"github.com/yetone/magpie/internal/agent"
	"github.com/yetone/magpie/internal/edit"
)

func nativeRead(r NativeRoot, p string) ([]byte, bool, error) {
	if r.distro != "" {
		return nativeWSLRead(r, p)
	}
	f, err := os.Open(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, false, err
	}
	if !st.Mode().IsRegular() {
		return nil, false, errors.New("native resource is not a regular file")
	}
	b, err := io.ReadAll(io.LimitReader(f, nativeMaxBytes+1))
	if err != nil {
		return nil, false, err
	}
	if len(b) > nativeMaxBytes {
		return nil, false, errors.New("native resource exceeds 1 MiB")
	}
	return b, true, nil
}

func nativeVersion(b []byte, exists bool) string {
	if !exists {
		return "missing"
	}
	return nativeDigest(b)
}

func nativeBackup(r NativeRoot, p string, b []byte) error {
	dir, err := freshBackup()
	if err != nil {
		return err
	}
	dir = filepath.Join(dir, fileName(r.Agent))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	name := nativeDigest([]byte(p))[:16] + "-" + filepath.Base(p)
	if err := os.WriteFile(filepath.Join(dir, name), b, 0o600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "native-path.txt"), []byte(r.Environment+"\n"+p+"\n"), 0o600)
}

func nativeWrite(r NativeRoot, p string, b []byte, version string, remove bool) error {
	mu.Lock()
	defer mu.Unlock()
	l, err := load()
	if err != nil {
		return err
	}
	old, exists, err := nativeRead(r, p)
	if err != nil {
		return err
	}
	if nativeVersion(old, exists) != version {
		return ErrNativeConflict
	}
	b, changed, err := nativeRelinquish(l, r, p, old, b, remove)
	if err != nil {
		return err
	}
	if err := nativeWriteBytes(r, p, b, version, remove); err != nil {
		return err
	}
	if changed {
		if err := l.save(); err != nil {
			return fmt.Errorf("native file saved, but releasing central library ownership failed: %w", err)
		}
	}
	return nil
}

func nativeWriteBytes(r NativeRoot, p string, b []byte, version string, remove bool) error {
	old, exists, err := nativeRead(r, p)
	if err != nil {
		return err
	}
	if nativeVersion(old, exists) != version {
		return ErrNativeConflict
	}
	if exists {
		if err := nativeBackup(r, p, old); err != nil {
			return fmt.Errorf("keep native backup: %w", err)
		}
	}
	if r.distro != "" {
		return nativeWSLWrite(r, p, b, version, remove)
	}
	current, currentExists, err := nativeRead(r, p)
	if err != nil {
		return err
	}
	if nativeVersion(current, currentExists) != version {
		return ErrNativeConflict
	}
	if remove {
		if !exists {
			return fs.ErrNotExist
		}
		if err := os.Remove(p); err != nil {
			return err
		}
		if _, err := os.Lstat(p); !errors.Is(err, fs.ErrNotExist) {
			return errors.New("removed resource is still present")
		}
		return nil
	}
	if err := edit.WriteAtomic(p, b); err != nil {
		return err
	}
	got, _, err := nativeRead(r, p)
	if err != nil {
		return err
	}
	// WriteAtomic retains an existing UTF-8 BOM.
	if !bytes.Equal(edit.TrimBOM(got), edit.TrimBOM(b)) {
		return errors.New("native file changed before readback completed")
	}
	return nil
}

func nativeValidate(item NativeItem, text string, remove bool) error {
	if remove {
		return nil
	}
	b := edit.TrimBOM([]byte(text))
	var doc any
	var err error
	switch strings.ToLower(filepath.Ext(item.Path)) {
	case ".json", ".jsonc":
		if !json.Valid(jsonc.ToJSON(b)) {
			return errors.New("invalid JSON/JSONC; the native file was not changed")
		}
	case ".toml":
		err = toml.Unmarshal(b, &doc)
	case ".yaml", ".yml":
		err = yaml.Unmarshal(b, &doc)
	}
	if err != nil {
		return errors.New("invalid native configuration syntax; the file was not changed")
	}
	if item.Kind != "mcp" || !strings.HasPrefix(item.Environment, "wsl:") {
		return nil
	}
	for _, a := range agent.Detected() {
		if a.ID != item.Agent {
			continue
		}
		t := targetOf(a)
		if t == nil || t.MCP == nil {
			return errors.New("MCP capability is no longer available")
		}
		es, err := t.MCP.parseEntries(b)
		if err != nil {
			return errors.New("invalid native MCP configuration; the file was not changed")
		}
		for name, entry := range es {
			server, ok := t.MCP.decode(name, entry)
			if !ok {
				continue
			}
			if _, err := t.MCP.side(server); err != nil {
				return fmt.Errorf("MCP %s: %w", name, err)
			}
		}
		return nil
	}
	return errors.New("native MCP agent is no longer available")
}
