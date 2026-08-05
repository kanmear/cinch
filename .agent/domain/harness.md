---
rule_prefix: HARNESS
owns:
  - main.go
  - internal/manifest/
  - internal/render/
  - internal/index/
  - internal/check/
  - internal/version/
  - templates/
---

# Harness Domain Rules

## Direction of checks

1. **HARNESS-001** — Checkers are doc-upstream or lateral only. A checker
   validates that code or a bound artifact conforms to the doc (doc is the
   spec), or that two authored artifacts agree. Never doc-downstream: a check
   that validates a doc's copy of a machine-readable fact means the
   duplication should be deleted instead (D009). *Enforced: no test;
   constrains every future checker decision.*
   <!-- cinch:ignore HARNESS-001 — a future-decision constraint; no test can enforce it -->

## The audit seam

2. **HARNESS-002** — The auditor seam is never cheap. Whatever role audits the
   integrity loop, its activation cost must stay high enough that a false
   checkmark anywhere in the loop is never the cheap path. *Enforced: C12.*

## Templates

3. **HARNESS-003** — Templates name no stack and no harness. A template
   references another workflow by `.agent/workflows/<name>.md` path, never by
   name or invocation syntax; command-shaped tokens, runner-config paths, and
   stack literals come only from the manifest. *Enforced: TestTemplatePurity
   purity patterns.*

4. **HARNESS-004** — Rendered output is never hand-edited.
   `.agent/workflows/` is generated; the harness guards its own repo by
   re-rendering and diffing against committed output. *Enforced: C2 tamper
   branch.*

5. **HARNESS-005** — Rule IDs are permanent and never reused. Once a rule is
   numbered, the number stays with it forever; a retired rule keeps its ID
   with a retirement note rather than freeing it for reuse. *Enforced:
   TestRuleIDsUnique in rule_test.go.*
