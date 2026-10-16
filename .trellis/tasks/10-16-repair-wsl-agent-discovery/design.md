# Design — In-place Windows/WSL management repair

## 1. Ownership and boundaries

The Windows shell remains an engine process supervisor. The existing Go engine remains the only management API; no second WSL daemon, database or synchronization service is introduced. Native CLI files are authoritative for the in-place management surface. Existing provider routing and optional centrally managed library resources remain available, but discovering or editing a native resource must not import/move it into that library or overwrite it during background library sync.

Use existing agent adapters, library metadata readers, structured config editors and error/result conventions. Extend them where needed; do not build a parallel per-CLI configuration registry. Preserve user customizations and original path identity.

## 2. Isolate only engine-owned storage

The shell must stop overriding XDG_CONFIG_HOME for the whole engine. Add an engine-specific absolute data-directory setting, `AGENTDOCK_DATA_DIR`, pointing directly at the current `%APPDATA%\AgentDock\magpie`. Apply it consistently in appdir's engine-state root resolution, including installed/redirected checks, without exposing it as the external CLI environment. Standalone engine behavior without this setting stays unchanged; inherited user XDG variables retain their meaning for CLIs.

The shell continues to own shell.json/logs as today. Existing AgentDock engine data stays at the same path. Private `AgentDock\opencode`/`cursor` files are not treated as canonical and are not automatically merged/deleted; present their existence as recovery information where relevant.

Before implementation, find references for any changed exported appdir symbol and preserve its existing portable/standalone contract. Avoid a process-wide environment mutation as a substitute for engine-only isolation.

## 3. Environment and CLI discovery

Keep one observable discovery result per distro/default user and distinguish:

- distro installation/running state;
- user and Linux home;
- discovery completion/error and last successful snapshot;
- executable path/provenance (native versus Windows interop);
- configuration/profile presence;
- per-resource resolved roots/capabilities;
- gateway reachability, separately timestamped.

Repair `wslProbeScript`: a Windows-mounted candidate must not short-circuit native lookup. Reuse existing tool-manager lookup knowledge; compare results using actual Windows mounts, not only a hard-coded `/mnt/c` assumption. Probe environment values required by existing CLI adapters (e.g. CODEX_HOME/CLAUDE_CONFIG_DIR, XDG roots and named profiles) in the distro context; never substitute Windows values. Only report values needed for paths; do not collect secret environment variables.

Align agent inventory and wslrun executable lookup so they agree on native candidates. Do not execute a Windows wrapper to obtain a WSL-native version. A config folder without its CLI remains visible as configuration-only, not a claim that the CLI is installed and usable.

Rescan invalidates the appropriate agent and executable caches before updating reachability. Preserve last-success data as stale when a probe fails, with the failure shown. Slow wsl.exe calls must not hold the shared inventory mutex or serialize unrelated GUI reads. Bound probes and concurrency using existing helpers. Never call distro execution for an asleep distro during listing; disclose the existing stop-between-check-and-exec race rather than claiming an atomic WSL state check.

## 4. Resource identity and native inventory

Extend existing inventory records with stable environment/CLI-profile/kind/path identity. A display name is not a mutation key. Server-resolved identities must select a known eligible resource; do not accept an arbitrary browser-supplied path for read/write/delete.

For each running environment, scan supported primary roots plus environment-local shared roots. Deduplicate the same canonical resource reached through several links, while retaining its visible paths and consumer relationships. Same-named Windows/WSL resources with different canonical paths remain independently selectable and editable; compare/copy operations name their sources explicitly.

Repair WSL shared-root associations using base CLI kind plus distro/profile identity, not unsuffixed `readsShared` membership against a global host root. Normalize named omp profiles for capability selection while preserving each profile's actual directory. An unsupported capability returns a reason, not a missing row or invented config path.

Resource kinds include skills, global instructions, existing native prompt/template files, agent definition files and MCP entries. Codex prompts/agents are confirmed native files, not an alias for Instructions/Subagents. Use the corresponding CLI's native layout and preserve unsupported fields. No attempt to turn every text file under a home directory into a managed resource.

## 5. In-place reads and writes

Read current target files on refresh/reopen. Do not return centrally cached library text as if it were native WSL content. Keep native inventory/editor operations distinct from existing library import/deploy actions; remove the name-only/physical-move behavior from any action presented as native management. Migrate all affected UI/API callers in one cutover rather than retaining an ambiguous compatibility endpoint.

Reuse existing atomic/structured writers for host resources. For WSL resources, resolve Linux paths/symlink destinations in the default user's distro context, preserving symlinks instead of replacing their directory entry. Use bounded wslrun operations where Windows UNC cannot faithfully read/resolve/write a Linux path; arguments/data must be passed separately from fixed scripts, not interpolated into shell source. Existing UNC-backed config editors may remain only where they preserve the same authoritative Linux target and permission semantics. Do not make a transport retry silently change identity, ownership or overwrite rules.

Edits carry a version/fingerprint of the content read. Re-read before mutation and reject an externally changed resource with a conflict/current-version result. This prevents Windows and WSL editors from silently losing each other's changes; it is not automatic bidirectional sync. Use existing backup mechanisms for modified files; never back up credentials into research/log output. Preserve unrelated JSON/TOML/YAML keys and instruction text outside the requested edit.

Every mutation rechecks target running/enabled/identity state. Stopped/unavailable distributions and permission failures produce target-specific errors. A successful response requires successful write and readback at the intended path, not merely updating library metadata. Do not elevate or change permissions. Cross-target operations may return explicit per-target results rather than pretending all targets succeeded.

## 6. MCP compatibility and explicit copy

Retain Windows-executable refusal for WSL. Filter default target choices using existing compatibility information. A Windows command cannot be made portable by deleting `.exe`; a user-selected Linux command belongs to that target and must remain distinct from the Windows command. Do not auto-install missing tools. Existing permissions are diagnostics, not permission to chmod/chown.

Reuse the environment sync preview/apply pattern for explicit native resource copies: source identity, target identities, current differences, confirmation, then bounded writes and readback. No write on preview, page load or ordinary refresh. A changed target invalidates a stale preview rather than losing the target's external edits. Copying between CLIs is limited to genuinely supported compatible formats; report incompatible capabilities instead of inventing conversion.

## 7. UI integration

Reuse the existing Environments and Library surfaces. Show environment/user, CLI/profile, native path, binary/config-only state and resource capability near the item being managed. Native resources are not presented as unavailable until imported into the central library. Show shared versus private roots and same-name variants explicitly.

Prompt/template and agent-file access must be visible in native management, not hidden behind a misleading global-instructions-only label. Editors show actual source environment/path and conflict/permission status. Keep destructive actions explicit. Keep reach refresh separate in meaning from full discovery rescan, even if one UI control requests both.

## 8. Compatibility, rollback and acceptance

Keep the present Windows+WSL desktop scope, default distro user, provider data and routing behavior. No forced migration of native resources or automatic merge of accidental private configs. New engine-owned storage override is additive; no change for a standalone upstream-like launch without it.

Rollback uses existing file backups for writes and preserved native file ownership; build rollback does not require relocating home directories. Update README, engine patch ledger and relevant prevention specs only after implemented behavior is demonstrated. The current spec templates are not evidence of compliance.

Acceptance must exercise the actual packaged Windows UI and WSL readback, not just Linux tests or a reachable gateway. See implement.md for ordering and checks.
