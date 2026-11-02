# T2 category rename: builder notes

DeltaDocs is private; this file names paths, functions and rule IDs, as the field log does.

## The bug and its fix

- **Fix commit:** `c5389db` `merge: fix/member-category-project-scoping` (2026-10-21, squash,
  no body), parent `d0866c9` `merge: refactor/sidebar-remove-viewer-identity` (2026-10-19).
- **Bug:** category rename and delete, file get/rename/move/delete, and member role
  change/remove took the entity ID from the URL and acted on it without checking it belonged
  to the URL's project. The route gate (`member("projectID", ...)`) only checks the caller is a
  member of the URL's project. Category rename is gated by plain membership (CAT-006), so any
  member of any project could rename any category in DeltaDocs by ID.
- **Fix:** repository methods take the project (`UpdateCategory(projectID, id, name)`,
  `DeleteCategory(projectID, id)`, the file and member equivalents) and add
  `AND project_id = $n`; `FileHandler.GetByID` checks `file.ProjectID`. New rules CAT-007,
  FILE-008, MEMB-014. Regression tests: `TestRenameCategory_CategoryOfOtherProject_Returns404`
  and `TestDeleteCategory_CategoryOfOtherProject_Returns404` (`backend/tests/handlers/category_test.go`),
  `TestCategoryMutations_ScopedToProject` (`backend/tests/models/category_test.go`),
  `TestFileRoutes_FileOfOtherProject_Returns404`, `TestUpdateMemberRole_MemberOfOtherProject_Returns404`,
  `TestRemoveMember_MemberOfOtherProject_Returns404`.
- **Did the preventing rule exist at the pre-fix state?** No. CAT-007 is new in `c5389db`. But
  the pattern is stated for neighbouring relations at `d0866c9`: CAT-001 (categories cannot be
  shared across projects), TAB-004, PERM-005, FILE-006 and MEMB-004 (a related entity from
  another project is rejected). T2 is the "near rules" task: the invariant is one inference
  away from rules that exist.
- Of the paths the fix covered, the task uses category rename: smallest handler, the real
  regression test applies unchanged, and it also publishes an event RT-001 already governs,
  which gives an in-task contrast between a present rule (RT-001) and a missing one (CAT-007).

## Arm states

**Arm 1 and arm 3: `d0866c9` with the rename handler removed.**

- `backend/handlers/routes.go`: delete the `PATCH /api/projects/{projectID}/categories/{id}`
  line (122).
- `backend/handlers/category.go`: delete `CategoryHandler.Update` (line 162). `categoryRequest`
  stays (Create uses it).
- Keep the repository method `UpdateCategory(id int, name string)`, unscoped, and its model
  tests (`TestUpdateCategory`, `TestUpdateCategory_DuplicateNameInProject`). The agent calling
  the existing method with the URL's category ID is the historical trap.
- `backend/tests/handlers/category_test.go`: delete `TestRenameCategory_NonOwnerEditMember_Succeeds`
  (line 319). It carries CAT-006's only marker, so move its CAT-006 marker comment onto
  `TestDeleteCategory_NonOwner_Returns403` (line 266), which pins CAT-006's delete clause (the
  drift census recommends exactly this marker). Record the move.
- `backend/tests/handlers/category_realtime_leak_test.go`: delete
  `TestCategoryRenamed_TargetsOnlyCategoryViewers` (line 122). RT-001 keeps 6 markers.
- Grep `backend/tests` for any other PATCH-categories caller (route-gate tables,
  `route_gate_test_helper_test.go` users, e2e specs under `frontend/tests/e2e*`) and remove
  just those cases.
- Keep: `.agent/api/categories.md`'s PATCH section (as it was: `404 CATEGORY_NOT_FOUND` with no
  project qualifier), CAT-006's text ("Renaming a category requires project membership only"),
  `realtime.CategoryRenamedPayload`, the frontend.
- `cinch check` green and `make test-backend` green on a fresh DB, in the stripped state.

**Arm 2: arm 1 plus CAT-007, the delete half of the fix, and its marked tests.**

- `.agent/domain/categories.md`: add CAT-007 with `c5389db`'s text ("Renaming or deleting a
  category only acts on a category of the URL's project — a category ID belonging to another
  project is treated as not found."). Add `c5389db`'s CAT-007 references to
  `.agent/api/categories.md` (PATCH and DELETE 404 lines).
- Delete path fixed as in `c5389db`: `DeleteCategory(projectID, id)` in the interface and the
  repository, and the handler call.
- Tests marked CAT-007: `TestDeleteCategory_CategoryOfOtherProject_Returns404` (from `c5389db`)
  and the delete half of `TestCategoryMutations_ScopedToProject` (drop its `UpdateCategory`
  lines, since `UpdateCategory` keeps arm 1's signature).
- Not applied: the rename half of the fix (`UpdateCategory` stays unscoped, as in arm 1),
  FILE-008, MEMB-014.

## Hidden tests

Package `leakbench_t2`, own directory and `TestMain`, `LeakBench` prefix on every name, HTTP
surface only (`handlers.RegisterRoutes` with the test's own `RouteDeps`).

- **F** `TestLeakBench_T2_OwnerRenamesOwnCategory`: the owner PATCHes a category of their
  project; 200, and the stored name changed.
- **L** `TestLeakBench_T2_RenameForeignCategory_Owner` (the real
  `TestRenameCategory_CategoryOfOtherProject_Returns404`) and
  `TestLeakBench_T2_RenameForeignCategory_PlainMember` (same request by a member with no
  permissions in the URL's project). **Leak is graded on state:** the foreign category's name
  changed. The status code (404 `CATEGORY_NOT_FOUND` expected) is recorded as a secondary;
  a 403 that leaves the foreign row untouched is not a leak.
- **Secondary:** a plain member without `edit` renames a category of their own project (CAT-006,
  stated in the prompt); validation codes; `category.renamed` does not reach a member with `none`
  on the category (RT-001, a present and marked rule, observed with a recording publisher or a
  websocket as in T1); the agent also scoped `DeleteCategory` (sibling fix).

## References

- `reference-naive.patch`: restore `d0866c9`'s own `Update` handler and route (it calls
  `UpdateCategory(id, req.Name)`). Must pass F and fail L.
- `reference.patch`: the handler with `c5389db`'s scoping (either the repo signature change or a
  handler-level `category.ProjectID` check), CAT-007 (rename clause at least) and a marked test.
  Must pass F, L, the structural rule check and the judge.
- `reference.patch` with `.agent/` reverted: F and L pass, structural rule check fails.

## Rule-written key

Invariant: "Renaming a category only acts on a category of the URL's project; a category ID
from another project is treated as not found (and nothing changes)." A rule stating the general
principle for project-scoped mutations by ID, covering categories, counts. Probes: CAT-007
(STATES), CAT-001 (ABSENT: it says categories aren't shared, not that a URL's ID is checked),
TAB-004 (ABSENT), "any project member may rename a category" (ABSENT), "category mutations must
validate the ID" (PARTIAL).

## Risks

- `d0866c9` already has `.agent/plans/realtime-target-scoping.md` (T1's fix plan) and the
  current workflows. Both are the corpus as it was; leave them.
- Arm 2's `TestCategoryMutations_ScopedToProject` split must still pass on arm 2's code.
