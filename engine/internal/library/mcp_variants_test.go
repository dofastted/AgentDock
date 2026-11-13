package library

import (
	"runtime"
	"slices"
	"strings"
	"testing"
)

func variantServer() *Server {
	return &Server{
		Name: "grok-search", Transport: "stdio",
		Command: `C:\Tools\uvx.exe`, Args: []string{"grok-search"}, Env: map[string]string{"K": "win"},
		Variants: map[string]ServerVariant{
			"wsl":             {Command: "uvx", Args: []string{"grok-search"}},
			"wsl:Debian":      {Command: "/home/me/.local/bin/uvx", Args: []string{"grok-search"}},
			"grok@wsl:Debian": {Env: map[string]string{"K": "grok"}},
		},
	}
}

func TestVariantMostSpecificWins(t *testing.T) {
	s := variantServer()
	cases := []struct {
		f          mcpFile
		cmd, env   string
		wantKeyArg string
	}{
		{mcpFile{Agent: "claude@wsl:Ubuntu", WSL: true, Distro: "Ubuntu"}, "uvx", "win", "grok-search"},
		{mcpFile{Agent: "claude@wsl:Debian", WSL: true, Distro: "Debian"}, "/home/me/.local/bin/uvx", "win", "grok-search"},
		// an agent's own variant wins whole: its env only, the rest the server's
		{mcpFile{Agent: "grok@wsl:Debian", WSL: true, Distro: "Debian"}, `C:\Tools\uvx.exe`, "grok", "grok-search"},
		{mcpFile{Agent: "claude"}, `C:\Tools\uvx.exe`, "win", "grok-search"},
	}
	for _, c := range cases {
		got := c.f.resolve(s)
		if got.Command != c.cmd || got.Env["K"] != c.env || !slices.Equal(got.Args, []string{c.wantKeyArg}) {
			t.Errorf("%s: got %s %v %v", c.f.Agent, got.Command, got.Args, got.Env)
		}
		if got.Variants != nil {
			t.Errorf("%s: resolved server still has variants", c.f.Agent)
		}
	}
	if s.Command != `C:\Tools\uvx.exe` || len(s.Variants) != 3 {
		t.Fatal("resolving changed the library's server")
	}
}

func TestVariantLetsWSLStartAWindowsServer(t *testing.T) {
	s := variantServer()
	f := &mcpFile{Agent: "codex@wsl:Ubuntu", WSL: true, Distro: "Ubuntu"}
	got, err := f.side(s)
	if err != nil || got.Command != "uvx" {
		t.Fatalf("side = %v, %v", got, err)
	}
	if err := f.supports(s); err != nil {
		t.Fatalf("supports: %v", err)
	}
	// without a variant it is still refused
	plain := *s
	plain.Variants = nil
	if _, err := f.side(&plain); err == nil {
		t.Fatal("a Windows program was given to WSL")
	}
}

// A server an agent in WSL has, whose command is that distro's Linux
// program, isn't written into a Windows agent's config, where it can't
// start (AgentDock B1: codegraph-mcp was in Windows Codex's config.toml).
// Its windows variant is what that agent is given.
func TestLinuxCommandRefusedOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows host only")
	}
	s := &Server{Name: "codegraph", Transport: "stdio", Command: "/home/me/.local/bin/codegraph-mcp",
		Agents: []string{"claude", "claude@wsl:Debian"}}
	f := &mcpFile{Agent: "claude"}
	if _, err := f.side(s); err == nil || !strings.Contains(err.Error(), "Linux program") {
		t.Fatalf("side = %v", err)
	}
	if err := f.supports(s); err == nil {
		t.Fatal("supports took it")
	}
	s.Variants = map[string]ServerVariant{"windows": {Command: `C:\Tools\codegraph-mcp.exe`}}
	got, err := f.side(s)
	if err != nil || got.Command != `C:\Tools\codegraph-mcp.exe` || len(got.Args) != 0 {
		t.Fatalf("side = %v, %v", got, err)
	}
	// the WSL twin keeps the server's own Linux command
	w := &mcpFile{Agent: "claude@wsl:Debian", WSL: true, Distro: "Debian"}
	if got, err := w.side(s); err != nil || got.Command != "/home/me/.local/bin/codegraph-mcp" {
		t.Fatalf("wsl side = %v, %v", got, err)
	}
}

// A command typed for this machine's agents alone is written as typed:
// magpie never checked that a program is there, and an absolute POSIX
// path is one on macOS and Linux (upstream's local-server tests).
func TestPosixCommandKeptForHostOnlyServer(t *testing.T) {
	s := &Server{Name: "here", Transport: "stdio", Command: "/opt/mine/mcp", Agents: []string{"claude"}}
	f := &mcpFile{Agent: "claude"}
	if got, err := f.side(s); err != nil || got.Command != "/opt/mine/mcp" {
		t.Fatalf("side = %v, %v", got, err)
	}
}

// A Windows program the distro starts through WSL's interop, named for
// that environment by its path there (the user's agents.yaml had
// /mnt/d/code/CC/feedback.exe for grok in Debian), is the user's own:
// magpie writes it as it is. One the distro can't start (C:\…) is still
// refused, variant or not.
func TestWSLVariantMayNameAWindowsProgramByItsPathThere(t *testing.T) {
	s := &Server{Name: "feedback", Transport: "stdio", Command: `D:\code\CC\feedback.exe`,
		Agents:   []string{"grok", "grok@wsl:Debian"},
		Variants: map[string]ServerVariant{"grok@wsl:Debian": {Command: "/mnt/d/code/CC/feedback.exe"}}}
	f := &mcpFile{Agent: "grok@wsl:Debian", WSL: true, Distro: "Debian"}
	got, err := f.side(s)
	if err != nil || got.Command != "/mnt/d/code/CC/feedback.exe" {
		t.Fatalf("side = %v, %v", got, err)
	}
	if err := f.supports(s); err != nil {
		t.Fatalf("supports: %v", err)
	}
	// by the distro's key, as a found server folds it (foundServersOf)
	s.Variants = map[string]ServerVariant{"wsl:Debian": {Command: "/mnt/d/code/CC/feedback.exe"}}
	if got, err := f.side(s); err != nil || got.Command != "/mnt/d/code/CC/feedback.exe" {
		t.Fatalf("by wsl:Debian: %v, %v", got, err)
	}
	if err := f.supports(s); err != nil {
		t.Fatalf("supports by wsl:Debian: %v", err)
	}
	// the server's own command, inherited rather than named here, is not
	s.Variants = nil
	if _, err := f.side(s); err == nil {
		t.Fatal("a Windows path went to WSL")
	}
	if err := f.supports(s); err == nil {
		t.Fatal("supports took an inherited Windows command")
	}
	// nor is a Windows path named for the distro
	s.Variants = map[string]ServerVariant{"wsl": {Command: `D:\code\CC\feedback.exe`}}
	if _, err := f.side(s); err == nil {
		t.Fatal("C:\\… named for WSL went through")
	}
}

func TestCheckVariants(t *testing.T) {
	s := &Server{Name: "x", Transport: "stdio", Command: "x", Variants: map[string]ServerVariant{
		" wsl ":      {Command: " uvx "},
		"windows":    {},
		"omp#work":   {Args: []string{}},
		"":           {Command: "y"},
		"wsl:Debian": {Env: map[string]string{" ": "z", "A": "b"}},
	}}
	if err := s.check(); err != nil {
		t.Fatal(err)
	}
	if len(s.Variants) != 3 || s.Variants["wsl"].Command != "uvx" || len(s.Variants["wsl:Debian"].Env) != 1 {
		t.Fatalf("variants = %#v", s.Variants)
	}
	if _, ok := s.Variants["omp#work"]; !ok {
		t.Fatal("an empty argument list is a variant")
	}
	s.Variants = map[string]ServerVariant{"bad key": {Command: "x"}}
	if err := s.check(); err == nil {
		t.Fatal("a key naming nothing was taken")
	}
	r := &Server{Name: "r", Transport: "http", URL: "https://x", Variants: map[string]ServerVariant{"wsl": {Command: "x"}}}
	if err := r.check(); err != nil || r.Variants != nil {
		t.Fatalf("remote kept variants: %v %v", r.Variants, err)
	}
}
