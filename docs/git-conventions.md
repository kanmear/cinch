# Git Conventions

The versioning machinery behind cinch releases: the hooks, the single-service versioning
scheme, and release tags. Ported from `project_deltadocs/.agent/git-conventions.md` and adapted
for a one-binary Go repo (see D043). The branching model: `feature/`, `refactor/`, `fix/`,
`chore/`, `docs/` branches squash-merge onto `dev`; `dev` is `--no-ff` merged onto `main` as a
release; `hotfix/` branches squash-merge onto `main` and are then synced down to `dev`.

## Hooks

Two hooks live in `.githooks/` and are activated by `make setup-hooks` (sets `core.hooksPath` to
`.githooks` and marks them executable). They cooperate around the squash-merge workflow:
`commit-msg` enforces the message format that `post-commit` then reads to decide what to bump.
(There is no `pre-commit` here — that hook in the source project ran the consumer's path-scoped
checks; cinch's checks run under `make test`/`make fixtures` instead.)

## `commit-msg`

Enforces the `merge: <branch>` commit message format for squash-merge commits landing on
`dev`/`main` — this is the signal `post-commit` relies on to decide which part of the version
(minor/patch/hotfix) to bump.

- Only fires during the squash-merge window (`SQUASH_MSG` present); regular commits on other
  branches are untouched.
- On `dev`, the message must match `merge: <feature|refactor|fix|chore|docs>/<name>`.
- On `main`, the message must match `merge: hotfix/<name>`. The `dev → main` release cut is a
  plain `--no-ff` merge (no `SQUASH_MSG`), so it never enters this hook.

Source: `.githooks/commit-msg`.

## `post-commit` — Versioning

Auto-bumps the cinch version fields and folds the change into the triggering commit via
`git commit --amend --no-edit --no-verify`. Only fires for single-parent commits on `dev`/`main`
whose message matched the `commit-msg` format above (merge commits with 2+ parents — e.g. the
`dev → main` release merge and the `main → dev` hotfix sync — are explicitly skipped).

Source: `.githooks/post-commit`.

### Scheme

cinch carries a single `x.y.z[letter]` version (e.g. `0.1.0`, `0.1.1`, `0.1.1a`), tracked as four
raw integers, **each in its own generated file** (not grouped into one struct/object):
`version/{major,minor,patch,hotfix}.go` (package `version`). The bump script lives in the binary
itself as `cinch version` — no node dependency (D003 — core is Go), so the hooks and release
script call `$CINCH version bump <type>` / `$CINCH version show`.

**Never hand-edit the generated files** — they're rewritten by `cinch version bump`, invoked
automatically by `.githooks/post-commit`. Run `make version-show` to print the formatted
`x.y.z[letter]` string (`cinch version show` computes it on demand — it is deliberately not
stored, see below).

| Field | Meaning | Trigger |
| --- | --- | --- |
| `x` (major) | Manual milestone/breaking marker | `make version-major` only |
| `y` (minor) | New dev-branch integration | Auto: a `merge: feature\|refactor\|chore/...` squash-merge lands on `dev` and touches the product |
| `z` (patch) | Regular bug fix | Auto: a `merge: fix/...` squash-merge lands on `dev` |
| `letter` | Urgent out-of-band fix, layered on top of the current `x.y.z` | Auto: a `merge: hotfix/...` squash-merge lands on `main` (a→b→c…) |

`docs/*` merges never bump the version. More generally, **a bump only happens if the squash
touched the product**: the bump *type* comes from the branch prefix, but whether anything happens
at all comes from the changed paths — `*.go`, `Makefile`, `go.mod`, `go.sum`, `templates/`,
`scaffold/`, `fixtures/`. A merge touching only `docs/`, `decisions.jsonl`, `.githooks/`, `bin/`,
or the `version/` files themselves lands with no version change.

A `y` or `z` bump resets everything below it (minor bump zeroes patch+letter; patch bump zeroes
letter). A `dev → main` release-cut merge (`git merge --no-ff dev`) does not bump anything: it's
a 2-parent commit, skipped by the hook, and `main` inherits `dev`'s version fields directly from
the merge. The subsequent `main → dev` sync merge (after a hotfix) is a 2-parent merge and is
explicitly skipped by the hook; git reconciles the fields without conflict because **each field
is a separate file** — two different files can never conflict with each other, regardless of how
git's diff context happens to fall. (The source project put all four fields in one file, one per
line, and that turned out to still conflict in practice, because git's default 3-line diff
context is wide enough to make two single-line edits in a 4-line block overlap. Hence the
one-field-one-file layout.) This is also why the formatted `x.y.z[letter]` string is never
stored in a generated file: it would be rewritten by both sides on every bump and reliably
conflict, even though the underlying integers don't.

Known edge case: if `dev` bumps patch again *before* a pending hotfix is synced down from `main`,
the sync merge combines both changes (e.g. `dev` patch 1→2, `main` hotfix 0→1 land together as
`0.1.2a`) rather than clearing the now-stale letter. This is harmless (no conflict, no data
loss) but can look odd; reset the hotfix field by hand in a follow-up commit if it matters.
Syncing `main → dev` promptly after each hotfix avoids it.

### `cinch version` — the bump script

Replaces `project_deltadocs/scripts/update-version.js`. `cinch version show` prints the formatted
version; `cinch version bump <major|minor|patch|hotfix>` rewrites the four `version/` files and
prints `old -> new`. The version files are resolved **next to the binary** (`<binary>/../version`,
i.e. the cinch clone's `version/`), so the command works from any directory — including inside a
consumer repo, where `show` reports the installed tool's version for comparison against the
consumer's `harness_version` pin (D008). Nothing is compiled into the binary: the files are the
single source of truth, so `show` can never go stale after a bump.

## Release tags

The hooks only *bump* the version integers (on squash-merges into `dev`). Version *tags* are cut
separately at release time and are **not** touched by any hook.

`make release` — run after the manual `dev → main` `--no-ff` merge is committed on `main` —
creates one annotated tag, `cinch-vX.Y.Z` (hotfix letter included, e.g. `cinch-v0.1.1a`), then
fast-forwards `dev` back up to `main`. Push a release with
`git push --follow-tags origin main dev`.

- **Tag the service, not the repo.** The tag sits on the release-merge commit; if the version
  hasn't changed since the last release, no new tag is created (the old one stays put).
- **A `make` target, not a git hook.** The `--no-ff` merge stays manual so conflicts are visible;
  `make release` only performs the mechanical, near-permanent tail (tag + fast-forward). Hooks
  are a poor fit here — `post-merge` doesn't fire on conflicted merges, so a hook would behave
  inconsistently across clean vs conflicted cuts, and silently minting near-permanent tags is
  surprising.

Source: `scripts/release.sh`.
