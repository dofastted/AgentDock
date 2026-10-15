package environments

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

// sandboxConfig points .wslconfig at a temp folder and names its copies by
// a clock that moves a second per copy.
func sandboxConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, ".wslconfig")
	oldPath, oldNow := configPath, now
	clock := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	now = func() time.Time { clock = clock.Add(time.Second); return clock }
	configPath = func() (string, error) { return p, nil }
	t.Cleanup(func() { configPath, now = oldPath, oldNow })
	return p
}

func utf16File(s string) []byte {
	u := utf16.Encode([]rune(s))
	out := []byte{0xff, 0xfe}
	for _, c := range u {
		out = append(out, byte(c), byte(c>>8))
	}
	return out
}

// SetMirrored leaves WSL's file as the user wrote it but for the one
// setting: other sections, keys and comments stay, its encoding and line
// endings are kept, localhostForwarding=false under [wsl2] goes (it turns
// mirrored's loopback relay off), and the file as it was is kept beside it.
func TestSetMirrored(t *testing.T) {
	cases := []struct {
		name, in, want string
		utf16          bool
	}{
		{
			name: "nat to mirrored, other keys kept",
			in:   "# mine\r\n[wsl2]\r\nmemory=8GB\r\nnetworkingMode=NAT\r\nlocalhostForwarding=false\r\n\r\n[experimental]\r\nautoMemoryReclaim=gradual\r\n",
			want: "# mine\r\n[wsl2]\r\nmemory=8GB\r\nnetworkingMode=mirrored\r\n\r\n[experimental]\r\nautoMemoryReclaim=gradual\r\n",
		},
		{
			name: "no wsl2 section",
			in:   "[experimental]\nsparseVhd=true\n",
			want: "[experimental]\nsparseVhd=true\n\n[wsl2]\nnetworkingMode=mirrored\n",
		},
		{
			name: "wsl2 without the key, before another section",
			in:   "[WSL2]\nprocessors=4\n\n[experimental]\nx=1\n",
			want: "[WSL2]\nprocessors=4\nnetworkingMode=mirrored\n\n[experimental]\nx=1\n",
		},
		{
			name: "a later networkingMode would win: only one is left",
			in:   "[wsl2]\nnetworkingMode=nat\nnetworkingmode = nat # again\n",
			want: "[wsl2]\nnetworkingMode=mirrored\n",
		},
		{
			name: "localhostForwarding true stays",
			in:   "[wsl2]\nlocalhostForwarding=true\n",
			want: "[wsl2]\nlocalhostForwarding=true\nnetworkingMode=mirrored\n",
		},
		{
			name:  "UTF-16LE with BOM, as Notepad's Unicode",
			in:    "[wsl2]\r\nnetworkingMode=nat\r\n",
			want:  "[wsl2]\r\nnetworkingMode=mirrored\r\n",
			utf16: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := sandboxConfig(t)
			in := []byte(c.in)
			if c.utf16 {
				in = utf16File(c.in)
			}
			if err := os.WriteFile(p, in, 0o644); err != nil {
				t.Fatal(err)
			}
			backup, err := SetMirrored()
			if err != nil {
				t.Fatal(err)
			}
			got, _ := os.ReadFile(p)
			want := []byte(c.want)
			if c.utf16 {
				want = utf16File(c.want)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("wrote %q, want %q", got, want)
			}
			if kept, _ := os.ReadFile(backup); !bytes.Equal(kept, in) {
				t.Fatalf("copy %s holds %q, want the file as it was", backup, kept)
			}
			cfg, err := ReadWSLConfig()
			if err != nil || !cfg.Mirrored || len(cfg.Backups) != 1 {
				t.Fatalf("read back %+v %v", cfg, err)
			}
		})
	}
}

// A file that asks for mirrored already is neither written nor copied; a
// missing one is made with just the setting.
func TestSetMirroredNothingToDo(t *testing.T) {
	p := sandboxConfig(t)
	if b, err := SetMirrored(); err != nil || b != "" {
		t.Fatalf("no file: copy %q, %v", b, err)
	}
	if got, _ := os.ReadFile(p); string(got) != "[wsl2]\r\nnetworkingMode=mirrored\r\n" {
		t.Fatalf("made %q", got)
	}
	before, _ := os.Stat(p)
	if b, err := SetMirrored(); err != nil || b != "" {
		t.Fatalf("already mirrored: copy %q, %v", b, err)
	}
	if after, _ := os.Stat(p); !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("a file already mirrored was written")
	}
	if cfg, _ := ReadWSLConfig(); len(cfg.Backups) != 0 {
		t.Fatalf("copies %v", cfg.Backups)
	}
}

// RestoreLatest brings back the file as it was before the newest change,
// and keeps the one it replaces, so the restore can be undone too.
func TestRestoreLatest(t *testing.T) {
	p := sandboxConfig(t)
	if _, err := RestoreLatest(); err == nil {
		t.Fatal("restored with no copy")
	}
	orig := "[wsl2]\nnetworkingMode=nat\n"
	os.WriteFile(p, []byte(orig), 0o644)
	if _, err := SetMirrored(); err != nil {
		t.Fatal(err)
	}
	if _, err := RestoreLatest(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(p); string(got) != orig {
		t.Fatalf("restored %q", got)
	}
	cfg, _ := ReadWSLConfig()
	if len(cfg.Backups) != 2 {
		t.Fatalf("copies %v", cfg.Backups)
	}
	if kept, _ := os.ReadFile(cfg.Backups[0]); !strings.Contains(string(kept), "mirrored") {
		t.Fatalf("the mirrored file replaced isn't kept: %q", kept)
	}
}

// Off the builds mirrored networking runs on, nothing is written.
func TestSetMirroredOldWindows(t *testing.T) {
	p := sandboxConfig(t)
	old := windowsBuild
	windowsBuild = func() uint32 { return 19045 }
	t.Cleanup(func() { windowsBuild = old })
	os.WriteFile(p, []byte("[wsl2]\n"), 0o644)
	if _, err := SetMirrored(); err == nil {
		t.Fatal("written on Windows 10")
	}
	if got, _ := os.ReadFile(p); string(got) != "[wsl2]\n" {
		t.Fatalf("changed to %q", got)
	}
}

// wsl.exe's list of running distros is read in either of its encodings,
// Docker Desktop's distros kept: wsl --shutdown stops them too.
func TestDecodeList(t *testing.T) {
	for _, b := range [][]byte{[]byte("Debian\r\ndocker-desktop\r\n"), utf16File("Debian\r\ndocker-desktop\r\n")} {
		got := strings.Fields(decodeList(b))
		if len(got) != 2 || got[0] != "Debian" || got[1] != "docker-desktop" {
			t.Fatalf("%q", got)
		}
	}
}
