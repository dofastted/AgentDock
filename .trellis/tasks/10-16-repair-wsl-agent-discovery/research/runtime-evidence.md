# Windows–WSL runtime evidence

## Inspection boundary

Read-only inspection of the running Windows release engine and its existing UI. No repair build, CLI configuration edits, privilege changes, distro shutdown, or installation was requested by the investigator. The application can perform its own background synchronization while running. API keys were consumed locally for authentication and are not recorded here.

## Progress audit

- `task.py current --source` returned no active task. `task.py list` listed `00-bootstrap-guidelines` and `10-15-unified-agent-manager`, both `in_progress`.
- Parent PRD AC1–AC10 are checked, but its own AC10 text reports pre-existing failing packages. These records do not establish current end-to-end usability.
- Backend/frontend spec indexes still label their guidelines `To fill` and contain no project-specific Quality Check section.
- At initial inspection, `git diff --name-only HEAD` returned no tracked product changes; `git status --short` showed untracked `.codex/`. That content was left untouched.

## Runtime identity

Windows process inventory identifies the running shell and engine as the repository release output under `D:\AgentDock\shell\src-tauri\target\release`, not the installed `%LOCALAPPDATA%` copy. The engine command is `serve --web=127.0.0.1:3430`.

Windows `wsl.exe --list --quiet` and `--list --running --quiet` list Debian and docker-desktop. Engine `/api/environments` lists Windows plus Debian, with Debian running/mirrored and seven agent IDs: codex, pi, claude, omp, hermes, grok, and omp#grok-pro, each suffixed `@wsl:Debian`. Docker exclusion is existing parent scope. This response proves only what this build currently reports, not correctness of detection or configuration.

## Confirmed defects and failures

### B1: Engine isolation also redirects external Windows CLI configuration

- `shell/src-tauri/src/sidecar.rs:183-187` sets `XDG_CONFIG_HOME` to the AgentDock data root for the engine.
- `engine/internal/agent/agents.go:80-86` uses that same variable as the external CLI config root; `builtins` passes it to OpenCode at line 98.
- Live `/api/library` advertises `C:\Users\Administrator\AppData\Roaming\AgentDock\opencode\opencode.json` and an AgentDock-private Cursor CLI config path.
- Both the AgentDock-private OpenCode config and the actual user config `C:\Users\Administrator\.config\opencode\opencode.json` exist. The latter directory also contains tui.json, package.json and bun.lock.
- Consequence: a successful write to the engine-advertised path need not reach the CLI the user actually starts. Fix must separate engine storage isolation from external CLI environment resolution, not add one OpenCode special case.

### B2: WSL shared skills absent from inventory

- Live `/api/library` returns 44 `foundSkills`, zero managed library skills, and no `foundSkills[].shared` path under `\\wsl.localhost`.
- Debian `/home/mci777/.agents/skills` exists with directories including codegraph, codegraphcontext, ssh-skill, impeccable, codex, grok and orca-cli. The default user is mci777 and HOME is `/home/mci777`.
- The UI Skills tab shows Windows shared paths and the same inventory gap. The WSL Claude-specific reverse-engineer-anything skill is found, so this is not a total UNC access failure.
- `engine/internal/library/targets.go:569-595` explicitly defaults WSL Claude/Codex paths and says distro environment overrides are not read.

### B3: Windows PATH entry masks an installed native CLI

A read-only `wsl.exe -d Debian --exec sh -c ...` reports Windows-mounted npm launchers for claude, opencode and gemini; codex resolves to `/usr/local/bin/codex`. omp/pi/grok/hermes are absent from that non-login shell PATH. The engine nevertheless reports several of those agents from its probe/cache/config-presence rules. Binary provenance, configuration presence, and shell context must remain distinct.

`engine/internal/agent/wsl.go:1208` uses `command -v TOOL || wslbin TOOL`. A Windows-mounted wrapper succeeds at the first lookup, preventing the native fallback. Lines 1302–1305 then discard Windows-drive results. A read-only demonstration using the same `sh -lc` context returns `probe-first-result=/mnt/c/Users/Administrator/AppData/Roaming/npm/gemini` while confirming `native-executable=/home/mci777/.nvm/versions/node/v20.20.0/bin/gemini`. The live environment agent list omits Gemini. This is a demonstrated installed-native-CLI discovery failure, not merely an inferred PATH risk.

The probe runs with a 30-second timeout while discovery caches and error handling live in `wsl.go`. Environments refresh (`engine/internal/gui/environments.go:124-135`) only refreshes reachability, not CLI discovery. Separate discovery/probe/cache paths must be audited rather than assuming a successful reach probe repairs agent inventory.

### B4: MCP failures are real and need target-specific treatment

The visible Library banner reports write problems. A subsequent metadata-only glance reads eleven problems:

- Five WSL assignments of grok-search reject a Windows `uvx.exe` command.
- Six Hermes MCP writes fail creating a temporary config under Debian `~/.hermes` with `Access is denied`.
- `stat` reports `/home/mci777/.hermes` owned by root:root with mode 755; config.yaml is absent. Do not silently change ownership, elevate, or claim success. A native write as the default WSL user would also lack directory write permission.

## Verification boundary

UI opened through a dedicated managed Chromium tab against the existing engine; Library/Instructions and Skills were inspected and a screenshot captured. Tab closed. No lint, type check, test suite or new build was run during planning. The user's reported failures are accepted rather than re-tested as a pass/fail acceptance exercise.

## Planning decision still required

The phrase “Windows one place to manage, WSL readable/editable” does not decide physical ownership: in-place native files presented through Windows versus Windows-canonical library with deployed WSL copies. Current implementation uses both native files and library-managed copies; bidirectional editing needs explicit semantics before product changes.
