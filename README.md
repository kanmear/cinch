# cinch

A referential integrity checker for agent documentation, plus the binding layer
that makes it portable across repositories.

Cinch renders portable workflow templates against a per-repository manifest,
and verifies the resulting corpus is referentially sound — that every path,
link, command, generated artifact, and rule-to-test binding resolves to
something that exists.

## What cinch is not

Not a memory system; not a semantic validator — even at its best it detects
*divergence* (C10/C14), never *correctness*. The database connotation of
"referential integrity" carries the limitation for free.

## Layout

- `main.go` — the entry point: usage, dispatch. `internal/` — the packages:
  `manifest/` (manifest.yml loading), `render/` (render + template purity),
  `index/` (generated index), `check/` (the C1–C15 checkers), `version/`
  (binary-relative version store). Paths resolve from the git repository root;
  templates and the version store resolve next to the binary.
- `templates/` — portable workflow templates and fragments, nothing else. Every
  `*.md` here renders into a consumer's `.agent/workflows/`.
- `scaffold/` — the starter contract a consumer binds against:
  `manifest.example.yml` is the variable contract templates are written to.
- `docs/` — core/philosophy only: the spec (`PHILOSOPHY.md`), the plan and its
  status (`ROADMAP.md`), the C6/C7 contract (`INTROSPECTION.md`). Historical
  artifacts are deleted — git is the archive.
- `ops/` — how this repo is run: git workflow, versioning, release.
- `decisions.jsonl` — append-only decision log.
- `version/` — generated one-field-per-file version store (D043).
- `tests/` — CLI-level tests exercising the built binary; unit tests live next
  to the code they exercise.
- `.agent/` — cinch's own consumer corpus: manifest, domain rules, rendered
  workflows. Cinch is its own first consumer.

## Glossary

- **harness** — a consumer's `.agent/` system: rendered workflows, domain docs,
  index, and manifest. What cinch builds and verifies.
- **cinch** — the tool that builds and verifies a harness.
- **binding layer** — templates + manifest + render step: the machinery that
  turns portable templates into repo-bound workflows.
- **corpus** — a consumer's authored and generated `.agent/` documentation set.
- **referential integrity** — the property that every path, link, command,
  generated artifact, and rule-to-test binding in the corpus resolves to
  something that exists.
- **template** — a portable workflow source under `templates/`, authored once,
  rendered into every consumer.
- **workflow** — a rendered template in `.agent/workflows/`: the agent-neutral
  procedure the runtime model reads.
- **fragment** — a template file under `templates/fragments/` that composes into
  a base template at an anchor comment when its manifest keys exist; never
  renders standalone (D063).
- **manifest** — a consumer's `manifest.yml`: the single source of
  project-specific bindings.
- **checker** — one of C1–C15, compiled into the binary and run by
  `cinch check`. (C13 is retired: the template-purity check is the unit test
  `TestTemplatePurity`, not a checker.)
- **rule ID** — a permanent `PREFIX-NNN` identifier on a numbered rule in a
  domain doc.
- **marker** — a `// cinch:rule <ID>` comment above a rule's enforcing test.
- **domain doc** — a `.agent/domain/<name>.md` file holding numbered rules and
  the `owns:` code paths they guard.
- **seam** — a declared workflow role (planner, executor, doc_maintainer,
  auditor) with a model tier in the manifest.
- **auditor** — the strong-model seam that runs the check-rules semantic
  verification over the script-produced closure.
- **scaffold** — the starter contract a consumer binds against;
  `scaffold/manifest.example.yml` is the variable contract.

## Checker registry

C# | checks | direction | severity | file
--- | --- | --- | --- | ---
C1 | committed `.agent/index.md` matches a fresh generation | lateral | error | internal/check/check.go
C2 | rendered workflows match a fresh render (stale) and their header hash (tamper) | lateral | error | internal/check/check.go
C3 | test-tier `cmd` ids resolve under `development.commands` | structural | error | internal/check/check.go
C4 | every `paths.*` binding points at something that exists | structural | error | internal/check/check.go
C5 | `overview.md` references every domain doc | lateral | error | internal/check/check.go
C6 | `models/*.md` matches introspect-types output | doc-upstream | error; coverage warn | internal/check/check.go
C7 | `api/*.md` matches introspect-routes output | doc-upstream | error; coverage warn | internal/check/check.go
C8 | rule→marker closure: marked or ignored, markers resolve, prefixes and numbering hold | lateral | error; unmarked warns | internal/check/rule.go
C9 | plans carry a `Status:` line; complete fix plans are deleted | structural | error | internal/check/check.go
C10 | a change touching a domain's `owns:` paths leaves its domain doc warning-free | lateral | warn | internal/check/diff.go
C11 | links and backticked path tokens inside `.agent/` resolve | structural | warn | internal/check/check.go
C12 | seam declarations: known tier, auditor strong, `allow` ids resolve | structural | error | internal/check/check.go
C14 | a rule's text changed and the file carrying its `// cinch:rule` marker did not | lateral | warn | internal/check/diff.go
C15 | the committed audit report (paths.audit) quotes rule text and assertion lines verbatim from their sources | lateral | error | internal/check/audit.go

Direction rule (D009): doc-upstream, lateral, and structural checks only —
never doc-downstream. A check that validates a doc's copy of a machine-readable
fact means the duplication should be deleted instead.
