package environments

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/yetone/magpie/internal/edit"
	"github.com/yetone/magpie/internal/wslrun"
)

// MirroredMinBuild is the first Windows build WSL's mirrored networking
// runs on (Windows 11 22H2).
const MirroredMinBuild = 22621

// WSLConfig is %UserProfile%\.wslconfig as the Environments page shows it.
type WSLConfig struct {
	Path                string `json:"path"`
	Exists              bool   `json:"exists"`
	NetworkingMode      string `json:"networkingMode,omitempty"`
	LocalhostForwardOff bool   `json:"localhostForwardingOff,omitempty"`
	// Mirrored is whether the file already asks for mirrored networking
	// with nothing that turns its loopback relay off.
	Mirrored bool `json:"mirrored"`
	// Backups are earlier copies AgentDock kept, newest first.
	Backups []string `json:"backups"`
	// Build is Windows' build number, 0 when unknown; Supported is whether
	// mirrored networking runs on it.
	Build     uint32 `json:"build"`
	Supported bool   `json:"supported"`
}

const backupPrefix = ".wslconfig.agentdock-"

// configPath is %UserProfile%\.wslconfig; a var for tests.
var configPath = func() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".wslconfig"), nil
}

// textFile is a text file's contents and how to write them back the same
// way: its encoding and line ending.
type textFile struct {
	text  string
	enc   string // "utf8", "utf8bom", "utf16le"
	crlf  bool
	found bool
}

func readText(path string) (textFile, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return textFile{enc: "utf8", crlf: true}, nil
	}
	if err != nil {
		return textFile{}, err
	}
	f := textFile{found: true, enc: "utf8"}
	switch {
	case len(b) >= 2 && b[0] == 0xff && b[1] == 0xfe:
		f.enc, f.text = "utf16le", decode16(b[2:])
	case len(b) >= 2 && b[0] == 0xfe && b[1] == 0xff:
		return textFile{}, errors.New(".wslconfig is UTF-16 big-endian, which AgentDock doesn't write")
	case bytes.HasPrefix(b, []byte{0xef, 0xbb, 0xbf}):
		f.enc, f.text = "utf8bom", string(b[3:])
	case len(b) >= 2 && b[1] == 0 && b[0] != 0:
		f.enc, f.text = "utf16le", decode16(b)
	default:
		f.text = string(b)
	}
	f.crlf = strings.Contains(f.text, "\r\n")
	return f, nil
}

func decode16(b []byte) string {
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
	}
	return string(utf16.Decode(u))
}

func (f textFile) bytes(text string) []byte {
	switch f.enc {
	case "utf16le":
		u := utf16.Encode([]rune(text))
		out := make([]byte, 2+2*len(u))
		out[0], out[1] = 0xff, 0xfe
		for i, c := range u {
			out[2+2*i], out[3+2*i] = byte(c), byte(c>>8)
		}
		return out
	case "utf8bom":
		return append([]byte{0xef, 0xbb, 0xbf}, text...)
	}
	return []byte(text)
}

// iniLine is one line split as WSL reads it: a section header, or a
// key=value under the current section.
func iniKey(line string) (key, val string, ok bool) {
	t := strings.TrimSpace(line)
	if t == "" || t[0] == '#' || t[0] == ';' || t[0] == '[' {
		return "", "", false
	}
	k, v, ok := strings.Cut(t, "=")
	if !ok {
		return "", "", false
	}
	v = strings.TrimSpace(v)
	if i := strings.IndexAny(v, "#;"); i >= 0 {
		v = strings.TrimSpace(v[:i])
	}
	return strings.TrimSpace(k), strings.Trim(v, `"`), true
}

func iniSection(line string) (string, bool) {
	t := strings.TrimSpace(line)
	if len(t) < 2 || t[0] != '[' || t[len(t)-1] != ']' {
		return "", false
	}
	return strings.ToLower(strings.TrimSpace(t[1 : len(t)-1])), true
}

func isFalse(v string) bool { return strings.EqualFold(v, "false") || v == "0" }

// settingsOf reads networkingMode and localhostForwarding under [wsl2],
// the last of each set winning, as WSL reads them.
func settingsOf(text string) (mode string, lfOff bool) {
	section := ""
	for _, l := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if s, ok := iniSection(l); ok {
			section = s
			continue
		}
		k, v, ok := iniKey(l)
		if !ok || section != "wsl2" {
			continue
		}
		switch strings.ToLower(k) {
		case "networkingmode":
			mode = v
		case "localhostforwarding":
			lfOff = isFalse(v)
		}
	}
	return mode, lfOff
}

// withMirrored is text with networkingMode=mirrored under [wsl2] and every
// localhostForwarding=false there taken out; every other line, comment and
// section stays as it was.
func withMirrored(text, nl string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var out []string
	section, sawWSL2, set := "", false, false
	flush := func() {
		if section == "wsl2" && !set {
			// after the section's last setting, before blank lines that end it
			at := len(out)
			for at > 0 && strings.TrimSpace(out[at-1]) == "" {
				at--
			}
			out = append(out[:at], append([]string{"networkingMode=mirrored"}, out[at:]...)...)
			set = true
		}
	}
	for _, l := range lines {
		if s, ok := iniSection(l); ok {
			flush()
			section = s
			sawWSL2 = sawWSL2 || s == "wsl2"
			out = append(out, l)
			continue
		}
		if section == "wsl2" {
			if k, v, ok := iniKey(l); ok {
				switch strings.ToLower(k) {
				case "networkingmode":
					if set {
						continue // a later one would win over ours
					}
					out = append(out, "networkingMode=mirrored")
					set = true
					continue
				case "localhostforwarding":
					if isFalse(v) {
						continue
					}
				}
			}
		}
		out = append(out, l)
	}
	flush()
	if !sawWSL2 {
		for len(out) > 0 && out[len(out)-1] == "" {
			out = out[:len(out)-1]
		}
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, "[wsl2]", "networkingMode=mirrored")
	}
	s := strings.Join(out, "\n")
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return strings.ReplaceAll(s, "\n", nl)
}

// ReadWSLConfig is .wslconfig as it is now.
func ReadWSLConfig() (WSLConfig, error) {
	p, err := configPath()
	if err != nil {
		return WSLConfig{}, err
	}
	c := WSLConfig{Path: p, Backups: backups(p), Build: windowsBuild()}
	c.Supported = c.Build >= MirroredMinBuild
	f, err := readText(p)
	if err != nil {
		return c, err
	}
	c.Exists = f.found
	c.NetworkingMode, c.LocalhostForwardOff = settingsOf(f.text)
	c.Mirrored = strings.EqualFold(c.NetworkingMode, "mirrored") && !c.LocalhostForwardOff
	return c, nil
}

// SetMirrored writes networkingMode=mirrored under [wsl2] in .wslconfig,
// first keeping a copy of the file beside it, in the file's own encoding
// and line endings. It writes nothing when the file says so already, and
// returns the copy's path ("" when there was no file or nothing changed).
// WSL reads it only after `wsl --shutdown`.
func SetMirrored() (backup string, err error) {
	if b := windowsBuild(); b != 0 && b < MirroredMinBuild {
		return "", errors.New("mirrored networking needs Windows 11 22H2 (build 22621) or later")
	}
	p, err := configPath()
	if err != nil {
		return "", err
	}
	f, err := readText(p)
	if err != nil {
		return "", err
	}
	if mode, lfOff := settingsOf(f.text); f.found && strings.EqualFold(mode, "mirrored") && !lfOff {
		return "", nil
	}
	nl := "\n"
	if f.crlf {
		nl = "\r\n"
	}
	next := withMirrored(f.text, nl)
	if f.found {
		if backup, err = keepCopy(p); err != nil {
			return "", err
		}
	}
	return backup, edit.WriteAtomic(p, f.bytes(next))
}

// RestoreLatest puts the newest copy SetMirrored kept back as .wslconfig,
// the file it replaces kept as a copy first. It returns the copy restored.
func RestoreLatest() (string, error) {
	p, err := configPath()
	if err != nil {
		return "", err
	}
	bs := backups(p)
	if len(bs) == 0 {
		return "", errors.New("no copy of .wslconfig kept by AgentDock")
	}
	b, err := os.ReadFile(bs[0])
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(p); err == nil {
		if _, err := keepCopy(p); err != nil {
			return "", err
		}
	}
	return bs[0], edit.WriteAtomic(p, b)
}

// now is the clock copies are named by; a var for tests.
var now = time.Now

func keepCopy(p string) (string, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	name := filepath.Join(filepath.Dir(p), backupPrefix+now().Format("20060102-150405.000")+".bak")
	if err := os.WriteFile(name, b, 0o600); err != nil {
		return "", err
	}
	return name, nil
}

// backups are the copies kept beside p, newest first.
func backups(p string) []string {
	m, _ := filepath.Glob(filepath.Join(filepath.Dir(p), backupPrefix+"*.bak"))
	sort.Sort(sort.Reverse(sort.StringSlice(m)))
	if m == nil {
		m = []string{}
	}
	return m
}

// RunningDistros are the distros running now, Docker Desktop's included:
// `wsl --shutdown` stops every one of them.
func RunningDistros() ([]string, error) {
	b, err := wslrun.Run(10*time.Second, "-l", "--running", "-q")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, l := range strings.Split(decodeList(b), "\n") {
		if l = strings.TrimSpace(strings.Trim(l, "\x00\ufeff\r")); l != "" {
			out = append(out, l)
		}
	}
	return out, nil
}

// decodeList is wsl.exe's list: UTF-8 under WSL_UTF8, else UTF-16LE.
func decodeList(b []byte) string {
	if len(b) >= 2 && (b[0] == 0xff && b[1] == 0xfe || b[1] == 0 && b[0] != 0) {
		if b[0] == 0xff && b[1] == 0xfe {
			b = b[2:]
		}
		return decode16(b)
	}
	return string(b)
}

// Shutdown stops WSL: every distro and WSL's VM (`wsl --shutdown`), so the
// next start reads .wslconfig.
func Shutdown() error {
	_, err := wslrun.Run(time.Minute, "--shutdown")
	return err
}
