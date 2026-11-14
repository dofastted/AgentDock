# AgentDock patches to upstream magpie

`engine/` is a squash `git subtree` of github.com/yetone/magpie `main`.

- Split: `9197804144c15c7765c0e8b13376981264dc02a2` (2026-11-12), pulled in `90568dc64`. Previous split was `6afbe3f3047419bfd50e522526457bac3fa25f2c`.
- Last resolution: 27 files conflicted (3041 upstream commits). Upstream now has its own native WSL probe (`wslProbeScript`/`parseProbe`, Windows-drive launchers looked past, `place.open` for links #4502): AgentDock's duplicate probe in `wsl_probe.go` was dropped, and only what upstream lacks was ported onto it — distro path variables (`env:`), `user:`, `winbin:`, `ProbeErr`, `WSLDistrosOff` in `wslReady`/classifier/`wslRefreshRunning`. `distro.Bins` became upstream's `distro.Paths`. A failed `wsl.exe -l` sets `listed=false` so known distros aren't forgotten. `gateway.go`: `foldDoubledVersion` is outermost in upstream's new `Handler()`. Settings save moved to `settingsapi.Apply`: `imageFit` omit-preserve and `WSLDistrosOff` carry ported there. `wslTargetOf` is upstream's table (links, distro skill store) with discovered custom roots for claude/codex/pi/omo/gemini; any other WSL agent falls back to the host adapter with `dropHostLeaks`. Shared skills: upstream's `sharedEntry`/`readSharedOwn`/`readsSharedDir` gained per-environment `…In(dir)` forms. Upstream tests assuming no WSL hooks/subagents were adapted to AgentDock's. Upstream's new library `under` clashed with ours: ours is `underDir`. Nothing else was dropped.
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
| `internal/agent/wsl_links.go` | `distroAt` refreshes the running list before taking the lock |
| `internal/agent/wsl.go` | distro switch, native fallback, running-distro gate |
| `internal/agent/wsl_claude_test.go` | discovery regression |
| `internal/agent/wsl_fake.go` | test double learns the native probe |
| `internal/agent/wsl_pi_test.go` | discovery regression |
| `internal/agent/wsl_test.go` | discovery regression |
| `internal/appdir/appdir.go` | engine data dir from `AGENTDOCK_DATA_DIR`, not `XDG_CONFIG_HOME` |
| `internal/appdir/appdir_test.go` | data-dir isolation |
| `internal/appdir/env.go` | inherited CLI env is not rewritten to the engine dir |
| `internal/appdir/env_test.go` | data-dir isolation |
| `internal/gui/api.go` | `environmentRoutes`; native routes; a settings save that omits `imageFit` keeps it |
| `internal/gateway/gateway.go` | calls `anthropicImagesFit` on the final Anthropic body; folds a doubled `/v1` (`/v1/v1/…`, `/v1/v1beta/…`) onto the real route before the mux |
| `internal/gui/assets/app.js` | Settings › Models › Images › Fit large images, default Off |
| `internal/gui/assets/i18n.js` | Environments name/sub in zh, zh-TW, ja, de; Fit large images |
| `docs/subsystems/gateway-routing.md` | why Fit large images defaults to off |
| `internal/gui/assets/icons/favicon.svg` | AgentDock icon |
| `internal/gui/assets/index.html` | AgentDock title, Environments nav, plugin tags |
| `internal/gui/assets/plugins/library/client.js` | WSL chip reason; native inventory UI |
| `internal/gui/assets/plugins/library/i18n.js` | native inventory strings |
| `internal/gui/assets/plugins/library/library.css` | native inventory layout |
| `internal/gui/builtins_test.go` | Environments counted as a built-in |
| `internal/gui/library.go` | native routes wired beside library routes |
| `internal/gui/settings_keep_test.go` | `WSLDistrosOff` stays in `KeepOwn` |
| `internal/gui/web.go` | sign-in title AgentDock |
| `internal/settingsapi/settingsapi.go` | `imageFit` in Keys and kept when omitted; `WSLDistrosOff` carried |
| `internal/library/hooks.go` | WSL hooks, command translation, Codex trust rekey |
| `internal/library/library.go` | native inventory entry |
| `internal/library/mcp.go` | Windows-only stdio command refused for a WSL target; `Server.Variants` and its check; `mcpFile.Agent`; `supports` resolves the variant; found servers fold another environment's command into a variant (this machine's agents first) |
| `internal/library/mcp_check.go` | a check starts the command the agents there are given (`variantFor`) |
| `internal/library/mcp_check_wsl.go` | the WSL side is checked when the server's `wsl` variant is a command WSL has |
| `internal/library/mcp_hub.go` | the MCP router runs this machine's variant of a server |
| `internal/library/mcp_wsl.go` | `side` resolves the environment's variant first, and refuses a cross-environment Linux command for a Windows agent |
| `internal/library/self_here.go` | the engine's own binary name counts as magpie's own, so `heal` can repoint a moved build |
| `internal/library/library.go` | `Library.Baseline`; `changeLocked` mirrors the baseline onto its followers before saving and settles a follower's own skill in the library's way after the sync |
| `internal/library/skills.go` | `useLibrarySkill` factored out of `UseLibrarySkills` for the baseline's followers |
| `internal/library/skills.go` | environment-local shared skills, no import-on-browse |
| `internal/library/subagents.go` | WSL subagents |
| `internal/library/sync.go` | native files are not moved by library sync; a server save that omits `variants` keeps them, as it does the tools turned off; the written server is the agent's own variant |
| `internal/library/targets.go` | WSL shared/private/profile roots and capabilities; `targetKind` strips `@wsl` and profile. 2026-10-16 pull also kept upstream `SharedName` |
| `internal/library/wsl_test.go` | target regression |
| `internal/sessions/wsl.go` | a distro turned off lists no sessions |
| `internal/settings/settings.go` | `WSLDistrosOff`, kept in `KeepOwn`; `ImageFit`, off when unset |
| `internal/testenv/testenv.go` | tests see the isolated data dir |
| `internal/wslrun/wslrun.go` | `Find` skips distros turned off |
| `internal/wslrun/wslrun_test.go` | `TestFindSkipsDistroOff` |
| `plugins/all/all.go` | imports `plugins/environments` |

## Fork-only files

No upstream counterpart at the current split.

- `AGENTDOCK_PATCHES.md`
- `internal/gateway/anthropic_images.go` — scale oversized Anthropic images only when `ImageFit` is on
- `internal/gateway/anthropic_images_test.go` — default-off and the fifth-image gate
- `internal/gateway/doubled_version_test.go` — `/v1/v1/messages` and `/v1/v1beta/…` hit the same route as the single prefix
- `internal/agent/environments.go`
- `internal/agent/wsl_discovery_test.go`
- `internal/agent/wsl_distros_off_test.go`
- `internal/agent/wsl_probe.go` — distro path variables the probe reads (`wslPathEnvs`, `wslEnvDirs`)
- `internal/environments/` — reach probes, `.wslconfig` mirrored switch, cross-environment sync, WSL magpie import, other-magpie detection
- `internal/gui/environments.go`, `internal/gui/environments_test.go`
- `internal/gui/assets/plugins/environments/`
- `internal/gui/native.go` — per-root paging (`ReadNativeRoot`); unfinished, not smoke-tested (`7737ccc`)
- `internal/library/bindings.go`
- `internal/library/mcp_variants.go`, `internal/library/mcp_variants_test.go` — one MCP entry per server with a command for each environment (`windows`, `wsl`, `wsl:<distro>`, an agent id), and the Linux-on-Windows refusal
- `internal/library/baseline.go`, `internal/library/baseline_test.go`, `internal/environments/baseline.go` — a main agent whose Library ticks its followers mirror on every change
- `internal/library/hooks_wsl.go`, `internal/library/subagents_wsl.go`
- `internal/library/native.go`, `native_copy.go`, `native_file.go`, `native_ownership.go`, `native_wsl.go` and their tests — in-place native inventory; `native.go` paging unfinished
- `internal/library/wsl_env_targets_test.go`, `internal/library/wsl_hooks_test.go`
- `internal/settings/wsl_distros.go`, `internal/settings/wsl_distros_test.go`
- `plugins/environments/`
