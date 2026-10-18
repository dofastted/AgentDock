<div align="center">
  <img src="shell/src-tauri/icons/icon.png" alt="AgentDock Logo" width="128" height="128">
  
  # AgentDock
  
  **One Windows app that manages every AI coding CLI on this computer _and_ in each WSL distro**
  
  [![Release](https://img.shields.io/github/v/release/dofastted/AgentDock?style=flat-square)](https://github.com/dofastted/AgentDock/releases)
  [![License](https://img.shields.io/badge/license-MIT-blue?style=flat-square)](LICENSE)
  [![Platform](https://img.shields.io/badge/platform-Windows%2011%20%7C%2010-blue?style=flat-square)](https://github.com/dofastted/AgentDock)
</div>

---

## Overview

AgentDock unifies provider management, model routing, skills, MCP servers, instructions, hooks, and subagents across Windows and WSL. Stop juggling configurations between environments—manage everything from one tray app.

### Architecture

- **Engine** — `engine/`, a `git subtree` of [magpie](https://github.com/yetone/magpie) (MIT), built `-tags nogui` and run as a sidecar (`serve --web`). It owns the gateway (`127.0.0.1:3425`), routing, the Library and every agent's config. AgentDock's changes are listed in `engine/AGENTDOCK_PATCHES.md`.
  
- **Shell** — `shell/`, a minimal Tauri 2 app extracted from [cc-switch](https://github.com/farion1231/cc-switch) (MIT): tray, single instance, autostart, window state, and the engine's lifecycle (Job Object, restart on crash). Its window shows the engine's own UI.

## Features

### Windows + WSL Integration

On the **Environments** page:

- **Side-by-side environment view** — this computer and each WSL distro visible together; probe gateway connectivity from inside each running distro (stopped distros never started just to check)
- **Per-distro gateway control** — enable/disable each distro individually
- **Cross-environment sync** — copy one agent's setup (connection, model, Library bindings) to the same agent in other environments with preview
- **`.wslconfig` automation** — switch to mirrored networking (backed up, restorable), then `wsl --shutdown` after confirmation; no admin rights required
- **Provider import** — bring in providers from CC Switch or from magpie instances running in distros
- **Conflict warnings** — alerts when another magpie instance would overwrite shared agent files

### Cross-Platform Hook Support

Hooks and subagents from the Library work in WSL too:
- Windows drive paths automatically convert to `/mnt/<drive>/…`
- Windows-only hooks (native programs) gracefully refuse execution in distro context with clear error messages

### Data Storage

- **Standard mode**: `%APPDATA%\AgentDock\` (engine data in `…\magpie`, logs in `…\logs`)
- **Portable mode**: Place a `.portable` file next to the executable; data stores in `<exe_dir>\data\`
- WSL configs (`~/.config/magpie`) remain untouched

## Installation

### Option 1: NSIS Installer (Recommended)

Download `AgentDock_1.0.0_x64_en-US.exe` from [Releases](https://github.com/dofastted/AgentDock/releases), run the installer, and launch from Start Menu or Desktop shortcut.

### Option 2: Portable

1. Download `AgentDock_1.0.0_x64_portable.zip` from [Releases](https://github.com/dofastted/AgentDock/releases)
2. Extract to any folder
3. Data is stored in `data/` subfolder (portable between systems)

## Build from Source (Windows)

### Prerequisites

- Go 1.23+ (per `engine/go.mod`)
- Rust 1.95+ (stable)
- Node.js 20+ with pnpm
- Visual Studio Build Tools (for Rust linking)

### Build Steps

```powershell
# Build engine (Go binary)
powershell -ExecutionPolicy Bypass -File scripts\build-engine.ps1

# Build shell (Tauri app)
cd shell
pnpm install
pnpm tauri build
```

**Output locations**:
- NSIS installer: `shell/src-tauri/target/release/bundle/nsis/`
- Portable build: run `scripts/make-portable.ps1` (creates zip with `.portable` marker)

**China users**: `build-engine.ps1` automatically uses `goproxy.cn` when `GOPROXY` is unset.

## Development

### Run Tests

```powershell
# Engine tests
cd engine
go vet ./...
go test -tags nogui ./...

# Shell tests
cd shell/src-tauri
cargo clippy -- -D warnings
cargo fmt --check
```

### Dev Mode

```powershell
# Terminal 1: Engine
cd engine
go run -tags nogui . serve --web

# Terminal 2: Shell
cd shell
pnpm tauri dev
```

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/your-feature`
3. Commit changes: `git commit -m 'Add some feature'`
4. Push to branch: `git push origin feature/your-feature`
5. Open a Pull Request

Follow the existing code style and ensure tests pass before submitting.

## Licenses

- **AgentDock**: MIT License (see `LICENSE`)
- **magpie** (engine subtree): MIT License (`LICENSES/magpie-MIT.txt`)
- **cc-switch** (shell base): MIT License (`LICENSES/cc-switch-MIT.txt`)

## Acknowledgments

Built on top of:
- [magpie](https://github.com/yetone/magpie) by yetone — AI gateway and agent management
- [cc-switch](https://github.com/farion1231/cc-switch) by farion1231 — Tauri shell foundation

Special thanks to the open-source community for making cross-platform development accessible.

---

<div align="center">
  <sub>Made with ❤️ for Windows + WSL developers</sub>
</div>
