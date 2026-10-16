# AgentDock patches to upstream magpie

`engine/` is a squash `git subtree` of github.com/yetone/magpie `main`.

- Split: `422d057ded55e4eddf0b719bf2229e537c1fef24` (2026-10-15). Not yet pulled forward.
- Next pull target, once `main` is still that SHA: `674b6ca2881f1289c4ccdec158762978a463320c`.
- Command: `git subtree pull --prefix=engine https://github.com/yetone/magpie.git main --squash`.
- After every pull, on the Windows box: `cd engine && go vet ./... && go test -tags nogui ./...`.

Inventory is `git diff --name-status <split> HEAD:engine`. The split's files live at the repository root, not under `engine/`. `M` is an edited upstream file. `A` is fork-only. Do not classify a later sync with `git diff b04160de`; after a pull that range also lists upstream files added since the old pin.

Conflict rule: keep upstream hunks and AgentDock hunks when they are not the same lines. When they exclude each other, keep the AgentDock behavior and note the dropped upstream hunk. Never take a whole overlapping file from one side.

## Edited upstream files

| File | Why |
|---|---|
| `internal/agent/agents.go` | wires `sessions.WSLDistroOff`; discovery no longer treats a Windows launcher as the native CLI |
| `internal/agent/gemini.go` | native executable behind a Windows npm launcher |
| `internal/agent/omo.go` | same native-vs-launcher discovery |
| `internal/agent/omo_test.go` | discovery regression |
| `internal/agent/omp_wsl_test.go` | discovery regression |
| `internal/agent/primeagent.go` | same native-vs-launcher discovery |
| `internal/agent/reasonix.go` | same native-vs-launcher discovery |
| `internal/agent/rescan.go` | explicit rescan drops the stale discovery cache |
| `internal/agent/wsl.go` | distro switch, native fallback, running-distro gate |
| `internal/agent/wsl_claude_test.go` | discovery regression |
| `internal/agent/wsl_fake.go` | test double learns the native probe |
| `internal/agent/wsl_pi_test.go` | discovery regression |
| `internal/agent/wsl_test.go` | discovery regression |
| `internal/appdir/appdir.go` | engine data dir from `AGENTDOCK_DATA_DIR`, not `XDG_CONFIG_HOME` |
| `internal/appdir/appdir_test.go` | data-dir isolation |
| `internal/appdir/env.go` | inherited CLI env is not rewritten to the engine dir |
| `internal/appdir/env_test.go` | data-dir isolation |
| `internal/gui/api.go` | `environmentRoutes`; native routes |
| `internal/gui/assets/i18n.js` | Environments name/sub in zh, zh-TW, ja, de |
| `internal/gui/assets/icons/favicon.svg` | AgentDock icon |
| `internal/gui/assets/index.html` | AgentDock title, Environments nav, plugin tags |
| `internal/gui/assets/plugins/library/client.js` | WSL chip reason; native inventory UI |
| `internal/gui/assets/plugins/library/i18n.js` | native inventory strings |
| `internal/gui/assets/plugins/library/library.css` | native inventory layout |
| `internal/gui/builtins_test.go` | Environments counted as a built-in |
| `internal/gui/library.go` | native routes wired beside library routes |
| `internal/gui/settings_keep_test.go` | `WSLDistrosOff` stays in `KeepOwn` |
| `internal/gui/web.go` | sign-in title AgentDock |
| `internal/library/hooks.go` | WSL hooks, command translation, Codex trust rekey |
| `internal/library/library.go` | native inventory entry |
| `internal/library/mcp.go` | Windows-only stdio command refused for a WSL target |
| `internal/library/skills.go` | environment-local shared skills, no import-on-browse |
| `internal/library/subagents.go` | WSL subagents |
| `internal/library/sync.go` | native files are not moved by library sync |
| `internal/library/targets.go` | WSL shared/private/profile roots and capabilities |
| `internal/library/wsl_test.go` | target regression |
| `internal/sessions/wsl.go` | a distro turned off lists no sessions |
| `internal/settings/settings.go` | `WSLDistrosOff`, kept in `KeepOwn` |
| `internal/testenv/testenv.go` | tests see the isolated data dir |
| `internal/wslrun/wslrun.go` | `Find` skips distros turned off |
| `internal/wslrun/wslrun_test.go` | `TestFindSkipsDistroOff` |
| `plugins/all/all.go` | imports `plugins/environments` |

## Fork-only files

No upstream counterpart at split `422d057`.

- `AGENTDOCK_PATCHES.md`
- `internal/agent/environments.go`
- `internal/agent/wsl_discovery_test.go`
- `internal/agent/wsl_distros_off_test.go`
- `internal/agent/wsl_probe.go`
- `internal/environments/` — reach probes, `.wslconfig` mirrored switch, cross-environment sync, WSL magpie import, other-magpie detection
- `internal/gui/environments.go`, `internal/gui/environments_test.go`
- `internal/gui/assets/plugins/environments/`
- `internal/gui/native.go` — per-root paging (`ReadNativeRoot`); unfinished, not smoke-tested (`7737ccc`)
- `internal/library/bindings.go`
- `internal/library/hooks_wsl.go`, `internal/library/subagents_wsl.go`
- `internal/library/native.go`, `native_copy.go`, `native_file.go`, `native_ownership.go`, `native_wsl.go` and their tests — in-place native inventory; `native.go` paging unfinished
- `internal/library/wsl_env_targets_test.go`, `internal/library/wsl_hooks_test.go`
- `internal/settings/wsl_distros.go`, `internal/settings/wsl_distros_test.go`
- `plugins/environments/`
