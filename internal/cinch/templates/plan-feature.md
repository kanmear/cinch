# Feature Planning Workflow

Produce a structured, atomic feature plan before writing any implementation code.

Outputs `.agent/plans/<feature-slug>.md` — a living plan file that tracks domain rules, the test plan, and per-task execution checklists with explicit context manifests.

**Do not write implementation code during this workflow.** Writing domain rule docs (via the rule-maintenance workflow, `.agent/workflows/maintain-domain.md`) and plan file content is the only output.

---

## When to run

Run this workflow at the start of every non-trivial feature that touches domain logic. Skip for
pure style changes, dependency bumps, or doc-only edits. Bug work routes through
`.agent/workflows/fix-bug.md`, not here.

---

## Phase 1: Domain Rules

**Goal:** Enumerate every domain rule governing this feature — existing and new — and validate them with the user before proceeding. Rules are the foundation; a wrong or missing rule corrupts every test and task downstream.

### 1.1 Identify affected domains

From the feature description, identify which domains are involved. The domain list is the set of
files in `{{paths.docs}}/` minus `overview.md` — list that directory
to see the current domains; each `<domain>.md` is one domain. (There is deliberately no domain list
in the manifest to copy out of sync — the filesystem is the source.)

Read only the relevant files. If uncertain which domains apply, read `{{paths.docs}}/overview.md`
first for orientation.

### 1.2 List applicable rules

For each affected domain file, list every rule that applies to this feature:

```
Existing rules:
- [SIG-001] <rule summary — e.g. an action stays available regardless of another entity's pending state>
- [SIG-005] <rule summary — e.g. a role assignment excludes the record's own creator>
- [MEMB-002] <rule summary — e.g. only one active record permitted per some uniqueness key>
```

Reference rules by their **IDs** — the `**<PREFIX>-0NN**` tokens in the source file, prefixed
from its `rule_prefix` front-matter. IDs are permanent; numbers and prose references may look
alike but only the ID is identity.

### 1.3 Build a rule interaction table (multi-domain features only)

If more than one domain is involved, cross the rules and surface edge cases. Every intersection is a potential bug.

```
| Rule A (domain)                     | Rule B (domain)                   | Edge Case                                                 | Resolution needed? |
|--------------------------------------|-------------------------------------|-------------------------------------------------------------|--------------------|
| <rule A summary> (SIG-001)          | <rule B summary> (MEMB-002)       | <edge case where the two rules interact>                  | No — existing rule |
| <rule A summary> (SIG-005)          | <rule B summary> (PERM-007)       | <edge case revealing a gap>                                | YES — new rule      |
```

For every row where "Resolution needed?" is YES, a new rule must be written before the plan proceeds.

### 1.4 ⛔ CHECKPOINT — Present to user

Before writing any rules or moving to Phase 2, present:

1. The list of existing rules that apply (with source references)
2. The rule interaction table (if multi-domain)
3. A list of identified gaps: edge cases with no governing rule, or ambiguous rules

Ask: *"Are these rules complete? Any ambiguities or missing cases? Shall I write the missing rules?"*

Wait for confirmation. Do not proceed until the user confirms or redirects.

### 1.5 Write new rules

For each gap or new rule identified, write it to the appropriate `{{paths.docs}}/<domain>.md` via
`.agent/workflows/maintain-domain.md`. Follow that workflow for placement, numbering, and style.

Do not inline rule text in the plan file. Reference the canonical location instead.

### 1.6 Fill in the plan file — Domain Rules section

Record the final rule set in `.agent/plans/<feature-slug>.md`:

```markdown
## Domain Rules

### Existing Rules (from {{paths.docs}}/)
- [domain.md #N] Rule summary

### New Rules (written during planning)
- [domain.md #N] Rule summary

### Rule Interactions (if multi-domain)
| Rule A | Rule B | Edge Case | Resolution |
|--------|--------|-----------|------------|
```

---

## Phase 2: Test Plan

**Goal:** Derive a test for every rule and edge case, classify by type, and get user confirmation before writing a single line of code.

### 2.1 One test per rule

For each rule from Phase 1, write one test row. For complex rules, add an additional row for each edge case.

Classify each test by **tier** — the tiers are defined in `.agent/workflows/task-primitive.md` § Test
tiers (sourced from `manifest.taxonomy.test_tiers`): `integration`, `unit`, `e2e`, `manual`. That
section holds the TDD policy per tier; e2e/`manual` operational guidance lives in the tier's `notes`
doc (`.agent/frontend/e2e.md`). Read those rather than re-deriving tiers here.

### 2.2 Mark TDD-mandatory tests

Apply each tier's `tdd` policy from `.agent/workflows/task-primitive.md` § Test tiers — do not
re-derive it here. Mark TDD-mandatory rows explicitly.

### 2.3 For each test, record

- Which rule it validates (traceable link)
- What assertion proves the rule holds (expected status code, state change, or error code)
- Which existing test file it belongs in (or new file if needed)

### 2.4 ⛔ CHECKPOINT — Present test plan to user

Present the full test plan. Ask: *"Does this cover all the cases you care about? Any rules that should not have automated tests? Any edge cases I've missed?"*

Wait for confirmation.

### 2.5 Fill in the plan file — Test Plan section

Tag each row with its tier `id`; test-file locations come from `manifest.paths.tests`. Resolve
each tier's run command via `taxonomy.test_tiers[].cmd` → `development.commands` (never hardcode
Make targets here).

```markdown
## Test Plan
[ ] integration: <description> — validates [domain.md #N]
      Assert: <what proves the rule holds>
      File: backend/tests/<layer>/<domain>_test.go
      TDD: mandatory
[ ] unit: <description> — validates [domain.md #N]
      Assert: <what proves the rule holds>
      File: frontend/tests/<area>/<component>.test.ts
      TDD: optional
[ ] e2e: <description> — full-stack smoke (opt-in, non-atomic)
      Assert: <what proves the flow works end-to-end>
      File: frontend/e2e/<domain>/<feature>.spec.ts   (auto-discovered by the e2e tier cmd)
[m] manual: <description> — manual only (visual/UX)
```

---

## Phase 3: Atomic Task Decomposition — via the task primitive

With the Domain Rules and Test Plan sections confirmed and filled in, decompose the
implementation using `.agent/workflows/task-primitive.md`. It supplies, unchanged for feature
work: Atomicity, Context manifest, Verification tiers, Task template, Pre-flight gate, Completion
ritual, and Compaction anchor — see that file's matching sections.

**Feature-specific inputs to the primitive's gate/ritual:**

- Planning checkpoints to clear: Phase 1 (rules confirmed) and Phase 2 (test plan confirmed).
- Plan location: `.agent/plans/<feature-slug>.md`, saved with status `in-progress`.
- Completion uses the **feature** lifecycle: the plan file **persists** with status `complete`.

Task execution — working through `T1`, `T2`, … — happens via `.agent/workflows/execute-plan.md`,
not inline here.
