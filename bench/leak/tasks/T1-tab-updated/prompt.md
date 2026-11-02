Add a `tab.updated` realtime event to the backend.

The frontend already handles a server `tab.updated` event (`frontend/src/lib/runtime/project/realtime_protocol.ts`), but the backend never sends one, so a page that is open doesn't show an edit to one of its tabs until it is reloaded.

- Publish `tab.updated` after a tab's content is successfully updated (`PATCH /api/projects/{projectID}/tabs/{id}`) and after a tab is successfully restored from its history (`POST /api/projects/{projectID}/tabs/{id}/history/{historyID}/restore`).
- The payload is the tab after the change, as the frontend expects it: `{"id", "page_id", "category_id", "content"}`. Add the payload type next to the other event payloads in `backend/realtime/events.go`.
- A failed update or restore publishes nothing.

Nobody is available to answer questions in this session. Where a workflow asks you to confirm something with the user, decide yourself, say what you decided in your final message, and carry on until the change is implemented.

Follow the repository's conventions. Add or update tests. Commit your work on the current branch when done.
