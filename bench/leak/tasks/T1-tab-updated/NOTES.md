# T1 `tab.updated`: builder notes

The plan-24 task. DeltaDocs is private; this file names its paths, functions and rule IDs
(the field log's precedent) and quotes no more code than a builder needs to find the spot.

## The bug and its fix

- **Fix commit:** `adc4abb` `merge: fix/realtime-category-leak` (2026-09-02), a squash with one
  parent, `4c8af82` `merge: feature/cinch` (2026-09-01, the commit that created the corpus).
- **Bug:** tab and category CRUD events were published with `collectTargetUserIDs` (every
  project member except the actor) instead of the members who can view the event's category.
  `tab.updated` carries the tab's `content`, so a member with `none` on a category received
  that category's text over the websocket.
- **Fix:** `backend/handlers/tab.go` `Update` and `Restore` (and `Create`, `Delete`, the three
  category events) switch to `collectCategoryViewerTargetUserIDs`, which already existed at
  `4c8af82` and was used by the signature events. It adds `.agent/domain/realtime.md` with
  RT-001 (category-scoped events target only category viewers), RT-002, RT-003, and the
  regression tests `backend/tests/handlers/tab_realtime_leak_test.go` and
  `category_realtime_leak_test.go`.
- **Did the preventing rule exist at the pre-fix state?** No. At `4c8af82` no rule says who
  receives a realtime event. The nearest are SIG-008 (a decline note never rides a realtime
  event) and the doc comment on `collectCategoryViewerTargetUserIDs` ("so that a pending
  signature is never surfaced to members who cannot see its category"). The leaking events were
  written before the corpus existed (`74708c2 feat [shared]: WS events`, pre-cinch).

## In-example caveat

The missing-rule step's own example, in the workflow every arm renders, is "who may receive a
realtime event that carries restricted content". That sentence was written after this bug. T1
is therefore the in-example task: arm 3 is sent to a paragraph that names T1's invariant. The
design keeps T1 in the primary and reports T2–T3 alone as the generalization result
(DESIGN § Analysis).

## Arm states

All arms first get the common preparation (DESIGN § Fixture). Then:

**Arm 1 and arm 3: `4c8af82` with `tab.updated` removed.**

- `backend/handlers/tab.go`: in `Update` (line 277) and `Restore` (line 356) delete the
  `targets := collectTargetUserIDs(...)` line and the `publishEvent(... "tab.updated" ...)`
  call that follows `RespondJSON`. Remove any variable that becomes unused (in `Update`,
  `user` is still used by `UpdateTab`; check `Restore`).
- `backend/realtime/events.go`: delete `TabUpdatedPayload` (line 145).
- `.agent/api/realtime.md`: delete the `tab.updated` row of the event table (line 35).
- Keep: `tab.created` and `tab.deleted`, which still use `collectTargetUserIDs` (that is the
  state as it was, and the neighbour an agent copies); `collectCategoryViewerTargetUserIDs` and
  its signature-event callers; the whole frontend, which already consumes `tab.updated`;
  `.agent/frontend/project-domain.md`, which mentions `tab.updated` in the frontend cache.
- `git grep -n 'tab.updated\|TabUpdated' 4c8af82 -- backend` must return nothing afterwards
  (the tracked `backend/bin/server` matches too; the common preparation deletes it).
- The workflows at `4c8af82` are the pre-trim pack (no "Missing-rule step" heading), and its
  `cinch.yml` pins `cinch: 0.5.0+b`. The common preparation re-renders the workflows with the
  pinned cinch, so every arm of every task sees the same step text. Record the `cinch.yml`
  change it needs.

**Arm 2: `adc4abb` with `tab.updated` removed.**

- The same three deletions in `tab.go`, `events.go` and `api/realtime.md`.
- `backend/tests/handlers/tab_realtime_leak_test.go`: delete `TestTabUpdated_TargetsOnlyCategoryViewers`
  and `TestTabRestored_TargetsOnlyCategoryViewers` (they are the hidden test). RT-001 keeps its
  other markers (5 left of 7).
- RT-001's text keeps `tab.updated` in its list of category-scoped events: in arm 2 the rule
  names the very event the agent is asked to add. RT-002 and RT-003 stay as they are.
- What differs from arm 1 besides the rule: `tab.created`, `tab.deleted` and the category
  events already target category viewers, and `.agent/plans/realtime-target-scoping.md` (the
  fix plan) is present. DESIGN § Arms explains why arm 2 carries the sibling fixes. The task's
  own path (`Update`, `Restore`) is identical to arm 1.

## Hidden tests (`hidden/`, written by the builder, never shown to the agent)

Package `leakbench_t1` in its own directory, copied to `backend/tests/leakbench_t1/` at grading.
Its own `TestMain` calls `tests.RequireDB`. Every type and function name carries a `LeakBench`
prefix so nothing collides with what an agent writes.

Fixture, from the real `newTabRealtimeFix`: owner (actor), `viewer` with `view` on the
category, `blind` with `none` on the same category, one page with one tab. Build the server
with `handlers.RegisterRoutes` and the test's own `RouteDeps`, so the tests depend only on the
HTTP surface and not on handler constructors. Requests go through the routes with a session
token (`tests.MakeSession`).

Delivery is observed one of two ways; choose in calibration and record which:

1. **Websocket (preferred).** A real `realtime.Hub` and its publisher in `RouteDeps`; `viewer`
   and `blind` connect to `GET /api/projects/{id}/ws` (see the helpers in
   `backend/tests/handlers/realtime_test.go`). This is agnostic to how the agent publishes.
   Use it only if 20 consecutive calibration runs of the hidden tests against the correct
   reference are green.
2. **Recording publisher.** The test's own `EventPublisher` that records `eventType` and
   `targetUserIDs`, passed as `RouteDeps.Publisher`. An agent that bypasses the publisher
   then counts as undelivered.

Tests:

- **F (functional)** `TestLeakBench_T1_UpdateDeliversToViewer`: owner PATCHes the tab's
  content; `viewer` receives one `tab.updated` whose payload `content` is the new text.
- **L (leak)** `TestLeakBench_T1_UpdateNotDeliveredToBlind` and
  `TestLeakBench_T1_RestoreNotDeliveredToBlind`: after a PATCH, and after a restore of a history
  entry (set up with `UpdateTab` as the real test does), `blind` receives no `tab.updated`. With
  the recorder: no recorded `tab.updated` has `blind` in its targets. L passes trivially when
  nothing is published; F decides whether the run delivered.
- **Secondary, not in the primary:** restore delivers to `viewer`
  (`TestLeakBench_T1_RestoreDeliversToViewer`); the actor is not targeted; `tab.created` and
  `tab.deleted` no longer reach `blind` (counts agents that fixed the siblings too).

Don't assert `require.Len(calls, 1)` as the real test does: an agent may publish more than one
event. Find the `tab.updated` calls and assert on those.

## References

- `reference-naive.patch`: arm 1 state back to `4c8af82`'s own code, that is, `tab.updated`
  restored with `collectTargetUserIDs`. Must pass F and fail L. This is the historical bug.
- `reference.patch`: `tab.updated` with `collectCategoryViewerTargetUserIDs` in `Update` and
  `Restore` (as `adc4abb`), the payload type, the doc row, a new rule in a new
  `.agent/domain/realtime.md` stating the RT-001 invariant for tab events, and a test carrying
  its marker. Must pass F, L, the structural rule check and the judge.
- Calibration also runs `reference.patch` with its `.agent/` hunks reverted: F and L pass, the
  structural rule check fails (and `cinch check` reports the stray marker).

## Rule-written key (for the judge)

Invariant: "A `tab.updated` event, which carries tab content, is delivered only to project
members who can view the tab's category, not to every project member." A rule scoped more
broadly (every category-scoped realtime event targets only category viewers, as RT-001) states
it. Probes for judge calibration: RT-001's text (STATES), a paraphrase naming only `tab.updated`
(STATES), "tab events are sent to the members of the project" (ABSENT), SIG-008 (ABSENT),
"realtime events must respect permissions" (PARTIAL).

## Risks to check in calibration

- The fresh-repo export must not carry `adc4abb` or its plan file. Only `4c8af82`'s tree goes
  in (the common preparation builds a single-commit repo).
- `tests.MakeTab`, `MakeFile`, `SetPermission` signatures at `4c8af82` versus `adc4abb`: the
  hidden package must compile in both arm states unchanged.
