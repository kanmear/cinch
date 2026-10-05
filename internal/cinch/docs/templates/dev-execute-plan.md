# Plan Execution

Use this to work through the tasks of an existing plan file under `{{paths.docs}}/plans/`.

Plans come from `{{paths.docs}}/workflows/dev-plan-feature.md` (`plans/<slug>.md`) and
`{{paths.docs}}/workflows/dev-fix-bug.md` (`plans/fix/<slug>.md`).

## Resuming

Read the plan's `## Handoff` section first: it names the task to continue and what is already
verified. If the previous task's changes are still uncommitted, the user hasn't reviewed them yet.
Wait for the commit or an explicit go-ahead before you start the next task.

## Per task, in dependency order

1. In a new session, run the project's test command (see AGENTS.md or the project's own docs)
   before you change anything, so a later failure is known to be yours.
2. When the task enforces a rule, write its test first and watch it fail.
3. Make the change. Run the task's tests, then the project's test command; both must pass.
4. Mark the task `**Status:** [x]`.
5. Propose a commit message. Don't run `git commit` unless the user asked you to.
6. Overwrite (don't append to) the plan's `## Handoff` section:

```markdown
## Handoff
**Last completed:** T<N> — <one line>
**Verified:** <what passed>
**Resume at:** T<N+1>
**Notes:** <anything not already in the plan; omit if none>
```

Stop at a task boundary, after writing the handoff, when the user asks you to or your context is
running low. Never stop mid-task.

## Commit messages

If `cinch.yml` sets `commit.pattern`, the subject must match it: `cinch check` rejects one that
doesn't. Otherwise follow the convention the project's docs state, or its recent `git log`.

## Completion

When every task is `[x]`:

- The project's full test suite passes and `cinch check` is green.
- If behavior or interfaces changed, run `{{paths.docs}}/workflows/docs-sync.md`.
- A feature plan stays, with `**Status:** complete`. A fix plan is deleted; git history is the
  archive.
