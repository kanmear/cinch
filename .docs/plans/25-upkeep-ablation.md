# 25. Upkeep ablation

**Tier 4 — evidence. Cost estimate: local runs, about two nights. This is the experiment
cinch's existence depends on.**

## Provenance

**Benchmark** (planned). This is claim #3 in [21](21-benchmark-suite.md). It closes the gap
between two earlier results in `~/code/cinch_bench/FINDINGS.md`:

- The layer ablation showed cinch's tooling adds nothing to **reading** the docs. Grep and a
  pointer were enough.
- The 6-of-6 vs 0-of-6 upkeep result compared docs-plus-cinch against **nothing**.

Nobody has measured whether cinch's tooling adds anything to **keeping docs true**, beyond what
the docs and the `AGENTS.md` pointer already do on their own.

## Arms

1. No docs.
2. Docs + `AGENTS.md`, no cinch (no hooks, no `cinch` on `PATH`).
3. Docs + `AGENTS.md` + cinch hooks.

## Tasks

Changes that break or extend existing rules, run on the fixture from
[23](23-public-fixture.md) (DeltaDocs as a labeled private data point). Each task's answer key
names the rules it touches, and whether a correct solution changes a rule, adds one, or must
leave all rules true.

## Grading (all mechanical)

- **Still true?** Hidden assertions per rule, run after the change.
- **New rules added and bound?** A new rule item exists and carries a `cinch:rule` marker on a
  test.
- **Bypassed?** Any `--no-verify` in arm 3, detected by the post-commit log from
  [19](19-field-log.md) or from the agent transcript.

## Pre-stated reading

- **If arm 3 doesn't beat arm 2**, the docs and the pointer do the work and the checker is
  optional. That gets published, on the front page.
- **If arm 3 wins**, that's the headline.
- This result also gates the longitudinal study (claim #8, item
  [07](07-workflow-prose-value-and-f1-scope.md)'s expensive half). Run that only if arm 3
  wins.

## Pre-registration

Before the first run, commit to `bench/upkeep/`: the task specs, the hidden assertions, the
grader, N per arm, the model(s) and a written prediction. The commit timestamp has to precede
every result file.

## Verification / definition of done

- Raw logs and a results CSV in `bench/upkeep/`.
- The pre-stated reading is applied to the result and published, whichever way it falls.
