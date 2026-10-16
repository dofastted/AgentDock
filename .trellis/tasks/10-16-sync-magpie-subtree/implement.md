# Implement — Sync vendored magpie subtree

Planning summary approved 2026-10-16. No product behavior changes beyond the merge and the checklist.

## 0. Gate

- `prd.md`, `design.md`, `implement.md` match D1–D4. Checklist classification compares `<split>` (files at repo root) with `HEAD:engine`. Never `422d057:engine/<path>`, and never `git diff b04160de` after the pull: that range also lists upstream files added since the old pin.
- `implement.jsonl` and `check.jsonl` each have real entries. `.trellis/spec/{backend,frontend}/index.md` are empty templates; they do not constrain this sync.
- `task.py start` flipped status to `in_progress` in degraded mode. No session pointer was written. Do not invent a context id.

## 1. Preserve native paging

Working tree must show only:

- `engine/internal/gui/native.go`
- `engine/internal/library/native.go`

Anything else: stop.

Commit those two files alone. Message: native root paging is unfinished and not smoke-tested. Do not stage `.cgcignore` or `.codex/`.

Rollback point: this commit's SHA. Revert it to drop paging without touching the later merge.

## 2. Refresh the checklist before pulling

Rewrite `engine/AGENTDOCK_PATCHES.md` to the three sections in `design.md`. Source of paths, after step 1:

```bash
git diff --name-status 422d057ded55e4eddf0b719bf2229e537c1fef24 HEAD:engine
```

`M` is an edited upstream file. `A` is fork-only. Include native paging under fork-only. Commit this file alone.

Rollback point: this commit. The pull has not run, so `git revert` of this commit restores the MVP list.

## 3. Confirm the tip, then pull

```bash
git status --porcelain
git ls-remote https://github.com/yetone/magpie.git refs/heads/main
```

Status must be empty. `ls-remote` must print `674b6ca2881f1289c4ccdec158762978a463320c`. If either check fails, stop. Record `git rev-parse HEAD` as the pre-pull parent.

```bash
git subtree pull --prefix=engine https://github.com/yetone/magpie.git main --squash
```

No `--strategy-option=ours`. No manual tree copy.

## 4. Resolve conflicts

For each conflicted path, apply the design table hunk by hunk. Read both sides before writing. After resolution:

```bash
git diff --check
```

No conflict markers. Then complete the merge with git's normal subtree commit message; do not replace that message.

Write into this task's `task.json` notes, per overlapping file: auto-merged, or which hunks were hand-kept, and any upstream hunk dropped under D2.

If add/add appears on a fork-only path, resolve that path in the same turn; do not leave the merge in progress across a yield.

## 5. Record the split

Confirm the merge message contains `git-subtree-split: 674b6ca2881f1289c4ccdec158762978a463320c`. Update section 1 of `AGENTDOCK_PATCHES.md` to that split and the pull date. Add a checklist line for any file whose resolution dropped upstream text. Commit only that file.

Diff check:

```bash
git diff --name-status 674b6ca2881f1289c4ccdec158762978a463320c HEAD:engine
git diff 674b6ca2881f1289c4ccdec158762978a463320c:internal/agent/agents.go HEAD:engine/internal/agent/agents.go
git diff 674b6ca2881f1289c4ccdec158762978a463320c:internal/library/targets.go HEAD:engine/internal/library/targets.go
git diff 674b6ca2881f1289c4ccdec158762978a463320c:internal/gui/assets/i18n.js HEAD:engine/internal/gui/assets/i18n.js
```

Every `M` is in section 2. Every `A` is in section 3. The three file diffs show AgentDock's remaining delta, not an empty file and not a silent full-file revert of upstream. The next sync classifies against the split recorded here, not against `b04160de`.

## 6. Verify on Windows

From `engine/` on Windows, not from the Debian session that cannot be the acceptance host for `-tags nogui` Windows tests:

```text
go vet ./...
go test -tags nogui ./...
```

Record the command, the host, and the summary on this task. Known upstream failures from parent AC10 (`cli`, `gateway`, flaky `TestFXStateDoesNotWait`) stay listed as pre-existing. A new red test is fixed if it is caused by the resolution; if unmodified `674b6ca` fails the same test, record that and do not patch upstream.

Do not run `pnpm tauri build`. Do not edit `/mnt/d/magpie`.

## Later syncs

Not executed here. The next time is the same sequence from step 2, with the split just recorded as the old pin: refresh the checklist from `git diff --name-status <that-split> HEAD:engine`, confirm `main`, squash-pull, resolve with D2, record the new split, run the same Windows tests. No new mechanism.
