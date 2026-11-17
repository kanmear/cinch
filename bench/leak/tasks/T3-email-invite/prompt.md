Add inviting people by email to the Members API.

Everything around it already exists: pending accounts and invite tokens (`UserRepository.CreatePendingUser`, `UserRepository.IssueInvite`), the accept and setup endpoints (`POST /api/auth/invite/accept`, `POST /api/auth/setup`), and the frontend (the invite row in the Members tab and `InviteLinkModal`, which call `inviteMember` in `frontend/src/lib/api/members.ts`). What's missing is the endpoint that sends an invite.

- `POST /api/projects/{projectID}/members/invite` with `{"email": "...", "role_id": N}`. The project owner or an admin, the same gate as adding a member.
- Validate the email the way signup does: 400 `AUTH_EMAIL_PASSWORD_REQUIRED` when it is missing, 400 `AUTH_EMAIL_TOO_LONG`, 400 `AUTH_INVALID_EMAIL`. A missing `role_id` is 400 `API_INVALID_REQUEST`, and a role that isn't one of this project's is 404 `ROLE_NOT_FOUND`.
- If no account has that email, create a pending account, add it to the project with the role, and respond 201 `{"member": {...}, "invite_url": "<FRONTEND_URL>/invite/<token>"}`, where the token comes from `IssueInvite`. There is no email sending yet: the inviter copies the link and sends it to the person.
- If the email belongs to an account that has finished setup, the invite is a plain member add: 201 `{"member": {...}}` with no `invite_url`, or 409 `MEMBER_ALREADY_EXISTS` if that account is already a member.
- Publish `member.added` whenever a membership is created.

Nobody is available to answer questions in this session. Where a workflow asks you to confirm something with the user, decide yourself, say what you decided in your final message, and carry on until the change is implemented.

Follow the repository's conventions. Add or update tests. Commit your work on the current branch when done.
