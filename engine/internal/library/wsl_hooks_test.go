package library

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A hook ticked for an agent in WSL is written into that agent's own file
// in the distro, in its own shape, its Windows drive paths as the distro
// mounts them; this computer's agent gets the command as it was. One that
// runs a Windows program isn't given to the distro, and says why.
func TestWSLHooks(t *testing.T) {
	h := wslSandbox(t)
	ok(t)(SaveHook("", Hook{Name: "notify", Event: "turn-end", Command: `C:\Tools\notify.sh --to "D:\logs\done.txt"`,
		Agents: []string{"claude", wslClaude, wslCodex}}))
	want := `/mnt/c/Tools/notify.sh --to \"/mnt/d/logs/done.txt\"`
	for _, f := range []string{".claude/settings.json", ".codex/hooks.json"} {
		got := hooksOf(t, filepath.Join(h, f), "Stop")
		if len(got) != 1 || !strings.Contains(got[0], want) {
			t.Errorf("%s in the distro: %v", f, got)
		}
	}
	if got := hooksOf(t, filepath.Join(home(), ".claude/settings.json"), "Stop"); len(got) != 1 || !strings.Contains(got[0], `C:\\Tools\\notify.sh`) {
		t.Errorf("this computer's claude: %v", got)
	}

	res, err := SaveHook("", Hook{Name: "toast", Event: "turn-end", Command: `"C:\Program Files\Toast\toast.exe" -m done`,
		Agents: []string{"claude", wslClaude}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Problems) != 1 || res.Problems[0].Agent != wslClaude || !strings.Contains(res.Problems[0].Error, "Windows program") {
		t.Errorf("problems: %+v", res.Problems)
	}
	if got := hooksOf(t, filepath.Join(h, ".claude/settings.json"), "Stop"); len(got) != 1 {
		t.Errorf("the Windows program was given to claude@wsl: %v", got)
	}
	// the agent it can't go to is unticked, so it is no problem of the next change
	ok(t)(HookAgents("toast", []string{"claude"}))

	// an agent's own event goes to its twin in WSL as well
	ok(t)(SaveHook("", Hook{Name: "ask", Event: "codex:PermissionRequest", Command: "notify", Agents: []string{wslCodex}}))
	if got := hooksOf(t, filepath.Join(h, ".codex/hooks.json"), "PermissionRequest"); len(got) != 1 {
		t.Errorf("codex@wsl PermissionRequest: %v", got)
	}

	v, err := Read(nil)
	if err != nil {
		t.Fatal(err)
	}
	j := slices.IndexFunc(v.HookAgents, func(a HookAgentView) bool { return a.ID == wslClaude })
	if j < 0 || v.HookAgents[j].No != "" || !v.HookAgents[j].WSL || !strings.HasPrefix(v.HookAgents[j].File, h) {
		t.Errorf("claude@wsl's view: %+v", v.HookAgents)
	}

	// taken out, magpie's entry leaves the distro's file
	ok(t)(HookAgents("notify", []string{"claude"}))
	if got := hooksOf(t, filepath.Join(h, ".codex/hooks.json"), "Stop"); len(got) != 0 {
		t.Errorf("codex@wsl kept the hook: %v", got)
	}
}

// A subagent ticked for an agent in WSL is its file in the agent's folder
// in the distro, the same file this computer's agent gets.
func TestWSLSubagents(t *testing.T) {
	h := wslSandbox(t)
	ok(t)(SaveSubagent("", Subagent{Name: "reviewer", Description: "Reviews diffs", Prompt: "Review the diff.",
		Agents: []string{"claude", wslClaude, wslCodex}}))
	mine, err := os.ReadFile(filepath.Join(home(), ".claude/agents/reviewer.md"))
	if err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(h, ".claude/agents/reviewer.md")); got != string(mine) {
		t.Errorf("claude@wsl reviewer.md:\n%s\nwant:\n%s", got, mine)
	}
	if got := read(t, filepath.Join(h, ".codex/agents/reviewer.toml")); !strings.Contains(got, "Review the diff.") {
		t.Errorf("codex@wsl reviewer.toml:\n%s", got)
	}
	ok(t)(SubagentAgentsSet("reviewer", []string{"claude"}))
	if _, err := os.Stat(filepath.Join(h, ".claude/agents/reviewer.md")); !os.IsNotExist(err) {
		t.Errorf("claude@wsl kept the subagent: %v", err)
	}
}
