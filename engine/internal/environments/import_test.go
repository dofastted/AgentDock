package environments

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/yetone/magpie/internal/provider"
)

// distroMagpie writes a WSL magpie's own providers.json, as it saves one.
func distroMagpie(t *testing.T, body string) string {
	t.Helper()
	p := magpieFileIn(t.TempDir())
	writeFile(t, p, body)
	return p
}

const distroProviders = `{
  "providers": [
    {"id": "relay", "name": "Relay", "chat": "https://relay.example.test/v1", "key": "sk-wsl", "models": ["gpt-5.5"]},
    {"id": "deep", "name": "Deep", "chat": "https://deep.example.test/v1", "key": "sk-deep", "models": ["v4"]}
  ],
  "groups": [
    {"id": "fast", "name": "Fast", "members": ["deep/v4"]},
    {"id": "mixed", "name": "Mixed", "members": ["relay/gpt-5.5", "deep/v4"]}
  ]
}`

// A provider new here comes in as it is; one here already with other
// settings comes in beside it as <id>-wsl, the one here left as it was; a
// group comes in only when its providers came in under their own ids.
// Imported again, nothing more is added.
func TestImportMagpie(t *testing.T) {
	syncSandbox(t)
	src := distroMagpie(t, distroProviders)
	before, _ := os.ReadFile(src)

	res, err := importFrom(src)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(res.Added, []string{"deep"}) || res.Renamed["relay"] != "relay-wsl" {
		t.Fatalf("result: %+v", res)
	}
	if !slices.Equal(res.Groups, []string{"fast"}) || res.Skipped["group/mixed"] == "" {
		t.Fatalf("groups: %+v", res)
	}
	stored, err := provider.Stored()
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]provider.Provider{}
	for _, p := range stored {
		by[p.ID] = p
	}
	if by["relay"].Key != "k" || by["relay-wsl"].Key != "sk-wsl" || by["deep"].Key != "sk-deep" {
		t.Fatalf("stored: %+v", stored)
	}
	if after, _ := os.ReadFile(src); string(after) != string(before) {
		t.Fatal("the distro's providers.json was written")
	}

	again, err := importFrom(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Added) != 0 || len(again.Renamed) != 0 || len(again.Groups) != 0 {
		t.Fatalf("imported again: %+v", again)
	}
	if n, _ := provider.Stored(); len(n) != len(stored) {
		t.Fatalf("%d providers after a second import, %d before", len(n), len(stored))
	}
}

// A providers.json here that can't be read is never taken as none: the
// import refuses and writes nothing over it.
func TestImportMagpieKeepsAnUnreadableFile(t *testing.T) {
	syncSandbox(t)
	bad := []byte(`{"providers": [ oops`)
	if err := os.WriteFile(provider.Path(), bad, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := importFrom(distroMagpie(t, distroProviders)); err == nil {
		t.Fatal("imported over an unreadable providers.json")
	}
	if b, _ := os.ReadFile(provider.Path()); string(b) != string(bad) {
		t.Fatalf("providers.json now %q", b)
	}
}

// The distro's own file that doesn't parse is said, and nothing is added.
func TestImportMagpieUnreadableSource(t *testing.T) {
	syncSandbox(t)
	if _, err := importFrom(distroMagpie(t, `{"providers": [`)); err == nil {
		t.Fatal("a broken source imported")
	}
	if _, err := importFrom(filepath.Join(t.TempDir(), "none.json")); err == nil {
		t.Fatal("a missing source imported")
	}
}
