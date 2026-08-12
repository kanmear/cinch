# Task Decomposition Primitive

The shared machinery for turning a confirmed plan into atomic, independently verifiable tasks.
`dev-plan-feature.md` and `dev-fix-bug.md` both compose this — they supply their own Phase 1/2 front
(domain rules + test plan / reproduce + root cause), then hand off here for decomposition,
verification, and completion. `dev-execute-plan.md` runs the tasks this produces.

This file is the single home for atomicity, context manifests, verification tiers, the test-tier
taxonomy, the task template, the pre-flight gate, and the completion ritual — so a generalization
(a new binding, a new tier) is made once, not copied across three workflows.

---

## Test tiers

Test tiers are **data, not prose baked into a procedure** — they come from
`manifest.taxonomy.test_tiers`. Each tier row declares its `id`, TDD policy (`tdd`), the command
that runs it (`cmd`, resolved against `development.commands`), and whether it is `opt_in`. A repo on
a different stack redefines its tiers in the manifest; everything below is unchanged.

For this repo's resolved tier bindings — which layer each tier enforces, its TDD policy, and the
command that runs it — read `cinch_manifest`'s `taxonomy.test_tiers` directly rather than a copy
here; that block is the single source and each row is self-describing (`id`, `tdd`, `cmd`,
`opt_in`, `automatable`).

Read TDD policy and opt-in status from the manifest row, not from memory of an older
classification.

**`opt_in` is a completion-gate signal, honored from data.** An opt-in tier is never a required
gate and runs only on explicit request — so any agent, whatever its own house rules about (say)
browser automation, can respect it mechanically without a human spelling it out.

**e2e tier / `manual` vs `e2e`:** see `manifest.taxonomy.test_tiers` `notes:` — the e2e row's
operational doc is findable via `cinch index`.

---

## Atomicity

A task is atomic when:

- It implements **one concern** with one rollback boundary.
- It is **independently verifiable** when complete.
- Its declared layers come from `manifest.taxonomy.layers` (`backend-model, backend-handler, frontend-state, frontend-ui`).
- Its working context fits one focused pass.

Prefer one layer per task. Split work when each layer can reach a meaningful passing state on its
own — for example, add a model query, then call it from a handler, then consume it in UI state.

Span layers only when splitting would create a non-compiling, non-runnable, or otherwise
unverifiable intermediate state. The combined task must still represent one concern, have one
rollback boundary, declare every affected layer, explain why the layers cannot pass independently,
and run regression checks for all of them. "These changes belong to the same feature" is not a
sufficient rationale.

If a task feels too large to complete in one focused pass, split it further. This is judged on the
spot against the running agent's context budget (see Context manifest) — not a fixed task count.

**Examples.**

- "Implement cascade-on-delete end-to-end" should normally split into a model/query task, a handler
  task, and a UI-state task because each can be verified independently.
- "Rename a shared contract field and update the generated consumer type" may stay together when
  either half alone makes the repository fail to compile. It is one contract transition, declares
  both layers, and runs both layers' checks.

**e2e specs span layers by nature** and remain a dedicated final concern in their own opt-in tier,
not part of an implementation task. Author a feature's e2e specs in a dedicated final task (or
append them any time); auto-discovery means no wiring task is ever needed.

---

## Context manifest

For each task, list the `{{paths.docs}}/` docs to load — **and nothing else**.

The constraint is a **context budget the running agent judges on the spot**, not a fixed count and
not a fixed number of tokens. A task's full working context includes AGENTS.md, the system prompt,
the manifest docs, and the source files being changed — all of which must fit alongside each other.
A large-context agent has room for more; a small local model has room for less. Use as many or as
few docs as the task genuinely needs — but if the manifest starts feeling like "load everything,"
that's a sign the task is too large and should be split, whatever the agent.

For each doc in the manifest: include it only if omitting it would force the implementer to guess at
something; include it if a decision hinges on it. List in loading order with a one-line rationale.

**Good manifest (single-layer model task):**

```
1. {{paths.docs}}/<entity>.md — struct and existing queries
2. {{paths.docs}}/<domain>.md — rules this task must enforce
3. {{paths.docs}}/<service>/conventions.md — query patterns (repository interface)
```

For a justified multi-layer task, include the specific contract and convention docs needed for
each declared layer. The atomicity rationale permits relevant cross-layer context, not broad
orientation material.

**Bad manifest (irrelevant docs crowd out source files):**

```
- {{paths.docs}}/<service A>/architecture.md
- {{paths.docs}}/<service A>/conventions.md
- {{paths.docs}}/<service B>/architecture.md   ← irrelevant to a <layer A> task
- {{paths.docs}}/<service B>/conventions.md    ← irrelevant to a <layer A> task
- {{paths.docs}}/overview.md       ← too broad; load the specific domain file
- {{paths.docs}}/<entity>.md
- {{paths.docs}}/<domain>-api.md               ← irrelevant at the model layer
```

The problem isn't the count — it's that half these docs don't apply to the task's concern or
declared layers. Each irrelevant doc crowds out context the source files need.

---

## Verification tiers

Each task has three verification tiers that catch different failure classes:

| Tier | What to run | What it catches |
| ------ | ------------ | ---------------- |
| Tier 1 | Test compiles and **fails** without the change | Proves the test tests the right thing |
| Tier 2 | Test **passes** after the change | Proves the change is correct |
| Tier 3 | applicable tier `cmd` for every affected layer, or the project-wide check when layers cannot be isolated | Proves nothing else broke |

Tier 1 applies only to TDD-mandatory tests (per the tier's `tdd` policy). For optional tests written
after the implementation, Tier 2 and Tier 3 are sufficient.

For a multi-layer task, list one Tier 3 command per affected layer. Consolidate duplicate commands;
if the manifest exposes only a project-wide command that covers the declared layers, list it once
and state that scope.

**Dependencies.** Record which tasks each task depends on. Tasks with no dependency can in principle
run in parallel; within a single pass, work in dependency order.

---

## Task template — fill in the plan file's `## Tasks` section

```markdown
### T<N>: <title>
**Status:** [ ]
**Depends on:** T<M> | —
**Layers:** <one or more values from backend-model, backend-handler, frontend-state, frontend-ui>
**Atomicity rationale:** <why this is one independently verifiable concern; for multiple layers,
why they cannot pass independently>

**Context** (load in order, nothing else):
1. {{paths.docs}}/xxx — rationale
2. {{paths.docs}}/yyy — rationale

**Files:**
- `path/to/file.go` — what to add/change/fix
- `path/to/file_test.go` — test to write first (if TDD-mandatory)

**Verify:**
- Tier 1: test compiles/runs and fails without the change
- Tier 2: <tier cmd> (or specific test name) — test passes
- Tier 3: <tier cmd(s)> — regression for every affected layer

**Checklist:**
- [ ] Previous task's Tier 3 still passes
- [ ] Layers and atomicity rationale confirmed
- [ ] Context docs loaded (and nothing else)
- [ ] Test written first — Tier 1 passes (compiles/runs, fails)  ← TDD-mandatory tasks only
- [ ] Implementation written — Tier 2 passes
- [ ] Affected-layer regression — Tier 3 passes
- [ ] Task marked done
```

Task execution itself — working through `T1`, `T2`, … — happens via the dev-execute-plan workflow
(`{{paths.docs}}/workflows/dev-execute-plan.md`), not inline here.

---

## Pre-flight gate

Do not write the first task until:

- [ ] Every planning-phase ⛔ checkpoint the calling workflow defines has been cleared with the user
- [ ] New/clarified domain rules written via the rules workflow (`{{paths.docs}}/workflows/docs-maintain-domain.md`)
      — not inlined in the plan
- [ ] Every task is one independently verifiable concern; any multi-layer task explains why its
      layers cannot pass independently
- [ ] Every task's context is scoped to its concern and declared layers (if it feels like "load
      everything," split it)
- [ ] The plan file's front sections (rules/root-cause, test plan, Tasks) are all filled in
- [ ] Plan saved to its location with status `in-progress`

The calling workflow names its specific checkpoints (feature: rules + test plan; bug: reproduce +
root cause + regression plan) and its plan-file location.

---

## Completion ritual

After all tasks are `[x]`, add a `## Completion` section and close the loop with the workflow suite.
The required checks are shared; the **plan-file lifecycle differs by plan type** (parameter below).

```markdown
## Completion
[ ] `make test` — full regression
[ ] `make check` — typecheck + lint clean
[ ] coverage audit (`{{paths.docs}}/workflows/docs-audit-coverage.md`) — every domain rule has a test
[ ] doc sync (`{{paths.docs}}/workflows/docs-sync.md`) — update {{paths.docs}}/ docs if any interfaces changed
[ ] (opt-in) run each `taxonomy.test_tiers` row with `opt_in: true` via its `cmd` key when the work
    touched the flows it covers (auth, cross-domain happy paths). Needs its own prerequisites —
    see the tier's `notes` doc.
[ ] <plan-type lifecycle step — see below>
```

Do not declare the work done until the required (non-opt-in) checks pass. Opt-in tiers are run at
discretion, never a required gate.

**Plan-type lifecycle:**

- **Feature plan** → final step is `Plan status → complete`. The plan file **persists** with status
  `complete` (it is the durable record of the decomposition).
- **Fix plan** → final step is `Plan file deleted`. `{{paths.docs}}/plans/fix/` holds only *active* fixes;
  once the regression test and any troubleshooting entry are committed, the durable record lives in
  the test and the docs, not the plan — delete `{{paths.docs}}/plans/fix/<bug-slug>.md`, git history is the
  archive. Plus a **post-fix troubleshooting update**: if the bug was non-obvious to diagnose (more
  than a few minutes of tracing), add a **Symptom → Root Cause → Fix → Prevention** entry to the
  troubleshooting docs for the affected layer.
Find each layer's troubleshooting doc via `cinch index`.

---

## Compaction anchor

A workflow session can end two ways: a deliberate SESSION STOP
(`{{paths.docs}}/workflows/dev-execute-plan.md` § Session Boundaries) between tasks, or a mid-session
summarization event the running agent's own context management triggers unasked — compacting away
the instructions this file loaded at session start. The first is a cold boundary the Session
Handoff template already covers. The second is a hot one: a long-running agent can compact
mid-task, after loading this file's rules but before finishing the task that used them.

The anchor is the minimum state a workflow declares must survive that event, re-injected from
source rather than reconstructed from whatever the compacted summary retained:

1. **Current task ID** — `T<N>`, from the plan file's `## Tasks` section (or `## Session Handoff`
   → `Resume at:`, per dev-execute-plan's tracking).
2. **Its context manifest** — that task's own `Context` list (§ Context manifest above), reloaded
   from the plan file, not paraphrased from memory.
3. **The completion ritual** — § Completion ritual above, including which plan-type lifecycle
   applies (feature persists / fix deletes).

Re-injection is the calling workflow's job at the point it notices compaction happened — a
discontinuity in its own working context is the signal; no explicit hook exists for this. Reload
the three items above from their source location before resuming the task in progress; do not
continue from the summary's paraphrase of them.
