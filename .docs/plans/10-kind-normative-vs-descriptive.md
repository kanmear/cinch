# 10. `kind:` — normative versus descriptive

**Tier 3 — decide before investing. Cost estimate: design work (the deepest item on this
list).**

## Provenance

**Both**, and explicitly the item the roadmap calls out as where the audit and the benchmark
"converge hardest." The audit reasons from the code's own design: cinch has no concept of a
rule's *kind*, only its ID and marker-resolution state. The benchmark reasons from measured
value: most of `project_deltadocs`'s corpus restates what the code already says, and that
redundant majority is exactly what carried no measurable benefit in the retrieval battery
while the few rules encoding non-code-visible intent (the `PERM-003` class — a rule about
something the code does *not* settle on its own) carried the entire effect.

## The claim, stated precisely

Not "most rules are bad" — the claim is narrower and more useful: rules split into two kinds
that need different treatment, and cinch currently treats them identically.

- **Normative rules**: constraints the system must satisfy that are *not* independently
  visible in the code — an invariant, a business decision, a "this looks like two valid
  approaches but only one is correct" disambiguation (the exact shape of the corrected SIG-015
  finding in `FINDINGS.md` §1.1). These deserve bindings, tests, enforcement — losing them
  loses information the code can't reconstruct.
- **Descriptive rules**: restatements of what a reader could derive from the code itself. These
  aren't worthless as onboarding material, but they're a **liability** in cinch's specific
  sense: nothing depends on them staying accurate, so nothing notices when they drift, and
  volume of descriptive rules dilutes the corpus's signal-to-noise for the rules that actually
  matter (this is the mechanism behind the benchmark's finding that 11 of 16 retrieval
  questions sat at ceiling regardless of arm — those were, in effect, descriptive-shaped
  questions).

## Why this is "the admission test made mechanical"

`docs/docs-philosophy.md` (embedded via `render.go:18-19`, rendered into every consumer's
`.docs/workflows/`) already carries an **admission test** in prose — guidance on what earns a
rule and what doesn't, which is precisely how a mature corpus like `project_deltadocs`'s
avoided becoming bloated with restated-code rules in the first place. The benchmark's cross-
reference (`FINDINGS.md` §7) found the admission test is "empirically the right filter" — it's
the single strongest predictor of a corpus's measured value. But it's prose, applied by
whoever's writing a rule, unverified by anything mechanical. `kind:` would be the frontmatter-
or per-rule-level tag that makes the admission test's judgment **checkable** rather than
purely aspirational — the same move item 6 makes for `owns:`, applied to the deeper question of
what a rule is *for*.

## Why this reframes items 4 and 6 as means to it, not ends in themselves

- **Item 4** (measure binding strength) becomes sharper once rules are typed: a weak binding on
  a *normative* rule is the T3 failure mode at full severity — a silently broken guarantee.
  The identical weak binding on a *descriptive* rule is much lower stakes, since nothing beyond
  onboarding quality depended on it. Calibrating binding-strength tooling (item 9's discipline,
  applied to item 4's measurement) against an undifferentiated rule set risks over- or under-
  weighting the finding; splitting by `kind:` first would make item 4's number more
  actionable.
- **Item 6** (`rules --json` + `owns:` → `cinch impact`) becomes more useful with a `kind:`
  field in the emitted JSON: a pre-commit advisory that says "this diff touches 3 normative
  rules and 7 descriptive ones" is more actionable than an undifferentiated count, and lets a
  future `cinch impact` (or a stricter mode of it) treat the two differently — maybe descriptive
  rules never block, only normative ones do, once enforcement is layered on at all (which the
  roadmap's "what's deliberately not here" table explicitly defers until this item resolves).

## What this item does *not* yet specify

This is deliberately left as a design question, not a schema, because the roadmap frames it as
"design work" rather than an implementation task with a clear shape:

- Where `kind:` lives — per-rule (each numbered item gets its own tag, likely infeasible given
  `parseRuleItems`'s current per-item granularity would need a new inline annotation syntax,
  parallel to how `cinch:ignore` already works as an inline HTML-comment annotation within a
  rule item's text, per `ignoreRe` in `rules.go:18`) versus per-doc (a frontmatter field
  covering every rule in the file, cheaper to parse — reusing item 6b's frontmatter extractor
  — but coarser, and probably wrong if a single doc mixes normative and descriptive rules, which
  `project_deltadocs`'s corpus likely does).
- Whether `kind:` is binary (normative/descriptive) or needs a third state for "onboarding-only,
  intentionally descriptive and fine with that" — distinct from "descriptive because nobody
  applied the admission test carefully," which is the failure mode worth surfacing versus the
  legitimate use worth leaving alone.
- Whether cinch should ever *retroactively suggest* a kind for existing rules (some kind of
  heuristic pass over a mature corpus like `project_deltadocs`'s 76 rules) or only enforce it
  going forward on newly authored rules — the former is much more valuable for adoption
  (nobody wants to hand-tag 76 existing rules) but is itself exactly the kind of heuristic item
  9 says must be calibrated before shipping, not assumed to work.

## Why tier 3, last

This is "the deepest item," placed last deliberately: it depends on item 6's machine surface
existing (nowhere to put or query `kind:` without `rules --json` and frontmatter parsing), and
its design benefits from item 4's binding-strength data and item 9's calibration discipline
being established practice by the time it's tackled, so the retroactive-suggestion question
above isn't answered by guessing.

## Verification / definition of done

This item's "done" is a design decision plus a small reference implementation, not a full
migration of any real corpus:

- A written decision on placement (per-rule vs. per-doc), state space (binary vs. ternary),
  and retroactive-vs-forward-only scope, each with the reasoning above referenced or extended.
- `kind:` parsed and emitted through `rules --json` (item 6a) for at least a hand-tagged subset
  of `project_deltadocs`'s corpus, as a proof that the schema is usable in practice, not just on
  paper.
- One demonstrated consumer of the field — even a simple report ("N normative, M descriptive,
  here's the split") — showing the distinction is queryable, ahead of committing to any
  enforcement behavior that treats the two kinds differently.
