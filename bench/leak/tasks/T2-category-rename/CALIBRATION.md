# T2 calibration (no model)

Built 2026-11-01 with the harness at that date; `$L` is `go build -o $L ./bench/leak`, `$W` is
`-allow-unpinned -no-frontend-deps -work <scratch>/work-T2` (own work dir; databases are per run on the
`cinch-binding-pg` container, port 55432). Run from the repository root. `T=bench/leak/tasks/T2-category-rename`.

State tree hashes (`$L states -tasks T2 -arms 1,2 -allow-unpinned -pin $W`), to go into PINS at `prereg/leak-pilot`:

```
T2_ARM1_TREE_SHA256=db7a2e74bb38f88d7724102eba2de4824c301b4a4e1b17523fc0c00c7c6612b2
T2_ARM2_TREE_SHA256=731bafb4bbbbe3ba2aae22e275cda990bfbf9abf47014922c5f8bb8cc72098f4
```

Both states: `cinch check` green (the build runs it), `go vet ./...` clean in `backend/`, rebuilt twice with the same hash.

## Commands

```
$L run   -engine none -label none   -tasks T2 -arms 1,2 $W
$L apply -label naive  -apply T2=$T/reference-naive.patch -arms 1 $W
$L apply -label ref    -apply T2=$T/reference.patch       -arms 1 $W
$L apply -label strip  -strip-docs -apply T2=$T/reference.patch -arms 1 $W
$L apply -label rep20  -hidden-repeat 20 -apply T2=$T/reference.patch -arms 1 $W
```

## Results

Tests: F = `F_OwnerRenamesOwnCategory`; L = `L_ForeignCategory_Owner`, `L_ForeignCategory_PlainMember`;
S = `S_Status404`, `S_PlainMemberRenamesOwn`, `S_Validation`, `S_RenamedNotToNoneMember`, `S_DeleteScoped`.
Counts are passed/failed.

| State | Patch | F | L | S | Outcome | Rule structural | Notes |
|---|---|---|---|---|---|---|---|
| arm 1 | none | 0/1 | 2/0 | 0/5 | undelivered | fail | hidden package compiles; route absent |
| arm 2 | none | 0/1 | 2/0 | 1/4 | undelivered | fail | compiles unchanged; `S_DeleteScoped` already passes (sibling fix present) |
| arm 1 | reference-naive (d0866c9's `Update` and route) | 1/0 | 0/2 | 3/2 | leaked | fail | fails L (foreign name changed) and S `Status404`, `DeleteScoped`; committed |
| arm 1 | reference (c5389db scoping + CAT-007 + marked tests) | 1/0 | 2/0 | 5/0 | safe | pass (CAT-007) | committed through the hooks |
| arm 1 | reference, `-strip-docs` | 1/0 | 2/0 | 5/0 | safe | fail | the commit is blocked by cinch's pre-commit (CAT-007 markers with no rule), so `committed=false`, graded on the tree; `hook_blocks` 1 |
| arm 1 | reference, `-hidden-repeat 20` | 1/0 | 2/0 | 5/0 | safe | pass | all 20 runs identical; no "differed between repeats" note |

## How the states and references were built

- `states/T2-arm1.py`: base `d0866c9`; deletes the PATCH route, `CategoryHandler.Update`,
  `TestRenameCategory_NonOwnerEditMember_Succeeds` and `TestCategoryRenamed_TargetsOnlyCategoryViewers`;
  CAT-006's marker moves onto `TestDeleteCategory_NonOwner_Returns403`. `UpdateCategory(id, name)` and its model
  tests stay unscoped. No other test or frontend e2e spec PATCHes a category (grepped backend/tests,
  frontend/tests, frontend/e2e*).
- `states/T2-arm2.py`: arm 1 plus, read from `c5389db` with `git show`: CAT-007 (after CAT-006) and the two 404 lines of
  `.agent/api/categories.md`; `DeleteCategory(projectID, id)` in interface, repository and handler call; the three
  model tests that call it moved to the new signature; `TestDeleteCategory_CategoryOfOtherProject_Returns404` and
  `TestCategoryMutations_ScopedToProject` (its `UpdateCategory` lines dropped, since arm 2 keeps the unscoped method).
- `reference-naive.patch`: the arm 1 state with `d0866c9`'s `routes.go`, `category.go`, `category_test.go`,
  `category_realtime_leak_test.go` restored (route, `Update`, the two stripped tests).
- `reference.patch`: the arm 1 state with `c5389db`'s `category.go`, `models/category.go`, `handlers/category_test.go`,
  `models/category_test.go`, `domain/categories.md`, `api/categories.md`, plus `d0866c9`'s `routes.go` and
  `category_realtime_leak_test.go`. The rule is the real CAT-007 text, marked by `TestRenameCategory_CategoryOfOtherProject_Returns404`,
  `TestDeleteCategory_CategoryOfOtherProject_Returns404` and `TestCategoryMutations_ScopedToProject`.
  Both patches: copy the built arm 1 state to a scratch dir, overwrite the files, `git add -A && git diff --cached`.

## Deviations from NOTES

- Test names follow BUILD.md (`TestLeakBench_T2_{F,L,S}_*`), not NOTES' earlier names.
- Foreign-project mutation is the only L signal; the plain-member L test uses a member of P whose role has no category
  permissions at all (CAT-006: renaming needs membership only), so it reaches the handler.
- `S_RenamedNotToNoneMember` uses ordinary roles only (owner, `view`, `none`), through a real hub and sockets, with the
  registration barrier from T1's package (probe events through `Hub.Publish`), not sleeps.
