package environments

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/yetone/magpie/internal/agent"
	"github.com/yetone/magpie/internal/provider"
)

// First-run import of a magpie a WSL distro ran on its own: the providers
// and routing groups of its providers.json, merged into this one's. Its
// file is only read. A provider already here under the same id and the
// same settings is the same one; one that differs comes in as <id>-wsl, so
// nothing here is ever written over. A group comes in only when every
// provider it names came in under its own id, its id is free, and it
// names no group that didn't.

// MagpieSource is a distro's own magpie providers.json.
type MagpieSource struct {
	Distro    string   `json:"distro"`
	Path      string   `json:"path"`
	Providers []string `json:"providers"`
	Groups    []string `json:"groups"`
	Error     string   `json:"error,omitempty"`
}

// ImportResult is what one import brought in, and what it left out.
type ImportResult struct {
	Added   []string          `json:"added"`
	Renamed map[string]string `json:"renamed"`
	Same    []string          `json:"same"`
	Groups  []string          `json:"groups"`
	Skipped map[string]string `json:"skipped"`
}

type magpieFile struct {
	Providers []provider.Provider `json:"providers"`
	Groups    []provider.Group    `json:"groups"`
}

const importSuffix = "-wsl"

// magpieFileIn is where a distro's own magpie keeps its providers, under
// home as magpie opens it: the default, its XDG_CONFIG_HOME not read.
func magpieFileIn(home string) string {
	return filepath.Join(home, ".config", "magpie", "providers.json")
}

// MagpieSources are the running distros that have a magpie of their own's
// providers.json; none is started.
func MagpieSources() []MagpieSource {
	envs, _ := agent.WSLEnvs()
	var out []MagpieSource
	for _, e := range envs {
		if e.Home == "" {
			continue
		}
		p := magpieFileIn(e.Home)
		if _, err := os.Stat(p); err != nil {
			continue
		}
		s := MagpieSource{Distro: e.Distro, Path: p, Providers: []string{}, Groups: []string{}}
		f, err := readMagpieFile(p)
		if err != nil {
			s.Error = err.Error()
		}
		for _, x := range f.Providers {
			s.Providers = append(s.Providers, x.ID)
		}
		for _, g := range f.Groups {
			s.Groups = append(s.Groups, g.ID)
		}
		out = append(out, s)
	}
	return out
}

func readMagpieFile(p string) (magpieFile, error) {
	var f magpieFile
	b, err := os.ReadFile(p)
	if err != nil {
		return f, err
	}
	if err := json.Unmarshal(bytes.TrimPrefix(b, []byte("\xef\xbb\xbf")), &f); err != nil {
		return magpieFile{}, fmt.Errorf("%s can't be read: %w", p, err)
	}
	return f, nil
}

// ImportMagpie merges distro's own magpie's providers and groups into
// this magpie's.
func ImportMagpie(distro string) (ImportResult, error) {
	for _, s := range MagpieSources() {
		if strings.EqualFold(s.Distro, distro) {
			if s.Error != "" {
				return ImportResult{}, errors.New(s.Error)
			}
			return importFrom(s.Path)
		}
	}
	return ImportResult{}, fmt.Errorf("no magpie of its own found in running WSL %s", distro)
}

func importFrom(path string) (ImportResult, error) {
	src, err := readMagpieFile(path)
	if err != nil {
		return ImportResult{}, err
	}
	// an unreadable providers.json here is never taken as none and written over
	have, err := provider.Stored()
	if err != nil {
		return ImportResult{}, err
	}
	byID := map[string]provider.Provider{}
	for _, p := range have {
		byID[p.ID] = p
	}
	res := ImportResult{Added: []string{}, Renamed: map[string]string{}, Same: []string{}, Groups: []string{}, Skipped: map[string]string{}}
	kept := map[string]bool{} // providers here under the id the source gave them
	for _, p := range src.Providers {
		if p.ID == "" || p.Account != nil {
			continue
		}
		if cur, ok := byID[p.ID]; ok {
			if sameProvider(cur, p) {
				res.Same = append(res.Same, p.ID)
				kept[p.ID] = true
				continue
			}
			alt := p.ID + importSuffix
			if cur, ok := byID[alt]; ok {
				if sameProvider(cur, withID(p, alt)) {
					res.Same = append(res.Same, alt)
				} else {
					res.Skipped[p.ID] = alt + " is here already, set otherwise"
				}
				continue
			}
			if err := provider.Save(withID(p, alt)); err != nil {
				res.Skipped[p.ID] = err.Error()
				continue
			}
			byID[alt] = p
			res.Renamed[p.ID] = alt
			continue
		}
		if err := provider.Save(p); err != nil {
			res.Skipped[p.ID] = err.Error()
			continue
		}
		byID[p.ID] = p
		kept[p.ID] = true
		res.Added = append(res.Added, p.ID)
	}
	groups := map[string]bool{}
	for _, g := range provider.Groups() {
		groups[g.ID] = true
	}
	srcGroups := map[string]bool{}
	for _, g := range src.Groups {
		srcGroups[g.ID] = true
	}
	for _, g := range src.Groups {
		if groups[g.ID] {
			res.Skipped["group/"+g.ID] = "a group by that id is here already"
			continue
		}
		if why := groupBlocked(g, kept, srcGroups, groups); why != "" {
			res.Skipped["group/"+g.ID] = why
			continue
		}
		if err := provider.SaveGroup(g); err != nil {
			res.Skipped["group/"+g.ID] = err.Error()
			continue
		}
		groups[g.ID] = true
		res.Groups = append(res.Groups, g.ID)
	}
	slices.Sort(res.Added)
	slices.Sort(res.Same)
	return res, nil
}

// groupBlocked says why g can't come in as it is, or "".
func groupBlocked(g provider.Group, kept, srcGroups, groups map[string]bool) string {
	for _, m := range append(append([]string{}, g.Members...), g.Classifier) {
		if m == "" {
			continue
		}
		if id, ok := strings.CutPrefix(m, "group/"); ok {
			if !srcGroups[id] && !groups[id] {
				return "it names group " + id + ", which isn't here"
			}
			continue
		}
		pid, _, _ := strings.Cut(m, "/")
		if !kept[pid] {
			return "its provider " + pid + " didn't come in under that id"
		}
	}
	return ""
}

func withID(p provider.Provider, id string) provider.Provider {
	p.ID = id
	return p
}

// sameProvider is whether two providers are set alike, the id aside.
func sameProvider(a, b provider.Provider) bool {
	a.ID, b.ID = "", ""
	a.Account, b.Account = nil, nil
	return reflect.DeepEqual(a, b)
}
