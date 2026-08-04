---
name: check-rules
description: Audit test coverage against the domain rules in `.agent/domain/`. Use for the rule-coverage audit — select the auditor agent, which is read-only and runs only the harness checkers.
---

# /check-rules

Audit test coverage against the domain rules in `.agent/domain/`.

Run this audit with the **auditor agent** (`.opencode/agent/auditor.md`) — its permission envelope is
the seam's enforcement: read-only, only the harness checkers may run. Read and follow
`.agent/workflows/check-rules.md` — the canonical, agent-neutral procedure. This skill is only the
opencode entry point.
