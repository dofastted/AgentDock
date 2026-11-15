package library

// AgentDock: a tidy-up of the library across this computer and each WSL
// distro, from what one machine's setup went through (11-15): the faults
// a library shared by Windows and WSL agents falls into on its own, found
// in one look and fixed in one go, archived first like a clean start
// (reset.go) and restored the same way.
//
//   - absorb: a library skill whose folder is in a distro (\\wsl.localhost):
//     every Windows agent then reads it across the share and each sync
//     goes through wsl.exe for it. Its files move into the library's own
//     folder (moveIn: copied, checked, the original set aside), and the
//     distro's agents get magpie's store copy as for any other skill.
//   - mcp-env: a server ticked for an agent that can't start it there
//     (side refuses it: a Linux program for a Windows agent, a Windows
//     one for a WSL agent, with no variant for that environment): the
//     tick is taken away; a variant added later can tick it again.
//   - broken-link: a link in an agent's skills folder, or in an
//     environment's ~/.agents/skills, to nothing (a skill moved or removed
//     elsewhere): the agent lists a skill it can't open. A distro's are
//     found by one find in it, as Windows can't follow its links.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	posix "path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/yetone/magpie/internal/wslrun"
)

// The kinds of a tidy-up's items (ResetItem.Kind).
const (
	doctorAbsorb = "absorb"
	doctorMCPEnv = "mcp-env"
	doctorLink   = "broken-link"
)

// errNothingToTidy is a tidy-up with nothing found.
var errNothingToTidy = errors.New("nothing to tidy up")

// doctorPlan is what a tidy-up would fix, the library and targets held.
func (l *Library) doctorPlan(ts []*Target) ResetPlan {
	p := ResetPlan{Items: []ResetItem{}, Files: []string{}, Problems: []Problem{}}
	// skills kept in a distro
	for _, s := range l.Skills {
		src := s.Source
		if src == nil || src.Kind != "folder" || linuxHome(src.Dir) == "" || !linked(skillDir(s.Name)) {
			continue
		}
		d := wslDistroOf(src.Dir)
		if d == "" || !wslrun.Up(d) {
			p.Problems = append(p.Problems, Problem{Agent: "library", What: "skill:" + s.Name, Error: "its folder is in WSL " + d + ", which isn't running"})
			continue
		}
		p.Items = append(p.Items, ResetItem{Agent: "library", Env: "wsl:" + d, Kind: doctorAbsorb, Name: s.Name, Path: src.Dir,
			Why: "kept in WSL: Windows agents read it across \\\\wsl.localhost"})
	}
	// servers ticked for an agent that can't start them
	byID := map[string]*Target{}
	for _, t := range ts {
		byID[t.Agent.ID] = t
	}
	for _, s := range l.MCP {
		if s.Hidden {
			continue
		}
		for _, id := range s.Agents {
			t := byID[id]
			if t == nil || t.MCP == nil {
				continue
			}
			if _, err := t.MCP.side(s); err != nil {
				p.Items = append(p.Items, ResetItem{Agent: id, Env: envOf(t), Kind: doctorMCPEnv, Name: s.Name, Path: t.MCP.Path, Why: err.Error()})
			}
		}
	}
	p.Items = append(p.Items, brokenLinks(ts)...)
	return p
}

// wslDistroOf is the distro a path as magpie opens it is in.
func wslDistroOf(local string) string {
	for _, pre := range []string{`\\wsl.localhost\`, `\\wsl$\`} {
		if len(local) > len(pre) && strings.EqualFold(local[:len(pre)], pre) {
			d, _, _ := strings.Cut(local[len(pre):], `\`)
			return d
		}
	}
	return ""
}

// brokenLinks are the links to nothing in the agents' skills folders and
// each environment's ~/.agents/skills.
func brokenLinks(ts []*Target) []ResetItem {
	type place struct{ agent, env, dir string }
	var win []place
	wsl := map[string][]place{} // by distro
	seen := map[string]bool{}
	add := func(agent, env, dir string, distro string) {
		k := strings.ToLower(filepath.Clean(dir))
		if dir == "" || seen[k] {
			return
		}
		seen[k] = true
		if distro != "" {
			wsl[distro] = append(wsl[distro], place{agent, env, dir})
		} else {
			win = append(win, place{agent, env, dir})
		}
	}
	for _, t := range ts {
		if t.Desktop != nil || t.Gone {
			continue
		}
		env := envOf(t)
		add(t.Agent.ID, env, t.Skills, t.Agent.WSL)
		if t.readsSharedDir() {
			add("shared:"+env, env, SharedSkillsDirOf(t.Agent), t.Agent.WSL)
		}
	}
	var out []ResetItem
	for _, pl := range win {
		es, err := os.ReadDir(pl.dir)
		if err != nil {
			continue
		}
		for _, e := range es {
			q := filepath.Join(pl.dir, e.Name())
			to, ok := linkTarget(q)
			if !ok {
				continue
			}
			if _, err := os.Stat(q); errors.Is(err, fs.ErrNotExist) {
				out = append(out, ResetItem{Agent: pl.agent, Env: pl.env, Kind: doctorLink, Name: e.Name(), Path: q, Link: to, Why: "leads to nothing"})
			}
		}
	}
	distros := make([]string, 0, len(wsl))
	for d := range wsl {
		distros = append(distros, d)
	}
	sort.Strings(distros)
	for _, d := range distros {
		if !wslrun.On || !wslrun.Up(d) {
			continue
		}
		local := map[string]place{}
		args := []string{"-d", d, "--exec", "find"}
		for _, pl := range wsl[d] {
			lx := linuxHome(pl.dir)
			if lx == "" {
				continue
			}
			local[lx] = pl
			args = append(args, lx)
		}
		if len(local) == 0 {
			continue
		}
		args = append(args, "-mindepth", "1", "-maxdepth", "1", "-xtype", "l", "-printf", `%p\t%l\n`)
		// a folder that isn't there makes find fail, the others still listed
		b, _ := wslrun.Run(time.Minute, args...)
		for _, line := range strings.Split(strings.ReplaceAll(string(b), "\r", ""), "\n") {
			lx, to, _ := strings.Cut(line, "\t")
			pl, ok := local[posix.Dir(lx)]
			if lx == "" || !ok {
				continue
			}
			name := posix.Base(lx)
			out = append(out, ResetItem{Agent: pl.agent, Env: pl.env, Kind: doctorLink, Name: name, Path: filepath.Join(pl.dir, name), Link: to, Why: "leads to nothing"})
		}
	}
	return out
}

// DoctorPreview is what Doctor would fix; nothing is written.
func DoctorPreview() (ResetPlan, error) {
	ts := Targets()
	mu.Lock()
	defer mu.Unlock()
	l, err := load()
	if err != nil {
		return ResetPlan{}, err
	}
	return l.doctorPlan(ts), nil
}

// Doctor archives what DoctorPreview lists (RestoreReset puts it back),
// fixes it, and syncs the agents. Nothing is fixed unless the archive was
// made; one item that can't be fixed is in the result's problems, and the
// others are fixed all the same.
func Doctor() (*ResetRecord, *Result, error) {
	var rec *ResetRecord
	var probs []Problem
	res, err := change(func(l *Library) error {
		p := l.doctorPlan(Targets())
		probs = p.Problems
		if len(p.Items) == 0 {
			return errNothingToTidy
		}
		id := time.Now().Format("20060102-150405")
		dir := filepath.Join(resetsDir(), id)
		if err := os.MkdirAll(filepath.Join(dir, "files"), 0o700); err != nil {
			return err
		}
		rec = &ResetRecord{ID: id, At: time.Now().Format(time.RFC3339), Agents: []string{}, Items: p.Items,
			Files: []string{}, Archives: map[string]string{}, Doctor: true}
		if b, err := os.ReadFile(path()); err == nil {
			if err := os.WriteFile(filepath.Join(dir, "library.json"), b, 0o600); err != nil {
				return err
			}
		}
		if err := archiveItems(rec, dir); err != nil {
			rec = nil
			return err
		}
		fail := func(it ResetItem, err error) {
			probs = append(probs, Problem{Agent: it.Agent, What: it.Kind + ":" + it.Name, Error: err.Error()})
		}
		wslGone := map[string][]string{}
		for _, it := range p.Items {
			switch it.Kind {
			case doctorMCPEnv:
				if s := l.server(it.Name); s != nil {
					s.Agents = slices.DeleteFunc(slices.Clone(s.Agents), func(a string) bool { return a == it.Agent })
				}
			case doctorAbsorb:
				left, err := moveIn(it.Path, skillDir(it.Name))
				if err != nil {
					fail(it, err)
					continue
				}
				if left != "" {
					fail(it, errors.New(left))
				}
				if s := l.skill(it.Name); s != nil {
					s.Source = nil
				}
			case doctorLink:
				if d, ok := strings.CutPrefix(it.Env, "wsl:"); ok {
					wslGone[d] = append(wslGone[d], linuxHome(it.Path))
				} else if err := os.Remove(it.Path); err != nil {
					fail(it, err)
				}
			}
		}
		for d, ps := range wslGone {
			if out, err := wslrun.Run(time.Minute, append([]string{"-d", d, "--exec", "rm", "-f", "--"}, ps...)...); err != nil {
				probs = append(probs, Problem{Agent: "wsl:" + d, What: doctorLink, Error: fmt.Sprintf("%v %s", err, strings.TrimSpace(string(out)))})
			}
		}
		return nil
	})
	if res != nil {
		res.Problems = append(probs, res.Problems...)
	}
	return rec, res, err
}

// relinkAbsorbed, as a tidy-up is restored, makes the library's folder of
// each skill it took in from a distro a link to it there again.
func relinkAbsorbed(r *ResetRecord) []error {
	var errs []error
	for _, it := range r.Items {
		if it.Kind != doctorAbsorb {
			continue
		}
		lib := skillDir(it.Name)
		if linked(lib) {
			continue
		}
		if err := os.RemoveAll(lib); err != nil {
			errs = append(errs, err)
			continue
		}
		if err := dirLink(it.Path, lib); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", it.Name, err))
		}
	}
	return errs
}
