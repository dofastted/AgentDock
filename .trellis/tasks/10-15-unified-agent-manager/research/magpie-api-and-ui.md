# Magpie /api auth, agent list, WSL twins, built-in UI

> Scout research captured 2026-10-15. Paths: magpie=/mnt/d/magpie, cc-switch=/mnt/x/cc-switch.

Full report also at local://magpie-agentdock-research.md.

Two ports: gateway 127.0.0.1:3425 (gateway.go:47) and magpie web 127.0.0.1:3430 (web_cli.go:16). magpie serve --web starts both (web_cli.go:169). Handler embeds assets (api.go:142,789).

Auth: cookie magpie_web_<port> from /?k= or MAGPIE_WEB_KEY (web.go:67,196). SameSite=Lax HttpOnly. /api has no CORS. CORSOrigins is gateway-only (cors.go:17). noFramedAPI 403s Origin:null or Sec-Fetch-Site:cross-site (pluginpage.go:215, api.go:2211). Tauri webview fetch to 127.0.0.1 is blocked; proxy via Rust or load magpie page same-origin.

WSL rows: id@wsl:<distro> in /api/state agents[] (no wsl field). Stopped distros asleep (wsl.go:746), last values in wsl.json, set starts distro. NAT needs gateway beyond loopback. Library copies skills (targets.go:39,569).

WSL twins: codex,pi,omo,prime-agent,stepcode,claude,opencode,mimocode,kimi,qwen,omp,crush,hermes,morph,grok,droid,fx,commandcode,minimax-code,dsh,empryo,ante,muse,qoder,qoder-cn,codebuddy,cline,atomcode,snow,gemini,goose,mothx,reasonix (wsl.go:288).

nogui: api.go/web.go untagged; gui_off.go only disables Wails. Sidecar: CGO=0 -tags nogui, modernc sqlite, no admin. No exclusive lock on serve; take-over can kill other magpie (gateway_takeover.go:45). Vanilla UI: assets/ app.js 1.2MB i18n 1.5MB; pages Agents/Sessions/Library/Providers/Routing/Gateway/Usage/Plugins/Settings.


## Full report

# Magpie GUI/API research for AgentDock

Two ports, one process. Gateway (LLM): `gateway.DefaultAddr` **127.0.0.1:3425** (`internal/gateway/gateway.go:47`). Control UI: **`magpie serve --web`** -> `gui.StartWeb` on **127.0.0.1:3430** (`internal/cli/web_cli.go:16-17,169-207`). `--web=host:port` overrides UI bind. Gateway bind is `MAGPIE_ADDR` else Settings port (`gateway.go:222-232`). `Handler` is `internal/gui/api.go:789-890` (`//go:embed assets` at 142-143).

## 1. /api route table

Writes typically return refreshed page JSON (`stateJSON`, `settingsJSON`, `groupsState`, library view). Errors: `{error, code?}`.

### State / agents
- GET `/api/state` api.go:844 — `stateJSON`: `agents[]` (`id,name,icon,path,fields[{key,label,value,options}],drift,wired,cliMissing`), `clients[]`, `profiles[]`, `settings`, `served`, `notice?`. `?last=1` cached. **No `wsl` field** — WSL rows are extra agents `id@wsl:<distro>`.
- POST `/api/set` :863 `{agent,field,value}` -> state
- GET `/api/drift` :894 `{[id]: Drift}`
- POST `/api/agents/rescan` :908 -> state
- GET `/api/agents/cli` :912 `{agents,pending}`
- GET `/api/agents/install` :918
- POST `/api/agents/stage/{id}` :922 `{field,value}` Aside offline
- POST `/api/agents/cli/{id}` :950
- GET `/api/agents/preview/{id}` :966 `{revision}`
- POST `/api/agents/disconnect-offline/{id}` :993 `{revision}`
- POST `/api/agents/{action}/{id}` :1027 action=connect|disconnect|reapply|keep; connect returns `connected` + `notice`
- POST `/api/agents/arrange` :1847 `{order,hidden,shown}`
- GET/POST `/api/agent-models/{id}` agent_models.go:199,227 POST `{hidden,shown}`
- POST `/api/agent-fast` :184 `{for,ref,fast}`
- POST `/api/profile/{action}` api.go:1060 `{name}`
- POST `/api/sync` :1100 catalog -> state

### Providers / groups
- GET `/api/providers` providers.go:950 (`?wait`)
- POST `/api/providers/arrange` :980 `{order}`
- POST `/api/provider/{action}` :1046 save/on/off/delete/test/detect/list/models/route/key/arrange/account*/move/adopt embeds `provider.Provider`
- GET `/api/lanes` `/api/upstream` :926,:937
- GET `/api/gateway/call` :961 `?id=` recent bodies
- GET `/api/groups` groups.go:356 groups, models, pools, found, templates
- POST `/api/groups/{action}` :359 found/save/delete/show/copy/share/import/template/switch/arrange; `{Group,from,on,order,ids,kind,text}`
- POST `/api/keys/{action}` providers.go:1975 add/import/use/remove/on/off `{id,key,name,ref,refs}`
- POST `/api/login/{action}` :1918 switch/forget/on/off `{agent,user}`
- GET `/api/login/usage` :1911 `?agent=`
- POST `/api/signin` + GET/POST `{id}` :2038+ OAuth
- POST `/api/codex/daemon/{action}` :1954 restart/dismiss

### Library
GET `/api/library` library.go:142 (`?glance=1`). GET skill/market/icon/rtk. POST skills/{probe,check,regroup}, rtk*, projects/choose, reveal, mcp-signin*, mcp/check, mcp/config.
POST `/api/library/{what}/{action}` :427 — instructions/save|import; servers/save|agents|tools|remove|import; hooks/save|agents|remove; subagents/save|agents|remove; skills/install|update|agents|remove|group|import|how; market/server|skill; projects/*; **all/sync**. Body: `name,old,agents,server,hook,subagent,source,paths,names,dir,copy,on,how,replace`. Response: library view + `Result`.

### Usage / sessions / context / tune
GET `/api/usage`, `/requests`, `/heatmap`, `/requests/content`, `/requests.csv`, `/quotas`, `/quotas/history`, `/quotas/history/holds`. POST quotas/refresh, *-checkin, requests/export, codex-reset, usage/arrange.
Sessions: GET list/stats/overview/progress/one/transcript/markdown/manage/search; POST export/terminal/relocate-claude/recording/delete/restore/purge/codex-provider.
GET `/api/context`, `/context/text`, `/api/tune`; POST tune/apply|undo.
GET `/api/gateway/trace|history|route`; POST session-titles, unrest.

### Settings / plugins / backup / caller-keys / takeover
GET/POST `/api/settings` api.go:1131. Many POST `/api/settings/*` (quota-left, lan, **cors**, allowips, **port**, redact-rules, github-token, search-*, gateway-mode, archive, login).
POST `/api/proxy/check`, `/api/copy` `{text}`, `/api/open` `{url}`, `/api/window/{hide|main|quit|fit|tint|titlebar}`.
Plugins: GET plugins, updates, listings, github, git, npm, market, search, page, plugin-pages, plugins/builtin. POST plugins/{add,remove,update,upgrade,off,options,prefer}, choose, check, mirror, plugin-signin*, plugin-pages/read|options, plugins/builtin `{id,on,config}`.
Backup: POST backup/export `{pass,keys,library}`, import `{data,pass,agents,library}`. GET/POST davsync/{save,now,off,restore,undo,upload,auto,reveal}. GET/POST archive*.
Caller keys: GET `/api/caller-keys`, `/models`; POST `{action}` `access.Change` -> `{keys,secret?}`.
POST `/api/gateway/take-over` | `/restart` -> `gatewayFixJSON{ok,reason,pid,path,port,version}` (**can SIGTERM/KILL another magpie on :3425**, gateway_takeover.go:45-75).
Also: fonts (desktop only), omarchy, update*, whatsnew*, cli, importapps, import/{id}, icons, stepfun. Dev-only `/api/dev/*`.

## 2. Auth / CORS / Tauri

**Web mode (`StartWeb`)** web.go:67-71,196-255:
- Run key in `/?k=` exchanged for cookie `magpie_web_<port>` (e.g. `magpie_web_3430`), HttpOnly, **SameSite=Lax**, Path=/
- Env: `MAGPIE_WEB_KEY` (fixed, min 16 chars `[A-Za-z0-9-._~]`, cookie 400d) or per-run hex; `MAGPIE_WEB_RUNKEY` after update
- Missing/wrong cookie -> 401. No Bearer for /api
- `gateway.AllowGuard` IP allow-list wraps the **web** server (web.go:106)

**/api has no Access-Control-Allow-Origin.** CORSOrigins (`settings.go:112`, `gateway/cors.go:17`) is for the **gateway :3425**, not control /api.

**noFramedAPI** (api.go:2211, pluginpage.go:215-237) 403s `/api` if Origin is `null`, Sec-Fetch-Dest is iframe/frame/embed/object, or **Sec-Fetch-Site is cross-site and Dest is not document**.

**Tauri:** fetch from `tauri://localhost` or `http://tauri.localhost` to `http://127.0.0.1:3430/api` is cross-origin. Cookie is bound to 127.0.0.1, not sent. WebView2 typically sets Sec-Fetch-Site=cross-site -> **403**. CORS blocks reading even if 403 were lifted. Listing tauri origin in gateway CORS does **not** help /api.

Works: (1) webview navigates to `http://127.0.0.1:3430/?k=KEY` (same-origin magpie UI); (2) Tauri Rust sidecar HTTP (no Origin/Sec-Fetch) then IPC; (3) disable web security — fragile. Prefer (2) for cc-switch React.

## 3. Agent adapters

`builtins()` agents.go:88-155. IDs: claude, claude-desktop, claude-science, codex, gemini, agy, opencode, openchamber, mimocode, deveco, pi, stepcode, mothx, aside, omo, prime-agent, goose, cursor, cursor-local [INFERENCE from cursorLocal()], zed, vscode/insiders/vscodium, copilot + JetBrains, air, crush, dsh, reasonix, commandcode, fx, omp (+ omp#profile), devin, hermes, morph, kimi, qwen, muse, empryo, ante, astrbot, jcode, minimax-code, droid, cline, qoder, qoder-cn, grok, grok-app, zcode, workbuddy + AI, codebuddy, pencil, raycast, t3code, hanako, atomcode, snow, alma, cindy, lody + plugin agents. Usage-only: magpie, curl, pi-desktop.

Typical files: Claude `~/.claude/settings.json` + `~/.claude.json`; Codex `~/.codex/config.toml`; OpenCode `~/.config/opencode/opencode.json`; Pi `~/.pi/agent`; omp `~/.omp/.../config.yml`. Magpie only rewrites what it wrote (`applied.json` / `stash.json`).

**WSL twins** (`wslKinds` wsl.go:288-577): codex, pi, omo, prime-agent, stepcode, claude, opencode, mimocode, kimi, qwen, omp (+ profiles), crush, hermes, morph, grok, droid, fx, commandcode, minimax-code, dsh, empryo, ante, muse, qoder, qoder-cn, codebuddy, cline, atomcode, snow, gemini, goose, mothx, reasonix. Not Desktop/IDE-only (claude-desktop, zed, vscode, cursor, raycast, cindy, lody).

## 4. WSL details

Probe running distros only (`wsl.exe -l -q`; never start stopped). ID `codex@wsl:Ubuntu`, Name `Codex · WSL Ubuntu`, `Agent.WSL=distro` (agent.go:213). **Not serialized on agentJSON** — UI parses `@wsl:` (app.js:3410). Settings GET `/api/settings` has `wsl` bool (api.go:116).

Stopped: `asleep()` wsl.go:746 — Path/Dir empty; fields from `wsl.json` last values; Notice "WSL {distro} isn't running" (wsl.go:1425) on connect/set; **set starts the distro**. Sessions: cached listing, not opened (gui-shell.md).

**NAT vs mirrored** (wsl.go:31-42,234-255): mirrored/consomme -> `gateway.URL()` (127.0.0.1:3425). NAT -> `http://<windows-as-seen-from-WSL>:<port>` — **gateway must listen beyond loopback** (`MAGPIE_ADDR=0.0.0.0:3425` or Settings LAN share) or WSL agents cannot reach it. `localhostForwarding=false` -> noRelay notice (wsl.go:1424).

Library: `wslTargetOf` Copy=true (targets.go:39-41,569-575); skipped if distro not running. Skills copied through `\\wsl.localhost\`, not links.

## 5. Magpie frontend

Vanilla JS, no bundler. `internal/gui/assets/` embedded. Size: app.js ~1.2MB, i18n.js ~1.5MB, app.css ~328KB, index.html ~36KB + plugin pages + ~170 icons. Pages: Agents, Sessions, Library, Providers, Routing, Gateway, Usage, Plugins, Settings (General, Allowances, Network, Models, Privacy, Observability, Sync, About). Plugin slots: gateway/routing/library/sessions/usage. **This is a full alternative UI** already served by the sidecar.

## 6. -tags nogui still has /api

`api.go`, `web.go`, library/providers/groups/sessions/usage: **no build tags**. Only Wails host (`app.go:1 //go:build !nogui`). `cli/gui_off.go`: hasGUI=false, runGUI errors; **`serve --web` still calls `gui.StartWeb`**. Makefile `cli:` / `release-cli:` `CGO_ENABLED=0 -tags nogui`. **Sidecar should be this CLI binary, not the Wails GUI exe.**

## 7. Windows sidecar packaging

- **No CGO, no admin.** SQLite is `modernc.org/sqlite` via `internal/sqlitedb` (pure Go).
- Windows GUI app also CGO=0 (WebView2, Makefile:101-111) — **not needed for serve --web**.
- **No exclusive magpie lock** on `serve`. GUI single-instance is Wails/flock (`instance_*.go`) — nogui sidecar does not take it.
- **Port conflict:** first magpie keeps :3425; second `serve --web` still serves UI/API, `gw==nil`, watches 15s (`backend.go:70-88`). Shared files use `filelock` (logins.json.lock). **`/api/gateway/take-over` kills the other magpie** — AgentDock must not call this against a user-installed magpie without consent.
- Bind :3425/:3430 as the installed magpie would; set `MAGPIE_WEB_KEY` for a stable cookie; for WSL NAT bind gateway `0.0.0.0` or LAN share.

