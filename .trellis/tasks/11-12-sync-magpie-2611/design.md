# Design — Sync magpie subtree (2026-11)

Same mechanism, conflict rule, checklist contract and rollback as
`10-16-sync-magpie-subtree/design.md`; only the pins change.

- Old split: `6afbe3f3047419bfd50e522526457bac3fa25f2c` (2026-10-27).
- New tip: whatever `magpie-upstream/main` is at pull time; record it from the
  `git-subtree-split:` trailer, not from memory.
- Inventory before pull: `git diff --name-status 6afbe3f30 HEAD:engine`.
- Conflict rule (per hunk): upstream-only → upstream; AgentDock-only → keep;
  both, compatible → keep both; both, exclusive → AgentDock wins, dropped
  upstream hunk written to task notes and to AGENTDOCK_PATCHES.md.
  Never whole-file ours/theirs.
- Scale: 3041 commits. Large conflicts in `internal/agent`, `internal/gui`,
  `internal/gateway`, `internal/library` expected. If upstream rewrote a file
  that carries an AgentDock patch, re-apply the patch's *behavior* on top of
  upstream rather than restoring the old file.
- Rollback: record pre-pull HEAD; `git merge --abort` while conflicted;
  after commit, `git revert -m 1 <merge>`.
- Verification: Windows `cd engine && go vet ./... && go test -tags nogui ./...`
  (copy-out rule per CLAUDE.md §5 applies for Windows toolchain), plus
  `scripts/build-engine.ps1`.
