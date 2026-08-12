# cinch

A referential integrity checker for the operational documentation that governs
a repository — the rules, workflows, and conventions code must conform to, read
by humans and executed by agents.

## Overview

Operational documentation — the rules, conventions, and workflows a codebase
expects contributors (human or AI) to follow — tends to drift from what the
code actually does. Nobody remembers to update the troubleshooting doc after
fixing the bug it describes; a "business rule" written down eighteen months
ago quietly stops being enforced by any test; a workflow file tells an agent
to read a path that hasn't existed since the last refactor. The failure mode
is always the same: the doc *looks* authoritative and isn't, and nothing
catches the gap until someone — increasingly, an autonomous agent — trusts
it and acts on stale information.

cinch closes that gap by making a repo's operational docs *checkable*, not
just readable. It doesn't understand what a rule means — it enforces that
every rule is bound to something concrete and verifiable:

- a **rule** in a doc must have a matching `// cinch:rule <ID>` marker
  somewhere in the source that enforces it (or an explicit, reasoned
  declaration that it's untestable) — so "documented" and "enforced" can't
  quietly diverge
- a **relative link** between docs must resolve on disk
- a rule's **text and its enforcing test** must change together — editing
  one without the other is a blocked commit, not a silent drift
- the **rendered output** — shared workflow guidance, git hook shims — must
  match what re-generating it right now would produce, byte for byte, so a
  hand-edit or tampering attempt is caught the same way a code change would
  be
- an optional **commit message convention** can be enforced the same way

All five are one command, `cinch check`, with no configuration required by
default. `cinch init` scaffolds a new consumer end to end: a manifest, a docs
directory structure, a set of rendered onboarding workflows (bug fixing,
feature planning, domain-rule maintenance, doc sync, and more — all generic,
with no assumption about a project's stack or directory layout), and
activated git hooks that run `cinch check` on every commit. From there, cinch
enforces itself: a commit that breaks a rule, tampers with generated output,
or violates a project's own registered checks gets blocked — a project's own
script (say, `check_error_codes.sh`) hooks in through the manifest, scoped by
path prefix, with no bespoke `.githooks/` scripting required.

The point is to make "the docs are accurate" a property a script verifies on
every commit, the way a linter verifies formatting — not a hope that has to
survive contact with a deadline.

## Install

```
make install    # go install . — puts `cinch` on your PATH via GOBIN
```

The generated git hook shims `exec cinch hook <event>` — a bare PATH lookup,
deliberately, so a shim never hardcodes one machine's path. The consequence is
that the binary has to be *installed*, not just built: `make build` produces
`./bin/cinch` for local iteration, but a repo whose hooks are activated needs
`make install`, or every commit fails with `cinch: not found` (exit 127)
before a single check runs.

## Status: 0.1.0 — init, self-enforcement, manifest-driven extension

`cinch init` scaffolds a new consumer end to end: a manifest, the docs
directory structure, a full render, and activated git hooks — one command,
idempotent. `cinch check` runs five checks, including one that verifies the
render output on disk hasn't been hand-edited or fallen out of date, and
another that enforces a commit message convention. Git hooks are generated
and, once activated, run `cinch check` on every commit — a project can also
hook its own scripts into `pre-commit`/`commit-msg` through the manifest,
scoped to the paths that should trigger them. This repo self-hosts: its own
`cinch.yml`, rendered docs, and `.githooks/` are the proof.

`cinch check` runs five checks against the current directory. The docs root
defaults to `.docs` and needs no configuration; a project can point it
elsewhere with `paths.docs` in `cinch.yml` (relative to the project
root, or an absolute path). `cinch render` substitutes the same value into
templates as `{{paths.docs}}` — one root, so a rule doc a workflow tells an
agent to write is guaranteed to be one `cinch check` actually scans:

- **links** (error) — a relative markdown link under the docs root that
  doesn't resolve on disk.
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
- **generated** (error) — the render output on disk doesn't match what
  `cinch render` would produce right now: a missing file, altered bytes, an
  orphaned generated file (one no longer produced by the current render), or
  a render that fails outright (an undefined `{{key}}` in a template, say) —
  that last one is a finding naming the manifest, not a silent skip, since a
  render that would fail is a defect this check can decide rather than an
  absence. A re-render diff proves tampering — this is the check that makes
  that claim true, and it's what makes hook shims tamper-evident: you cannot
  edit a generated `pre-commit` hook to skip cinch without `cinch check`
  failing, and deleting the docs directory to get out of the way doesn't
  help — verification is gated on *every* expected output being absent, not
  on one directory existing. A no-op, not a pass, when render genuinely
  hasn't run yet.
- **commit** (error) — when `MSGFILE` is given and the manifest's
  `commit.pattern` key is set, the commit's subject line must match it.
  Absent key, no behavior change.

`cinch render` writes into `paths.docs`'s `workflows/` subdirectory (default
`.docs/workflows/`): `docs-philosophy.md` (copied verbatim — no variables, no
renderer needed) and one file per template under
`internal/cinch/docs/templates/` (value-substituted against
`cinch.yml`). And it writes a thin shim per supported git hook event
under `paths.hooks` (default `.githooks/`): `exec cinch hook <event> "$@"` —
the dispatch table lives in the manifest and is read at runtime, so editing
it takes effect immediately with no re-render. Every generated output is
stamped with a header carrying a body-sha (comment-syntax aware: a shell
script's shebang stays the literal first line, with the marker on the line
below). Substitution is `{{key}}` literal replacement only — no loops, no
conditionals — and an undefined variable is an error, never a blank. The
render is idempotent: running it twice produces byte-identical output, so a
re-render diff proves tampering.

Workflow and doc navigation are *not* part of the render output — nothing is
persisted, so nothing can drift or need re-rendering. `cinch workflows`
computes and prints the workflow trigger table from whatever is under
`workflows/` right now; `cinch index` computes and prints every doc's path
and title from whatever is under `paths.docs` right now (excluding
`plans/`). Both read the current tree at call time.

**Version-skew note:** upgrading the cinch binary can redden every
consumer's `generated` check until `cinch render` is re-run — there's no
version pin or warn to soften this. The green state is reachable and the
fix is exact (`cinch render`), which is what matters; a fuzzy warn was
considered and rejected.

### `cinch init` and self-enforcement

`cinch init` is the entry point: run it once in a repo (a fresh one, or an
existing one adopting cinch) and it writes a starter manifest if none exists,
creates `paths.docs/plans/`, renders everything, and — in a git repo — runs
`git config core.hooksPath <paths.hooks>` to activate the generated hooks.
Idempotent: running it twice produces a byte-identical tree. Outside a git
repo, the shims are still generated but init says on stderr that they
weren't activated.

Once activated, `.githooks/pre-commit` and `.githooks/commit-msg` both
`exec cinch hook <event> "$@"`, which runs `cinch check` and then any
`hooks.<event>.<name>` entries a project has registered in its manifest,
scoped by `when` — a comma-separated list of path prefixes (not globs)
matched against the staged set. Every registered entry runs regardless of an
earlier one's failure (failures accumulate, not fail-fast), and a skipped
entry says so on stderr, so a hook run that did nothing is never silent.
`when` is ignored for `commit-msg` — a commit message has no changed paths to
scope against.

**Extension boundary:** cinch guarantees *dispatch* — did the `when` prefixes
match the staged set, did the registered command run, was its exit code
propagated — not your script's correctness. What a registered script does is
opaque to cinch; a green `cinch check` means dispatch worked, not that the
script itself is bug-free.

```yaml
# cinch.yml
paths:
  docs: .docs           # docs corpus root
  hooks: .githooks       # where generated hook shims land

hooks:
  pre-commit:
    error-codes:
      run: scripts/check_error_codes.sh
      when: [frontend/src/lib/api/, backend/errors/]
    frontend:
      run: make check-frontend
      when: [frontend/]

commit:
  pattern: '^\[[a-z-]+\] .+'
```

### Commands instead of per-harness skills or a persisted index

Don't hand-maintain a `SKILL.md`/slash-command wrapper per workflow per agent
harness — that's one file per workflow times one per harness, all of it
duplicated boilerplate that only ever says "read this file." And don't
persist a doc/workflow index either — a written-to-disk index is one more
render output that can drift and one more thing `cinch check` has to verify.
Compute both on demand instead. Add two lines to the consumer repo's
`AGENTS.md`, once — `cinch init` writes them for you if `AGENTS.md` doesn't
already exist:

```
Workflows: run `cinch workflows` to see what's available, `cinch workflow <name>` to load one.
Docs: run `cinch index` to see every doc's path and title.
```

Both are commands, not paths, so neither can go stale under a customized
`paths.docs`. `cinch workflows` computes and prints the workflow trigger
table; `cinch workflow NAME` prints one workflow's full rendered content;
`cinch index` computes and prints the whole doc corpus's path + title list.
Any agent that reads `AGENTS.md` — not just one harness's proprietary skill
system — can run these commands directly. The trade-off: harnesses with
native slash-command UX (typing `/domain`) lose that explicit affordance in
exchange for zero duplication and agent-neutrality.

`cinch.yml` binds the values templates and hooks reference — plain YAML, but
still no schema: nesting is notation for writing dotted keys hierarchically
(`paths: {docs: x}` and `paths.docs = x` bind the same thing), not a shape
cinch validates. Declaration order is preserved (hook entries run in the
order they're written). `paths.docs` is the one variable any shipped
template uses, and defaults to `.docs` when unset. The manifest itself
always lives at the repo root, independent of `paths.docs` — its own
location can't depend on a value it defines, so it's pinned outside the
directory `paths.docs` controls.

### A richer project manifest

`cinch.yml` only binds what cinch itself reads: `paths.*`, `hooks.*`,
`commit.pattern`. Workflow-template *prose*, though, can reference arbitrary
project-specific values cinch never touches — commands, ports, service
layout, test taxonomy — and a project with enough of those is better served
by its own separate, schema-free file than by overloading `cinch.yml`.
There's no cinch convention format for this (by design — principle 5, cinch
binds values, not shape), but the convention name is `manifest.yml` —
deliberately distinct from `cinch.yml` so the two are never confused: one is
cinch's own config, the other a project's. A common shape looks like:

```yaml
# manifest.yml — project-owned, cinch never reads this file
project:
  name: my-app
services:
  backend:  { path: backend,  port: 8080 }
  frontend: { path: frontend, port: 5173 }
development:
  commands:
    test-backend:  make test-backend
    test-frontend: make test-frontend
taxonomy:
  test_tiers:
    - { id: unit, cmd: test-backend }
```

Note this manifest is outside cinch's `{{key}}` substitution entirely — it's
not `cinch.yml`, so cinch's renderer never sees or resolves against it.
A shipped template can only reference `{{paths.docs}}` (an undefined `{{key}}`
is a render error, not a blank), so any reference to this richer manifest has
to live in project-owned prose the *reader* — human or agent — resolves by
hand: a project's own doc saying "run the command named `test-backend` under
`development.commands` in `manifest.yml`," for instance. This keeps the
richer manifest entirely a project concern: no new render output, no schema
for cinch to version or validate.

Exit codes: `0` clean, `1` findings (`check`) or a render/dispatch failure,
`2` usage error.

`make test` builds `bin/cinch` and runs `go vet` plus every check's mutation
fixtures end-to-end.

## Layout

- `main.go` — CLI dispatch (`init`, `check`, `render`, `hook`, `workflows`,
  `workflow`, `index`, `ignores`); the only `package main` file — everything
  else lives in `internal/cinch`, a private package the Go compiler forbids
  other modules from importing.
- `internal/cinch/check.go` — the `Finding` model and `CmdCheck` orchestrator.
- `internal/cinch/links.go`, `internal/cinch/rules.go`,
  `internal/cinch/coupling.go`, `internal/cinch/generated.go`,
  `internal/cinch/commit.go` — one file per check, each paired with a
  `_test.go` carrying its mutation fixtures.
- `internal/cinch/render.go` — the `{{key}}` substituter, header/body-sha
  (comment-style aware), hook shims, and `CmdRender`; embeds
  `docs/philosophy.md` and the whole `docs/templates/` directory via
  `go:embed` (single static binary, no runtime template resolution) and
  iterates it, so adding a template needs no code change.
- `internal/cinch/title.go` — H1 title/trigger extraction, shared by
  `workflow.go` and `index.go`.
- `internal/cinch/manifest.go` — the `cinch.yml` parser and its
  accessors (`List`, `Names`, declaration order).
- `internal/cinch/hook.go` — `cinch hook`'s dispatcher: staged-set
  computation, `when` prefix matching, command execution.
- `internal/cinch/init.go` — `cinch init`.
- `internal/cinch/workflow.go` — `cinch workflows` / `cinch workflow NAME`,
  computed on demand from `workflows/` on disk.
- `internal/cinch/index.go` — `cinch index`, computed on demand from
  `paths.docs` on disk.
- `internal/cinch/docs/docs-philosophy.md` — copied verbatim into every consumer.
- `internal/cinch/docs/templates/*.md` — the nine workflow templates cinch
  ships, grouped by a shared filename prefix: the feature/bug dev lifecycle
  (`dev-plan-feature`, `dev-fix-bug`, `dev-execute-plan`, `dev-task-primitive`)
  and the docs-corpus toolkit (`docs-audit-quality`, `docs-audit-coverage`,
  `docs-maintain-domain`, `docs-sync`, plus `docs-philosophy` copied
  verbatim); each renders to `<paths.docs>/workflows/<name>.md`.
- `.docs/PRINCIPLES.md` — the spec the rebuild must satisfy: the six
  principles (deterministic over semantic, the direction rule, the
  derivability and holdability gates, rule→test markers, bind values not
  shape, no proxy metrics), each with the evidence that earned it and its
  rebuild constraint.
- `tests/` — CLI-level tests exercising the built binary.
- `Makefile` — `build`, `test`.

## Known limitation

The marker scan is deliberately textual, not Go-aware (principle 1: a
property is checkable only if it's decidable by a script, not a parser tied
to one language). `cinch check` is clean against this repo (the self-check
that makes self-enforcement possible), including its own test fixtures — a
`marker()` helper composes `// cinch:rule` text at runtime so no test file's
source carries a literal for the scan to find, and fenced markers in example
prose are skipped. The residual: a repo that legitimately quotes marker
syntax as a string literal *outside* a fenced code block is still scanned as
if it were a real marker. No file-name exclusion is built for it — that
would be exactly the kind of ad-hoc config the principles avoid.

## The rebuild

Stage 1 (three checks) and Stage 2 (render) are done. 0.1.0 closed the three
gaps that blocked cinch from being self-enforcing: an entry point
(`cinch init`), a check that proves the render output hasn't been tampered
with (`generated`), and a manifest-driven extension seam (`cinch hook` +
`hooks.*`) so a consumer's own scripts attach without hand-building git
hooks and a doc index from scratch. This repo is the proof — it runs
`cinch init` on itself and its own `cinch check` gates its own commits.
Next is Stage 3: a second real repo, and letting it strain.
