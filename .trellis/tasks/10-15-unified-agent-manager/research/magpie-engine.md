# Magpie engine overview (routing, gateway, WSL, storage, sidecar)

> Scout research captured 2026-10-15. Paths: magpie=/mnt/d/magpie, cc-switch=/mnt/x/cc-switch.

# magpie research (AgentDock reuse)

## 1. What magpie is

Local control plane + LLM gateway for coding agents: one catalog of providers/subscriptions/local models, rewrites each agent's config to point at magpie, and translates + routes requests. MIT, Go 1.26.8 (go.mod:3). GUI is Wails v3, not Tauri. Binary: main.go:14 -> cli.Main.

Modes: desktop magpie / tray / app; TUI magpie tui; browser magpie web default 127.0.0.1:3430 (internal/cli/web_cli.go:17); HTTP gateway magpie serve; daemon magpie daemon install (Linux systemd user / macOS launchd; Windows = autostart app, not a Windows service; docs/reference.md:2102-2153). Dev ports 3426-3429 (Makefile:87-90).

Listen: gateway 127.0.0.1:3425 (internal/gateway/gateway.go:45-47; settings.DefaultPort=3425 at internal/settings/settings.go:1086-1118). Override MAGPIE_ADDR or settings.json port. Web UI is a second listener (serve --web, same process).

How CLIs attach: adapters in internal/agent rewrite the agent file to gateway URL + key magpie-<agent> (docs/integrating.md, docs/subsystems/agent-wiring.md). Generic env: OPENAI_BASE_URL=http://127.0.0.1:3425/v1 OPENAI_API_KEY=magpie; ANTHROPIC_BASE_URL=http://127.0.0.1:3425; GOOGLE_GEMINI_BASE_URL=http://127.0.0.1:3425. GET /api/hello returns name magpie (gateway.go:610-612). Usage attribution: Bearer magpie-<app-id>.

## 2. Model routing / scheduling

Config: ~/.config/magpie/providers.json (provider.Path at internal/provider/provider.go:435). Schema type file (provider.go:411-435): providers[], groups[], searches[], noAutoGroups, order, groupOrder, pluginLeft. Also magpie group add / Routing page. Auto-groups: same model name on several providers (derived unless edited).

- internal/provider: vendors, keys, accounts, groups. Provider provider.go:47-117; Group group.go:143-248; Rule grouprule.go:42-83; ServeGroups group_serve.go:52-55
- internal/gateway: resolve model, candidates, failover before first byte, affinity, sink, patience. Handler gateway.go:598+; candidate fallback.go:38-63
- internal/routes: order a pool. Seat/Strategy routes.go:20-60; Smart is core; Builtin strategies.go:216-223 (order/rotate/usage/pace/weight)
- plugins/groups: gates user groups (plugins/groups/groups.go:17-29)
- plugins/rotation: registers non-smart strategies (rotation.go:17-30)
- plugins/routing: Routing page only (routing.go:13-21)
- plugins/budget + internal/budget + internal/gwbudget: gateway-key token/cost windows from usage.jsonl (budget.go:1-46)
- internal/usage: Record usage.go:23+; Path() usage.jsonl usage.go:211-212; ledger.go Row
- plugins/usage: Usage page plugin

Routing modes (Provider.Routing / Group.Routing): empty=smart, order, rotate, usage, pace, weight, plus group manual (group.go:80). Affinity auto/session/turn/off/strict (group.go:47-63). Failover: Provider.Fallback[] then next key/account (fallback.go:1-14). Intent: Group.Classifier + Rule.Intent (grouprule.go:54-57). Nested groups allowed; loops refused (group.go:20-21).

## 3. Protocol translation

Upstream Protocol (provider.go:33-41): chat, responses, anthropic, gemini.

Client surfaces (gateway.go:598-671): POST /v1/chat/completions; POST /v1/responses (SSE; WS 426); POST /v1/messages and count_tokens; Gemini POST /v1beta/models/{call...} GET /v1beta/models; GET /v1/models; Codex GET /v1/codex/models; Codex shim /backend-api/codex (codex_backend.go:35); images/videos/embeddings/rerank/STT; POST /v1/systemone; MCP /mcp/magpie/web-search|/vision|/tools and /mcp/{name}; /v1/magpie/quotas|usage|route|limit|concurrency. Pass-through if vendor speaks client API; else translate.

## 4. Windows / WSL

internal/wslrun: from Windows magpie, find a running distro command (wsl.exe -l --running), never start stopped ones (wslrun.go:1-7,65-68,150-154). Tool: Distro/Path/PATH/Mount/Host/Root (wslrun.go:31-38). Linux() maps C:\ to /mnt/c (336-348). Env uses WSLENV + NAT loopback rewrite (355-395). Exe() is Windows magpie path as seen in distro (448-456). Distro list UTF-16 (413-415). NoWSLAgents disables Find (65-68).

internal/agent/wsl.go: probe each running distro; agents become id@wsl:distro; files via \\wsl.localhost\distro else \\wsl$\ (wsl.go:31-42,929-933). State wsl.json. Gateway 127.0.0.1 if mirrored/consomme (wslrun.HostLoopback); NAT uses Windows host IP and needs gateway beyond loopback.

Also sessions/wsl.go, library WSL skill copies, imagemcp/wsl.go. GOOS==windows is the on-switch (wslrun.go:41). Does not run a second magpie inside WSL by default; Windows process manages both sides.

## 5. Skills / MCP / CLI configs -- yes

Library (internal/library, docs/subsystems/library.md): library.json + library/ for instructions, MCP servers, skills, hooks, subagents, RTK; writes each agent's format; WSL agents get copies. Agent wiring edits CLI configs via format-preserving internal/edit. Sessions read agent files/DBs; magpie does not replace CLIs.

internal packages (~65): access gateway keys; agent adapters; agentenv env sandbox; agentplug plugin agents; appdir config/cache/portable; autostart login; awake prevent sleep; backup sealed backup; budget key limits; catalog models.dev; claudebridge Claude-sub MCP helper; claudecode find/install claude; cli commands; codexcat Codex catalog; daemon serve-as-service; davsync WebDAV/S3/GitHub; desktopdir Claude Desktop dirs; dirent dir walk; edit JSONC/TOML/YAML; filelock; filememo; fonts; fx USD/CNY; gateway LLM proxy; ghtoken; gui Wails+web /api; gwbudget; gwmw; gwtool; iconproxy; imagemcp stdio; jsmod moejs; lastgood; libc glibc/musl; library; masking; mcpauth OAuth; middleware JS; netproxy; omarchy; otlp; plugin npm/Bun; pluginrt; proc exec; profile; provider; qoder; redact; routes; sessions; settings; shortcut; source mirrors; sqlitedb modernc env fix; stats telemetry; steady racy IO; testenv; tui; tune compact/cache; update; upstream status pages; usage jsonl; usageapi; wslrun; zed.

## 6. Management API

Yes. internal/gui.Handler (api.go:789) serves UI + JSON /api/*. Same handler for desktop webview and magpie web. Not on 3425 by default: web 3430, gateway 3425. magpie serve --web runs both in one process.

Core (api.go): GET /api/state, POST /api/set, GET /api/drift, POST /api/agents/{action}/{id}, GET/POST /api/settings, POST /api/sync, POST /api/profile/{action}.

More HandleFunc: groups gui/groups.go:355-359; providers gui/providers.go:950,1046; library gui/library.go:141+; plugins gui/plugins.go:508+; usage gui/usage.go:99+; sessions gui/sessions.go:70+; traces gui/trace.go:247-296; backup gui/backup.go:21+; caller-keys gui/caller_keys.go:49+; gateway take-over gui/gateway_takeover.go:45-50.

Gateway extras: /v1/magpie/quotas, /usage, /route (gateway.go:613-622) -- not full admin CRUD.

## 7. Storage

No magpie-owned SQLite schema. Flat JSON under ~/.config/magpie (docs/reference.md:2714-2773; settings.Dir = appdir.Config() settings.go:986-988). Windows: user folder /.config/magpie unless XDG_CONFIG_HOME. Portable: data/ beside exe (appdir.go:1-21).

Files: settings.json, providers.json (0600), library.json, plugins.json/plugins.yml, caller-keys.json, logins.json, stash.json, applied.json, usage.jsonl, routing/<date>.jsonl, affinity.json, wsl.json. Cache ~/.cache/magpie.

internal/sqlitedb only sanitizes env then registers modernc.org/sqlite so magpie can read other apps' DBs (Codex/OpenCode/Alma/Cursor/CC-Switch import). Not magpie's ledger.

## 8. Build / test / license

MIT (LICENSE, 2026 yetone). Go 1.26.8. make build (GUI, production + Linux gtk3); make cli CGO_ENABLED=0 -tags nogui sidecar; make test = go vet && go test; make test-ui Playwright; make release-cli cross-compiles nogui including windows/amd64+arm64 (Makefile:33-87). Community PRs generally not accepted (AGENTS.md / README).

## 9. Embedding in a Tauri host

Sidecar binary is the designed shape. Evidence: make cli / release-cli produce cgo-less magpie (gateway+CLI+TUI+web) Makefile:38-39,80-87. magpie serve --web = one process, gateway :3425 + control UI/API :3430 (cli/main.go:106-108, web_cli.go:17, gui/web.go:67-72). Full CRUD is HTTP /api/* (gui-shell.md). magpie web auth: run key / MAGPIE_WEB_KEY cookie (web.go:67-72). Always needs a TCP port (or two). No Unix-socket or Rust FFI. internal/ is not a stable library API. Desktop magpie is Wails, not Tauri -- do not reuse internal/gui as AgentDock shell; reuse serve + /api as sidecar, or copy packages into a fork.

AgentDock mapping [INFERENCE]: Tauri (from cc-switch) owns Windows+WSL UX; spawn magpie-cli.exe serve --web with MAGPIE_ADDR=127.0.0.1:3425; call /api/state, /api/groups, /api/library, /api/agents/*. Magpie already dual-homes Windows host + WSL distro agents.
