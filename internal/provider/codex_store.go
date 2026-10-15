package provider

// Codex's own sign-in, where Codex keeps it (#2367, #2312, Dony2050).
//
// Codex 0.162's config.toml says where (codex-rs/login/src/auth/storage.rs,
// codex-rs/config/src/types.rs): cli_auth_credentials_store "file" (the
// default) is auth.json in CODEX_HOME; "keyring" is the system keychain
// alone; "auto" is the keychain, else auth.json, read and saved in that
// order; "ephemeral" is Codex's memory. With [features]
// secret_auth_storage on (the default on Windows), keyring and auto mean
// Codex's encrypted auth storage instead of the keychain item.
//
// magpie reads and writes the keychain item on macOS only, through
// `security` as it does Claude Code's (readClaudeKeychainItem): the
// generic password of service "Codex Auth" and account
// "cli|<first 16 hex of SHA-256 of the canonical CODEX_HOME>", holding
// auth.json's JSON on one line. Elsewhere, and for the encrypted
// storage, the store is one magpie doesn't read: keyring reads as
// unknown, auto as auth.json, which is what Codex falls back to.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/yetone/magpie/internal/filememo"
	"github.com/yetone/magpie/internal/proc"
)

const codexKeychainService = "Codex Auth"

// keychain is the system keychain as magpie reaches an item in it. Find
// gives fs.ErrNotExist for no item; Remove, nil. Tests put a fake in
// codexKeychain: no test reaches the user's own keychain.
type keychain interface {
	Find(service, account string) ([]byte, error)
	Put(service, account string, secret []byte) error
	Remove(service, account string) error
}

var (
	codexKeychain keychain = securityKeychain{}
	// codexKeychainHere: the keychain item is magpie's to read on this
	// system (macOS); a var so tests run the macOS path everywhere
	codexKeychainHere = runtime.GOOS == "darwin"
	// secretAuthStorageDefault is Codex's default for [features]
	// secret_auth_storage: cfg!(windows) (codex-rs/features/src/lib.rs)
	secretAuthStorageDefault = runtime.GOOS == "windows"
)

// keychainWait bounds one `security` run: one waiting on a keychain
// prompt nobody answers would hold whoever asked.
var keychainWait = 10 * time.Second

var errKeychainUnread = errors.New("the keychain didn't give it to magpie (refused, or no answer)")

// securityKeychain is macOS' `security`, as readClaudeKeychainItem and
// writeClaudeCredential run it. Neither its output nor its arguments go
// into an error: both can hold the credential.
type securityKeychain struct{}

func (securityKeychain) Find(service, account string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), keychainWait)
	defer cancel()
	out, err := proc.CommandContext(ctx, "security", "find-generic-password", "-s", service, "-a", account, "-w").Output()
	if err != nil {
		if keychainItemMissing(err) {
			return nil, fs.ErrNotExist
		}
		return nil, errKeychainUnread
	}
	b, _ := keychainText(bytes.TrimSpace(out))
	return b, nil
}

func (securityKeychain) Put(service, account string, secret []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), keychainWait)
	defer cancel()
	if err := proc.CommandContext(ctx, "security", "add-generic-password", "-U", "-s", service, "-a", account, "-w", string(secret)).Run(); err != nil {
		return errors.New("the keychain didn't take the sign-in")
	}
	return nil
}

func (securityKeychain) Remove(service, account string) error {
	ctx, cancel := context.WithTimeout(context.Background(), keychainWait)
	defer cancel()
	if err := proc.CommandContext(ctx, "security", "delete-generic-password", "-s", service, "-a", account).Run(); err != nil && !keychainItemMissing(err) {
		return errors.New("the keychain didn't remove the sign-in")
	}
	return nil
}

// keychainItemMissing: `security` exits 44 for errSecItemNotFound.
func keychainItemMissing(err error) bool {
	var exit *exec.ExitError
	return errors.As(err, &exit) && exit.ExitCode() == 44
}

// codexKeychainAccount is Codex's compute_store_key: the home made
// canonical where it can be (else as given), SHA-256, its first 16 hex
// digits after "cli|".
func codexKeychainAccount(home string) string {
	p := home
	if abs, err := filepath.Abs(home); err == nil {
		if real, err := filepath.EvalSymlinks(abs); err == nil {
			p = real
		}
	}
	sum := sha256.Sum256([]byte(p))
	return "cli|" + hex.EncodeToString(sum[:])[:16]
}

// codexStoreSetting is what config.toml in a Codex home says of where
// the sign-in is kept.
type codexStoreSetting struct {
	Mode    string // file, keyring, auto or ephemeral
	Secrets bool   // [features] secret_auth_storage
}

// readCodexStoreSetting reads config.toml as Codex does: no file or no
// key is "file". A file that doesn't parse, or a value Codex doesn't
// know (it would refuse to start), is an error: where the sign-in is
// isn't known.
func readCodexStoreSetting(home string) (codexStoreSetting, error) {
	path := filepath.Join(home, "config.toml")
	s, err := filememo.Read("codex credential store", path, func(b []byte) (codexStoreSetting, error) {
		var c struct {
			Store    string         `toml:"cli_auth_credentials_store"`
			Features map[string]any `toml:"features"`
		}
		if err := toml.Unmarshal(b, &c); err != nil {
			return codexStoreSetting{}, fmt.Errorf("%s doesn't read as TOML", path)
		}
		s := codexStoreSetting{Mode: c.Store, Secrets: secretAuthStorageDefault}
		if v, ok := c.Features["secret_auth_storage"].(bool); ok {
			s.Secrets = v
		}
		return s, nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return codexStoreSetting{Mode: "file", Secrets: secretAuthStorageDefault}, nil
	}
	if err != nil {
		return codexStoreSetting{}, err
	}
	switch s.Mode {
	case "":
		s.Mode = "file"
	case "file", "keyring", "auto", "ephemeral":
	default:
		return codexStoreSetting{}, fmt.Errorf("cli_auth_credentials_store = %q in %s isn't one Codex knows", s.Mode, path)
	}
	return s, nil
}

// keychainItem: the store is the keychain item magpie reads.
func (s codexStoreSetting) keychainItem() bool {
	return (s.Mode == "keyring" || s.Mode == "auto") && !s.Secrets && codexKeychainHere
}

// where names the store keyring and auto mean here.
func (s codexStoreSetting) where() string {
	switch {
	case s.Secrets:
		return "Codex's encrypted auth storage (features.secret_auth_storage)"
	case runtime.GOOS == "darwin" || codexKeychainHere:
		return "the macOS keychain"
	}
	return "the system keychain"
}

// codexStoreError: Codex keeps its sign-in where magpie couldn't read it,
// so whether it is signed in, and to whom, isn't known.
type codexStoreError struct {
	Setting codexStoreSetting
	Config  string
	Cause   error // nil: a store magpie doesn't read on this system
}

func (e *codexStoreError) Error() string {
	if e.Setting.Mode == "ephemeral" {
		return fmt.Sprintf("Codex keeps its sign-in in its own memory (cli_auth_credentials_store = \"ephemeral\" in %s), which no other program can read", e.Config)
	}
	if e.Cause != nil {
		return fmt.Sprintf("Codex keeps its sign-in in %s (cli_auth_credentials_store = %q in %s), and %v", e.Setting.where(), e.Setting.Mode, e.Config, e.Cause)
	}
	return fmt.Sprintf("Codex keeps its sign-in in %s (cli_auth_credentials_store = %q in %s), which magpie doesn't read", e.Setting.where(), e.Setting.Mode, e.Config)
}

// codexStore is where a Codex sign-in was read from, so that a refresh
// writes its rotated tokens back there.
type codexStore struct {
	home     string
	keychain bool
}

func (s codexStore) file() string { return filepath.Join(s.home, "auth.json") }

// codexKeychainCache keeps what the keychain gave for a home: each read is
// a `security` run, which may put a prompt in front of the user, and
// every request and page reads the sign-in. A refusal or no answer is
// kept longer, so that a prompt isn't put up again at once (as Copilot's,
// copilotSecret). codexKeychainMu has one `security` ask at a time: two
// readers at once would put up two prompts.
var (
	codexKeychainMu    sync.Mutex
	codexKeychainCache = map[string]codexKeychainRead{}
)

type codexKeychainRead struct {
	b   []byte
	err error
	at  time.Time
}

const (
	codexKeychainTTL     = 10 * time.Second
	codexKeychainRefused = 10 * time.Minute
)

// findCodexKeychain reads the item for home; fresh asks the keychain
// whatever was kept, for a read whose tokens will be spent or saved.
func findCodexKeychain(home string, fresh bool) ([]byte, error) {
	account := codexKeychainAccount(home)
	codexKeychainMu.Lock()
	defer codexKeychainMu.Unlock()
	if r, ok := codexKeychainCache[account]; ok && !fresh {
		ttl := codexKeychainTTL
		if r.err != nil && !errors.Is(r.err, fs.ErrNotExist) {
			ttl = codexKeychainRefused
		}
		if time.Since(r.at) < ttl {
			return r.b, r.err
		}
	}
	b, err := codexKeychain.Find(codexKeychainService, account)
	if err == nil {
		var obj map[string]json.RawMessage
		if json.Unmarshal(b, &obj) != nil || obj == nil {
			b, err = nil, errors.New("the keychain item isn't a Codex sign-in magpie can read")
		}
	}
	codexKeychainCache[account] = codexKeychainRead{b: b, err: err, at: time.Now()}
	return b, err
}

func keepCodexKeychain(home string, b []byte) {
	codexKeychainMu.Lock()
	codexKeychainCache[codexKeychainAccount(home)] = codexKeychainRead{b: b, at: time.Now()}
	codexKeychainMu.Unlock()
}

// forgetCodexKeychain has the next read ask the keychain.
func forgetCodexKeychain() {
	codexKeychainMu.Lock()
	clear(codexKeychainCache)
	codexKeychainMu.Unlock()
}

// readCodexLogin is Codex's sign-in under home as Codex would load it,
// and where it came from. fs.ErrNotExist (wrapped or not): signed out,
// as far as magpie can see. *codexStoreError: kept where magpie couldn't
// read it, so not known. A config.toml that doesn't read leaves auth.json
// read as before there was a setting (Codex itself wouldn't start), and
// nothing is saved over it (saveCodexLogin).
func readCodexLogin(home string, fresh bool) ([]byte, codexStore, error) {
	st := codexStore{home: home}
	if discoveryOff() {
		return nil, st, errAccountDiscoveryOff
	}
	s, err := readCodexStoreSetting(home)
	if err != nil {
		b, err := readAccountFile(st.file())
		return b, st, err
	}
	cfg := filepath.Join(home, "config.toml")
	switch {
	case s.Mode == "ephemeral":
		return nil, st, &codexStoreError{Setting: s, Config: cfg}
	case s.keychainItem():
		b, kerr := findCodexKeychain(home, fresh)
		if kerr == nil {
			st.keychain = true
			return b, st, nil
		}
		if s.Mode == "keyring" {
			if errors.Is(kerr, fs.ErrNotExist) {
				return nil, st, kerr
			}
			return nil, st, &codexStoreError{Setting: s, Config: cfg, Cause: kerr}
		}
		// auto: the file, as Codex falls back to it on no item and on an
		// item it couldn't load
		b, err := readAccountFile(st.file())
		if err != nil && !errors.Is(kerr, fs.ErrNotExist) {
			// Codex, which the item lets in, may well be signed in
			return nil, st, &codexStoreError{Setting: s, Config: cfg, Cause: kerr}
		}
		return b, st, err
	case s.Mode == "keyring":
		return nil, st, &codexStoreError{Setting: s, Config: cfg}
	case s.Mode == "auto":
		// a store magpie doesn't read: auth.json is there only when Codex
		// is on its fallback (a save to the store removes it)
		b, err := readAccountFile(st.file())
		if errors.Is(err, fs.ErrNotExist) {
			return nil, st, &codexStoreError{Setting: s, Config: cfg}
		}
		return b, st, err
	}
	b, err := readAccountFile(st.file())
	return b, st, err
}

// write puts a sign-in back where it was read from: rotated tokens, every
// other field as it was. The keychain item takes it on one line, as Codex
// writes it; `security -w` prints one with a newline as hex (#70).
func (s codexStore) write(b []byte) error {
	if discoveryOff() {
		return errAccountDiscoveryOff
	}
	if !s.keychain {
		// the user's own sign-in, which Codex CLI and the Codex app
		// read: written beside it, flushed and renamed in, never
		// truncated in place (#2239, #1505)
		return writePrivate(s.file(), append(bytes.TrimSpace(b), '\n'))
	}
	var one bytes.Buffer
	if err := json.Compact(&one, b); err != nil {
		return errors.New("the Codex sign-in to save isn't JSON")
	}
	if err := codexKeychain.Put(codexKeychainService, codexKeychainAccount(s.home), one.Bytes()); err != nil {
		forgetCodexKeychain()
		return err
	}
	keepCodexKeychain(s.home, one.Bytes())
	return nil
}

// saveCodexLogin makes b the sign-in Codex goes by (a switch, or the
// first sign-in in magpie), in the store its config.toml names. The
// store's own sign-in is read first, never taken for absent when it
// couldn't be read: one that wasn't read is never written over. auth.json
// left beside a keychain item is left to Codex, which removes it at its
// next save; the item goes first when it reads.
func saveCodexLogin(home string, b []byte) error {
	if discoveryOff() {
		return errAccountDiscoveryOff
	}
	s, err := readCodexStoreSetting(home)
	if err != nil {
		return fmt.Errorf("where Codex keeps its sign-in isn't known (%v); nothing was changed", err)
	}
	st := codexStore{home: home}
	cfg := filepath.Join(home, "config.toml")
	switch {
	case s.Mode == "ephemeral":
		return &codexStoreError{Setting: s, Config: cfg}
	case s.keychainItem():
		_, kerr := findCodexKeychain(home, true)
		if kerr != nil && !errors.Is(kerr, fs.ErrNotExist) {
			return &codexStoreError{Setting: s, Config: cfg, Cause: kerr}
		}
		st.keychain = true
		werr := st.write(b)
		if werr == nil || s.Mode == "keyring" || kerr == nil {
			// an item that is there goes first with Codex: a file beside
			// it would be passed over
			return werr
		}
		// auto with no item, and the keychain not taking one: Codex
		// saves to the file then, and loads it
		st.keychain = false
	case s.Mode == "keyring":
		return &codexStoreError{Setting: s, Config: cfg}
	case s.Mode == "auto":
		// a store magpie doesn't read: only a Codex on its file fallback
		// goes by auth.json
		if _, err := os.Stat(st.file()); err != nil {
			return &codexStoreError{Setting: s, Config: cfg}
		}
	}
	return st.write(b)
}

// deleteCodexLogin signs Codex out, as `codex logout` does: the item
// for this home and auth.json both go (auto would fall back to the file).
// The item not removed, the file is left too.
func deleteCodexLogin(home string) error {
	if discoveryOff() {
		return errAccountDiscoveryOff
	}
	s, err := readCodexStoreSetting(home)
	if err != nil {
		return fmt.Errorf("where Codex keeps its sign-in isn't known (%v); nothing was changed", err)
	}
	cfg := filepath.Join(home, "config.toml")
	switch {
	case s.Mode == "ephemeral":
		return &codexStoreError{Setting: s, Config: cfg}
	case s.keychainItem():
		err := codexKeychain.Remove(codexKeychainService, codexKeychainAccount(home))
		forgetCodexKeychain()
		if err != nil {
			return err
		}
	case s.Mode == "keyring":
		return &codexStoreError{Setting: s, Config: cfg}
	}
	defer filememo.Forget()
	if err := os.Remove(filepath.Join(home, "auth.json")); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// CodexSignIn is the sign-in Codex goes by under its home dir, from
// wherever its config.toml has it kept (readCodexLogin); ok is false for
// none, or one magpie couldn't read. Agents' wiring reads it here.
func CodexSignIn(dir string) (b []byte, ok bool) {
	b, _, err := readCodexLogin(dir, false)
	return b, err == nil
}

// CodexKeychainForTest has Codex's keychain item read through find (no
// item: fs.ErrNotExist) on every system, until restore.
func CodexKeychainForTest(find func(service, account string) ([]byte, error)) (restore func()) {
	old, oldHere := codexKeychain, codexKeychainHere
	codexKeychain, codexKeychainHere = findOnly(find), true
	forgetCodexKeychain()
	return func() {
		codexKeychain, codexKeychainHere = old, oldHere
		forgetCodexKeychain()
	}
}

type findOnly func(service, account string) ([]byte, error)

func (f findOnly) Find(service, account string) ([]byte, error) { return f(service, account) }
func (findOnly) Put(string, string, []byte) error {
	return errors.New("this test's keychain takes nothing")
}
func (findOnly) Remove(string, string) error {
	return errors.New("this test's keychain removes nothing")
}
