# 34. Harness-template gaps from the rollout

**Status:** in-progress

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
6. **There is no repo-local layer for managed `.claude/settings.json` (T13, limen).** `sync`
   overwrites the whole file, so a repo's own permissions or hooks are lost. Options: a managed
   base merged with a seeded `.claude/settings.repo.json`, or managing only the `hooks` key.
7. **`prune.sh` misses branches that landed long ago (seen while landing T15–T19).** It counts a
   branch as landed only when `git merge-tree --write-tree main <branch>` would change nothing.
   A branch whose squash landed and whose files `main` edited later does not pass that test.
   The plan expected about 24 landed branches in verus; prune found 4. ortho, limen, forum and
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
6. **`sync` manages only the `hooks` key of `.claude/settings.json`.** It replaces `.hooks`
   with the template's using `jq`, and keeps every other key. A target without the file gets
   the template's copy. A target whose file isn't valid JSON makes `sync` fail without writing.
   `tests/managed.sh` compares `.hooks` for this path, not bytes. `jq` becomes a dependency of
   the template's `sync`, `diff` and `make check`, not of the repos (approved 2026-11-18).
   limen moves `HERDR_ENV` back into the committed file. *As built (T5):* `diff` never reads
   managed files, so it neither compares `.hooks` nor needs `jq`; only `sync` and `make check` do.
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
**Status:** [ ]
**Depends on:** T7
**Files:** rollout steps; `VERSION_VAR` in the Makefile.
**Rules:** —
**Verify:** see Rollout

### T9: limen (go-cli)
**Status:** [ ]
**Depends on:** T7
**Files:** rollout steps; `HERDR_ENV` moves from `.claude/settings.local.json` into
`.claude/settings.json`; `.docs/architecture.md` (the `HERDR_ENV` row) says so. About 36 branches
left from plan 33.
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
