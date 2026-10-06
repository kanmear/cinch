# 24. Leak replay

**Tier 4 — evidence. Cost estimate: local runs, one night.**

## Provenance

**Field data turned into a benchmark.** This is claim #4 in [21](21-benchmark-suite.md): the
missing-rule step prevents the realtime-leak kind of bug. The realtime leak was a real bug
in a real product, a restricted event delivered to recipients who shouldn't receive it. It's
the example the missing-rule step itself cites ("who may receive a realtime event that carries
restricted content").

## Why it's the best showcase

The task is mechanically graded, and it directly tests the newest workflow step. A result like
"with the rule, 0 of N leaked; without it, X of N" is more convincing than any rate in the
table.

## Task

"Add a `tab.updated` realtime event," run on the fixture from [23](23-public-fixture.md) (and
on DeltaDocs as a labeled private data point).

## Arms

1. **The corpus as it was**: no targeting rule, and the workflows present.
2. **The corpus with the targeting rule**: rule plus marked test present.
3. **The missing-rule step with no rule present**: like arm 1, but the agent is explicitly
   sent through `dev-plan-feature`'s missing-rule step.

## Scoring (mechanical)

- **Leaked?** A hidden permission test that the agent never sees asserts that only permitted
  recipients get the event.
- **Wrote the rule?** A new rule item about event targeting exists, and it has a `cinch:rule`
  marker on a test.

## Pre-registration

Before the first run, commit to `bench/leak-replay/`: the task prompt, the hidden test, the
scoring script, N per arm, the model(s), and a written prediction per arm. The commit
timestamp has to precede every result file.

## Verification / definition of done

- Raw per-run logs and a results CSV in `bench/leak-replay/`.
- A results line per arm (leaked k/N, rule written k/N), published in the README table,
  including a null result if arm 3 doesn't beat arm 1.
