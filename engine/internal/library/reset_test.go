package library

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A reset archives what the agent has that the library didn't give it,
// takes it out, keeps the library's own; restoring puts it all back.
func TestResetArchivesAndRestores(t *testing.T) {
	h := sandbox(t)
	write(t, filepath.Join(h, ".claude/skills/mine/SKILL.md"), "---\nname: mine\ndescription: x\n---\nhi")
	write(t, filepath.Join(h, ".claude/skills/_shared/util.md"), "shared")
	write(t, filepath.Join(h, ".claude/skills/INDEX.md"), "index")
	write(t, filepath.Join(h, ".claude.json"), `{"mcpServers":{"own":{"type":"stdio","command":"own-mcp"}}}`)
	ok(t)(SaveServer("", Server{Name: "lib", Transport: "stdio", Command: "lib-mcp", Agents: []string{"claude"}}))

	p, err := ResetPreview([]string{"claude"}, false)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	for _, it := range p.Items {
		kinds[it.Kind+":"+it.Name] = true
	}
	if !kinds["skill:mine"] || !kinds["mcp:own"] || kinds["mcp:lib"] || kinds["skill:_shared"] || kinds["skill:INDEX.md"] {
		t.Fatalf("plan: %+v", p.Items)
	}

	rec, _, err := Reset([]string{"claude"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(h, ".claude/skills/mine")); err == nil {
		t.Error("the agent's own skill is still there")
	}
	c := read(t, filepath.Join(h, ".claude.json"))
	if strings.Contains(c, "own-mcp") || !strings.Contains(c, "lib-mcp") {
		t.Errorf("after reset: %s", c)
	}
	if _, err := os.Stat(rec.Archives["windows"]); err != nil {
		t.Errorf("no archive: %v", err)
	}
	if rs, _ := Resets(); len(rs) != 1 || rs[0].ID != rec.ID {
		t.Errorf("resets: %+v", rs)
	}

	ok(t)(RestoreReset(rec.ID))
	if s := read(t, filepath.Join(h, ".claude/skills/mine/SKILL.md")); !strings.Contains(s, "hi") {
		t.Errorf("skill not back: %q", s)
	}
	if c := read(t, filepath.Join(h, ".claude.json")); !strings.Contains(c, "own-mcp") {
		t.Errorf("server not back: %s", c)
	}
}
