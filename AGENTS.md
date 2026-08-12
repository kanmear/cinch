# cinch

A referential integrity checker for the operational documentation that governs
a repository — the rules, workflows, and conventions code must conform to, read
by humans and executed by agents. **Status: 0.1.0 — init, self-enforcement,
manifest-driven extension. Five checks (links, rules, coupling, generated,
commit), `render` (philosophy, workflows, hook shims), `init`, `hook`,
`workflows`/`workflow`/`docs`. See README.md.**

Workflows: run `cinch workflows` to see what's available, `cinch workflow <name>` to load one.
Docs: run `cinch docs` to see every doc's path and title.

## What this repo is

- `.docs/PRINCIPLES.md` — the spec the rebuild must satisfy. The six principles,
  ranked by evidence, each with its rebuild constraint. Read this before any
  rebuild work; it is the only memory of the old design that is allowed in the
  room — the old design itself is git history.
- `.docs/plans/cinch-0.1.0.md` — the plan that took cinch from Stage 2 to
  0.1.0: init, the `generated` check, and manifest-driven extension.
- `README.md` — the orientation artifact: status, layout map, pointers.
- `cinch_manifest` — this repo's own manifest (`paths.docs = .docs`, plus a
  `hooks.pre-commit.build.run` entry) — cinch self-hosts.
- `main.go` — CLI dispatch, the only `package main` file. Everything else
  lives in `internal/cinch` (compiler-enforced private to this module):
  `check.go`, `links.go`, `rules.go`, `coupling.go`, `generated.go`,
  `commit.go` — the five checks, each paired with a `_test.go` carrying its
  mutation fixtures. `render.go`, `manifest.go` — the `render` command:
  `{{key}}` substitution, hook shims, and the `cinch_manifest` parser.
  `title.go` — shared H1 title/trigger extraction. `hook.go` — the
  `cinch hook` dispatcher. `init.go` — `cinch init`. `workflow.go` —
  `cinch workflows` / `cinch workflow NAME`, computed on demand. `docs.go` —
  `cinch docs`, computed on demand. `docs/philosophy.md`,
  `docs/templates/*.md` (nine workflows) — the content `render` ships,
  embedded via `go:embed`. `tests/` — CLI-level tests against the built
  binary. `Makefile` — `build` / `test`.

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

Read `.docs/PRINCIPLES.md` before any rebuild work.
