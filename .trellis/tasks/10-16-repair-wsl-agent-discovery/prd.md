# Repair Windows–WSL agent discovery and management

## Goal

Make AgentDock a reliable Windows entry point for managing the existing Windows and WSL installations of AI CLIs and their skills, prompts/instructions, agent definitions and MCP configurations. Files must remain usable and editable inside WSL.

## Approved Product Decision

The user chose **in-place management of each environment's files**, not a Windows-canonical master library with WSL deployment copies. Existing native files remain authoritative in their own environment. Windows presents a unified inventory/editor; refresh reads changes made inside WSL. Cross-environment copying is explicit and identifies source and targets. Browsing or adding a resource to management must not move it into Windows, create a second authoritative copy, or overwrite a same-named variant.

## Background and Evidence

The user reports the compiled version's dual-environment management is largely abnormal. Treat that report as ground truth. Parent task `10-15-unified-agent-manager` remains `in_progress`, although AC1–AC10 are checked; historical acceptance records are not proof that this version works. `00-bootstrap-guidelines` is also unfinished. Backend/frontend spec indexes and quality files remain unfilled templates.

The running application is the repository Windows release output, with web UI on 3430. Read-only API, browser and filesystem investigation found:

- **B1 — wrong external config roots:** `shell/src-tauri/src/sidecar.rs:183-187` sets process XDG_CONFIG_HOME to AgentDock; `engine/internal/agent/agents.go:80-103` uses it for external CLIs. Live Windows OpenCode targets AgentDock-private config despite an existing user `~/.config/opencode` config.
- **B2 — missing WSL shared skills and identities:** `engine/internal/library/skills.go:434,1671-1686` scans only the host shared root plus primary target roots. Debian `~/.agents/skills` is absent from inventory. `skills.go:1673,1697-1736,1781-1835` groups/imports by name and can move native folders. `targets.go:576-624,744-767` omits some WSL capabilities and does not normalize named omp profiles for library targeting.
- **B3 — installed native CLI missed:** `engine/internal/agent/wsl.go:1208` short-circuits native fallback when a Windows launcher is first on PATH; lines 1302–1305 then discard it. A read-only Debian demonstration selected Windows npm Gemini while its executable also existed under native nvm; Gemini was missing from the live agent inventory.
- **B4 — stale detection and hidden failures:** `engine/internal/gui/environments.go:39-58,124-135` has no list/probe diagnostic contract and refreshes only reachability. Agent and executable discovery use separate caches/resolvers.
- **B5 — incompatible/unwritable MCP targets:** current Library records five Windows uvx.exe refusals for WSL and six Hermes atomic-write failures. Debian `~/.hermes` is root:root mode 755; config.yaml is absent. `mcp_wsl.go:34-39` refuses Windows programs; UI target defaults do not account for that compatibility. Do not weaken the refusal or change Linux permissions.
- **B6 — prompt/agent coverage is not equivalent to instructions:** native Debian Codex has `~/.codex/prompts/` and `~/.codex/agents/` files. The library's existing Instructions and Subagents surfaces do not establish coverage of those files; the user's prompt/agent requirement must not silently be reduced to global instructions alone.

Full evidence and review anchors are in `research/runtime-evidence.md`, `research/discovery-review.md`, and `research/library-review.md`. Credentials and config contents were not recorded.

## Requirements

- **R1 Environment discovery:** detect Windows and eligible WSL distributions, their default user/home, running/asleep state and discovery errors. Keep gateway reachability distinct from CLI discovery. Read-only listing must not wake stopped distros; failed lookup must not look like an empty environment. Explicit refresh refreshes discovery as well as reachability.
- **R2 CLI and resource roots:** distinguish native executable, Windows interop launcher, configuration-only installation and missing executable. Locate native tool-manager binaries even when Windows PATH entries precede them. Respect native custom config roots and profiles; engine data isolation must not redirect external CLI configuration.
- **R3 Native inventory and editing:** show environment, CLI/profile, resource kind and actual path for existing skills, prompt templates/global instructions, agent definitions and MCP configs. Include environment-local shared skills, CLI-private locations, existing symlinks and named profiles. Expose actual supported capabilities rather than silently hiding unavailable ones.
- **R4 Ownership and updates:** edit the selected native resource in place; re-read WSL-side edits on refresh/reopen. Identity must distinguish same-named resources across environments/paths. Preserve unrelated settings and resource files. Refuse stale writes after external modification and present the current version rather than silently overwriting it.
- **R5 Target-safe actions:** creation, editing, removal, binding and supported enable/disable operations affect only selected native targets. Cross-environment copy requires source selection, difference preview and explicit apply; no automatic two-way synchronization or source-directory migration. Do not invent a CLI feature such as prompt enable/disable where the CLI has none.
- **R6 MCP and permissions:** prevent default assignment of Windows-only stdio commands to incompatible WSL targets. A Linux command is explicit and target-specific, not a renamed .exe. Show unsupported and permission-denied outcomes with actionable context; never report false success, elevate, chmod/chown, or imply unchanged retry will fix ownership.
- **R7 Storage compatibility:** retain AgentDock engine settings/providers/library metadata in its existing isolated data directory, independently of CLI config resolution. Do not silently merge, delete or prefer unintended AgentDock-private CLI config copies over the actual CLI config.

## Acceptance Criteria

- [ ] **AC1 (R1):** packaged Windows application lists Windows and eligible WSL distros with correct states/user identity. Empty/missing WSL and failed/timed-out probing are distinguishable. Listing does not wake stopped distros.
- [ ] **AC2 (R1,R2):** native Debian Gemini is discovered with a Windows npm Gemini first on PATH. Configuration-only agents and interop launchers are distinct from native executables. An explicit rescan updates inventory without restarting AgentDock.
- [ ] **AC3 (R2,R3):** Windows and WSL resource inventories show the actual shared/private/custom/profile roots. Debian `~/.agents/skills`, named omp profile skills and existing Codex prompt/agent files are not silently omitted. Unsupported capabilities have an explicit explanation.
- [ ] **AC4 (R3–R5):** edit a WSL skill, prompt/instruction, agent definition and MCP entry from Windows; read the resulting native files inside WSL. Edit them inside WSL and refresh Windows to see changes. Native resource folders are not moved; same-named Windows/WSL variants and unrelated settings survive. A stale editor cannot silently overwrite an external change.
- [ ] **AC5 (R5):** explicit cross-environment copy previews differences and changes only chosen targets after confirmation. Read-only inventory and editing never implicitly deploy to other environments.
- [ ] **AC6 (R6):** Windows-only MCP commands are not preselected for WSL; incompatible assignments are explicitly refused. An unwritable WSL directory yields a permission error without repeated misleading success, elevated writes or permission changes.
- [ ] **AC7 (R7):** engine state remains under `%APPDATA%\AgentDock\magpie`; Windows OpenCode reads/writes its real user configuration, not a new AgentDock-private root. Existing private copies are preserved without automatic merging.
- [ ] **AC8 (all):** run applicable quality checks and exercise the packaged Windows UI against real Windows + live Debian paths, including read/write/readback. Cover stopped-distro/timeout cases without stopping the distro hosting this session. Record existing upstream failures separately; compilation alone is not acceptance.

## Out of Scope

CLI installation/upgrades, WSL network/firewall changes, automatic Linux ownership/permission repair, taking over other magpie processes, automatic bidirectional replication, moving native resource ownership to Windows, whole new desktop platforms, and unrelated guideline bootstrap work. This repair manages the configured/default distro user; it does not add a multi-user account management product.

## Planning Status

Ownership decision resolved. Technical design and execution plan accompany this PRD. Implementation requires explicit approval of the latest final planning summary; task creation consent and the ownership answer alone are not implementation approval.
