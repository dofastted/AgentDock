package library

import (
	"errors"
	"fmt"
	"path/filepath"
)

type NativeCopyRequest struct {
	Source  string   `json:"source"`
	Targets []string `json:"targets"`
}

type NativeCopyPreview struct {
	Source        NativeItem       `json:"source"`
	SourceVersion string           `json:"sourceVersion"`
	Targets       []NativeCopyDiff `json:"targets"`
}

type NativeCopyDiff struct {
	Root    NativeRoot `json:"root"`
	Name    string     `json:"name"`
	Exists  bool       `json:"exists"`
	Before  string     `json:"before"`
	After   string     `json:"after"`
	Version string     `json:"version"`
	Error   string     `json:"error,omitempty"`
}

type NativeCopyTarget struct {
	Root    string `json:"root"`
	Version string `json:"version"`
}

type NativeCopyApply struct {
	Source        string             `json:"source"`
	SourceVersion string             `json:"sourceVersion"`
	Targets       []NativeCopyTarget `json:"targets"`
}

type NativeCopyResult struct {
	Root  string `json:"root"`
	Error string `json:"error,omitempty"`
}

type NativeCopyResults struct {
	Inventory NativeInventory    `json:"inventory"`
	Results   []NativeCopyResult `json:"results"`
}

func nativeCopyItem(source NativeItem, rootID string) (NativeRoot, NativeItem, error) {
	r, err := nativeRootByID(rootID)
	if err != nil {
		return r, NativeItem{}, err
	}
	if r.Kind != source.Kind || r.CLI != source.CLI {
		return r, NativeItem{}, errors.New("choose the same CLI and resource kind; native formats are not interchangeable")
	}
	if r.Kind == "mcp" {
		return r, NativeItem{}, errors.New("MCP files also contain local settings and commands; edit native MCP entries per environment instead of replacing the whole configuration")
	}
	name := source.Name
	if !r.directory {
		name = filepath.Base(r.Path)
	}
	name, err = nativeName(r, name)
	if err != nil {
		return r, NativeItem{}, err
	}
	item := nativeItem(r, name)
	if item.ID == source.ID || item.Path == source.Path && item.Environment == source.Environment {
		return r, item, errors.New("source and target are the same native file")
	}
	return r, item, nil
}

// PreviewNativeCopy reads actual target files and performs no mutation.
func PreviewNativeCopy(in NativeCopyRequest) (NativeCopyPreview, error) {
	if len(in.Targets) == 0 || len(in.Targets) > 32 {
		return NativeCopyPreview{}, errors.New("select between 1 and 32 native target locations")
	}
	source, err := ReadNativeFile(in.Source)
	if err != nil {
		return NativeCopyPreview{}, err
	}
	out := NativeCopyPreview{Source: source.Item, SourceVersion: source.Version, Targets: []NativeCopyDiff{}}
	seen := map[string]bool{}
	for _, id := range in.Targets {
		if seen[id] {
			continue
		}
		seen[id] = true
		r, item, err := nativeCopyItem(source.Item, id)
		d := NativeCopyDiff{Root: r, Name: item.Name, After: source.Text}
		if err == nil {
			var b []byte
			b, d.Exists, err = nativeRead(r, item.Path)
			d.Before, d.Version = string(b), nativeVersion(b, d.Exists)
		}
		if err != nil {
			d.Error = err.Error()
		}
		out.Targets = append(out.Targets, d)
	}
	return out, nil
}

// ApplyNativeCopy accepts the versions shown by preview, not blind overwrite.
func ApplyNativeCopy(in NativeCopyApply) (NativeCopyResults, error) {
	if len(in.Targets) == 0 || len(in.Targets) > 32 {
		return NativeCopyResults{}, errors.New("select between 1 and 32 native target locations")
	}
	if in.SourceVersion == "" {
		return NativeCopyResults{}, errors.New("preview the native source before copying")
	}
	nativeChanges.Lock()
	source, err := ReadNativeFile(in.Source)
	if err != nil {
		nativeChanges.Unlock()
		return NativeCopyResults{}, err
	}
	if source.Version != in.SourceVersion {
		nativeChanges.Unlock()
		return NativeCopyResults{}, ErrNativeConflict
	}
	out := NativeCopyResults{Results: []NativeCopyResult{}}
	seen := map[string]bool{}
	for _, target := range in.Targets {
		if seen[target.Root] {
			continue
		}
		seen[target.Root] = true
		result := NativeCopyResult{Root: target.Root}
		r, item, err := nativeCopyItem(source.Item, target.Root)
		if err == nil && target.Version == "" {
			err = errors.New("preview this target before copying")
		}
		if err == nil {
			err = nativeValidate(item, source.Text, false)
		}
		if err == nil {
			err = nativeWrite(r, item.Path, []byte(source.Text), target.Version, false)
		}
		if err != nil {
			result.Error = fmt.Sprint(err)
		}
		out.Results = append(out.Results, result)
	}
	nativeChanges.Unlock()
	out.Inventory = ReadNative()
	return out, nil
}
