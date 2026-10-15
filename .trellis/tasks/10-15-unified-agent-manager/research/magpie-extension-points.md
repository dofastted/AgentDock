# Magpie extension points for G1-G5, data dir, importers, branding

> Scout research captured 2026-10-15. Paths: magpie=/mnt/d/magpie, cc-switch=/mnt/x/cc-switch.

# Magpie extension points (AgentDock)

## 1. Plugin/page mechanism (G1 environments page)

**Built-in (use this for G1).** Not community `magpie.page` iframes: those cannot call `/api` (`pluginpage.go:25-32`).

Registration:
- `pluginrt.Register(Manifest)` from `init` (`pluginrt.go:127-153`). `API: "^1"`, `Requires: []string{"window"}`, `Parts: [{Slot:"nav.page", Key:"environments"}]`, `Client: []string{"i18n.js","environments.css","client.js"}`.
- `plugins/all/all.go:5-19` blank-imports every plugin. **Must add** `_ "github.com/yetone/magpie/plugins/environments"` (`TestPluginsImportOnlyCore`, `pluginrt_test.go:94-147`). Only `main.go:8` imports `plugins/all`. **No build tags.**
- Client files: `internal/gui/assets/plugins/<id>/`. Kernel: `magpie.plugin(id, setup)` + `ctx.slots.add("page", {key, html, title, loads, mount, open, …})` (`plugin-runtime.md` P1; pattern `plugins/gateway/client.js` ending `magpie.plugin("gateway", …)`).
- **index.html is not automatic.** `TestBuiltinClientsLoaded` (`builtins_test.go:182-225`) requires: CSS in `<head>`, scripts between `kernel.js` and `app.js` (`index.html:446-474`); `page.js` only between `app.js` and `start.js`. Nav buttons for core pages are hardcoded (`index.html:47-59`); plugin pages appear via kernel `page` slot (`index.html:124-125`). Mark rows `data-feature="environments"` so off hides them.
- `/api` routes: **not** via pluginrt. Add `internal/gui/environments.go` + call from `Handler` next to `libraryRoutes` (`api.go:1128`). Plugin package may import `internal/*` only, never another plugin.

**Minimal new-plugin steps (additive):**
1. New `plugins/environments/environments.go` (copy `plugins/usage/usage.go:7-16`).
2. New assets under `internal/gui/assets/plugins/environments/`.
3. One line in `plugins/all/all.go`.
4. `<link>` + `<script>` tags in `index.html`.
5. New `internal/gui/environments.go` routes; optional `internal/environments/` package.
6. `i18n.js` via `magpie.strings({zh,…})`.

Community alternative (`internal/plugin/page.go:32-74`, `package.json` `magpie.page`) is iframe-only + `PageReads` — too weak for G1.

## 2. WSL probing + G4 per-distro allow/deny

**List/probe (never starts stopped distros):**
- `wslrun.On` Windows-only (`wslrun.go:40-41`). `running()` = `wsl.exe -l --running -q` (`wslrun.go:131-148`). `Up(distro)` `wslrun.go:156-161`. `Find` also gated by `settings.Load().NoWSLAgents` (`wslrun.go:65-68`).
- Agents: `wslLooks()` = `wslOn && !NoWSLAgents` (`wsl.go:933-937`). `wslAgents()` `801-807` → `wslDistros()` → `wslAgentsOf` (`809-816`) per `wslKinds` (`288+`). ID `<kind>@wsl:<distro>`.
- Probe: `wslProbeScript` (`wsl.go:1168-1220`) prints `home:`, `dir:`, `bin:`, `route:` (NAT host), `net:` (`wslinfo --networking-mode`). Cached in `wsl.json` (`wsl.go:879-882`) so asleep distros list last-seen fields (`asleep`, `noticeWSLAsleep` `1425`).
- Networking: `distro.Net`, `Mirrored`, `Gateway` (`wsl.go:166-196`). `mirror(cfg)` `234-237` uses `wslrun.HostLoopback` or `.wslconfig` if `Net==""`.

**G4 hook:** today **only global** `Settings.NoWSLAgents` (`settings.go:344-347`). Filter in `wslAgentsOf` loop (`wsl.go:811`) and `wslHomes`/`wslLooks` callers. Suggested field next to NoWSLAgents: `WSLDistrosOff []string` or `WSLAgents map[string]bool` + `KeepOwn` (`settings.go:931-938`; test `lightweight_test.go:76-86`). `wslrun.Find` should honor the same list [INFERENCE].

**Settings save:** `GET/POST /api/settings` (`api.go:1131-1139`). Prefs UI posts `noWSLAgents` (`app.js:18532-18535`, `prefsKeep` `21563-21564`). POST keeps many maps from current (`api.go:1178+`); a **new field on Settings unmarshals from the body** unless you add a dedicated `POST /api/settings/wsl-distros`. Include in `KeepOwn` so sync/backup does not clobber this machine.

## 3. Gateway reachability (G1)

- Twin gateway URL: `distro.base()` `wsl.go:250-255` — loopback `gateway.URL()` if mirrored/consomme, else `http://<NAT host>:<port>`.
- `HostLoopback` `wslrun.go:251-266`: mirrored|consomme|virtioproxy.
- Drift `unreachable` `reach.go` ~240-261: TCP to pointed-at addr vs `gateway.URL()`; WSL NAT advice + firewall + mirrored `.wslconfig`. `wslMovedFrom` `reach.go:269-298` offers new host. GUI test `internal/gui/tests/agent-unreachable.test.cjs`.
- `gatewayMode` is **web UI hide-Agents**, not WSL reach (`gatewaymode.go:33-36`). Do not reuse for G1 env cards.
- G1 data already in `distro`: Name, Running/`live()`, Net/Mirrored, Gateway, Has → agent count. Windows row = non-WSL `agent.All()` + `gateway.URL()` listen.

## 4. G5 sync model/provider/library to twins

**Set model/field:** `POST /api/set` `{"agent","field","value"}` (`api.go:863-892`) → `agent.Pick`. Twin id e.g. `codex@wsl:Ubuntu`. CLI: `magpie <agent> <field> <value>`.
**Catalog visibility is already shared:** `ListsFor()` strips `@wsl:` (`wsl.go:619-631`). `POST /api/agent-models/{id}` `{hidden,shown,order,only,ownHidden}` (`agent_models.go:226-235`) keys `HiddenModels`/`PickedModels` by ListsFor — copying those maps is a no-op.
**Need copy:** each `Field` value (`model`, `effort`, …) via Pick on dest; Connect/Disconnect (`POST /api/agents/{action}/{id}`). Applied/stash keyed by **full** id (`place.key` `wsl.go:155-163`).
**Library bindings:** `library.json` — `Instructions.Agents`, `Server.Agents`, skill agents, `Hook.Agents` (`hooks.go:40-56`), `Subagent.Agents` (`subagents.go:46-53`). Add dest id then `library.Sync()`. Skills to WSL are **copies** (`targets.go:39-41`, `wslTargetOf` `569-612`). Hooks/subagents currently cannot be given to WSL (G2).

## 5. G2 hooks/subagents to WSL (Go, not only JS)

Refusals (must change both):
- `hookSpecOf`: `if t.Agent.WSL != ""` → nil (`hooks.go:133-136`). Paths are **Windows** `claudeDir()`/`codexDir()` (`hooks.go:138-157`).
- `subagentSpecOf`: same (`subagents.go:93-96`); dirs `~/.claude/agents` etc. (`subagents.go:22-37`).
- UI mirrors `No`/`WSL` (`hooks.go:1102-1116`; `client.js` ~2261 and ~2465).

**To enable:** drop WSL early-return; build specs from `wslTargetOf` (`targets.go:576-612`) UNC paths (`\\wsl.localhost\<distro>\home\...`). Distro stopped → `wslTargetOf` nil (`578-579`) — writing starts it.
**Path translation:** hook `Command` is copied as-is; a Windows exe is useless in WSL (MCP already refuses Windows programs except magpie image sidecar, `mcp_wsl.go:31-39`). Need Linux command or `/mnt/<drive>/...`. MCP env uses `WSLENV` (`mcp_wsl.go:54-63`).

## 6. Data dir + serve --web

`appdir.Config()` (`appdir.go:223-234`): portable `data/` beside exe **else** `$XDG_CONFIG_HOME/magpie` **else** `~/.config/magpie`. **Honors XDG_CONFIG_HOME** (`rooted` abs path). **Does not use %APPDATA%** unless you set XDG. AgentDock `XDG_CONFIG_HOME=%APPDATA%\AgentDock` → **`%APPDATA%\AgentDock\magpie`** (always appends `magpie`). Portable: `data/` or `.portable` (`appdir.go:47-53`).

Gateway: `DefaultAddr 127.0.0.1:3425` (`gateway.go:45-47`). `MAGPIE_ADDR` wins (`settings.go:1100-1105`). `settings.Port` otherwise. LAN share listens `0.0.0.0` unless MAGPIE_ADDR pins host (`lan.go:28-32`). Beyond-loopback needs a **named gateway key**.

Web: `magpie serve --web` → `127.0.0.1:3430`; `--web=127.0.0.1:3430` (`web_cli.go:16-17,169-193`). Key: `MAGPIE_WEB_KEY` ≥16 chars `[A-Za-z0-9._~-]` (`web.go:146-191`); else random per run. Link `http://host:port/?k=`. `MAGPIE_WEB_URL` for reverse-proxy printed link. Sidecar env: `XDG_CONFIG_HOME`, `MAGPIE_WEB_KEY`, optional `MAGPIE_ADDR`, `magpie serve --web=127.0.0.1:<port>`.

## 7. Importers

`GET /api/importapps`, `GET /api/importapps/discovery`, `POST /api/importapps {picks}` (`gui/importapps.go:27-85`). Sources (`provider/importapps.go:81-93`): `alma`, **`cc-switch`**, `claude-code`, `codex`, `opencode` (asked-only). CC Switch: `%USERPROFILE%\.cc-switch\cc-switch.db` else `config.json` (`489-499`). **No magpie `providers.json` import/merge.** First-run WSL magpie merge = new reader (additive `appReaders` row) or one-shot copy/merge of `providers.json` from `\\wsl.localhost\<distro>\home\<user>\.config\magpie\providers.json` [INFERENCE]. `provider.Stored()` never treats unreadable file as empty.

## 8. .wslconfig (G3)

**Read-only.** `wslConfig()` `%UserProfile%\.wslconfig` UTF-8/UTF-16 (`wsl.go:1358-1364`). `wslMirrored` / `wslSetting` `[wsl2] networkingMode` (`1379-1392`). Notices tell the user to set mirrored and run `wsl --shutdown` (`1416-1424`) — **no writer, no shutdown helper.** `wslrun.Run` can invoke `wsl.exe --shutdown`. G3 = new package: backup, set `networkingMode=mirrored` under `[wsl2]`, confirm, shutdown.

## 9. Branding

- Title: `index.html:6` `<title>magpie</title>`; favicon `icons/favicon.svg` (`:7`).
- Header word + bird SVG: `index.html:37-38` `<span>magpie</span>` + inline `#bird`.
- Product icon: `internal/gui/assets/icons/magpie.svg` (and `favicon.svg`).
- i18n product name scattered in `assets/i18n.js` + plugin `i18n.js`.
- CLI/web copy: `cli/main.go` help; `web.go:284` `magpie web` sign-in page.
- Tray glyphs: `internal/gui/trayglyph.go` / `trayimage_*.go`.
Minimal rebrand: `index.html` title+brand span+favicon, `magpie.svg`, sign-in HTML. Deeper: i18n + CLI strings.

## AgentDock sidecar notes

- `magpie serve --web=127.0.0.1:<uiPort>` + `MAGPIE_WEB_KEY=<fixed>` + `XDG_CONFIG_HOME=%APPDATA%\AgentDock` (files in `...\AgentDock\magpie`).
- Load Tauri window at printed `/?k=` URL (or cookie after first hit).
- `--gateway` / Settings gateway mode hides Agents+Library — **do not** pass `--gateway` if G1/G2 need those pages.
- Additive-first: new plugin + new `/api/environments*` + new settings field + new import reader; touch `all.go`, `index.html`, `wsl.go` filter, `hooks.go`/`subagents.go` WSL guards, `KeepOwn`.

