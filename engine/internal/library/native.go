package library

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/yetone/magpie/internal/agent"
	"github.com/yetone/magpie/internal/settings"
)

const nativeMaxBytes = 1024 * 1024
const nativeMaxItems = 4000
const nativeMaxPerRoot = 80

// ErrNativeConflict means the file changed after an editor or preview read it.
var ErrNativeConflict = errors.New("the native file changed; reload it before saving")

// NativeRoot is a CLI's own resource location, not a library import destination.
// Paths are display-only: mutations resolve an opaque root id again on the server.
type NativeRoot struct {
	ID          string `json:"id"`
	Environment string `json:"environment"`
	Agent       string `json:"agent"`
	CLI         string `json:"cli"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Path        string `json:"path"`
	Error       string `json:"error,omitempty"`
	directory   bool
	distro      string
	mcp         *mcpFile
}

// NativeItem keeps same-named files in different environments independently editable.
type NativeItem struct {
	ID          string `json:"id"`
	Root        string `json:"root"`
	Environment string `json:"environment"`
	Agent       string `json:"agent"`
	CLI         string `json:"cli"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	Version     string `json:"version"`
}

type NativeInventory struct {
	Roots    []NativeRoot `json:"roots"`
	Items    []NativeItem `json:"items"`
	Problems []Problem    `json:"problems"`
}

type NativeDocument struct {
	Item    NativeItem `json:"item"`
	Text    string     `json:"text"`
	Version string     `json:"version"`
}

type NativeChange struct {
	ID      string `json:"id"`
	Root    string `json:"root"`
	Name    string `json:"name"`
	Text    string `json:"text"`
	Version string `json:"version"`
	Remove  bool   `json:"remove"`
}

// Native operations serialize their own compare/write sequence. External editors
// are checked again immediately before replacement by the platform writer.
var nativeChanges sync.Mutex

var nativeRoots = nativeRootsForAgents

func nativeDigest(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func nativeRootsForAgents() []NativeRoot {
	out := []NativeRoot{}
	seen := map[string]bool{}
	for _, a := range append(agent.Detected(), apps()...) {
		if seen[a.ID] || !a.Detected() {
			continue
		}
		seen[a.ID] = true
		cli, _, _ := strings.Cut(a.ID, "@")
		cli, _, _ = strings.Cut(cli, "#")
		env := runtime.GOOS
		if a.WSL != "" {
			env = "wsl:" + a.WSL
		}
		add := func(kind, p string, directory bool, reason string) {
			r := NativeRoot{Environment: env, Agent: a.ID, CLI: cli, Name: a.Name, Kind: kind, Path: p, Error: reason, directory: directory, distro: a.WSL}
			r.ID = nativeDigest([]byte(env + "\x00" + a.ID + "\x00" + kind + "\x00" + p))
			out = append(out, r)
		}
		if a.WSL != "" && (!agent.WSLRunning(a.WSL) || settings.Load().WSLDistroOff(a.WSL)) {
			for _, kind := range []string{"skill", "prompt", "agent", "instructions", "mcp"} {
				add(kind, "", false, "distribution is stopped or disabled; no files were opened")
			}
			continue
		}
		t := targetOf(a)
		if t == nil {
			for _, kind := range []string{"skill", "prompt", "agent", "instructions", "mcp"} {
				add(kind, "", false, "this CLI has no known native location for "+kind)
			}
			continue
		}
		location := func(kind, p string, directory bool) {
			reason := ""
			if p == "" {
				reason = "this CLI has no known native location for " + kind
			}
			add(kind, p, directory, reason)
		}
		location("skill", t.Skills, true)
		if ReadsSharedSkills(a.ID) {
			shared := SharedSkillsDirOf(a)
			if filepath.Clean(shared) != filepath.Clean(t.Skills) {
				add("skill", shared, true, "")
			}
		}
		location("instructions", t.Instructions, false)
		if t.Override != "" {
			add("instructions", t.Override, false, "")
		}
		mcp := ""
		if t.MCP != nil {
			mcp = t.MCP.Path
		}
		location("mcp", mcp, false)
		out[len(out)-1].mcp = t.MCP
		sp, reason := subagentSpecOf(t)
		if sp != nil {
			add("agent", sp.dir, true, "")
		} else if cli == "omp" && a.Dir != "" {
			// Native registries are editable even when the central subagent
			// renderer cannot deploy that CLI's agent-definition format.
			add("agent", filepath.Join(a.Dir, "agents"), true, "")
		} else {
			add("agent", "", true, reason)
		}
		prompt := ""
		d := a.Dir
		if t.Instructions != "" {
			d = filepath.Dir(t.Instructions)
		}
		switch cli {
		case "codex", "pi", "omp", "omo":
			prompt = filepath.Join(d, "prompts")
		case "claude", "gemini", "qwen":
			prompt = filepath.Join(d, "commands")
		case "opencode":
			prompt = filepath.Join(filepath.Dir(a.Path), "commands")
		}
		location("prompt", prompt, true)
	}
	return out
}

func nativeRootByID(id string) (NativeRoot, error) {
	for _, r := range nativeRoots() {
		if r.ID == id {
			if r.Error != "" {
				return r, errors.New(r.Error)
			}
			if r.distro != "" && (!agent.WSLRunning(r.distro) || settings.Load().NoWSLAgents || settings.Load().WSLDistroOff(r.distro)) {
				return r, errors.New("distribution is stopped or disabled")
			}
			return r, nil
		}
	}
	return NativeRoot{}, errors.New("native location is no longer available; refresh discovery")
}

func nativeName(r NativeRoot, name string) (string, error) {
	if !r.directory {
		if name != "" && name != filepath.Base(r.Path) {
			return "", errors.New("this location names one configuration file")
		}
		return filepath.Base(r.Path), nil
	}
	if name == "" || strings.ContainsAny(name, "\\\x00\r\n") || pathpkg.IsAbs(name) || pathpkg.Clean(name) != name || name == "." {
		return "", errors.New("use a relative native resource filename")
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." || strings.HasPrefix(part, ".") || strings.Contains(part, ":") {
			return "", errors.New("hidden files and parent paths are not native resource names")
		}
	}
	if !nativeTextFile(name) {
		return "", errors.New("native resources must be text, Markdown, TOML, JSON, YAML or script files")
	}
	return name, nil
}

func nativeTextFile(name string) bool {
	switch strings.ToLower(pathpkg.Ext(name)) {
	case ".md", ".txt", ".toml", ".json", ".jsonc", ".yaml", ".yml", ".sh", ".py", ".js", ".ts", ".mjs", ".cjs", ".ps1", ".cmd", ".bat":
		return true
	}
	return false
}

func nativeItem(r NativeRoot, name string) NativeItem {
	p := r.Path
	if r.directory {
		p = filepath.Join(p, filepath.FromSlash(name))
	}
	return NativeItem{ID: base64.RawURLEncoding.EncodeToString([]byte(r.ID + "\n" + name)), Root: r.ID, Environment: r.Environment, Agent: r.Agent, CLI: r.CLI, Kind: r.Kind, Name: name, Path: p}
}

func nativeResolve(id string) (NativeRoot, NativeItem, error) {
	b, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return NativeRoot{}, NativeItem{}, errors.New("invalid native resource identity")
	}
	root, name, ok := strings.Cut(string(b), "\n")
	if !ok {
		return NativeRoot{}, NativeItem{}, errors.New("invalid native resource identity")
	}
	r, err := nativeRootByID(root)
	if err != nil {
		return r, NativeItem{}, err
	}
	name, err = nativeName(r, name)
	return r, nativeItem(r, name), err
}

// ReadNative enumerates files in place. It never imports, moves or synchronizes them.
func ReadNative() NativeInventory {
	v := NativeInventory{Roots: nativeRoots(), Items: []NativeItem{}, Problems: []Problem{}}
	type scan struct {
		names []string
		err   error
	}
	got := make([]scan, len(v.Roots))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i, r := range v.Roots {
		if r.Error != "" {
			continue
		}
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			got[i].names, got[i].err = nativeList(r)
		}()
	}
	wg.Wait()
	type bucket struct {
		root  NativeRoot
		names []string
	}
	var buckets []bucket
	for i, r := range v.Roots {
		if got[i].err != nil {
			v.Roots[i].Error = got[i].err.Error()
			v.Problems = append(v.Problems, Problem{Agent: r.Agent, What: r.Kind, Error: got[i].err.Error()})
		}
		names := nativePrefer(got[i].names)
		if len(names) > nativeMaxPerRoot {
			names = names[:nativeMaxPerRoot]
			v.Problems = append(v.Problems, Problem{Agent: r.Agent, What: r.Kind, Error: "this location has more native files than the inventory shows; open its path for the rest"})
		}
		if len(names) > 0 {
			buckets = append(buckets, bucket{root: r, names: names})
		}
	}
	// One file from each location before deeper files, so one large Windows
	// skill tree cannot hide WSL.
	next := make([]int, len(buckets))
	for len(v.Items) < nativeMaxItems {
		added := false
		for i := range buckets {
			if next[i] >= len(buckets[i].names) {
				continue
			}
			name := buckets[i].names[next[i]]
			next[i]++
			if _, err := nativeName(buckets[i].root, name); err != nil {
				continue
			}
			v.Items = append(v.Items, nativeItem(buckets[i].root, name))
			added = true
			if len(v.Items) == nativeMaxItems {
				break
			}
		}
		if !added {
			break
		}
	}
	sort.Slice(v.Items, func(i, j int) bool {
		a, b := v.Items[i], v.Items[j]
		if a.Environment != b.Environment {
			return a.Environment < b.Environment
		}
		if a.Agent != b.Agent {
			return a.Agent < b.Agent
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Name < b.Name
	})
	return v
}
func nativeSkipEntry(name string) bool {
	switch name {
	case "node_modules", "blobs", "sessions", "cache", "backups", "__pycache__", "vendor", "dist", "skills._archive", "agents-ccw-archive":
		return true
	}
	return strings.HasPrefix(name, ".")
}

func nativePrefer(names []string) []string {
	out := append([]string(nil), names...)
	sort.SliceStable(out, func(i, j int) bool {
		return nativeRank(out[i]) < nativeRank(out[j])
	})
	return out
}

func nativeRank(name string) string {
	base := strings.ToLower(pathpkg.Base(name))
	pri := "9"
	switch {
	case base == "skill.md":
		pri = "0"
	case base == "agents.md" || base == "claude.md":
		pri = "1"
	case strings.HasSuffix(base, ".md") || strings.HasSuffix(base, ".toml"):
		pri = "2"
	}
	return pri + "/" + name
}

func nativeList(r NativeRoot) ([]string, error) {
	if r.distro != "" {
		return nativeWSLList(r)
	}
	if !r.directory {
		st, err := os.Stat(r.Path)
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if !st.Mode().IsRegular() {
			return nil, errors.New("configuration path is not a regular file")
		}
		return []string{filepath.Base(r.Path)}, nil
	}
	var names []string
	var walk func(string, string, int) error
	visited := map[string]bool{}
	walk = func(dir, rel string, depth int) error {
		if depth > 8 {
			return nil
		}
		real, err := filepath.EvalSymlinks(dir)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if visited[real] {
			return nil
		}
		visited[real] = true
		es, err := os.ReadDir(real)
		if err != nil {
			return err
		}
		for _, e := range es {
			if nativeSkipEntry(e.Name()) {
				continue
			}
			name := pathpkg.Join(rel, e.Name())
			p := filepath.Join(real, e.Name())
			st, err := os.Stat(p)
			if err != nil {
				return fmt.Errorf("read %s: %w", name, err)
			}
			if st.IsDir() {
				if err := walk(p, name, depth+1); err != nil {
					return err
				}
			} else if st.Mode().IsRegular() && st.Size() <= nativeMaxBytes && nativeTextFile(name) {
				names = append(names, name)
				if len(names) >= 2000 {
					return nil
				}
			}
		}
		return nil
	}
	err := walk(r.Path, "", 0)
	return names, err
}

// ReadNativeFile reads the current native bytes rather than a library copy.
func ReadNativeFile(id string) (NativeDocument, error) {
	r, item, err := nativeResolve(id)
	if err != nil {
		return NativeDocument{}, err
	}
	b, exists, err := nativeRead(r, item.Path)
	if err != nil {
		return NativeDocument{}, err
	}
	if !exists {
		return NativeDocument{}, fs.ErrNotExist
	}
	if !utf8.Valid(b) || strings.IndexByte(string(b), 0) >= 0 {
		return NativeDocument{}, errors.New("this resource is not UTF-8 text")
	}
	item.Version = nativeDigest(b)
	return NativeDocument{Item: item, Text: string(b), Version: item.Version}, nil
}

// ChangeNativeFile checks the version before touching the selected file only.
func ChangeNativeFile(in NativeChange) error {
	nativeChanges.Lock()
	defer nativeChanges.Unlock()
	var r NativeRoot
	var item NativeItem
	var err error
	if in.ID != "" {
		r, item, err = nativeResolve(in.ID)
		if in.Version == "" {
			return errors.New("read the native file before editing or removing it")
		}
	} else {
		if in.Remove {
			return errors.New("select an existing native file to remove")
		}
		r, err = nativeRootByID(in.Root)
		if err == nil {
			var name string
			name, err = nativeName(r, in.Name)
			item = nativeItem(r, name)
		}
		in.Version = "missing"
	}
	if err != nil {
		return err
	}
	if len(in.Text) > nativeMaxBytes || !utf8.ValidString(in.Text) || strings.IndexByte(in.Text, 0) >= 0 {
		return errors.New("native text must be valid UTF-8 and at most 1 MiB")
	}
	if err := nativeValidate(item, in.Text, in.Remove); err != nil {
		return err
	}
	return nativeWrite(r, item.Path, []byte(in.Text), in.Version, in.Remove)
}

// NativeRevealable accepts only the roots and text files the native UI can show.
func NativeRevealable(p string) bool {
	for _, r := range nativeRoots() {
		if r.Error != "" || r.Path == "" {
			continue
		}
		if p == r.Path {
			return true
		}
		if !r.directory {
			continue
		}
		rel, err := filepath.Rel(r.Path, p)
		if err != nil {
			continue
		}
		if _, err := nativeName(r, filepath.ToSlash(rel)); err == nil {
			return true
		}
	}
	return false
}
