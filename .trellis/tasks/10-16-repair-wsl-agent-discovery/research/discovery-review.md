# Discovery chain review

Read-only review by WslDiscovery; important failure paths independently checked against runtime evidence in runtime-evidence.md. No product implementation or checks executed.

## Discovery versus reachability

- `engine/internal/agent/wsl.go:1197-1228`: generated probe executes `sh -lc`; `command -v TOOL || wslbin TOOL` prevents fallback when a Windows-mounted npm wrapper exists.
- `engine/internal/agent/wsl.go:1302-1331`: parser drops Windows-drive binaries, after the fallback opportunity has already been lost.
- `engine/internal/wslrun/wslrun.go:195-204`: separate tool resolver skips mounted candidates and tries additional contexts. Agent inventory and execution can disagree.
- `engine/internal/agent/wsl.go:589-594`: configuration-directory presence can qualify an agent even without an executable; UI must not imply these are equivalent signals.
- `engine/internal/agent/wsl.go:878-911,938-974`: probe/cache lifecycle includes once-per-process discovery, a ten-minute failed-probe retry, and slow work under the discovery lock.
- `engine/internal/agent/rescan.go:16-28`: explicit agent rescan resets discovery.
- `engine/internal/gui/environments.go:124-135`: environment refresh only refreshes reachability; it does not rescan CLIs.
- `engine/internal/gui/environments.go:39-58`: response contract has no distro-list/probe error field, so empty inventory can conceal a failed lookup.

## Data directory isolation

- `shell/src-tauri/src/sidecar.rs:157-158,183-187`: sidecar points process-wide XDG_CONFIG_HOME at AgentDock.
- `engine/internal/agent/agents.go:80-103`: external CLI config resolution consumes the same variable.
- Engine-owned state must be isolated without changing the environment root used to locate external CLI configurations. Preserve existing AgentDock data; do not silently migrate or overwrite either user's real CLI files or unintended private copies.

## Scope boundaries

Default distro user is correct on this machine; multi-user selection is not currently implemented. Docker distributions are intentionally excluded from management and included in shutdown impact lists. Neither distinction should be mistaken for a new defect.

Installation, distro wakeup, appendWindowsPath changes, privilege elevation and network-mode changes are not repair prerequisites. Native executable, Windows interop launcher, leftover configuration, unsupported target and probe failure should be distinguishable without installing or deleting anything.

## Verification requirements

Exercise Windows-mounted wrapper plus installed native executable, configuration-only agents, explicit rescan after installation/path changes, command/list timeout, and Windows XDG-configured CLI locations. Re-run against the packaged Windows binary, not merely a Linux engine or mocked probe. A reachable gateway alone is not acceptance evidence for discovery or writeback.
