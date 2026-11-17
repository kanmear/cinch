# leak-state: base=FIX
"""T1 `tab-updated`, arm 2 (bench/leak/BUILD.md § 4): the fix commit with the task
stripped. RT-001 is present and marked; tab.created, tab.deleted and the category
events already target category viewers; the task's own path (Update, Restore) is
as bare as in arm 1.

Same three deletions as arm 1 (the publish blocks in Update and Restore,
TabUpdatedPayload, the tab.updated row of .agent/api/realtime.md), plus the two
real tests that are the hidden test: TestTabUpdated_TargetsOnlyCategoryViewers
and TestTabRestored_TargetsOnlyCategoryViewers. RT-001's text, which names
tab.updated, stays; so do its five other markers.
"""
import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import anchors as a

PUBLISH = re.compile(
    r"\ttargets := collectCategoryViewerTargetUserIDs\(h\.members, h\.access, projectID, tab\.CategoryID, user\.ID\)\n"
    r"\tpublishEvent\(r\.Context\(\), h\.publisher, projectID, \"tab\.updated\", realtime\.TabUpdatedPayload\{\n"
    r"[^\n]*\n"
    r"\t\}, user\.ID, targets\)\n"
)


def strip_publish(body):
    new, n = PUBLISH.subn("", body)
    if n != 1:
        a.die("tab.updated publish block matched %d times in one handler, want 1" % n)
    return new


def build(tree):
    rel = "backend/handlers/tab.go"
    text = a.read(tree, rel)
    for header in ("func (h *TabHandler) Update(", "func (h *TabHandler) Restore("):
        text = a.edit_in_go_func(text, header, strip_publish)
    a.write(tree, rel, text)

    rel = "backend/realtime/events.go"
    text = a.read(tree, rel)
    text = a.replace_once(
        text,
        "type TabUpdatedPayload struct {\n"
        "\tID         int    `json:\"id\"`\n"
        "\tPageID     int    `json:\"page_id\"`\n"
        "\tCategoryID int    `json:\"category_id\"`\n"
        "\tContent    string `json:\"content\"`\n"
        "}\n\n",
        "",
        "TabUpdatedPayload",
    )
    a.write(tree, rel, text)

    rel = ".agent/api/realtime.md"
    text = a.read(tree, rel)
    text = a.delete_line(text, "| `tab.updated` | Tab updated — [Tabs API](tabs.md) |", "tab.updated row")
    a.write(tree, rel, text)

    rel = "backend/tests/handlers/tab_realtime_leak_test.go"
    text = a.read(tree, rel)
    for name in ("TestTabUpdated_TargetsOnlyCategoryViewers", "TestTabRestored_TargetsOnlyCategoryViewers"):
        text = a.delete_test(text, name)
    a.write(tree, rel, text)

    a.no_match(tree, r"tab\.updated|TabUpdated", "backend")
    a.no_match(tree, r"tab\.updated", ".agent/api")


if __name__ == "__main__":
    build(a.tree_arg())
