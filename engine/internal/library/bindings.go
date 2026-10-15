package library

import "slices"

// An agent's bindings are what the library gives it: the shared
// instructions, and each server, skill, hook and subagent ticked for it.
// AgentDock copies one agent's onto its twins in other environments
// (claude onto claude@wsl:Debian), as the user would tick them one by one.

// Bindings are, by kind, what the library gives one agent.
type Bindings struct {
	Instructions bool     `json:"instructions"`
	MCP          []string `json:"mcp"`
	Skills       []string `json:"skills"`
	Hooks        []string `json:"hooks"`
	Subagents    []string `json:"subagents"`
}

// AgentBindings are what the library gives agent id now.
func AgentBindings(id string) (Bindings, error) {
	mu.Lock()
	defer mu.Unlock()
	l, err := load()
	if err != nil {
		return Bindings{}, err
	}
	return l.bindingsOf(id), nil
}

func (l *Library) bindingsOf(id string) Bindings {
	b := Bindings{Instructions: slices.Contains(l.Instructions.Agents, id), MCP: []string{}, Skills: []string{}, Hooks: []string{}, Subagents: []string{}}
	for _, s := range l.MCP {
		if slices.Contains(s.Agents, id) {
			b.MCP = append(b.MCP, s.Name)
		}
	}
	for _, s := range l.Skills {
		if slices.Contains(s.Agents, id) {
			b.Skills = append(b.Skills, s.Name)
		}
	}
	for _, h := range l.Hooks {
		if slices.Contains(h.Agents, id) {
			b.Hooks = append(b.Hooks, h.Name)
		}
	}
	for _, s := range l.Subagents {
		if slices.Contains(s.Agents, id) {
			b.Subagents = append(b.Subagents, s.Name)
		}
	}
	return b
}

// MirrorBindings gives each of targets exactly what the library gives
// from: each item from's is ticked for a target, and every other one
// unticked, then written into the agents as any change is. What a target
// can't take is said in the result, as when the user ticks it.
func MirrorBindings(from string, targets []string) (*Result, error) {
	return change(func(l *Library) error {
		for _, to := range targets {
			if to == from {
				continue
			}
			l.Instructions.Agents = set(l.Instructions.Agents, to, slices.Contains(l.Instructions.Agents, from))
			for _, s := range l.MCP {
				s.Agents = set(s.Agents, to, slices.Contains(s.Agents, from))
			}
			for _, s := range l.Skills {
				s.Agents = set(s.Agents, to, slices.Contains(s.Agents, from))
			}
			for _, h := range l.Hooks {
				h.Agents = orNone(set(h.Agents, to, slices.Contains(h.Agents, from)))
			}
			for _, s := range l.Subagents {
				s.Agents = orNone(set(s.Agents, to, slices.Contains(s.Agents, from)))
			}
		}
		return nil
	})
}
