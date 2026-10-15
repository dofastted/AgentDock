package environments

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/yetone/magpie/internal/proc"
)

// stubWSL stands in for wsl.exe: which distros run, and what reachScript
// prints in each (or the error wsl.exe gives); it records every distro a
// probe was run in.
func stubWSL(t *testing.T, up map[string]bool, out map[string]string, errs map[string]error) *[]string {
	t.Helper()
	oldOne, oldUp := probeOne, distroUp
	t.Cleanup(func() {
		probeOne, distroUp = oldOne, oldUp
		reaches.Lock()
		reaches.by = nil
		reaches.Unlock()
	})
	var mu sync.Mutex
	ran := &[]string{}
	distroUp = func(d string) bool { return up[d] }
	probeOne = func(d, url string) ([]byte, error) {
		mu.Lock()
		*ran = append(*ran, d)
		mu.Unlock()
		if err := errs[d]; err != nil {
			return nil, err
		}
		return []byte(out[d]), nil
	}
	return ran
}

const hello = `{"name":"magpie","version":"0.1.0"}`

// Each distro's status comes from the probe run in it: the gateway's own
// answer is ok, a refused or timed-out connection is blocked even though
// the host listens, and a distro without a tool to ask with, or a wsl.exe
// that fails, is unknown rather than guessed. A stopped distro is never run
// in, and keeps what it said while it ran.
func TestProbeStatuses(t *testing.T) {
	ran := stubWSL(t,
		map[string]bool{"Ok": true, "Blocked": true, "Other": true, "Bare": true, "Broken": true, "TCP": true, "Asleep": false},
		map[string]string{
			"Ok":      "magpie:812 \nvia:curl\nrc:0\nout:" + hello + "\n",
			"Blocked": "via:curl\nrc:7\nout:curl: (7) Failed to connect to 127.0.0.1 port 3425: Connection refused\n",
			"Other":   "via:wget\nrc:0\nout:<html>nginx</html>\n",
			"Bare":    "via:none\n",
			"TCP":     "via:tcp\nrc:0\n",
		},
		map[string]error{"Broken": errors.New("exit status 1")})
	url := "http://127.0.0.1:3425"
	var targets []Target
	for _, d := range []string{"Ok", "Blocked", "Other", "Bare", "Broken", "TCP", "Asleep"} {
		targets = append(targets, Target{Distro: d, URL: url})
	}
	reaches.Lock()
	reaches.by = map[string]Reach{"Asleep": {Status: ReachOK, Via: "curl", URL: url}}
	reaches.Unlock()

	Probe(targets)
	got := Reaches()
	want := map[string]string{
		"Ok": ReachOK, "Blocked": ReachBlocked, "Other": ReachBlocked, "Bare": ReachUnknown,
		"Broken": ReachUnknown, "TCP": ReachOK, "Asleep": ReachNotProbed,
	}
	for d, s := range want {
		if got[d].Status != s {
			t.Errorf("%s: status %q (%s), want %q", d, got[d].Status, got[d].Detail, s)
		}
	}
	if !strings.Contains(got["Blocked"].Detail, "Connection refused") {
		t.Errorf("blocked detail %q", got["Blocked"].Detail)
	}
	if !strings.Contains(got["Asleep"].Detail, "last probe: ok") {
		t.Errorf("asleep detail %q", got["Asleep"].Detail)
	}
	if len(got["Ok"].Magpie) != 1 || got["Ok"].Magpie[0] != "812" || len(got["Blocked"].Magpie) != 0 {
		t.Errorf("magpie pids: ok %v, blocked %v", got["Ok"].Magpie, got["Blocked"].Magpie)
	}
	for _, d := range *ran {
		if d == "Asleep" {
			t.Fatal("a stopped distro was run in, which starts it")
		}
	}
	if len(*ran) != 6 {
		t.Fatalf("ran in %v", *ran)
	}
}

// A probe is fresh for a while: the page reads it again without asking
// wsl.exe, and asks again when it ages, the URL moves, or a distro is new.
func TestStale(t *testing.T) {
	stubWSL(t, map[string]bool{"Debian": true}, map[string]string{"Debian": "via:curl\nrc:0\nout:" + hello}, nil)
	url := "http://127.0.0.1:3425"
	ts := []Target{{Distro: "Debian", URL: url}}
	if !Stale(ts) {
		t.Fatal("never probed, not stale")
	}
	Probe(ts)
	if Stale(ts) {
		t.Fatal("just probed, stale")
	}
	if !Stale([]Target{{Distro: "Debian", URL: "http://172.20.0.1:3425"}}) {
		t.Fatal("a moved URL isn't stale")
	}
	if !Stale(append(ts, Target{Distro: "Ubuntu", URL: url})) {
		t.Fatal("a new distro isn't stale")
	}
	reaches.Lock()
	r := reaches.by["Debian"]
	r.CheckedAt = r.CheckedAt.Add(-reachFresh - 1)
	reaches.by["Debian"] = r
	reaches.Unlock()
	if !Stale(ts) {
		t.Fatal("an old probe isn't stale")
	}
}

// reachScript, run by a real sh as wsl.exe runs it, finds the gateway
// that answers and calls a closed port blocked, whichever tool the
// machine has; parseProbe reads what it prints.
func TestReachScriptRuns(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh here")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/hello" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(hello))
	}))
	defer srv.Close()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closed := "http://" + l.Addr().String()
	l.Close()
	run := func(url string) Reach {
		out, err := proc.Command(sh, "-c", reachScript, "sh", url).Output()
		if err != nil {
			t.Fatalf("sh: %v", err)
		}
		return parseProbe(Reach{URL: url}, string(out))
	}
	if r := run(srv.URL); r.Status != ReachOK {
		t.Fatalf("the gateway: %+v", r)
	}
	if r := run(closed); r.Status != ReachBlocked {
		t.Fatalf("a closed port: %+v", r)
	}
}

// HostListener says whether the gateway's address accepts a connection
// here, nothing more.
func TestHostListener(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	if !HostListener("http://" + addr) {
		t.Fatal("a listening address said down")
	}
	l.Close()
	if HostListener("http://" + addr) {
		t.Fatal("a closed address said up")
	}
}
