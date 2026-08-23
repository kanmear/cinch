# Git conventions

Branch model, commit-message shapes, and the auto-versioning scheme enforced by
`.githooks/commit-msg` and `.githooks/post-commit`. Ported from `project_deltadocs`'s
version-bump hooks, adapted for a single-binary Go repo and wired as plain git hooks
rather than through cinch's own `hooks.*` dispatch — cinch doesn't dogfood its own
hook-dispatch mechanism here.

## Setup

New clone: `make setup-hooks` (`git config core.hooksPath .githooks`).

## Branch model

- `dev` — integration branch. Feature/fix/refactor/chore work squash-merges here.
- `main` — release branch. Only receives the manual `dev -> main` release merge.

## Commit message shapes

- **Squash-merge onto `dev`:** `merge: <branch-name>`, where `<branch-name>` must
  start with `feature/`, `refactor/`, `fix/`, `chore/`, or `docs/`. Enforced by
  `.githooks/commit-msg` only while `$GIT_DIR/SQUASH_MSG` exists (i.e. mid
  `git merge --squash`).
- **Every other commit:** `<type> [<scope>]: <description>` — types: `feat`, `fix`,
  `ui`, `docs`, `style`, `refactor`, `test`, `chore`, `perf`; scope optional.
- Real merge commits (`git merge --no-ff` — the release cut) are exempt: git's own
  default merge message stands.

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

`.githooks/post-commit` reads the commit subject to decide the bump, then folds the
change into the triggering commit via `git commit --amend --no-edit --no-verify`.

| Field | Trigger |
|---|---|
| major | manual only — `make version-major` |
| minor | auto: `merge: feature/…` squash-merged onto `dev` |
| patch | auto: `merge: fix/…` or `merge: refactor/…` squash-merged onto `dev` |
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
