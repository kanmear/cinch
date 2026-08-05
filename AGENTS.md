# cinch

A referential integrity checker for agent documentation, plus the binding layer
that makes it portable across repositories.

## What this repo is

- `*.go` — the CLI: `render`, `index`, `check`, `ignores`, `version`. Parser unit tests live
  next to the code they test at the module root; CLI-level tests live in `tests/` and exercise
  the built binary.
- `templates/` — portable workflow templates. `render` walks every `*.md` under here
  and renders it into a consumer's `.agent/workflows/`, with no filename exclusion — a meta-doc
  placed in this directory (a README, notes-to-porters) gets rendered as if it were a workflow.
  Directory-level conventions belong here in `AGENTS.md`, not a nested README. Reference
  `{{commands.check}}`, `{{paths.tests.integration}}`, `{{taxonomy.layers}}`. Never mention a
  language, framework, or command literal.
- `docs/` — spec, contract, plan+status only: `PHILOSOPHY.md`, `INTROSPECTION.md` (the C6/C7 JSON
  contract), `ROADMAP.md`. `ops/` — how this repo is run: git workflow, versioning, release.
- `decisions.jsonl` — append-only decision log. Read it before proposing a change to a settled
  question; append a superseding entry rather than diverging silently.
- `README.md` — the orientation artifact: layout map, glossary, checker registry. This file
  carries only what a session needs without asking; the map and registry live there, not here.

## Rules

**Nothing lands here that is not rendering into a real project.** A template or checker is not done
until it runs against a consuming repo.

**Nothing project-specific crosses the boundary.** If a change requires naming a stack, it belongs
in the consumer's `manifest.yml` or in a manifest-declared command, not here.

**Nothing harness-specific crosses the boundary either.** A harness's entry point (skill shim,
slash command) is consumer-side glue; a template references another workflow by
`.agent/workflows/<name>.md` path, never by name or invocation syntax. Command-shaped tokens,
runner-config paths, and stack literals hardcoded from the scaffold contract are checked
structurally by `go test` (`templates_test.go`) — a template describing in prose *how* its runner
happens to surface something is a judgment call for whoever ports it, not something a script
catches by name.

**Checkers are doc-upstream or lateral only.** A check that validates a doc's copy of a
machine-readable fact means the duplication should be deleted instead.

**Core never parses project source.** Type and route introspection are commands the consumer
declares; this repo defines their output contract and nothing more.

## Session start

Read `docs/ROADMAP.md` for the phase you are working, and the decision IDs it references. Not the
whole roadmap.
