# cc-switch Tauri shell pieces to extract

> Scout research captured 2026-10-15. Paths: magpie=/mnt/d/magpie, cc-switch=/mnt/x/cc-switch.

See architecture. Key anchors: lib.rs:352 run, 354-418 single-instance, 419-420 deep-link OMIT, 456-464 process/dialog/opener/store/window-state, 473-518 log, 522-528 updater skip, 1135-1191 tray, 1393-1398 decorations false, 1827-1873 ExitRequested, 2231-2265 window-state+restart. tray.rs:2062 create_tray_menu, 2396 show_main, 2651 handle events. auto_launch.rs:34-68. CREATE_NO_WINDOW 0x08000000 codex_config.rs:27 misc.rs:25. magpie serve web_cli.go:169. G2 hooks.go:133-135 subagents.go:93-95. G4 settings.go:343-347 wsl.go:915. Import importapps.go:491-499.


## Structured payload

```json
{
  "summary": "Extract a Windows-only Tauri 2 shell from cc-switch (single-instance, window-state, process, opener, dialog, log, tray show/hide, auto-launch crate, CREATE_NO_WINDOW). Drop SQLite/proxy/CLI writers, React UI, deep-link, store, updater pubkey, decorations=false. Magpie already has serve --web, WSL twins, cc-switch import; G1/G4/G5/G2/G3 and WSL providers.json merge are additive.",
  "architecture": "AgentDock = thin Tauri 2 Windows host that spawns nogui magpie serve --web with XDG_CONFIG_HOME=%APPDATA%\\AgentDock and loads http://127.0.0.1:3430/?k=KEY. Magpie is SSOT; G1-G5 are new Go /api + plugin-page files under engine/.",
  "files": [
    {
      "path": "/mnt/x/cc-switch/src-tauri/src/lib.rs",
      "description": "Plugin init, tray builder, close-to-tray, exit/restart, decorations"
    },
    {
      "path": "/mnt/x/cc-switch/src-tauri/src/tray.rs",
      "description": "4310-line tray; keep show/hide/quit only"
    },
    {
      "path": "/mnt/x/cc-switch/src-tauri/src/auto_launch.rs",
      "description": "auto-launch 0.5 crate, not tauri-plugin-autostart"
    },
    {
      "path": "/mnt/x/cc-switch/src-tauri/Cargo.toml",
      "description": "Tauri 2.8.2; lockfile tauri 2.10.3"
    },
    {
      "path": "/mnt/x/cc-switch/src-tauri/tauri.conf.json",
      "description": "bundle/updater/CSP/deep-link; no externalBin"
    },
    {
      "path": "/mnt/x/cc-switch/src-tauri/tauri.windows.conf.json",
      "description": "decorations false"
    },
    {
      "path": "/mnt/x/cc-switch/src-tauri/capabilities/default.json",
      "description": "window controls; no remote.urls"
    },
    {
      "path": "/mnt/d/magpie/internal/cli/web_cli.go",
      "description": "serve --web 127.0.0.1:3430"
    },
    {
      "path": "/mnt/d/magpie/internal/agent/wsl.go",
      "description": "WSL twins, NoWSLAgents, .wslconfig"
    },
    {
      "path": "/mnt/d/magpie/internal/provider/importapps.go",
      "description": "CC Switch importer"
    },
    {
      "path": "/mnt/d/magpie/internal/library/hooks.go",
      "description": "hooks refused for WSL"
    },
    {
      "path": "/mnt/d/magpie/internal/library/subagents.go",
      "description": "subagents refused for WSL"
    }
  ],
  "report": "See architecture. Key anchors: lib.rs:352 run, 354-418 single-instance, 419-420 deep-link OMIT, 456-464 process/dialog/opener/store/window-state, 473-518 log, 522-528 updater skip, 1135-1191 tray, 1393-1398 decorations false, 1827-1873 ExitRequested, 2231-2265 window-state+restart. tray.rs:2062 create_tray_menu, 2396 show_main, 2651 handle events. auto_launch.rs:34-68. CREATE_NO_WINDOW 0x08000000 codex_config.rs:27 misc.rs:25. magpie serve web_cli.go:169. G2 hooks.go:133-135 subagents.go:93-95. G4 settings.go:343-347 wsl.go:915. Import importapps.go:491-499."
}
```
