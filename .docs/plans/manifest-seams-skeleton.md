# cinch — document the "seams" pattern the richer manifest already half-covers

Status: **proposed** — not started.

## Context

The original audit behind this plan flagged `.agent/manifest.yml`'s
taxonomy/seams/`development.commands` layer as "100% a project_deltadocs
invention with zero cinch-core skeleton" — but on closer reading, README's
existing §"A richer project manifest" (README.md:240-276, landed `c31e645`)
already documents most of this: the `manifest.yml` convention name, the
`project`/`services`/`development.commands`/`taxonomy` shape, and the "cinch
never reads this file, prose resolves it by hand" boundary. That part of the
gap is closed; the audit finding was stale relative to `c31e645`.

What that README section does **not** mention at all is `seams` —
project_deltadocs' pattern of using `manifest.yml` to declare, per AI-agent
persona (e.g. `auditor`), an allowlist of commands that persona may run
(`seams.auditor.allow`, resolved through `development.commands`), enforced at
runtime by per-harness glue (`.claude/hooks/auditor-bash-guard.py`,
`.pi/extensions/auditor.ts`). This is a genuinely separate pattern from
taxonomy/commands — it's about *restricting* an agent, not describing the
project to one — and it has no mention, worked example, or pointer anywhere
in cinch-core's docs. A new consumer wanting the same persona-gating
capability has nothing to start from but reverse-engineering
project_deltadocs' two hand-written parsers (see
`auditor-guard-consolidation.md` for that duplication problem).

## Decision needed — how much cinch-core should say about seams

- **A — Add a `seams` sub-example to the existing README section.** Extend
  the `manifest.yml` worked example (README.md:252-266) with a `seams:` block
  and a paragraph explaining the pattern (declare-an-allowlist-per-persona,
  enforce-in-harness-glue), explicitly framed as *one more instance* of the
  richer-manifest convention, not a new cinch mechanism. Zero cinch code
  change, consistent with the "cinch binds values, not shape" boundary the
  existing section already draws.
- **B — Also point at a reference enforcement implementation.** Beyond the
  doc addition in A, note where a worked reference implementation lives (once
  `auditor-guard-consolidation.md` produces one) so a new consumer isn't
  starting from zero even for the harness-glue half.

Recommend **A** now, standalone; fold in the **B** pointer only after
`auditor-guard-consolidation.md` actually produces something to point at —
don't block this documentation fix on that separate, larger decision.

## Step 1 — extend the README's richer-manifest example

Add a `seams:` block to the existing `manifest.yml` example
(README.md:252-266), e.g.:

```yaml
seams:
  auditor:
    allow: [test-backend, lint-frontend]
```

Follow immediately with a short paragraph: seams declare, per persona, which
`development.commands` entries that persona's harness-side guard may run;
cinch never reads or enforces this (same boundary as the rest of the richer
manifest) — enforcement is the consumer's own per-harness glue.

## Step 2 — cross-link

Add a one-line pointer from this new paragraph to wherever
`auditor-guard-consolidation.md` lands (once it exists as a plan, and later
once it ships) — "see `auditor-guard-consolidation.md` for the shared
enforcement problem this pattern raises," so a reader isn't left wondering
who actually enforces this.

## Verification

- README renders/reads sensibly; the new example is consistent in style with
  the surrounding section.
- A fresh reader unfamiliar with project_deltadocs can, from this section
  alone, understand what "seams" are and why they exist, without needing to
  read project_deltadocs' `manifest.yml` or either guard implementation.

### Critical files

- `README.md` (§"A richer project manifest", ~line 240-276)

### Relationship to other plans

Small and independent; can land before or after `auditor-guard-consolidation.md`
(Step 2's cross-link is the only coupling, and it degrades gracefully to "no
link yet" if landed first). Narrower in scope than originally sketched — the
broader taxonomy/`development.commands` documentation gap this plan was meant
to close turned out to already be shipped in `c31e645`.
