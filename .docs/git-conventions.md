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

`internal/version/VERSION` holds `MAJOR.MINOR.PATCH`. `.githooks/post-commit` reads
the commit subject to decide the bump, then folds the change into the triggering
commit via `git commit --amend --no-edit --no-verify`.

| Field | Trigger |
|---|---|
| major | manual only — `make version-major` |
| minor | auto: `merge: feature/…`, `merge: refactor/…`, or `merge: chore/…` squash-merged onto `dev` |
| patch | auto: `merge: fix/…` squash-merged onto `dev` |

A bump only fires for a single-parent (squash-merge) commit that touches `main.go`,
`internal/`, or `Makefile` — docs/notes-only commits never bump.

## Release

```
git checkout main
git merge --no-ff dev -m "merge: dev"   # resolve conflicts if any, then commit
make release
git push --follow-tags origin main dev
```

`make release` tags `v$(scripts/update-version.sh show)` (skipping if that tag
already exists — i.e. no version change since the last release) and fast-forwards
`dev` to `main`.

## Manual version commands

- `make version-show` — print the current version.
- `make version-major` — bump major, reset minor/patch.
