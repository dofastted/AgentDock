package environments

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/yetone/magpie/internal/agent"
	"github.com/yetone/magpie/internal/library"
)

// Cross-environment sync (G5): one agent's setup — whether it goes through
// magpie, its fields (model, effort …) and what the library gives it —
// copied onto the same agent in other environments (claude onto
// claude@wsl:Debian). A preview says what would change and writes nothing;
// an apply makes the same changes the user could make row by row. Sign-ins
// and accounts aren't copied: they are each environment's own.

// skippedFields are fields never copied: a sign-in or an account is the
// environment's own.
var skippedFields = map[string]bool{"login": true, "account": true}

// Change is one thing an apply changes on a target.
type Change struct {
	What string `json:"what"` // "connect", "disconnect", "field", "library"
	Key  string `json:"key,omitempty"`
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
}

// TargetPlan is what an apply would do to one target, or why it can't.
type TargetPlan struct {
	Agent   string   `json:"agent"`
	Skip    string   `json:"skip,omitempty"`
	Changes []Change `json:"changes"`
	// Error is what went wrong applying, after the changes before it
	Error string `json:"error,omitempty"`
}

// Plan is a preview, or what an apply did.
type Plan struct {
	Source  string       `json:"source"`
	Targets []TargetPlan `json:"targets"`
	// Problems are what the library couldn't give a target
	Problems []library.Problem `json:"problems,omitempty"`
}

// kindOf is an agent id's kind: claude for claude@wsl:Debian.
func kindOf(id string) string { k, _, _ := strings.Cut(id, "@"); return k }

// findAgent finds an agent by its exact id, not a prefix or alias of one.
func findAgent(id string) (*agent.Agent, error) {
	a, err := agent.Find(id)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(a.ID, id) {
		return nil, fmt.Errorf("no agent %s", id)
	}
	return a, nil
}

// fieldValues are a's copyable fields' values, by key.
func fieldValues(a *agent.Agent) map[string]string {
	out := map[string]string{}
	for _, f := range a.Fields {
		if skippedFields[f.Key] || f.Get == nil || f.Set == nil {
			continue
		}
		out[f.Key] = f.Get()
	}
	return out
}

// Preview is what Apply(source, targets) would change; it writes nothing.
func Preview(source string, targets []string) (Plan, error) {
	p, _, err := plan(source, targets)
	return p, err
}

func plan(source string, targets []string) (Plan, map[string]*agent.Agent, error) {
	src, err := findAgent(source)
	if err != nil {
		return Plan{}, nil, err
	}
	if src.WSL != "" && !agent.WSLRunning(src.WSL) {
		return Plan{}, nil, fmt.Errorf("WSL %s isn't running: start it to copy %s from it", src.WSL, src.Name)
	}
	srcWired, srcVals := src.Wired(), fieldValues(src)
	srcLib, err := library.AgentBindings(src.ID)
	if err != nil {
		return Plan{}, nil, err
	}
	out := Plan{Source: src.ID, Targets: []TargetPlan{}}
	found := map[string]*agent.Agent{}
	seen := map[string]bool{}
	for _, id := range targets {
		if seen[strings.ToLower(id)] {
			continue
		}
		seen[strings.ToLower(id)] = true
		tp := TargetPlan{Agent: id, Changes: []Change{}}
		t, err := findAgent(id)
		switch {
		case err != nil:
			tp.Skip = err.Error()
		case t.ID == src.ID:
			tp.Skip = "it is the agent copied from"
		case kindOf(t.ID) != kindOf(src.ID):
			tp.Skip = fmt.Sprintf("%s is another agent than %s", t.Name, src.Name)
		case t.WSL != "" && !agent.WSLRunning(t.WSL):
			tp.Skip = "WSL " + t.WSL + " isn't running: writing its files would start it"
		case !t.Detected():
			tp.Skip = t.Name + " isn't installed there"
		}
		if tp.Skip != "" {
			out.Targets = append(out.Targets, tp)
			continue
		}
		tp.Agent = t.ID
		found[t.ID] = t
		wired := t.Wired()
		switch {
		case srcWired && !wired:
			tp.Changes = append(tp.Changes, Change{What: "connect"})
		case !srcWired && wired:
			tp.Changes = append(tp.Changes, Change{What: "disconnect"})
		}
		// the fields are copied for an agent through magpie; one that
		// isn't keeps its own config, which disconnecting puts back
		if srcWired {
			tv := fieldValues(t)
			keys := make([]string, 0, len(srcVals))
			for k := range srcVals {
				keys = append(keys, k)
			}
			slices.Sort(keys)
			for _, k := range keys {
				if t.Field(k) == nil {
					continue
				}
				if tv[k] != srcVals[k] {
					tp.Changes = append(tp.Changes, Change{What: "field", Key: k, From: tv[k], To: srcVals[k]})
				}
			}
		}
		tl, err := library.AgentBindings(t.ID)
		if err != nil {
			return Plan{}, nil, err
		}
		tp.Changes = append(tp.Changes, libraryChanges(srcLib, tl)...)
		out.Targets = append(out.Targets, tp)
	}
	return out, found, nil
}

// libraryChanges are what mirroring src's bindings onto t's changes.
func libraryChanges(src, t library.Bindings) []Change {
	var out []Change
	if src.Instructions != t.Instructions {
		out = append(out, onOff("instructions", "", src.Instructions))
	}
	for _, k := range []struct {
		kind     string
		src, has []string
	}{{"mcp", src.MCP, t.MCP}, {"skill", src.Skills, t.Skills}, {"hook", src.Hooks, t.Hooks}, {"subagent", src.Subagents, t.Subagents}} {
		for _, n := range k.src {
			if !slices.Contains(k.has, n) {
				out = append(out, onOff(k.kind, n, true))
			}
		}
		for _, n := range k.has {
			if !slices.Contains(k.src, n) {
				out = append(out, onOff(k.kind, n, false))
			}
		}
	}
	return out
}

func onOff(kind, name string, on bool) Change {
	c := Change{What: "library", Key: kind, To: "off"}
	if name != "" {
		c.Key = kind + ":" + name
	}
	if on {
		c.To = "on"
	}
	return c
}

// Apply makes the changes Preview(source, targets) says, target by target:
// connected or disconnected first, then each field picked, then the
// library's bindings mirrored onto every target it could be applied to.
func Apply(source string, targets []string) (Plan, error) {
	p, found, err := plan(source, targets)
	if err != nil {
		return Plan{}, err
	}
	var mirrored []string
	for i := range p.Targets {
		tp := &p.Targets[i]
		t := found[tp.Agent]
		if tp.Skip != "" || t == nil {
			continue
		}
		var errs []error
		lib := false
	changes:
		for _, c := range tp.Changes {
			var err error
			switch c.What {
			case "connect":
				err = t.Connect()
			case "disconnect":
				err = t.Disconnect()
			case "field":
				err = t.Pick(c.Key, c.To)
			case "library":
				lib = true
			}
			if err != nil {
				errs = append(errs, fmt.Errorf("%s %s: %w", c.What, c.Key, err))
				if c.What == "connect" {
					break changes // its fields would go to its own config, not magpie's
				}
			}
		}
		if err := errors.Join(errs...); err != nil {
			tp.Error = err.Error()
			continue
		}
		if lib {
			mirrored = append(mirrored, t.ID)
		}
	}
	if len(mirrored) > 0 {
		res, err := library.MirrorBindings(p.Source, mirrored)
		if err != nil {
			return p, err
		}
		if res != nil {
			p.Problems = res.Problems
		}
	}
	return p, nil
}
