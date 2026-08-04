# Domain Overview

cinch's domain is the harness itself: rendering portable templates, checking
the rendered result for drift, and indexing it. It is a different-shaped
consumer than project_deltadocs (D062) — a CLI whose domain is its own
behavior — and it exists here because the audit proved the rules are real:
several were already enforced by tests before they were written down (C12,
C13, C2) and others were prose in `AGENTS.md` that nothing checked.

## Structure Convention

The domain list is the filesystem: `.agent/domain/*.md` minus this file. Each file
declares its owning code paths in front-matter (D010) and holds numbered rules
with permanent IDs. `check-rules` audits them against test coverage; missing
coverage is a gap, not an accusation.

## R001 — The guardrail (what earns a place here)

**A rule earns a place in `domain/` only if a test can enforce it, or it
constrains a future decision. Otherwise it is a log entry.** Without this,
cinch accumulates rules as ceremony — the GROW failure (D021) in a different
costume.

This file is exempt from the coverage audit (the templates read every
`domain/*.md` except the overview); the guardrail's enforcing test is the
commit review that rejects rule-doc additions lacking either a test reference
or a future-decision constraint.
