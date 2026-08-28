# 4. Measure binding strength before designing for it

**Tier 2 — make green mean something. Cost estimate: one-off measurement, not a code change.**

## Provenance

**Benchmark** (measured), motivating a design question the audit raised abstractly. This item
is deliberately a *study*, not an implementation — it exists to decide whether items like
typed markers or mutation-verified bindings (raised in `~/code/cinch_bench/FINDINGS.md` §4.3)
are worth building at all.

## What a "binding" currently claims, and what it actually guarantees

A `// cinch:rule SIG-019` marker (matched by `markerRe` in `internal/cinch/rules.go:19`) is
cinch's only mechanism connecting a prose rule to code. `checkRulesFrom` verifies exactly one
thing about it: that the ID resolves — the marker's ID appears in some rule item's `id` field
(`rules.go:295-300`), and every rule item has at least one marker or an explicit
`cinch:ignore` (`rules.go:295-300` again, the inverse direction). That's a **name-resolution**
check, not a **behavioral** one. Nothing verifies that the code near the marker actually
implements, or would break without, the rule it's bound to.

The benchmark's T3 task demonstrated this is reachable by ordinary refactoring, not just
adversarial construction: `~/code/cinch_bench/tasks/T3.patch` deletes
`TestUnordered_FinalAcceptAfterDeclineResolvesDeclinedWithoutBaseline` — the only test
carrying the `SIG-019` marker — while leaving the marker's *comment* in place elsewhere in the
file (or moves it to a test that no longer exercises the invariant; see the patch for the
exact mechanics). `cinch check` reports `rules: ok`. The binding "resolves" in cinch's sense
right up until the moment it's meaningless.

## What this item asks

Before designing a fix (typed bindings that constrain what a marker may attach to; mutation
verification that a bound test actually fails when the guarded code is perturbed; something
else), measure how common the T3 failure mode already is across a real corpus. For each of the
~76 bound rules in `project_deltadocs`'s `.agent/` (the fixture used throughout the
benchmark):

1. Locate the marker and the code/test it's attached to.
2. Apply a small, targeted mutation to the guarded logic — the kind a refactor or a bug would
   plausibly introduce (flip a conditional, drop a field from a comparison, invert a boolean
   — the same class of change as the T3 patch itself).
3. Run the bound test (or, if the marker is on non-test code, the nearest test suite that
   should exercise it).
4. Record: did the test fail? If the marker is on a test, does *that* test still pass despite
   the mutation (a false-negative binding, exactly T3's shape)?

Aggregate: what fraction of the 76 bindings actually bite when the thing they claim to guard
is broken?

## Why measure before designing

Two outcomes, and they point at different amounts of work:

- **If most bindings bite** (the bound test reliably fails under mutation), T3 was closer to
  an edge case than a pattern, and typed bindings / mutation verification is a solution
  without a widespread problem — worth a lighter-weight fix (maybe just the near-miss/duplicate
  work in item 3, plus documentation of the risk) rather than new machinery.
- **If a large fraction don't** (mutation passes silently), this becomes the most important
  item on the list — cinch's core promise, that a marker means "code and doc are bound," would
  be unreliable at the rate this measurement finds, and a structural fix (typed bindings that
  can only attach to specific AST shapes, or a `cinch verify`-style mutation pass) is
  justified by data rather than by one adversarial example.

Building typed bindings speculatively, before knowing which of these worlds is true, risks
solving a rare problem with permanent complexity (every marker author now has to satisfy a
type constraint) or under-solving a common one (a documentation footnote where a structural
guarantee was needed).

## Scope and method notes

- This can run entirely on `project_deltadocs` as it exists today — no new fixture needed,
  reusing the corpus already characterized in `~/code/cinch_bench/FINDINGS.md`.
- Mutation should be scoped to the *guarded* logic specifically, not random code nearby — the
  question is "does this binding detect a violation of the rule it claims," not general
  mutation-testing coverage of the file.
- Where a marker sits on production code rather than a test (check the actual distribution —
  the benchmark's fixture bound most markers to tests, per `FINDINGS.md`'s discussion of
  SIG-019, but this should be confirmed rather than assumed for all 76), "does a test fail" may
  require identifying the nearest covering test rather than running one already named by the
  marker.
- Record results per-rule in a simple table (rule ID, marker location, mutation applied, bound
  test/nearest test, result) — this becomes both the headline number (X/76 bite) and a
  reusable list of the specific bindings that don't, useful independent of which design
  direction gets chosen.

## Verification / definition of done

This item is done when the aggregate number exists and is written down (a short results note,
not necessarily a huge report — the roadmap and `FINDINGS.md` are the right home for it), and
the design decision for typed bindings / mutation verification is made *with that number cited*
rather than deferred indefinitely or decided from the single T3 anecdote alone.
