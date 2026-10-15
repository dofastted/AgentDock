package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/sessions"
	"github.com/yetone/magpie/internal/settings"
)

// A distro turned off (WSLDistrosOff) is left alone while the others are
// looked in: none of its agents or sessions is shown, wsl.exe is asked
// nothing about it, and it is said not to run; turned on again its agents
// are back. Detect agents in WSL off still turns every distro off.
func TestWSLDistroOff(t *testing.T) {
	syncHome(t)
	roots := map[string]string{}
	probes := map[string]string{}
	ids := map[string]string{
		"Ubuntu": "55555555-2222-3333-4444-555555555555",
		"Debian": "66666666-2222-3333-4444-555555555555",
	}
	for distro, id := range ids {
		root := t.TempDir()
		project := filepath.Join(root, "home", "me", ".claude", "projects", "-home-me-app")
		if err := os.MkdirAll(project, 0o755); err != nil {
			t.Fatal(err)
		}
		line := `{"type":"user","message":{"role":"user","content":"Tidy the README"},"uuid":"u1","timestamp":"2026-09-20T10:00:01.000Z","cwd":"/home/me/app","sessionId":"` + id + `"}` + "\n"
		if err := os.WriteFile(filepath.Join(project, id+".jsonl"), []byte(line), 0o644); err != nil {
			t.Fatal(err)
		}
		roots[distro], probes[distro] = root, "home:/home/me\ndir:.claude\n"
	}
	t.Cleanup(FakeWSL(probes, roots))
	var asked []string
	run := wslRun
	wslRun = func(d time.Duration, args ...string) ([]byte, error) {
		asked = append(asked, strings.Join(args, " "))
		return run(d, args...)
	}
	sessions.Reset()
	t.Cleanup(sessions.Reset)
	t.Cleanup(func() { settings.Save(settings.Settings{}) })
	agentsIn := func() map[string]int {
		n := map[string]int{}
		for _, a := range All() {
			if a.WSL != "" {
				n[a.WSL]++
			}
		}
		return n
	}
	listed := func(id string) bool {
		for _, s := range sessions.List(0) {
			if s.ID == id {
				return true
			}
		}
		return false
	}
	askedOf := func(distro string) []string {
		var out []string
		for _, a := range asked {
			if strings.HasPrefix(a, "-d "+distro+" ") {
				out = append(out, a)
			}
		}
		return out
	}

	if n := agentsIn(); n["Ubuntu"] == 0 || n["Debian"] == 0 || !listed(ids["Debian"]) {
		t.Fatalf("both distros on: agents %v, Debian's session listed %v", n, listed(ids["Debian"]))
	}

	if err := settings.Save(settings.Settings{WSLDistrosOff: []string{"debian"}}); err != nil {
		t.Fatal(err)
	}
	// what was probed is forgotten, so a distro turned off would be asked again
	wsl.Lock()
	wsl.probed = map[string]bool{}
	wsl.Unlock()
	sessions.Reset()
	asked = nil
	if n := agentsIn(); n["Debian"] != 0 || n["Ubuntu"] == 0 {
		t.Fatalf("Debian off: agents %v", n)
	}
	if listed(ids["Debian"]) || !listed(ids["Ubuntu"]) {
		t.Fatalf("Debian off: Debian's session listed %v, Ubuntu's %v", listed(ids["Debian"]), listed(ids["Ubuntu"]))
	}
	if WSLRunning("Debian") || !WSLRunning("Ubuntu") {
		t.Fatal("a distro turned off is said to run, or one on isn't")
	}
	if got := askedOf("Debian"); len(got) != 0 {
		t.Fatalf("wsl.exe asked of a distro turned off: %q", got)
	}

	if err := settings.Save(settings.Settings{NoWSLAgents: true}); err != nil {
		t.Fatal(err)
	}
	if n := agentsIn(); len(n) != 0 {
		t.Fatalf("Detect agents in WSL off: agents %v", n)
	}

	if err := settings.Save(settings.Settings{}); err != nil {
		t.Fatal(err)
	}
	if n := agentsIn(); n["Debian"] == 0 {
		t.Fatalf("turned on again, Debian's agents aren't back: %v", n)
	}
}
