# 20. Replay history for rule growth

**Tier 4 — evidence. Cost estimate: one script, run once at the end of the MVP push.**

## Provenance

**Field data** (retrospective). It's the companion to [19](19-field-log.md): the hook log
counts blocks and bypasses, and this item counts whether the corpus grew while code changed.

## Why

The missing-rule step (step 2 of `dev-plan-feature`) is the newest and most important part of
the workflow pack. It's also the part item [07](07-workflow-prose-value-and-f1-scope.md) could
not measure: the workflow ablation held the corpus fixed. If feature and fix merges land
without adding or changing rules, the step isn't working and the workflow prose still isn't
paying for itself. This replay needs no setup now, because it reads the repo after the fact.

## Method

For each merge onto `project_deltadocs`'s `dev` since the release:

1. Check out that commit in a temporary worktree (`git worktree add --detach`).
2. Run `cinch rules --json`.
3. Record: rule count, how many rules have markers, how many have `cinch:ignore`, whether the
   merge touched product code (`backend/`, `frontend/` outside `.agent/`), and the merge's
   branch type (`feature/`, `fix/`, …, taken from the `merge: <branch>` subject).
4. Diff each row against the previous one: rules added, rules changed, rules removed.

This is the "walk history, run at each commit" mechanism from item
[09](09-calibrate-heuristics-against-history.md). Write it as a reusable script so 09's
calibration replays and claim #7 (history replay) in [21](21-benchmark-suite.md) can reuse it,
rather than writing three bespoke loops.

## Headline number

The share of `feature/` and `fix/` merges that touched product code **and** added or changed a
rule.

## Decision rule (pre-registered, also stated in [19](19-field-log.md))

If that share stays close to zero across the MVP push, cut the dev workflows down to the
missing-rule step and the entry gate.

## Verification / definition of done

- A CSV in `~/code/cinch_bench/` with one row per merge, plus the script that produced it.
- The headline number is written down, the decision rule is applied, and the outcome is
  recorded against item 07's open question.
