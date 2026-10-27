# 21. A benchmark suite worth publishing

**Tier 4 — evidence. Cost estimate: the umbrella for items 22–25; each item states its own cost.**

## Provenance

**Benchmark** (planned), shaped by a comparison with CodeGraph's published benchmark.
CodeGraph's benchmark had good external validity: seven real repos. But it measured only cost
(tool calls and tokens), never whether answers were right, and it couldn't be reproduced.
Cinch's benchmark (`~/code/cinch_bench/FINDINGS.md`) was the reverse: it measured correctness
and separated the layers, but it used one private fixture and lives outside the repo. A
benchmark set worth showing keeps cinch's rigor and fixes its reach.

The field data from [19](19-field-log.md) gets published as one piece of this set, not as the
proof on its own.

## One claim, one benchmark that could falsify it

| # | Claim | Benchmark | Cost | Priority | Item |
|---|---|---|---|---|---|
| 1 | The checker catches what it says it catches | Catch matrix: every defect class × caught / documented non-catch, including what cinch deliberately can't see, like a hollowed test | Hours, no models | Do first | [22](22-catch-matrix-and-cost-curve.md) |
| 2 | A passing check means the rules are actually tested | Binding strength: the share of bound rules whose test fails when the rule is broken. This is the counterpart to CodeGraph's per-language resolution rate | One local run, overnight | High | [04](04-measure-binding-strength.md) |
| 3 | Cinch keeps docs true as code changes | Upkeep ablation | Local, about 2 nights | Highest | [25](25-upkeep-ablation.md) |
| 4 | The missing-rule step prevents the realtime-leak kind of bug | Leak replay | Local, 1 night | High, and the strongest showcase | [24](24-leak-replay.md) |
| 5 | The audits catch what the checker can't | Audit recall: seed K falsified rules and hollowed tests, run `docs-audit-coverage`, score recall and precision against the known seeds | 1 night | Medium | Deferred, below |
| 6 | It's cheap enough to run on every commit | Cost curve: hook latency against repo size, fast path vs snapshot path | An hour, no models | Medium; it's honest about cost | [22](22-catch-matrix-and-cost-curve.md) |
| 7 | It doesn't cry wolf | History replay: `cinch check` on every historical DeltaDocs commit, how often it went red and why; plus the field log | One script | Medium | Deferred, below |
| 8 | The workflows produce a better corpus over time | Longitudinal run: N tasks in sequence, with and without workflows, then grade the accumulated corpus | Expensive | Later, if ever | Deferred, below |

## Current state (2026-10-12)

`~/code/cinch_bench/` arrived on 2026-10-11. [22](22-catch-matrix-and-cost-curve.md) and
[04](04-measure-binding-strength.md) are done: claims #1, #2 and #6 hold. On 2026-10-27,
[23](23-public-fixture.md) settled the fixture question without building one: Budgetron,
made public, is the reproducible fixture for [25](25-upkeep-ablation.md), and
[24](24-leak-replay.md) runs on DeltaDocs as a private data point. Both are unblocked.
`bench/binding` is the closest template for their harnesses: a cached clone, a fresh
database per run, and resumable per-item runs.

## Rules for publishing

- **Put it in the repo.** A `bench/` directory holds the harness, task specs, answer keys, raw
  CSVs and logs, plus a `bench/README.md` covering method, limits and how to reproduce. The
  main README gets one results table that links there.
- **Commit hypotheses and grading keys before running.** The same person built the tool,
  wrote the corpus and designed the tests, so the predictions and keys go in first, and the
  commit timestamp is the evidence. The earlier report already caught a rubric being edited
  mid-study. Committing first prevents that.
- **Publish the null results.** "The tooling adds nothing to retrieval" belongs on the front
  page. It's what makes the positive results believable, and it's what CodeGraph didn't do.
- **Fix the fixture problem.** DeltaDocs is a private product, so a benchmark built on it can't
  be reproduced. Item [23](23-public-fixture.md) makes Budgetron the public fixture, pinned
  to a tag per benchmark. DeltaDocs runs as a private data point, labeled as such.
- **Headline correctness and upkeep, not speed.** Report cost (#6) honestly, but keep it out
  of the headline.

## Order

1. Catch matrix and cost curve ([22](22-catch-matrix-and-cost-curve.md)). Cheap,
   deterministic, publishable this week.
2. Binding strength ([04](04-measure-binding-strength.md)), which was already planned.
3. Public fixture ([23](23-public-fixture.md)): decided, with Budgetron made public. No
   fixture gets built.
4. Leak replay ([24](24-leak-replay.md)).
5. Upkeep ablation ([25](25-upkeep-ablation.md)).
6. The field log at the end of the MVP push ([19](19-field-log.md), [20](20-rule-growth-replay.md)).

Audit recall comes after that. The longitudinal study runs only if the upkeep ablation says the
workflows are worth investigating further.

## Deferred claims

- **#5 audit recall.** This waits until 22–25 are done. It seeds into Budgetron, the public fixture from 23, and its answer key (the seeded set) must be committed before the run, like every
  other key.
- **#7 history replay.** It reuses the per-commit walker from [20](20-rule-growth-replay.md)
  and [09](09-calibrate-heuristics-against-history.md), running `cinch check` instead of
  `cinch rules --json`. It's published together with the field log, since the two answer the
  same claim from opposite directions (history before the hooks, observed commits after).
- **#8 longitudinal.** This is item [07](07-workflow-prose-value-and-f1-scope.md)'s expensive
  half. It's gated on [25](25-upkeep-ablation.md): run it only if arm 3 beats arm 2 there.

## Verification / definition of done

- `bench/` exists with a README, and every published number links to its raw data and to the
  commit that pre-registered its hypothesis and key.
- The main README carries one results table, null results included.
