"""T3 `email-invite`, arm 2 (bench/leak/BUILD.md § 4): arm 1 plus MEMB-024 (the
preventing rule), its repository helper and its marked model test, all read from
the fix commit f99130f at build time.

The task's own path stays as bare as in arm 1: no Invite handler, no route. The
rule's other markers (four handler tests in invite_test.go) cannot exist without
the endpoint, so MEMB-024 has one marker in this state. f99130f's text for MEMB-024
has no reference to MEMB-019 (arm 2 has no MEMB-019), so nothing is dropped from
it; the "temporary link" paragraph is added above the rule list as in the fix.
"""
import importlib.util
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import anchors as a

spec = importlib.util.spec_from_file_location("t3_arm1", os.path.join(HERE, "T3-arm1.py"))
arm1 = importlib.util.module_from_spec(spec)
spec.loader.exec_module(arm1)


def between(text, start, end, what):
    i = text.find(start)
    if i < 0 or text.count(start) != 1:
        a.die("%s: start anchor matched %d times" % (what, text.count(start)))
    j = text.find(end, i)
    if j < 0:
        a.die("%s: end anchor not found" % what)
    return text[i:j]


def build(tree):
    arm1.build(tree)
    fix = a.env("FIX_SHA")

    # MEMB-024 and the paragraph above the invite rules
    rel = ".agent/domain/members.md"
    src = a.show(fix, rel)
    item = a.rule_item(src, "MEMB-024")
    para = between(
        src,
        "The copyable invite link is a **temporary** delivery mechanism",
        "\n\n18. ",
        "temporary-link paragraph",
    )
    text = a.read(tree, rel)
    text = a.replace_once(
        text,
        "A *pending* account is one with no password yet (`password_hash IS NULL`).\n\n",
        "A *pending* account is one with no password yet (`password_hash IS NULL`).\n\n" + para + "\n\n",
        "pending definition",
    )
    last = a.rule_item(text, "MEMB-023")
    text = a.replace_once(text, last, last + "\n" + item, "MEMB-023 item")
    a.write(tree, rel, text)

    # repository: interface line and method
    rel = "backend/models/member.go"
    src = a.show(fix, rel)
    method = between(
        src,
        "func (r *memberRepository) HasMembershipUnmanagedBy(",
        "func (r *memberRepository) UpdateMemberRole(",
        "HasMembershipUnmanagedBy",
    )
    text = a.read(tree, rel)
    text = a.replace_once(
        text,
        "\tGetMemberByProjectAndUser(projectID, userID int) (*Member, error)\n",
        "\tGetMemberByProjectAndUser(projectID, userID int) (*Member, error)\n"
        "\tHasMembershipUnmanagedBy(userID, managerID int) (bool, error)\n",
        "interface",
    )
    text = a.replace_once(
        text,
        "func (r *memberRepository) UpdateMemberRole(",
        method + "func (r *memberRepository) UpdateMemberRole(",
        "UpdateMemberRole",
    )
    a.write(tree, rel, text)

    # the marked model test
    rel = "backend/tests/models/member_test.go"
    src = a.show(fix, rel)
    i = src.find("// cinch:rule MEMB-024\nfunc TestHasMembershipUnmanagedBy(")
    if i < 0 or src.count("// cinch:rule MEMB-024\nfunc TestHasMembershipUnmanagedBy(") != 1:
        a.die("TestHasMembershipUnmanagedBy not found exactly once in the fix commit")
    text = a.read(tree, rel)
    if not text.endswith("\n"):
        text += "\n"
    a.write(tree, rel, text + "\n" + src[i:])

    a.no_match(tree, r"MEMB-019", ".")
    a.no_match(tree, r"memberHandler\.Invite|MemberHandler\) Invite", "backend")


if __name__ == "__main__":
    build(a.tree_arg())
