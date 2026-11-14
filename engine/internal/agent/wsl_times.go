package agent

import (
	"strconv"
	"strings"
	"time"
)

// AgentDock: one wsl.exe for the times of every skill copy in a folder of
// a distro (magpie's store), in place of walking each copy's files through
// \\wsl.localhost, a round trip per file: the sync's look for edits made
// in the copies took seconds for a few dozen skills.

// wslTimesScript prints, for each entry below $1 (but the mark named $2,
// .git and .DS_Store, as changedAt leaves them out), its path and
// modification time, and then each copy's mark with "M\t" before it.
const wslTimesScript = `cd "$1" || exit 1
find . -mindepth 1 \( -name .git -o -name .DS_Store -o -name __pycache__ \) -prune -o ! -name "$2" -printf '%P\t%T@\n'
find . -mindepth 2 -maxdepth 2 -name "$2" -printf 'M\t%P\t%T@\n'`

// WSLTreeTimes is, for each folder right in local (a path in a running
// distro as magpie opens it), when anything in it last changed and when
// the file called marker in it was written; ok is false when the distro
// couldn't be asked.
func WSLTreeTimes(local, marker string) (changed, marked map[string]time.Time, ok bool) {
	d, found := distroAt(local)
	if !found {
		return nil, nil, false
	}
	out, err := wslExec(d.Name, "sh", "-c", wslTimesScript, "sh", d.native(local), marker)
	if err != nil {
		return nil, nil, false
	}
	changed, marked = map[string]time.Time{}, map[string]time.Time{}
	for _, line := range strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n") {
		f := strings.Split(line, "\t")
		into := changed
		if len(f) == 3 && f[0] == "M" {
			into, f = marked, f[1:]
		}
		if len(f) != 2 || f[0] == "" {
			continue
		}
		at, good := unixTime(f[1])
		if !good {
			continue
		}
		name, _, _ := strings.Cut(f[0], "/")
		if into[name].Before(at) {
			into[name] = at
		}
	}
	return changed, marked, true
}

// unixTime reads find's %T@: seconds, a dot and the fraction.
func unixTime(s string) (time.Time, bool) {
	sec, frac, _ := strings.Cut(s, ".")
	n, err := strconv.ParseInt(sec, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	var ns int64
	if frac != "" {
		frac = (frac + "000000000")[:9]
		if ns, err = strconv.ParseInt(frac, 10, 64); err != nil {
			return time.Time{}, false
		}
	}
	return time.Unix(n, ns), true
}
