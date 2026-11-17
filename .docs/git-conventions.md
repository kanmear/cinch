# Git conventions
<!-- harness-template v2 -->

The one git doc: branch model, commit shapes, the hooks, the version bump, releases. The rule
items live in [Conventions](conventions.md#git); the mechanics live here. `.githooks/*` are shims
rendered by `cinch init`: they call `cinch hook <event>`, which runs cinch's own checks and then
the scripts registered under `hooks.*` in `cinch.yml`. cinch runs on itself, so this repo's hooks
are its own checks: the message shapes are cinch's `commit` check (CINCH-015, CINCH-016,
CINCH-020, CINCH-021). The hooks exec the installed `cinch`; after changing a check or a template,
run `make install-bin` so they use the new build.

## Branch model

Trunk with tags. `main` is the only long-lived branch.

- Topic branches (`feature/`, `fix/`, `refactor/`, `perf/`, `ui/`, `chore/`, `docs/`) start from
  `main`, one per change. A hotfix is a `fix/` branch: there is no `hotfix/` and no `dev`.
- `make merge`, run from the topic branch, squashes it onto `main` as one commit
  `merge: <branch>`, runs the hooks on it, and leaves you on `main`. The topic branch is kept.
- `make prune` lists local topic branches whose squash already landed on `main`;
  `make prune confirm=1` deletes them.
- Nothing is committed on `main` except that squash. A conflicting squash stops on `main` with the
  files listed: resolve, `git add`, then `git commit -m "merge: <branch>"`.

## Commit message shapes

- **Squash onto `main`:** `merge: <branch-name>`, where `<branch-name>` starts with `feature/`,
  `refactor/`, `fix/`, `chore/`, `docs/`, `perf/` or `ui/`. `commit.squash.main` in `cinch.yml`
  enforces it, and the commit-msg hook applies it only while `$GIT_DIR/SQUASH_MSG` exists (that
  is, mid `git merge --squash`).
- **Every other commit** (on a topic branch): `<type>[!] [<scope>]: <description>`. Types: `feat`,
  `fix`, `ui`, `docs`, `style`, `refactor`, `test`, `chore`, `perf`; `!` marks a breaking change;
  scope is optional, lowercase letters, `+` and `-` only (`commit.pattern`).
- A real merge commit (`git merge`, `--no-ff` or not) is rejected: `commit.merge: reject` fails
  any commit made while `$GIT_DIR/MERGE_HEAD` exists and names `make merge`.

## Enforcement

Three `commit:` keys in `cinch.yml`, all checked by the commit-msg hook:

- `commit.squash.main`: the subject shape of a squash onto `main`.
- `commit.merge: reject`: no merge commits (CINCH-020), so no bump is lost to a 2-parent commit.
- `commit.protect: [main]`: a commit on `main` passes only mid squash merge (`SQUASH_MSG` exists).
  The first commit on an unborn `main` and a commit on a detached HEAD pass (CINCH-021).

Both keys came with [plan 33](plans/33-harness-alignment.md), and this repo has them on: before
the switch it kept `commit.merge: exempt` for the release cut and protected no branch. A
`--no-ff` merge can't land now, so no bump is lost to a 2-parent commit.

The version bump's `--no-verify` amend runs no hooks, so it is not checked: git skips them by
design.

## Setup

`make install` installs the dependencies and runs `cinch init`, which renders `.docs/workflows/*`
and the `.githooks/` shims and sets `core.hooksPath`. Rendered files are generated: never edit
them by hand, change the templates or `cinch.yml` and re-render.

A fresh clone, including every cloud session, has no hooks until `cinch init` has run. The Claude
Code SessionStart hook `.claude/hooks/check-git-setup.sh` runs it when `core.hooksPath` is unset
and `cinch` is installed. Otherwise it only warns: that `cinch` is missing, that `core.hooksPath`
points elsewhere (it is never changed), or that the session starts on `main`.

## Hooks

Configured under `hooks.*` in `cinch.yml`, not in the shims. cinch runs `cinch check` first
(pre-commit only), then each entry whose `when` path prefixes match the staged set (for
post-commit, the committed set).

- **`pre-commit`** runs `cinch check`, then `make check`. A squash also runs the tests:
  `scripts/pre-commit-merge-test.sh make test` does nothing unless `$GIT_DIR/SQUASH_MSG` exists.
  Per-area entries (`make check-backend` when `backend/` is staged) are an alternative to the
  single `make check`.
- **`commit-msg`** checks the subject and the branch guard as described above.
- **`post-commit`** runs `scripts/post-commit-version-bump.sh`, which bumps the version as below
  and folds it into the commit with `git commit --amend --no-edit --no-verify`.

## Version

One version per repo, tag `v<x.y.z>`. It is stored in the root `VERSION` file, one line `x.y.z`.
Where a stack differs (Flutter's `pubspec.yaml` `x.y.z+N`, Android's `version.properties`), the
stack conventions doc says so. `scripts/version.sh` reads and writes it; never hand-edit the file.
`make version` prints it; Go binaries get it through `-ldflags -X`, and a SPA through a vite
`define`.

The post-commit hook bumps only on `main`, only for a single-parent commit whose subject is a
`merge: <prefix>/...` squash, and only if the squash touches a path matching one of the arguments
`cinch.yml` passes to the script (``main.go`, `internal/*`, `go.mod`, `go.sum``). A docs-only or tooling-only squash never
bumps. The binary is `main.go` plus `internal/`, including the embedded non-`.go` workflow
templates, so a change there bumps; `bench/` code, docs, the Makefile and CI do not.

| Squash subject | Bump |
|---|---|
| `merge: feature/...` | minor |
| `merge: fix/...`, `refactor/...`, `perf/...`, `ui/...` | patch |
| `merge: chore/...`, `docs/...` | none |
| major | by hand: `make version-major` |

A minor bump resets patch; a major bump resets both.

## Release

```
make release     # tag v<version> on main, push main and the tag to origin
make ship        # make release, then the stack's delivery
```

`make release` (`scripts/release.sh`) refuses to run off `main`, with uncommitted changes to
tracked files, or with a merge or squash in progress. The tag is annotated. It can be rerun: a tag
already on `HEAD` is reported and the push still happens; a tag on an older commit means nothing
version-relevant has landed since, so nothing is tagged and `main` is still pushed.

`make ship` is `release`; for cinch that is the whole delivery. The `v<version>` tag push runs
`.github/workflows/release.yml`, which runs goreleaser (`.goreleaser.yml`) and publishes the GitHub
release that `install.sh` and `cinch upgrade` install from. goreleaser stamps `main.Version`, the
variable `make build` stamps too. `make deploy` and `make logs` do not apply (exit 2).

`make merge` and `make deploy` print one line per step and a closing summary instead of the raw
git, hook and build output. `V=1` shows everything, and the logs are kept in `.git/make-logs/`
(newest 50).

A `make` target and not a git hook: a hook would silently mint a near-permanent tag.
