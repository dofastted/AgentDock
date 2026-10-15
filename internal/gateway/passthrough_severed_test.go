package gateway

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// postMessages sends an Anthropic Messages request to the gateway, as dsh's
// DeepSeek provider and Claude Code do.
func postMessages(t *testing.T, body string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(body))
	New().Handler().ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

// An upstream that speaks Messages itself is relayed as-is; when it died
// mid-reply the relay just stopped, so the client saw a stream that ended
// cleanly with neither message_stop nor an error: dsh's "DeepSeek Messages
// stream ended before message_stop" (STREAM_CLOSED), which it does not
// retry and which says nothing of what went wrong (#370).
func TestPassthroughStreamSeveredUpstream(t *testing.T) {
	fresh(t)
	severed(t, anthropicStart, anthropicText)
	code, body := postMessages(t, `{"model":"up/m","stream":true,"max_tokens":8,"messages":[{"role":"user","content":"hi"}]}`)
	if code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", code, body)
	}
	if !strings.Contains(body, "event: error") || !strings.Contains(body, "UP:") {
		t.Fatalf("a severed stream ended without an error event naming the provider: %s", body)
	}
	if strings.Contains(body, "message_stop") {
		t.Fatalf("a severed stream was finished as if complete: %s", body)
	}
}

// One the upstream ended cleanly but short of its last event is as broken.
func TestPassthroughStreamEndsShort(t *testing.T) {
	fresh(t)
	f := &fake{reply: anthropicStart + anthropicText}
	setup(t, "anthropic", f)
	code, body := postMessages(t, `{"model":"fake/m1","stream":true,"max_tokens":8,"messages":[{"role":"user","content":"hi"}]}`)
	if code != 200 || !strings.Contains(body, "event: error") {
		t.Fatalf("a stream cut short ended without an error event: %d %s", code, body)
	}
}

// A whole stream goes through untouched, nothing added after its end.
func TestPassthroughStreamWhole(t *testing.T) {
	fresh(t)
	f := &fake{reply: anthropicStart + anthropicText + anthropicStop}
	setup(t, "anthropic", f)
	code, body := postMessages(t, `{"model":"fake/m1","stream":true,"max_tokens":8,"messages":[{"role":"user","content":"hi"}]}`)
	if code != 200 || body != anthropicStart+anthropicText+anthropicStop {
		t.Fatalf("a whole stream was changed: %d %q", code, body)
	}
}

// A Responses stream cut short fails as Codex reads a failed turn.
func TestPassthroughResponsesEndsShort(t *testing.T) {
	fresh(t)
	f := &fake{reply: sse(`event: response.created`+"\n"+`data: {"type":"response.created","response":{"id":"r1","model":"m1"}}`,
		`event: response.output_text.delta`+"\n"+`data: {"type":"response.output_text.delta","delta":"hel"}`)}
	setup(t, "responses", f)
	code, body := post(t, "/v1/responses", `{"model":"fake/m1","stream":true,"input":"hi"}`)
	if code != 200 || !strings.Contains(body, `"type":"response.failed"`) || !strings.Contains(body, "Fake:") {
		t.Fatalf("a Responses stream cut short did not fail: %d %s", code, body)
	}
}

// A whole stream whose last event is too big for the sniffer to read (an
// image's base64 in response.completed) is still whole: nothing is added.
func TestPassthroughResponsesBigLastEvent(t *testing.T) {
	fresh(t)
	big := strings.Repeat("A", 2<<20)
	reply := sse(`event: response.created`+"\n"+`data: {"type":"response.created","response":{"id":"r1","model":"m1"}}`,
		`event: response.completed`+"\n"+`data: {"type":"response.completed","response":{"id":"r1","output":[{"type":"image_generation_call","result":"`+big+`"}]}}`)
	f := &fake{reply: reply}
	setup(t, "responses", f)
	code, body := post(t, "/v1/responses", `{"model":"fake/m1","stream":true,"input":"hi"}`)
	if code != 200 || body != reply {
		t.Fatalf("a whole stream with a big last event was changed: %d, %d bytes added", code, len(body)-len(reply))
	}
}
