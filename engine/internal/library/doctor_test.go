package library

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

// A tidy-up finds a server ticked for an agent that can't start it and a
// skills link to nothing, archives them, fixes both, and a restore puts
// them back.
func TestDoctorFixesAndRestores(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the Linux-program-on-Windows rule is Windows' own")
	}
	h := sandbox(t)
	SaveServer("", Server{Name: "linuxonly", Transport: "stdio", Command: "/home/me/bin/x-mcp",
		Variants: map[string]ServerVariant{"wsl": {Command: "x-mcp"}}, Agents: []string{"claude"}})
	gone := filepath.Join(h, "gone-skill")
	write(t, filepath.Join(gone, "SKILL.md"), "---\nname: gone\ndescription: x\n---\n")
	link := filepath.Join(h, ".claude", "skills", "gone")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := dirLink(gone, link); err != nil {
		t.Skip("no link here: ", err)
	}
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}

	p, err := DoctorPreview()
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	for _, it := range p.Items {
		kinds[it.Kind+":"+it.Name] = true
	}
	if !kinds[doctorMCPEnv+":linuxonly"] || !kinds[doctorLink+":gone"] {
		t.Fatalf("plan: %+v", p.Items)
	}

	rec, _, err := Doctor()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(link); err == nil {
		t.Error("the link to nothing is still there")
	}
	l, _ := load()
	if s := l.server("linuxonly"); s == nil || slices.Contains(s.Agents, "claude") {
		t.Errorf("server after tidy-up: %+v", s)
	}
	if p, _ := DoctorPreview(); len(p.Items) != 0 {
		t.Errorf("left after tidy-up: %+v", p.Items)
	}
	if _, _, err := Doctor(); err != errNothingToTidy {
		t.Errorf("second tidy-up: %v", err)
	}

	if _, err := RestoreReset(rec.ID); err != nil {
		t.Logf("restore: %v", err)
	}
	if _, err := os.Lstat(link); err != nil {
		t.Errorf("link not back: %v", err)
	}
	l, _ = load()
	if s := l.server("linuxonly"); s == nil || !slices.Contains(s.Agents, "claude") {
		t.Errorf("tick not back: %+v", s)
	}
}
