# cinch

A referential integrity checker for the operational documentation that governs
a repository — the rules, workflows, and conventions code must conform to, read
by humans and executed by agents.

## Status: C1 landed, rebuild in progress

The previous implementation was purged on 2026-08-07: every checker,
template, self-harness, and operational artifact was deleted. Git history is
the archive of everything that was tried; `.docs/PRINCIPLES.md` is the memory
of what survived contact with evidence — six principles ranked by what was
tested and passed, not by what seemed clever.

The rebuild has landed its first checker, **C1 — the index covers the corpus**:
`cinch check` verifies `.docs/index.md` matches `.docs/` in both directions
(every file listed, every entry real), with mutation fixtures proving each
failure class. `make test` builds the binary, exercises it end-to-end, and
runs the check against this repository's own corpus.

## Layout

- `main.go`, `check.go` — the CLI and the checkers. `cinch check [-root DIR]`
  (default root: `.docs`).
- `.docs/` — the corpus: `PRINCIPLES.md` (the spec the rebuild must satisfy),
  `index.md` (the corpus index, enforced by C1).
- `tests/` — CLI-level tests exercising the built binary; enforcing tests
  carry `// cinch:rule <ID>` markers.
- `Makefile` — `build`, `test`.
