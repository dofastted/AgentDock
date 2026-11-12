# Implement — Sync magpie subtree (2026-11)

1. Clean tree: commit Trellis planning files alone.
2. Refresh `engine/AGENTDOCK_PATCHES.md` vs `git diff --name-status 6afbe3f30 HEAD:engine`; commit alone if changed.
3. `git fetch magpie-upstream`; record `git rev-parse HEAD` (pre-pull) and upstream tip.
4. `git subtree pull --prefix=engine https://github.com/yetone/magpie.git main --squash`.
5. Resolve conflicts per design rule; `git diff --check`; no markers; keep git's subtree message.
6. Update AGENTDOCK_PATCHES.md section 1 (new split/date/resolution notes); re-diff inventory vs new split; commit.
7. Build + vet + test on Windows; fix fallout in separate commit(s).
8. Archive 10-16-sync-magpie-subtree.
