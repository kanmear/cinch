# Rule Coverage Audit Report

Audit of cinch's own domain (`.agent/domain/harness.md`) against its test coverage,
per `.agent/workflows/check-rules.md`. Closure from `./bin/cinch check` (C8) and
`./bin/cinch ignores`; the report follows the C15-verified format — every row quotes
the rule text verbatim.

## harness.md

| Rule | Rule text (verbatim) | Testable | Covered | Test file | Test | Assertion (verbatim) |
|------|----------------------|----------|---------|-----------|------|----------------------|
| HARNESS-001 | `Checkers are doc-upstream, lateral, or structural only.` | N/A | N/A | — | — | — |
| HARNESS-002 | `The auditor seam is never cheap.` | N/A | N/A | — | — | — |
| HARNESS-003 | `Templates name no stack and no harness.` | N/A | N/A | — | — | — |
| HARNESS-004 | `Rendered output is never hand-edited.` | N/A | N/A | — | — | — |
| HARNESS-005 | `Rule IDs are permanent and never reused.` | N/A | N/A | — | — | — |

All five HARNESS rules are declared `<!-- cinch:ignore -->` in the doc (D062): they
constrain future decisions, not current behavior, and each carries its enforcement note
in the rule item (`*Enforced:* ...`).
