# cinch

A portable agent harness: templates plus a Go CLI that renders them against a project's
`manifest.yml` and checks the result for drift.

## What this repo is

- `*.go` — the CLI. `render`, `index`, `check`.
- `templates/` — portable workflow templates. Reference `{{commands.check}}`,
  `{{paths.tests.integration}}`, `{{taxonomy.layers}}`. Never mention a language, framework, or
  command literal.
- `fixtures/` — toy repos CI renders into, one per stack. The portability test.
- `docs/` — core/philosophy only: `ROADMAP.md` (phased plan), `RATIONALE.md` (why the design is
  what it is), `PHILOSOPHY.md`, decisions log.
- `ops/` — operational docs, not core: git workflow, versioning, release. Anything that
  describes how this repo is *run* goes here, never in `docs/`.
- `decisions.jsonl` — append-only decision log. Read it before proposing a change to a settled
  question; append a superseding entry rather than diverging silently.

## Rules

**Nothing lands here that is not rendering into a real project.** A template or checker is not done
until it runs against a consuming repo.

**Nothing project-specific crosses the boundary.** If a change requires naming a stack, it belongs
in the consumer's `manifest.yml` or in a manifest-declared command, not here.

**Checkers are doc-upstream or lateral only.** A check that validates a doc's copy of a
machine-readable fact means the duplication should be deleted instead.

**Core never parses project source.** Type and route introspection are commands the consumer
declares; this repo defines their output contract and nothing more.

## Session start

Read `docs/ROADMAP.md` for the phase you are working, and the decision IDs it references. Not the
whole roadmap.
