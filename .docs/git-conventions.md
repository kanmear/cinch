# Git conventions

Branch model, commit-message shapes, and the auto-versioning scheme. The message
shapes are enforced by cinch's own `commit` check from the `commit:` block in `cinch.yml`
(CINCH-015, CINCH-016); the version bump by `scripts/post-commit-version-bump.sh`. Ported
from `project_deltadocs`'s version-bump hooks and adapted for a single-binary Go repo.
Cinch runs on itself: `.githooks/*` are shims rendered by `cinch init`, and they call
`cinch hook <event>`. That runs cinch's own checks, then dispatches to the bump script
registered under `hooks.post-commit` in `cinch.yml`.

## Setup

New clone: `make setup-hooks`. It installs the current cinch onto `PATH` (the shims
exec it) and runs `cinch init`, which renders `.docs/workflows/*` and the shims and
sets `core.hooksPath`. Rendered files are generated. Never edit them by hand. Change
the templates or `cinch.yml` and re-render. After changing cinch's checks or
templates, run `make install` again so the hooks use the new build.

## Branch model

- `dev` — integration branch. Feature/fix/refactor/chore work squash-merges here.
- `main` — release branch. Only receives the manual `dev -> main` release merge.

## Commit message shapes

- **Squash-merge onto `dev`:** `merge: <branch-name>`, where `<branch-name>` must
  start with `feature/`, `refactor/`, `fix/`, `chore/`, `docs/`, `perf/` or `ui/`.
  Enforced by `commit.squash.dev` in `cinch.yml`, which the commit-msg hook applies only
  while `$GIT_DIR/SQUASH_MSG` exists (i.e. mid `git merge --squash`).
- **Squash-merge onto `main`:** `merge: hotfix/<name>` (`commit.squash.main`).
- **Every other commit:** `<type>[!] [<scope>]: <description>` — types: `feat`, `fix`,
  `ui`, `docs`, `style`, `refactor`, `test`, `chore`, `perf`; `!` marks a breaking
  change; scope optional, lowercase letters, `+` and `-` only (`commit.pattern`).
- Real merge commits (`git merge --no-ff` — the release cut) are exempt
  (`commit.merge: exempt`): git's own default merge message stands.

## Trunk-flow guards

Two `cinch.yml` keys enforce the trunk flow of
[plan 33](plans/33-harness-alignment.md) (`main` only, topic branches land as squash
commits through `make merge`). Both run in the commit-msg hook. This repo turns them on
when it switches to that flow (plan 33 T12); until then it keeps `commit.merge: exempt`
for the release cut and protects no branch.

- **`commit.merge: reject`** (CINCH-020): any commit made while `$GIT_DIR/MERGE_HEAD`
  exists fails, whatever its subject, and the finding points at `make merge`. A
  `--no-ff` merge can't land, so no bump is lost to a 2-parent commit.
- **`commit.protect: [main]`** (CINCH-021): a commit on a listed branch passes only
  mid squash merge (`SQUASH_MSG` exists), or mid merge when `commit.merge` isn't
  `reject`. A plain commit on `main` fails. The first commit on an unborn
  branch and a commit on a detached HEAD pass. The version bump's
  `--no-verify` amend runs no hooks, so it isn't checked.

## Version bump table

`internal/version/{major,minor,patch,hotfix}` each hold one plain integer — the
version is `x.y.z[+letter]`, where the hotfix letter is a bijective base-26
encoding of the hotfix integer (`0` → no `+letter` suffix at all, `1` → `+a`,
`26` → `+z`, `27` → `+aa`, …), computed on demand by
`scripts/update-version.sh show`. The letter is attached as semver *build
metadata* (`+a`), not a pre-release (`-a`): a bare suffix (`0.2.3a`) isn't valid
semver at all and breaks goreleaser's tag parsing, and a `-a` pre-release segment
would parse but gets flagged as a pre-release release on GitHub, which isn't the
intent — these are real bumps, not release candidates. Build metadata is ignored
for semver *precedence* (`0.2.3+a` and `0.2.3+b` compare equal), but nothing here
relies on semver-precedence ordering — git tags are distinct strings and release
order follows tag/push order.

Kept as four separate files rather
than one combined file: even edits to *different* fields can conflict when two
people's direct/squash-merge commits on `dev` are later reconciled (rebase/merge),
because git's default 3-line diff context overlaps adjacent single-line edits in a
small file — two different files can never conflict with each other.

`scripts/post-commit-version-bump.sh` reads the commit subject to decide the bump, then folds the
change into the triggering commit via `git commit --amend --no-edit --no-verify`.

| Field | Trigger |
|---|---|
| major | manual only — `make version-major` |
| minor | auto: `merge: feature/…` squash-merged onto `dev` |
| patch | auto: `merge: fix/…`, `merge: refactor/…`, `merge: perf/…` or `merge: ui/…` squash-merged onto `dev` |
| hotfix (letter) | auto: a direct `fix: …` or `refactor: …` commit landing on `dev` (not a squash-merge) |

`merge: chore/…` and `merge: docs/…` squash-merges never bump anything. A bump only
fires for a single-parent commit (squash-merge result, or a direct commit) that
touches `main.go`, `internal/`, or `Makefile` — docs/notes-only commits never bump.

A minor bump resets patch and hotfix to 0; a patch bump resets hotfix to 0; a hotfix
bump resets nothing (it's the lowest field).

## Release

```
git checkout main
git merge --no-ff dev -m "merge: dev"   # resolve conflicts if any, then commit
make release
make push-release
```

`make release` tags `v$(scripts/update-version.sh show)` (skipping if that tag
already exists — i.e. no version change since the last release) and fast-forwards
`dev` to `main`. `make push-release` runs `git push --follow-tags origin main dev` —
a separate, deliberate step so nothing publishes until you choose to (and so the
tag can't be dropped by pushing without `--follow-tags`).

## Manual version commands

- `make version-show` — print the current version.
- `make version-major` — bump major, reset minor/patch.
