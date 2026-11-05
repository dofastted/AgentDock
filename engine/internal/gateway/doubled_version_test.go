package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWithoutDoubledVersion(t *testing.T) {
	cases := []struct {
		in, out string
		ok      bool
	}{
		{in: "/v1/messages", ok: false},
		{in: "/v1/chat/completions", ok: false},
		{in: "/v1/models", ok: false},
		{in: "/v1beta/models", ok: false},
		{in: "/messages", ok: false},
		{in: "/v1/v10/messages", ok: false},
		{in: "/v1/v1betaextra", ok: false},
		{in: "/v1/v1/messages", out: "/v1/messages", ok: true},
		{in: "/v1/v1/v1/messages", out: "/v1/messages", ok: true},
		{in: "/v1/v1/messages/count_tokens", out: "/v1/messages/count_tokens", ok: true},
		{in: "/v1/v1/chat/completions", out: "/v1/chat/completions", ok: true},
		{in: "/v1/v1/responses", out: "/v1/responses", ok: true},
		{in: "/v1/v1", out: "/v1", ok: true},
		{in: "/v1/v1/models", out: "/v1/models", ok: true},
		{in: "/v1/v1beta/models/x:generateContent", out: "/v1beta/models/x:generateContent", ok: true},
		{in: "/v1/v1/v1beta/models", out: "/v1beta/models", ok: true},
	}
	for _, c := range cases {
		got, ok := withoutDoubledVersion(c.in)
		if ok != c.ok || (c.ok && got != c.out) {
			t.Errorf("%s → %q %v, want %q %v", c.in, got, ok, c.out, c.ok)
		}
	}
}

func TestDoubledV1ReachesTheRoute(t *testing.T) {
	h := New().Handler()
	same := func(method, doubled, canonical string, body string) {
		t.Helper()
		ask := func(path string) (int, string) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
			return rec.Code, rec.Body.String()
		}
		code, out := ask(doubled)
		wantCode, want := ask(canonical)
		if code != wantCode || out != want {
			t.Fatalf("%s %s: %d %s\nwant %s: %d %s", method, doubled, code, out, canonical, wantCode, want)
		}
		if strings.Contains(out, "magpie serves") {
			t.Fatalf("%s still the catch-all: %s", doubled, out)
		}
	}
	same(http.MethodGet, "/v1/v1/models", "/v1/models", "")
	same(http.MethodGet, "/v1/v1beta/models", "/v1beta/models", "")
	same(http.MethodPost, "/v1/v1/messages", "/v1/messages",
		`{"model":"missing-model","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/not-a-route", nil))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "magpie serves") {
		t.Fatalf("unknown path: %d %s", rec.Code, rec.Body.String())
	}
}
