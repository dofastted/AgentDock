package library

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/agent"
)

// A WSL Gemini CLI is a library target at the distro's files, not this
// machine's ~/.gemini. Named omp profiles keep the profile folder. Custom
// Agent.Dir from discovery is the skills/MCP root.
func TestWSLTargetsHonorDiscoveryDir(t *testing.T) {
	sandbox(t)
	root := t.TempDir()
	h := filepath.Join(root, "home", "me")
	write(t, filepath.Join(h, ".gemini", "settings.json"), "{}\n")
	write(t, filepath.Join(h, ".omp", "profiles", "work", "agent", "config.yml"), "{}\n")
	custom := filepath.Join(h, "opt", "codex")
	write(t, filepath.Join(custom, "config.toml"), "model = \"gpt-5.5\"\n")
	t.Cleanup(agent.FakeWSL(map[string]string{
		"Debian": "home:/home/me\nbin:gemini\ndir:.omp\nbin:omp\n",
	}, map[string]string{"Debian": root}))

	gem := targetByID("gemini@wsl:Debian")
	if gem == nil {
		t.Fatalf("gemini@wsl isn't a target: %v", ids(Targets()))
	}
	wantSkills := filepath.Join(h, ".gemini", "skills")
	if gem.Skills != wantSkills || gem.Instructions != filepath.Join(h, ".gemini", "GEMINI.md") || gem.MCP == nil || gem.MCP.Path != filepath.Join(h, ".gemini", "settings.json") || !gem.Copy || !gem.MCP.WSL || gem.MCP.Distro != "Debian" {
		t.Errorf("gemini@wsl: %+v mcp %+v", gem, gem.MCP)
	}
	if within(gem.Skills, home()) && !within(gem.Skills, h) {
		t.Errorf("gemini@wsl leaked this machine's home: %s vs %s", gem.Skills, home())
	}

	prof := filepath.Join(h, ".omp", "profiles", "work", "agent")
	omp := targetOf(&agent.Agent{
		ID: "omp#work@wsl:Debian", Name: "omp · work · WSL Debian",
		WSL: "Debian", Home: h, Dir: prof, Path: filepath.Join(prof, "config.yml"),
	})
	if omp == nil || omp.Agent.ID != "omp#work@wsl:Debian" || omp.Skills != filepath.Join(prof, "skills") || omp.MCP == nil || omp.MCP.Path != filepath.Join(prof, "mcp.json") || !omp.Copy {
		t.Errorf("omp#work@wsl: %+v mcp %+v", omp, omp.MCP)
	}

	cx := targetOf(&agent.Agent{
		ID: "codex@wsl:Debian", Name: "Codex · WSL Debian",
		WSL: "Debian", Home: h, Dir: custom, Path: filepath.Join(custom, "config.toml"),
	})
	if cx == nil || cx.Skills != filepath.Join(custom, "skills") || cx.MCP == nil || cx.MCP.Path != filepath.Join(custom, "config.toml") {
		t.Errorf("codex custom Dir: %+v mcp %+v", cx, cx.MCP)
	}
}

// A named omp profile on this machine is omp's target in the profile folder,
// not the default ~/.omp/agent.
func TestNamedOmpProfileKeepsFolder(t *testing.T) {
	h := sandbox(t)
	dir := filepath.Join(h, ".omp", "profiles", "lab", "agent")
	write(t, filepath.Join(dir, "config.yml"), "{}\n")
	tg := targetOf(&agent.Agent{ID: "omp#lab", Name: "omp · lab", Dir: dir, Path: filepath.Join(dir, "config.yml")})
	if tg == nil || tg.Agent.ID != "omp#lab" || tg.Skills != filepath.Join(dir, "skills") || tg.Instructions != filepath.Join(dir, "AGENTS.md") || tg.MCP == nil || tg.MCP.Path != filepath.Join(dir, "mcp.json") {
		t.Errorf("omp#lab: %+v mcp %+v", tg, tg.MCP)
	}
	if _, err := Takes("omp#lab", "skills"); err != nil {
		// Takes uses agent.Find; the profile row may be missing without omp's
		// detect. targetOf itself is the contract this test owns.
		t.Log(err)
	}
}

// Same-named shared skills in Windows and a WSL distro stay separate found
// rows; bringing the name in does not move the WSL folder.
func TestWSLSharedSkillsStayInEnvironment(t *testing.T) {
	h := sandbox(t)
	root := t.TempDir()
	wh := filepath.Join(root, "home", "me")
	write(t, filepath.Join(wh, ".codex", "config.toml"), "model = \"gpt-5.5\"\n")
	t.Cleanup(agent.FakeWSL(map[string]string{
		"Debian": "home:/home/me\ndir:.codex\nbin:codex\n",
	}, map[string]string{"Debian": root}))

	hostShared := filepath.Join(h, ".agents", "skills", "codegraph")
	wslShared := filepath.Join(wh, ".agents", "skills", "codegraph")
	skill(t, hostShared, "codegraph", "Windows")
	skill(t, wslShared, "codegraph", "Debian")

	v, err := Read(nil)
	if err != nil {
		t.Fatal(err)
	}
	var host, wsl *FoundSkill
	for i := range v.FoundSkills {
		f := &v.FoundSkills[i]
		if f.Name != "codegraph" {
			continue
		}
		switch f.Env {
		case "":
			host = f
		case "wsl:Debian":
			wsl = f
		}
	}
	if host == nil || host.Shared != hostShared {
		t.Errorf("host codegraph: %+v found %+v", host, v.FoundSkills)
	}
	if wsl == nil || wsl.Shared != wslShared {
		t.Errorf("wsl codegraph: %+v found %+v", wsl, v.FoundSkills)
	}

	cx := targetByID("codex")
	wcx := targetByID("codex@wsl:Debian")
	if cx == nil || wcx == nil {
		t.Fatalf("targets: %v", ids(Targets()))
	}
	if slices.Contains(cx.SkillsAlso, "codex@wsl:Debian") || slices.Contains(wcx.SkillsAlso, "codex") {
		t.Errorf("shared roots collapsed: host %v wsl %v", cx.SkillsAlso, wcx.SkillsAlso)
	}

	ok(t)(ImportSkill("codegraph"))
	if fi, err := os.Lstat(wslShared); err != nil || !fi.IsDir() {
		t.Errorf("WSL shared folder was moved: %v", err)
	}
	if read(t, filepath.Join(wslShared, "SKILL.md")) == "" || !strings.Contains(read(t, filepath.Join(wslShared, "SKILL.md")), "Debian") {
		t.Error("WSL skill content was lost")
	}
	if !linked(skillDir("codegraph")) || realDir(skillDir("codegraph")) != realDir(hostShared) {
		t.Errorf("library linked to %s, want host %s", realDir(skillDir("codegraph")), realDir(hostShared))
	}
}

// A skill that exists only in a WSL agent's folder is linked into the
// library, never moved out of the distro.
func TestImportWSLSkillDoesNotMove(t *testing.T) {
	sandbox(t)
	root := t.TempDir()
	wh := filepath.Join(root, "home", "me")
	write(t, filepath.Join(wh, ".codex", "config.toml"), "model = \"gpt-5.5\"\n")
	native := filepath.Join(wh, ".codex", "skills", "notes")
	skill(t, native, "notes", "WSL notes")
	t.Cleanup(agent.FakeWSL(map[string]string{
		"Debian": "home:/home/me\ndir:.codex\nbin:codex\n",
	}, map[string]string{"Debian": root}))

	ok(t)(ImportSkill("notes"))
	if fi, err := os.Lstat(native); err != nil || !fi.IsDir() {
		t.Fatalf("native WSL folder was moved: %v", err)
	}
	if !linked(skillDir("notes")) || realDir(skillDir("notes")) != realDir(native) {
		t.Errorf("library is %s, want link to %s", realDir(skillDir("notes")), realDir(native))
	}
}

func TestWindowsMCPCommand(t *testing.T) {
	for _, cmd := range []string{`C:\Tools\uvx.exe`, `uvx.exe`, `npx.cmd`, `D:/bin/tool.bat`} {
		if !WindowsMCPCommand(cmd) {
			t.Errorf("%q is a Windows program", cmd)
		}
	}
	for _, cmd := range []string{"uvx", "npx", "/usr/bin/uvx", ""} {
		if WindowsMCPCommand(cmd) {
			t.Errorf("%q is not a Windows program", cmd)
		}
	}
}
