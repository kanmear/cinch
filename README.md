# cinch

A referential integrity checker for the operational documentation that governs
a repository — the rules, workflows, and conventions code must conform to, read
by humans and executed by agents.

## Status: Stage 1 landed — three checks, no config

The previous implementation was purged on 2026-08-07: every checker,
template, self-harness, and operational artifact was deleted. Git history is
the archive of everything that was tried; `.docs/PRINCIPLES.md` is the memory
of what survived contact with evidence — six principles ranked by what was
tested and passed, not by what seemed clever. `.docs/cinch-rebuild-plan.md` is
the staged plan the rebuild follows.

`cinch check` runs three checks against the current directory, no
configuration:

- **links** (error) — a relative markdown link under `.docs` that doesn't
  resolve on disk.
- **rules** (error) — a rule ID (`1. **ID-NNN** ...`) with no `// cinch:rule`
  marker anywhere in source, or a marker that resolves to no rule ID. A rule
  can declare `<!-- cinch:ignore: <reason> -->` under itself instead of a
  marker — a *declaration* that the rule is outside a test's domain, not a
  suppression; it must carry a reason, and a rule that's both ignored and
  marked is a contradiction and is itself a finding. `cinch ignores` lists
  every declaration with its reason.
- **coupling** (block) — a rule's item-scoped text changed in the working
  tree vs HEAD while its marked test file didn't. Transition-scoped (working
  tree vs HEAD), so it's a no-op post-commit or outside a git repo, and says
  so on stderr rather than reporting a silent pass. Escape via
  `rule-reword: <ID>` in a commit message, wired through `cinch check
  [MSGFILE]` (matches git's own `commit-msg` hook contract).

Exit codes: `0` clean, `1` findings, `2` usage error.

`make test` builds `bin/cinch` and runs `go vet` plus every check's mutation
fixtures end-to-end.

## Layout

- `main.go` — CLI dispatch (`check`, `ignores`).
- `check.go` — the `Finding` model and `cmdCheck` orchestrator.
- `links.go`, `rules.go`, `coupling.go` — one file per check, each paired with
  a `_test.go` carrying its mutation fixtures.
- `.docs/PRINCIPLES.md` — the spec the rebuild must satisfy: the six
  principles (deterministic over semantic, the direction rule, the
  derivability and holdability gates, rule→test markers, bind values not
  shape, no proxy metrics), each with the evidence that earned it and its
  rebuild constraint.
- `.docs/cinch-rebuild-plan.md` — the staged rebuild plan; Stage 1 done, Stage
  2 (philosophy/workflow templating) next.
- `tests/` — CLI-level tests exercising the built binary.
- `Makefile` — `build`, `test`.

## Known limitation

The marker scan is deliberately textual, not Go-aware (principle 1: a
property is checkable only if it's decidable by a script, not a parser tied
to one language). That means a repo whose own source *quotes* marker syntax
as a string literal (this repo's `rules_test.go`, for its own mutation
fixtures) will see those quotes picked up as if they were real markers. No
file-name exclusion is built for it — that would be exactly the kind of
ad-hoc config Stage 1 avoids — and Stage 1 doesn't require `cinch check` to
be clean against its own repo, since no real rule corpus lives here yet.

## The rebuild

Stage 1 is done: three checks, three mutation-fixture suites, `cinch check`
runs on any repo with no configuration. Next is Stage 2 — philosophy copied
verbatim, workflows as value-substitution-only templates — per
`.docs/cinch-rebuild-plan.md`.
