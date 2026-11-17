"""T1 `tab-updated`, arms 1 and 3 (bench/leak/BUILD.md § 4): the base commit with
the backend's tab.updated event removed.

In backend/handlers/tab.go, Update and Restore lose the `targets :=
collectTargetUserIDs(...)` line and the `publishEvent(... "tab.updated" ...)`
statement after RespondJSON; backend/realtime/events.go loses TabUpdatedPayload;
.agent/api/realtime.md loses the tab.updated row. tab.created and tab.deleted keep
collectTargetUserIDs (the neighbour an agent copies), the whole frontend stays.
"""
import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import anchors as a

PUBLISH = re.compile(
    r"\ttargets := collectTargetUserIDs\(h\.members, projectID, user\.ID\)\n"
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

    a.no_match(tree, r"tab\.updated|TabUpdated", "backend")
    a.no_match(tree, r"tab\.updated", ".agent/api")


if __name__ == "__main__":
    build(a.tree_arg())
