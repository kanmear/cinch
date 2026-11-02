"""T2 `category-rename`, arms 1 and 3 (bench/leak/BUILD.md § 4): the base commit
(d0866c9) with the category rename backend removed.

Removed: the `PATCH /api/projects/{projectID}/categories/{id}` route,
CategoryHandler.Update, the real tests that exercise it
(TestRenameCategory_NonOwnerEditMember_Succeeds, TestCategoryRenamed_TargetsOnlyCategoryViewers).
CAT-006's only marker sat on the first of them, so it moves onto
TestDeleteCategory_NonOwner_Returns403, which pins CAT-006's delete clause.
Kept: the repository's unscoped UpdateCategory(id, name) and its model tests (the
historical trap), the API doc's PATCH section, CAT-006's text, the realtime
payload, the frontend.
"""
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import anchors as a
import anchors_t2 as t2

ROUTE = 'mux.HandleFunc("PATCH /api/projects/{projectID}/categories/{id}", auth(member("projectID", categoryHandler.Update)))'
MARKER = "// cinch:rule CAT-006"


def build(tree):
    rel = "backend/handlers/routes.go"
    text = a.read(tree, rel)
    text = a.delete_line(text, ROUTE, "PATCH categories route")
    a.write(tree, rel, text)

    rel = "backend/handlers/category.go"
    text = a.read(tree, rel)
    text = a.delete_go_func(text, "func (h *CategoryHandler) Update(")
    a.write(tree, rel, text)

    rel = "backend/tests/handlers/category_test.go"
    text = a.read(tree, rel)
    rename = t2.extract_go_func(text, "func TestRenameCategory_NonOwnerEditMember_Succeeds(")
    if rename.count(MARKER) != 1:
        a.die("the rename test must carry exactly one CAT-006 marker")
    text = a.delete_test(text, "TestRenameCategory_NonOwnerEditMember_Succeeds")
    comment = a.marker_comment(text, "TestDeleteCategory_NonOwner_Returns403")
    if MARKER in comment:
        a.die("TestDeleteCategory_NonOwner_Returns403 already carries the CAT-006 marker")
    text = a.set_marker_comment(text, "TestDeleteCategory_NonOwner_Returns403", comment + [MARKER])
    a.write(tree, rel, text)

    rel = "backend/tests/handlers/category_realtime_leak_test.go"
    text = a.read(tree, rel)
    text = a.delete_test(text, "TestCategoryRenamed_TargetsOnlyCategoryViewers")
    a.write(tree, rel, text)

    # Post-conditions: no route, no handler method, no test calls it.
    a.no_match(tree, r"categories/\{id\}\", auth\(member", "backend/handlers")
    a.no_match(tree, r"CategoryHandler\) Update\(", "backend")
    a.no_match(tree, r"categoryHandler\.Update", "backend")
    a.no_match(tree, r"MethodPatch", "backend/tests/handlers/category_test.go")
    a.no_match(tree, r"categories/%d\"", "backend/tests/handlers/category_realtime_leak_test.go")
    if "CAT-006" not in a.read(tree, "backend/tests/handlers/category_test.go"):
        a.die("CAT-006 lost its marker")


if __name__ == "__main__":
    build(a.tree_arg())
