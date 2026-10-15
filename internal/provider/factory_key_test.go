package provider

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

// #506: a Factory account added by its API key, as droid takes
// FACTORY_API_KEY: whoami asked with the key says whose it is; the account's
// requests carry the key as the bearer with droid's headers and no active
// org (droid's comes from a sign-in), and nothing is renewed with WorkOS. A
// key whoami refuses isn't added; the same key again is "exists".
func TestFactoryAPIKey(t *testing.T) {
	signIn(t)
	const good, bad = "fk-good_0123456789abcdef", "fk-bad_0123456789abcdef"
	var mu sync.Mutex
	var sent []http.Header
	factorySite(t, func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		switch {
		case strings.HasPrefix(r.URL.Path, "/wos/"):
			t.Errorf("WorkOS asked for a key account: %s", r.URL.Path)
			w.WriteHeader(400)
		case r.URL.Path == "/api/cli/whoami":
			if auth != "Bearer "+good {
				factoryJSON(w, 401, map[string]any{"detail": "Invalid API key"})
				return
			}
			factoryJSON(w, 200, map[string]any{"userId": "user_k", "orgId": "fac_K", "email": "key@example.com"})
		case r.URL.Path == "/api/llm/a/v1/messages":
			mu.Lock()
			sent = append(sent, r.Header.Clone())
			mu.Unlock()
			// a key's org isn't one droid sends: the header refused, as
			// Factory refuses an org the caller didn't pick
			if auth != "Bearer "+good || r.Header.Get("X-Factory-Org-Id") != "" {
				w.WriteHeader(403)
				io.WriteString(w, `{"detail":"Forbidden"}`)
				return
			}
			io.WriteString(w, `{"type":"message"}`)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	})

	res, err := ImportFactoryKeys(context.Background(), []string{"FACTORY_API_KEY=" + good + "\n" + bad + "\n" + good})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 || res[0].User != "key@example.com" || res[0].Status != "added" || res[1].Status != "failed" || strings.Contains(res[1].User, "0123456789") {
		t.Fatalf("imported %+v", res)
	}
	if c := factoryKept(t, "key@example.com"); !c.Key || c.Access != good || c.Refresh != "" || c.Active != "" {
		t.Fatalf("kept %+v", c)
	}
	p, ok := find(Accounts(), "factory")
	if !ok || p.Account.User != "key@example.com" {
		t.Fatalf("account: %v %+v", ok, p.Account)
	}
	if code, b := factorySend(t, p); code != 200 {
		t.Fatalf("a key account's request: %d %s", code, b)
	}
	mu.Lock()
	h := sent[len(sent)-1]
	mu.Unlock()
	if h.Get("Authorization") != "Bearer "+good || h.Get("X-Factory-Client") != "cli" || h.Get("User-Agent") != "factory-cli/"+factoryVersion || h.Get("X-Factory-Org-Id") != "" {
		t.Fatalf("sent %v", h)
	}
	// still no org after the request, and no whoami asked for one
	if c := factoryKept(t, "key@example.com"); c.Active != "" {
		t.Fatalf("a key account got an org: %q", c.Active)
	}

	res, err = ImportFactoryKeys(context.Background(), []string{good})
	if err != nil || len(res) != 1 || res[0].Status != "exists" {
		t.Fatalf("again: %+v %v", res, err)
	}
	if _, err := ImportFactoryKeys(context.Background(), []string{"sk-not-factory"}); err == nil {
		t.Fatal("no key in it, and no error")
	}
}
