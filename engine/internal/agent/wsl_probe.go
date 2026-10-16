package agent

import (
	"bufio"
	"errors"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/yetone/magpie/internal/wslrun"
)

// wslPathEnvs are the path-root variables known adapters read, probed in
// the distro. Secrets, profile names and credential files are not listed.
var wslPathEnvs = []string{
	"ANTE_HOME", "ATOMCODE_HOME", "CLAUDE_CONFIG_DIR", "CLINE_DATA_DIR", "CLINE_DIR",
	"CODEBUDDY_CONFIG_DIR", "CODEX_HOME", "DEVECO_CONFIG_DIR", "DSH_HOME",
	"FACTORY_HOME_OVERRIDE", "GEMINI_CLI_HOME", "GROK_HOME", "HERMES_HOME",
	"KIMI_CODE_HOME", "KIMI_SHARE_DIR", "MIMOCODE_HOME", "MINIMAX_DATA_DIR",
	"MISTER_MORPH_CONFIG", "MOTHX_DIR", "NVM_DIR", "OMO_CODING_AGENT_DIR",
	"OPENCODE_CONFIG_DIR", "PI_CODING_AGENT_DIR", "PRIME_AGENT_CODING_AGENT_DIR",
	"QODERCN_CONFIG_DIR", "QODER_CONFIG_DIR", "QWEN_HOME", "REASONIX_HOME",
	"SENPI_CODING_AGENT_DIR", "SNOW_CONFIG_DIR", "STEP_CODING_AGENT_DIR",
	"XDG_CONFIG_HOME", "XDG_DATA_HOME",
}

// wslEnvDirs maps a probed path variable to the wslKind.dir Has key it
// relocates, so a custom root still counts as the agent being there.
var wslEnvDirs = []struct{ env, dir string }{
	{"CODEX_HOME", ".codex"},
	{"CLAUDE_CONFIG_DIR", ".claude"},
	{"OPENCODE_CONFIG_DIR", ".config/opencode"},
	{"MIMOCODE_HOME", ".config/mimocode"},
	{"HERMES_HOME", ".hermes"},
	{"GROK_HOME", ".grok"},
	{"KIMI_CODE_HOME", ".kimi-code"},
	{"QWEN_HOME", ".qwen"},
	{"DSH_HOME", ".dsh"},
	{"ANTE_HOME", ".ante"},
	{"ATOMCODE_HOME", ".atomcode"},
	{"CLINE_DIR", ".cline"},
	{"CODEBUDDY_CONFIG_DIR", ".codebuddy"},
	{"FACTORY_HOME_OVERRIDE", ".factory"},
	{"MOTHX_DIR", ".mothx"},
	{"QODER_CONFIG_DIR", ".qoder"},
	{"QODERCN_CONFIG_DIR", ".qoder-cn"},
	{"SNOW_CONFIG_DIR", ".snow"},
	{"REASONIX_HOME", ".reasonix"},
	{"DEVECO_CONFIG_DIR", ".config/deveco"},
	{"MINIMAX_DATA_DIR", ".minimax"},
	{"PI_CODING_AGENT_DIR", ".pi"},
	{"OMO_CODING_AGENT_DIR", ".omo"},
	{"PRIME_AGENT_CODING_AGENT_DIR", ".prime/agent"},
	{"STEP_CODING_AGENT_DIR", ".stepcode"},
}

var wslProbeScript = buildWSLProbeScript()

func buildWSLProbeScript() string {
	s := wslrun.LookupScript()
	s += `echo "home:$HOME"; echo "user:$(id -un)"; `
	for _, name := range wslPathEnvs {
		s += `[ -n "$` + name + `" ] && case "$` + name + `" in /*|~*) echo "env:` + name + `=$` + name + `";; esac; `
	}
	for _, e := range wslEnvDirs {
		s += `[ -n "$` + e.env + `" ] && [ -d "$` + e.env + `" ] && echo dir:` + e.dir + `; `
	}
	s += `if [ -z "$OPENCODE_CONFIG_DIR" ] && [ -n "$XDG_CONFIG_HOME" ]; then [ -d "$XDG_CONFIG_HOME/opencode" ] && echo dir:.config/opencode; fi; `
	s += `if [ -n "$XDG_CONFIG_HOME" ]; then [ -d "$XDG_CONFIG_HOME/crush" ] && echo dir:.config/crush; fi; `
	for _, k := range wslKinds {
		if k.dir != "" {
			s += `[ -d "$HOME/` + k.dir + `" ] && echo dir:` + k.dir + `; `
		}
		if k.bin != "" {
			s += `p=$(command -v ` + k.bin + ` 2>/dev/null); ` +
				`if [ -n "$p" ] && onwin "$p"; then echo "winbin:` + k.bin + ` $p"; p=; fi; ` +
				`[ -z "$p" ] && p=$(wslbin ` + k.bin + `); ` +
				`if [ -n "$p" ] && onwin "$p"; then p=; fi; ` +
				`[ -n "$p" ] && echo "bin:` + k.bin + ` $p"`
			if k.version {
				s += ` && echo "ver:` + k.id + ` $(PATH="${p%/*}:$PATH" timeout 10 "$p" --version </dev/null 2>&1 | head -n1)"`
			}
			s += `; `
		}
	}
	s += `[ -d "$HOME/` + wslCopilotDir + `" ] && echo dir:` + wslCopilotDir + `; `
	s += `awk '$1 ~ /^[A-Za-z]:/ {print "win:" $2}' /proc/mounts 2>/dev/null; `
	s += `for d in "$HOME"/.omp/profiles/*; do [ -d "$d" ] && echo "profile:omp:$(basename "$d")"; done; `
	return s + `ip route show default 2>/dev/null | head -n1 | sed 's/^/route:/'; ` +
		`grep -m1 '^nameserver' /etc/resolv.conf 2>/dev/null | sed 's/^/ns:/'; ` +
		`command -v wslinfo >/dev/null 2>&1 && echo "net:$(wslinfo --networking-mode 2>/dev/null)"; true`
}

func wslProbe(name string) (*distro, error) {
	b, err := wslRun(30*time.Second, "-d", name, "-e", "sh", "-lc", wslProbeScript)
	if err != nil {
		return nil, err
	}
	d := parseProbe(name, string(b))
	if d == nil {
		return nil, errors.New("probe printed no home")
	}
	if wslKept(*d) {
		d.Root = wslRoot(name)
	}
	return d, nil
}

// parseProbe reads wslProbeScript's output. A command on one of Windows'
// drives — WSL puts Windows' PATH on its own, so Windows npm's pi is
// /mnt/c/Users/me/AppData/Roaming/npm/pi there — is recorded as WinBins,
// not the distro's native binary.
func parseProbe(name, out string) *distro {
	d := &distro{Name: name, Has: map[string]bool{}, Probe: wslProbeVersion}
	var ns string
	bins := map[string]string{}
	var win []string
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		switch k, v, _ := strings.Cut(l, ":"); k {
		case "home":
			d.Home = strings.TrimRight(v, "/")
		case "user":
			d.User = strings.TrimSpace(v)
		case "dir":
			d.Has[l] = true
		case "ver":
			id, said, _ := strings.Cut(v, " ")
			if ver := parseVersion(said); ver != "" {
				if d.Versions == nil {
					d.Versions = map[string]string{}
				}
				d.Versions[id] = ver
			}
		case "bin":
			bin, path, _ := strings.Cut(v, " ")
			bins[bin] = strings.TrimSpace(path)
		case "winbin":
			bin, path, _ := strings.Cut(v, " ")
			if d.WinBins == nil {
				d.WinBins = map[string]string{}
			}
			d.WinBins[bin] = strings.TrimSpace(path)
		case "env":
			key, val, ok := strings.Cut(v, "=")
			if !ok || key == "" {
				continue
			}
			if d.Env == nil {
				d.Env = map[string]string{}
			}
			d.Env[key] = val
		case "win":
			if v = strings.TrimRight(v, "/"); strings.HasPrefix(v, "/") {
				win = append(win, v+"/")
			}
		case "route":
			if f := strings.Fields(v); len(f) >= 3 && f[1] == "via" && net.ParseIP(f[2]) != nil {
				d.Gateway = f[2]
			}
		case "ns":
			if f := strings.Fields(v); len(f) >= 2 && net.ParseIP(f[1]) != nil {
				ns = f[1]
			}
		case "net":
			if m := strings.ToLower(strings.TrimSpace(v)); m != "" && !strings.ContainsAny(m, " \t") {
				d.Net = m
			}
		case "profile":
			kind, pname, _ := strings.Cut(v, ":")
			if kind != "omp" || !ompProfileOK(pname) {
				continue
			}
			if !slices.Contains(d.OmpProfiles, pname) {
				d.OmpProfiles = append(d.OmpProfiles, pname)
			}
		}
	}
	if !strings.HasPrefix(d.Home, "/") {
		return nil
	}
	if d.User == "" {
		if u, ok := strings.CutPrefix(d.Home, "/home/"); ok && u != "" && !strings.Contains(u, "/") {
			d.User = u
		}
	}
	d.Bins = map[string]string{}
	for bin, path := range bins {
		if path != "" && wslrun.OnWindowsDrive(path, win) {
			if d.WinBins == nil {
				d.WinBins = map[string]string{}
			}
			d.WinBins[bin] = path
			continue
		}
		d.Has["bin:"+bin] = true
		if path != "" {
			d.Bins[bin] = path
		}
	}
	if len(d.Bins) == 0 {
		d.Bins = nil
	}
	if d.Gateway == "" {
		d.Gateway = ns
	}
	if wslrun.HostLoopback(d.Net) {
		d.Gateway = ""
	}
	return d
}

func onWindowsDrive(path string, win []string) bool {
	return wslrun.OnWindowsDrive(path, win)
}
