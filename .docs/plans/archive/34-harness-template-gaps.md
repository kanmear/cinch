# 34. Harness-template gaps from the rollout

**Status:** complete

The work happens in `~/code/harness-template` (`origin` is `kanmear/harness_template`, private),
and its rules are in that repo's `README.md`. This plan lives here, like
[33](../33-harness-alignment.md), because cinch is where the harness plans are kept.

## Provenance

[Plan 33](../33-harness-alignment.md) Part C moved twelve repos onto harness-template v2, one
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
   compares `.hooks` for this path, not bytes. *As built (T5):* `diff` never reads managed files, so
   it neither compares `.hooks` nor needs `jq`; only `sync` and `make check` do.
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

- [x] down: in every stack, `make down` on a live tmux session whose pane runs a child that
  ignores SIGHUP leaves no such process and no session. Skipped with a note when tmux is
  missing. `tests/down.sh`
- [x] down: `make down` with no session is silent and exits 0. `tests/down.sh`
- [x] version var: `make -n build` contains `-X main.version=` by default and
  `-X example/version.Version=` with `VERSION_VAR=example/version.Version`, in go, go-cli and
  go-svelte. `tests/apply.sh`
- [x] deploy name: `make -n deploy-setup` in go and go-svelte uses `NAME` by default and
  `DEPLOY_NAME` when set. `tests/apply.sh`
- [x] skip list: with `skip=` in the target's `.harness`, `apply` seeds none of those paths, a
  second `apply` still doesn't, and `diff` prints nothing, neither for the skipped paths nor for
  the `skip=` line. `tests/apply.sh`
- [x] skip list: a `skip=` path the stack doesn't seed is harmless (no error, no output).
  `tests/apply.sh`
- [x] base stack: applying go-svelte-pg seeds go-svelte's frontend files and docs plus
  go-svelte-pg's Makefile. The contract guard and `cinch check` pass on it. `make -n dev`
  includes `dev-db` and `make -n down` includes `stop-db`. The Makefile has no `create-owner`,
  `DB_PATH` or `ENV_FILE`. `tests/apply.sh` and the existing contract loop
- [x] base stack: a `base.txt` naming a missing stack makes `apply` fail. `tests/apply.sh`
- [x] settings: `sync` into a repo whose settings has an `env` key and an edited `hooks` key
  restores `hooks` and keeps `env`. A second `sync` changes nothing. A repo without the file gets
  the template's. Invalid JSON fails and leaves the file untouched. `tests/apply.sh`
- [x] settings: `tests/managed.sh` passes when two stacks' settings differ only outside `hooks`
  and fails when `hooks` differs. `tests/managed.sh`
- [x] prune: a branch squashed onto `main` as `merge: <b>`, whose file `main` edited later, is
  listed with `(merge <sha>)`. `tests/scripts.sh`
- [x] prune: a branch named by a `merge:` commit but with a newer commit of its own is not
  listed. `tests/scripts.sh`
- [x] prune: a branch with no `merge:` commit and a real diff is not listed, and `merge: <b>x`
  does not match `<b>`. `tests/scripts.sh`

## Tasks

T1–T7 are in `~/code/harness-template`, each on its own topic branch, landed with `make merge`.
T8–T16 are one repo each.

### T1: `down` stops the dev processes
**Status:** [x]
**Depends on:** —
**Files:** `stacks/*/Makefile` (`down`: C-c to every pane, wait up to 5 s, `kill-session`;
android keeps its `force-stop`); new `tests/down.sh`, called from `make check`; README § make
check; `VERSION` → 3.
**Rules:** the `down` invariant
**Verify:** `make check`, with tmux present and once with tmux absent from `PATH` (skip note)

### T2: `VERSION_VAR`
**Status:** [x]
**Depends on:** T1
**Files:** `stacks/{go,go-cli,go-svelte}/Makefile` (parameter block, every `-X`);
`stacks/go/conventions.md` and the go-svelte backend conventions (the version paragraph names
`VERSION_VAR`); new `tests/apply.sh` with the version-var test, called from `make check`; README
§ Variables or the parameter-block note.
**Rules:** —
**Verify:** `make check`

### T3: repo skip list and `DEPLOY_NAME`
**Status:** [x]
**Depends on:** T2 (`tests/apply.sh`)
**Files:** `apply.sh` (read `skip=` from the target's `.harness` in seed and diff mode; leave
the `skip=` line out of the `.harness` diff); `stacks/{go,go-svelte}/Makefile` (`DEPLOY_NAME ?=
$(NAME)`, used in every deploy path); `tests/apply.sh`; README § Apply, sync, diff and
§ After applying.
**Rules:** the skip invariant
**Verify:** `make check`

### T4: `go-svelte-pg` and stack inheritance
**Status:** [x]
**Depends on:** T3 (the go-svelte Makefile it starts from has `DEPLOY_NAME`)
**Files:** `apply.sh` (`base.txt`: copy the base, then overlay; metadata fallback; fail on a
missing base); new `stacks/go-svelte-pg/` (`base.txt`, `Makefile`, `files.txt`), with the
Makefile built from go-svelte's and from ortho's and dd's Postgres blocks; the `make check`
stack list; `tests/apply.sh`; README § Stacks and the apply usage line.
**Rules:** —
**Verify:** `make check`

### T5: `sync` manages only `hooks` in settings
**Status:** [x]
**Depends on:** T3 (`apply.sh` sync and diff code)
**Files:** `apply.sh` (`--sync` special-cases `.claude/settings.json` through `jq`; refuse
without `jq`; `--diff` compares `.hooks`); `tests/managed.sh`; `tests/apply.sh`; README
§ Managed and seeded (and the `jq` requirement).
**Rules:** the settings invariant
**Verify:** `make check`

### T6: prune counts `merge:` commits
**Status:** [x]
**Depends on:** T1
**Files:** `common/scripts/prune.sh` (second test, reason in the listing, header comment);
`tests/scripts.sh`; README § Choices (`prune.sh` bullet).
**Rules:** the prune invariant
**Verify:** `make check`; then `scripts/prune.sh` (list only) run with the new script against
verus and forum by hand, with counts noted in the handoff (verus expected about 26)

### T7: README pass and template gaps
**Status:** [x]
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
**Status:** [x]
**Depends on:** T7
**Files:** rollout steps; `VERSION_VAR` in the Makefile.
**Rules:** —
**Verify:** see Rollout

### T10: forum (go-svelte)
**Status:** [x]
**Depends on:** T7
**Files:** rollout steps; its `VERSION_VAR` takes the template's form. About 59 branches.
**Rules:** —
**Verify:** see Rollout

### T11: verus (go)
**Status:** [x]
**Depends on:** T7
**Files:** rollout steps; `VERSION_VAR := verus/version.Version`, replacing the hard-coded `-X`
(the Dockerfile keeps its own `-X`). About 26 branches.
**Rules:** —
**Verify:** see Rollout

### T12: lthy (`~/code/lucky_to_have_you`, go-svelte)
**Status:** [x]
**Depends on:** T7
**Files:** rollout steps; `skip=` in `.harness` lists `frontend/eslint.config.js`,
`frontend/bunfig.toml` and `frontend/tests/support/*` (each path); its hand-added `DEPLOY_NAME`
block becomes the template's.
**Rules:** —
**Verify:** see Rollout

### T13: ortho (`~/code/ortho-store`, go-svelte → go-svelte-pg)
**Status:** [x]
**Depends on:** T7
**Files:** rollout steps with `stack=go-svelte-pg` in `.harness`; its TERM-based `down` goes.
About 10 branches.
**Rules:** —
**Verify:** see Rollout; `make test-e2e`

### T14: dd (`~/code/project_deltadocs`, go-svelte → go-svelte-pg)
**Status:** [x]
**Depends on:** T7
**Files:** as T13. The remote branches on `origin` stay out of scope.
**Rules:** —
**Verify:** see Rollout; `make test-e2e` (2 specs fail on `main` already, plan 33 handoff)

### T15: budgetron (flutter)
**Status:** [x]
**Depends on:** T7
**Files:** rollout steps; `skip=.docs/stack-conventions.md` in `.harness`. About 16 branches,
including `feat/sms-import` (check its squash).
**Rules:** —
**Verify:** see Rollout

### T16: pugio (android)
**Status:** [x]
**Depends on:** T7
**Files:** rollout steps. About 28 branches.
**Rules:** —
**Verify:** see Rollout

## Handoff

Planned 2026-11-18: the seven gaps are decided (§ Decisions) and split into T1–T16. Releases and
pushes in the rollout repos are the owner's.

**T1 done 2026-11-18**, landed on harness-template `main` as `deda20d` (not pushed). Every
stack's `down` sends C-c to each pane, waits up to 5 s for the session to end by itself, then
kills it. android keeps its `force-stop`. `VERSION` is 3. New `tests/down.sh`, called from
`make check`, runs on a private tmux server (`TMUX_TMPDIR`). The no-session case is silent in
every stack. With a live two-pane session running SIGHUP-proof processes, `down` leaves neither
the session nor the processes. With the old Makefiles the test fails.

One addition beyond the decision: `down` names its session with tmux's exact `=` form. Without
it, `has-session demo-dev-` (PORT_BASE unset) matches `demo-dev-5000` as a prefix. The new wait
loop would then wait out the 5 s and kill the wrong session, and the old `down` had the same bug
on its single call. `tests/down.sh` keeps such a sibling session and checks that it survives. The
old `down` fails this case too. `dev` still uses the prefix form. Verified: `make check` passes
with tmux present (20 checks in `tests/down.sh`) and with tmux absent from PATH (5 checks, skip
note). The 5 s fallback was checked by hand with a pane that ignores SIGINT too (5.2 s, session
killed, sibling kept). harness-template has no `make merge` target, so T1 was committed on its
topic branch and fast-forwarded onto `main`, as plan 33's template commits were. T2–T7 land the
same way. Next is T2 (`VERSION_VAR`).

**T2 done 2026-11-18**, landed on harness-template `main` as `748b20e` (not pushed).
go, go-cli and go-svelte have `VERSION_VAR ?= main.version` in the parameter block, and every
`-X` is `-X $(VERSION_VAR)=$(VERSION)`. The go and go-cli conventions name the variable, and the
go-svelte backend conventions gain a § Version. goreleaser can't read make variables, so the
go-cli `.goreleaser.yml` keeps `-X main.version` and its header says to change it together with
`VERSION_VAR`. The README says the same of a repo's Dockerfile. New `tests/apply.sh`, called
from `make check`, holds the version-var test (6 checks), with one section per task for T3–T5.
`VERSION` stays 3. Verified: `make check` passes, and with go-svelte's `-X` reverted to
`main.version` it fails in `tests/apply.sh`. Next is T3 (skip list and `DEPLOY_NAME`).

**T3 built 2026-11-18** on harness-template branch `feature/skip-deploy-name`, not committed yet.
`apply.sh` reads a `skip=` line from the target's `.harness`: target paths, space-separated,
exact, split without globbing. `render_one` drops a listed destination path before staging, so
`apply` neither seeds, prints nor counts it, and a common file of the same path can't take its
place. `--sync` treats a listed managed path like a `skip.txt` one (counted as `skipped`).
`--diff` leaves out the listed paths and the `skip=` line. Two additions beyond the decision:
`--force` re-appends the repo's `skip=` line to the rewritten `.harness`, so a forced apply
doesn't lose the list, and `sync` honours `skip=`, not only `apply` and `diff`. `skip=` is read
from the target, so on a new repo it takes effect from the second `apply` (README says so). go and
go-svelte have `DEPLOY_NAME ?= $(NAME)`, and every `~/apps/`, `~/git/`, `lab` remote and
`backups/` path uses it. The container binary, `bin/`, `data/`, `-drill` and the dev session keep
`NAME`, as in lthy. `tests/apply.sh` has 20 checks, up from 6: deploy-setup and backup-pull with
and without `DEPLOY_NAME`, a grep that no deploy path in the applied Makefile names `NAME`, and
the skip list through apply, a second apply, diff, `--force` and sync. go-svelte seeds no
`.docs/stack-conventions.md`, so the remapped path in the skip test is
`.docs/backend/conventions.md`. README § Apply, sync, diff, § After applying (new step 5) and
§ make check (seven guards) say so. Verified: `make check` passes. Each of these makes it fail in
`tests/apply.sh`: removing the `render_one` skip, reverting go-svelte's `deploy-setup` to
`$(NAME)`, or dropping the `--force` re-append. Proposed commit (harness-template):
`feat [harness]: repo skip list in .harness and DEPLOY_NAME in go stacks (plan 34 T3)`, then
fast-forward onto `main` as T1–T2. Next is T4 (`go-svelte-pg`).

**T4 done 2026-11-18**, landed on harness-template `main` as `3d5c58e` (not pushed). `apply.sh` reads `stacks/<stack>/base.txt`: one stack name.
Render order is the stack, then its base, then common, and the first writer of a path wins.
`vars.txt`, `skip.txt` and `managed.txt` fall back to the base's as whole files, not merged.
`files.txt` is read against the stack's own directory, and `base.txt` is never copied.
apply, sync and diff all exit 2 before writing anything when `base.txt` is empty, names more than
one stack, names a missing stack or a bad name, names the stack itself, or names a base that has
a `base.txt` of its own (one level only). `tests/managed.sh` uses the same fallback.

The new `stacks/go-svelte-pg/` holds `base.txt` (`go-svelte`), `files.txt`, a `Makefile` and a
`docker-compose.dev-db.yml` with `{{NAME}}` placeholders. The compose file goes beyond the
plan's file list, because `dev-db` needs it. Its volume follows `<name>`, not ortho's or dd's
pinned project names. Choices made while building:
- `DB_PORT` is `PORT_BASE + 2`, as in go-svelte and `ports.md`.
- `dev: require-port-base dev-db`.
- `down: stop-dev stop-db`. go-svelte's T1 `down` body became `stop-dev`, so the session stops
  before the database (ortho stops the database first).
- `stop-db` keeps ortho's paused-container logic. Without docker, `make down` fails at `stop-db`,
  as it does in ortho.
- One shared stub recipe prints "not designed yet" and exits 2 for `deploy`, `logs`,
  `deploy-setup`, `rollback`, `restore-drill`, `prod-backup` and `backup-pull`.
- Dropped targets: `backup`, `create-owner`, `reset-password`, `prod-create-owner`,
  `prod-reset-password`, `prod-logs` and `require-deploy`. Dropped parameters: every deploy
  parameter, including `DEPLOY_NAME`, plus `DB_PATH` and `ENV_FILE`.
- `ship` is release only, as in ortho. go-svelte's ship runs the SQLite restore drill and then
  deploys.
- `install` copies the `.env` examples.
- `test-backend` creates `<name>_test`, runs with `-p 1`, and skips the create when
  `TEST_DB_PREPARED=1`.
- goose is v3.28.0.
- `dev-backend` stays `go run . serve`, not Air.

No go-svelte doc contradicts Postgres, so none changed. `backend-conventions.md` still leans on
SQLite with a Postgres note, and go-svelte's Makefile still ends with its commented-out
"Postgres (off by default)" block. T7 should delete that block or point it at go-svelte-pg.

Tests:
- `tests/apply.sh` has 39 checks. They cover the files inherited from the base, the stack's own
  Makefile and compose file, the vars fallback, the absent `create-owner`, `DB_PATH` and
  `ENV_FILE`, `dev` and `down` under `make -n`, the deploy and logs stubs exiting 2, and the six
  broken `base.txt` cases plus a control. The broken cases run in a scratch copy of the template.
- `tests/down.sh` runs every `down` with a `DB_COMPOSE` shim, so it never needs docker. It checks
  that a stack with `stop-db` calls the shim with `down` and that one without it doesn't.

The README covers § Stacks, the usage lines, § Layout and § make check. The new "Base stack"
guard makes eight guards in § make check. The template Makefile's usage lists `go-svelte-pg`.

Verified: `make check` passes. Each of these makes it fail and was reverted:
- dropping the base render pass
- letting a missing base pass
- dropping `dev-db` from `dev`
- dropping `stop-db` from `down`

Neither `make dev` nor `make down` has been run against a real docker daemon. That happens in
T13 and T14. Proposed commit (harness-template):
`feat [harness]: go-svelte-pg stack and stack inheritance through base.txt (plan 34 T4)`, then
fast-forward onto `main` as T1–T3. Next is T5 (`sync` manages only `hooks` in settings).

**T5 built 2026-11-18** on harness-template branch `feature/sync-settings-hooks`, not committed
yet. `apply.sh --sync` replaces only `.hooks` of `.claude/settings.json`
(`jq --slurpfile t <template> '.hooks = $t[0].hooks'`) and keeps every other key. A repo without
the file gets the template's copy. Before writing any managed file, sync exits 2 when `jq` is
missing or when the repo's file is not exactly one JSON object, so a bad file leaves no
half-synced repo. `tests/managed.sh` compares `jq -cS .hooks` for the settings file and bytes for
every other path. `make check` fails first thing without `jq`. README § Managed and seeded states
the invariant and the `jq` requirement, and § make check gains a ninth guard ("Settings").
Choices beyond the decision:
- "Unchanged" is a semantic compare (`jq -S .` on both sides), so a hand-formatted or
  key-reordered file with equal content is neither rewritten nor reported.
- `jq` is needed only when the settings file is synced. A repo whose `skip=` lists it syncs
  without `jq`.
- Empty, `[]`, `null` and two concatenated values fail too, not only syntax errors. The
  template's own settings must hold a `hooks` object.
- `--diff` is unchanged. It skips managed paths, so it never reads the settings file and needs
  no `jq`. Decision 6's "dependency of `diff`" and T5's "`--diff` compares `.hooks`" don't hold.
  T7 can correct the wording.

`tests/apply.sh` has 46 checks, up from 39: a sync that restores edited `hooks` and keeps `env`
and `permissions`; a second sync, and a sync of a compact reordered copy, that change nothing;
a repo without the file; five invalid files that fail with exit 2 and leave every managed file
untouched; a PATH without `jq`; and `tests/managed.sh` passing when stacks differ only outside
`hooks` and failing when `hooks` differs. Verified: `make check` passes. Each of these makes it
fail and was reverted: a whole-file overwrite in place of the merge, and a byte compare of the
settings file in `managed.sh`. Proposed commit (harness-template):
`feat [harness]: sync manages only the hooks key of settings.json (plan 34 T5)`, then
fast-forward onto `main` as T1–T4. Next is T6 (prune counts `merge:` commits).

**T6 done 2026-11-19**, landed on harness-template `main` as `5b35083` (not pushed).
`prune.sh` tries the no-diff test first, then the merge-commit test: `main` has a commit
whose whole subject is `merge: <b>` and the branch tip's committer time is not later than that
commit's. One `git log refs/heads/main` pass collects the `merge:` commits. An `awk` compares the
subject as a fixed string (the branch name comes in through `ENVIRON`), so `merge: <b>x` doesn't
match `<b>`. With several `merge: <b>` commits, the newest decides. The listing prints
`landed  <b>  (no diff)` or `landed  <b>  (merge <short-sha>)`, and `--confirm` is unchanged.
One choice beyond the decision: branches are read with `%(refname:lstrip=2)`, not
`refname:short`, so a tag of the same name can't turn the branch into `heads/<b>`. The header
comment and README § Choices describe both tests.

`tests/scripts.sh` has 121 checks. New `docs/` squashes leave VERSION at 0.2.4, and the test
asserts that:
- a squash that `main` edited later is listed with its merge sha and deleted by `--confirm`;
- a branch squashed twice is listed with the newest merge sha;
- a reused branch with a newer commit (`GIT_COMMITTER_DATE` +100 s) is neither listed nor deleted;
- `docs/long` is not matched by `merge: docs/longx`;
- `feature/x` shows `(no diff)`;
- `fix/w` and `fix/conf` stay unlisted.

Verified: `make check` passes. Each of these mutations makes it fail and was reverted: dropping
the second test, dropping the time condition, a prefix match, oldest-wins, and changing the
`(no diff)` text. A list-only run by hand, nothing deleted:

| Repo | Local branches | Old script | New script | `(merge …)` |
|---|---|---|---|---|
| verus | 26 | 0 | 25 | 25 |
| forum | 65 | 0 | 64 | 64 |

The only branch left unlisted in each repo is `main`. verus matches the expected count of about
26. The old script finds 0 now, not the 4 noted in Gaps item 7, so those 4 were probably pruned
since. harness-template also still has the landed T3–T5 topic branches (`feature/skip-deploy-name`,
`feature/go-svelte-pg`, `feature/sync-settings-hooks`). Next is T7 (README pass and template gaps).

**T7 done 2026-11-19**, landed on harness-template `main` as `af9d641` (not pushed).
T1–T6 had already written their README sections, Layout included, and the README's Gaps never
listed the seven items, so the pass came down to three stale spots:
- go-svelte's Makefile loses the commented "Postgres (off by default)" block (64 lines, the
  older `ENV_FILE` form). A one-line pointer to go-svelte-pg replaces it, and the `DB_PORT`
  comment no longer names the block. README § Choices (the go-svelte-pg and Deploy bullets)
  says so. forum and lthy will see the deletion in `make diff` during T10 and T12 and take it.
- README § Gaps: the "no GitHub remote, branches stacked off `main`" bullet is gone (`origin`
  exists and T1–T6 are on `main`). It now says the template has no `make merge` and pushing is
  the owner's. The compose bullet notes go-svelte-pg's dev compose file. Two bullets are new:
  go-svelte-pg has no deploy, and its `make dev` / `make down` haven't been run against docker.
- Decision 6 gains an as-built note: `diff` needs no `jq`.

The other choices T1–T6 made beyond the decisions (`=` session names, `--force` keeping
`skip=`, `sync` honouring `skip=`, one-level `base.txt`) are in the README already.
Verified: `make check` in harness-template passes; `cinch check` in cinch passes.

`main` there is 7 commits ahead of `origin/main` (T1–T7); pushing is the owner's. Landed topic
branches still in harness-template: `feature/skip-deploy-name`, `feature/go-svelte-pg`,
`feature/sync-settings-hooks`, `feature/prune-merge-commits`, `docs/readme-pass-t7`. Next is T8 (cinch rollout).

**T8 done 2026-11-19** on cinch branch `chore/harness-v3`, not committed yet. `make sync` updated
9 managed files (the v3 stamp everywhere, and the new `prune.sh`); `settings.json` was already
equal. A second sync changed nothing. `.harness` says `template=3`. Ported by hand:
- Makefile: the template's `down` (C-c to each pane, 5 s wait, `=` session name) and help line.
  `VERSION_VAR ?= main.Version` in the parameter block, since cinch's variable is the exported `var
  Version`, and `LDFLAGS` uses it.
- `.docs/stack-conventions.md` § Release names `VERSION_VAR` (`main.Version` here).
  `.goreleaser.yml` gets a one-line comment that its `-X` must match `VERSION_VAR`.
- The Makefile and stack conventions carry the v3 stamp. The other seeded files keep v2,
  because nothing in them changed in v3.

cinch had no workaround of its own for either gap.

Prune listed 47 branches (3 `(no diff)`, 44 `(merge …)`), and `--confirm` deleted them. Three
are left for the owner:
- `docs/plan-33-t6-managed-sync` and `docs/plan-33-t7-managed-scripts`: these were stacked,
  and both are ancestors of `docs/plan-33-t8-make-contract`, which landed as `e9a692b`. No
  `merge:` commit names them, so prune can't see them. Safe to `git branch -D`.
- `backup/pre-rewrite`: kept on purpose.

Verified: `make check`, `make test` and `cinch check` pass. `make build` stamps the version
(`bin/cinch version` prints 0.18.0). `make down` with no session is silent. Owner: `make down` on a
live session in a real terminal (cinch's `dev` is foreground, so this only matters for a helper
session). The hooks exec the installed `cinch`, and nothing here changes it. Proposed commit
(cinch): `chore [harness]: harness-template v3, VERSION_VAR and the new down (plan 34 T8)`.

`make sync` updated 9 managed files (the v3 stamp and the new `prune.sh`); `settings.json` was
already equal. A second sync changed nothing. `.harness` says `template=3`. Ported by hand:
- Makefile: the template's `down` (C-c to each pane, 5 s wait, `=` session name), its comment and
  help line.
- `.docs/stack-conventions.md` § Release names `VERSION_VAR`. The Makefile and stack conventions
  carry the v3 stamp; the other seeded files keep v2.
- `HERDR_ENV=0` moved into the `env` block of `.claude/settings.json`, and the gitignored
  `.claude/settings.local.json` (it held only that) is deleted. `.docs/architecture.md`'s
  `HERDR_ENV` row says the setting is committed and survives a sync, since sync manages only
  `hooks`. A third sync after the move left `env` in place.

Prune listed 25 of 37 branches, all `(merge …)`, and `--confirm` deleted them. Eleven are left
for the owner:
- Stacked and never named by a `merge:` commit: `feature/l0-scaffold`, `-llm`, `-tool`,
  `-session` and `-agent`, each an ancestor of the next (L0 landed as `merge: feature/l0-print`,
  `ab0ddd1`); `feature/l1-settings`, an ancestor of `docs/l2-plan`; `docs/l2-plan`,
  `docs/l3-plan` and `docs/native-search-plan` (L1–L3 and the native tools landed under other
  branch names). Likely safe to `git branch -D`, but check that each is in what landed.
- Probably unlanded work: `feature/ux-conveniences` has one commit (`c6eaae1`, `/compact` takes
  a focus, CORE-017) made 20 minutes after its `merge:` commit `2ccf6a4`; `main` has no
  `focus` in `agent/compact.go` or `tui/commands.go`. `ui/calm-colours` has one commit (quieter
  thinking label, tool targets and inline code) with no `merge:` commit. Land or drop each.

Verified: `make check`, `make test` (no failures) and `cinch check` pass. `make down` with no session is
silent and exits 0. This handoff sits uncommitted on cinch's `chore/harness-v3`, whose T8 commit isn't landed
yet. Next is T10 (forum).

**T10 done 2026-11-19** on forum branch `chore/harness-v3`, not committed yet. `make sync`
updated 9 managed files (the v3 stamp and the new `prune.sh`); `settings.json` was already equal.
A second sync changed nothing. `.harness` says `template=3`. Ported by hand:
- Makefile: the template's `down` (C-c to each pane, 5 s wait, `=` session name), its comment
  and help line. forum's own `VERSION_VAR := forum_backend/version.Version`, which sat below the
  parameter block, becomes the template's `VERSION_VAR ?=` in the block, with its comment. The
  Dockerfile's `-X forum_backend/version.Version` already matches.
- `DEPLOY_NAME ?= $(NAME)` in the parameter block, used in every server path (`~/apps/`,
  `~/git/`, the `lab` remote) and backup directory name. That includes forum's extra
  backup flow: `BACKUP_STAMP`, the stamp check, the `grep` patterns in `BACKUP_PULL_CMD`,
  `DEPLOY_PULL_CMD` and `migrate-check`, `prod-restore` and the `prod-*` verbs. `/app/$(NAME)`,
  `bin/`, `data/$(NAME).db` and the dev session keep `NAME`. `make -n deploy-setup` names
  `forum` by default, and with `DEPLOY_NAME=x` names only `x`.
- forum never had the commented Postgres block, so T7's deletion was a no-op here; only the
  `DB_PORT` comment changed.
- `.docs/backend/conventions.md` gains a § Version naming `VERSION_VAR`, as the template's
  go-svelte backend conventions do. The Makefile carries the v3 stamp; the other seeded files
  keep v2.

`make diff` shows only forum's deliberate drift: its parameter values and extra targets
(`install-agent`, `check-error-codes`, tokens, seed and reindex, `prod-restore`, `verify`,
`migrate-check`), the richer `backup-pull`, no prettier or eslint gate (the template's
`frontend/.prettierignore`, `.prettierrc` and `eslint.config.js` don't exist here), the realtime
test's `app-navigation.ts`, `.dockerignore` adding `bin`, `cinch.yml`'s per-area
entries, `VERSION`, and its own docs.

Prune listed 64 branches, all `(merge …)`, and `--confirm` deleted them. No local branch is left
but `main` and `chore/harness-v3`. The old `origin/*` remote branches are outside prune and stay
for the owner.

Verified: `make check`, `make test` and `cinch check` pass. `make build` stamps the version
(`-X forum_backend/version.Version=0.41.0`, and the binary holds `0.41.0`). `make down` with no
session is silent and exits 0. One `make test` run in the build session failed on
`TestFreedesktopNotifierReplacesByIdAndStopsWaitingOnTheOldProcess` (`tests/client`,
freedesktop_test.go:159); four reruns and the verify run passed, so it is a timing flake
unrelated to this change, worth its own fix. Owner: `make down` on a live `make dev` session in a
real terminal. Proposed commit (forum):
`chore [harness]: harness-template v3, VERSION_VAR, DEPLOY_NAME and the new down (plan 34 T10)`.
Next is T11 (verus).

**T11 done 2026-11-19** on verus branch `chore/harness-v3`, not committed yet. `make sync`
updated 9 managed files (the v3 stamp and the new `prune.sh`); `settings.json` was already equal.
A second sync changed nothing. `.harness` says `template=3`. Ported by hand:
- Makefile: the template's `down` (C-c to each pane, 5 s wait, `=` session name), its comment
  and help line. `VERSION_VAR ?= verus/version.Version` in the parameter block, and both
  hard-coded `-X verus/version.Version` (`build` and `dev`) use it. The Dockerfile keeps its own
  `-X verus/version.Version`, which already matches; the parameter comment says to keep them in step.
- `DEPLOY_NAME ?= $(NAME)` in the parameter block, used in every server path: `~/apps/` in
  `PROD_BACKUP_CMD`, `deploy-setup`, `LAB_REBUILD_CMD` and `logs`, `~/git/`, and the `lab`
  remote. verus has no `backup-pull`, so there is no `backups/<name>-*` path. `/app/$(NAME)`,
  `bin/`, `data/$(NAME).db` and the dev session keep `NAME`. `make -n deploy-setup` names
  `verus` by default, and with `DEPLOY_NAME=x` names only `x`.
- `.docs/stack-conventions.md` § Version names `VERSION_VAR` and carries the v3 stamp, like the
  Makefile; the other seeded files keep v2. `.docs/git-conventions.md` § Version says `make build`
  and `make dev` stamp through `VERSION_VAR`.

`make diff` shows only verus's deliberate drift: its parameter values (`PORT_BASE` 4120,
`HOST_PORT` 3300, `COMPOSE_SERVICE` verus), the extras (`prompts-lock` and `prompts check` in
`check`, `contract-pull`, `tunnel`, `prod-logs`), `START_CURSOR` in `dev`, no `rollback`,
`restore-drill` or `backup-pull` (so no `MIGRATIONS_DIR` or `VERIFY_SUBCOMMAND`), `ship` as
release then deploy, and its own docs, `VERSION` and `cinch.yml`.

Prune listed 25 branches, all `(merge …)`, as T6's dry run did, and `--confirm` deleted them. No
local branch is left but `main` and `chore/harness-v3`.

Verified: `make check`, `make test` (12 packages ok) and `cinch check` pass. `make build` stamps
the version (`-X verus/version.Version=0.15.0`, and the binary holds `0.15.0`). `make down` with
no session is silent and exits 0. Owner: `make down` on a live helper session in a real terminal
(verus's `dev` is foreground). Proposed commit (verus):
`chore [harness]: harness-template v3, VERSION_VAR, DEPLOY_NAME and the new down (plan 34 T11)`.
This handoff sits uncommitted on cinch's `docs/plan-34-t10`, whose T10 handoff commit isn't
landed yet. Next is T12 (lthy).

**T12 done 2026-11-19** on lthy branch `chore/harness-v3`, not committed yet. `make sync`
updated 9 managed files (the v3 stamp and the new `prune.sh`); `settings.json` was already equal.
A second sync changed nothing. `.harness` says `template=3`. Ported by hand:
- Makefile: the template's `down` (C-c to each pane, 5 s wait, `=` session name), its comment,
  help line and the `dev` comment that says "stops". lthy's own `VERSION_VAR := us_backend/version.Version`,
  which sat below the parameter block, becomes the template's `VERSION_VAR ?=` in the block, with
  its comment. The Dockerfile's `-X us_backend/version.Version` already matches. The Makefile
  header says v3.
- `DEPLOY_NAME ?= lthy`, the hand-added block, now has the template's comment and placement,
  plus one line saying why lthy sets it; the value stays `lthy` because the binary and `NAME` are
  `us`. Every deploy path (`~/apps/`, `~/git/`,
  the `lab` remote, `backups/`) already used it. `/app/$(NAME)`, `bin/`, `data/$(NAME).db` and
  the dev session keep `NAME`. `make -n deploy-setup` names `lthy` by default, and with
  `DEPLOY_NAME=x` names only `x` (no `lthy`); `make -n backup-pull` names `backups/lthy-*` and
  `backups/x-*`.
- lthy never had the commented Postgres block, so T7's deletion was a no-op; its own `DB_PORT`
  comment ("lthy has none yet") stays, and it doesn't take the one-line pointer to go-svelte-pg
  that the template's Makefile ends with.
- `.docs/backend/conventions.md` gains a § Version naming `VERSION_VAR` (`us_backend/version.Version`,
  `backend/version/version.go`), as the template's go-svelte backend conventions do. That doc
  never carried a stamp, so nothing there to bump; the other seeded files keep v2.
- `skip=` in `.harness` lists `frontend/eslint.config.js`, `frontend/bunfig.toml` and all four
  files the stack seeds under `frontend/tests/support/`: `app-environment.ts`, `app-navigation.ts`,
  `app-state.ts` and `svelte-plugin.ts` (the template's `files.txt` names only the plugin, but the
  stack directory holds the other three). None of them exists in lthy. `make diff` shows nothing
  for them or for the `skip=` line, and a `make sync` after adding it changed nothing and created none.

`make diff` shows only lthy's deliberate drift (16 files): its parameter values (`PORT_BASE` 4100,
`HOST_PORT` 3100, `PUBLIC_URL`, `DEPLOY_NAME` lthy, empty `MIGRATIONS_DIR` and `VERIFY_SUBCOMMAND`),
the extras (`check-error-codes`, `icons`, `reset-dev-data`, `seed`, `fresh-dev-data`, `UPLOADS_DIR`),
its help lines, `ship` as restore-drill, release, deploy, its own `.prettierrc` and `.prettierignore`
(no tailwind plugin, width 110), `.dockerignore` adding `bin`, 
`VERSION`, and its own docs.

Prune listed 52 branches, all `(merge …)` (none `(no diff)`), and `--confirm` deleted them. No
local branch is left but `main` and `chore/harness-v3`, so no leftovers.

Verified: `make check` (svelte-check 0 errors, go vet, error codes in sync), `make test` (frontend
and backend, every package ok) and `cinch check` pass. `make build` stamps the version
(`-X us_backend/version.Version=0.63.0`, and the binary holds `0.63.0`). `make down` with no session
is silent and exits 0. Owner: `make down` on a live `make dev` session in a real terminal.
Proposed commit (lthy):
`chore [harness]: harness-template v3, VERSION_VAR, DEPLOY_NAME, skip list and the new down (plan 34 T12)`.
Next is T13 (ortho).

**T13 done 2026-11-19** on ortho branch `chore/harness-v3`, not committed yet. `.harness` says
`stack=go-svelte-pg` and `template=3`, set before the sync. `make sync` updated 9 managed files
(the v3 stamp and the new `prune.sh`); `settings.json` was already equal. A second sync changed
nothing. go-svelte-pg falls back to go-svelte's `managed.txt`, so the stack switch changed no
managed path. Ported by hand:
- Makefile: the TERM-based `down` goes. The template's `stop-dev` (C-c to each pane, 5 s wait,
  `=` session name) takes its place, and `down: stop-dev stop-db` stops the session before the
  database (ortho stopped the database first). `dev-db` moves into `dev`'s prerequisites, as in
  the template, and the trailing `dev: dev-db` line goes. The header names go-svelte-pg and v3.
- `VERSION_VAR ?= ortho_backend/version.Version` in the parameter block, and `build-backend` uses
  it. `backend/Dockerfile` keeps its own `-X ortho_backend/version.Version`, which matches; the
  parameter comment says to keep them in step.
- `deploy` and `logs` are now stubs that exit 2 and are listed in `make help`, so ortho has every
  make-contract verb. It doesn't take the template's other five deploy stubs (`deploy-setup`,
  `rollback`, `restore-drill`, `prod-backup`, `backup-pull`). Those are go-svelte's SQLite verbs,
  and ortho never had them. `ship` keeps ortho's wording (not deployed, see `.docs/decisions.md`).
- `docker-compose.dev-db.yml` gets the v3 stamp and keeps its pinned `name: ortho_backend` (the
  old volume) and `${DB_PORT:-4132}` fallback.
- `.docs/backend/conventions.md` gains a § Version naming `VERSION_VAR`, as forum and lthy did.
  `.docs/git-conventions.md` § Version says `make build` stamps through `VERSION_VAR`. Neither
  doc's stamp changes.

`make diff` shows only ortho's deliberate drift, 22 files: `PORT_BASE` 4130, Air in
`dev-backend` and the frontend's env in `dev-frontend`, the split `install-*` and `build-*`
targets, `check-error-codes` and `check-placeholders`, `bun run` check, lint, test and format,
`test-e2e` and `install-e2e`, the seeding and `create-staff` targets, no `preview` or
`BUILD_NOTE`, the two deploy stubs above, the compose file's pinned project name, its own docs,
`VERSION` and `cinch.yml`. The go-svelte frontend files ortho doesn't have (`bunfig.toml`,
`tests/support/*`, `.dockerignore`) show as "only in template". ortho has no `skip=` line, as
T13 didn't ask for one; lthy-style skips would quiet them.

Prune listed 11 branches (10 `(merge …)`, `fix/ci-install-dir` `(no diff)`), and `--confirm`
deleted them. No local branch is left but `main` and `chore/harness-v3`.

Verified: `make check`, `make test` (21 frontend files, 119 tests; backend packages ok),
`cinch check` and `make test-e2e` (5 passed) pass. `make build-backend` stamps the version (the
binary holds `0.13.0`). `make -n dev` runs `dev-db` first, and `make -n down` runs `stop-dev`, then
`stop-db`. `make deploy` exits 2 with its message. `make down` with no session printed no session
line. It did stop the `ortho_db` container, which was running; `make dev-db` brought it back with
its volume. Owner: `make dev` and `make down` on a live session in a real terminal, this time
against a real docker daemon (T4 left that open for go-svelte-pg). Proposed commit (ortho):
`chore [harness]: go-svelte-pg stack, harness-template v3, VERSION_VAR and the new down (plan 34 T13)`.
Next is T14 (dd).

**T14 done 2026-11-19** on dd branch `chore/harness-v3`, not committed yet. `.harness` says
`stack=go-svelte-pg` and `template=3`, set before the sync. `make sync` updated 9 managed files
(the v3 stamp and the new `prune.sh`). `settings.json` was already equal, and a second sync changed
nothing. The hand port matches T13's in ortho:
- Makefile: the TERM-based `down` goes. The template's `stop-dev` takes its place, and
  `down: stop-dev stop-db`. `dev-db` moves into `dev`'s prerequisites, and the trailing
  `down: stop-db` and `dev: dev-db` lines go. The header names go-svelte-pg and v3.
- `VERSION_VAR ?= deltadocs_backend/version.Version`, used by `build-backend`.
  `backend/Dockerfile` keeps its own matching `-X` from `APP_VERSION`, and the parameter comment
  says to keep the two in step.
- `deploy` and `logs` are stubs that exit 2, listed in `make help`. As in ortho, the five
  SQLite deploy stubs are left out, and `ship` keeps dd's wording ("not deployed").
- `docker-compose.dev-db.yml` gets the v3 stamp. It keeps `name: backend`, so the old
  `backend_postgres_data` volume stays in use, and keeps the `${DB_PORT:-4142}` fallback.
- `.docs/backend/conventions.md` gains a § Version naming `VERSION_VAR`.
  `.docs/git-conventions.md` § Version says `make build` stamps through it.
- `GOOSE_VERSION` goes from v3.27.2 to the template's v3.28.0 (owner's pick).
  `make migrate-status` and the backend tests pass with it.

`make diff` shows only dd's deliberate drift, 22 files:
- `PORT_BASE` 4140.
- Air in `dev-backend`, and `PUBLIC_API_BASE_URL` in `dev-frontend`.
- The split `install-*` and `build-*` targets, `install-e2e`, `test-e2e` and `test-e2e-deep`.
- `check-error-codes`, and `bun run` for check, lint, test and format.
- The seeding, `admin-reset-password`, `import-docs`, `sync-colors`, `introspect-*` and `clean-*`
  targets.
- No `preview` or `BUILD_NOTE`.
- The two deploy stubs, and the `ship` wording.
- `/usr/local/go/bin` in PATH.
- The compose file's pinned name.
- dd's own docs, `frontend/.prettierignore`, `frontend/eslint.config.js`, `AGENTS.md`, `VERSION`
  and `cinch.yml`.

`frontend/bunfig.toml`, `frontend/tests/support/*` and `.dockerignore` show as "only in template".
dd has no `skip=` line, as with ortho.

Prune listed 10 branches, all `(merge …)`, and `--confirm` deleted them. Two local branches remain
because neither landed. Both are pre-trunk work from 2026-10-09, and the owner keeps them for now:
- `chore/align-conventions`: 5 commits ahead of `main`.
- `refactor/db-context`: holds those commits plus its own, 70 files.

Remote branches on `origin` were not touched.

Verified:
- `make check` and `cinch check` pass.
- `make test` passes with the dev database up: 789 frontend tests, and the backend packages ok.
- `make test-e2e` gives 5 passed and 2 failed: `permissions/stranding.spec.ts:13` and
  `signatures/attestation.spec.ts:16`. The same two fail on `main`.
- `make build-backend` stamps `0.18.0` into the binary.
- `make -n dev` runs `dev-db` first, and `make -n down` runs `stop-dev`, then `stop-db`.
- `make deploy` and `make logs` exit 2.
- `make down` with no session prints no session line. Its volume survived, and the container is left
  stopped, as it was found.

Owner: `make dev` and `make down` on a live session in a real terminal, against docker.
Proposed commit (dd):
`chore [harness]: go-svelte-pg stack, harness-template v3, VERSION_VAR and the new down (plan 34 T14)`.
Next is T15 (budgetron).

**T15 done 2026-11-19** on budgetron branch `chore/harness-v3`, not committed yet. `make sync`
updated 9 managed files (the v3 stamp and the new `prune.sh`; flutter's own `scripts/version.sh`
only took the stamp). `settings.json` was already equal, and a second sync changed nothing.
`.harness` says `template=3` and `skip=.docs/stack-conventions.md`. budgetron keeps its code
style in `.docs/conventions.md`, and with the skip line `make diff` no longer shows the stack
conventions as "only in template". Ported by hand:
- Makefile: the template's `down` (C-c to each pane, 5 s wait, `=` session name), its comment,
  the `DEV_SESSION` comment ("stops") and the help line. The header says v3. flutter has no
  `VERSION_VAR` or `DEPLOY_NAME`, so nothing else to port. budgetron had no `down` workaround.
- The seeded docs keep their v2 stamp, because nothing in them changed in v3. README's `make
  down` line ("stop a tmux helper session") already fits.

`make diff` shows only budgetron's deliberate drift, 11 files: the ObjectBox parameters and
`setup-test`, `test` with `-x perf`, the `perf` and `coverage` targets and their help lines,
the `lib/objectbox.g.dart` analyzer exclude, and its own docs, `AGENTS.md` and `cinch.yml`.
`.docs/domain/overview.md` shows as "only in template". The roadmap says it's not written yet.

Prune listed 13 branches, all `(merge …)`, and `--confirm` deleted them. Four are left for the
owner:
- `feat/sms-import` landed. Each of its 9 commits has a commit on `main` with the same subject,
  timestamp and tree (`2f1ee59` to `5ee2d71`). The co-author strip rewrote them, so prune can't
  match them. Safe to `git branch -D`.
- `docs/installments-home-ui-plan` was stacked under `feature/installments-home-ui`, which
  landed as `f43f18e`, and its plan is archived on `main`. Safe to `git branch -D`.
- `backup/pre-coauthor-strip` is the same commit as `feat/sms-import`. It's the backup from
  before the rewrite, kept on purpose like cinch's `backup/pre-rewrite`.
- `claude/practical-cannon-e5p7hj` holds one commit from 2026-10-06, a first SMS-import
  prototype from a cloud session (`61ed2d2`, 24 files). Plan 12 rebuilt the feature and replaced
  it (`lib/logic/sms/sms_parser.dart` and others on `main`). Probably safe to drop.

Verified:
- `make check` passes: format, gen-l10n, analyze with no issues, and `strings: ok`.
- `make test` passes: 1259 tests.
- `cinch check` passes.
- `make down` with no session is silent and exits 0.
- On a private tmux server, `make down` stopped a live `budgetron-dev` session whose pane runs
  a SIGHUP-proof process. The session and the process are gone, and the sibling
  `budgetron-dev-x` session survived.

Owner: `make down` on a live helper session in a real terminal (budgetron's `dev` is a
foreground `flutter run`). Proposed commit (budgetron):
`chore [harness]: harness-template v3, skip list and the new down (plan 34 T15)`.
Next is T16 (pugio).

**T16 done 2026-11-19** on pugio branch `chore/harness-v3`, not committed yet. `make sync`
updated 10 managed files: the v3 stamp, and the new `prune.sh`. `settings.json` was already
equal, and a second sync changed nothing. `.harness` says `template=3`. Ported by hand in the
Makefile:
- The template's `down`: C-c to each pane, the 5 s wait, and the `=` session name.
- The `DEV_SESSION` comment ("stops"), the help line and the v3 header.

pugio keeps its deliberate exception: `down` doesn't force-stop the app. Pugio is the phone's
home screen, and a force-stop switches its accessibility service off. The comment above `down`
still says so, and `.docs/roadmap.md` already records it. android has no `VERSION_VAR` or
`DEPLOY_NAME`, and pugio had no workaround to delete. The seeded docs keep their v2 stamp.

`make diff` shows only pugio's deliberate drift, 10 files:
- Makefile: `APP_ID`, `A11Y_SERVICE`, the `startup` and `push-config` targets with their help
  lines, and the `down` without force-stop.
- pugio's own docs, `AGENTS.md` and `cinch.yml`.

Nothing shows as "only in template".

Prune listed 27 branches, all `(merge …)`, and `--confirm` deleted them. One is left for the
owner:
- `feature/p9-config-keys` landed. `fix/wallpaper-surface` was stacked on it, and that squash
  (`386b7e5`) holds all 9 of its commits: from the branch tip to `386b7e5` only the fix's 6 files
  differ. No `merge:` commit names the branch, so prune can't match it. Safe to `git branch -D`.

Verified:
- `make check` passes: ktlint and Android lint.
- `make test` passes. Gradle had the results cached (no source changed), so the run was all
  up-to-date.
- `cinch check` passes.
- `make down` with no session is silent and exits 0.
- On a private tmux server, `make down` stopped a live two-pane `pugio-dev` session whose panes
  run SIGHUP-proof processes. The session and both processes are gone, and the sibling
  `pugio-dev-x` session survived.

Owner: `make down` on a live helper session in a real terminal (pugio's `dev` is a foreground
`installDebug` and `am start`). Proposed commit (pugio):
`chore [harness]: harness-template v3 and the new down (plan 34 T16)`.

**Resume at:** nothing in this plan. All of T1–T16 is done. Left for the owner:
- Push harness-template `main` (`af9d641`, VERSION 3).
- Commit, land and push the rollout branches.
- Run each repo's `make dev` and `make down` in a real terminal (listed per task above).
- Delete the leftover branches named in T14–T16.

The plan can then be marked `complete` and archived (`cinch plan archive 34`). Plan 33 waits on
the same kind of owner step.

**Closed 2026-11-19.** The owner dropped the real-terminal `make dev` and `make down` checks.
They'll run them in time and report anything that breaks. pugio and budgetron were committed and
landed with `make merge`. harness-template `main` and the `main` of every rollout repo were
pushed. No tag was pushed and nothing was released. The plan is complete and archived.
