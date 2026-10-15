// Package environments is AgentDock's Environments page: this computer and
// each WSL distro side by side — whether the gateway listens here and
// whether each distro reaches it from inside, each distro turned on or off,
// an agent's setup copied onto its twins, WSL switched to mirrored
// networking, and providers brought in from CC Switch or a distro's own
// magpie.
package environments

import "github.com/yetone/magpie/internal/pluginrt"

func init() {
	pluginrt.Register(pluginrt.Manifest{
		ID: "environments", API: "^1", Requires: []string{"window"}, Essential: true,
		Name: "Environments", Sub: "This computer and each WSL distro: whether each reaches the gateway, which are on, and one agent's setup copied onto its twins",
		Parts:  []pluginrt.Part{{Slot: "nav.page", Key: "environments"}},
		Client: []string{"i18n.js", "environments.css", "client.js"},
	})
}
