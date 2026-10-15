package gui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/settings"
)

func envSandbox(t *testing.T) {
	t.Helper()
	h := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("USERPROFILE", h)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(h, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(h, ".cache"))
}

func envCall(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	Handler(nil, nil).ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

// Turning a distro off on the Environments page keeps it in settings, and
// a save of the Settings page, which doesn't send the list, keeps it too;
// turned on again it is gone from the list.
func TestEnvironmentDistroSwitch(t *testing.T) {
	envSandbox(t)
	if rec := envCall(t, "POST", "/api/environments/Debian/enabled", `{"enabled":false}`); rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if !settings.Load().WSLDistroOff("Debian") {
		t.Fatal("Debian not turned off")
	}
	envCall(t, "POST", "/api/settings", `{"theme":"dark"}`)
	if !settings.Load().WSLDistroOff("Debian") {
		t.Fatal("a save of the Settings page turned Debian back on")
	}
	envCall(t, "POST", "/api/environments/Debian/enabled", `{"enabled":true}`)
	if settings.Load().WSLDistrosOff != nil {
		t.Fatalf("still off: %v", settings.Load().WSLDistrosOff)
	}
}

// The page's state names this computer's row with whether the gateway
// listens here, apart from any distro's reach; with no providers yet it is
// a first run.
func TestEnvironmentState(t *testing.T) {
	envSandbox(t)
	rec := envCall(t, "GET", "/api/environments", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var s struct {
		Environments []struct {
			ID           string          `json:"id"`
			HostListener *bool           `json:"hostListener"`
			Reach        json.RawMessage `json:"reach"`
		} `json:"environments"`
		FirstRun bool `json:"firstRun"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Environments) == 0 || s.Environments[0].ID != "windows" || s.Environments[0].HostListener == nil || s.Environments[0].Reach != nil {
		t.Fatalf("state: %s", rec.Body)
	}
	if !s.FirstRun {
		t.Fatal("no providers, not a first run")
	}
}

// wsl --shutdown stops every distro: the page has to say the user
// confirmed it, or nothing is run.
func TestEnvironmentShutdownNeedsConfirm(t *testing.T) {
	envSandbox(t)
	if rec := envCall(t, "POST", "/api/environments/wsl/shutdown", `{}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("unconfirmed shutdown: %d %s", rec.Code, rec.Body)
	}
}

// Something else on the gateway's port — a magpie behind a key answers 401
// and names nothing — is said, so the user knows agents reach it, not this
// engine.
func TestEnvironmentGatewayHeldElsewhere(t *testing.T) {
	envSandbox(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"no API key was sent"}}`))
	}))
	defer srv.Close()
	t.Setenv("MAGPIE_ADDR", strings.TrimPrefix(srv.URL, "http://"))
	var s struct {
		HeldElsewhere bool            `json:"heldElsewhere"`
		Holder        json.RawMessage `json:"holder"`
	}
	rec := envCall(t, "GET", "/api/environments", "")
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	if !s.HeldElsewhere || s.Holder != nil {
		t.Fatalf("state: %s", rec.Body)
	}
}
