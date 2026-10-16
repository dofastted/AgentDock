package library

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func nativeFixture(t *testing.T, kind string) (NativeRoot, NativeRoot) {
	t.Helper()
	sandbox(t)
	dir := t.TempDir()
	root := func(env string) NativeRoot {
		p := filepath.Join(dir, strings.ReplaceAll(env, ":", "-"), kind)
		return NativeRoot{ID: nativeDigest([]byte(p)), Environment: env, Agent: "codex@" + env, CLI: "codex", Name: "Codex", Kind: kind, Path: p, directory: true}
	}
	a, b := root("windows"), root("wsl:Debian")
	old := nativeRoots
	nativeRoots = func() []NativeRoot { return []NativeRoot{a, b} }
	t.Cleanup(func() { nativeRoots = old })
	return a, b
}

func nativePut(t *testing.T, p, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func nativeGet(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestNativeVariantsAndExternalEditConflict(t *testing.T) {
	a, b := nativeFixture(t, "skill")
	name := "review/SKILL.md"
	ap, bp := filepath.Join(a.Path, filepath.FromSlash(name)), filepath.Join(b.Path, filepath.FromSlash(name))
	nativePut(t, ap, "Windows rules\n")
	nativePut(t, bp, "Debian rules\n")
	v := ReadNative()
	if len(v.Items) != 2 || v.Items[0].ID == v.Items[1].ID {
		t.Fatalf("same-name variants collapsed: %+v", v.Items)
	}
	id := nativeItem(b, name).ID
	doc, err := ReadNativeFile(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := ChangeNativeFile(NativeChange{ID: id, Version: doc.Version, Text: "Edited from Windows\n"}); err != nil {
		t.Fatal(err)
	}
	if got := nativeGet(t, ap); got != "Windows rules\n" {
		t.Fatalf("unselected environment changed: %q", got)
	}
	if got := nativeGet(t, bp); got != "Edited from Windows\n" {
		t.Fatalf("wrong native target: %q", got)
	}
	if st, err := os.Stat(filepath.Dir(bp)); err != nil || !st.IsDir() {
		t.Fatal("native skill folder moved", err)
	}
	doc, err = ReadNativeFile(id)
	if err != nil {
		t.Fatal(err)
	}
	nativePut(t, bp, "Edited inside WSL\n")
	if err := ChangeNativeFile(NativeChange{ID: id, Version: doc.Version, Text: "stale editor\n"}); !errors.Is(err, ErrNativeConflict) {
		t.Fatalf("stale save: %v", err)
	}
	fresh, err := ReadNativeFile(id)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Text != "Edited inside WSL\n" || fresh.Version == doc.Version {
		t.Fatalf("external change not reflected: %+v", fresh)
	}
}

func TestNativeCopyPreviewProtectsBothEditors(t *testing.T) {
	a, b := nativeFixture(t, "prompt")
	name := "review.md"
	ap, bp := filepath.Join(a.Path, name), filepath.Join(b.Path, name)
	nativePut(t, ap, "Source\n")
	nativePut(t, bp, "Target\n")
	source := nativeItem(a, name).ID
	preview, err := PreviewNativeCopy(NativeCopyRequest{Source: source, Targets: []string{b.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if got := nativeGet(t, bp); got != "Target\n" {
		t.Fatal("preview wrote target")
	}
	if preview.Targets[0].Before != "Target\n" || preview.Targets[0].After != "Source\n" {
		t.Fatalf("wrong diff: %+v", preview)
	}
	request := NativeCopyApply{Source: source, SourceVersion: preview.SourceVersion, Targets: []NativeCopyTarget{{Root: b.ID, Version: preview.Targets[0].Version}}}
	nativePut(t, bp, "Target editor\n")
	result, err := ApplyNativeCopy(request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Results[0].Error == "" || nativeGet(t, bp) != "Target editor\n" {
		t.Fatal("stale preview overwrote target")
	}
	nativePut(t, ap, "New source\n")
	if _, err := ApplyNativeCopy(request); !errors.Is(err, ErrNativeConflict) {
		t.Fatalf("stale source copied: %v", err)
	}
	preview, err = PreviewNativeCopy(NativeCopyRequest{Source: source, Targets: []string{b.ID}})
	if err != nil {
		t.Fatal(err)
	}
	request.SourceVersion, request.Targets[0].Version = preview.SourceVersion, preview.Targets[0].Version
	result, err = ApplyNativeCopy(request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Results[0].Error != "" || nativeGet(t, bp) != "New source\n" || nativeGet(t, ap) != "New source\n" {
		t.Fatalf("explicit copy failed: %+v", result)
	}
}

func TestNativeCreateTraversalAndDeleteVersion(t *testing.T) {
	a, _ := nativeFixture(t, "agent")
	for _, name := range []string{"../outside.md", "/outside.md", `dir\outside.md`, ".hidden.md", "a/../../outside.md"} {
		if err := ChangeNativeFile(NativeChange{Root: a.ID, Name: name, Text: "must not write"}); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	in := NativeChange{Root: a.ID, Name: "reviewer.toml", Text: "name = 'reviewer'\n"}
	if err := ChangeNativeFile(in); err != nil {
		t.Fatal(err)
	}
	if err := ChangeNativeFile(in); !errors.Is(err, ErrNativeConflict) {
		t.Fatalf("create overwrote file: %v", err)
	}
	id := nativeItem(a, in.Name).ID
	doc, err := ReadNativeFile(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := ChangeNativeFile(NativeChange{ID: id, Version: "stale", Remove: true}); !errors.Is(err, ErrNativeConflict) {
		t.Fatalf("stale delete: %v", err)
	}
	if err := ChangeNativeFile(NativeChange{ID: id, Version: doc.Version, Remove: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(a.Path, in.Name)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("native file not removed", err)
	}
}

func TestNativeInvalidConfigDoesNotLeakOrReplace(t *testing.T) {
	a, _ := nativeFixture(t, "agent")
	p := filepath.Join(a.Path, "reviewer.toml")
	nativePut(t, p, "name = 'reviewer'\n")
	doc, err := ReadNativeFile(nativeItem(a, "reviewer.toml").ID)
	if err != nil {
		t.Fatal(err)
	}
	err = ChangeNativeFile(NativeChange{ID: doc.Item.ID, Version: doc.Version, Text: "api_key = 'private-value\n"})
	if err == nil || strings.Contains(err.Error(), "private-value") || nativeGet(t, p) != "name = 'reviewer'\n" {
		t.Fatalf("invalid config handling: %v", err)
	}
}

func TestNativeWSLScriptsPreserveSymlinksAndVersions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Linux file semantics; exercised against live WSL by packaged smoke")
	}
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("POSIX shell unavailable")
	}
	old := nativeWSLRun
	nativeWSLRun = func(_ NativeRoot, script string, input io.Reader, args ...string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, shell, append([]string{"-c", script, "sh"}, args...)...)
		cmd.Stdin = input
		return cmd.Output()
	}
	t.Cleanup(func() { nativeWSLRun = old })
	dir := t.TempDir()
	source := filepath.Join(dir, "external", "skills", "review")
	nativePut(t, filepath.Join(source, "SKILL.md"), "Native\n")
	root := filepath.Join(dir, "skills")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "review")
	if err := os.Symlink(source, link); err != nil {
		t.Fatal(err)
	}
	r := NativeRoot{Path: root, distro: "fixture", directory: true}
	names, err := nativeWSLList(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "review/SKILL.md" {
		t.Fatalf("native linked skill absent: %v", names)
	}
	p := filepath.Join(link, "SKILL.md")
	b, exists, err := nativeWSLRead(r, p)
	if err != nil || !exists {
		t.Fatal("read linked native file", err)
	}
	version := nativeDigest(b)
	if err := nativeWSLWrite(r, p, []byte("Changed\n"), version, false); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Lstat(link); err != nil || st.Mode()&os.ModeSymlink == 0 {
		t.Fatal("native skill link replaced", err)
	}
	if nativeGet(t, filepath.Join(source, "SKILL.md")) != "Changed\n" {
		t.Fatal("symlink destination did not change")
	}
	if err := nativeWSLWrite(r, p, []byte("Stale\n"), version, false); !errors.Is(err, ErrNativeConflict) {
		t.Fatalf("native stale write: %v", err)
	}
	if err := nativeWSLWrite(r, p, nil, nativeDigest([]byte("Changed\n")), true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(source, "SKILL.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("file remained after explicit native removal")
	}
}

func TestNativeInventoryKeepsEveryEnvironment(t *testing.T) {
	a, b := nativeFixture(t, "skill")
	for i := range 100 {
		nativePut(t, filepath.Join(a.Path, "bulk", fmt.Sprintf("ref-%03d.md", i)), "windows\n")
	}
	nativePut(t, filepath.Join(a.Path, "keep", "SKILL.md"), "windows skill\n")
	nativePut(t, filepath.Join(b.Path, "debian", "SKILL.md"), "debian skill\n")
	v := ReadNative()
	var sawWin, sawDebian bool
	for _, it := range v.Items {
		if it.Name == "keep/SKILL.md" && it.Environment == a.Environment {
			sawWin = true
		}
		if it.Name == "debian/SKILL.md" && it.Environment == b.Environment {
			sawDebian = true
		}
	}
	if !sawWin || !sawDebian {
		t.Fatalf("inventory hid an environment: windows=%v debian=%v items=%d", sawWin, sawDebian, len(v.Items))
	}
}
