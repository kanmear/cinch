# 23. A public fixture

**Tier 4 — evidence. Cost estimate: days. It's the prerequisite for 24, 25 and audit recall.**

## Provenance

**Benchmark** (methodology). Every agent-run result so far comes from `project_deltadocs`,
which is a private product. A benchmark built only on it can't be reproduced, and that's the
weakness [21](21-benchmark-suite.md) exists to fix.

## What to build

Pick one of these two, in this order of preference:

1. **A sanitized slice of DeltaDocs**: permissions, signatures and realtime, with its rules,
   marked tests and enough history to replay. Strip product-specific names and anything
   proprietary, but keep the shapes that matter: rules spanning more than one doc, a realtime
   fan-out where the targeting decision lives, and Go tests carrying `cinch:rule` markers.
2. **A purpose-built app with the same shape**, if the slice can't be sanitized without losing
   the shape.

Either way, the fixture needs:

- A `cinch.yml` and a docs root that pass `cinch check`.
- A one-command test run (the hidden assertions in 24 and 25 run through it).
- Tagged states for each benchmark arm (for example, with the realtime targeting rule and
  without it, for [24](24-leak-replay.md)) so that an arm is a `git checkout`, not a hand edit.

The private repos still run as extra data points. Every published table labels them as
private and not reproducible.

## Verification / definition of done

- The fixture lives in a public location (`bench/fixture/` or its own repo, decided when built)
  and `cinch check` plus its tests pass from a clean clone.
- Someone without access to DeltaDocs can run the 24 and 25 harnesses against it.
