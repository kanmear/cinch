# Plan Execution Workflow

Execute the tasks in an already-decomposed plan file — one atomic task per session, git
conventions enforced, and a Session Handoff trail so work resumes cold.

---

## When to run

Run this workflow any time a plan file's `## Tasks` section needs working through: starting
`T1` right after decomposition finishes, resuming after a break ("continue `<plan>.md`"), or
picking up where any other session left off. This is the **only** entry point for implementing
`T<N>` tasks — even immediately after `.agent/workflows/plan-feature.md` or
`.agent/workflows/fix-bug.md` finishes decomposition in the same conversation, task execution
routes through here rather than continuing inline. Applies to both plan locations:

- `.agent/plans/<feature-slug>.md` (from `.agent/workflows/plan-feature.md`)
- `.agent/plans/fix/<bug-slug>.md` (from `.agent/workflows/fix-bug.md`)

`.agent/workflows/fix-bug.md`'s fast path is exempt from the session machinery below — it's scoped
to one session with no persisted plan file — but its single commit still follows the Commit
Conventions section here.

---

## Session Boundaries

Designed to run across as many sessions as the running agent's context budget calls for — judged on
the spot, not calibrated to a fixed token count. A large-context agent (e.g. a frontier hosted
model) will cross far fewer of these boundaries than a small local model; the markers are the same
either way. Two markers:

- **CHECKPOINT** — pause for human confirmation (carried over from a planning-phase checkpoint,
  or raised mid-task if something unanticipated turns up). Once confirmed, treat it as a hard stop
  rather than rolling into the next task in the same context window.
- **SESSION STOP** — propose a commit message, update the plan file's
  `## Session Handoff` section, and end the session — leaving the commit itself for the user to
  review and run manually.

A third event isn't a marker the workflow raises — it's a mid-session summarization the running
agent's own context management triggers unasked, potentially erasing this file's instructions
before a task finishes. If that happens, reload `.agent/workflows/task-primitive.md` §
Compaction anchor before continuing — the current task ID, its context manifest, and the
completion ritual come from source, not from what the compacted summary retained.

**Session Handoff template** (overwrite, don't append, at every stop):

```markdown
## Session Handoff
**Last completed:** <Phase N / Task T<N>> — <one-line what was done>
**Verified:** <which tier(s) passed>
**Resume at:** <Phase N+1 / Task T<N+1>>
**Notes for next session:** <anything transient not already in the plan file — omit if none>
```

Resuming: read the plan file's `## Session Handoff` section first — it names the exact task to
continue and what's already verified. Don't re-derive it from the Tasks checklist alone. If the
previous task's changes are still staged and uncommitted, that means the user hasn't reviewed them
yet — resolve that first (wait for the commit, or the user's explicit go-ahead) before starting the
next task.

---

## Commit Conventions

Load `.agent/conventions.md` § Git before proposing the first commit message of the session,
unconditionally. It is **not** subject to the per-task manifest-minimization rule in
`.agent/workflows/task-primitive.md` § Context manifest — those manifests are deliberately narrow
to protect context budget for a single task; this is a fixed rule of the execution workflow itself,
not a task-specific doc, so it doesn't get squeezed out.

Gotcha: **`[brackets]`, never `(parens)`** for scope — this is not Conventional Commits; don't
default to `type(scope): description` from general training. Commit body is optional (prefer a
single-line subject; add a short body only for a non-obvious decision that would otherwise be
lost — Session Handoff remains the primary place for task context).

Compliant shape (for calibration — `<scope>` is this project's own vocabulary, e.g. a service
or package name):

```
feat [<scope>]: <short description>
fix [<scope>]: <short description>
docs [<scope>]: <short description>
```

---

## Per-task execution loop

For each `T<N>` in dependency order:

1. Read `## Session Handoff` → `Resume at:` to find the task.
2. Confirm the previous task's Tier 3 still passes (skip for the first task).
3. Load **only** that task's `Context` manifest — the Commit Conventions section above is already
   in effective context for the whole session and isn't part of any task's manifest.
4. Run the task's Verify tiers in order, per `.agent/workflows/task-primitive.md` § Verification
   tiers (Tier 1 → Tier 2 → Tier 3). Some tasks defer Tier 3 to a later regression-checkpoint task —
   follow that task's own Verify spec.
5. Mark the task's `**Status:**` `[x]` in the plan file.
6. Propose a commit message (per conventions.md § Git). **Do not run `git commit`.** The user reviews the diff and commits manually.
7. Update `## Session Handoff` (template above).
8. **SESSION STOP** — end the session, unless the user explicitly asks to continue further
   tasks in the same session.

Once every task is `[x]`, run the Completion Ritual in `.agent/workflows/task-primitive.md` §
Completion ritual — using the plan's lifecycle (feature: plan persists as `complete`; fix: plan
file deleted). Not duplicated here.
