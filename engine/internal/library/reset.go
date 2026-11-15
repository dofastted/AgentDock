package library

// AgentDock: a clean start for the agents the user picks. Everything in
// their folders that the library didn't put there — skills of their own
// (folders, links to the retired ~/.agents/skills), MCP entries, subagent
// files, and on request their own instructions text — is archived, then
// taken out, so the library alone sets them up again. Each environment's
// ~/.agents/skills, which several agents read besides their own folder, is
// cleaned the same way. An agent's own app's servers (Codex's node_repl),
// magpie's skill router and dot-folders (.system) stay.
//
// The archive is one folder under backups/resets/<id>: library.json as it
// was, every config file touched as it was, a tar.gz of this computer's
// items, and one of each distro's, made inside the distro by its own tar
// so links stay links (a copy of it is also kept in the distro, in
// ~/agentdock-archive). RestoreReset puts all of it back as it was.

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/yetone/magpie/internal/wslrun"
)

// ResetItem is one thing a reset archives and takes out.
type ResetItem struct {
	Agent string `json:"agent"`          // its agent, or shared:<env> for ~/.agents/skills
	Env   string `json:"env"`            // windows, or wsl:<distro>
	Kind  string `json:"kind"`           // skill, mcp, subagent, instructions
	Name  string `json:"name"`           // the skill, server or subagent; "" for instructions
	Path  string `json:"path"`           // where it is, as magpie opens it
	Link  string `json:"link,omitempty"` // where a link leads
	// Why is, for a tidy-up's item (doctor.go), what is wrong with it
	Why string `json:"why,omitempty"`
}

// ResetPlan is what a reset would take out; nothing is written.
type ResetPlan struct {
	Items []ResetItem `json:"items"`
	// Files are the config files archived whole before an entry in them
	// (an MCP server, the instructions) is taken out
	Files    []string  `json:"files"`
	Problems []Problem `json:"problems"`
}

// ResetRecord is a reset done: what was archived, and where.
type ResetRecord struct {
	ID           string      `json:"id"`
	At           string      `json:"at"`
	Agents       []string    `json:"agents"`
	Instructions bool        `json:"instructions"`
	Items        []ResetItem `json:"items"`
	Files        []string    `json:"files"`
	// Archives are the tar.gz of each environment's items, by env
	Archives map[string]string `json:"archives"`
	Restored string            `json:"restored,omitempty"`
	// Doctor: a tidy-up (doctor.go) made it, not a clean start
	Doctor bool `json:"doctor,omitempty"`
}

func resetsDir() string { return filepath.Join(BackupDir(), "resets") }

// envOf is a target's environment as a reset names it.
func envOf(t *Target) string {
	if t.Agent.WSL != "" {
		return "wsl:" + t.Agent.WSL
	}
	return "windows"
}

// keepName is a skill folder entry a reset never takes out: dot-folders
// (.system), magpie's router, and _-folders the skills share (_shared's
// mermaid-utils.md), which are no skill of their own.
func keepName(n string) bool {
	return strings.HasPrefix(n, ".") || strings.HasPrefix(n, "_") || n == RouterSkill || n == marker
}

func resetTargets(agents []string) ([]*Target, []Problem) {
	var out []*Target
	var probs []Problem
	all := Targets()
	for _, id := range agents {
		i := slices.IndexFunc(all, func(t *Target) bool { return t.Agent.ID == id })
		switch {
		case i < 0:
			probs = append(probs, Problem{Agent: id, What: "reset", Error: "not found here, or its distro isn't running"})
		case all[i].Unreached != nil:
			probs = append(probs, Problem{Agent: id, What: "reset", Error: all[i].Unreached.Error()})
		case len(all[i].Desktop) > 0:
			probs = append(probs, Problem{Agent: id, What: "reset", Error: "Claude Desktop's skills are its app's own: left as they are"})
		default:
			out = append(out, all[i])
		}
	}
	return out, probs
}

// plan is what resetting the targets takes out; the library held.
func (l *Library) resetPlan(ts []*Target, instructions bool) ResetPlan {
	p := ResetPlan{Items: []ResetItem{}, Files: []string{}, Problems: []Problem{}}
	seen := map[string]bool{}
	add := func(it ResetItem) {
		k := strings.ToLower(filepath.Clean(it.Path)) + "\x00" + it.Kind + "\x00" + it.Name
		if seen[k] {
			return
		}
		seen[k] = true
		if to, ok := linkTarget(it.Path); ok {
			it.Link = to
		}
		p.Items = append(p.Items, it)
	}
	file := func(f string) {
		if _, err := os.Stat(f); err == nil && !slices.Contains(p.Files, f) {
			p.Files = append(p.Files, f)
		}
	}
	skillsIn := func(dir, agent, env string) {
		es, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range es {
			n, q := e.Name(), filepath.Join(dir, e.Name())
			if keepName(n) || ours(q, n) || realDir(q) == realDir(skillDir(n)) {
				continue
			}
			// a file beside the skills (INDEX.md) is no skill
			if _, isLink := linkTarget(q); !isLink && !e.IsDir() {
				continue
			}
			add(ResetItem{Agent: agent, Env: env, Kind: "skill", Name: n, Path: q})
		}
	}
	for _, t := range ts {
		id, env := t.Agent.ID, envOf(t)
		if t.Skills != "" {
			skillsIn(t.Skills, id, env)
		}
		if t.readsSharedDir() {
			skillsIn(SharedSkillsDirOf(t.Agent), "shared:"+env, env)
		}
		a := l.applied(id)
		if t.MCP != nil {
			have, err := t.MCP.read()
			if err != nil {
				p.Problems = append(p.Problems, Problem{Agent: id, What: "mcp", Error: err.Error()})
			}
			var names []string
			for n, s := range have {
				if n == HubEntry || slices.Contains(a.MCP, n) || appOwned(s, t.Agent) {
					continue
				}
				names = append(names, n)
			}
			sort.Strings(names)
			for _, n := range names {
				add(ResetItem{Agent: id, Env: env, Kind: "mcp", Name: n, Path: t.MCP.Path})
			}
			if len(names) > 0 {
				for _, f := range t.MCP.files() {
					file(f)
				}
			}
		}
		if sp, _ := subagentSpecOf(t); sp != nil && sp.dir != "" {
			es, _ := os.ReadDir(sp.dir)
			for _, e := range es {
				q := filepath.Join(sp.dir, e.Name())
				if e.IsDir() || !strings.HasSuffix(e.Name(), sp.ext) ||
					slices.ContainsFunc(a.Subagents, func(f SubagentFile) bool { return strings.EqualFold(filepath.Clean(f.Path), filepath.Clean(q)) }) {
					continue
				}
				add(ResetItem{Agent: id, Env: env, Kind: "subagent", Name: strings.TrimSuffix(e.Name(), sp.ext), Path: q})
			}
		}
		if instructions && t.Instructions != "" && t.Field == "" && !t.Profile {
			if b, err := os.ReadFile(t.Instructions); err == nil && strings.TrimSpace(own(string(b))) != "" {
				add(ResetItem{Agent: id, Env: env, Kind: "instructions", Path: t.Instructions})
				file(t.Instructions)
			}
		}
	}
	return p
}

// ResetPreview is what Reset(agents, instructions) would take out.
func ResetPreview(agents []string, instructions bool) (ResetPlan, error) {
	ts, probs := resetTargets(agents)
	mu.Lock()
	defer mu.Unlock()
	l, err := load()
	if err != nil {
		return ResetPlan{}, err
	}
	p := l.resetPlan(ts, instructions)
	p.Problems = append(probs, p.Problems...)
	return p, nil
}

// Reset archives what ResetPreview lists, takes it out, and gives the
// agents the library's again. Nothing is taken out unless the archive of
// every environment was made.
func Reset(agents []string, instructions bool) (*ResetRecord, *Result, error) {
	ts, probs := resetTargets(agents)
	if len(ts) == 0 {
		return nil, nil, errors.New("no agent to reset here")
	}
	mu.Lock()
	rec, err := resetLocked(ts, agents, instructions)
	mu.Unlock()
	if err != nil {
		return rec, nil, err
	}
	res, err := Sync()
	if res != nil {
		res.Problems = append(probs, res.Problems...)
	}
	return rec, res, err
}

func resetLocked(ts []*Target, agents []string, instructions bool) (*ResetRecord, error) {
	l, err := load()
	if err != nil {
		return nil, err
	}
	p := l.resetPlan(ts, instructions)
	id := time.Now().Format("20060102-150405")
	dir := filepath.Join(resetsDir(), id)
	if err := os.MkdirAll(filepath.Join(dir, "files"), 0o700); err != nil {
		return nil, err
	}
	rec := &ResetRecord{ID: id, At: time.Now().Format(time.RFC3339), Agents: agents, Instructions: instructions,
		Items: p.Items, Files: p.Files, Archives: map[string]string{}}
	// library.json and every whole file, as they are
	if b, err := os.ReadFile(path()); err == nil {
		if err := os.WriteFile(filepath.Join(dir, "library.json"), b, 0o600); err != nil {
			return nil, err
		}
	}
	for i, f := range p.Files {
		if err := copyFile(f, filepath.Join(dir, "files", fmt.Sprint(i))); err != nil {
			return nil, fmt.Errorf("archiving %s: %w", f, err)
		}
	}
	if err := archiveItems(rec, dir); err != nil {
		return nil, err
	}
	// only now, everything archived, take it out
	var errs []error
	byPath := map[string]*Target{}
	for _, t := range ts {
		if t.MCP != nil {
			byPath[t.MCP.Path] = t
		}
	}
	wslGone := map[string][]string{}
	for _, it := range p.Items {
		switch it.Kind {
		case "mcp":
			if t := byPath[it.Path]; t != nil {
				if err := t.MCP.del(it.Name); err != nil {
					errs = append(errs, fmt.Errorf("%s mcp %s: %w", it.Agent, it.Name, err))
				}
			}
		case "instructions":
			b, err := os.ReadFile(it.Path)
			if err == nil {
				err = os.WriteFile(it.Path, []byte(magpiePart(string(b))), 0o644)
			}
			if err != nil {
				errs = append(errs, fmt.Errorf("%s instructions: %w", it.Agent, err))
			}
		default:
			if d, ok := strings.CutPrefix(it.Env, "wsl:"); ok {
				wslGone[d] = append(wslGone[d], linuxHome(it.Path))
			} else if err := removeEntry(it.Path); err != nil {
				errs = append(errs, err)
			}
		}
	}
	for d, ps := range wslGone {
		if out, err := wslrun.Run(5*time.Minute, append([]string{"-d", d, "--exec", "rm", "-rf", "--"}, ps...)...); err != nil {
			errs = append(errs, fmt.Errorf("WSL %s: %v %s", d, err, strings.TrimSpace(string(out))))
		}
	}
	return rec, errors.Join(errs...)
}

// archiveItems puts each environment's folders and files of rec's items
// in one tar.gz (a distro's made by its own tar), then writes the record:
// nothing is taken out before this has worked.
func archiveItems(rec *ResetRecord, dir string) error {
	for env, paths := range itemPaths(rec.Items) {
		var a string
		var err error
		if env == "windows" {
			a = filepath.Join(dir, "windows.tgz")
			err = tarWindows(a, paths)
		} else {
			a, err = tarWSL(strings.TrimPrefix(env, "wsl:"), rec.ID, paths, dir)
		}
		if err != nil {
			return fmt.Errorf("archiving %s: %w", env, err)
		}
		rec.Archives[env] = a
	}
	return writeRecord(rec)
}

// magpiePart is an instructions file with only magpie's part left.
func magpiePart(file string) string {
	i := strings.Index(file, blockBegin)
	if i < 0 {
		return ""
	}
	j := strings.Index(file[i:], blockEnd)
	if j < 0 {
		return ""
	}
	return file[i:i+j+len(blockEnd)] + "\n"
}

// itemPaths are the folders and files to archive, by environment: an MCP
// entry or instructions are in Files, archived whole.
func itemPaths(items []ResetItem) map[string][]string {
	out := map[string][]string{}
	for _, it := range items {
		if it.Kind == "mcp" || it.Kind == "instructions" || it.Kind == doctorMCPEnv || slices.Contains(out[it.Env], it.Path) {
			continue
		}
		out[it.Env] = append(out[it.Env], it.Path)
	}
	return out
}

// removeEntry takes p out: a link (or junction) alone, never what it
// leads to.
func removeEntry(p string) error {
	if _, ok := linkTarget(p); ok {
		return os.Remove(p)
	}
	return os.RemoveAll(p)
}

func copyFile(from, to string) error {
	b, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return os.WriteFile(to, b, 0o600)
}

// ---- this computer's archive: links kept as links ------------------------

// tarName is a Windows path as a name in the archive: C:\x\y is C/x/y.
func tarName(p string) string { return strings.Replace(filepath.ToSlash(p), ":", "", 1) }

// fromTarName is tarName undone.
func fromTarName(n string) string {
	if len(n) > 1 && n[1] == '/' {
		n = n[:1] + ":" + n[1:]
	}
	return filepath.FromSlash(n)
}

func tarWindows(out string, paths []string) error {
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	put := func(p string) error {
		fi, err := os.Lstat(p)
		if err != nil {
			return err
		}
		if to, ok := linkTarget(p); ok {
			return tw.WriteHeader(&tar.Header{Typeflag: tar.TypeSymlink, Name: tarName(p), Linkname: to, Mode: 0o777, ModTime: fi.ModTime()})
		}
		if fi.IsDir() {
			return tw.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: tarName(p) + "/", Mode: 0o755, ModTime: fi.ModTime()})
		}
		if !fi.Mode().IsRegular() {
			return nil
		}
		if err := tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: tarName(p), Size: fi.Size(), Mode: 0o644, ModTime: fi.ModTime()}); err != nil {
			return err
		}
		r, err := os.Open(p)
		if err != nil {
			return err
		}
		defer r.Close()
		_, err = io.Copy(tw, r)
		return err
	}
	for _, root := range paths {
		if _, ok := linkTarget(root); ok {
			if err := put(root); err != nil {
				return err
			}
			continue
		}
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if err := put(p); err != nil {
				return err
			}
			if _, ok := linkTarget(p); ok && p != root && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	return f.Close()
}

func untarWindows(in string) error {
	f, err := os.Open(in)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		p := fromTarName(strings.TrimSuffix(h.Name, "/"))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(p, 0o755); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := dirLink(h.Linkname, p); err != nil {
				return fmt.Errorf("link %s: %w", p, err)
			}
		case tar.TypeReg:
			w, err := os.Create(p)
			if err != nil {
				return err
			}
			_, err = io.Copy(w, tr)
			w.Close()
			if err != nil {
				return err
			}
		}
	}
}

// ---- a distro's archive, made by its own tar -----------------------------

func resetDistroHome(distro string) string {
	for _, t := range Targets() {
		if t.Agent.WSL == distro && t.Agent.Home != "" {
			return linuxHome(t.Agent.Home)
		}
	}
	return ""
}

func tarWSL(distro, id string, paths []string, dir string) (string, error) {
	h := resetDistroHome(distro)
	if h == "" {
		return "", fmt.Errorf("no home known in %s", distro)
	}
	in := h + "/agentdock-archive/reset-" + id + ".tgz"
	if out, err := wslrun.Run(time.Minute, "-d", distro, "--exec", "mkdir", "-p", h+"/agentdock-archive"); err != nil {
		return "", fmt.Errorf("%v %s", err, out)
	}
	args := []string{"-d", distro, "--exec", "tar", "-czf", in, "-C", "/"}
	for _, p := range paths {
		args = append(args, strings.TrimPrefix(linuxHome(p), "/"))
	}
	if out, err := wslrun.Run(5*time.Minute, args...); err != nil {
		return "", fmt.Errorf("%v %s", err, strings.TrimSpace(string(out)))
	}
	// a copy beside the rest of the reset, read through \\wsl.localhost
	local := filepath.Join(dir, "wsl-"+distro+".tgz")
	if err := copyFile(`\\wsl.localhost\`+distro+filepath.FromSlash(in), local); err != nil {
		return "", err
	}
	return in, nil
}

// ---- the resets kept, and putting one back --------------------------------

func writeRecord(r *ResetRecord) error {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(resetsDir(), r.ID, "reset.json"), b, 0o600)
}

// Resets are the resets kept, newest first.
func Resets() ([]ResetRecord, error) {
	es, err := os.ReadDir(resetsDir())
	if errors.Is(err, fs.ErrNotExist) {
		return []ResetRecord{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []ResetRecord{}
	for _, e := range es {
		b, err := os.ReadFile(filepath.Join(resetsDir(), e.Name(), "reset.json"))
		if err != nil {
			continue
		}
		var r ResetRecord
		if json.Unmarshal(b, &r) == nil {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

// RestoreReset puts back what reset id took out, and the library as it
// was then: what was set up since is undone.
func RestoreReset(id string) (*Result, error) {
	if strings.ContainsAny(id, `/\.`) || id == "" {
		return nil, fmt.Errorf("no reset %q", id)
	}
	dir := filepath.Join(resetsDir(), id)
	b, err := os.ReadFile(filepath.Join(dir, "reset.json"))
	if err != nil {
		return nil, fmt.Errorf("no reset %s: %w", id, err)
	}
	var r ResetRecord
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	if err := restoreLocked(&r, dir); err != nil {
		return nil, err
	}
	r.Restored = time.Now().Format(time.RFC3339)
	_ = writeRecord(&r)
	return Sync()
}

func restoreLocked(r *ResetRecord, dir string) error {
	mu.Lock()
	defer mu.Unlock()
	var errs []error
	// what stands where an item was (the library's link given since) goes
	wslClear := map[string][]string{}
	for env, paths := range itemPaths(r.Items) {
		for _, p := range paths {
			if d, ok := strings.CutPrefix(env, "wsl:"); ok {
				wslClear[d] = append(wslClear[d], linuxHome(p))
			} else if _, err := os.Lstat(p); err == nil {
				if err := removeEntry(p); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}
	for env, a := range r.Archives {
		if env == "windows" {
			if err := untarWindows(a); err != nil {
				errs = append(errs, fmt.Errorf("windows: %w", err))
			}
			continue
		}
		d := strings.TrimPrefix(env, "wsl:")
		if ps := wslClear[d]; len(ps) > 0 {
			if out, err := wslrun.Run(5*time.Minute, append([]string{"-d", d, "--exec", "rm", "-rf", "--"}, ps...)...); err != nil {
				errs = append(errs, fmt.Errorf("WSL %s: %v %s", d, err, out))
			}
		}
		if out, err := wslrun.Run(5*time.Minute, "-d", d, "--exec", "tar", "-xzf", a, "-C", "/"); err != nil {
			errs = append(errs, fmt.Errorf("WSL %s: %v %s", d, err, strings.TrimSpace(string(out))))
		}
	}
	for i, f := range r.Files {
		if err := copyFile(filepath.Join(dir, "files", fmt.Sprint(i)), f); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", f, err))
		}
	}
	if lb, err := os.ReadFile(filepath.Join(dir, "library.json")); err == nil {
		if err := os.WriteFile(path(), lb, 0o600); err != nil {
			errs = append(errs, err)
		}
	}
	errs = append(errs, relinkAbsorbed(r)...)
	return errors.Join(errs...)
}
