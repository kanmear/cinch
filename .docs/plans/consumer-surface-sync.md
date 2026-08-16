# cinch — resync `project_deltadocs` to cinch's current surface

Status: **proposed** — not started.

## Context

Five sessions of cinch changes landed on 2026-08-16 (`2d572e8` through
`9cea562`). Cinch self-hosts every one of them, so its own tree is clean —
but `project_deltadocs`, the only other consumer, has not been touched. It is
now behind in four separate ways, one of which leaves its `cinch check` red.

The general shape of the problem is the version-skew tax the README already
names: cinch ships, and every consumer needs a follow-up commit before it is
green again. This plan is that commit. It exists because the four items are
easy to lose track of individually, and because item 1 means the consumer is
*failing* right now, not merely out of date.

Verified against `~/code/project_deltadocs` on 2026-08-16.

## What drifted

1. **Seven rendered workflows are stale — `cinch check` is red there.**
   The template genericization (`8bafaea`) changed the shipped templates, so
   the consumer's `.agent/workflows/` no longer byte-matches a fresh render.
   Its `generated` check reports 7 findings. Its in-memory re-render succeeds
   against its own manifest values, so the fix is exactly `cinch render` —
   no content decisions to make.

2. **`AGENTS.md` has no `cinch context` pointer.** It carries the
   `cinch workflows` line (`AGENTS.md:46`) and the `cinch index` line
   (`AGENTS.md:55`), both predating `276aedd`. `cinch init` now scaffolds a
   third, but only into a file it creates — this consumer already had one, so
   init would only print the suggestion. Without the line, an agent starting
   a session in that repo has no session-start entry point.

3. **No `plans.statuses`.** Its plans use `open`/`complete`
   (`.agent/plans/*.md`), a two-term vocabulary that would order correctly
   under `e50064f`. Without the key, `cinch context` lists its plans in path
   order and completed work sorts among live work. Purely a quality-of-life
   gap — no failure state.

4. **A commit convention enforced by script rather than by `commit.pattern`.**
   `hooks.commit-msg.squash-format` runs `scripts/check-squash-merge-msg.sh`,
   which enforces the `merge: <branch>` subject that the post-commit
   version-bump entry reads. This is *already* the workflow-postcondition
   discipline (`24debab`) working as intended — a real postcondition in the
   manifest — so it needs no change. Recorded here only so a later reader
   doesn't "fix" it into a `commit.pattern` it doesn't fit: the script does
   conditional work `commit.pattern`'s single regex cannot express (it
   no-ops outside squash merges).

## Steps

1. `cinch render` in `project_deltadocs`; confirm `cinch check` returns to
   clean. Commit the 7 changed workflows on their own, with a message naming
   the cinch commit that caused the change (`8bafaea`) — a re-render commit
   that also carries hand edits is the one shape that makes the `generated`
   check useless as evidence.
2. Add the `cinch context` line to `.agent/`'s `AGENTS.md`, next to the
   existing workflows and index pointers. Copy the wording from
   `agentsContextLine` (`internal/cinch/init.go`) rather than paraphrasing,
   so the two stay comparable.
3. Add `plans.statuses: [open, complete]` to `project_deltadocs/cinch.yml`.
   Confirm with `cinch context` that its open plans now sort above its
   complete ones.
4. Leave item 4 above alone. No step.

## Verification

- `cinch check` exits 0 in `project_deltadocs`.
- `cinch context` there prints the branch, its plans with `open` first, the
  workflow table, and staged paths.
- `git log` in the consumer shows the re-render as its own commit, separable
  from the two config edits.
- Re-run `cinch check` in cinch itself: unaffected, but confirm rather than
  assume, since step 1 is run with the installed binary and this is exactly
  the situation where a stale one misleads (see README § Install — templates
  are `go:embed`-ed, so `make install` must have run after `8bafaea`).

### Critical files

- `project_deltadocs/.agent/workflows/*.md` (re-render output, 7 files)
- `project_deltadocs/AGENTS.md`
- `project_deltadocs/cinch.yml`

### Relationship to other plans

Independent of the two plans still open. Worth doing before
`docs-path-migration.md`, whose Step 3 also edits consumer files
(`inject-agents.sh`) — landing this first means that plan starts from a green
consumer rather than debugging a red one it didn't cause.

This plan is disposable: it is a checklist for one sync, not a standing
document. Delete it once the consumer is green, per this repo's rule that
historical artifacts are not kept alive.
