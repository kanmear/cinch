# 28. Absorb the squash-message and version-bump scripts

**Part A tier 1 — trust the checker. Part B tier 3 — decide before investing. Cost estimate:
A is about a day with tests; B is two or three days, most of it the replay against consumers.**

## Provenance

**Field data.** A conventions sweep of every consumer on 2026-10-09, a hand alignment the same
day, and a re-measure on 2026-10-27. Nothing here comes from the audit or the benchmarks.

## Why

Every dev/main consumer carries the same two scripts, registered as cinch hook entries:

- `scripts/check-squash-merge-msg.sh` (commit-msg). It requires `merge: <branch>` while a
  squash-merge is in progress, exempts real merges, and checks every other subject against a
  type pattern.
- `scripts/post-commit-version-bump.sh` (post-commit). It reads the subject, picks a bump level,
  runs the repo's version writer, stages the result and amends the commit.

The first exists because `checkCommit` (`internal/cinch/checks_misc.go:61`) matches one
`commit.pattern` against every subject with no knowledge of git's state. A pattern strict enough
for ordinary commits rejects git's merge subjects, and a pattern loose enough to accept them
can't require the squash shape the bump depends on. Four consumers leave `commit.pattern` unset
and say so in a comment: `cinch.yml` here, `lucky_to_have_you`, `ortho-store` and
`project_deltadocs`. `budgetron` takes the other road: its pattern accepts `merge: <branch>`
at any time. That passes a squash-merge committed as `feat: …`, which then never bumps.

## Evidence: copies drift, and hand alignment doesn't hold

| Date | `check-squash-merge-msg.sh` | `post-commit-version-bump.sh` |
|---|---|---|
| 2026-10-09, before alignment | 4 copies, 3 variants | 5 copies, 4 variants |
| 2026-10-09, after alignment | identical in `ortho-store`, `project_deltadocs` and `lucky_to_have_you`; this repo's differs | identical in `ortho-store` and `project_deltadocs`; `lucky_to_have_you` a single-service variant |
| 2026-10-27 | 3 variants again: `project_deltadocs`' alignment branch never merged | 5 variants in 5 copies: `lucky_to_have_you` moved to per-service versions on 2026-10-18 |

Two lessons. A fix made by hand has to merge in every repo before it counts, and one didn't.
And the copies change for real reasons (per-service versions) that never reach the others.

The decision logic is the same everywhere. What differs is where the version lives: four plain
integer files here (`internal/version/`), `pubspec.yaml` in `budgetron`, Go and TS files per
service in `ortho-store` and `project_deltadocs`. That split is the design below.

## Part A: a commit check that knows git's state

```yaml
commit:
  pattern: '^(feat|fix|ui|docs|style|refactor|test|chore|perf)(!)?( \[[^]]+\])?: .+'
  # While $GIT_DIR/SQUASH_MSG exists: the subject must match the entry for the
  # current branch. A branch with no entry falls back to commit.pattern.
  squash:
    dev: '^merge: (feature|refactor|fix|chore|docs)/.+'
    main: '^merge: hotfix/.+'
  # While $GIT_DIR/MERGE_HEAD exists: exempt (default) or pattern.
  merge: exempt
```

- `checkCommit` reads the git dir once and picks the pattern: squash entry, then the merge rule,
  then `commit.pattern`.
- A finding names which state it was judged in ("squash-merge onto dev must match …"), so the
  hint from `b7de782` still applies to real merges.
- No `squash` or `merge` key means today's behaviour, byte for byte.
- One choice differs from the scripts: a squash onto an unlisted branch is checked against
  `commit.pattern`. This repo's script skips checking there entirely.

That removes `check-squash-merge-msg.sh` from all four repos, and lets `budgetron` drop the
`merge:` alternatives from its pattern.

## Part B: the version bump as a hook step

```yaml
version:
  branches: [dev, main]
  # First match wins, against the subject of a single-parent commit.
  levels:
    - {subject: '^merge: feature/', level: minor}
    - {subject: '^merge: (fix|refactor)/', level: patch}
    - {subject: '^merge: hotfix/', level: hotfix, branch: main}
    - {subject: '^(fix|refactor)( \[[^]]+\])?: ', level: hotfix, branch: dev}
  services:
    backend:
      when: [backend/]
      run: bun scripts/update-version.js backend {level}
      stage: [backend/version/]
    frontend:
      when: [frontend/]
      run: bun scripts/update-version.js frontend {level}
      stage: [frontend/src/lib/constants/version/]
```

At post-commit, before `hooks.post-commit` entries, cinch:

1. Skips a merge commit (more than one parent), a branch not in `branches`, and a subject no
   level matches.
2. Skips re-entry: a lock file in the git dir, as every copy has now. The amend fires
   post-commit again, and cinch owns that guard instead of each script.
3. For each service whose `when` prefixes match the files changed in `HEAD^..HEAD`, runs `run`
   with `{level}` substituted, then stages `stage`.
4. Amends once with `--no-edit --no-verify` if anything was staged. A failing `run` is reported
   in the hook summary and nothing is amended.

The writers stay in each repo (`update-version.js`, `update-version.sh`), because the storage
format really does differ. A single-service repo has one service; `odin_web` and
`odin_operator`, which bump on every commit, use one catch-all level.

**Threshold, set before building:** build B only if at least three consumers share the decision
logic once A ships. Today it's five (this repo, `budgetron`, `lucky_to_have_you`, `ortho-store`,
`project_deltadocs`), so it is met.

## What stays in the repos

- **The version writers.** Storage is project-specific.
- **`scripts/release.sh` and `make release`.** Tagging and the dev fast-forward are short and
  rarely change; reopen if they start drifting too.
- **`check_error_codes.sh`.** A project contract, which is what `hooks.<event>` scripts are for.
- **`cinch-log.sh`.** An instrument for [19](19-field-log.md); it shouldn't become a feature
  before that study ends.

The README's line that cinch "only runs these scripts" stays true for everything above. This
item moves only the git-state mechanics that every consumer reimplements the same way.

## Verification / definition of done

- **Rules.** New `CINCH-0NN` rules in `.docs/rules.md`, each bound by a marker: the squash
  pattern applies only while `SQUASH_MSG` exists; `MERGE_HEAD` is exempt by default; no new key
  means no behaviour change; a bump never fires on a merge commit or on re-entry; a failed
  writer means no amend.
- **Unit tests** in temp repos, like `staged_test.go`: squash onto dev with a good and a bad
  subject; squash onto an unlisted branch; a real merge; level selection; two services changed
  in one commit amending once; a writer that exits 1.
- **Catch matrix.** One row for "squash-merge committed with an ordinary subject", caught by
  Part A, and listed as a gap until it ships.
- **Replay.** In scratch clones of `ortho-store`, `project_deltadocs`, `lucky_to_have_you`,
  `budgetron` and this repo: delete the two scripts, add the config, replay the last ten squash
  merges, and diff the version files against history. Identical output is the bar.
- **Migration.** Each consumer removes its scripts in its own commit once it requires the
  release that ships this. `cinch upgrade` doesn't rewrite `cinch.yml`; the README documents the
  keys.

## Open

- `PATH` for hook commands. Every copy prepends tool paths (`mise` shims, `~/.bun/bin`, `go`)
  because GUI git clients run hooks with a bare `PATH`. Part B inherits that. A `hooks.path`
  key would fix it for every hook entry and is a separate item if wanted.
