# cinch — make the shipped workflow templates actually stack-agnostic

Status: **proposed** — not started.

## Context

The README claims cinch's workflow templates are "generic, with no assumption
about a project's stack or directory layout." `render_test.go`'s
`TestTemplates_NoStackSpecificPaths` partially enforces this, but its guard
regex (`stackSpecificPathRe`) only catches `{{paths.docs}}/(backend|frontend|api|models)`
— stack-specific paths *nested under the docs root*. It says nothing about
stack-specific paths and vocabulary everywhere else in the same templates,
which is where most of the leakage actually lives:

- `docs-audit-coverage.md:37,40,95,96` — `backend/tests/handlers`,
  `backend/tests/models` as the worked "look here" examples, plus two example
  rows (`SIG-001`, `SIG-002`) lifted straight from project_deltadocs' domain.
- `docs-sync.md:106-107` — "the frontend layer's conventions doc", "the
  backend layer's troubleshooting doc".
- `dev-task-primitive.md:44,144` — `manifest.taxonomy.layers` example values
  hardcoded as `backend-model, backend-handler, frontend-state, frontend-ui`.
- `dev-plan-feature.md:140,144,148` — worked test-file paths
  `backend/tests/<layer>/<domain>_test.go`,
  `frontend/tests/<area>/<component>.test.ts`,
  `frontend/e2e/<domain>/<feature>.spec.ts`.
- `dev-fix-bug.md:15,25,69,110,118,173,190,195` — "backend↔frontend" framing
  throughout, `make dev-backend`/`make dev-frontend`, `backend/tests/`,
  `frontend/tests/`, and a test-tier vocabulary (`integration (backend), unit
  (frontend)`) that assumes exactly two services named backend and frontend.

None of these are under `{{paths.docs}}`, so the existing guard doesn't see
them — they slipped through because the guard was written narrowly for the
one incident it was created to catch, not the general property. A consumer
with a different shape (single service, three services, a library with no
frontend at all) gets worked examples that don't map onto their repo, which
undercuts the "drop cinch into any repo" story this tool is going for.

## A second leak class: docs the templates assume exist

*Added 2026-08-16.* The same root cause — templates encoding one consumer's
repo shape — produces a second, distinct failure the stack vocabulary above
doesn't cover: templates that reference **docs under the docs root that cinch
neither creates nor checks**. `cinch init` makes only `plans/` and
`workflows/`, so every fresh consumer's rendered workflows point at files
that don't exist:

- `{{paths.docs}}/conventions.md` — 3 references.
- `{{paths.docs}}/overview.md` — 4 references.

Live in cinch's own tree right now: `.docs/` holds only `PRINCIPLES.md`,
`plans/` and `workflows/`, so both are dangling here. The `links` check
cannot see them — they are inline code spans, not relative markdown links,
which is exactly the blind spot that let them persist.

The references are not equally harmful, and only one is sharp:

1. **`dev-execute-plan.md:66` — "Load `{{paths.docs}}/conventions.md` § Git
   before proposing the first commit message of the session,
   unconditionally."** A hard instruction to load a missing file. This one
   should point at the manifest instead of a doc: `commit.pattern` is now the
   enforced home of a project's commit convention (see README § Workflow
   postconditions), so the line can name a mechanism every consumer has
   rather than a doc most won't.
2. `dev-fix-bug.md:77,100` — "prefer … and `{{paths.docs}}/conventions.md`".
   Soft; degrades fine when absent. Use the idiom `docs-sync.md:106` already
   establishes elsewhere: "find it via `cinch index`".
3. `docs-sync.md:70,104` and the `overview.md` references — a routing table
   saying *where to put* things. Prescriptive rather than assumptive; leave
   as-is unless Step 1's rewrite touches the surrounding prose anyway.

Deliberately **not** proposed: creating `.docs/conventions.md` in this repo.
That fixes cinch's instance, leaves the class untouched for every other
consumer, and adds a doc whose only reader is a template line item 1 changes.
Also not proposed: a check for unresolved inline `{{paths.docs}}/…`
references. It would fire in every consumer on day one for docs they
legitimately haven't written, and the fix here is to stop asserting the docs
exist, not to demand that they do.

## Decision needed — how to genericize

- **A — Replace with domain-neutral placeholders.** Swap `backend`/`frontend`
  for something like `<service-a>`/`<service-b>` or a single generic
  `<component>`, and drop the SIG-001/SIG-002 example rows in favor of
  clearly-synthetic placeholder rules (e.g. `RULE-001`). Zero schema growth;
  purely a prose rewrite. Risk: genericized examples read as more abstract and
  less immediately legible than a concrete worked example.
- **B — Parameterize via new template vars.** Add `{{vars.layers}}` or similar
  so a consumer can substitute their own layer names/test-tier vocabulary via
  `cinch.yml`. More correct per-consumer, but grows the manifest schema for a
  feature only the templates need — the kind of schema growth other plans in
  this repo have been wary of ("shape is the real variance" — see this
  plan's own recommendation below, and the same reasoning in the 0.1.0 plan,
  deleted in `0ed1ebd`).

Recommend **A**. The worked examples are illustrative, not load-bearing config
— they exist to show an agent *the shape* of a good answer, not to encode
this consumer's actual layer names. A well-chosen neutral placeholder (e.g.
"the `<layer-a>` and `<layer-b>` directories your project's layout implies")
carries the same teaching value without hardcoding two-service-web-app as the
only shape cinch templates understand. Reserve **B** only if genericized
prose turns out to read as meaningfully worse in practice — verify by
rendering into a non-backend/frontend scratch repo and reading the result
cold.

## Step 1 — rewrite the offending templates

For each file/line listed in Context: replace the hardcoded `backend`/
`frontend` vocabulary and the two lifted domain-example rows with neutral
placeholders. Keep the *structure* (worked example present, same level of
concreteness) — only the vocabulary changes. `dev-fix-bug.md`'s
"single layer" vs "full workflow (spans layers)" distinction should survive
in generic form (e.g. "spans components" instead of "backend↔frontend").

Same pass, per the second leak class above: repoint
`dev-execute-plan.md:66` at `commit.pattern` rather than
`{{paths.docs}}/conventions.md`, and soften `dev-fix-bug.md:77,100` to
"find it via `cinch index`". One pass over these files, not two — each pass
forces a re-render commit in every consumer.

## Step 2 — widen the regression guard

`TestTemplates_NoStackSpecificPaths` currently only matches paths nested
under `{{paths.docs}}`. Either:
- widen `stackSpecificPathRe` to also match bare `backend/`, `frontend/`,
  `backend-`, `frontend-` tokens anywhere in template bodies, or
- add a sibling test with its own word-list check (`backend`, `frontend`, and
  any other consumer-specific vocabulary discovered during Step 1).

Whichever is chosen, the test's own doc comment should state what class of
leakage it guards (not just the one incident), so the next occurrence of this
bug gets caught at the same layer instead of requiring another audit.

## Verification

- `make test` green.
- `grep -rn "backend\|frontend" internal/cinch/docs/templates/*.md` returns
  nothing (or only intentional, clearly-labeled example text if any survives
  by design — confirm none does).
- Re-render `project_deltadocs` (`cinch render`); confirm `cinch check`'s
  `generated` check stays clean (the rendered output changes, so the consumer
  needs a matching re-render commit — call this out, don't silently break
  their `generated` check).
- Hand-read the rewritten templates in a scratch non-backend/frontend repo
  context to confirm the worked examples still teach the intended shape.
- `cinch init` a scratch repo, then grep the rendered workflows for
  `{{paths.docs}}`-rooted doc references that don't resolve on disk: only the
  prescriptive routing-table entries (leak class 2, item 3) should remain.

### Critical files

- `internal/cinch/docs/templates/dev-execute-plan.md` (leak class 2, item 1)
- `internal/cinch/docs/templates/dev-fix-bug.md`
- `internal/cinch/docs/templates/dev-plan-feature.md`
- `internal/cinch/docs/templates/dev-task-primitive.md`
- `internal/cinch/docs/templates/docs-sync.md`
- `internal/cinch/docs/templates/docs-audit-coverage.md`
- `internal/cinch/render_test.go` (`stackSpecificPathRe` / `TestTemplates_NoStackSpecificPaths`)
- `project_deltadocs/.agent/workflows/*.md` (re-render fallout)

### Relationship to other plans

Independent. Complements the richer-manifest convention, which has since
shipped as a scaffolded `manifest.example.yml` (`8a7c375`): that is the
*optional* pattern a consumer opts into, whereas this plan fixes the
*default* templates every consumer gets regardless. Worth landing before
`onboarding-checklist.md`'s dogfood run, so that run isn't distracted by
template examples it can't map onto a scratch repo.
