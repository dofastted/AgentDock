package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Native Gemini behind a Windows PATH candidate is the distro's CLI, not
// missing and not the Windows npm launcher (#406 / B3).
func TestParseProbeNativeGeminiBehindWindowsPATH(t *testing.T) {
	d := parseProbe("Debian", ""+
		"home:/home/mci777\n"+
		"user:mci777\n"+
		"winbin:gemini /mnt/c/Users/Administrator/AppData/Roaming/npm/gemini\n"+
		"bin:gemini /home/mci777/.nvm/versions/node/v20.20.0/bin/gemini\n"+
		"win:/mnt/c\n")
	if d == nil {
		t.Fatal("no distro")
	}
	if d.User != "mci777" || d.Home != "/home/mci777" {
		t.Fatalf("user/home %+v", d)
	}
	if !d.Has["bin:gemini"] {
		t.Fatalf("native gemini lost: %v", d.Has)
	}
	if d.Bins["gemini"] != "/home/mci777/.nvm/versions/node/v20.20.0/bin/gemini" {
		t.Fatalf("native path %q", d.Bins["gemini"])
	}
	if d.WinBins["gemini"] != "/mnt/c/Users/Administrator/AppData/Roaming/npm/gemini" {
		t.Fatalf("interop path %q", d.WinBins["gemini"])
	}
	k := wslKindOf("gemini")
	if !k.found(*d) {
		t.Fatal("gemini not found")
	}
	if p := k.provenance(*d); p != ProvenanceNative {
		t.Fatalf("provenance %s", p)
	}
}

// A Windows-only launcher is interop, not a native CLI and not an empty miss.
func TestParseProbeInteropOnlyIsNotNative(t *testing.T) {
	d := parseProbe("Debian", "home:/home/me\nwinbin:gemini /mnt/c/Users/me/AppData/Roaming/npm/gemini\nwin:/mnt/c\n")
	if d == nil {
		t.Fatal("no distro")
	}
	if d.Has["bin:gemini"] {
		t.Fatalf("Windows gemini taken as native: %v", d.Has)
	}
	k := wslKindOf("gemini")
	if k.found(*d) {
		t.Fatal("interop-only gemini listed as a magpie agent")
	}
	if p := k.provenance(*d); p != ProvenanceInterop {
		t.Fatalf("provenance %s", p)
	}
}

func TestParseProbeCustomCodexHomeIsUNC(t *testing.T) {
	d := parseProbe("Debian", "home:/home/me\nuser:me\nenv:CODEX_HOME=/home/me/custom-codex\ndir:.codex\n")
	if d == nil || d.Env["CODEX_HOME"] != "/home/me/custom-codex" || !d.Has["dir:.codex"] {
		t.Fatalf("%+v", d)
	}
	d.Root = `\\wsl.localhost\Debian`
	d.Running = true
	a := wslCodex(*d)
	want := d.local("/home/me/custom-codex")
	if a.Dir != want {
		t.Fatalf("Dir %q, want UNC custom root %q", a.Dir, want)
	}
	if !strings.Contains(a.Path, "config.toml") {
		t.Fatalf("Path %q", a.Path)
	}
}

func TestPlaceGetenvIgnoresHostForDistro(t *testing.T) {
	t.Setenv("CODEX_HOME", filepath.Join(t.TempDir(), "windows-codex"))
	d := distro{Name: "Debian", Home: "/home/me", Root: `\\wsl.localhost\Debian`, Running: true, Env: map[string]string{"CODEX_HOME": "/home/me/wsl-codex"}}
	p := d.place("codex@wsl:Debian")
	if got, want := p.getenv("CODEX_HOME"), d.local("/home/me/wsl-codex"); got != want {
		t.Fatalf("getenv %q, want distro UNC %q", got, want)
	}
	if p.codexHome() != d.local("/home/me/wsl-codex") {
		t.Fatalf("codexHome %q", p.codexHome())
	}
}

func TestWSLProbeFailureKeepsLastSuccess(t *testing.T) {
	codexHome(t, "", "")
	b, _ := json.MarshalIndent(map[string]*distro{"Debian": {Name: "Debian", Home: "/home/me", User: "me", Has: map[string]bool{"dir:.codex": true, "bin:codex": true}, Probe: wslProbeVersion}}, "", "  ")
	osMkdirWrite(t, wslStatePath(), b)
	asked, _ := fakeWSL(t, "Debian\r\n", "Debian\r\n", map[string]string{"Debian": ""}, nil)
	ds := wslDistros()
	if len(ds) != 1 || ds[0].Name != "Debian" || !ds[0].Has["dir:.codex"] {
		t.Fatalf("failed probe dropped last success: %+v", ds)
	}
	if ds[0].ProbeErr == "" {
		t.Fatal("failed probe looked like an empty environment")
	}
	if !sameNames(*asked, "Debian") {
		t.Fatalf("asked %v", *asked)
	}
	envs, _ := WSLEnvs()
	var e WSLEnv
	for _, x := range envs {
		if x.Distro == "Debian" {
			e = x
		}
	}
	if !e.Stale || e.ProbeError == "" || !e.Known {
		t.Fatalf("stale row %+v", e)
	}
}

func TestWSLRefreshRescansThenReachCaches(t *testing.T) {
	codexHome(t, "", "")
	fakeWSL(t, "Debian\r\n", "Debian\r\n", map[string]string{
		"Debian": "home:/home/me\nuser:me\nbin:gemini /home/me/.nvm/bin/gemini\nwin:/mnt/c\n",
	}, map[string]string{"Debian": t.TempDir()})
	if ds := wslDistros(); len(ds) != 1 || !ds[0].Has["bin:gemini"] {
		t.Fatalf("first look %+v", ds)
	}
	wsl.Lock()
	wsl.at, wsl.runAt = time.Now(), time.Now()
	wsl.failed = map[string]time.Time{"Debian": time.Now()}
	wsl.Unlock()
	Rescan()
	wsl.Lock()
	at, failed := wsl.at, len(wsl.failed)
	wsl.Unlock()
	if !at.IsZero() || failed != 0 {
		t.Fatalf("Rescan kept WSL cache: at %v failed %d", at, failed)
	}
	WSLRefresh()
	ds := wslDistros()
	if len(ds) != 1 || !ds[0].Has["bin:gemini"] {
		t.Fatalf("refresh %+v", ds)
	}
}

func TestWSLStoppedDistroNotProbedOnInventory(t *testing.T) {
	codexHome(t, "", "")
	seen := map[string]*distro{"Asleep": {Name: "Asleep", Home: "/home/s", Has: map[string]bool{"dir:.codex": true}, Probe: wslProbeVersion}}
	osMkdirWrite(t, wslStatePath(), []byte(mustJSON(t, seen)))
	asked, opened := fakeWSL(t, "Asleep\r\n", "", nil, nil)
	_ = wslDistros()
	if len(*asked) != 0 || len(*opened) != 0 {
		t.Fatalf("inventory woke a stopped distro: asked %v opened %v", *asked, *opened)
	}
}

func osMkdirWrite(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestWSLEnvDetailsExposeNativeAndInterop(t *testing.T) {
	d := parseProbe("Debian", "home:/home/me\nuser:me\ndir:.codex\nbin:codex /usr/local/bin/codex\nwinbin:gemini /mnt/c/npm/gemini\nwin:/mnt/c\n")
	d.Root = `\\wsl.localhost\Debian`
	d.Running = true
	details := wslAgentDetails(*d)
	var gemini, codex WSLAgentInfo
	for _, i := range details {
		switch i.Kind {
		case "gemini":
			gemini = i
		case "codex":
			codex = i
		}
	}
	if gemini.Provenance != ProvenanceInterop || gemini.InteropPath == "" {
		t.Fatalf("gemini %+v", gemini)
	}
	if codex.Provenance != ProvenanceNative || codex.Path != "/usr/local/bin/codex" {
		t.Fatalf("codex %+v", codex)
	}
}
