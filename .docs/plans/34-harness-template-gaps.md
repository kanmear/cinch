# 34. Harness-template gaps from the rollout

**Status:** planned

The work happens in `~/code/harness-template` (`origin` is `kanmear/harness_template`, private),
and its rules are in that repo's `README.md`. This plan lives here, like
[33](33-harness-alignment.md), because cinch is where the harness plans are kept.

## Provenance

[Plan 33](33-harness-alignment.md) Part C moved twelve repos onto harness-template v2, one
session per repo. Each session ported the seeded files by hand and recorded where the template
did not fit the repo. Those notes are collected here. Every repo already carries its own
workaround, so none of this blocks anything. The goal is that the next `apply` or `sync` needs
no hand patching.

## Gaps

Each item names the task that found it and the repo that shows it.

1. **`down` leaves backends running (T17, ortho).** Every stack's `down` calls `tmux
   kill-session` and nothing else. Killing the session sends SIGHUP to the panes, and an Air or
   `go run` backend survives it and keeps its port. talk-workbench, fsed and ortho send C-c to
   each pane first, then kill the session. The fix belongs in every stack's Makefile and in the
   `make check` contract guard.
2. **The version path in the go stacks is hard-coded (T15, verus and forum).** The `go` and
   `go-svelte` stacks build with `-X main.version=$(VERSION)`, and `go/conventions.md` says the
   same. verus and forum keep the version in a `version` package. Either add a stack variable
   (for example `VERSION_VAR`, defaulting to `main.version`) or change the default to the
   package form.
3. **go-svelte has no deploy name (T16, lthy).** It has no `DEPLOY_NAME` for a deploy name that
   differs from `NAME`. The seeded `bunfig.toml`, the eslint config and `tests/support` assume
   eslint and the svelte preload, so a repo without them deletes those files after `apply`.
4. **go-svelte has no variant for Node with Postgres (T17 ortho, T18 dd).** These repos have no
   `DB_PATH`, no `create-owner`, no deploy block and no `ENV_FILE`. Both repos stripped those
   parts by hand and added a Postgres block (`dev-db`). Options: a `go-svelte-pg` stack, or a
   `DB` choice inside go-svelte.
5. **flutter always seeds `stack-conventions.md` (T19, budgetron).** A repo that keeps its code
   style in `conventions.md` ends up with two docs that say the same thing. Seed it only when the
   repo has no style section, or make it a `skip.txt` choice.
6. **There is no repo-local layer for managed `.claude/settings.json` (T13, limen).** `sync`
   overwrites the whole file, so a repo's own permissions or hooks are lost. Options: a managed
   base merged with a seeded `.claude/settings.repo.json`, or managing only the `hooks` key.
7. **`prune.sh` misses branches that landed long ago (seen while landing T15–T19).** It counts a
   branch as landed only when `git merge-tree --write-tree main <branch>` would change nothing.
   A branch whose squash landed and whose files `main` edited later does not pass that test.
   The plan expected about 24 landed branches in verus; prune found 4. ortho, limen, forum and
   budgetron keep long lists of old branches for the same reason. A second test could fill the
   gap: a `merge: <branch>` commit on `main` naming the branch, which every trunk squash writes.
   Branches from before the trunk flow have no such commit, so they stay hand work.

## Before tasks

This plan only collects findings. Before any implementation, run the feature-planning workflow
(`cinch workflow dev-plan-feature`): decide each option above with the owner, write the
template's test plan (`tests/scripts.sh` and the `make check` guards), and split the work into
tasks. Item 7 changes a managed script, so every repo picks it up on its next `sync`.
