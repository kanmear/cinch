# cinch

A referential integrity checker for the operational documentation that governs
a repository — the rules, workflows, and conventions code must conform to, read
by humans and executed by agents. **Status: Stage 2 landed —
three checks (links, rules, coupling) plus `render` (philosophy verbatim,
eight workflows templated). See README.md.**

## What this repo is

- `.docs/PRINCIPLES.md` — the spec the rebuild must satisfy. The six principles,
  ranked by evidence, each with its rebuild constraint. Read this before any
  rebuild work; it is the only memory of the old design that is allowed in the
  room — the old design itself is git history.
- `.docs/cinch-rebuild-plan.md` — the staged rebuild plan. Stage 1 and Stage 2
  done, Stage 3 next.
- `README.md` — the orientation artifact: status, layout map, pointers.
- `main.go` — CLI dispatch, the only `package main` file. Everything else
  lives in `internal/cinch` (compiler-enforced private to this module):
  `check.go`, `links.go`, `rules.go`, `coupling.go` — the three checks, each
  paired with a `_test.go` carrying its mutation fixtures. `render.go`,
  `manifest.go` — the `render` command: `{{key}}` substitution and the
  `.docs/manifest` parser. `philosophy.md`, `templates/*.md` (eight
  workflows) — the content `render` ships, embedded via `go:embed`.
  `tests/` — CLI-level tests against the built binary. `Makefile` —
  `build` / `test`.

## Rules

**The six principles govern everything that lands here** (.docs/PRINCIPLES.md is
canonical): deterministic checkers with mutation fixtures over model audits;
the direction rule (doc-upstream, lateral, structural — never doc-downstream);
the derivability gate on docs and the holdability gate on the system; rule→test
markers in the test file; bind values in one place, shape by absence; no proxy
metrics — every check has a demonstrable failing state, every warn a reachable
green state, no accepted baselines.

**Handoffs go to plan files, never `.docs/`.** .docs/ holds
spec only; session state is point-in-time and belongs in a plan file — git is
the archive.

**Historical artifacts are deleted, not kept alive.** If an old artifact
explains the new one better than the new one does, the new one is incomplete.
Recovering old material from git is normal; committing it back is not.

## Session start

Read `.docs/PRINCIPLES.md`, then `.docs/cinch-rebuild-plan.md`. The plan is
deliberately short — if it grows past two pages, that's the failure it exists
to avoid — and is not the old roadmap; the purge deleted that, and this one
is planned against the principles, staged, one stage at a time.
