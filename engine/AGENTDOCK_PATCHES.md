# AgentDock patches to upstream magpie

`engine/` is a `git subtree` of github.com/yetone/magpie (`main`). AgentDock's
own code is in new files; this list is every **upstream file** it edits, so a
`git subtree pull --prefix=engine https://github.com/yetone/magpie.git main --squash`
can be checked hunk by hunk. After every pull run, on the Windows box:

    cd engine && go vet ./... && go test -tags nogui ./...

## Edited upstream files

| File | Why |
|---|---|
| `internal/settings/settings.go` | `WSLDistrosOff` field (per-distro switch, G4); kept in `KeepOwn` |
| `internal/agent/wsl.go` | `wslDistros`, `WSLRunning`, `wslClaudeStandIn` skip distros turned off |
| `internal/agent/agents.go` | wires `sessions.WSLDistroOff` |
| `internal/sessions/wsl.go` | `WSLDistroOff` hook: a distro turned off lists no sessions |
| `internal/wslrun/wslrun.go` | `Find` skips distros turned off |
| `internal/library/hooks.go` | WSL agents get hooks (`wslHookSpecOf`), commands translated (`sp.command`/`sp.runnable`), Codex trust rekey by the distro's Linux path, `sameHookFamily`/`hookTakes` take `<kind>@wsl:<distro>` |
| `internal/library/subagents.go` | WSL agents get subagents (`wslSubagentSpecOf`) |
| `internal/gui/api.go` | `environmentRoutes(mux)`; `POST /api/settings` carries `WSLDistrosOff` |
| `internal/gui/assets/plugins/library/client.js` | WSL chips show the server's reason; an own event matched by agent kind |
| `internal/gui/assets/index.html` | title/brand "AgentDock", Environments nav button, plugin `<link>`/`<script>` tags |
| `internal/gui/assets/i18n.js` | Environments plugin Name/Sub in zh, zh-TW, ja, de |
| `internal/gui/assets/icons/favicon.svg` | AgentDock icon |
| `internal/gui/web.go` | sign-in page titled AgentDock |
| `plugins/all/all.go` | imports `plugins/environments` |

Upstream tests adjusted for the above: `internal/gui/builtins_test.go`
(15 built-ins, Environments essential), `internal/gui/settings_keep_test.go`
(`WSLDistrosOff` set), `internal/wslrun/wslrun_test.go` (`TestFindSkipsDistroOff`).

## AgentDock's own files (no upstream counterpart)

- `internal/environments/` — reach probes, `.wslconfig` mirrored switch, cross-environment sync, WSL magpie import, other-magpie detection
- `internal/agent/environments.go` — `WSLEnvs`, `HostAgents`
- `internal/library/{hooks_wsl,subagents_wsl,bindings}.go`
- `internal/settings/wsl_distros.go`
- `internal/gui/environments.go` — `/api/environments*`
- `plugins/environments/`, `internal/gui/assets/plugins/environments/`
