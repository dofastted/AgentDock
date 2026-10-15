# cc-switch React frontend inventory and invoke coupling

> Scout research captured 2026-10-15. Paths: magpie=/mnt/d/magpie, cc-switch=/mnt/x/cc-switch.

# cc-switch React frontend inventory (AgentDock planning)

Nav model: `src/lib/navigation.ts:11-29`. Shell: `src/App.tsx` (`renderAppPage` 1358, `renderGlobalPage` 1426, `renderContent` 1513). Sidebar: `src/components/shell/Sidebar.tsx` (apps + globals mcp/skills/prompts/sessions/auth/usage + footer Apps/Settings; settings directory 612).

## 1. Page inventory

| View | Entry | ~LOC | API / commands |
|---|---|---|---|
| **providers** | `App.tsx:1280` -> `SwitchModePanel` 92 (~667) if MODE_APP; else `ProviderList` 93 (~634) | + `ProviderForm` 323 (**3064**) | `providersApi` CRUD/switch/sort/live-import/editor_view/tray; `proxyApi` mode/takeover/stack/route; `piApi`; `omoApi`; `openclawApi`; `hermesApi`; `failoverApi`; `usageApi.query`; `vscodeApi`; `model-fetch`; `auth`/`copilot` |
| **workspace** (openclaw) | `WorkspaceFilesPanel` 55 (~165) + `DailyMemoryPanel` | `workspaceApi` read/write/list/search/open dir |
| **openclawConfig** | `App.tsx:1364` env/tools/agents | `EnvPanel` (~110), `ToolsPanel` (~288), `AgentsDefaultsPanel` (~415) | `openclawApi` get/set env, tools, agents, health |
| **hermesMemory** | `HermesMemoryPanel` 264 (~352) | `hermesApi` memory, limits, web UI, dashboard |
| **usage** | `UsagePage` 14 (**44**) wraps `UsageDashboard` 154 (**871**) | `usageApi` summary/trends/stats/logs/pricing/session-sync |
| **auth** | `AuthCenterPanel` 21 (**72**) | `authApi` (8), `copilotApi` (~15), `subscriptionApi` |
| **mcp** | `UnifiedMcpPanel` 118 (**1146**) | `mcpApi` unified getAll/upsert/delete/toggle/import/resync |
| **skills** / **skillsDiscovery** | `UnifiedSkillsPanel` 119 (**2097**) | `skillsApi` installed/discover/install/uninstall/toggle/repos/zip |
| **prompts** | `PromptPanel` 583 (**599**); Pi -> `PiPromptPanel` | `promptsApi` CRUD + Pi native files/templates |
| **sessions** | `SessionManagerPage` 136 (**1464**) + reader/ | `sessionsApi` list/stream/delete/export/reveal/terminal; `piApi.getSessionDiscovery` |
| **apps** | `AppsPage` 111 (**794**) | `settingsApi` tool versions/lifecycle/installs |
| **settings/general** | `GeneralSection` 39 (**252**) | `settingsApi` get/save, auto-launch, terminal |
| **settings/appConfig** | `AppConfigSection` 74 (~220) | per-app config dirs + Claude plugin + Codex auth |
| **settings/routing** | `RoutingSection` 93 (**511**) | `proxyApi` start/stop/global config; rectifier/optimizer |
| **settings/network** | `GlobalProxySettings` | `globalProxy` get/set/test/scan |
| **settings/data** | import/export, backups, WebDAV/S3, logs | `settingsApi` dialogs + webdav/s3; `backupsApi` (8) |
| **settings/about** | `AboutSection` | `getVersion`, updater |

OpenClaw tabs: `App.tsx:1234-1251`. Hermes: `1253-1266`. Claude Desktop prompts/sessions share Claude via `sharedFeatureAppOf` (`navigation.ts:123`).

## 2. src/lib/api/* HTTP remap seam

All invoke wrappers. Counts = exported command functions (not types).

| Module | Fns | Domain |
|---|---|---|
| providers.ts | 24 + universal 5 | per-app provider CRUD, live import, editor view, tray |
| settings.ts | ~50 + backupsApi 8 | settings, dirs, dialogs, WebDAV/S3, tools, rectifier, logs |
| proxy.ts | 24 | :15721 gateway, takeover, stack |
| skills.ts | ~25 | unified skills + repos + zip |
| usage.ts | 23 | proxy logs + session usage + pricing |
| mcp.ts | 13 | unified MCP + legacy Claude MCP |
| prompts.ts | 11 | prompt files + Pi templates |
| openclaw.ts | 11 | env/tools/agents/catalog |
| sessions.ts | 10 | session files/reader/terminal |
| vscode.ts | 9 | live settings + endpoint speed test |
| workspace.ts | 8 | OpenClaw workspace/memory files |
| auth.ts | 8 | managed OAuth (copilot/codex/xai) |
| copilot.ts | 15 | GitHub Copilot device flow/quota |
| failover.ts | 9 | circuit breaker + failover queue |
| hermes.ts | 6 | memory + web UI |
| deeplink.ts | 6 | ccs:// import |
| omo.ts | 6 | OpenCode OMO / OMO-slim |
| profiles.ts | 5 | Claude/Desktop/Codex snapshots |
| globalProxy.ts | 5 | outbound HTTP/SOCKS |
| subscription.ts | 5 | official quotas / coding-plan / balance |
| env.ts | 4 | env-var conflict cleanup |
| customIcons.ts | 4 | icon library |
| model-fetch.ts | 4 | /models fetch |
| pi.ts | 3 | Pi current state / usage script |
| connectivity-check.ts | 1 | stream health |

[INFERENCE] ~280 invoke command names. Re-pointing = swap invoke in these modules for fetch to magpie /api/*; UI mostly stays.

## 3. AppId coupling (tight)

AppId (`lib/api/types.ts:2-12`) = 10: claude, claude-desktop, codex, gemini, grokbuild, opencode, openclaw, hermes, pi, mcode.

appConfig.tsx: PROXY_APP_IDS (claude/codex/gemini/grokbuild), MODE_APP_IDS (+claude-desktop), STACK_APP_IDS (claude/codex/claude-desktop), ADDITIVE_APP_IDS (opencode/openclaw/hermes/pi/mcode), MCP_APP_IDS, SKILLS_APP_IDS, EDITOR_VIEW_APP_IDS.

Switcher: Sidebar maps APP_IDS x visibleApps. OpenClaw/Hermes get extra PageTabs.

Forms: ProviderForm.tsx:323-332 early-returns mcode/pi/claude-desktop/grokbuild dedicated forms; remainder is a huge appId === tree.

Presets: src/config/{claude,claudeDesktop,codex,gemini,grokBuild,opencode,openclaw,hermes,pi,mcode}ProviderPresets.ts plus universalProviderPresets.ts.

AgentDock cannot treat AppId as a cosmetic tab; magpie needs equivalent per-agent config shapes or a large form rewrite.

## 4. Data layer

Query: src/lib/query/ — queries.ts ([providers, appId], [settings], [sessions], usage script), usage.ts (usageKeys.*), proxy.ts (proxyKeys, [providers, app, mode|direct|stack]), prompts.ts, profiles.ts, pi.ts, omo.ts, failover.ts, subscription.ts, copilot.ts, customIcons.ts, sessions.ts, mutations.ts.

Also: hooks/useMcp.ts [mcp,all]; hooks/useSkills.ts [skills, installed|discoverable|repos|unmanaged|backups]; hooks/useOpenClaw.ts / useHermes.ts.

Direct invoke outside lib/api:
- App.tsx:539,557 get_migration_result, get_skills_migration_result
- main.tsx:95 get_init_error
- theme-provider.tsx:125 set_window_theme
- DatabaseUpgrade.tsx open_external, open_app_config_folder
- PathChip.tsx:56 reveal_session_path (dup sessionsApi)
- SessionMarkdown.tsx:585 open_external
- useTrayNavigation.ts:95 tray_app_page_seen
- lib/clipboard.ts:5 copy_text_to_clipboard

Events (listen / useTauriEvent): configLoadError (main), deeplink-pending, tray-navigate, usage-log-recorded, provider-switched, universal-provider-synced, profile-applied.

## 5. Tauri-only (keep in thin Rust shell)

- Tray: update_tray_menu, tray-navigate, tray_app_page_seen — keep
- Window: WindowControls.tsx min/max/close; App.tsx:503 decorations; set_window_theme — keep
- Updater: lib/updater.ts plugin-updater; settingsApi.checkUpdates / install — keep
- Deep link: deeplinkApi + DeepLinkImportDialog — keep (or magpie URL scheme)
- Dialog: plugin-dialog only main.tsx:18 message(); file pickers are invoke pick_directory/open_file_dialog — keep
- Opener: no plugin-opener; open_external / open_*_folder / reveal_* invokes — keep
- Store: not used
- Log: plugin-log via frontendLogger.ts — optional
- Process: plugin-process exit — keep
- Path: homeDir/join in useDirectorySettings — keep
- App version: getVersion() — keep
- Clipboard: native invoke + web fallback — keep

## 6. Effort signal [INFERENCE]

Maps cleanly to magpie generic concepts:
- MCP list (UnifiedMcpPanel + mcpApi unified) -> magpie library MCP
- Skills (UnifiedSkillsPanel + skillsApi) -> magpie skills
- Prompts (minus Pi native files) -> instructions
- Usage charts if magpie emits comparable stats
- Provider list/switch UI if magpie providers/groups replace per-CLI writers
- Auth-center device-code flows if magpie owns OAuth

cc-switch-specific (high rewrite / drop):
- Local gateway :15721 (proxyApi, SwitchModePanel, RoutingSection, failover/stack/rectifier)
- Claude Desktop routing + official provider seed
- OpenClaw env/tools/agents + workspace daily memory
- Hermes memory files + hermes dashboard
- Additive live-config merge (opencode/openclaw/hermes/pi/mcode providerKey IDs)
- Per-app preset catalogs and ProviderForm 3k-line switchyard
- Profiles (Claude/Desktop/Codex snapshots)
- Apps page CLI install/upgrade/WSL shells
- Session log reader over per-CLI session files
- SQLite-backed usage pricing / models.dev sync

## 7. Size

- App.tsx 1691; ProviderForm 3064; skills 2097; sessions 1464; MCP 1146; usage dashboard 871; apps 794; sidebar 689.
- Frontend LOC [INFERENCE] ~70-85k in src/**/*.{ts,tsx} (glob capped at 200 files; i18n JSON extra).
- Vitest: tests/** glob capped at 200; plus 16 colocated src/**/*.test.* => >=216 files.

