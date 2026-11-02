"""Exercises states/anchors.py on the fixtures beside this file. Prints one
line per check ("ok name") and exits non-zero at the first failure."""
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "..", "..", "states"))
import anchors as a


def read(name):
    with open(os.path.join(HERE, name), encoding="utf-8") as f:
        return f.read()


def check(name, cond):
    if not cond:
        sys.stderr.write("FAIL " + name + "\n")
        sys.exit(1)
    print("ok", name)


go = read("handler.go.txt")
out = a.delete_go_func(go, "func (h *ThingHandler) Update(")
check("delete_go_func removes the func and its doc comment", "Update renames" not in out and "h.things.Rename(1" not in out)
check("delete_go_func keeps UpdateAll", "func (h *ThingHandler) UpdateAll(" in out)
check("delete_go_func keeps the rest", "func helper() {}" in out)

out = a.edit_in_go_func(go, "func (h *ThingHandler) Update(", lambda body: body.replace('h.things.Rename(1, "x")', "h.things.Gone()"))
check("edit_in_go_func edits only its function", "h.things.Gone()" in out and "h.things.RenameAll()" in out)

rules = read("rules.md.txt")
out = a.delete_rule_item(rules, "THING-002")
check("delete_rule_item takes the item with continuation lines and the ignore comment", "THING-002" not in out and "cinch:ignore" not in out and "THING-003" in out)
check("rule_item returns the item", "continued on an indented line" in a.rule_item(rules, "THING-002"))

tests = read("tests.go.txt")
out = a.delete_test(tests, "TestRename_Succeeds")
check("delete_test takes the marker comments with it", "THING-001" not in out and "THING-002" not in out and "THING-003" in out)
moved = a.set_marker_comment(tests, "TestRename_Fails", a.marker_comment(tests, "TestRename_Succeeds"))
check("set_marker_comment moves a marker block", "THING-001" in moved and moved.count("THING-003") == 0)

out = a.replace_once("a b a", "b", "c")
check("replace_once on a single match", out == "a c a")

for name, fn in [
    ("replace_once refuses two matches", lambda: a.replace_once("a b a", "a", "c")),
    ("replace_once refuses no match", lambda: a.replace_once("a b a", "z", "c")),
    ("delete_go_func refuses an unknown func", lambda: a.delete_go_func(go, "func (h *ThingHandler) Nope(")),
    ("delete_line refuses an absent line", lambda: a.delete_line("x\ny", "z")),
]:
    try:
        fn()
    except SystemExit:
        print("ok", name)
    else:
        sys.stderr.write("FAIL " + name + "\n")
        sys.exit(1)
