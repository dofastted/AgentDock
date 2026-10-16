package agent

import (
	"path/filepath"

	"github.com/yetone/magpie/internal/gateway"
	"github.com/yetone/magpie/internal/settings"
)

// WSLAgentInfo is one CLI the probe found in a distro, for the Environments
// page. Agents (the ids magpie manages) stay native-or-config-only; an
// interop-only launcher is diagnostic and is not a magpie agent.
type WSLAgentInfo struct {
	ID          string `json:"id"`
	Kind        string `json:"kind,omitempty"`
	Provenance  string `json:"provenance"`
	Path        string `json:"path,omitempty"`        // native Linux executable
	InteropPath string `json:"interopPath,omitempty"` // Windows-mounted launcher
	ConfigDir   string `json:"configDir,omitempty"`   // UNC while the distro runs
	ConfigPath  string `json:"configPath,omitempty"`  // UNC config file while it runs
}

// WSLEnv is one installed WSL distro as the Environments page shows it.
type WSLEnv struct {
	Distro string `json:"distro"`
	// Enabled is false when the distro is turned off (WSLDistrosOff) or
	// Detect agents in WSL is off: magpie leaves it alone.
	Enabled bool `json:"enabled"`
	Running bool `json:"running"`
	// Known is whether the distro was probed before (wsl.json): a stopped
	// or turned-off one shows what was found then.
	Known bool `json:"known"`
	// User is the distro's default user as last probed (id -un).
	User string `json:"user,omitempty"`
	// HomeLinux is $HOME inside the distro (/home/me).
	HomeLinux string `json:"homeLinux,omitempty"`
	// Net is WSL's networking mode as the distro said at its last probe;
	// "" when it couldn't say (.wslconfig decides Mirrored then).
	Net      string `json:"net,omitempty"`
	Mirrored bool   `json:"mirrored"`
	// Gateway is magpie's gateway as the distro reaches it.
	Gateway string `json:"gateway,omitempty"`
	// Agents are the ids of the distro's agents (<kind>@wsl:<distro>);
	// none while it is turned off. Interop-only launchers are omitted.
	Agents []string `json:"agents"`
	// AgentDetails are native, config-only and interop-only CLIs the probe
	// found, including ones that are not magpie agents.
	AgentDetails []WSLAgentInfo `json:"agentDetails,omitempty"`
	// Home is the distro user's $HOME as magpie opens it
	// (\\wsl.localhost\Debian\home\me), while it runs and is on; "" else,
	// as opening it would start it.
	Home string `json:"home,omitempty"`
	// ProbeError is why the last probe of this running distro failed.
	// Last success stays in the other fields (Stale).
	ProbeError string `json:"probeError,omitempty"`
	// Stale is whether this row is the last successful snapshot because
	// listing or probing failed.
	Stale bool `json:"stale,omitempty"`
}

// WSLEnvs are the installed WSL distros, in wsl.exe's order, turned off
// ones included. Off Windows, or with Detect agents in WSL off (which asks
// wsl.exe nothing), looked says the list is only what was known before.
// Like wslDistros, a running distro on is probed once and a stopped one is
// never started. A failed list or probe keeps the last success and sets
// Stale/ProbeError rather than looking empty.
func WSLEnvs() (envs []WSLEnv, looked bool) {
	if !wslOn {
		return nil, false
	}
	set := settings.Load()
	looked = !set.NoWSLAgents
	live := map[string]distro{}
	if looked {
		for _, d := range wslDistros() {
			live[d.Name] = d
		}
	}
	cfg := wslConfig()
	wsl.Lock()
	names := append([]string(nil), wsl.names...)
	running := map[string]bool{}
	for n, on := range wsl.running {
		running[n] = on
	}
	seen := map[string]distro{}
	for n, d := range wsl.seen {
		if d != nil {
			seen[n] = *d
		}
	}
	failErr := map[string]string{}
	for n, e := range wsl.failErr {
		failErr[n] = e
	}
	listErr := wsl.listErr
	wsl.Unlock()

	for _, n := range names {
		e := WSLEnv{Distro: n, Enabled: looked && !set.WSLDistroOff(n), Running: running[n], Agents: []string{}}
		d, ok := live[n]
		if !ok {
			d, ok = seen[n]
			if ok {
				d.Running = running[n]
				d.mirror(cfg)
			}
		}
		if ok {
			e.Known, e.Net, e.Mirrored, e.Gateway = true, d.Net, d.Mirrored, d.base()
			e.User, e.HomeLinux = d.User, d.Home
			e.ProbeError = d.ProbeErr
			if e.ProbeError == "" {
				e.ProbeError = failErr[n]
			}
			e.Stale = e.ProbeError != "" || listErr != ""
			if e.Enabled {
				for _, a := range wslAgentsOf([]distro{d}) {
					e.Agents = append(e.Agents, a.ID)
				}
				e.AgentDetails = wslAgentDetails(d)
				if e.Running && d.Root != "" && d.Home != "" {
					e.Home = d.local(d.Home)
				}
			}
		} else if failErr[n] != "" {
			e.ProbeError = failErr[n]
			e.Stale = false
		}
		if e.Gateway == "" {
			e.Gateway = gateway.URL()
		}
		envs = append(envs, e)
	}
	return envs, looked
}

func wslAgentDetails(d distro) []WSLAgentInfo {
	var out []WSLAgentInfo
	for _, k := range wslKinds {
		p := k.provenance(d)
		if p == ProvenanceMissing {
			continue
		}
		info := WSLAgentInfo{
			ID:          k.id + "@wsl:" + d.Name,
			Kind:        k.id,
			Provenance:  p,
			Path:        d.Bins[k.bin],
			InteropPath: d.WinBins[k.bin],
		}
		if d.Running && d.Root != "" && k.found(d) {
			a := k.in(d.place(info.ID))
			info.ConfigDir, info.ConfigPath = a.Dir, a.Path
		}
		out = append(out, info)
	}
	for _, name := range d.OmpProfiles {
		info := WSLAgentInfo{
			ID:         "omp#" + name + "@wsl:" + d.Name,
			Kind:       "omp",
			Provenance: ProvenanceNative,
			Path:       d.Bins["omp"],
		}
		if info.Path == "" {
			info.Provenance = ProvenanceConfig
		}
		if d.Running && d.Root != "" && d.Home != "" {
			info.ConfigDir = filepath.Join(d.local(d.Home), ".omp", "profiles", name, "agent")
		}
		out = append(out, info)
	}
	return out
}

// WSLRefresh invalidates nothing itself: Rescan already forgot the caches.
// It waits for a new probe of every running distro that is on.
func WSLRefresh() {
	if !wslLooks() {
		return
	}
	_ = wslDistros()
}

// HostAgents are the ids of this machine's own agents found installed or
// configured (not a WSL distro's, nor a plugin's client).
func HostAgents() []string {
	out := []string{}
	for _, a := range All() {
		if a.WSL == "" && a.Detected() {
			out = append(out, a.ID)
		}
	}
	return out
}
