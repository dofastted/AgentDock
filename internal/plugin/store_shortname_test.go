package plugin

import (
	"reflect"
	"testing"
)

// The community's READMEs say `magpie plugin options model-map '<json>'`:
// a package's short name finds it, as its full name and its spec do.
func TestSetOptionsShortName(t *testing.T) {
	storeSandbox(t)
	writeList(t, `{"plugins":[{"spec":"@magpie-community/opencode-zed-auth"},{"spec":"@magpie-community/middleware-model-map@0.1.0"}]}`)
	want := map[string]any{"mapping": map[string]any{"fast": "deepseek-chat"}}
	if err := SetOptions("model-map", want); err != nil {
		t.Fatal(err)
	}
	if err := SetOptions("zed", map[string]any{"x": true}); err != nil {
		t.Fatal(err)
	}
	l := Load()
	if !reflect.DeepEqual(l.Plugins[1].Options, want) {
		t.Errorf("model-map's options = %v", l.Plugins[1].Options)
	}
	if l.Plugins[0].Options["x"] != true {
		t.Errorf("zed's options = %v", l.Plugins[0].Options)
	}
	if err := SetOptions("map", want); err == nil {
		t.Error("a part of a name found a plugin")
	}
	for in, out := range map[string]string{
		"@magpie-community/middleware-param-override": "param-override",
		"@magpie-community/opencode-cursor-auth":      "cursor",
		"opencode-foo":                                "foo",
		"/Users/me/plugins/think-tags":                "think-tags",
		`C:\Users\me\plugins\middleware-zz-tags`:      "zz-tags",
	} {
		if got := ShortName(in); got != out {
			t.Errorf("ShortName(%q) = %q, want %q", in, got, out)
		}
	}
}
