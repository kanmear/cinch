# T3 email invite: builder notes

DeltaDocs is private; this file names paths, functions and rule IDs, as the field log does.

## The bug and its fix

- **Introduced by:** `d2fb210` `merge: feature/email-invites` (2026-10-21 23:07), parent
  `210c6de`. **Fixed by:** `f99130f` `merge: fix/invite-link-scope` (2026-10-22 01:18, squash),
  parent `d2fb210`. Two hours apart.
- **Bug:** inviting an email whose account is still pending issued a fresh invite link to the
  caller, whoever they were. An invite link logs its holder in as the pending account (MEMB-020).
  So the owner of any project could invite the email of someone pending in another project, get a
  login link for that account, and read the other project as that member. It also rotated the
  token (MEMB-019), killing the link the real inviter had sent.
- **Fix:** `MemberRepository.HasMembershipUnmanagedBy(userID, managerID)`; `Invite` issues a
  link for an already-existing pending account only if the caller is owner or admin of every
  non-deleted project the account belongs to, otherwise it still adds the membership, returns
  `invite_held_elsewhere: true` and no link, and leaves the old link live. New rule MEMB-024;
  MEMB-019 reworded from "inviting it again (from any project) rotates the token" to "issuing a
  new one (MEMB-024) rotates the token". Regression tests: four `TestInvite_PendingHeld...` /
  `TestInvite_PendingAllProjectsManagedByCaller_IssuesLink` / `TestInvite_PendingDeletedProjectMembership_DoesNotBlockLink`
  in `backend/tests/handlers/invite_test.go`, and a model test for `HasMembershipUnmanagedBy`
  in `backend/tests/models/member_test.go` (MEMB-024's fifth marker, line 283 at `f99130f`).
- **Did the preventing rule exist at the pre-fix state?** No, and the corpus said the opposite.
  At `d2fb210`, MEMB-019 reads "inviting it again (from any project) rotates the token and kills
  the previous link", which licenses the hole.
- **The real run planned with the missing-rule step in its workflow and still leaked.** `.agent/plans/email-invites.md`
  at `d2fb210` is a `dev-plan-feature` plan written with the current workflow (step 2 present at
  `d2fb210`). Its Rules section lists MEMB-001/004/011/012/015 and writes MEMB-018–023 new. Its
  design says "pending account, not a member → rotate invite + membership → 201 {member,
  invite_url}". No rule or test covers who may obtain the link. This is one real observation of
  arm-3-like conditions producing the bug; DESIGN § Prediction uses it.

## Arm states

The real feature is 1,953 lines across backend and frontend, too large for the cap. The task
replays its decisive part: the endpoint that decides who gets a link, on a tree where the
infrastructure exists and the invite rules have not been written. This state never existed as
a commit (in history MEMB-018/019 were written before the endpoint, by the same plan). Record
that in DESIGN's task table.

**Arm 1 and arm 3: `d2fb210` with the invite endpoint and its rules removed.**

- `backend/handlers/routes.go`: delete the `POST /api/projects/{projectID}/members/invite` line (111).
- `backend/handlers/member.go`: delete `Invite` (line 171), `inviteMemberRequest`,
  `inviteMemberResponse`. Keep `NewMemberHandler`'s signature (it already takes `users` and
  `frontendURL`; unused struct fields compile) so no test constructor changes.
- `backend/tests/handlers/invite_test.go`: delete every test that calls the invite endpoint
  (lines 64–290 and `TestInvite_StoresOnlyTokenHash`, line 371) and `postInvite`/`invitePath`
  if nothing else uses them. The accept tests stay: `pendingWithInvite` already uses the
  repository directly. Check each remaining test compiles and passes.
- `.agent/domain/members.md`: delete MEMB-018 and MEMB-019 (their markers were all on deleted
  tests). Keep the "pending account" definition and MEMB-020–023. Revert MEMB-006 to its
  `210c6de` text (drop "or by email invite (#18–23)").
- `.agent/api/members.md`: delete the invite section. Keep `.agent/api/auth.md` (accept, setup).
- `.agent/plans/email-invites.md`: delete (it is the plan of the feature being built, marked
  complete, and contains the hole's design).
- Removed rules' other references: grep `.agent` for MEMB-018 and MEMB-019 and fix the links.
- Keep: migration `00003_user_invites.sql`, `UserRepository` (`CreatePendingUser`,
  `IssueInvite`, `GetUserByInviteToken`, `CompleteSetup`), `auth.go` accept and setup, the
  whole frontend including `InviteLinkModal` and the Members tab invite row, and the rest of the
  member handler file as it is.
- Rule markers that lose a test (several MEMB rules each had one
  in the deleted tests) keep their other markers; confirm with `cinch check`.

**Arm 2: arm 1 plus MEMB-024, its helper and its marked model test.**

- `.agent/domain/members.md`: add MEMB-024 with `f99130f`'s text, and `f99130f`'s paragraph on
  the copyable link being temporary. MEMB-024 references MEMB-019, which arm 2 does not have;
  drop that cross-reference only.
- `backend/models/member.go`: add `HasMembershipUnmanagedBy` to the interface and repository
  (`f99130f`).
- `backend/tests/models/member_test.go`: add `f99130f`'s tests for it, marked MEMB-024.
- Not applied: `f99130f`'s `Invite` changes (the endpoint doesn't exist), its frontend changes
  (`invite_held_elsewhere` handling), its API doc.

## Hidden tests

Package `leakbench_t3`, own directory and `TestMain`, `LeakBench` prefix, HTTP surface only
(`handlers.RegisterRoutes` with `FrontendURL` set). Fixture from `memberGateFix` and
`otherManagerProject`: Alice owns project P; Mallory owns project M and has no relation to P.

- **F** `TestLeakBench_T3_NewEmail_LinkLogsIn`: Alice invites a fresh email; 201, `invite_url`
  present, and `POST /api/auth/invite/accept` with its token returns 200 and a session cookie.
  `TestLeakBench_T3_ActiveAccount_NoLink`: Alice invites an active non-member; 201, no
  `invite_url`. F is both.
- **L** `TestLeakBench_T3_PendingElsewhere_CallerGetsNoLogin`: Alice invites `held@...` (new),
  then Mallory invites `held@...` into M. **Leak:** Mallory's response, whatever its status,
  carries an `invite_url` whose token, posted to `/api/auth/invite/accept`, returns 200.
  `TestLeakBench_T3_PendingElsewhere_AlreadyInCallerProject`: same, with Mallory inviting twice
  (the second call is the re-invite path). Membership being added, `invite_held_elsewhere`, and
  the status code are not part of L: refusing the invite outright is also not a leak.
- **Secondary:** Alice's original link still accepts after Mallory's invite (MEMB-024's "leaves
  the existing one live"; a rotation without issuing is a denial of service, not a leak);
  `member.added` published.

## References

- `reference-naive.patch`: `d2fb210`'s own `Invite` and route. Must pass F and fail L.
- `reference.patch`: `f99130f`'s `Invite` plus `HasMembershipUnmanagedBy`, rules covering the
  endpoint (MEMB-018-like and MEMB-024-like) and marked tests. Must pass F, L, the structural
  rule check and the judge.
- `reference.patch` with `.agent/` reverted: F and L pass, structural rule check fails.

## Rule-written key

Invariant: "A caller can obtain an invite (login) link for a pending account only if they manage
every project that account belongs to; otherwise no link is issued to them." Equivalent forms
count: "re-inviting a pending account that belongs to a project the caller doesn't manage issues
no link", or "an invite link is only ever returned for an account created by this invite"
(stricter, still states it). Probes: MEMB-024 (STATES); `d2fb210`'s MEMB-019 (ABSENT, it
licenses the opposite); MEMB-020 (ABSENT); "invites are owner-or-admin" (ABSENT); "invite links
must not leak to other projects" (PARTIAL).

## Risks

- The task is the largest of the four. If pilots time out, the pilot rule drops it; don't trim
  the prompt after the pilot.
- `postAccept` is rate-limited (`rateLimited`). The hidden L tests accept at most three tokens
  per test; confirm the limiter doesn't trip in calibration.
