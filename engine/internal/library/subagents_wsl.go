package library

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Subagents for an agent in a WSL distro (AgentDock): the same file as on
// this computer, in the agent's folder under the distro's $HOME, opened
// through \\wsl.localhost. A subagent names nothing on Windows, so it is
// written as it is.

// wslSubagentDirs are the folders under the distro's $HOME of the agents
// that keep their subagents beside their home there.
var wslSubagentDirs = map[string]string{
	"claude":  filepath.Join(".claude", "agents"),
	"codex":   filepath.Join(".codex", "agents"),
	"gemini":  filepath.Join(".gemini", "agents"),
	"cursor":  filepath.Join(".cursor", "agents"),
	"copilot": filepath.Join(".copilot", "agents"),
}

// wslSubagentSpecOf is how t, an agent in a WSL distro, keeps subagents:
// its kind's spec on this computer, in its folder in the distro. An agent
// that keeps them in its own folder (OpenCode, Qwen Code, Droid, Grok) has
// that folder in the distro already.
func wslSubagentSpecOf(t *Target) (*subagentSpec, string) {
	a := t.Agent
	if a.Home == "" {
		return nil, fmt.Sprintf("WSL %s isn't running: %s is given its subagents once it runs", a.WSL, a.Name)
	}
	kind, _, _ := strings.Cut(a.ID, "@")
	local := *a
	local.ID, local.WSL = kind, ""
	sp, why := subagentSpecOf(&Target{Agent: &local})
	if sp == nil {
		return nil, why
	}
	c := *sp
	if rel, ok := wslSubagentDirs[kind]; ok {
		c.dir = filepath.Join(a.Home, rel)
		return &c, ""
	}
	if !under(c.dir, a.Home) {
		return nil, fmt.Sprintf("magpie gives subagents to %s only on this computer, not in WSL", a.Name)
	}
	return &c, ""
}

// under is whether path is in dir, Windows' case aside.
func under(path, dir string) bool {
	rel, err := filepath.Rel(strings.ToLower(dir), strings.ToLower(path))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
