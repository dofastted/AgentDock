package library

import (
	"bytes"
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
)

// nativeRelinquish drops deployment bookkeeping, not native files. Calling
// change() here would synchronize every target as a side effect of a local edit.
// The caller holds mu until the native write and metadata save have completed.
func nativeRelinquish(l *Library, r NativeRoot, p string, old, next []byte, deleting bool) ([]byte, bool, error) {
	ids := []string{r.Agent}
	for _, other := range nativeRoots() {
		if other.Kind == r.Kind && other.Environment == r.Environment && other.Path == r.Path && !slices.Contains(ids, other.Agent) {
			ids = append(ids, other.Agent)
		}
	}
	var mcpChanged []string
	if r.Kind == "mcp" && r.mcp != nil {
		before, err := r.mcp.parseEntries(old)
		if err != nil {
			return next, false, errors.New("existing MCP configuration cannot be parsed safely")
		}
		after := map[string]map[string]any{}
		if !deleting {
			after, err = r.mcp.parseEntries(next)
			if err != nil {
				return next, false, errors.New("new MCP configuration cannot be parsed safely")
			}
		}
		for name, entry := range before {
			if !reflect.DeepEqual(entry, after[name]) {
				mcpChanged = append(mcpChanged, name)
			}
		}
		for name := range after {
			if _, existed := before[name]; !existed {
				mcpChanged = append(mcpChanged, name)
			}
		}
	}
	instructionChanged := false
	if r.Kind == "instructions" && !strings.Contains(strings.ToLower(filepath.Base(p)), ".override.") {
		_, before, _, had := split(string(old))
		_, after, _, has := split(string(next))
		instructionChanged = deleting || had != has || before != after
		if instructionChanged {
			// Retain the text, but remove comments granting the library ownership.
			next = bytes.ReplaceAll(bytes.ReplaceAll(next, []byte(blockBegin), nil), []byte(blockEnd), nil)
		}
	}
	changed := false
	remove := func(list []string, id string) []string {
		if slices.Contains(list, id) {
			changed = true
			return set(list, id, false)
		}
		return list
	}
	for _, id := range ids {
		a := l.Applied[id]
		switch r.Kind {
		case "instructions":
			if !instructionChanged {
				continue
			}
			l.Instructions.Agents = remove(l.Instructions.Agents, id)
			if a != nil && (a.Instructions || a.InstrHash != "") {
				a.Instructions, a.InstrHash = false, ""
				changed = true
			}
		case "skill":
			if !deleting && bytes.Equal(old, next) {
				continue
			}
			rel, err := filepath.Rel(r.Path, p)
			if err != nil {
				continue
			}
			name, _, _ := strings.Cut(filepath.ToSlash(rel), "/")
			if s := l.skill(name); s != nil {
				s.Agents = remove(s.Agents, id)
			}
			if a != nil {
				a.Skills = remove(a.Skills, name)
			}
		case "mcp":
			// Release only edited MCP entries; unrelated bindings and settings stay.
			for _, name := range mcpChanged {
				for _, server := range l.MCP {
					if server.Name == name {
						server.Agents = remove(server.Agents, id)
					}
				}
				if a != nil {
					a.MCP = remove(a.MCP, name)
					if _, ok := a.ToolsOff[name]; ok {
						delete(a.ToolsOff, name)
						changed = true
					}
				}
			}
		}
	}
	return next, changed, nil
}
