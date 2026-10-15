package environments

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/agent"
	"github.com/yetone/magpie/internal/agentenv"
	"github.com/yetone/magpie/internal/library"
	"github.com/yetone/magpie/internal/provider"
	"github.com/yetone/magpie/internal/testenv"
)

// syncSandbox is a home with Codex on this computer and in a running WSL
// distro, Ubuntu, and one provider with models; it returns the distro's
// Codex config as magpie opens it.
func syncSandbox(t *testing.T) string {
	t.Helper()
	h := t.TempDir()
	testenv.SetHome(t, h)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(h, ".config"))
	t.Setenv("APPDATA", "")
	t.Setenv("LOCALAPPDATA", "")
	for _, k := range agentenv.Vars {
		t.Setenv(k, "")
	}
	bin := filepath.Join(h, "bin")
	testenv.Program(t, filepath.Join(bin, "codex"), "#!/bin/sh\n")
	writeFile(t, filepath.Join(bin, "codex.exe"), "")
	t.Setenv("PATH", bin)
	writeFile(t, filepath.Join(h, ".codex", "config.toml"), "model = \"gpt-5.5\"\n")

	root := t.TempDir()
	cfg := filepath.Join(root, "home", "me", ".codex", "config.toml")
	writeFile(t, cfg, "model = \"o3\"\n")
	t.Cleanup(agent.FakeWSL(map[string]string{"Ubuntu": "home:/home/me\ndir:.codex\nbin:/usr/bin/codex\nnet:mirrored\n"},
		map[string]string{"Ubuntu": root}))
	if err := provider.Save(provider.Provider{ID: "relay", Name: "Relay", Chat: "https://relay.example.test/v1", Key: "k",
		Models: []string{"gpt-5.5", "gpt-5.4"}}); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func writeFile(t *testing.T, p, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

const twin = "codex@wsl:Ubuntu"

// Codex connected on this computer, with a server from the library, is
// copied onto Codex in WSL: the preview names each change and writes
// nothing; the apply connects the twin on the same model and gives it the
// same server, in the distro's own config.
func TestSyncCodexToItsTwin(t *testing.T) {
	cfg := syncSandbox(t)
	src, err := agent.Find("codex")
	if err != nil {
		t.Fatal(err)
	}
	if err := src.Connect(); err != nil {
		t.Fatal(err)
	}
	if _, err := library.SaveServer("", library.Server{Name: "fetch", Transport: "stdio", Command: "uvx",
		Args: []string{"mcp-server-fetch"}, Agents: []string{"codex"}}); err != nil {
		t.Fatal(err)
	}
	model := src.Values()["model"]
	before, _ := os.ReadFile(cfg)

	p, err := Preview("codex", []string{twin, "claude", "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Targets) != 3 || p.Targets[1].Skip == "" || p.Targets[2].Skip == "" {
		t.Fatalf("targets: %+v", p.Targets)
	}
	var what []string
	for _, c := range p.Targets[0].Changes {
		what = append(what, c.What+" "+c.Key+" "+c.To)
	}
	got := strings.Join(what, "|")
	if !strings.HasPrefix(got, "connect") || !strings.Contains(got, "field model "+model) || !strings.Contains(got, "library mcp:fetch on") {
		t.Fatalf("preview: %s", got)
	}
	if after, _ := os.ReadFile(cfg); string(after) != string(before) {
		t.Fatalf("the preview wrote the distro's config:\n%s", after)
	}

	a, err := Apply("codex", []string{twin})
	if err != nil {
		t.Fatal(err)
	}
	if a.Targets[0].Error != "" || len(a.Problems) != 0 {
		t.Fatalf("apply: %+v", a)
	}
	tw, err := agent.Find(twin)
	if err != nil {
		t.Fatal(err)
	}
	if !tw.Wired() || tw.Values()["model"] != model {
		t.Fatalf("the twin: wired %v, model %q, want %q", tw.Wired(), tw.Values()["model"], model)
	}
	s, _ := os.ReadFile(cfg)
	if !strings.Contains(string(s), "[mcp_servers.fetch]") {
		t.Fatalf("the distro's config:\n%s", s)
	}
	if again, _ := Preview("codex", []string{twin}); len(again.Targets[0].Changes) != 0 {
		t.Fatalf("changes left after the apply: %+v", again.Targets[0].Changes)
	}
}

// A twin in a stopped distro is skipped, saying so, and nothing is read
// or written there: opening its files would start it.
func TestSyncSkipsAStoppedDistro(t *testing.T) {
	cfg := syncSandbox(t)
	if _, err := agent.Find(twin); err != nil {
		t.Fatal(err)
	}
	agent.StopFakeWSL("Ubuntu")
	before, _ := os.ReadFile(cfg)
	p, err := Apply("codex", []string{twin})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Targets) != 1 || p.Targets[0].Skip == "" {
		t.Fatalf("targets: %+v", p.Targets)
	}
	if after, _ := os.ReadFile(cfg); string(after) != string(before) {
		t.Fatal("the stopped distro's config was written")
	}
}
