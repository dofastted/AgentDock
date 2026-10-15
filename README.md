# AgentDock

One Windows app that manages every AI coding CLI on this computer **and** in
each WSL distro: providers, model routing, skills, MCP servers, instructions,
hooks and subagents.

- **Engine** — `engine/`, a `git subtree` of [magpie](https://github.com/yetone/magpie)
  (MIT), built `-tags nogui` and run as a sidecar (`serve --web`). It owns the
  gateway (`127.0.0.1:3425`), routing, the Library and every agent's config.
  AgentDock's changes are listed in `engine/AGENTDOCK_PATCHES.md`.
- **Shell** — `shell/`, a minimal Tauri 2 app extracted from
  [cc-switch](https://github.com/farion1231/cc-switch) (MIT): tray, single
  instance, autostart, window state, and the engine's lifecycle (Job Object,
  restart on crash). Its window shows the engine's own UI.

## What AgentDock adds for Windows + WSL

On the **Environments** page:

- this computer and each WSL distro side by side; whether the gateway listens
  on Windows, and — probed from inside each running distro — whether the distro
  reaches it. A stopped distro is never started to check.
- each distro turned on or off on its own.
- one agent's setup (connected, model and other picks, Library bindings)
  copied onto the same agent in other environments, previewed first.
- `.wslconfig` switched to mirrored networking (backed up, restorable), then
  `wsl --shutdown` after a second confirmation. No admin rights needed.
- providers brought in from CC Switch or from a magpie a distro ran on its own.
- a warning when another magpie would write the same agents' files.

Hooks and subagents from the Library now reach agents in WSL too: Windows
drive paths become `/mnt/<drive>/…`; a hook that runs a Windows program is
refused for the distro, with the reason shown.

Data lives in `%APPDATA%\AgentDock` (engine data in `…\magpie`, logs in
`…\logs`); `~/.config/magpie` is never touched.

## Build (Windows)

    powershell -ExecutionPolicy Bypass -File scripts\build-engine.ps1
    cd shell && pnpm install && pnpm tauri build

The NSIS installer lands in `shell/src-tauri/target/release/bundle/nsis/`.
Go needs the toolchain `engine/go.mod` names; `build-engine.ps1` uses
`goproxy.cn` when `GOPROXY` is unset.

## Test

    cd engine && go vet ./... && go test -tags nogui ./...
    cd shell/src-tauri && cargo clippy -- -D warnings && cargo fmt --check

Licenses: `LICENSES/`.
