# Benchmarks

Evidence for the claims cinch makes, one benchmark per claim, each designed so that it could
prove the claim false. The plan behind this directory is `.docs/plans/21-benchmark-suite.md`,
and each benchmark has its own plan item.

## Rules this directory follows

- **Hypotheses and grading keys are committed before the run.** The same person built the tool,
  wrote the corpus and designed the tests, so the commit timestamp of a key is the evidence it
  wasn't fitted to the result.
- **Null results are published.** A benchmark that finds cinch adds nothing gets the same space
  as one that finds it helps.
- **Private repos are labeled.** Results from private consumers can't be reproduced, and say so.
- **Correctness and upkeep lead; cost is reported but kept out of the headline.**

## Contents

| Claim | Benchmark | Result | Reproduce |
|---|---|---|---|
| #1 The checker catches what it says it catches | [Catch matrix](catch-matrix.md) | 28 defect classes: 15 caught, 2 warn, 5 gaps, 5 correctly pass, 1 control | `make catch-matrix` |
| #6 It's cheap enough to run on every commit | [Cost curve](cost-curve.csv) | Holds: 31 ms at 1k files, 215 ms at 10k, 1.8 s at 100k (was 7.3 s before 0.10.1) | `make cost-curve` |

Claims #2–#5, #7 and #8 are listed in plan 21 and land here as they run.

## Catch matrix

**Method.** Every defect class has one test in `internal/cinch` that seeds it and asserts what
the owning check does: report it (caught), report a skip (warns), or pass. A pass is either a
known gap or something that only looks like a defect. Each test carries a
`// catch-matrix: <check> | <verdict> | <defect>` line, and `bench/catchmatrix` builds the
table from those lines. `TestCatchMatrixIsCurrent` fails `go test ./...` when the committed
table is stale, and the generator refuses to run while any `TestSeededDefect*` test lacks a
line.

**Limits.** The matrix covers what a unit test can seed. It says nothing about how often each
defect class happens in real repos (that's the field log, plan 19) or whether a bound test
really enforces its rule (that's binding strength, plan 04). Its gaps are the classes known
today; an unknown gap has no row by definition.

**Reproduce.** `go test ./...` runs every row. `make catch-matrix` regenerates the table.

## Cost curve

**Method.** `bench/costcurve` generates synthetic repos of exactly 1k, 10k and 100k tracked
files, with fixed shares: 5% docs that link to each other, one rule doc of 10 rules per 100
files, one marker per rule in code files, and rendered workflows. It optionally commits a
pseudo-random binary of 10 or 100 MB. Each cell stages one doc edit and times
`cinch hook pre-commit`: 7 timed runs after one warm-up, reporting the median. The "fast"
path has a clean working tree. The "snapshot" path adds one unstaged edit, which makes the
hook copy the whole index out with `git checkout-index`. Two builds run side by side: 0.9.0
(before either scan fix) and 0.10.1. The machine is recorded in
[cost-curve.env](cost-curve.env): Ryzen 9 6900HS, 16 threads, btrfs, run 2026-10-07.

**Result.** Median ms per commit:

| Tracked files | 0.10.1 fast | 0.10.1 snapshot | 0.9.0 fast | 0.10.1, 100 MB binary, fast | 0.9.0, 100 MB binary, fast |
|---|---|---|---|---|---|
| 1,000 | 31 | 42 | 88 | 31 | 13,874 |
| 10,000 | 215 | 331 | 807 | 199 | 15,047 |
| 100,000 | 1,765 | 3,210 | 7,556 | 1,738 | 19,455 |

- **Claim #6 holds.** Cost is linear at about 18 µs per tracked file on the fast path:
  31 ms at 1k files, 215 ms at 10k. At 100k files a commit costs 1.8 s on the fast path and
  3.2 s with unstaged changes. That's noticeable, but tolerable for a repo of that size.
- **Two fixes produced the gain.** Before 0.10.1 the hook scanned every file for markers
  twice (once for `rules`, once for the impact advisory), and line-scanned each file through
  about 136 KB of fresh buffers. Since 0.10.1 it reads each file once, line-scans only files
  that contain `cinch:rule`, and shares one scan across the run. At 100k files that's 7.6 s →
  1.8 s; at 1k files, 88 → 31 ms.
- **Tracked binaries are nearly free.** 0.9.0 scanned them line by line at about 140 ms per
  MB (a 100 MB binary cost 13.9 s at 1k files). Since 0.9.1 only their first 8 KB is read
  (CINCH-009). The snapshot path still copies the file: about 340 ms for 100 MB.
- **The snapshot path is now the larger cost at scale.** Copying the index out adds about 35%
  at 1k files and 80% at 100k (1.8 → 3.2 s). It only runs when the working tree has unstaged
  changes. Narrowing what the snapshot copies is the next lever if a large consumer needs it.
- **Real repos.** The four consumers in the field log (plan 19), each about 100–430 files,
  run their logged checks in 44–60 ms. That's `cinch check --staged` rather than the hook,
  and the repos are private.

**Limits.** This is one machine and one filesystem. The synthetic files are uniform (30-line
Go files), and the random binary has more newlines than a real compiled binary, so 0.9.0's
binary cost is somewhat overstated relative to DeltaDocs' 12 MB server (850 ms there, 1.5 s
for 10 MB here). Small cells move by about 20 ms between runs (a 1k-file, 10 MB cell measured
45 ms and then 68 ms), most likely from git's own index checks on freshly written files, so
read differences under ~30 ms as noise. The impact advisory's cost is included because the
hook pays it, even though it never blocks a commit.

**Reproduce.** `make cost-curve` builds both versions and rewrites `cost-curve.csv` and
`cost-curve.env`. The repos are cached under `~/.cache/cinch-costcurve` (about 1 GB). Pass
`BENCH_ARGS='-files 1000 -runs 3'` for a quick run.
