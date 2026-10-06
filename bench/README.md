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
| #6 It's cheap enough to run on every commit | [Cost curve](cost-curve.csv) | Holds to ~10k files (under 1 s); fails at 100k (7.3 s), and the cause is found | `make cost-curve` |

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
(the last before binary files were skipped) and 0.9.2. The machine is recorded in
[cost-curve.env](cost-curve.env): Ryzen 9 6900HS, 16 threads, btrfs, run 2026-10-07.

**Result.** Median ms per commit for cinch 0.9.2:

| Tracked files | No binary, fast | No binary, snapshot | 100 MB binary, fast | 100 MB binary, snapshot |
|---|---|---|---|---|
| 1,000 | 100 | 99 | 93 | 369 |
| 10,000 | 819 | 869 | 761 | 1,070 |
| 100,000 | 7,276 | 7,757 | 7,292 | 7,979 |

- **Cost grows linearly, at about 73 µs per tracked file.** Claim #6 holds up to around 10k
  files and fails at 100k: seven seconds on every commit is too slow.
- **Almost all of it is the marker scan, which runs twice.** At 100k files the `rules` check
  takes 3.5 s and the impact advisory repeats the same scan for another 3.5 s. Every other
  check together takes about 0.6 s. Each scan allocates about 136 KB of buffers per file
  and line-scans files with no marker in them. In a trial build, reading each file once and
  skipping files that don't contain `cinch:rule` cut the scan to 1.36 s and the hook to
  2.8 s. That fix isn't shipped yet, and this table will be re-run when it is.
- **Tracked binaries are now nearly free.** Before 0.9.1, cinch scanned them line by line at
  about 140 ms per MB: a 100 MB binary cost 14.5 s at 1k files. 0.9.2 reads only their first
  8 KB (CINCH-009). The snapshot path still copies the file, which costs about 270 ms for
  100 MB.
- **The snapshot path adds 0–10%.** Copying the index out is cheap next to the scan.
- **Real repos.** The four consumers in the field log (plan 19), each about 100–430 files,
  run their logged checks in 44–60 ms. That's `cinch check --staged` rather than the hook,
  and the repos are private.

**Limits.** This is one machine and one filesystem. The synthetic files are uniform (30-line
Go files), and the random binary has more newlines than a real compiled binary, so 0.9.0's
binary cost is somewhat overstated relative to DeltaDocs' 12 MB server (850 ms there, 1.5 s
for 10 MB here). The impact advisory's cost is included because the hook pays it, even though
it never blocks a commit.

**Reproduce.** `make cost-curve` builds both versions and rewrites `cost-curve.csv` and
`cost-curve.env`. The repos are cached under `~/.cache/cinch-costcurve` (about 1 GB). Pass
`BENCH_ARGS='-files 1000 -runs 3'` for a quick run.
