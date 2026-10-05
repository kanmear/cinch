# Feature Planning

Use this when the change spans more than one layer, more than one session, or adds or changes a rule.

Otherwise make the change directly, but still do the missing-rule step (step 2).

The output is a plan file, `{{paths.docs}}/plans/<feature-slug>.md`, confirmed by the user before
any implementation code is written. Rule docs are any doc under `{{paths.docs}}/` with numbered
`ID-NNN` items; `cinch rules --json` lists every rule with its file, line, and markers.

## 1. Find the rules that apply

- Run `cinch impact <files you expect to touch>`: it names the rules whose doc `owns:` those
  paths or whose markers sit in them.
- Search the rule docs for the feature's nouns and verbs, and read each candidate rule in full.
- When rules from more than one doc apply, check each pair for a case neither one decides. Each
  such case is a gap for step 2.

Reference rules by ID only. Never copy a rule's numbered item into the plan: cinch parses every
doc under `{{paths.docs}}/`, plans included, so a copied item becomes a duplicate rule ID.

## 2. Missing-rule step

List the invariants this feature introduces or relies on that no rule states. Apply the Admission
Test ([Doc Philosophy](docs-philosophy.md)): anything a code change could violate and a reader
couldn't recover from the source becomes a new rule before any code is written. Example: who may
receive a realtime event that carries restricted content.

Also list rules that read two ways for this feature; each needs clarifying before it can be tested.

## 3. Test plan

One test per rule the feature enforces, plus one per edge case a rule's text leaves open. For each,
record the rule ID, the assertion that proves the rule holds (a response, a state change, an
error), and the test file. The test that enforces a rule carries a `cinch:rule <ID>` comment above
it.

## 4. ⛔ Checkpoint

Present in one message: the existing rules that apply (by ID), the gaps and ambiguities from steps
1–2 with the rule text you propose for each, and the test plan. Ask whether the rules are complete
and correct and whether the tests cover what matters. Wait for confirmation. A corrected rule means
a revised test plan.

Then write the confirmed new or changed rules via `{{paths.docs}}/workflows/docs-maintain-domain.md`.

## 5. Tasks

A task is atomic when it is one concern and independently verifiable: once it is done, the
project's test command (see AGENTS.md or the project's own docs) passes and proves the task's part
works. Split by what can pass on its own (a query, then the handler that calls it, then the UI that
consumes it). Keep several layers in one task only when splitting would leave a state that doesn't
build or run, and say so in the task.

## 6. Plan file

```markdown
# <Feature>
**Status:** in-progress

## Rules
- Existing: <ID>, <ID> — <why each applies>
- New: <ID> — written in <doc path>

## Test Plan
- [ ] <ID>: <assertion> — <test file>

## Tasks
### T1: <title>
**Status:** [ ]
**Depends on:** —
**Files:** <paths and what changes>
**Rules:** <IDs this task enforces>
**Verify:** <the tests that prove it, run with the project's test command>

## Handoff
```

Work through the tasks with `{{paths.docs}}/workflows/dev-execute-plan.md`, which also covers
completion.
