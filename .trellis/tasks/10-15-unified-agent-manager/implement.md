# Implement — AgentDock

Ordered; each step ends green before the next. Paths relative to repo root `D:\AgentDock`.

## Checklist

1. **Repo bootstrap**
   - `git subtree add --prefix=engine https://github.com/yetone/magpie.git main --squash` (if offline: `--prefix=engine /mnt/d/magpie <branch>`).
   - Add `LICENSES/magpie-MIT.txt`, `LICENSES/cc-switch-MIT.txt`, `engine/AGENTDOCK_PATCHES.md`.
   - Baseline: `cd engine && go vet ./... && go test ./...` (record pre-existing failures, do not fix upstream).
2. **Engine build script** `scripts/build-engine.ps1`: `CGO_ENABLED=0 go build -tags nogui -o shell/src-tauri/binaries/agentdock-engine-x86_64-pc-windows-msvc.exe .` (in `engine/`).
3. **Shell** `shell/src-tauri` (Tauri 2, Rust 1.95): copy/trim from cc-switch per design §3; `sidecar.rs` (key, spawn, Job Object, readiness, navigate); tray show/hide/quit; autostart toggle in tray; `dist/index.html` splash.
   - Verify: `pnpm tauri dev` on Windows → window shows magpie UI; quit from tray leaves no `agentdock-engine.exe` (`tasklist`).
4. **Branding** (engine edits listed in design §4).
5. **G4** settings field + filters; unit tests in `internal/agent` and `internal/settings` alongside `lightweight_test.go:76-86` style.
6. **G1** `internal/environments` model + `GET /api/environments` + `POST .../{distro}/enabled` + `POST .../reach/refresh`. Host-listener dial (Windows) and in-distro reach probe (`wslrun.Run` 5 s, curl→wget→/dev/tcp, ≤4 parallel, 30 s cache, gated on `wslrun.Up`) are separate fields. Go tests stub `wslrun.Run` for ok/blocked/unknown and assert asleep distros get zero `-d` execs. Plugin page; i18n (zh, en at minimum; other upstream locales fall back).
7. **G3** wslconfig writer (+ backup/rollback) + shutdown endpoint + UI confirmations; Go tests with temp `USERPROFILE` covering UTF-8, UTF-16LE BOM, existing `[wsl2]`, missing section, `localhostForwarding=false` removal.
8. **G2** hooks/subagents WSL targets + command translation; tests mirroring `mcp_wsl` tests; update library UI chips.
9. **G5** sync preview/apply + UI dialog; Go tests with `agent.FakeWSL` (`agent_models_wsl_test.go:30`).
10. **First-run import**: banner + WSL magpie providers.json merge (tests: id collision, never overwrite keys, unreadable source).
11. **Second-engine detection** notice (processes + :3425 owner).
12. **Package**: `pnpm tauri build` → NSIS installer; install on this machine; smoke.

## Validation commands

- Engine: `cd engine && go vet ./... && go test ./...` (Windows, so WSL code paths compile/run); GUI tests touched: `make test-ui` subset for environments/library pages if Playwright is available.
- Shell: `cd shell/src-tauri && cargo clippy -- -D warnings && cargo test`; `cargo fmt --check`.
- E2E smoke on this machine (Windows + Debian mirrored):
  1. Launch AgentDock → Environments page lists `windows` (host listener up) and `wsl:Debian` (running, mirrored, reach `ok` probed from inside Debian; check `checkedAt` updates after Refresh). Asleep/blocked/unknown branches are covered by Go tests with stubbed `wslrun.Run`, not by stopping Debian, since this session runs in it.
  2. Disable `Debian` → its twins disappear from Agents and no probe runs; re-enable → they return.
  3. Import cc-switch providers from `C:\Users\Administrator\.cc-switch` and WSL magpie providers from Debian.
  4. Connect `claude` and `claude@wsl:Debian`; set model on Windows; G5 sync to Debian; verify `\\wsl.localhost\Debian\home\mci777\.claude\settings.json` and a real request from Debian `claude -p` routed through :3425 (usage row appears).
  5. Add a hook + subagent bound to `claude@wsl:Debian` → files appear in Debian `~/.claude`.
  6. G3 on a copy: point `USERPROFILE` to a temp dir in tests; on the real machine only exercise the read path (already mirrored) — do NOT run shutdown without the user present.

## Risky files / rollback points

- `engine/internal/agent/wsl.go`, `engine/internal/library/{hooks,subagents}.go`, `engine/internal/settings/settings.go`: upstream hot files; keep diffs minimal, log in `AGENTDOCK_PATCHES.md`.
- `%USERPROFILE%\.wslconfig`: only written through the G3 endpoint with backup.
- Real CLI configs on this machine: magpie `applied.json`/`stash.json` restore on disconnect; before E2E, back up `~/.claude`, `~/.codex` on both sides.

## Before `task.py start`

- `implement.jsonl` / `check.jsonl` curated with `research/*.md` (specs are still unfilled templates).
- User approved the final planning summary.
