package library

// AgentDock: one library server, a command for each environment
// (.trellis/tasks/11-12-unified-env-config). A server whose program is
// uvx.exe on Windows and uvx in WSL, or a Linux binary only WSL has, is
// one entry: Variants holds what differs where. The agent's own id is the
// most specific key (grok@wsl:Debian, as agents.yaml's override.<cli>),
// then its distro (wsl:Debian), then any distro (wsl), then this machine
// (windows). The first key the server has wins whole; the server's own
// command is the fallback.

import (
	"fmt"
	"maps"
	"runtime"
	"slices"
	"strings"

	"github.com/yetone/magpie/internal/agent"
	"github.com/yetone/magpie/internal/lazyre"
)

// ServerVariant is what a server runs in one environment. A command
// replaces the server's command and arguments together; Args alone
// replaces only the arguments; Env, when set, replaces the variables.
type ServerVariant struct {
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// Variant keys that name an environment rather than an agent.
const (
	VariantWindows = "windows"
	VariantWSL     = "wsl"
)

// variantKeys are the keys looked for, most specific first, for agent id
// in WSL distro (distro "" for this machine).
func variantKeys(id, distro string, wsl bool) []string {
	var keys []string
	if id != "" {
		keys = append(keys, id)
	}
	if wsl {
		if distro != "" {
			keys = append(keys, "wsl:"+distro)
		}
		return append(keys, VariantWSL)
	}
	if runtime.GOOS == "windows" {
		keys = append(keys, VariantWindows)
	}
	return keys
}

// variantFor is s as given where keys name, with no variants left on it:
// s itself when it has none.
func (s *Server) variantFor(keys ...string) *Server {
	if len(s.Variants) == 0 {
		return s
	}
	c := *s
	c.Variants = nil
	if s.Remote() {
		return &c
	}
	for _, k := range keys {
		v, ok := s.Variants[k]
		if !ok {
			continue
		}
		if v.Command != "" {
			c.Command, c.Args = v.Command, slices.Clone(v.Args)
		} else if v.Args != nil {
			c.Args = slices.Clone(v.Args)
		}
		if v.Env != nil {
			c.Env = maps.Clone(v.Env)
		}
		break
	}
	return &c
}

// variantHit is s as keys name it and the key that named it, "" for the
// server's own command.
func (s *Server) variantHit(keys ...string) (*Server, string) {
	for _, k := range keys {
		if _, ok := s.Variants[k]; ok {
			return s.variantFor(k), k
		}
	}
	return s.variantFor(), ""
}

// resolve is s as the file's agent is given it (variantFor).
func (f *mcpFile) resolve(s *Server) *Server {
	return s.variantFor(variantKeys(f.Agent, f.Distro, f.WSL)...)
}

// resolveWith is resolve with the key whose variant was taken, "" when
// the server's own command is what the agent is given.
func (f *mcpFile) resolveWith(s *Server) (*Server, string) {
	return s.variantHit(variantKeys(f.Agent, f.Distro, f.WSL)...)
}

// linuxPath is a program named by a Linux path: /home/me/bin/x, ~/bin/x.
var linuxPath = lazyre.New(`^(?:/[^/]|~/)`)

// errLinuxOnWindows says why an agent on Windows can't start s.
func errLinuxOnWindows(s *Server) error {
	return fmt.Errorf("it runs a Linux program (%s), which an agent on Windows can't start: give it a Windows command in the server's windows variant", s.Command)
}

// linuxOnlyFor is whether s, as agent id would be given it, is a Linux
// program an agent on this Windows machine can't start: the baseline's
// tick is not copied onto such a follower (bindings.go mirror).
func linuxOnlyFor(s *Server, id string) bool {
	if runtime.GOOS != "windows" || strings.Contains(id, "@wsl:") || s.Remote() {
		return false
	}
	cmd := s.variantFor(variantKeys(id, "", false)...)
	return !isSelf(cmd) && linuxPath.MatchString(cmd.Command)
}

// crossEnv is whether s is one entry for several environments: it has a
// variant, or an agent in WSL has it. Only such a server's command is
// held to one environment; a command typed for this machine's agents
// alone is written as typed, as it always was (an absolute POSIX path is
// a program on macOS and Linux, where magpie runs too).
func crossEnv(s *Server) bool {
	if len(s.Variants) > 0 {
		return true
	}
	return slices.ContainsFunc(s.Agents, func(id string) bool { return strings.Contains(id, "@wsl:") })
}

// linuxOnWindows is whether cmd, s as resolved for an agent on this
// Windows machine, names a Linux program while s is shared with WSL
// (crossEnv): the distro's command would go into a Windows agent's
// config, where it can't start (AgentDock B1). Its windows variant is the
// fix. magpie's own server is left alone, as side does on the WSL side.
func linuxOnWindows(f *mcpFile, s, cmd *Server) bool {
	return runtime.GOOS == "windows" && !f.WSL && !cmd.Remote() && !isSelf(cmd) &&
		crossEnv(s) && linuxPath.MatchString(cmd.Command)
}

// checkVariants tidies s's variants: no empty key or variant, none on a
// server by URL, and only keys that name an environment or an agent.
func (s *Server) checkVariants() error {
	if s.Remote() {
		s.Variants = nil
		return nil
	}
	for k, v := range s.Variants {
		key := strings.TrimSpace(k)
		v.Command = strings.TrimSpace(v.Command)
		for e := range v.Env {
			if strings.TrimSpace(e) == "" {
				delete(v.Env, e)
			}
		}
		delete(s.Variants, k)
		if key == "" || v.Command == "" && v.Args == nil && v.Env == nil {
			continue
		}
		if !validVariantKey(key) {
			return fmt.Errorf("%s: variant %q names no environment (windows, wsl, wsl:<distro>) or agent", s.Name, key)
		}
		if v.Command != "" {
			v.Command, v.Args = spacedProgram(v.Command, v.Args)
		}
		s.Variants[key] = v
	}
	if len(s.Variants) == 0 {
		s.Variants = nil
	}
	return nil
}

// validVariantKey: windows, wsl, wsl:<distro>, or an agent id (claude,
// omp#work, codex@wsl:Debian).
var variantKeyRe = lazyre.New(`^(?:windows|wsl|wsl:[^\s:@]+|[a-z0-9][a-z0-9._#-]*(?:@wsl:[^\s:@]+)?)$`)

func validVariantKey(k string) bool { return variantKeyRe.MatchString(k) }

// variantKeyOf is the key of an agent's environment: wsl:<distro> for one
// in a distro, windows for one on this machine. "" where an environment
// isn't a variant's to name (magpie on macOS or Linux, with no WSL).
func variantKeyOf(a *agent.Agent) string {
	switch {
	case a == nil:
		return ""
	case a.WSL != "":
		return "wsl:" + a.WSL
	case runtime.GOOS == "windows":
		return VariantWindows
	}
	return ""
}

// foldVariant takes s, a server an agent has under the name f already
// holds but started another way, as f's variant for that agent's
// environment: Windows' uvx.exe and the distro's uvx are one entry, as
// they are one server (AgentDock, foundServersOf). base is the key of the
// environment f's own command came from. A second way of starting it in
// the same environment is a conflict the user settles, not a variant.
func foldVariant(f *Found, base string, s *Server, a *agent.Agent) bool {
	k := variantKeyOf(a)
	if k == "" || k == base || f.Server.Remote() || s.Remote() || s.Command == "" {
		return false
	}
	if _, taken := f.Server.Variants[k]; taken {
		return false
	}
	v := ServerVariant{Command: s.Command, Args: slices.Clone(s.Args)}
	if !maps.Equal(s.Env, f.Server.Env) {
		v.Env = maps.Clone(s.Env)
	}
	if f.Server.Variants == nil {
		f.Server.Variants = map[string]ServerVariant{}
	}
	f.Server.Variants[k] = v
	return true
}
