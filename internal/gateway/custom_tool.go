package gateway

import (
	"bytes"
	"encoding/json"
	"regexp"
)

// Codex offers custom tools on Responses (exec in code mode, apply_patch):
// a tool of type custom whose call carries free-form input. A vendor's
// Responses API may take none of them, or only some: DeepSeek's answers
// 400 "Unsupported custom tool: 'exec'. Only 'apply_patch' is supported."
// to a request that only offers exec (#2656). A relay sends the tools as
// they came, so such a request is translated instead, on the same API:
// the translation offers each custom tool as a function taking its input
// (customSchema) and gives a call to one back as the custom_tool_call
// Codex runs. Once the model has turned them away it is translated from
// the start.

// customToolRefusal is an upstream turning a custom tool away: DeepSeek's
// "Unsupported custom tool: 'exec'", or the type named unsupported. A
// refusal of anything else about a tool doesn't match.
var customToolRefusal = regexp.MustCompile(`(?i)unsupported custom tool|custom tools? (?:is |are )?not (?:supported|allowed)|(?:unsupported|unknown|invalid) tool type[^\n{}]{0,20}?['"\x60]custom['"\x60]|tool type ['"\x60]custom['"\x60] (?:is )?(?:not supported|unsupported)`)

// customToolRefused is how unfit remembers model turning custom tools
// away.
func customToolRefused(model string) string { return "custom tools\x00" + model }

// hasCustomTool reports whether a Responses body offers a custom tool:
// among its tools, in a namespace there, or in an input item that carries
// tools (Responses Lite's additional_tools, a tool search's output).
func hasCustomTool(body []byte) bool {
	if !bytes.Contains(body, []byte(`"custom"`)) {
		return false
	}
	var m map[string]any
	if json.Unmarshal(body, &m) != nil {
		return false
	}
	if customIn(m["tools"]) {
		return true
	}
	items, _ := m["input"].([]any)
	for _, it := range items {
		if im, _ := it.(map[string]any); im != nil && customIn(im["tools"]) {
			return true
		}
	}
	return false
}

// customIn reports whether a list of tools has a custom one, a namespace's
// included.
func customIn(v any) bool {
	tools, _ := v.([]any)
	for _, t := range tools {
		tm, _ := t.(map[string]any)
		if tm == nil {
			continue
		}
		if tm["type"] == "custom" || tm["type"] == "namespace" && customIn(tm["tools"]) {
			return true
		}
	}
	return false
}
