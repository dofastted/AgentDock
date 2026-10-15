package agent

import (
	"github.com/yetone/magpie/internal/gateway"
	"github.com/yetone/magpie/internal/settings"
)

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
	// Net is WSL's networking mode as the distro said at its last probe;
	// "" when it couldn't say (.wslconfig decides Mirrored then).
	Net      string `json:"net,omitempty"`
	Mirrored bool   `json:"mirrored"`
	// Gateway is magpie's gateway as the distro reaches it.
	Gateway string `json:"gateway,omitempty"`
	// Agents are the ids of the distro's agents (<kind>@wsl:<distro>);
	// none while it is turned off.
	Agents []string `json:"agents"`
	// Home is the distro user's $HOME as magpie opens it
	// (\\wsl.localhost\Debian\home\me), while it runs and is on; "" else,
	// as opening it would start it.
	Home string `json:"-"`
}

// WSLEnvs are the installed WSL distros, in wsl.exe's order, turned off
// ones included. Off Windows, or with Detect agents in WSL off (which asks
// wsl.exe nothing), looked says the list is only what was known before.
// Like wslDistros, a running distro on is probed once and a stopped one is
// never started.
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
		}
		if _, on := live[n]; on && e.Enabled {
			for _, a := range wslAgentsOf([]distro{d}) {
				e.Agents = append(e.Agents, a.ID)
			}
			if e.Running && d.Root != "" && d.Home != "" {
				e.Home = d.local(d.Home)
			}
		}
		if e.Gateway == "" {
			e.Gateway = gateway.URL()
		}
		envs = append(envs, e)
	}
	return envs, looked
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
