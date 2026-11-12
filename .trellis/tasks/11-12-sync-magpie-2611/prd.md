# PRD — Sync magpie subtree (2026-11)

## Goal
Pull `magpie-upstream/main` into `engine/` with `git subtree pull --squash`, from split `6afbe3f30` (2026-10-27). 3041 upstream commits, 5923 files; heavy in internal/gateway, agent, provider, gui; new `third_party/`.

## Constraints
- Same procedure as 10-16-sync-magpie-subtree: refresh `engine/AGENTDOCK_PATCHES.md` against `<split> HEAD:engine` first, pull, resolve conflicts hunk by hunk keeping AgentDock patches, record new split.
- No `-X ours`, no tree copy.
- Clean tree before pull (currently `shell/src-tauri/Cargo.lock` dirty — handle first).

## Acceptance
- Merge commit carries `git-subtree-split: <magpie-upstream/main sha>`.
- `AGENTDOCK_PATCHES.md` updated; every `M`/`A` vs new split listed.
- Engine builds on Windows (`scripts/build-engine.ps1`); Go tests for touched packages pass; shell launches and gateway serves.
- Old task 10-16-sync-magpie-subtree archived.
