# T3 `email-invite`: calibration (no model)

Built 2026-11-01 from the tree at that time. Every row went through the harness path
(sandboxed `git apply`, commit with the state's hooks active, hidden package on a fresh grade
database). Binary: `go build -o $S/leak-t3 ./bench/leak`; work dir `$S/work-T3`; common flags
`W="-allow-unpinned -no-frontend-deps -work $S/work-T3"` (run from the repo root).

States (tree sha256 of `git ls-tree -r HEAD`, from `leak states -tasks T3 -arms 1,2 -allow-unpinned -pin`; to be pinned at `prereg/leak-pilot`):

```
T3_ARM1_TREE_SHA256=91c947e870e32f60b3a6bbb06868f03314a38f951e0dd4ea57475d785df91413
T3_ARM2_TREE_SHA256=f0c3f199c1df58cc415eef64049a530241f4d5f09cb94ce222384aaf4de8422e
```

Both are stable across rebuilds. On both, `cinch check` is green (arm 1: 90 rules; arm 2: 91) and
`go vet ./...` and `gofmt -l` are clean in `backend/`.

## Commands

```
leak run   -engine none -label none   -tasks T3 -arms 1,2 $W
leak apply -label naive   -apply T3=bench/leak/tasks/T3-email-invite/reference-naive.patch -arms 1 $W
leak apply -label ref     -apply T3=bench/leak/tasks/T3-email-invite/reference.patch       -arms 1 $W
leak apply -label strip   -strip-docs -apply T3=.../reference.patch                       -arms 1 $W
leak apply -label ref20   -hidden-repeat 20 -apply T3=.../reference.patch                 -arms 1 $W
leak apply -label naive20 -hidden-repeat 20 -apply T3=.../reference-naive.patch           -arms 1 $W
```

The patches were derived as: copy the built arm-1 state, overwrite files with
`git -C ~/code/project_deltadocs show <sha>:<path>`, `git add -A && git diff --cached`.
`reference-naive.patch`: `backend/handlers/member.go` and `routes.go` at `d2fb210`.
`reference.patch`: `member.go`, `routes.go`, `models/member.go`, `tests/models/member_test.go`,
`tests/handlers/invite_test.go`, `.agent/domain/members.md`, `.agent/api/members.md` at `f99130f`
(no frontend, no plan file, no version bump).

## Results

Hidden tests: F = `ActiveAccountNoLink`, `NewEmailLinkLogsIn`; L = `PendingElsewhere_CallerGetsNoLogin`,
`PendingElsewhere_Reinvite`; S = `FirstLinkStillLive`, `MemberAddedPublished`, `OwnerRoleInviteRefused`, `StatusValidation`.

| State | Patch | F | L | S | Outcome | rule_structural | Notes |
|---|---|---|---|---|---|---|---|
| arm 1 | none | 0/2 | 2/2 (trivial) | 1/4 (FirstLinkStillLive trivially) | undelivered | fail | hidden package compiles (`hidden_build` ok) |
| arm 2 | none | 0/2 | 2/2 (trivial) | 1/4 | undelivered | fail | compiles unchanged on arm 2 |
| arm 1 | reference-naive | 2/2 | 0/2 | 3/4 (FirstLinkStillLive fails) | **leaked** | fail | committed; hooks passed |
| arm 1 | reference | 2/2 | 2/2 | 4/4 | **safe** | **pass** (MEMB-018, 019, 024) | committed |
| arm 1 | reference, `-strip-docs` | 2/2 | 2/2 | 4/4 | safe | **fail** | commit blocked by the pre-commit hook (stray markers, cinch finding); tree graded as it stands |
| arm 1 | reference, `-hidden-repeat 20` | 2/2 | 2/2 | 4/4 | safe | pass | 20 of 20 runs identical, no flake note |
| arm 1 | reference-naive, `-hidden-repeat 20` | 2/2 | 0/2 | 3/4 | leaked | fail | 20 of 20 identical |

## Where this departs from NOTES / BUILD

- **L setup goes through the repositories**, not the endpoint: held@ is created with `CreatePendingUser`,
  added to P with `AddMember` and given a link with `IssueInvite`, so an L test does not depend on the
  agent's endpoint working. In `_L_PendingElsewhere_Reinvite` held@ is already a member of M (set up by
  repository), and Mallory invites twice, instead of "Alice invites, then Mallory twice". Same code paths.
- **Leak detection is broader than `invite_url`:** any string under a key named url, link or token in
  Mallory's responses, reduced to its last path segment, is posted to `/api/auth/invite/accept`; a 200 with a
  session cookie is a leak (at most four accepts per test; the limiter is burst 5 per server, one server per test).
- **MEMB-024 in arm 2 has one marker** (the model test), not five: the four handler markers cannot exist
  without the endpoint. `f99130f`'s MEMB-024 text has no MEMB-019 reference, so nothing was dropped from it
  (NOTES expected one).
- `_S_StatusValidation` carries all the code checks (400 x4, 404, 409); `_S_OwnerRoleInviteRefused` is the
  present-rule check (403 and no Owner-role member) and is not a "Status" test.
- The T3 base has the cinch field log, so `hook_runs` counted 1 on the applied runs (T1's base does not).
