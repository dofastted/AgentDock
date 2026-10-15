package gateway

import (
	"net/http"

	"github.com/yetone/magpie/internal/gwmw"
	"github.com/yetone/magpie/internal/middleware"
	"github.com/yetone/magpie/internal/provider"
)

// serveAgent is serve for a request an agent sent: the agent middleware
// of the gateway's middleware point (gwmw.Agents: the JS gateway middleware
// plugins, internal/middleware, while plugins/middleware is on) see its
// body first, and its reply on the way back, in the agent's own API.
// magpie's own requests through serve (titles, the router's classifier,
// search and vision stand-ins) go without them, as their replies are
// magpie's to read.
func (s *Server) serveAgent(w http.ResponseWriter, r *http.Request, from provider.Protocol, body []byte) {
	withGatewaySession(w, r)
	w, recorded := recordConversation(w, r, from, body)
	defer recorded()
	agents := gwmw.AgentsWanted(middlewareWanted)
	if len(agents) == 0 {
		s.serve(w, r, from, body)
		return
	}
	info := gwmw.Info{Protocol: string(from), Model: modelOf(body), Stream: streamOf(body), Path: r.URL.Path, Agent: agentOf(r)}
	for _, a := range agents {
		run := a.Begin(info)
		if run == nil {
			continue
		}
		defer run.End()
		var turned *gwmw.Reject
		body, turned = run.Request(body)
		if turned != nil {
			writeError(w, from, turned.Status, turned.Message)
			return
		}
		var done func()
		w, done = run.Wrap(w)
		defer done()
	}
	s.serve(w, r, from, body)
}

// middlewareWanted: plugins.json has a JS middleware on, so plugins/middleware
// is in use.
func middlewareWanted() bool { return middleware.On() > 0 }
