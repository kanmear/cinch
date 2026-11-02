# Leak replay: build brief

What a builder needs to make the harness, the constructed states, the hidden tests and the
references for [DESIGN.md](DESIGN.md). Per-task background is in each task's `NOTES.md`. The
withheld task T4 is specified only under `private/` (see § T4).

Hard limits, all builders:

- **DeltaDocs is read-only.** Reach it only with `git -C ~/code/project_deltadocs archive|show|ls-tree|rev-parse`.
  No worktrees, no tags, no checkouts, no tests in its tree. No DeltaDocs code is copied into this
  repo except the hidden tests and references named below (owner review before any push).
- **Never port 5432, never docker against it.** The benchmark's Postgres is the `cinch-binding-pg`
  container on 55432, user `deltadocs_user` (see `bench/binding/main.go`: `createDB`,
  `recreateDB`, `testDatabaseURL`).
- **No model calls** until `prereg/leak-pilot` (DESIGN § Calibration). `-engine fake` and the
  smoke prompt are the only engine uses before it, and the Claude smoke session needs the owner.
- Nothing here commits. The owner tags.

## 1. Harness layout

`bench/leak/` is `package main`, modelled on `bench/upkeep` (copy and adapt; don't import across
`main` packages):

| File | From | Change |
|---|---|---|
| `main.go` | upkeep | subcommands `states`, `apply`, `run`, `grade`, `judge`, `calibrate`, `analyze`, `manifest` |
| `sandbox.go` | upkeep | DeltaDocs toolchain (Go, bun, Go module cache and `node_modules` as copy-on-write overlays); the `docker-compose`/`docker` shim (§ 3) first on `PATH`; no docker socket; cinch on `PATH` in every arm |
| `engine.go` | upkeep | unchanged engines (`pi`, `claude`, `fake`); per-task cap from `PINS` |
| `states.go` | new | builds a constructed state (§ 2) and verifies its tree hash |
| `provision.go` | upkeep + binding | workspace from a constructed state; two DBs per run (§ 3) |
| `grade.go` | upkeep | hidden package, outcome, rule checks, bypass, validity (§ 5) |
| `judge.go`, `calibrate.go`, `rubric.md` | recover | rule-written judge (§ 6) |
| `analyze/` | upkeep analyze | CMH two looks over the kept tasks, exact p, secondaries (DESIGN § Analysis) |
| `main_test.go` | new | state-script anchors, outcome classification, shim behaviour, manifest check |

`PINS` is the pin of record. The harness refuses to run while a value is `TODO`, when a base or fix
SHA doesn't resolve, when a constructed tree's hash differs from its pin, or when a file under
`private/` differs from `bench/private-manifest.txt`.

## 2. Constructed states (no tags in DeltaDocs)

A state is `(task, arm)`. Arm 3 uses arm 1's state; T4 has arm 1 only. One script per state:
`states/<task>-arm<k>.py` (T4: `private/states/T4-arm1.py`). `leak states -task T -arm k -out DIR`
does, in order:

1. `git -C $DELTADOCS archive <T_BASE_SHA> | tar -x` into an empty temp dir.
2. **Common preparation** (`states/common.py`, shared by every script): delete `.claude/agents/`,
   `.cursor/`, `.pi/`, `.pi-lens.json`, `backend/bin/server`; keep `.claude/settings.json` and
   `.claude/hooks/`; set `require.cinch` in `cinch.yml` to `'>=0.9.0'` if it is an exact pin
   (only `4c8af82`); keep the state's own `AGENTS.md`.
3. **Task strip and arm additions**, the script's own steps (§ 4). Files an arm adds from a fix
   commit are read at build time with `git -C $DELTADOCS show <T_FIX_SHA>:<path>`, never stored here.
4. `cinch render` with the pinned cinch, then `cinch check` (must be green).
5. `git init`, branch `feature/task`, one commit `chore: prepare workspace` with a fixed author and
   date (`bench <bench@localhost>`, `2026-01-01T00:00:00Z`).
6. `git ls-tree -r HEAD | sha256sum` must equal `PINS` `<T>_ARM<k>_TREE_SHA256`. `leak states -pin`
   prints the hashes for the owner to commit at `prereg/leak-pilot`.

Script rules: Python, run with `python3 -I`, operating on the exported tree only. Every edit is
anchored (a Go function by its `func (h *X) Name(` line through the next `\n}\n` at column 0, a
route by its exact `mux.HandleFunc(` line, a rule by its bold ID's list item, a test by its
`func TestX(` plus the marker comment above it) and asserts its anchor matches exactly once, so a
wrong base fails loudly. Each script ends by asserting the post-conditions listed in § 4 (for
example `grep` for the removed event name returns nothing in `backend/`).

## 3. Workspace, databases, shim

- Per run, two fresh DBs in `cinch-binding-pg`: `leak_<engine>_<task>_a<arm>_r<rep>_work` and
  `..._grade` (`docker exec cinch-binding-pg createdb -U deltadocs_user <name>`), dropped after
  grading unless the run is flagged. `TEST_DATABASE_URL` is the template from the state's
  `backend/.env.example` or `.agent/manifest.yml`, with host `127.0.0.1`, port 55432 and the work
  DB's name; write it to the workspace's `backend/.env` (git-ignored by `.env*`) and the sandbox
  environment.
- **Shim.** The sandbox `PATH` starts with a directory holding `docker-compose` and `docker`
  scripts. `docker-compose exec -T postgres createdb ...` prints
  `createdb: error: database "deltadocs_test" already exists` and exits 1 (the backend Makefile's
  `test` target accepts that text and continues to `go test -count=1 -p 1 ./...`). Every other
  invocation of either name prints `docker is not available in this workspace` and exits 1. The
  sandbox has no docker socket. `main_test.go` tests both behaviours.
- Smoke prompt (`smoke-prompt.md`, fixed shell probes, no task): work DB reachable;
  `make test-backend` reaches `go test`; `docker ps` refused; `git log --all` shows one commit;
  `cinch version` matches `PINS`; `~/code` absent.

## 4. Tasks

Hidden-package conventions, every task:

- Source in `tasks/<T>/hidden/*.go` (T4: `private/tasks/T4/hidden/`), package `leakbench_<t>`,
  copied to `backend/tests/leakbench_<t>/` at grading and run alone:
  `go test ./tests/leakbench_<t>/ -count=1 -json` against the grade DB.
- Own `TestMain` (`os.Exit(tests.RequireDB(m))`); own helper types; every identifier prefixed
  `LeakBench`. Test names encode the class the grader uses: `TestLeakBench_<T>_F_*` (functional),
  `_L_*` (leak), `_S_*` (secondary).
- Only the HTTP surface: `handlers.RegisterRoutes(mux, handlers.RouteDeps{...})` with named
  fields, `httptest.NewServer`, session tokens from `tests.MakeSession`, setup through `tests.Make*`
  and `models.New*Repository`. Never call handler methods or constructors directly.
- L asserts state or delivery; status codes are `_S_` tests.
- Must compile unchanged on both arm states of the task and pass 20 consecutive times on
  `reference.patch`.

References, every task: `tasks/<T>/reference-naive.patch` (the historical code; usually the
reverse of the strip, `diff` between the arm-1 state and the base export restricted to the task's
files) and `tasks/<T>/reference.patch` (historical fix plus a rule and a marked test), both
applying to the arm-1 state. Expected calibration results are in DESIGN § Calibration.

### T1 `tab-updated`

- Base `4c8af8295b948244c39c09698c6b2470f04fc8ac`; fix `adc4abbbbb090d2ce981da0fa9c9d5f787c15ea5`.
- `states/T1-arm1.py` on the base: in `backend/handlers/tab.go`, inside `Update` and `Restore`,
  delete the `targets := collectTargetUserIDs(...)` line and the `publishEvent(... "tab.updated" ...)`
  statement after `RespondJSON` (drop any variable left unused); delete `TabUpdatedPayload` from
  `backend/realtime/events.go`; delete the `tab.updated` row from `.agent/api/realtime.md`.
  Post-condition: `grep -rn 'tab.updated\|TabUpdated' backend` is empty.
- `states/T1-arm2.py` on the **fix** SHA (export `adc4abb` instead of the base): the same three
  deletions, plus delete `TestTabUpdated_TargetsOnlyCategoryViewers` and
  `TestTabRestored_TargetsOnlyCategoryViewers` from `backend/tests/handlers/tab_realtime_leak_test.go`.
  RT-001 text unchanged.
- Hidden, adapted from `adc4abb:backend/tests/handlers/tab_realtime_leak_test.go`
  (`newTabRealtimeFix`, the Updated and Restored tests): same fixture (owner, `viewer` with `view`,
  `blind` with `none`, one page and tab), but through the routes. Delivery via a real hub and
  websocket clients (helpers in `4c8af82:backend/tests/handlers/realtime_test.go`:
  `newWSTestServer`, `dialMember`, `receivesWithin`); fall back to an own recording publisher in
  `RouteDeps.Publisher` if the websocket version flakes. Drop the real test's `require.Len(calls, 1)`.
  `_F_UpdateDeliversToViewer`; `_L_UpdateNotToBlind`, `_L_RestoreNotToBlind`;
  `_S_RestoreDeliversToViewer`, `_S_ActorNotTargeted`, `_S_CreatedNotToBlind`, `_S_DeletedNotToBlind`.
- Naive: the base's own lines restored. Correct: `adc4abb`'s `Update`/`Restore` targeting, the
  payload, the doc row, a new `.agent/domain/realtime.md` rule and a marked test.

### T2 `category-rename`

- Base `d0866c9089fe81fabf2cc4d5558c006fc72dfd62`; fix `c5389db6232b7f7505299caa6c226f4a313c4938`.
- `states/T2-arm1.py`: delete the `PATCH /api/projects/{projectID}/categories/{id}` route and
  `CategoryHandler.Update`; delete `TestRenameCategory_NonOwnerEditMember_Succeeds` and move its
  CAT-006 marker comment onto `TestDeleteCategory_NonOwner_Returns403`; delete
  `TestCategoryRenamed_TargetsOnlyCategoryViewers`; remove any other test case that PATCHes a
  category. Keep the repository's `UpdateCategory(id, name)` and its model tests, the API doc, the
  frontend.
- `states/T2-arm2.py`: arm 1, then from the fix commit: CAT-007's list item into
  `.agent/domain/categories.md` and its two 404 lines into `.agent/api/categories.md`; the
  `DeleteCategory(projectID, id)` signature in `backend/models/category.go` (interface and
  method) and its call in the handler; `TestDeleteCategory_CategoryOfOtherProject_Returns404`; and
  `TestCategoryMutations_ScopedToProject` with its `UpdateCategory` lines removed. `UpdateCategory`
  keeps arm 1's signature.
- Hidden, adapted from `c5389db:backend/tests/handlers/category_test.go`
  (`TestRenameCategory_CategoryOfOtherProject_Returns404`): `_F_OwnerRenamesOwnCategory`;
  `_L_ForeignCategory_Owner` and `_L_ForeignCategory_PlainMember` (leak = the foreign category's
  stored name changed); `_S_Status404`, `_S_PlainMemberRenamesOwn`, `_S_Validation`,
  `_S_RenamedNotToNoneMember` (build this fixture with ordinary roles only; don't reuse the
  real `category_realtime_leak_test.go` fixture's extra members), `_S_DeleteScoped`.
- Naive: the base's `Update` and route. Correct: scoped rename (as the fix), CAT-007-like rule,
  marked test.

### T3 `email-invite`

- Base `d2fb2106ddd90e993a13ab5f7b436d4c7a23d94d`; fix `f99130f7637bdcb70430e9c158bef7ab432347ad`.
- `states/T3-arm1.py`: delete the `POST .../members/invite` route; delete `Invite`,
  `inviteMemberRequest` and `inviteMemberResponse` from the member handler file (keep
  `NewMemberHandler` and the rest of the file unchanged); delete every test in
  `backend/tests/handlers/invite_test.go` that calls the invite endpoint, keeping the accept tests
  (`pendingWithInvite` uses the repository); delete MEMB-018 and MEMB-019 and fix links to them;
  restore MEMB-006's text from `210c6defd8fa0a14b3a2fcb0ce6066636db62949`; delete the invite section
  of `.agent/api/members.md` and `.agent/plans/email-invites.md`. Post-condition: `cinch check`
  green (no orphan, no stray marker).
- `states/T3-arm2.py`: arm 1, then from the fix commit: MEMB-024's list item (cross-reference to
  MEMB-019 dropped) and the "temporary link" paragraph; `HasMembershipUnmanagedBy` in
  `backend/models/member.go` (interface and method); its tests from
  `backend/tests/models/member_test.go`, marked MEMB-024.
- Hidden, adapted from `f99130f:backend/tests/handlers/invite_test.go`
  (`TestInvite_PendingHeldByUnmanagedProject_NoLinkAndExistingLinkSurvives`,
  `..._AlreadyMemberOfCallerProject_NoLink`, and for F `TestInvite_NewEmail_CreatesPendingAccountAndMember`,
  `TestInvite_ActiveNonMember_PlainAddWithoutLink` from `d2fb210`): `_F_NewEmailLinkLogsIn`,
  `_F_ActiveAccountNoLink`; `_L_PendingElsewhere_CallerGetsNoLogin`,
  `_L_PendingElsewhere_Reinvite` (leak = the second project's caller receives an `invite_url`
  whose token returns 200 from `POST /api/auth/invite/accept`, whatever the invite's status);
  `_S_FirstLinkStillLive`, `_S_MemberAddedPublished`. Mind the accept endpoint's rate limiter.
- Naive: `d2fb210`'s `Invite` and route. Correct: `f99130f`'s `Invite` and repository method,
  invite rules and marked tests.

### T4 (withheld)

Everything for T4 lives under `private/`, which is git-ignored: `private/T4-DESIGN.md` (the task's
design), `private/tasks/T4/` (prompt, notes, hidden tests, references) and
`private/states/T4-arm1.py`. T4 has arm 1 (and arm 3, which uses arm 1's state), no arm 2. The
harness treats it like any task, reading its paths from `PINS`. Write nothing about it in public
files, commit messages or test names outside `private/`.

## 5. Grader

Per run, after the engine exits (DESIGN § Grading):

1. Copy the hidden package; run it on the grade DB; classify by name prefix. A build failure of the
   package marks every `_F_` failed.
2. Outcome: delivered (all `_F_` pass), leaked, safe, undelivered.
3. `cinch rules --json` on the final tree and on the arm's state; structural rule-written (new or
   changed rule outside `.agent/plans/`, with a marker under `backend/tests/` or `frontend/tests/`);
   collect candidate rule texts for the judge.
4. Backend suite on a second fresh DB (secondary).
5. Transcript scans: bypass, validity, `make dev-db`, "address already in use", whether
   `dev-plan-feature.md` was read.
6. One CSV row per run (`results-<engine>.csv`): task, arm, rep, engine, delivered, leaked, safe,
   each test's result, structural rule check, candidate IDs, bypass, valid, timed_out, committed,
   hook_blocks, turns, tokens, wall_s.

## 6. Judge

Reuse `bench/recover/judge.go` and `calibrate.go`. `rubric.md` gives the judge one invariant (from
the task's NOTES) and one candidate rule text, and asks for STATES, PARTIAL or ABSENT with a
one-line reason. Probes per task in `tasks/<T>/judge-probes.json` (T4's under `private/`), taken
from each NOTES' "Rule-written key". Judge for pi runs: Claude Sonnet. Judge for Claude runs:
pi/Qwen. Both judges are calibrated on the same probes; each engine's candidates also get a
seeded blind owner audit sample, as in `bench/recover`.

## 7. Manifest

`leak manifest` writes `bench/private-manifest.txt`: one `sha256sum`-format line per file under
`bench/leak/private/`, sorted by path. It is committed at `prereg/leak-pilot`; the harness refuses
to run if a private file no longer matches.

## 8. Order of work

1. Harness skeleton, shim, DB provisioning, `-engine fake` probe.
2. State scripts for T1–T3 (then T4 under `private/`), with their tree hashes.
3. Hidden packages and references per task; calibration (DESIGN § Calibration) green.
4. Judge rubric and probes; judge calibration (paid for the Sonnet judge: owner confirms first).
5. Hand back for the owner's predictions and `prereg/leak-pilot`.
