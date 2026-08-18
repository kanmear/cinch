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

`cinch version` prints the installed binary's version (`devel` for a `go
build`/`go install` that skipped `make`'s `-ldflags`).

The generated git hook shims `exec cinch hook <event>` — a bare PATH lookup,
deliberately, so a shim never hardcodes one machine's path. The consequence is
that the binary has to be *installed*, not just built: `make build` produces
`./bin/cinch` for local iteration, but a repo whose hooks are activated needs
`make install`, or every commit fails with `cinch: not found` (exit 127)
before a single check runs.

Re-install after changing cinch itself, too. The workflow templates are
compiled into the binary (`go:embed`), so a stale installed binary re-renders
stale templates and the `generated` check reports a mismatch that isn't
really there — a confusing failure, since `make build && ./bin/cinch check`
passes while the commit hook rejects the same tree.

## Found a problem in a rendered file?

If you're reading a workflow or doc under `<paths.docs>/workflows/` (or
another render output) and it's wrong — stale advice, a bad ordering claim, a
missing case — don't edit it in place. `generated` will flag the hand-edit as
tamper (correctly: it's a byte-for-byte check against what `cinch render`
would produce right now), and the fix would be lost on the next re-render
anyway. The fix belongs upstream, in the source cinch renders from:
`internal/cinch/docs/templates/*.md` for a workflow, `docs/philosophy.md` for
the philosophy doc. Open the issue against cinch, not the consumer repo — the
consumer repo doesn't own the content, only a rendered copy of it. Once the
fix ships and you've upgraded (see § Version-skew note below for the general
mechanic), `cinch render` picks it up for every consumer, not just yours.

## Adopting cinch: the linear version

Everything below is documented in full elsewhere in this README. This is the
order to do it in, not a second explanation — run it inside the repo you're
adopting cinch into.

1. **`make install`** — from a cinch checkout, once per machine. Not `make
   build`; see § Install just above for why, including the stale-binary
   failure.

2. **`cinch init`** — in the target repo. Writes `cinch.yml` and `AGENTS.md` if
   they're absent, creates `<paths.docs>/plans/`, renders the workflows and the
   hook shims, scaffolds `manifest.example.yml`, and activates the hooks with
   `git config core.hooksPath`. See § `cinch init` and self-enforcement for the
   idempotence guarantee and the outside-a-git-repo behavior.

3. **Point `paths.docs` somewhere else, if `.docs` doesn't suit** — set it in
   `cinch.yml` *before* the first `cinch init`, and it must name a directory
   inside the repo. `project_deltadocs` uses `.agent` as a worked precedent,
   over a pre-existing directory rather than a fresh one.

   **Changing it after `init` has already rendered needs a move, not just an
   edit.** cinch never moves or deletes files on its own, so editing
   `paths.docs` without moving the directory leaves the old tree behind, full
   of generated files nothing scans. Use `cinch move-docs NEW-PATH` instead
   of hand-editing the key: it does the `git mv`, the `cinch.yml` rewrite,
   and the re-render as one atomic step, so the manifest can never point at a
   path the directory hasn't followed. See § Known limitations.

4. **Write the rules you want enforced** under `paths.docs`: numbered
   `1. **ID-NNN** ...` items in a markdown doc, a `// cinch:rule ID-NNN`
   marker above the test that enforces each one, and
   `<!-- cinch:ignore: <reason> -->` under any rule no test can cover. See
   the `rules` check in § Status for the exact shapes. Nothing here is
   cinch-specific vocabulary — pick your own ID prefixes.

5. **Register your project's own checks** as
   `hooks.<event>.<name>.run` entries with a `when` path scope, so they run
   on the commits that touch the relevant paths and nowhere else. This is
   where project-specific automation belongs — never in cinch itself. While
   you're here, see § Workflow postconditions belong in the manifest: if a
   workflow you've adopted makes a demand a script can decide, this is where
   that demand becomes real.

6. **(Optional) adopt the richer manifest convention** — `cinch init` already
   dropped a commented `manifest.example.yml` at your repo root as a starting
   point. See § A richer project manifest.

7. **`cinch check` exits 0.** That's the adoption criterion. Until you've
   written rules, `links` and `rules` pass trivially, `generated` verifies
   what `init` rendered, and `coupling`/`commit` announce themselves as
   skips rather than passes.

8. **Tell your harness to start with `cinch context`** — `cinch init`'s
   `AGENTS.md` already says so if it wrote the file; add the line yourself if
   you had one already.

## Status: 0.1.0 — init, self-enforcement, manifest-driven extension

`cinch init` scaffolds a new consumer end to end (see Overview above) — one
command, idempotent. `cinch check` runs six checks, including one that
verifies the render output on disk hasn't been hand-edited or fallen out of
date, and another that enforces a commit message convention. Git hooks are
generated and, once activated, run `cinch check` on every commit — a project
can also hook its own scripts into `pre-commit`/`commit-msg` through the
manifest, scoped to the paths that should trigger them. This repo self-hosts:
its own `cinch.yml`, rendered docs, and `.githooks/` are the proof.

`cinch check` runs six checks against the current directory. The docs root
defaults to `.docs` and needs no configuration; a project can point it
elsewhere with `paths.docs` in `cinch.yml` — anywhere **inside the
repository**. A value that escapes it, absolute or `../`-relative, is refused
when the manifest loads, naming the key: cinch renders into that directory
and scans it, and a root outside the repo sends those two resolutions to
different places, leaving every check green against a corpus none of them
can see. `paths.hooks` carries the same constraint for the same reason.
`cinch render` substitutes the same value into
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
- **identity** (error) — a rule ID present at `HEAD^` is absent at `HEAD`
  with no recognized tombstone in its place — the gap `coupling` structurally
  can't see, since its own window goes dark the moment a change is
  committed. A tombstone convention is project-defined: set `rules.tombstone`
  in `cinch.yml` to a regexp, and an ID counts as tombstoned when a match
  against the file's `HEAD` content contains the ID's own text. Absent the
  key, no convention is recognized and any disappearance is a finding.
  Bounded to one commit back; see § Known limitations.
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
  hasn't run yet. Also scans the docs/hooks roots implied by `cinch.yml` as
  of `HEAD`, not just the working copy's current value — an uncommitted
  `paths.docs` edit that wasn't paired with moving the directory (`cinch
  move-docs` exists to make that impossible) leaves the old root's generated
  files orphaned, and this is what catches it instead of five clean checks
  over a directory nothing else scans. Bounded to one commit back; see §
  Known limitations.
- **commit** (error) — when `MSGFILE` is given and the manifest's
  `commit.pattern` key is set, the commit's subject line must match it.
  Absent key, no behavior change.
- **core** (error) — when the manifest's `require.cinch` key is set, it must
  match the running binary's version (`cinch version`). Absent key, no
  behavior change; an unreleased (`devel`) binary skips the comparison so a
  local dev build never reddens a consumer that pins a release.

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

`cinch context` is the third of that family and the one meant to run first:
it prints what a session needs to re-orient — current branch, the plans
under `plans/` with their `Status:` lines verbatim, the workflow trigger
table, and the staged set. Every input is decidable from the tree, so there
is no model in it and nothing to persist. It is deliberately **not a check**:
no findings, no green mark, and no exit code beyond 0 or an unresolvable
docs root — a session-start command that quietly ran the checks would be a
sixth check whose green state is "you have read your context." Missing
inputs (no git repo, no plans, render not yet run) announce themselves as
skips and the remaining sections still print, because the first command a
session runs must not refuse to run.

Plan statuses print verbatim. A project can optionally declare its own
vocabulary to give them a running order, so live work reads first:

```yaml
plans:
  statuses: [proposed, in progress, partially shipped, blocked, analysis]
```

Absence-based, the same contract as `commit.pattern`: with no key, plans sort
by path. Matching is a case-insensitive prefix test with markdown emphasis
stripped, so `**partially shipped** — Step 1 landed` matches the term
`partially shipped` without the vocabulary dictating the rest of the line.
**Ordering only** — an unrecognized status sorts last and prints unchanged;
it is never a finding, because a plan may say anything about itself and this
is a report, not a check. The vocabulary is per-project because lifecycles
differ: cinch's plans are `proposed`/`partially shipped` and have no terminal
"shipped" state at all (a shipped plan is deleted, not marked), where another
consumer's are `open`/`complete`.

It is harness-neutral by construction: a harness with a session-start hook
calls it, and a harness without one has AGENTS.md say "run `cinch context`."
The distribution is the point — it lives in the repo, versioned and
invocable by any harness or by a human, rather than in per-harness config
that evaporates with the session.

**Version-skew note:** upgrading the cinch binary can redden every
consumer's `generated` check until `cinch render` is re-run. `require.cinch`
in `cinch.yml` (see § `core` check above) makes this a decidable, enforced
property instead of a hope: set it to the version a project expects, and a
mismatch is a `core` finding naming both versions and the exact fix (`make
install` / re-run `cinch render`) — not a fuzzy warn, which was considered
and rejected. Absent the key, nothing changes; `cinch version` still lets
you check by hand.

### `cinch init` and self-enforcement

`cinch init` is the entry point: run it once in a repo (a fresh one, or an
existing one adopting cinch) and it writes a starter manifest if none exists,
creates `paths.docs/plans/`, renders everything, and — in a git repo — runs
`git config core.hooksPath <paths.hooks>` to activate the generated hooks.
Idempotent: running it twice produces a byte-identical tree. Outside a git
repo, the shims are still generated but init says on stderr that they
weren't activated.

Once activated, `.githooks/pre-commit`, `.githooks/commit-msg`, and
`.githooks/post-commit` all `exec cinch hook <event> "$@"`, which runs any
`hooks.<event>.<name>` entries a project has registered in its manifest,
scoped by `when` — a comma-separated list of path prefixes (not globs).
`pre-commit` and `post-commit` match `when` against the staged and committed
set respectively (`post-commit`'s committed set is `git diff --name-only
HEAD^ HEAD`, falling back to the commit's full tree for a root commit with no
parent); `pre-commit` and `commit-msg` also run `cinch check` first,
`post-commit` does not — the checks already ran before the commit existed,
and git fires `post-commit` for commits `pre-commit` never gated (e.g. `git
commit --amend`). Every registered entry runs regardless of an earlier one's
failure (failures accumulate, not fail-fast), and a skipped entry says so on
stderr, so a hook run that did nothing is never silent. `when` is ignored for
`commit-msg` — a commit message has no changed paths to scope against. A
`hooks.commit-msg.*` entry's command receives the commit message file path
as `$1`, the same argument git itself hands the `commit-msg` hook —
`pre-commit`/`post-commit` entries get no positional args.

**Extension boundary:** cinch guarantees *dispatch* — did the `when` prefixes
match the staged/committed set, did the registered command run, was its exit
code propagated — not your script's correctness. What a registered script
does is opaque to cinch; a green `cinch check` means dispatch worked, not
that the script itself is bug-free. This includes reentrancy: a
`post-commit` script that amends its own triggering commit re-fires
`post-commit` (the amend is itself a commit), so a script that mutates and
amends owns its own loop guard — cinch runs the dispatch table once per
event, it does not deduplicate a script re-triggering itself.

**Worked example — a project-specific referential check.** The `error-codes`
entry in the `cinch.yml` example just below is a real pattern, not a
placeholder: a consumer with a frontend/backend split can have a domain rule
cinch has no vocabulary for — every `@throws {ApiError} CODE` annotation in
its TypeScript API client must resolve to a matching Go error constant *and*
a translation key in every locale file, a three-way, cross-language
contract. That's not a rule cinch should learn to check (it's specific to
one project's error-handling convention, not a general property of docs or
code), and it's not a gap in `hooks.*` either — it's exactly what the seam is
for. All three prefixes below are registered together on the same entry
because a change on any one side of the contract should re-trigger the
check, not just a change to the side that happens to match a narrower
`when`. This is the general shape for "a check specific to one project's
domain, not cinch-worthy": write the script, register it once, done — no
semantic harness validator, no generic `make docs-check` target, no
project-manifest schema for cinch to grow and enforce. Those are the three
different ways a check like this tends to get proposed when the `hooks.*`
seam isn't the first thing reached for; this is what already answers all
three, per project, for free.

```yaml
# cinch.yml
paths:
  docs: .docs           # docs corpus root
  hooks: .githooks       # where generated hook shims land

hooks:
  pre-commit:
    error-codes:
      run: scripts/check_error_codes.sh
      when: [frontend/src/lib/api/, backend/errors/, frontend/src/lib/translations/]
    frontend:
      run: make check-frontend
      when: [frontend/]

commit:
  pattern: '^\[[a-z-]+\] .+'

require:
  cinch: 0.1.0           # pinned core version; mismatch is a `core` finding
```

### Workflow postconditions belong in the manifest

A workflow is prose until something can decide whether it was followed.
`cinch workflow <name>` prints markdown; whether the agent that read it
actually ran the tests, or actually used the commit convention, is not
knowable from the workflow's own sentences.

The discipline, which needs no cinch feature beyond what's above:

> When a workflow is adopted or written, its checkable postconditions get
> written into `cinch.yml` in the same commit.

"Checkable" is load-bearing. Most of what a workflow says is judgment and
stays judgment; the subset worth encoding is the subset a script can decide —
"tests must pass before this lands" becomes a `hooks.pre-commit` entry scoped
by `when`, "commits follow this convention" becomes `commit.pattern`. This is
the rule→test marker pattern one level up: a rule doc claims something and a
marker ties it to a test; a workflow claims something and a manifest entry
ties it to a command. Structural rather than semantic in both cases — a real
limit, and still better than prose alone.

This repo self-hosts the discipline. `dev-execute-plan.md` § Commit
Conventions declares a commit-message convention, and `commit.pattern` in
this repo's own `cinch.yml` is the half of it a script can decide; before
that key was set, the `commit` check reported "opt-in, not configured" on
every commit, which is to say the workflow's one decidable demand went
unenforced in the repo that ships the workflow.

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

The same reasoning rules out a fourth, hand-written thing: a per-project
"which docs to read for which kind of task" list. That list follows one
shape everywhere — an area's `architecture.md`/`conventions.md` pair for
work in that area, an `overview.md` for judgment calls anywhere under its
folder, everything else read when directly relevant — only the area names
differ per project, and those are exactly what `cinch index` already
prints. `cinch init` writes the shape once, generically, as a fourth
`AGENTS.md` line; no consumer needs to re-derive or hand-enumerate it.

`cinch.yml` binds the values templates and hooks reference — plain YAML, but
still no schema: nesting is notation for writing dotted keys hierarchically
(`paths: {docs: x}` and `paths.docs = x` bind the same thing), not a shape
cinch validates. `hooks.<event>.*` is the one place this is slightly
qualified: which per-entry keys mean something is event-dependent — `when`
is read and matched against the staged/committed set for `pre-commit` and
`post-commit`, but ignored for `commit-msg` (see above), so a `when:` key
written under a `commit-msg` entry parses fine and is silently never
consulted. Harmless today, and worth naming rather than leaving the "not a
shape" claim unqualified. Declaration order is preserved (hook entries run in
the order they're written). `paths.docs` is the one variable any shipped
template uses, and defaults to `.docs` when unset. The manifest itself
always lives at the repo root, independent of `paths.docs` — its own
location can't depend on a value it defines, so it's pinned outside the
directory `paths.docs` controls.

### A richer project manifest

`cinch.yml` only binds what cinch itself reads: `paths.*`, `hooks.*`,
`commit.pattern`, `require.cinch`. Workflow-template *prose*, though, can
reference arbitrary project-specific values cinch never touches — ports,
service layout, test taxonomy — and a project with enough of those is
better served by its own separate, schema-free file than by overloading
`cinch.yml`. Commands are deliberately not one of these: a project's
Makefile, self-documented (`target: ## comment`), already answers "what can
I run" on demand via `make help`; mirroring target names into the manifest
would just be a second, hand-maintained copy nothing re-checks against the
Makefile when a target is renamed.
There's no cinch convention format for this (by design — principle 5, cinch
binds values, not shape), but the convention name is `manifest.yml` —
deliberately distinct from `cinch.yml` so the two are never confused: one is
cinch's own config, the other a project's. `cinch init` writes a starting
point at the repo root, `manifest.example.yml`, commented with the why and
how — rename or restructure it as you like; cinch never reads it, so nothing
about its shape is enforced.

One value worth naming as a convention rather than inventing per-project: a
`permissions` key mapping a workflow name to a tool-call allow-list, e.g.
`permissions: {docs-audit-coverage: [Read, Grep]}`, for a harness's own
permission system to consume — a Claude Code `PreToolUse` hook or an
opencode auditor envelope, say. This is the one deliberately narrow piece
kept from an earlier, fuller design
(`.docs/plans/workflow-permission-convention.md` has the history); what's
kept is only the key shape, documented in `manifest.example.yml`. cinch has
no `PreToolUse` guard, no `cinch hook`-driven enforcement of this key, and no
opinion on which harnesses consume it — `cinch check` will never fail on it,
same as every other key in this file.

Note this manifest is outside cinch's `{{key}}` substitution entirely — it's
not `cinch.yml`, so cinch's renderer never sees or resolves against it.
A shipped template can only reference `{{paths.docs}}` (an undefined `{{key}}`
is a render error, not a blank), so any reference to this richer manifest has
to live in project-owned prose the *reader* — human or agent — resolves by
hand: a project's own doc saying "the `unit` test tier runs `test-backend`
under `taxonomy.test_tiers` in `manifest.yml`, i.e. `make test-backend`," for
instance. This keeps the richer manifest entirely a project concern: no new
render output, no schema for cinch to version or validate.

Exit codes: `0` clean, `1` findings (`check`) or a render/dispatch failure,
`2` usage error.

`make test` builds `bin/cinch` and runs `go vet` plus every check's mutation
fixtures end-to-end.

## Layout

- `main.go` — CLI dispatch (`init`, `check`, `render`, `move-docs`, `hook`,
  `workflows`, `workflow`, `index`, `context`, `ignores`); the only `package
  main` file — everything else lives in `internal/cinch`, a private package
  the Go compiler forbids other modules from importing.
- `internal/cinch/check.go` — the `Finding` model and `CmdCheck` orchestrator.
- `internal/cinch/links.go`, `internal/cinch/rules.go`,
  `internal/cinch/coupling.go`, `internal/cinch/identity.go`,
  `internal/cinch/generated.go`, `internal/cinch/commit.go` — one file per
  check, each paired with a `_test.go` carrying its mutation fixtures.
- `internal/cinch/render.go` — the `{{key}}` substituter, header/body-sha
  (comment-style aware), hook shims, and `CmdRender`; embeds
  `docs/philosophy.md` and the whole `docs/templates/` directory via
  `go:embed` (single static binary, no runtime template resolution) and
  iterates it, so adding a template needs no code change.
- `internal/cinch/title.go` — H1 title/trigger extraction, shared by
  `workflow.go` and `index.go`.
- `internal/cinch/manifest.go` — the `cinch.yml` parser and its
  accessors (`List`, `Names`, declaration order), plus `writeManifestValue`:
  an in-place, comment-preserving rewrite of one dotted key, used by
  `move-docs` instead of a YAML round-trip (which retains comment text but
  not layout).
- `internal/cinch/docsmove.go` — `cinch move-docs NEW-PATH`: moves the docs
  root (`git mv` in a git repo, plain rename otherwise), rewrites
  `paths.docs`, and re-renders as one operation.
- `internal/cinch/hook.go` — `cinch hook`'s dispatcher: staged-set
  computation, `when` prefix matching, command execution.
- `internal/cinch/init.go` — `cinch init`.
- `internal/cinch/workflow.go` — `cinch workflows` / `cinch workflow NAME`,
  computed on demand from `workflows/` on disk.
- `internal/cinch/index.go` — `cinch index`, computed on demand from
  `paths.docs` on disk.
- `internal/cinch/context.go` — `cinch context`, computed on demand: branch,
  plans, workflow trigger table, staged set. Reports state, never a verdict.
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

## Known limitations

**`identity`'s window is bounded to the single most recent commit.** Like
`generated`'s path-migration detection, it only ever diffs `HEAD^` against
`HEAD`: a rule ID removed several commits back is only caught if `identity`
happened to run (as a pre-commit hook, typically) on the very next commit
after the removal — once a second commit lands on top, the removal is no
longer the `HEAD^`→`HEAD` transition and drops out of view for good. A
tombstone added several commits after the removal, rather than in the same
commit, is invisible for the same reason: by the time it lands, the removal
itself is already outside the window. Consistent, regular use of the
pre-commit hook is what keeps the window from ever skipping a commit; a repo
that only runs `cinch check` sporadically can lose coverage this way.

**Changing `paths.docs` without `cinch move-docs` still strands the old
directory, and the safety net only reaches back one commit.** cinch only ever
creates files — it never moves or deletes them — so repointing `paths.docs`
by hand without moving the directory leaves the old tree's generated files
behind. `generated` catches this while the manifest edit and the stale
directory coexist *uncommitted*, by also scanning the docs/hooks roots
implied by `cinch.yml` as of `HEAD`: a rename committed several commits back
without ever moving the directory won't be caught retroactively, since the
check only diffs against the single immediately-preceding commit. The old
hand-authored docs under the stranded directory also drop out of `links`,
`rules`, `coupling`, `cinch index` and `cinch workflows` the moment the
manifest changes, committed or not — that half has no detector, only the
avoidance below. `cinch move-docs NEW-PATH` is the real fix: it makes editing
`paths.docs` and moving the directory one atomic operation, so this class of
drift can't happen through it at all. Use it instead of hand-editing the key.

**The marker scan is textual, not language-aware.** Deliberately (principle 1: a
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
