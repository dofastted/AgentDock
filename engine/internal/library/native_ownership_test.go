package library

import (
	"path/filepath"
	"strings"
	"testing"
)

func nativeOnlyRoot(t *testing.T, r NativeRoot) {
	t.Helper()
	old := nativeRoots
	nativeRoots = func() []NativeRoot { return []NativeRoot{r} }
	t.Cleanup(func() { nativeRoots = old })
}

func TestNativeInstructionEditSurvivesLaterLibraryChange(t *testing.T) {
	h := sandbox(t)
	shared := "Original centrally managed rules."
	if _, err := SaveInstructions(InstructionsChange{Shared: &shared, Agents: []string{"claude", "codex"}}); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(h, ".claude", "CLAUDE.md")
	r := NativeRoot{ID: nativeDigest([]byte(p)), Environment: "windows", Agent: "claude", CLI: "claude", Name: "Claude", Kind: "instructions", Path: p}
	nativeOnlyRoot(t, r)
	doc, err := ReadNativeFile(nativeItem(r, filepath.Base(p)).ID)
	if err != nil {
		t.Fatal(err)
	}
	local := strings.ReplaceAll(doc.Text, shared, "Native WSL-editable rules.")
	if err := ChangeNativeFile(NativeChange{ID: doc.Item.ID, Version: doc.Version, Text: local}); err != nil {
		t.Fatal(err)
	}
	shared = "New rules for the remaining central target."
	if _, err := SaveInstructions(InstructionsChange{Shared: &shared, Agents: []string{"codex"}}); err != nil {
		t.Fatal(err)
	}
	got := nativeGet(t, p)
	if !strings.Contains(got, "Native WSL-editable rules.") || strings.Contains(got, blockBegin) || strings.Contains(got, shared) {
		t.Fatalf("native ownership lost: %s", got)
	}
	if got := nativeGet(t, filepath.Join(h, ".codex", "AGENTS.md")); !strings.Contains(got, shared) {
		t.Fatal("unrelated central target was detached")
	}
}

func TestNativeMCPEditReleasesOnlyChangedServer(t *testing.T) {
	h := sandbox(t)
	for _, s := range []Server{
		{Name: "first", Transport: "stdio", Command: "first-command", Agents: []string{"claude"}},
		{Name: "second", Transport: "stdio", Command: "second-command", Agents: []string{"claude"}},
	} {
		if _, err := SaveServer("", s); err != nil {
			t.Fatal(err)
		}
	}
	p := filepath.Join(h, ".claude.json")
	r := NativeRoot{ID: nativeDigest([]byte(p)), Environment: "windows", Agent: "claude", CLI: "claude", Name: "Claude", Kind: "mcp", Path: p, mcp: &mcpFile{Path: p, Format: fmtClaude}}
	nativeOnlyRoot(t, r)
	doc, err := ReadNativeFile(nativeItem(r, filepath.Base(p)).ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := ChangeNativeFile(NativeChange{ID: doc.Item.ID, Version: doc.Version, Text: strings.ReplaceAll(doc.Text, "first-command", "native-command")}); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveServer("second", Server{Name: "second", Transport: "stdio", Command: "updated-second", Agents: []string{"claude"}}); err != nil {
		t.Fatal(err)
	}
	got := nativeGet(t, p)
	if !strings.Contains(got, "native-command") || !strings.Contains(got, "updated-second") || strings.Contains(got, "first-command") {
		t.Fatalf("native edit or unrelated MCP binding lost: %s", got)
	}
}
