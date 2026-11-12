# Implement — Sync magpie subtree (2026-11)

1. Clean tree: commit Trellis planning files alone.
2. Refresh `engine/AGENTDOCK_PATCHES.md` vs `git diff --name-status 6afbe3f30 HEAD:engine`; commit alone if changed.
3. `git fetch magpie-upstream`; record `git rev-parse HEAD` (pre-pull) and upstream tip.
4. `git subtree pull --prefix=engine https://github.com/yetone/magpie.git main --squash`.
5. Resolve conflicts per design rule; `git diff --check`; no markers; keep git's subtree message.
6. Update AGENTDOCK_PATCHES.md section 1 (new split/date/resolution notes); re-diff inventory vs new split; commit.
7. Build + vet + test on Windows; fix fallout in separate commit(s).
8. Archive 10-16-sync-magpie-subtree.

## Progress log (2026-11-12)
- Pre-pull HEAD `08dfe5287`. Pull ran with WSL git (`git -c core.autocrlf=true subtree pull ...`); git.exe subtree is broken from WSL. Upstream moved during pull: split is `9197804144c15c7765c0e8b13376981264dc02a2`, squash `57e20e927`.
- 27 conflicted files. Resolved by hand: gateway.go (upstream split Handler/routes; foldDoubledVersion now outermost in Handler), api.go (upstream settingsapi.Apply; imageFit omit-preserve + WSLDistrosOff carry ported into settingsapi.go, `imageFit` added to Keys), settings.go/settings_keep_test.go (both), agents.go/gemini.go (distro env roots kept, wrapped in upstream at.open for links), primeagent/rescan/sessions wsl.go (both).
- Delegated to codex: group A (wsl.go, wsl_probe.go dedupe onto upstream probe, wslrun, tests), group B (library + library UI, index.html, app.js). Briefs in /tmp/rs/cx/.
- Merge committed `90568dc64` (split 919780414). Build + vet clean (Windows go.exe 1.26). Touched-package tests pass; remaining failures (agent *Kept, settings TestSharedChecked…, sessions Agy*/CCListing, appdir TestFoldersJoinedOnce) reproduce on pristine upstream 57e20e927 → environment, not merge.
