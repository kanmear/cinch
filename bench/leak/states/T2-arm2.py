"""T2 `category-rename`, arm 2 (bench/leak/BUILD.md § 4): arm 1 plus CAT-007, the
delete half of c5389db's fix, and its marked tests.

Added from the fix commit (read with git show at build time): CAT-007's list item
in .agent/domain/categories.md and its 404 lines in .agent/api/categories.md;
DeleteCategory(projectID, id) in the interface, the repository and the handler
call (the three model tests that call it move to the new signature);
TestDeleteCategory_CategoryOfOtherProject_Returns404; and
TestCategoryMutations_ScopedToProject with its UpdateCategory lines dropped.
Not applied: the rename half (UpdateCategory keeps arm 1's unscoped signature),
FILE-008, MEMB-014.
"""
import importlib.util
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import anchors as a
import anchors_t2 as t2

spec = importlib.util.spec_from_file_location("t2_arm1", os.path.join(HERE, "T2-arm1.py"))
arm1 = importlib.util.module_from_spec(spec)
spec.loader.exec_module(arm1)


def build(tree):
    arm1.build(tree)
    fix = a.env("FIX_SHA")

    # CAT-007's list item and the two 404 lines.
    rel = ".agent/domain/categories.md"
    fixed = a.show(fix, rel)
    item = a.rule_item(fixed, "CAT-007")
    text = a.read(tree, rel)
    cat006 = a.rule_item(text, "CAT-006")
    text = a.replace_once(text, cat006, cat006 + "\n" + item, "CAT-006 item")
    a.write(tree, rel, text)

    rel = ".agent/api/categories.md"
    text = a.read(tree, rel)
    text = a.replace_once(
        text,
        "- `404` `CATEGORY_NOT_FOUND`\n- `409` `CATEGORY_DUPLICATE_NAME`",
        "- `404` `CATEGORY_NOT_FOUND` — no category with this ID in the URL project ([CAT-007](../domain/categories.md))\n"
        "- `409` `CATEGORY_DUPLICATE_NAME`",
        "PATCH 404 line",
    )
    text = a.replace_once(
        text,
        "- `403` `API_FORBIDDEN` — caller's resolved role is not the Owner role\n\n## GET /api/projects/{projectID}/categories/{categoryID}/eligible-signers",
        "- `403` `API_FORBIDDEN` — caller's resolved role is not the Owner role\n"
        "- `404` `CATEGORY_NOT_FOUND` — no category with this ID in the URL project ([CAT-007](../domain/categories.md))\n\n"
        "## GET /api/projects/{projectID}/categories/{categoryID}/eligible-signers",
        "DELETE 404 line",
    )
    a.write(tree, rel, text)

    # DeleteCategory(projectID, id): interface, repository, handler call.
    rel = "backend/models/category.go"
    text = a.read(tree, rel)
    text = a.replace_once(text, "\tDeleteCategory(id int) error\n", "\tDeleteCategory(projectID, id int) error\n", "interface")
    text = a.replace_once(
        text,
        "func (r *categoryRepository) DeleteCategory(id int) error {\n"
        '\tresult, err := r.db.Exec("DELETE FROM categories WHERE id = $1", id)\n',
        "func (r *categoryRepository) DeleteCategory(projectID, id int) error {\n"
        '\tresult, err := r.db.Exec("DELETE FROM categories WHERE id = $1 AND project_id = $2", id, projectID)\n',
        "repository",
    )
    a.write(tree, rel, text)

    rel = "backend/handlers/category.go"
    text = a.read(tree, rel)
    text = a.replace_once(text, "h.categories.DeleteCategory(id)", "h.categories.DeleteCategory(projectID, id)", "handler call")
    a.write(tree, rel, text)

    # Model tests: the three existing DeleteCategory calls take the new signature.
    rel = "backend/tests/models/category_test.go"
    text = a.read(tree, rel)
    text = a.replace_once(text, "err = repo.DeleteCategory(cat.ID)", "err = repo.DeleteCategory(project.ID, cat.ID)", "TestDeleteCategory")
    text = a.replace_once(text, "repo.DeleteCategory(999999)", "repo.DeleteCategory(1, 999999)", "NotFound")
    text = a.replace_once(text, "catRepo.DeleteCategory(cat.ID)", "catRepo.DeleteCategory(project.ID, cat.ID)", "CascadesTabs")
    fixed = a.show(fix, rel)
    scoped = t2.extract_go_func(fixed, "func TestCategoryMutations_ScopedToProject(")
    scoped = a.replace_once(
        scoped,
        '\tupdated, err := repo.UpdateCategory(project.ID, foreign.ID, "Hijacked")\n'
        "\trequire.NoError(t, err)\n"
        "\tassert.Nil(t, updated)\n\n"
        "\terr = repo.DeleteCategory(project.ID, foreign.ID)\n",
        "\terr = repo.DeleteCategory(project.ID, foreign.ID)\n",
        "scoped test update lines",
    )
    text = t2.append_go(text, scoped)
    a.write(tree, rel, text)

    rel = "backend/tests/handlers/category_test.go"
    text = a.read(tree, rel)
    fixed = a.show(fix, rel)
    delete_test = t2.extract_go_func(fixed, "func TestDeleteCategory_CategoryOfOtherProject_Returns404(")
    text = t2.append_go(text, delete_test)
    a.write(tree, rel, text)

    # Post-conditions: the delete half is scoped, the rename half is not.
    if "UpdateCategory(id int, name string)" not in a.read(tree, "backend/models/category.go"):
        a.die("UpdateCategory must keep arm 1's unscoped signature")
    if "DeleteCategory(projectID, id int)" not in a.read(tree, "backend/models/category.go"):
        a.die("DeleteCategory must be scoped")
    a.no_match(tree, r"categoryHandler\.Update|CategoryHandler\) Update\(", "backend")


if __name__ == "__main__":
    build(a.tree_arg())
