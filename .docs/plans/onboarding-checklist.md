# cinch — a linear "adopting cinch" checklist, past `cinch init`

Status: **proposed** — not started.

## Context

README already documents each individual piece well: `## Install`
(README.md:51-65), `cinch init` and what it scaffolds (README.md:67-148+),
the `hooks.<event>.<name>.run`/`when` shape, `commit.pattern`, and the
optional richer `manifest.yml` convention (README.md:240-276). What's missing
is the thing this session's audit kept reaching for by hand: a single linear
walkthrough from "empty repo" to "ready for work," so a new consumer (or an
agent onboarding one) doesn't have to assemble the sequence themselves from
scattered subsections.

This gap is real but narrow — it's an *ordering and cross-reference* problem,
not a missing-content problem. Every fact the checklist needs already exists
in README; nothing here should introduce new cinch behavior or new
documented conventions beyond what `manifest-seams-skeleton.md` and the
existing richer-manifest section already cover.

## Step 1 — write the checklist

Add a new README section (recommend placing it directly after `## Install`,
before the `## Status: 0.1.0` history section, since a fresh reader wants
"how do I adopt this" before "here's what shipped when") — or a standalone
`docs/onboarding.md` if the README is judged too long already (it currently
runs 339+ lines; check current length before deciding placement). Content,
each step pointing at the existing section that already documents it rather
than re-explaining:

1. `make install` (→ ## Install).
2. `cinch init` in the target repo — what it creates, idempotency (→ `cinch
   init` subsection).
3. Set `paths.docs` in `cinch.yml` if the default `.docs` doesn't fit (point
   at the manifest section; note `project_deltadocs` itself uses `.agent`
   as a worked precedent).
4. Add domain docs/rules under `paths.docs` — numbered `**ID-NNN**` rule
   markers, `// cinch:rule ID` source markers, `<!-- cinch:ignore -->` for
   intentional exceptions (point at whatever section documents the rule
   marker format — confirm exact anchor when writing).
5. Wire `hooks.<event>.<name>.run`/`when` entries to project-specific scripts
   (pre-commit/commit-msg/post-commit) — point at the hooks section; note
   this is where all project-specific automation logic belongs, never in
   cinch-core.
6. (Optional) Adopt the richer `manifest.yml` convention for taxonomy/
   commands/seams (→ "A richer project manifest" + `manifest-seams-skeleton.md`
   once it lands).
7. `cinch check` clean — the exit criterion. Note what each of the 5 checks
   verifies at this point (mostly: links resolve, rule markers exist,
   generated output matches, commit pattern if configured).

## Step 2 — dogfood it

Walk a fresh scratch repo (`mktemp -d && git init`, same technique used
elsewhere this session for hook-behavior rehearsal) through the checklist
literally, step by step, using only what the checklist says — not tribal
knowledge. Fix any step that's ambiguous, out of order, or missing a
prerequisite the checklist didn't mention (e.g. does `cinch init` need to run
inside a git repo already, or does it degrade gracefully outside one — the
existing scaffolding survey found it skips `core.hooksPath` with a stderr
note when not in a git repo; the checklist should say so up front rather than
let a reader discover it via a confusing partial-scaffold).

## Verification

- The dogfood run in Step 2 completes with `cinch check` clean, using only
  the checklist's own steps.
- No content in the new section duplicates prose that already exists
  elsewhere in README — it should read as a table of contents with brief
  connective tissue, not a restatement.

### Critical files

- `README.md` (new section, or a new `docs/onboarding.md` if length dictates)

### Relationship to other plans

References `manifest-seams-skeleton.md` (Step 1, item 6) — land that first if
convenient, but this checklist degrades gracefully to "see the richer-manifest
section" if it hasn't landed yet. Independent of
`template-stack-agnosticism.md` and `auditor-guard-consolidation.md`.
