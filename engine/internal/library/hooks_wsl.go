package library

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// Hooks for an agent in a WSL distro (AgentDock): the same hook, in the
// same agent's own file and shape as on this computer, at the file's place
// under the distro's $HOME, opened through \\wsl.localhost. A command that
// names a Windows drive (C:\tools\notify.sh) is given as the distro names
// it (/mnt/c/tools/notify.sh); one that runs a Windows program (.exe,
// .cmd, .bat, .ps1) is refused for the distro, as a Windows MCP server is:
// its paths and variables wouldn't cross over.

// wslHookFiles are where each agent magpie gives hooks to in WSL keeps
// them, under the distro's $HOME.
var wslHookFiles = map[string]string{
	"claude": filepath.Join(".claude", "settings.json"),
	"codex":  filepath.Join(".codex", "hooks.json"),
	"gemini": filepath.Join(".gemini", "settings.json"),
	"cursor": filepath.Join(".cursor", "hooks.json"),
	"grok":   filepath.Join(".grok", "config.toml"),
}

// wslHookSpecOf is how t, an agent in a WSL distro, keeps its hooks: its
// kind's spec on this computer, its file under the distro's home.
func wslHookSpecOf(t *Target) (*hookSpec, string) {
	kind, _, _ := strings.Cut(t.Agent.ID, "@")
	rel, ok := wslHookFiles[kind]
	if !ok {
		return nil, fmt.Sprintf("magpie gives hooks to %s only on this computer, not in WSL", t.Agent.Name)
	}
	if t.Agent.Home == "" {
		return nil, fmt.Sprintf("WSL %s isn't running: %s is given its hooks once it runs", t.Agent.WSL, t.Agent.Name)
	}
	local := *t.Agent
	local.ID, local.WSL = kind, ""
	sp, why := hookSpecOf(&Target{Agent: &local})
	if sp == nil {
		return nil, why
	}
	c := *sp
	c.path, c.wsl = filepath.Join(t.Agent.Home, rel), true
	c.named = linuxHome(t.Agent.Home) + "/" + filepath.ToSlash(rel)
	return &c, ""
}

// windowsDrive is a Windows path with a drive letter, anywhere in a
// command: at its start, after a space or a quote.
var windowsDrive = regexp.MustCompile(`(^|[\s"'=])([A-Za-z]):[\\/]([^\s"']*)`)

// windowsProgram is a command word that runs a Windows program.
var windowsProgram = regexp.MustCompile(`(?i)\.(?:exe|cmd|bat|ps1)$`)

// command is h's command as the agent's file gives it.
func (sp *hookSpec) command(h *Hook) string {
	if !sp.wsl {
		return h.Command
	}
	return wslHookCommand(h.Command)
}

// runnable says why the agent can't run h's command, or nil.
func (sp *hookSpec) runnable(h *Hook) error {
	if !sp.wsl {
		return nil
	}
	if p := firstWord(h.Command); windowsProgram.MatchString(p) {
		return fmt.Errorf("it runs a Windows program (%s), which an agent in WSL can't run: give it a command WSL has", p)
	}
	return nil
}

// wslHookCommand is cmd with each Windows drive path as WSL mounts it:
// C:\tools\a b.sh stays one word only when quoted, as a shell reads it.
func wslHookCommand(cmd string) string {
	return windowsDrive.ReplaceAllStringFunc(cmd, func(m string) string {
		sub := windowsDrive.FindStringSubmatch(m)
		return sub[1] + "/mnt/" + strings.ToLower(sub[2]) + "/" + strings.ReplaceAll(sub[3], `\`, "/")
	})
}

// firstWord is the program cmd runs: its first word, unquoted.
func firstWord(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return ""
	}
	if q := cmd[0]; q == '"' || q == '\'' {
		if end := strings.IndexByte(cmd[1:], q); end >= 0 {
			return cmd[1 : 1+end]
		}
		return cmd[1:]
	}
	w, _, _ := strings.Cut(cmd, " ")
	return w
}
