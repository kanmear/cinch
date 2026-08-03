# Git Conventions

The versioning machinery: the hooks, the scheme, and release tags.

Branching model: `feature/`, `refactor/`, `fix/`, `chore/`, `docs/` branches squash-merge onto
`dev`; `dev` is `--no-ff` merged onto `main` as a release; `hotfix/` branches squash-merge onto
`main` and are then synced down to `dev`.

## Hooks

Two hooks in `.githooks/`, activated by `make setup-hooks` (`core.hooksPath`). They cooperate
around the squash-merge workflow: `commit-msg` enforces the message format that `post-commit`
reads to decide what to bump. (No `pre-commit`: checks run under `make test`/`make fixtures`.)

### `commit-msg`

Enforces the `merge: <branch>` format for squash-merge commits onto `dev`/`main` — the signal
`post-commit` relies on. Only fires during the squash-merge window (`SQUASH_MSG` present).

- `dev`: `merge: <feature|refactor|fix|chore|docs>/<name>`
- `main`: `merge: hotfix/<name>` (the `dev → main` release cut is a plain `--no-ff` merge, so it
  never enters this hook)

### `post-commit`

Auto-bumps the version fields and folds the change into the triggering commit via
`git commit --amend --no-edit --no-verify`. Only fires for single-parent commits on `dev`/`main`
with a matching message; 2-parent merges (the release cut and the `main → dev` sync) are skipped.

## Scheme

cinch carries a single `x.y.z[letter]` version (e.g. `0.1.0`, `0.1.1a`) as four raw integers,
**each in its own generated file**: `version/{major,minor,patch,hotfix}.go`. The bump script
lives in the binary (`cinch version`), invoked by the hooks and release script as
`$CINCH version bump <type>` / `$CINCH version show`. Never hand-edit the generated files;
`make version-show` prints the formatted string (computed on demand — deliberately not stored).

| Field | Meaning | Trigger |
| --- | --- | --- |
| `x` (major) | Manual milestone/breaking marker | `make version-major` only |
| `y` (minor) | New dev-branch integration | Auto: `merge: feature\|refactor\|chore/...` onto `dev` touching the product |
| `z` (patch) | Regular bug fix | Auto: `merge: fix/...` onto `dev` |
| `letter` | Out-of-band fix on top of `x.y.z` | Auto: `merge: hotfix/...` onto `main` (a→b→c…) |

The bump *type* comes from the branch prefix; whether anything happens at all comes from the
changed paths — the product is `*.go`, `Makefile`, `go.mod`, `go.sum`, `templates/`,
`scaffold/`, `fixtures/`. A merge touching only `docs/`, `decisions.jsonl`, `.githooks/`, `bin/`,
or `version/` itself lands with no version change.

A `y`/`z` bump resets everything below it. A `dev → main` release merge doesn't bump (2-parent,
skipped) and `main` inherits `dev`'s version files; the later `main → dev` sync merge is likewise
skipped, and git reconciles the fields without conflict **because each field is a separate file**
— two files can never conflict regardless of diff context. (A single-file layout did conflict in
practice: two single-line edits in a 4-line block overlap under git's default 3-line context.)
This is also why the formatted string is never stored — both sides would rewrite it every bump.

Known edge case: if `dev` bumps patch before a pending hotfix is synced down, the sync merge
combines both changes (e.g. `0.1.2` + hotfix → `0.1.2a`) rather than clearing the stale letter.
Harmless; reset the letter by hand if it matters. Sync `main → dev` promptly after each hotfix.

### `cinch version`

`cinch version show` prints the formatted version; `cinch version bump <type>` rewrites the four
`version/` files and prints `old -> new`. Files resolve next to the binary
(`<binary>/../version`, the clone's `version/`), so `show` works from any directory — including
a consumer repo, to compare against its `harness_version` pin (D008). Nothing is compiled into
the binary: the files are the single source of truth, so `show` never goes stale after a bump.

## Release tags

Hooks only bump the integers; tags are cut at release time. `make release` — after the manual
`dev → main` `--no-ff` merge is committed on `main` — creates one annotated tag
`cinch-vX.Y.Z` (hotfix letter included), then fast-forwards `dev` up to `main`. Push with
`git push --follow-tags origin main dev`.

- **Tag the service, not the repo.** No new tag if the version hasn't changed.
- **A `make` target, not a git hook.** The `--no-ff` merge stays manual so conflicts are visible;
  `make release` only performs the mechanical tail. Hooks are a poor fit — `post-merge` doesn't
  fire on conflicted merges, and silently minting near-permanent tags is surprising.
