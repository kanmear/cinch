"""T3 `email-invite`, arms 1 and 3 (bench/leak/BUILD.md § 4): the base commit
d2fb210 with the invite endpoint and its rules removed. This state never existed
as a commit (MEMB-018/019 were written before the endpoint, by the same plan).

Removed: the POST .../members/invite route; MemberHandler.Invite and its request
and response types; every test of backend/tests/handlers/invite_test.go that
calls the invite endpoint (the accept tests stay) and the helpers only they used;
MEMB-018 and MEMB-019 (their markers were all on deleted tests); MEMB-006 reverts
to its 210c6de text; the invite section of .agent/api/members.md; the plan
.agent/plans/email-invites.md. Links to the removed rules are fixed.

Kept: migration 00003, UserRepository (CreatePendingUser, IssueInvite, ...),
auth.go accept and setup, the whole frontend, MemberHandler.Add, NewMemberHandler
with its signature, MEMB-020..023 and the pending-account definition.
"""
import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import anchors as a
import anchors_t3 as b

INVITE_TESTS = [
    "TestInvite_NewEmail_CreatesPendingAccountAndMember",
    "TestInvite_ActiveNonMember_PlainAddWithoutLink",
    "TestInvite_PublishesMemberAdded",
    "TestInvite_Validation",
    "TestInvite_Twice_RotatesToken",
    "TestInvite_PendingAlreadyMember_ReissuesWithoutDuplicate",
    "TestInvite_ActiveAlreadyMember_Returns409",
    "TestInvite_CrossProjectRole_Returns404",
    "TestInvite_OwnerRole_Returns403",
    "TestInvite_PlainMember_Returns403",
    "TestInvite_Admin_Succeeds",
    "TestInvite_StoresOnlyTokenHash",
]
INVITE_HELPERS = ["func invitePath(", "func inviteTokenFromURL(", "func postInvite("]

MEMB006_OLD = (
    "6. **MEMB-006** All other members are added explicitly via the Members API — by user ID or by email invite (#18–23). There is no open-join flow.\n"
    "   <!-- cinch:ignore: asserts the absence of a flow; no test can enforce a nonexistent endpoint -->"
)
MEMB006_210C6DE = (
    "6. **MEMB-006** All other members are added explicitly via the Members API — there is no open-join or invitation flow in the current system.\n"
    "   <!-- cinch:ignore: asserts the absence of a flow; no test can enforce a nonexistent endpoint -->"
)


def build(tree):
    # routes
    rel = "backend/handlers/routes.go"
    text = a.read(tree, rel)
    text = a.delete_line(
        text,
        'mux.HandleFunc("POST /api/projects/{projectID}/members/invite", auth(manager("projectID", memberHandler.Invite)))',
        "invite route",
    )
    a.write(tree, rel, text)

    # handler
    rel = "backend/handlers/member.go"
    text = a.read(tree, rel)
    text = a.delete_go_func(text, "func (h *MemberHandler) Invite(")
    text = a.delete_go_func(text, "type inviteMemberRequest struct {")
    text = a.delete_go_func(text, "type inviteMemberResponse struct {")
    text = b.prune_unused_imports(text)
    a.write(tree, rel, text)

    # tests: the invite endpoint's tests and the helpers only they used
    rel = "backend/tests/handlers/invite_test.go"
    text = a.read(tree, rel)
    for name in INVITE_TESTS:
        text = a.delete_test(text, name)
    for header in INVITE_HELPERS:
        text = a.delete_go_func(text, header)
    text = b.prune_unused_imports(text)
    a.write(tree, rel, text)

    # rules
    rel = ".agent/domain/members.md"
    text = a.read(tree, rel)
    text = a.delete_rule_item(text, "MEMB-018")
    text = a.delete_rule_item(text, "MEMB-019")
    text = a.replace_once(text, MEMB006_OLD, MEMB006_210C6DE, "MEMB-006")
    a.write(tree, rel, text)

    # docs
    rel = ".agent/api/members.md"
    text = a.read(tree, rel)
    text = b.delete_md_section(text, "## POST /api/projects/{projectID}/members/invite")
    a.write(tree, rel, text)

    rel = ".agent/models/user.md"
    text = a.read(tree, rel)
    text = a.replace_once(text, "MEMB-018 – MEMB-022", "MEMB-020 – MEMB-022", "user.md rule range")
    a.write(tree, rel, text)

    a.remove(tree, ".agent/plans/email-invites.md")

    # post-conditions
    a.no_match(tree, r"MEMB-018|MEMB-019", ".")
    a.no_match(tree, r"memberHandler\.Invite|inviteMember|MemberHandler\) Invite", "backend")
    a.no_match(tree, r"members/invite", "backend")
    a.no_match(tree, r"members/invite", ".agent")
    a.no_match(tree, r"email-invites", ".agent")


if __name__ == "__main__":
    build(a.tree_arg())
