package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeKeychain is the system keychain for a test: no test reaches the
// user's own (isolate puts an empty one in).
type fakeKeychain struct {
	mu           sync.Mutex
	items        map[string][]byte // by account
	findErr      error             // every Find answers it
	putErr       error
	finds, puts  int
	removes      int
	findAccounts []string
}

func (k *fakeKeychain) Find(service, account string) ([]byte, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.finds++
	k.findAccounts = append(k.findAccounts, account)
	if service != codexKeychainService {
		return nil, errors.New("not Codex's service: " + service)
	}
	if k.findErr != nil {
		return nil, k.findErr
	}
	b, ok := k.items[account]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return append([]byte(nil), b...), nil
}

func (k *fakeKeychain) Put(service, account string, secret []byte) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.puts++
	if k.putErr != nil {
		return k.putErr
	}
	if k.items == nil {
		k.items = map[string][]byte{}
	}
	k.items[account] = append([]byte(nil), secret...)
	return nil
}

func (k *fakeKeychain) Remove(service, account string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.removes++
	delete(k.items, account)
	return nil
}

func (k *fakeKeychain) item(account string) []byte {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.items[account]
}

// codexInKeychain moves signIn's auth.json into a fake keychain item, as
// Codex 0.162 does with cli_auth_credentials_store = store: the item on
// one line, with a field magpie doesn't know, and auth.json gone.
func codexInKeychain(t *testing.T, home, store string) (*fakeKeychain, string) {
	t.Helper()
	dir := filepath.Join(home, ".codex")
	b, err := os.ReadFile(filepath.Join(dir, "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	json.Unmarshal(b, &m)
	m["agent_identity"] = map[string]any{"kept": "as is"}
	one, _ := json.Marshal(m)
	os.Remove(filepath.Join(dir, "auth.json"))
	os.WriteFile(filepath.Join(dir, "config.toml"), []byte("cli_auth_credentials_store = \""+store+"\"\n"), 0o600)
	k := &fakeKeychain{items: map[string][]byte{codexKeychainAccount(dir): one}}
	codexKeychain, codexKeychainHere = k, true // isolate puts them back
	forgetCodexKeychain()
	return k, codexKeychainAccount(dir)
}

// Codex's account key is its compute_store_key: SHA-256 of the canonical
// CODEX_HOME, its first 16 hex digits after "cli|". A home reached
// through a link is the same item.
func TestCodexKeychainAccountIsCodexs(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "codex-home")
	if err := os.Symlink(real, link); err != nil {
		t.Skip(err)
	}
	canon, _ := filepath.EvalSymlinks(real)
	sum := sha256.Sum256([]byte(canon))
	want := "cli|" + hex.EncodeToString(sum[:])[:16]
	if got := codexKeychainAccount(real); got != want {
		t.Fatalf("account %q, want %q", got, want)
	}
	if got := codexKeychainAccount(link); got != want {
		t.Fatalf("through a link: %q, want %q", got, want)
	}
	// a home not there is hashed as given, as Codex does
	gone := filepath.Join(real, "nothing-here")
	sum = sha256.Sum256([]byte(gone))
	if got := codexKeychainAccount(gone); got != "cli|"+hex.EncodeToString(sum[:])[:16] {
		t.Fatalf("a home not there: %q", got)
	}
}

// #2367 (Dony2050): Codex set to keep its sign-in in the keychain has no
// auth.json. Its account is found there all the same: the provider, its
// saved login, the session identity and the agent's wiring all go by it.
func TestCodexSignedInInKeychain(t *testing.T) {
	for _, store := range []string{"keyring", "auto"} {
		t.Run(store, func(t *testing.T) {
			home := signIn(t)
			k, account := codexInKeychain(t, home, store)
			dir := filepath.Join(home, ".codex")
			p, ok := codexAccount(home)
			if !ok || p.Account.User != "me@example.com" {
				t.Fatalf("no Codex provider from the keychain item: %v %+v", ok, p.Account)
			}
			if l, ok := liveLogin("codex"); !ok || l.User != "me@example.com" || !strings.Contains(string(l.Auth), "agent_identity") {
				t.Fatalf("live login: %v %+v", ok, l)
			}
			if b, ok := CodexSignIn(dir); !ok || !strings.Contains(string(b), "access_token") {
				t.Fatalf("CodexSignIn: %v %s", ok, b)
			}
			ids := currentSessionIdentities(dir)
			if len(ids) == 0 || ids[0].Agent != "codex" || ids[0].User != "me@example.com" {
				t.Fatalf("session identities %+v", ids)
			}
			if k.findAccounts[0] != account {
				t.Fatalf("asked the keychain for %q, want %q", k.findAccounts[0], account)
			}
			if _, err := os.Stat(filepath.Join(dir, "auth.json")); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("auth.json written by reading: %v", err)
			}
		})
	}
}

// auto with no item is auth.json, as Codex falls back to it; keyring with
// no item is signed out, whatever auth.json is left.
func TestCodexKeychainFallback(t *testing.T) {
	home := signIn(t)
	dir := filepath.Join(home, ".codex")
	codexKeychain, codexKeychainHere = &fakeKeychain{}, true
	os.WriteFile(filepath.Join(dir, "config.toml"), []byte("cli_auth_credentials_store = \"auto\"\n"), 0o600)
	if _, ok := codexAccount(home); !ok {
		t.Fatal("auto, no item: auth.json wasn't read")
	}
	os.WriteFile(filepath.Join(dir, "config.toml"), []byte("cli_auth_credentials_store = \"keyring\"\n"), 0o600)
	forgetCodexKeychain()
	if _, ok := codexAccount(home); ok {
		t.Fatal("keyring, no item: auth.json left beside it was taken for the sign-in")
	}
	// auto, the keychain refusing: Codex reads the file then too
	codexKeychain = &fakeKeychain{findErr: errKeychainUnread}
	os.WriteFile(filepath.Join(dir, "config.toml"), []byte("cli_auth_credentials_store = \"auto\"\n"), 0o600)
	forgetCodexKeychain()
	if _, ok := codexAccount(home); !ok {
		t.Fatal("auto, keychain refused: auth.json wasn't read")
	}
}

// A refresh's rotated tokens go back into the item they came from, on one
// line, every other field kept; no auth.json is made.
func TestCodexKeychainRefreshWritesItem(t *testing.T) {
	home := signIn(t)
	dir := filepath.Join(home, ".codex")
	k, account := codexInKeychain(t, home, "keyring")
	var m map[string]any
	json.Unmarshal(k.item(account), &m)
	toks := m["tokens"].(map[string]any)
	toks["access_token"] = fakeJWT(map[string]any{"exp": float64(time.Now().Add(time.Minute).Unix())})
	toks["refresh_token"] = "r-work"
	b, _ := json.Marshal(m)
	k.items[account] = b
	fakeCodexRefresh(t)
	if tok, _, err := codexToken(context.Background(), dir); err != nil || tok == "" {
		t.Fatalf("codexToken: %q %v", tok, err)
	}
	got := k.item(account)
	if strings.Contains(string(got), "\n") || !strings.Contains(string(got), `"r-work-2"`) || !strings.Contains(string(got), `"agent_identity":{"kept":"as is"}`) {
		t.Fatalf("item after the refresh: %s", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "auth.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the refresh wrote auth.json: %v", err)
	}
	// the next read is the rotated one, not what was kept from before
	if tok, _, err := codexToken(context.Background(), dir); err != nil || tok == "" || k.puts != 1 {
		t.Fatalf("again: %q %v, %d puts", tok, err, k.puts)
	}
}

// A switch puts the saved account into the item Codex reads, and keeps
// the one it replaces; a keychain that wouldn't say what is there has
// nothing written over it.
func TestCodexKeychainSwitch(t *testing.T) {
	home := signIn(t)
	dir := filepath.Join(home, ".codex")
	rememberLogins(true)
	codexSignIn(t, home, "work@example.com", "r-work")
	rememberLogins(true)
	k, account := codexInKeychain(t, home, "keyring")
	if from, err := switchSavedLogin("codex", "me@example.com"); err != nil || from != "work@example.com" {
		t.Fatalf("switch: %q %v", from, err)
	}
	if got := k.item(account); !strings.Contains(string(got), `"refresh_token":"r"`) {
		t.Fatalf("the item after the switch: %s", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "auth.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the switch wrote auth.json beside the item: %v", err)
	}
	saved := false
	for _, l := range readLogins() {
		if l.User == "work@example.com" && strings.Contains(string(l.Auth), "agent_identity") {
			saved = true
		}
	}
	if !saved {
		t.Fatal("the account replaced wasn't saved as the keychain had it")
	}

	was := string(k.item(account))
	k.findErr = errKeychainUnread
	forgetCodexKeychain()
	if _, err := switchSavedLogin("codex", "work@example.com"); err == nil {
		t.Fatal("switched over a sign-in the keychain wouldn't give")
	}
	if k.puts != 1 || string(k.item(account)) != was {
		t.Fatalf("written over a sign-in not read: %d puts", k.puts)
	}
}

// Codex's encrypted auth storage (features.secret_auth_storage) and a
// system keychain magpie doesn't read are neither read nor written; one
// whose config.toml doesn't parse is read as auth.json, as before, and
// nothing is saved over it.
func TestCodexStoreNotReadIsLeftAlone(t *testing.T) {
	home := signIn(t)
	dir := filepath.Join(home, ".codex")
	k, _ := codexInKeychain(t, home, "keyring")
	login := []byte(`{"tokens":{"access_token":"x"}}`)
	for name, config := range map[string]string{
		"encrypted": "cli_auth_credentials_store = \"keyring\"\n[features]\nsecret_auth_storage = true\n",
		"ephemeral": "cli_auth_credentials_store = \"ephemeral\"\n",
	} {
		os.WriteFile(filepath.Join(dir, "config.toml"), []byte(config), 0o600)
		forgetCodexKeychain()
		if _, ok := codexAccount(home); ok {
			t.Errorf("%s: read as signed in", name)
		}
		if err := saveCodexLogin(dir, login); err == nil {
			t.Errorf("%s: saved", name)
		}
	}
	codexKeychainHere = false
	os.WriteFile(filepath.Join(dir, "config.toml"), []byte("cli_auth_credentials_store = \"keyring\"\n"), 0o600)
	if _, ok := codexAccount(home); ok {
		t.Error("keychain not read here: read as signed in")
	}
	if err := saveCodexLogin(dir, login); err == nil {
		t.Error("keychain not read here: saved")
	}
	if k.finds != 0 || k.puts != 0 {
		t.Fatalf("the item was reached: %d finds, %d puts", k.finds, k.puts)
	}
	if _, err := os.Stat(filepath.Join(dir, "auth.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("auth.json written: %v", err)
	}

	codexKeychainHere = true
	os.WriteFile(filepath.Join(dir, "auth.json"), login, 0o600)
	os.WriteFile(filepath.Join(dir, "config.toml"), []byte("cli_auth_credentials_store = [\n"), 0o600)
	if b, ok := CodexSignIn(dir); !ok || string(b) != string(login) {
		t.Fatalf("config.toml not parsed: auth.json not read: %s", b)
	}
	if err := saveCodexLogin(dir, []byte(`{"tokens":{"access_token":"y"}}`)); err == nil {
		t.Fatal("saved while where Codex keeps it isn't known")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "auth.json")); string(b) != string(login) {
		t.Fatalf("auth.json written over: %s", b)
	}
}

// Every read is a `security` run, which may put a prompt in front of the
// user: reads close together ask once, and a refusal isn't asked again
// for minutes.
func TestCodexKeychainAskedSeldom(t *testing.T) {
	home := signIn(t)
	k, _ := codexInKeychain(t, home, "keyring")
	for range 20 {
		if _, ok := codexAccount(home); !ok {
			t.Fatal("not read")
		}
	}
	if k.finds != 1 {
		t.Fatalf("%d keychain asks for 20 reads", k.finds)
	}
	k.findErr = errKeychainUnread
	forgetCodexKeychain()
	for range 20 {
		codexAccount(home)
	}
	if k.finds != 2 {
		t.Fatalf("a refusal asked again: %d asks", k.finds-1)
	}
}

// Signing Codex out, as `codex logout`, takes the item and auth.json both.
func TestCodexKeychainSignOut(t *testing.T) {
	home := signIn(t)
	dir := filepath.Join(home, ".codex")
	k, account := codexInKeychain(t, home, "auto")
	os.WriteFile(filepath.Join(dir, "auth.json"), []byte(`{}`), 0o600)
	if err := deleteCodexLogin(dir); err != nil {
		t.Fatal(err)
	}
	if k.item(account) != nil {
		t.Fatal("the item was left")
	}
	if _, err := os.Stat(filepath.Join(dir, "auth.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("auth.json left: %v", err)
	}
}
