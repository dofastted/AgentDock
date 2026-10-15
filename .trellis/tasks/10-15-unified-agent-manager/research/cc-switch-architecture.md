# cc-switch architecture, WSL handling, proxy, rebrand surface

> Scout research captured 2026-10-15. Paths: magpie=/mnt/d/magpie, cc-switch=/mnt/x/cc-switch.

# cc-switch reuse report (v4.0.13)

## 1. Stack and versions

- App: cc-switch / CC Switch 4.0.13; identifier `com.ccswitch.desktop` (`src-tauri/tauri.conf.json:4`).
- Frontend: React 18.2, Vite 7.3, TypeScript 5.3, pnpm 10.12.3, Node 22.12.0 (`.node-version`).
- UI: Tailwind 3.4 + shadcn (`components.json`), Radix, lucide, framer-motion, sonner, CodeMirror 6.
- Data/i18n: TanStack Query 5.90, i18next 25, zod 4, react-hook-form. No Zustand.
- Tauri: CLI ^2.10.1, runtime 2.8.2, tray-icon + protocol-asset.
- Rust: toolchain 1.95 (`rust-toolchain.toml`), crate rust-version 1.85.0.
- Backend: rusqlite 0.31 bundled, axum 0.7, hyper 1, rustls, reqwest 0.12, rquickjs, toml_edit, zip.
- Plugins (`lib.rs:355-525`): log, opener, process, updater, dialog, store, deep-link scheme `ccswitch`, window-state, single-instance.
- Updater (`tauri.conf.json:50-57`): `https://dl.ccswitch.io/latest.json` then GitHub `farion1231/cc-switch`.
- Windows overlay: `tauri.windows.conf.json` (custom title bar, decorations false).

## 2. Rust backend module map (`src-tauri/src`)

Approx on-disk sizes from listing.

- `lib.rs` 109KB: bootstrap, plugins, deeplink, invoke_handler.
- `commands/`: Tauri surface; `misc.rs` 391KB (tool lifecycle/WSL probes).
- `services/`: skill.rs 292KB, usage_stats.rs 219KB, provider/mod.rs 340KB.
- `proxy/`: forwarder.rs 341KB, handlers.rs 166KB, streaming_responses.rs 361KB.
- `mode/`: controller.rs 321KB Direct/Routing/Aggregation; live-state.json.
- `live/`: ownership-aware live-file patch (JSON/TOML/.env).
- `database/`: schema.rs 149KB, SCHEMA_VERSION 20.
- `mcp/`: per-app MCP adapters to live files.
- `deeplink/`: `ccswitch://` import (provider/MCP/prompt/skill).
- `session_manager/`: session scan/resume per CLI.
- `tray.rs` 162KB; `settings.rs` 50KB; `config.rs` 38KB; `provider.rs` 67KB; `app_config.rs` 44KB.

Per-CLI live writers (settings override unless noted):

- Claude Code `config.rs`: `~/.claude` + `~/.claude.json` MCP.
- Claude Desktop `claude_desktop_config.rs:1470+`: `%LOCALAPPDATA%\Claude\claude_desktop_config.json`; no config_dir setting.
- Codex `codex_config.rs:269`: `~/.codex/config.toml` + auth.json.
- Gemini `gemini_config.rs:9`: `~/.gemini/.env` + settings.json MCP.
- Grok Build `grok_config.rs:24`: `~/.grok/config.toml`.
- OpenCode `opencode_config.rs:49`: `~/.config/opencode/opencode.json(c)`.
- OpenClaw `openclaw_config.rs:35`: `~/.openclaw/openclaw.json`.
- Hermes `hermes_config.rs:53`: Win `%LOCALAPPDATA%\hermes\config.yaml` else `~/.hermes`.
- Pi `pi_config/mod.rs:34`: `~/.pi/agent` (PI_CODING_AGENT_DIR).
- Mcode `mcode_config.rs:12`: `~/.minimax` (MINIMAX_DATA_DIR/MAVIS_DATA_DIR); no settings override.

AppType (`app_config.rs:402-431`): Claude, ClaudeDesktop, Codex, Gemini, GrokBuild, OpenCode, OpenClaw, Hermes, Pi, Mcode.
Switch-mode: Claude, Codex, Gemini (+ Desktop via route). Additive: OpenCode, OpenClaw, Hermes, Pi, Mcode (`:455`). Local proxy: Claude, Codex, Gemini, GrokBuild (`:462`); Desktop has_route_mode without failover (`:472`).

Also: prompts (CLAUDE.md/AGENTS.md/GEMINI.md/SOUL.md), skills SSOT `~/.cc-switch/skills` or `~/.agents/skills`, usage/cost + session importers, WebDAV/S3, Copilot/Codex/xAI OAuth.

Storage: SQLite `~/.cc-switch/cc-switch.db` (`database/mod.rs:99-101`). settings.json + plugin-store for app_config_dir (`app_store.rs`). Legacy config.json path (`config.rs:359`).

## 3. Tauri command surface

`lib.rs:1429-1818` generate_handler. Rough count ~340 plus tray helpers.

Groups: providers/live switch; config/settings/updater; Claude MCP + unified MCP; prompts; profiles; model fetch/speedtest; import/export/WebDAV/S3/backups; deeplink; env; skills unified+legacy; auto-launch; proxy/mode/stack/circuit breaker/failover; usage + session sync; sessions; tool lifecycle; universal providers/icons; OpenCode/OpenClaw/Hermes/OMO/workspace; upstream proxy; auth + Copilot; lightweight mode.

## 4. Frontend map (`src/`)

- Entry `main.tsx`: QueryClientProvider, ThemeProvider storageKey `cc-switch-theme`, UpdateProvider, i18n.
- Nav `lib/navigation.ts`: app pages providers|workspace|openclawConfig|hermesMemory; global usage|auth|mcp|skills|prompts|sessions|apps; settings general|appConfig|routing|network|data|about. Keys `cc-switch-last-view` / `cc-switch-last-app`.
- Components: shell, providers (mode/forms), settings, usage, apps, skills, mcp, prompts, sessions, proxy, profiles, universal, openclaw, hermes, workspace, quota, ui (shadcn).
- State: TanStack Query (`lib/query/`) + hooks; no Zustand.
- i18n: zh, zh-TW, en, ja, tr; default zh, fallback en.
- Presets: `src/config/*ProviderPresets.ts`. API: `src/lib/api/*.ts` invoke wrappers.

## 5. Windows / WSL handling TODAY

Does not auto-detect WSL. User sets each tool dir to UNC e.g. `\\wsl.localhost\Ubuntu\home\user\.claude` (`settings.rs:484-498`). README lists Claude Code, Codex, Gemini, Grok Build, OpenCode, OpenClaw, Hermes, Pi.

- `is_wsl_path` (`config.rs:35-48`) UNC server wsl$ or wsl.localhost. Drive-letter maps not detected (`:33`).
- `atomic_write` (`config.rs:462-645`) skips ReplaceFileW on WSL (ERROR_NOT_SUPPORTED 50), rename fallback. Contract test `:780`.
- `derive_wsl_home_dir` (`config.rs:218`) for OMO ~/.omo when OpenCode dir is in WSL.
- App home is Windows Known Folder, not $HOME (`config.rs:9-16`). Test: CC_SWITCH_TEST_HOME.
- Codex state_*.sqlite on 9P skipped (no locks) in `codex_state_db.rs`.
- Tool version/install/uninstall: wsl.exe -d distro when env_type=wsl (`commands/misc.rs`; UI `AppsPage.tsx:571-679`). Distro from UNC path, not wsl -l.
- Proxy writes 127.0.0.1; WSL2 NAT cannot reach Windows localhost; docs require mirrored networking.

Not a dual control plane: one Windows process. No simultaneous Windows+WSL configs per app. Claude Desktop and Mcode have no *_config_dir override.

## 6. Model routing / local proxy / failover

Yes, first-class, in-process Axum/hyper (not Magpie).

- Default bind 127.0.0.1:15721 (`proxy/types.rs:42-47`).
- `proxy/mod.rs:1-3` HTTP proxy + failover + passthrough.
- `proxy/provider_router.rs:40-45` queue-ordered failover when auto_failover_enabled.
- `proxy/circuit_breaker.rs` closed/open/half-open; defaults fail 4 / success 2 / 60s / 60% (`:63-71`).
- Protocol bridges in `proxy/providers/` (Anthropic, Responses, Chat, Gemini, Copilot, xAI).
- `mode/`: Direct / Routing / Aggregation; live takeover of CLI base URLs.
- Usage from proxy (`proxy/usage/`) plus session-file importers.
- Codex Official excluded from failover (`provider_router.rs:18-21`).

[INFERENCE] Magpie duplicates this unless AgentDock wants Magpie scheduler as extra policy.

## 7. Tests and CI

- Frontend: `pnpm test:unit` Vitest (`vitest.config.ts` jsdom `{src,tests}/**/*.{test,spec}.*`); also typecheck, format:check.
- Backend: `cd src-tauri && cargo test` or CI `cargo nextest run --profile ci`. Integration `src-tauri/tests/`. clippy -D warnings, cargo fmt --check.
- CI `.github/workflows/ci.yml`: path-filter FE/BE; FE ubuntu Node 20; BE ubuntu-22.04/windows-latest/macos-latest; backend-windows-wsl2 UNC atomic-write contract. Nightly `wsl2-nightly.yml`. release.yml + sync-r2.yml.

## 8. License

MIT, Copyright 2025 Jason Young (`LICENSE`). Repo https://github.com/farion1231/cc-switch.

## 9. Coupling to change for AgentDock fork

Must remap: package/Cargo names; lib `cc_switch_lib`; productName; identifier `com.ccswitch.desktop`; updater pubkey/endpoints; scheme `ccswitch://` (`lib.rs:261`); Linux `cc-switch-handler.desktop` (`lib.rs:1067`); AppUserModelID; tray tooltip CC Switch (`lib.rs:1145`); `~/.cc-switch` / cc-switch.db (`config.rs:312-318`); theme/nav localStorage keys; Claude Desktop profile name CC Switch + UUID ...157210 (`claude_desktop_config.rs:21-22`) tied to port 15721; env CC_SWITCH_*; Flatpak com.ccswitch.desktop; ccswitch.io / support@ccswitch.io; R2; deplink.html; sponsor `source=ccswitch`; i18n/WiX/README.

Reusable as logic: live patch engine, per-CLI writers, MCP/skills/prompts, proxy+failover, WSL UNC atomic write, wsl.exe tool lifecycle, SQLite schema, React shell.

AgentDock gap: no dual inventory of Windows host and each distro at once; one override path per app; localhost proxy (WSL NAT break); no Magpie scheduler.
