// Package environments is AgentDock's view of where agents run: this
// Windows machine and each of its WSL distros. It says whether the gateway
// listens here (HostListener) and whether each running distro reaches it
// from inside (Reach), which only a probe run in the distro can tell: the
// host's own dial succeeds while Windows' firewall may still keep WSL out.
package environments

import (
	"net"
	"strings"
	"sync"
	"time"

	"github.com/yetone/magpie/internal/wslrun"
)

// Reach statuses.
const (
	ReachOK        = "ok"
	ReachBlocked   = "blocked"
	ReachUnknown   = "unknown"
	ReachNotProbed = "not-probed"
)

// Reach is what a probe run inside a distro found of the gateway.
type Reach struct {
	Status    string    `json:"status"`
	Via       string    `json:"via,omitempty"` // curl, wget or tcp
	Detail    string    `json:"detail,omitempty"`
	URL       string    `json:"url,omitempty"`
	CheckedAt time.Time `json:"checkedAt,omitzero"`
	// Magpie are the pids of a magpie of the distro's own running there,
	// which would write the same agents' files as this one
	Magpie []string `json:"magpie,omitempty"`
}

const (
	probeTimeout  = 5 * time.Second
	reachFresh    = 30 * time.Second
	probeParallel = 4
)

// reachScript asks url's /api/hello from inside the distro with the first
// of curl, wget or bash's /dev/tcp it has, and prints what it used ("via:"),
// its exit code ("rc:") and, for an HTTP ask, the answer ("out:"); first,
// the pids of a magpie of the distro's own running there ("magpie:").
const reachScript = `u="$1"
if command -v pgrep >/dev/null 2>&1; then
  printf 'magpie:%s\n' "$(pgrep -x magpie | tr '\n' ' ')"
fi
if command -v curl >/dev/null 2>&1; then
  out=$(curl -sS -m 3 "$u/api/hello" 2>&1); rc=$?
  printf 'via:curl\nrc:%s\nout:%s\n' "$rc" "$out"; exit 0
fi
if command -v wget >/dev/null 2>&1; then
  out=$(wget -q -T 3 -O - "$u/api/hello" 2>&1); rc=$?
  printf 'via:wget\nrc:%s\nout:%s\n' "$rc" "$out"; exit 0
fi
if command -v bash >/dev/null 2>&1 && command -v timeout >/dev/null 2>&1; then
  hp=${u#http://}; hp=${hp%%/*}; h=${hp%:*}; p=${hp##*:}
  timeout 3 bash -c 'exec 3<>"/dev/tcp/$0/$1"' "$h" "$p" 2>/dev/null; rc=$?
  printf 'via:tcp\nrc:%s\n' "$rc"; exit 0
fi
printf 'via:none\n'
`

// probeOne runs reachScript in distro against url; a var for tests.
var probeOne = func(distro, url string) ([]byte, error) {
	return wslrun.Run(probeTimeout, "-d", distro, "--exec", "sh", "-c", reachScript, "sh", url)
}

// distroUp is whether distro runs now (wsl.exe -l --running, which starts
// none); a var for tests.
var distroUp = wslrun.Up

// probe asks distro whether it reaches url, never starting it.
func probe(distro, url string) Reach {
	r := Reach{URL: url, CheckedAt: time.Now()}
	if !distroUp(distro) {
		r.Status, r.Detail = ReachNotProbed, "distro not running"
		return r
	}
	out, err := probeOne(distro, url)
	if err != nil {
		// wslrun.Run's timeout kills wsl.exe, which then reads as an exit error
		r.Status, r.Detail = ReachUnknown, "wsl.exe: "+err.Error()
		return r
	}
	return parseProbe(r, string(out))
}

// parseProbe reads reachScript's output into r.
func parseProbe(r Reach, out string) Reach {
	var rc, body string
	for _, l := range strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n") {
		switch {
		case strings.HasPrefix(l, "via:"):
			r.Via = strings.TrimPrefix(l, "via:")
		case strings.HasPrefix(l, "rc:"):
			rc = strings.TrimPrefix(l, "rc:")
		case strings.HasPrefix(l, "out:"):
			body = strings.TrimPrefix(l, "out:")
		case strings.HasPrefix(l, "magpie:"):
			r.Magpie = strings.Fields(strings.TrimPrefix(l, "magpie:"))
		}
	}
	magpie := strings.Contains(strings.ReplaceAll(body, " ", ""), `"name":"magpie"`)
	switch r.Via {
	case "none", "":
		r.Status, r.Via, r.Detail = ReachUnknown, "", "no curl, wget or bash with timeout in the distro"
	case "curl":
		switch {
		case rc == "0" && magpie:
			r.Status = ReachOK
		case rc == "0":
			r.Status, r.Detail = ReachBlocked, "something other than the gateway answers there"
		// couldn't resolve, couldn't connect, timed out, empty reply, recv failure
		case rc == "6" || rc == "7" || rc == "28" || rc == "52" || rc == "56":
			r.Status, r.Detail = ReachBlocked, firstLine(body)
		default:
			r.Status, r.Detail = ReachUnknown, "curl exit "+rc+": "+firstLine(body)
		}
	case "wget":
		switch {
		case rc == "0" && magpie:
			r.Status = ReachOK
		case rc == "0" || rc == "8":
			r.Status, r.Detail = ReachBlocked, "something other than the gateway answers there"
		case rc == "4":
			r.Status, r.Detail = ReachBlocked, firstLine(body)
		default:
			r.Status, r.Detail = ReachUnknown, "wget exit "+rc+": "+firstLine(body)
		}
	case "tcp":
		switch rc {
		case "0":
			r.Status, r.Detail = ReachOK, "TCP connect only (no curl or wget in the distro)"
		case "1", "124":
			r.Status, r.Detail = ReachBlocked, "TCP connect failed"
		default:
			r.Status, r.Detail = ReachUnknown, "bash exit "+rc
		}
	default:
		r.Status, r.Detail = ReachUnknown, "unexpected probe output"
	}
	return r
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	return s
}

// Target is a distro to probe and the gateway URL it reaches magpie at.
type Target struct{ Distro, URL string }

var reaches struct {
	sync.Mutex
	by      map[string]Reach // by distro
	running bool
}

// Reaches are the last probes of the distros, by name; none is run here.
func Reaches() map[string]Reach {
	reaches.Lock()
	defer reaches.Unlock()
	out := make(map[string]Reach, len(reaches.by))
	for k, v := range reaches.by {
		out[k] = v
	}
	return out
}

// Stale is whether any of targets has no probe younger than reachFresh,
// or was probed at another URL.
func Stale(targets []Target) bool {
	reaches.Lock()
	defer reaches.Unlock()
	for _, t := range targets {
		r, ok := reaches.by[t.Distro]
		if !ok || r.URL != t.URL || time.Since(r.CheckedAt) > reachFresh {
			return true
		}
	}
	return false
}

// Probe probes targets, at most probeParallel at once, and keeps what each
// said; a second call while one runs returns at once. Distros no longer in
// targets are forgotten.
func Probe(targets []Target) {
	reaches.Lock()
	if reaches.running {
		reaches.Unlock()
		return
	}
	reaches.running = true
	reaches.Unlock()
	defer func() {
		reaches.Lock()
		reaches.running = false
		reaches.Unlock()
	}()

	got := make([]Reach, len(targets))
	sem := make(chan struct{}, probeParallel)
	var wg sync.WaitGroup
	for i, t := range targets {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			got[i] = probe(t.Distro, t.URL)
		}()
	}
	wg.Wait()

	reaches.Lock()
	defer reaches.Unlock()
	by := map[string]Reach{}
	for i, t := range targets {
		r := got[i]
		// a stopped distro keeps what it said while it ran, marked not probed
		if r.Status == ReachNotProbed {
			if old, ok := reaches.by[t.Distro]; ok {
				r.Via, r.Detail = old.Via, "distro not running; last probe: "+old.Status
				r.CheckedAt = old.CheckedAt
			}
		}
		by[t.Distro] = r
	}
	reaches.by = by
}

// HostListener is whether something accepts a TCP connection at the
// gateway's url from this machine. It says nothing of WSL reaching it.
func HostListener(url string) bool {
	hp := strings.TrimPrefix(strings.TrimPrefix(url, "http://"), "https://")
	hp, _, _ = strings.Cut(hp, "/")
	c, err := net.DialTimeout("tcp", hp, time.Second)
	if err != nil {
		return false
	}
	c.Close()
	return true
}
