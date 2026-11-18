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
6. **There is no repo-local layer for managed `.claude/settings.json` (T13).** `sync`
   overwrites the whole file, so a repo's own permissions or hooks are lost. Options: a managed
   base merged with a seeded `.claude/settings.repo.json`, or managing only the `hooks` key.
7. **`prune.sh` misses branches that landed long ago (seen while landing T15–T19).** It counts a
   branch as landed only when `git merge-tree --write-tree main <branch>` would change nothing.
   A branch whose squash landed and whose files `main` edited later does not pass that test.
   The plan expected about 24 landed branches in verus; prune found 4. ortho, forum and
   budgetron keep long lists of old branches for the same reason. A second test could fill the
   gap: a `merge: <branch>` commit on `main` naming the branch, which every trunk squash writes.
   Branches from before the trunk flow have no such commit, so they stay hand work.

## Decisions

Taken with the owner on 2026-11-18, through the feature-planning workflow.

1. **`down`** sends C-c to every pane of the dev session, waits up to 5 s for the session to
   end by itself, then kills it. The panes run `make <target>` directly, so ortho's and dd's
   `kill -TERM <pane_pid>` signals only that `make`. C-c sends SIGINT to the pane's whole
   foreground process group, which includes `go run`'s child binary. talk-workbench already
   does it this way. This applies to all five stacks.
2. **`VERSION_VAR ?= main.version`** goes into the parameter block of `go`, `go-cli` and
   `go-svelte`. Every `-X` uses `-X $(VERSION_VAR)=$(VERSION)`, and the stack conventions name
   the variable. The package form can't be the default because the module path isn't `NAME`
   (forum's is `forum_backend/version.Version`). This is forum's workaround made general.
3. **A repo-level skip list: a `skip=` line in `.harness`.** The value is a space-separated
   list of target paths. `apply` neither seeds nor reports them, and `diff` leaves them and the
   `skip=` line out. This covers lthy (eslint config, `bunfig.toml`, `tests/support/*`) and
   item 5 (budgetron skips `.docs/stack-conventions.md`). The stack defaults stay as they are.
   `go` and `go-svelte` also get `DEPLOY_NAME ?= $(NAME)`, and every deploy path (`~/apps/`,
   `~/git/`, `backups/`) uses it.
4. **A `go-svelte-pg` stack that inherits from go-svelte.** `stacks/go-svelte-pg/base.txt`
   names `go-svelte`. `apply` copies the base stack's files and then overlays the stack's own.
   `vars.txt`, `skip.txt` and `managed.txt` fall back to the base's when the stack has none, and
   `files.txt` lists only what the stack adds. Its Makefile is the only duplicate, and the
   contract guard covers it. It has no `DB_PATH`, no `create-owner` or `reset-password`, and no
   `ENV_FILE`. It has the Postgres block (`dev-db`, `stop-db`, `drop-db`, `migrate-*`), `dev`
   starts the database, and `down` stops it. The deploy verbs (`deploy`, `logs`) are stubs that
   exit 2 until a Postgres deploy is designed. The reference is ortho's and dd's current
   Makefiles.
5. **Covered by 3.** budgetron lists `.docs/stack-conventions.md` in `skip=`.
6. **`sync` manages only the `hooks` key of `.claude/settings.json`.** It replaces `.hooks` with the
   template's using `jq`, and keeps every other key. A target without the file gets the template's
   copy. A target whose file isn't valid JSON makes `sync` fail without writing. `tests/managed.sh`
   compares `.hooks` for this path, not bytes.
7. **`prune.sh` gets a second way to count a branch as landed:** `main` has a commit whose
   subject is exactly `merge: <branch>`, and the branch tip's committer time is not later than
   that commit's. The second condition keeps a reused branch name from losing new work. The
   listing says which test matched (`landed  <b>  (no diff)` or `landed  <b>  (merge <sha>)`).

The template goes to `VERSION` 3. Every repo then gets one rollout session (T8–T16), as in
plan 33 Part C.

## Rules

- Existing: none bind here. harness-template has no numbered rules of its own. Its contract is
  its `README.md`, enforced by `make check` (`tests/*.sh` and the `check` recipe). The seeded
  TOOL-001 and GIT-001 don't change.
- New: none as cinch rules. Four invariants become README contract statements, each with its
  test below:
  - `down` leaves no process from the dev session running (README § make check).
  - `apply` and `diff` never seed, report or diff a path in the repo's `skip=` (§ Apply, sync,
    diff).
  - `sync` changes only the `hooks` key of `.claude/settings.json` (§ Managed and seeded).
  - `prune.sh` deletes a branch only when one of its two landed tests matches (§ Choices).

## Test Plan

All tests are in `~/code/harness-template` and run under `make check`. New apply-level
assertions go into a new `tests/apply.sh`, which `make check` calls. The existing assertions
stay in the `check` recipe.

- [ ] down: in every stack, `make down` on a live tmux session whose pane runs a child that
  ignores SIGHUP leaves no such process and no session. Skipped with a note when tmux is
  missing. `tests/down.sh`
- [ ] down: `make down` with no session is silent and exits 0. `tests/down.sh`
- [ ] version var: `make -n build` contains `-X main.version=` by default and
  `-X example/version.Version=` with `VERSION_VAR=example/version.Version`, in go, go-cli and
  go-svelte. `tests/apply.sh`
- [ ] deploy name: `make -n deploy-setup` in go and go-svelte uses `NAME` by default and
  `DEPLOY_NAME` when set. `tests/apply.sh`
- [ ] skip list: with `skip=` in the target's `.harness`, `apply` seeds none of those paths, a
  second `apply` still doesn't, and `diff` prints nothing, neither for the skipped paths nor for
  the `skip=` line. `tests/apply.sh`
- [ ] skip list: a `skip=` path the stack doesn't seed is harmless (no error, no output).
  `tests/apply.sh`
- [ ] base stack: applying go-svelte-pg seeds go-svelte's frontend files and docs plus
  go-svelte-pg's Makefile. The contract guard and `cinch check` pass on it. `make -n dev`
  includes `dev-db` and `make -n down` includes `stop-db`. The Makefile has no `create-owner`,
  `DB_PATH` or `ENV_FILE`. `tests/apply.sh` and the existing contract loop
- [ ] base stack: a `base.txt` naming a missing stack makes `apply` fail. `tests/apply.sh`
- [ ] settings: `sync` into a repo whose settings has an `env` key and an edited `hooks` key
  restores `hooks` and keeps `env`. A second `sync` changes nothing. A repo without the file gets
  the template's. Invalid JSON fails and leaves the file untouched. `tests/apply.sh`
- [ ] settings: `tests/managed.sh` passes when two stacks' settings differ only outside `hooks`
  and fails when `hooks` differs. `tests/managed.sh`
- [ ] prune: a branch squashed onto `main` as `merge: <b>`, whose file `main` edited later, is
  listed with `(merge <sha>)`. `tests/scripts.sh`
- [ ] prune: a branch named by a `merge:` commit but with a newer commit of its own is not
  listed. `tests/scripts.sh`
- [ ] prune: a branch with no `merge:` commit and a real diff is not listed, and `merge: <b>x`
  does not match `<b>`. `tests/scripts.sh`

## Tasks

T1–T7 are in `~/code/harness-template`, each on its own topic branch, landed with `make merge`.
T8–T16 are one repo each.

### T1: `down` stops the dev processes
**Status:** [ ]
**Depends on:** —
**Files:** `stacks/*/Makefile` (`down`: C-c to every pane, wait up to 5 s, `kill-session`;
android keeps its `force-stop`); new `tests/down.sh`, called from `make check`; README § make
check; `VERSION` → 3.
**Rules:** the `down` invariant
**Verify:** `make check`, with tmux present and once with tmux absent from `PATH` (skip note)

### T2: `VERSION_VAR`
**Status:** [ ]
**Depends on:** T1
**Files:** `stacks/{go,go-cli,go-svelte}/Makefile` (parameter block, every `-X`);
`stacks/go/conventions.md` and the go-svelte backend conventions (the version paragraph names
`VERSION_VAR`); new `tests/apply.sh` with the version-var test, called from `make check`; README
§ Variables or the parameter-block note.
**Rules:** —
**Verify:** `make check`

### T3: repo skip list and `DEPLOY_NAME`
**Status:** [ ]
**Depends on:** T2 (`tests/apply.sh`)
**Files:** `apply.sh` (read `skip=` from the target's `.harness` in seed and diff mode; leave
the `skip=` line out of the `.harness` diff); `stacks/{go,go-svelte}/Makefile` (`DEPLOY_NAME ?=
$(NAME)`, used in every deploy path); `tests/apply.sh`; README § Apply, sync, diff and
§ After applying.
**Rules:** the skip invariant
**Verify:** `make check`

### T4: `go-svelte-pg` and stack inheritance
**Status:** [ ]
**Depends on:** T3 (the go-svelte Makefile it starts from has `DEPLOY_NAME`)
**Files:** `apply.sh` (`base.txt`: copy the base, then overlay; metadata fallback; fail on a
missing base); new `stacks/go-svelte-pg/` (`base.txt`, `Makefile`, `files.txt`), with the
Makefile built from go-svelte's and from ortho's and dd's Postgres blocks; the `make check`
stack list; `tests/apply.sh`; README § Stacks and the apply usage line.
**Rules:** —
**Verify:** `make check`

### T5: `sync` manages only `hooks` in settings
**Status:** [ ]
**Depends on:** T3 (`apply.sh` sync and diff code)
**Files:** `apply.sh` (`--sync` special-cases `.claude/settings.json` through `jq`; refuse
without `jq`; `--diff` compares `.hooks`); `tests/managed.sh`; `tests/apply.sh`; README
§ Managed and seeded (and the `jq` requirement).
**Rules:** the settings invariant
**Verify:** `make check`

### T6: prune counts `merge:` commits
**Status:** [ ]
**Depends on:** T1
**Files:** `common/scripts/prune.sh` (second test, reason in the listing, header comment);
`tests/scripts.sh`; README § Choices (`prune.sh` bullet).
**Rules:** the prune invariant
**Verify:** `make check`; then `scripts/prune.sh` (list only) run with the new script against
verus and forum by hand, with counts noted in the handoff (verus expected about 26)

### T7: README pass and template gaps
**Status:** [ ]
**Depends on:** T1–T6
**Files:** `README.md` (Layout lists `tests/down.sh`, `tests/apply.sh`, `stacks/go-svelte-pg`;
the Gaps section loses what this plan closed). If anything changed in this plan's decisions while
building, write it here too.
**Rules:** —
**Verify:** `make check`; `cinch check` in cinch

### Rollout (T8–T16)

Each rollout task does the same steps in its repo, on a `chore/harness-v3` branch:

1. `make sync REPO=<repo>` from the template, then a second run that changes nothing.
2. Port the seeded changes: the new `down`, and `VERSION_VAR` and `DEPLOY_NAME` where the stack
   has them. Delete the repo's own workaround for the same thing.
3. Set `template=3` in `.harness`, then `make diff REPO=<repo>`. Only the repo's deliberate drift
   should show.
4. `scripts/prune.sh`, then `--confirm` on what it lists. Any leftovers go to the handoff.

**Verify** (each): `make check`, `make test`, `cinch check`. `make dev` and `make down` in a real
terminal are the owner's, listed in the handoff.

### T8: cinch (go-cli)
**Status:** [ ]
**Depends on:** T7
**Files:** rollout steps; `VERSION_VAR` in the Makefile.
**Rules:** —
**Verify:** see Rollout

### T10: forum (go-svelte)
**Status:** [ ]
**Depends on:** T7
**Files:** rollout steps; its `VERSION_VAR` takes the template's form. About 59 branches.
**Rules:** —
**Verify:** see Rollout

### T11: verus (go)
**Status:** [ ]
**Depends on:** T7
**Files:** rollout steps; `VERSION_VAR := verus/version.Version`, replacing the hard-coded `-X`
(the Dockerfile keeps its own `-X`). About 26 branches.
**Rules:** —
**Verify:** see Rollout

### T12: lthy (`~/code/lucky_to_have_you`, go-svelte)
**Status:** [ ]
**Depends on:** T7
**Files:** rollout steps; `skip=` in `.harness` lists `frontend/eslint.config.js`,
`frontend/bunfig.toml` and `frontend/tests/support/*` (each path); its hand-added `DEPLOY_NAME`
block becomes the template's.
**Rules:** —
**Verify:** see Rollout

### T13: ortho (`~/code/ortho-store`, go-svelte → go-svelte-pg)
**Status:** [ ]
**Depends on:** T7
**Files:** rollout steps with `stack=go-svelte-pg` in `.harness`; its TERM-based `down` goes.
About 10 branches.
**Rules:** —
**Verify:** see Rollout; `make test-e2e`

### T14: dd (`~/code/project_deltadocs`, go-svelte → go-svelte-pg)
**Status:** [ ]
**Depends on:** T7
**Files:** as T13. The remote branches on `origin` stay out of scope.
**Rules:** —
**Verify:** see Rollout; `make test-e2e` (2 specs fail on `main` already, plan 33 handoff)

### T15: budgetron (flutter)
**Status:** [ ]
**Depends on:** T7
**Files:** rollout steps; `skip=.docs/stack-conventions.md` in `.harness`. About 16 branches,
including `feat/sms-import` (check its squash).
**Rules:** —
**Verify:** see Rollout

### T16: pugio (android)
**Status:** [ ]
**Depends on:** T7
**Files:** rollout steps. About 28 branches.
**Rules:** —
**Verify:** see Rollout

## Handoff

Planned 2026-11-18: the seven gaps are decided (§ Decisions) and split into T1–T16. Next is T1
in `~/code/harness-template`. Releases and pushes in the rollout repos are the owner's.
