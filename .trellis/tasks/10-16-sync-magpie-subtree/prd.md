# Sync vendored magpie subtree

## Goal

Bring `engine/` from vendored magpie `422d057` to the already-fetched `main` commit `674b6ca`, keep AgentDock's second-development changes, and leave the same squash-subtree command as the way every later sync works. Conflict means: take the upstream change where it does not remove an AgentDock behavior, and keep the AgentDock behavior where the two edits touch the same lines. This task adds no product feature.

## Background

Parent `10-15-unified-agent-manager` decision D4: `engine/` is a squash `git subtree` of `github.com/yetone/magpie` `main`. Upstream edits are recorded in `engine/AGENTDOCK_PATCHES.md`. Sibling `10-16-repair-wsl-agent-discovery` stays `in_progress`; this sync does not close it and does not rerun its UI smoke.

Evidence checked 2026-10-16, before any merge:

- `ea88c7ae` records `git-subtree-dir: engine` and `git-subtree-split: 422d057ded55e4eddf0b719bf2229e537c1fef24`. `b04160de` merges that tree. This repo has no git remote. `/mnt/d/magpie` is still `422d057` (2026-10-15 23:15 +0800) and is not a newer upstream.
- `git fetch https://github.com/yetone/magpie.git main` wrote `refs/tmp/magpie-main` = `674b6ca2881f1289c4ccdec158762978a463320c` (2026-10-16 08:43 +0800). `422d057` is an ancestor. Distance: 53 commits, 176 files, +7270/−523. The fetch also imported upstream tags and did not change the worktree. `674b6ca` is the named target. Commits that appear on `main` after that SHA are out of this pull.
- Committed AgentDock engine delta since `b04160de`: 80 files, +7637/−520. Do not measure the patch with `git diff ea88c7ae HEAD -- engine`; that range counts the whole squashed tree (5426 additions).
- Of 33 modified upstream files, three paths also changed in `422d057..674b6ca`. Overlap is not a merge result:
  - `internal/agent/agents.go` — upstream `486ea8237`, `a80bc3009` (Cursor CLI row), +13. AgentDock +41/−22.
  - `internal/library/targets.go` — same two commits, +17. AgentDock +165/−102.
  - `internal/gui/assets/i18n.js` — 8 upstream commits, +180/−43. AgentDock +8/−0, brand strings.
- The other 30 modified upstream files were not touched upstream in this range, including `internal/agent/wsl.go`, `internal/library/{skills,hooks}.go`, `internal/appdir/appdir.go`, `plugins/all/all.go`. Add/add collisions are still unknown until the pull runs.
- `AGENTDOCK_PATCHES.md` lists only the MVP edits. It omits later edits in `internal/appdir/{appdir,env}.go`, `internal/testenv/testenv.go`, `internal/agent/{gemini,omo,primeagent,reasonix,rescan,wsl_fake}.go`, `internal/library/{library,mcp,skills,sync,targets}.go`, `internal/gui/library.go`, and `internal/gui/assets/plugins/library/{i18n.js,library.css}`.
- Dirty worktree, not part of any recorded acceptance check: `engine/internal/gui/native.go` (+13) and `engine/internal/library/native.go` (+62/−3). They add per-root paging (`Total`, `NativePage`, `ReadNativeRoot`).

## Decisions

- **D1 Target.** This pull stops at `674b6ca`. The command is still `git subtree pull --prefix=engine https://github.com/yetone/magpie.git main --squash`, run only while that ref is still `674b6ca`. If `main` has moved, fetch the new tip aside and do not pull it in this task.
- **D2 Conflict rule.** On a conflict, keep the AgentDock behavior and bring in the upstream change wherever the two hunks are not the same lines. Dropping an AgentDock behavior requires a written reason in the task notes. Do not resolve by taking one whole file from either side when both sides edited it.
- **D3 Repeatability.** After this pull, the next sync is the same squash command plus the same conflict rule plus an updated `AGENTDOCK_PATCHES.md`. No submodule, patch stack, or second vendor tree.
- **D4 Dirty paging.** Commit the native-paging edit by itself before the pull, with a message that says it is unfinished and not smoke-tested. Do not restore it and do not let the squash commit contain it.

## Requirements

- **R1** `engine/` subtree split becomes `674b6ca2881f1289c4ccdec158762978a463320c` via one squash subtree pull.
- **R2** AgentDock behavior committed since `b04160de`, plus the native-paging commit, still builds. Existing AgentDock tests that passed before the pull still pass. Upstream-owned behavior comes in as upstream left it.
- **R3** Before the pull, `AGENTDOCK_PATCHES.md` lists every upstream file this fork edits and every fork-only file, including native paging. After the pull it records the new split and any conflict resolution.
- **R4** `git status` is clean before the pull. The paging commit is separate from the squash commit.
- **R5** The documented update command stays the squash subtree pull in D1. Later syncs use D2 and D3 without a new mechanism.
- **R6** On Windows: `cd engine && go vet ./... && go test -tags nogui ./...`. Failures that also fail on unmodified `674b6ca` are recorded, not fixed.

## Acceptance Criteria

- [ ] **AC1 (R1, R5):** the new subtree commit message contains `git-subtree-split: 674b6ca2881f1289c4ccdec158762978a463320c`, and that commit is a `git subtree pull`, not a copied tree.
- [ ] **AC2 (R2, D2):** `agents.go`, `targets.go`, and `i18n.js` each contain the upstream edit and the AgentDock edit, or the task notes say which side was dropped and why. No conflict marker remains.
- [ ] **AC3 (R3):** after the pull, every modified upstream path in `git diff b04160de HEAD -- engine` is named in `AGENTDOCK_PATCHES.md`, and the file records split `674b6ca`.
- [ ] **AC4 (R4, D4):** native paging is its own commit, created from the current dirty diff, and is an ancestor of the squash commit rather than part of it. `git status` was clean at pull time.
- [ ] **AC5 (R6):** Windows vet/test output is recorded on this task. New failures are fixed or listed with package, test name, and whether unmodified `674b6ca` fails the same way.

## Out of Scope

- Closing the repair task or repeating its packaged Windows/WSL UI smoke.
- Product work to adopt upstream features beyond what the merge already brings in.
- Replacing subtree, pushing patches upstream, adding a git remote, deleting `refs/tmp/magpie-main`, pruning fetched tags, or editing `/mnt/d/magpie`.
- A scheduler, hook, or bot that pulls magpie by itself. "Keep syncing this way" means the command and the conflict rule stay stable, not an unattended job.
