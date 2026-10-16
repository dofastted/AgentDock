# Implement — In-place Windows/WSL management repair

## Approval gate

Planning only until the user explicitly approves the latest final summary. Task-creation consent and the in-place ownership selection are not implementation approval. Do not run task.py start, edit product code or dispatch implementation before that response.

Before activation: PRD convergence pass complete; design.md/implement.md present; implement.jsonl/check.jsonl contain real evidence/spec paths; task manifests validate. Resolve session identity through the existing Trellis runtime rather than stamping an invented context ID if task.py start has no session.

## Ordered implementation

1. **Storage isolation first (B1, AC7).** Read appdir/shell conventions and references for exported changes. Add engine-only AGENTDOCK_DATA_DIR resolution consistently; shell stops overriding XDG_CONFIG_HOME. Keep the current AgentDock magpie data directory and inherited CLI environment. Locate actual Windows OpenCode/Cursor config roots without merging private copies. Update affected contract tests rather than retaining assertions that pin the wrong path.
2. **Discovery/provenance (B3/B4, AC1/AC2).** Fix Windows-wrapper short-circuit; align existing WSL binary resolvers and native tool-manager fallback. Capture required native config roots/profile paths. Expose native/interop/config-only/missing/error states. Remove slow probing from inventory critical sections. Make explicit discovery rescan invalidate the correct caches; keep reachability separate. Preserve asleep/disabled gating and last-success diagnostics.
3. **Environment-scoped native resources (B2/B6, AC3).** Reuse existing target adapters/readers to enumerate shared/private/custom/profile roots. Add environment/path resource identity and canonical symlink grouping; retain same-name variants. Repair named omp profiles and supported WSL capability gaps. Include existing native prompt/templates and agent definitions, including confirmed Codex directories, rather than treating instructions as full prompt coverage.
4. **Native editing and conflict-safe writes (AC4).** Add in-place read/edit/create/remove actions using server-resolved resource identities, existing structured writers, safe Linux path resolution and bounded WSL I/O where needed. Preserve native folders and links. Do not call library import/move when browsing or saving native resources. Re-read external edits and reject stale writes. Update every affected API/UI caller; no ambiguous name-only compatibility shim for native resources.
5. **Target-safe MCP and explicit copies (B5, AC5/AC6).** Respect Windows versus Linux command compatibility when choosing defaults; keep explicit per-target Linux commands separate. Show permission/incompatibility reasons without elevated operations. Reuse preview/apply semantics for compatible resource copying, with explicit source/targets, stale-preview protection and per-target readback. No writes on preview or refresh; no implicit central deployment.
6. **Surface integration (all).** Reuse Environments/Library UI; expose paths, environment/user/profile, native/config-only status, shared roots and capability/error reasons. Make native prompt/agent resources and source identity visible. Update i18n using existing conventions. Avoid unrelated restyling.

Do not run build/lint/tests/formatters mid-flight in delegated slices. Establish a unique integration owner for shared appdir/agent/library API contracts; fan out only independent work after those contracts are agreed. Investigation notes are evidence, not permission to rewrite unrelated routing or upstream packages.

## Verification after integration

Permanent regression tests must cover consumer-visible risks, not source strings or wiring:

- native CLI behind a successful Windows wrapper, custom native config root and named profile;
- environment-local shared skills and distinct same-named Windows/WSL variants;
- in-place edits, external WSL edit/reload, stale editor refusal, no native-folder move and no unrelated-key loss;
- target compatibility, timeout/failed lookup versus absent CLI, asleep target with no execution;
- per-target write/permission outcomes and explicit copy preview/apply identity.

Delete incidental source/wording tests rather than re-pinning them. Reuse existing fixtures; do not depend on actual ownership changes or chmod root-owned user directories. No permanent test is required merely to assert a new function exists.

Run established project commands once after integration, fix local/mechanical failures and re-run affected checks:

- Engine on Windows: `go vet ./...` and `go test -tags nogui ./...` from engine. Use the repository's Windows environment/build scripts as appropriate; record actual commands and upstream failures separately, never declare the full suite green if it is not.
- Shell on Windows: `cargo fmt --check`, `cargo clippy --all-targets -- -D warnings`, `cargo test` from shell/src-tauri.
- UI checks: follow engine/Makefile's existing relevant UI checks if applicable; browser smoke is mandatory regardless. Determine actual available lint/type checks from package config rather than invent commands.
- Build: `powershell -ExecutionPolicy Bypass -File scripts\build-engine.ps1`, then `pnpm tauri build` from shell. Verify that the running engine is the newly packaged binary, not the old repository/installed copy.

## Packaged Windows + WSL smoke

Use a dedicated browser tab against the new Windows engine and inspect real visible state. Keep authentication keys out of logs/screenshots/artifacts.

1. Verify Windows/eligible Debian/user identity and refresh diagnostics; confirm native Gemini is found while the Windows wrapper is first on PATH. Compare CLI resource roots with native Windows/WSL paths, including custom roots and named profiles.
2. Verify Debian shared/private skills and native Codex prompt/agent files appear with their own environment/path. Same-name cross-environment variants stay distinct.
3. Use disposable, uniquely named native resources to create/edit/remove one skill, prompt/instruction and agent definition plus an isolated MCP entry. Read files from inside Debian and verify unrelated existing settings remain unchanged. Do not run a user's real hook/MCP command as an incidental probe.
4. Edit the disposable resource from WSL, refresh in Windows, verify the new content. Open a Windows editor, modify in WSL again, and verify stale save is refused.
5. Preview a compatible Windows→WSL copy: no writes before apply; explicit apply changes only the chosen disposable targets. Verify readback and native path ownership.
6. Exercise incompatible Windows MCP target selection and permission error presentation without changing ownership, installing software or writing to the real root-owned Hermes directory.
7. Exercise stopped-distro and timeout behavior with existing isolated test harnesses or an already-stopped non-session distro. Never stop/shutdown Debian hosting this session. Report the limitation if no safe real stopped target exists.
8. Close the dedicated browser tab. Restore any explicitly backed-up real config edits and remove disposable smoke resources; do not delete user data or existing private config copies.

## Post-proof artifacts and review

After runtime proof, update README and engine/AGENTDOCK_PATCHES.md for the actual new storage/detection/native-management contract. Capture the specific prevention conventions in applicable specs using trellis-update-spec; do not fill the entire unrelated bootstrap task. Record exercised acceptance criteria and limits in the repair task; do not overwrite historical parent acceptance notes as if they were current evidence.

Apply trellis-check: local/mechanical issues may be fixed within scope; design/public-boundary changes outside this approved plan require an evidence-backed stop, not a silent rewrite. Do not commit unless separately authorized by the applicable user/workflow constraints.
