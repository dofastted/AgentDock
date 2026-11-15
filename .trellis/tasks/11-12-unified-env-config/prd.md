# PRD — Windows/WSL unified skill + MCP config

Was blocked on 11-12-sync-magpie-2611; merge `90568dc64` (split 919780414) is in, build + vet clean, so design can proceed on the new library code.

## Problem
Skills and MCP configured on Windows mostly fail in WSL and vice versa. A library entry holds one command/path; `library/mcp_wsl.go side()` refuses any Windows-path command except magpie's image server, hooks only map `C:\` -> `/mnt/c`. Today three sources coexist:
- WSL `~/.agents/agents.yaml` + `agents-sync` (14 skills, per-CLI MCP override) — works inside WSL.
- Windows `C:\Users\Administrator\.agents` (38 skills, `servers/*.json`).
- AgentDock (magpie) library.

## Decisions (user, 2026-11-12)
- D1 SSOT = AgentDock library. `agents-sync` retires once parity is reached.
- D2 Skills: one shared source dir; every environment links/copies it; per-environment exclude list.
- D3 magpie upstream sync goes first.

## Requirements (draft)
- An MCP server entry can carry per-environment overrides (windows, wsl:<distro>) for command/args/env, analogous to agents.yaml `override.<cli>`.
- Skills: one source, per-env exclude; status per env visible in UI.
- Import existing WSL agents.yaml and Windows `.agents` into the library without loss.
- Secrets (`${VAR}`) resolved per environment.

## Open
- Where the shared skill dir lives (Windows side vs WSL side) and link vs copy across `\\wsl.localhost`.
- Migration/retirement path for agents-sync and CLAUDE.md references to it.

## Environment survey (2026-11-13, read-only)

| Source | Skills | MCP | Writer of |
|---|---|---|---|
| WSL `~/.agents` (git repo, `agents.yaml` + `secrets.env`, `agents-sync` 617-line uv script) | 14 in `skills/`, symlinked into omp/codex/claude/grok/pi | 10+ servers, per-CLI `override` / `enabled` | WSL omp/codex/claude/grok/pi configs |
| Windows `C:\Users\Administrator\.agents` | 38 in `skills/` | `servers/{context7,exa,memory,sequential-thinking}.json` | nothing (no sync tool) |
| AgentDock library `%APPDATA%\AgentDock\magpie\library.json` | 4 (2 GitHub, 2 folder from `\\wsl.localhost\Debian\...`) | codegraph, codegraphcontext, context7, grok-search, sequential-thinking, magpie-image | Windows **and** WSL CLI configs |

Confirmed defects (why cross-system management is broken today):
- **B1 Linux command written to Windows agents.** `codegraph` / `codegraphcontext` (`/home/mci777/.local/bin/...`) are bound to Windows `claude`, `codex`, `cursor`, … and are present in Windows `~/.codex/config.toml`; they cannot start there. `side()` (`engine/internal/library/mcp_wsl.go`) only guards the WSL direction.
- **B2 Windows command silently dropped for WSL.** `grok-search` uses `...\WinGet\Links\uvx.exe`; WSL twins are bound but `applied.json` shows it is not written (refused by `side()`); one entry cannot carry `uvx` for WSL.
- **B3 Two writers on WSL configs.** `agents-sync` (dry run) wants to rewrite context7 / sequential-thinking in WSL omp/codex/claude/grok that AgentDock last wrote → ping-pong; `~/.claude/skills` mixes hub symlinks (agents-sync) with copied dirs (AgentDock).
- **B4 No skill SSOT.** 14 vs 38 vs 4; overlap (`ssh-skill`, `codex`, `charles-packet-capture`, …) maintained twice.
- **B5 magpie-image points at a dev build** (`D:\AgentDock\shell\src-tauri\target\release\agentdock-engine.exe`) in Windows codex and WSL claude, not the installed engine.
- Leftover smoke artefact: WSL `~/.codex/skills/agentdock-smoke-20261016`.

## Design (approved 2026-11-14: skills on the Windows side, secrets as plain values in the library, order P1 → P3 → P2 → P4)

- **P1 MCP variants (fixes B1/B2).** Add `Variants map[string]ServerVariant` to `library.Server`; key = `windows` | `wsl` | `wsl:<distro>` | full agent id (`grok@wsl:Debian`, covers agents.yaml `override.<cli>`). Fields: command, args, env, cwd. Most specific key wins, then base. `side()` resolves the variant first; after resolution, a Linux-path command for a Windows target is refused with a reason (symmetric to the existing WSL rule). UI: per-environment tab in the server editor + status chip per env.
- **P2 Skills SSOT (fixes B4).** One physical source dir; library skills of kind `folder` point there; each env gets it via its native mechanism (Windows: junction/copy as magpie does today; WSL: symlink). Per-env exclude needs **no new field**: library skill `agents` already lists agent ids incl. `@wsl:` twins — exclusion = not binding.
- **P3 Import (lossless).** One-shot import of WSL `agents.yaml` (+ `secrets.env` values into env, never logged), Windows `.agents/servers/*.json` and both skill dirs; name collisions merged when content hash equal, otherwise shown for user choice. Preview before write.
- **P4 Retire agents-sync (fixes B3).** After parity check (AgentDock-rendered WSL configs ⊇ agents-sync output), rename `~/.local/bin/agents-sync` → `agents-sync.retired`, keep `~/.agents` git repo as archive; update CLAUDE.md/AGENTS.md references.
- **P5** Point magpie-image at the installed engine path; delete the smoke skill.

### Design corrections made while implementing (2026-11-14)

- **P1 refusal narrowed to cross-environment entries.** A hard "no Linux path on Windows" rule broke 7+ upstream test files (`mcp_local_test.go`, `backup/local_server_test.go`, `davsync/local_server_test.go`, `self_*`, `market_test.go`): upstream writes a command as typed and an absolute POSIX path is a program on macOS/Linux, where magpie also runs. The refusal now applies only when the server is one entry for several environments — it has a variant, or an agent in WSL has it (`crossEnv`) — which is exactly B1's shape (codegraph is bound to both sides). magpie's own image server stays exempt, as on the WSL side. Residual gap: a Linux-path server bound to Windows agents **only** is still written as typed; the server check reports it as `notfound`.
- **P3 reuses magpie's own discovery instead of a new agents.yaml parser.** `foundServersOf` already reads every agent's real config — including each WSL twin's, i.e. exactly what `agents-sync` rendered — and dedupes by name. Its gap was that a same-name server defined differently landed in `Others` and was dropped on import. It now folds a differing definition from **another environment** into the first as that environment's variant (this machine's agents are read first, so the entry's own command is the host one). A second definition in the *same* environment stays a conflict for the user to settle; per-agent variants (agents.yaml `override.<cli>` within one distro) are editable in the server editor instead. No YAML parser, no secrets copied by magpie: the rendered configs already hold the resolved values.
- **Windows `C:\Users\Administrator\.agents\servers\*.json` is not an import source.** The four files are stubs of another tool's shape (`{id,label,description,transport,command,args:[],env:{},url:null}`): `context7` carries `command: npx` with empty args, so importing them would create servers that cannot start. The Windows MCP truth is each CLI's own config, which magpie already discovers.
- **P2 home = the library's own skill store**, `%APPDATA%\AgentDock\magpie\library\skills\<name>` (on the Windows side, as decided), not `C:\Users\Administrator\.agents\skills`. That store is magpie's SSOT: it already copies/links a skill into every environment, including each distro, keeps `~/.agents/skills` as a *delivery* target for the agents that read it, detects an agent's edited copy and backs it up. Making `~/.agents/skills` the source as well would put the library behind a second source.
- **Skill overlap measured** (read-only): 32 Windows-only, 7 WSL-only, 7 in both. Of the 7, `_shared` is byte-identical; the other 6 differ only in `SKILL.md` (plus `__pycache__` and, in `ssh-skill`, a 1386-file `.venv` on the Windows side). The Windows copies carry rewritten `description:` lines; the Windows `codex` copy also carries a bad global replace (`~/.Codex/skills`, "委托 Codex subagent"). The import takes the Windows copy as the library's, and the WSL `~/.agents` git repo stays as the archive; magpie then shows each differing WSL copy as changed and offers to take it.

## Acceptance (draft)
- Same library entry yields a working MCP in Windows Claude/Codex and WSL Claude/Codex.
- Skill set identical across envs except declared excludes.
- No Linux path in any Windows CLI config, and no Windows `.exe` in any WSL CLI config (except magpie-image via interop), for every server shared between the two environments.
- `agents-sync` no longer runs; one writer for all CLI configs.

## Done (2026-11-14, on this machine)

Engine: `internal/library/mcp_variants.go` (new) + edits listed in `engine/AGENTDOCK_PATCHES.md`. UI: the server editor's **按环境** section (`windows` / `wsl` / `wsl:<distro>` / an agent id), agent chips judged on the resolved command, zh/zh-TW strings. `go vet` clean; `internal/library` and the `internal/gui` asset tests show **no new failures against HEAD** (worktree baseline: both fail only `TestClaudeDesktopSkillRootsBehindALink` — `mklink` is unavailable in the tests' cleaned environment — and `TestCoreScriptBudget` with the same byte count on both sides, a pre-existing overage in core scripts that plugin assets don't count towards).

- **AC1 one entry, a command per environment — proven on real files.** A disposable `adk-variant-smoke` entry with a `windows` variant wrote `C:\Tools\codegraph-mcp.exe --win` into `C:\Users\Administrator\.claude.json` and `/home/mci777/.local/bin/codegraph-mcp` into `\\wsl.localhost\Debian\…\.claude.json`; removing it took both away. `chrome-devtools` (rebuilt from `agents.yaml`, agent-id variant) writes plain `npx --browserUrl=…` for Codex in Debian and grok's own nvm npx with the Windows Chrome path for Grok Build in Debian.
- **AC2 B1 fixed.** `codegraph` / `codegraphcontext` are Linux-only programs (no Windows build on this machine): their 12 Windows bindings were wrong and are gone; the Windows agents' configs no longer carry a Linux command. A cross-environment Linux command for a Windows agent is now refused with the reason naming the `windows` variant.
- **AC3 B2 fixed.** `grok-search` (Windows `uvx.exe`) has a `wsl` variant (`uvx`), so the five WSL twins are written for the first time. A Windows program the distro starts through interop (`feedback` → `/mnt/d/code/CC/feedback.exe`) is written when a variant names it for that environment, and still refused when inherited or named as `C:\…`.
- **AC4 B3 fixed.** `~/.local/bin/agents-sync` → `agents-sync.retired` (off `PATH`); nothing (cron, systemd timer, shell rc) ran it automatically. Its last preview showed it would have rewritten codegraph, context7, grok-search, sequential-thinking, feedback and added chrome-devtools over AgentDock's writes — that ping-pong is over. `~/.claude/CLAUDE.md` §6/§7 and the `agents.yaml` header now name the AgentDock library as the source; `agents.yaml` + `secrets.env` are archive.
- **AC5 B4 fixed.** Library: 6 → **18 servers**, 4 → **50 skills**. `C:\Users\Administrator\.agents\skills` is down to `_shared`, `.system`, `ssh-skill`; the rest live in `%APPDATA%\AgentDock\magpie\library\skills` and reach the agents as links, including each distro (`/home/mci777/.local/share/magpie/skills/<name>`). Only Codex's own `cua_repl` / `node_repl` stay unmanaged, as they should.
- **AC6 B5 fixed.** `magpie-image` points at the installed engine; `magpieBinary` now counts the engine's own name, so `heal()` can repoint a moved build. The 10-16 smoke leftover `~/.codex/skills/agentdock-smoke-20261016` is gone.

### Left open (needs the user)

- `ssh-skill`: the Windows copy carries a 1386-file `.venv` and `__pycache__`; the WSL copy (58 files) is clean. Not imported either way — the name-only import API would take the Windows row.
- Five names where the distro keeps its own copy and magpie refused to overwrite it (`charles-packet-capture`, `chrome-devtools-guide`, `codex`, `codex-blackhole`, `wsl-windows-git-bridge`): the library holds the Windows version. Only `SKILL.md` differs; the Windows `codex` copy carries a bad global replace (`~/.Codex/skills`, "委托 Codex subagent"), so taking it would be a regression. `skills/use-library` per agent applies the library's; `skills/keep-own` keeps the distro's.
- `hermes@wsl:Debian`: `~/.hermes/.config.yaml` and `~/.hermes/skills` are root-owned, so every sync reports "Access is denied" for it. Pre-existing (recorded in 10-16); fixing it means `chown` in the distro, which AgentDock does not do.
- Rollback: engine binary backup at `C:\Users\Administrator\AppData\Local\AgentDock\agentdock-engine.exe.bak-20261114`; library backups under `%APPDATA%\AgentDock\magpie\backups`; `~/.agents` git repo holds the handed-over WSL skills' history; `agents-sync.retired` can be renamed back.
