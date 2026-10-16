package gui

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/yetone/magpie/internal/agent"
	"github.com/yetone/magpie/internal/environments"
	"github.com/yetone/magpie/internal/gateway"
	"github.com/yetone/magpie/internal/provider"
	"github.com/yetone/magpie/internal/settings"
)

// The Environments page (AgentDock): this computer and each WSL distro,
// side by side — whether the gateway listens here and whether each distro
// reaches it from inside, each distro turned on or off, an agent's setup
// copied onto its twins, WSL switched to mirrored networking, a distro's
// own magpie's providers brought in, and other magpies that would write
// the same files.

// envJSON is one environment as the page draws it.
type envJSON struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"` // windows or wsl
	State string `json:"state"`
	// Reach (a distro's row) is whether the distro reaches the gateway, as
	// a probe run inside it said
	Reach *environments.Reach `json:"reach,omitempty"`
	*agent.WSLEnv
	// HostListener (this computer's row) is whether the gateway accepts a
	// connection here; nothing about WSL
	HostListener *bool    `json:"hostListener,omitempty"`
	HostAgents   []string `json:"hostAgents,omitempty"`
}

type envsJSON struct {
	Environments []envJSON `json:"environments"`
	// Looked is false with Detect agents in WSL off: no distro was asked
	Looked     bool   `json:"looked"`
	Gateway    string `json:"gateway"`
	ServedHere bool   `json:"servedHere"`
	// HeldElsewhere: something other than this process holds the
	// gateway's port, so agents set up here reach it, not this engine.
	// Holder says which magpie, when it answers as one (one behind a key
	// answers 401 and names nothing).
	HeldElsewhere bool                       `json:"heldElsewhere"`
	Holder        *gateway.Served            `json:"holder,omitempty"`
	Others        []environments.OtherMagpie `json:"others"`
	OthersOK      bool                       `json:"othersOK"`
	// FirstRun is whether this magpie has no providers yet: the page
	// offers to bring them in
	FirstRun bool `json:"firstRun"`
	Probing  bool `json:"probing"`
	// ListError is why wsl.exe -l failed; last known distros stay listed.
	ListError string                  `json:"listError,omitempty"`
	WSLConfig *environments.WSLConfig `json:"wslconfig,omitempty"`
}

// envState builds the page's state; it never waits on a probe of a
// distro, starting one in the background when the last is stale.
func envState() envsJSON {
	gw := gateway.URL()
	listen := environments.HostListener(gw)
	out := envsJSON{Gateway: gw, ServedHere: served.Load() != nil, Environments: []envJSON{
		{ID: "windows", Kind: "windows", State: "running", HostListener: &listen, HostAgents: agent.HostAgents()},
	}}
	if !out.ServedHere && listen {
		out.HeldElsewhere = true
		if o := gateway.ServedBy(); o.Running {
			out.Holder = &o
		}
	}
	out.Others, out.OthersOK = environments.OtherMagpies()
	envs, looked := agent.WSLEnvs()
	out.Looked = looked
	out.ListError = agent.WSLListError()
	var targets []environments.Target
	for _, e := range envs {
		if e.Enabled && e.Running {
			targets = append(targets, environments.Target{Distro: e.Distro, URL: e.Gateway})
		}
	}
	if len(targets) > 0 && environments.Stale(targets) {
		out.Probing = true
		go environments.Probe(targets)
	}
	reach := environments.Reaches()
	for _, e := range envs {
		state := "asleep"
		if e.Running {
			state = "running"
		}
		r, ok := reach[e.Distro]
		if !e.Enabled {
			r = environments.Reach{Status: environments.ReachNotProbed, Detail: "turned off"}
		} else if !ok {
			r = environments.Reach{Status: environments.ReachNotProbed}
		}
		out.Environments = append(out.Environments, envJSON{ID: "wsl:" + e.Distro, Kind: "wsl", State: state, Reach: &r, WSLEnv: &e})
	}
	if c, err := environments.ReadWSLConfig(); err == nil {
		out.WSLConfig = &c
	}
	stored, err := provider.Stored()
	out.FirstRun = err == nil && len(stored) == 0
	return out
}

// envErr says err on the page with the status the cause asks for.
func envErr(rw http.ResponseWriter, err error) {
	if errors.Is(err, os.ErrPermission) {
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(rw).Encode(map[string]string{"error": err.Error()})
		return
	}
	fail(rw, err)
}

func environmentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/environments", func(rw http.ResponseWriter, r *http.Request) {
		writeJSON(rw, envState())
	})
	// rescan agent and executable caches, then probe reachability
	mux.HandleFunc("POST /api/environments/reach/refresh", func(rw http.ResponseWriter, r *http.Request) {
		agent.Rescan()
		agent.WSLRefresh()
		envs, _ := agent.WSLEnvs()
		var targets []environments.Target
		for _, e := range envs {
			if e.Enabled && e.Running {
				targets = append(targets, environments.Target{Distro: e.Distro, URL: e.Gateway})
			}
		}
		environments.Probe(targets)
		writeJSON(rw, envState())
	})
	// a distro turned off or on (Settings' WSLDistrosOff)
	mux.HandleFunc("POST /api/environments/{distro}/enabled", func(rw http.ResponseWriter, r *http.Request) {
		var in struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			fail(rw, err)
			return
		}
		distro := strings.TrimSpace(r.PathValue("distro"))
		if distro == "" {
			fail(rw, errors.New("no distro named"))
			return
		}
		s, err := settings.LoadChecked()
		if err != nil {
			fail(rw, err)
			return
		}
		if s.SetWSLDistroOff(distro, !in.Enabled) {
			if err := settings.Save(s); err != nil {
				fail(rw, err)
				return
			}
		}
		writeJSON(rw, envState())
	})
	mux.HandleFunc("GET /api/environments/wslconfig", func(rw http.ResponseWriter, r *http.Request) {
		c, err := environments.ReadWSLConfig()
		if err != nil {
			envErr(rw, err)
			return
		}
		writeJSON(rw, c)
	})
	// .wslconfig asked for mirrored networking, its copy kept; WSL reads it
	// after a shutdown, which the page asks for apart
	mux.HandleFunc("POST /api/environments/wslconfig/mirrored", func(rw http.ResponseWriter, r *http.Request) {
		backup, err := environments.SetMirrored()
		if err != nil {
			envErr(rw, err)
			return
		}
		running, _ := environments.RunningDistros()
		c, _ := environments.ReadWSLConfig()
		writeJSON(rw, map[string]any{"backup": backup, "running": orEmpty(running), "wslconfig": c})
	})
	mux.HandleFunc("POST /api/environments/wslconfig/restore", func(rw http.ResponseWriter, r *http.Request) {
		from, err := environments.RestoreLatest()
		if err != nil {
			envErr(rw, err)
			return
		}
		c, _ := environments.ReadWSLConfig()
		writeJSON(rw, map[string]any{"restored": from, "wslconfig": c})
	})
	// the distros a shutdown would stop, for the page to name them first
	mux.HandleFunc("GET /api/environments/wsl/running", func(rw http.ResponseWriter, r *http.Request) {
		running, err := environments.RunningDistros()
		if err != nil {
			fail(rw, err)
			return
		}
		writeJSON(rw, map[string]any{"running": orEmpty(running)})
	})
	// wsl --shutdown, only when the page says the user confirmed it
	mux.HandleFunc("POST /api/environments/wsl/shutdown", func(rw http.ResponseWriter, r *http.Request) {
		var in struct {
			Confirm bool `json:"confirm"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			fail(rw, err)
			return
		}
		if !in.Confirm {
			fail(rw, errors.New("a shutdown stops every WSL distro: confirm it first"))
			return
		}
		if err := environments.Shutdown(); err != nil {
			fail(rw, err)
			return
		}
		writeJSON(rw, envState())
	})
	type syncIn struct {
		Source  string   `json:"source"`
		Targets []string `json:"targets"`
	}
	readSync := func(rw http.ResponseWriter, r *http.Request) (syncIn, bool) {
		var in syncIn
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			fail(rw, err)
			return in, false
		}
		if in.Source == "" || len(in.Targets) == 0 {
			fail(rw, errors.New("name the agent to copy from and at least one to copy onto"))
			return in, false
		}
		return in, true
	}
	mux.HandleFunc("POST /api/environments/sync/preview", func(rw http.ResponseWriter, r *http.Request) {
		in, ok := readSync(rw, r)
		if !ok {
			return
		}
		p, err := environments.Preview(in.Source, in.Targets)
		if err != nil {
			fail(rw, err)
			return
		}
		writeJSON(rw, p)
	})
	mux.HandleFunc("POST /api/environments/sync/apply", func(rw http.ResponseWriter, r *http.Request) {
		in, ok := readSync(rw, r)
		if !ok {
			return
		}
		p, err := environments.Apply(in.Source, in.Targets)
		if err != nil {
			fail(rw, err)
			return
		}
		writeJSON(rw, p)
	})
	// a running distro's own magpie, its providers to bring in
	mux.HandleFunc("GET /api/environments/import/magpie", func(rw http.ResponseWriter, r *http.Request) {
		writeJSON(rw, map[string]any{"sources": orEmpty(environments.MagpieSources())})
	})
	mux.HandleFunc("POST /api/environments/import/magpie", func(rw http.ResponseWriter, r *http.Request) {
		var in struct {
			Distro string `json:"distro"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			fail(rw, err)
			return
		}
		res, err := environments.ImportMagpie(in.Distro)
		if err != nil {
			fail(rw, err)
			return
		}
		writeJSON(rw, res)
	})
}

// orEmpty is xs, or an empty list for the page rather than null.
func orEmpty[T any](xs []T) []T {
	if xs == nil {
		return []T{}
	}
	return xs
}
