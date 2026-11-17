# Benchmarks

Evidence for the claims cinch makes, one benchmark per claim, each designed so that it could
prove the claim false. The plan behind this directory is `.docs/plans/archive/21-benchmark-suite.md`,
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
| #1 The checker catches what it says it catches | [Catch matrix](catch-matrix.md) | 42 defect classes: 19 caught, 2 warn, 5 gaps, 15 correctly pass, 1 control | `make catch-matrix` |
| #6 It's cheap enough to run on every commit | [Cost curve](cost-curve.csv) | Holds: 31 ms at 1k files, 215 ms at 10k, 1.8 s at 100k (was 7.3 s before 0.10.1) | `make cost-curve` |

| #2 A passing check means the rules are actually tested | [Binding strength](#binding-strength) | Holds: 72 of 75 bindings bite (96%), 67 through the marked test. Private fixture | `make binding-strength` |
| #3 Cinch keeps docs true as code changes | [Upkeep ablation](upkeep/DESIGN.md) | Pre-registered 2026-10-28 (`prereg/upkeep`); stage 1 running. Public fixture (Budgetron) | `go run ./bench/upkeep -engine pi` (see DESIGN) |

Claims #4, #5, #7 and #8 are listed in plan 21 and land here as they run.

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

## Binding strength

**Private fixture.** This runs on `project_deltadocs`, a private product, so it can't be
reproduced from outside. Plan [04](../.docs/plans/archive/04-measure-binding-strength.md) has the
reasoning; this section is the pre-registered method.

**Question.** A `cinch:rule` marker only proves that a rule ID resolves. When the code
breaks the rule, does a test fail, and is it the test the marker names?

**Pin.** DeltaDocs `ec4df00` (`dev`, 2026-10-09), which has 80 rules: 75 bound to Go tests
and 5 `cinch:ignore`d. The model is `Qwen3.8-27B-UD-Q4_K_M` on llama.cpp, driven by pi
1.1.0 at thinking `high`.

**Method.** `bench/binding` clones the pin into a cache directory and never touches the
real repo. It runs the backend suite (`go test -json -count=1 -p 1 ./...`) twice, and any
test that fails in either baseline run is excluded as flaky. Then for each bound rule, in
a clean tree:

1. pi gets [prompt.md](binding/prompt.md): the rule text and the source of its bound tests.
   It asks for the smallest production-code change that violates the rule. pi has
   read/grep/find/ls/edit and no shell, so it can't run tests or pick a mutation by its
   outcome.
2. A diff that touches a test file, anything outside `backend/`, or nothing at all is
   **invalid**.
3. `go build ./... && go vet ./...` must pass, which also compiles the test packages. If
   it doesn't, pi gets one retry with the error ([retry.md](binding/retry.md)). A second
   failure is **nocompile**. A build break would fail every test in the package, so it
   must never count as a bite.
4. The full suite runs, and the outcome is graded with flaky tests excluded:
   - **marked**: a test the rule's marker binds failed
   - **other**: only tests the marker doesn't bind failed
   - **silent**: nothing failed

   A bite is marked or other. Other is reported separately, because it means the marker
   points at the wrong test.

**Hand-check.** About 15 mutations, stratified across outcomes, are read against the rule
text. Every silent one is included if there are 15 or fewer. Each is labeled *violates*,
*partial* or *doesn't violate*. A mutation that doesn't violate its rule is a measurement
error: it's listed, and left out of the adjusted rate.

**Decision rule (from plans 04 and 19).** If more than about 90% of bindings bite, close
the question. If fewer than about 70% bite, add a mutation-check step to the coverage
audit. Between the two, decide from the silent list.

**Prediction, before the run** (kept as committed). About 80% bite, nearly all of them as marked. Silent cases
are expected where a rule spans several code paths and the bound test exercises only one,
and where the rule is a schema constraint the model can't change without a migration.

**Control.** Before any model call, T3's production-side bug (the SIG-019 resolution flip from
`~/code/cinch_bench/tasks/T3.patch`) was graded through `-apply` and came out marked, as it
must.

**Result** (run 2026-10-12, [binding-strength.csv](binding-strength.csv), one diff per rule in
[binding/mutations](binding/mutations), hand-check in [binding/handcheck.csv](binding/handcheck.csv)):

| Outcome | Rules | Share |
|---|---|---|
| marked: the bound test failed | 67 | 89% |
| other: only unbound tests failed | 5 | 7% |
| silent: nothing failed | 3 | 4% |
| invalid / nocompile | 0 | — |

- **Claim #2 holds, and the question closes.** 72 of 75 bindings bite (96%), above the
  pre-registered 90% line. Typed bindings and a mutation-check step aren't justified by this
  corpus. The prediction (about 80%) was too pessimistic.
- **The "other" five share one shape.** The marker sits on the test that checks the allowed
  case ("has edit → may manage files", "sign-level signer allowed"). The test that checks the
  forbidden case exists, and it caught the mutation, but carries no marker (FILE-007,
  PERM-003, SIG-005), or the marker sits on a neighbouring test (PROJ-004 on create rather
  than transfer, SIG-015 on the handler rather than the model). The rule is enforced, but
  the marker doesn't point at the test that enforces it. A test of the allowed case can't fail
  when a rule is loosened.
- **The silent three are real gaps in the fixture.**
  - CAT-001: listing categories without the project filter goes unnoticed, because no test
    has two projects.
  - SIG-004: pinning the oldest history row instead of the newest goes unnoticed, because
    the bound test saves only once.
  - MEMB-002: dropping `NOT NULL` from `members.role_id` is latent, since no code path
    writes a null role. The hand-check rates it *partial*.
- **Schema rules are well bound.** 13 mutations were edits to the baseline migration (a
  `UNIQUE`, `CHECK` or `NOT NULL` dropped, an `ON DELETE` changed). 12 were caught by
  their marked test.
- **Hand-check.** 19 mutations were checked: all 3 silent, all 5 other, and 11 marked. 18
  violate their rule and one is partial (MEMB-002). None was a non-violation, so the adjusted
  rate equals the raw one. Most mutations change 2 lines (the median). One needed the
  compile retry (CAT-002).
- **Cost.** A proposal takes a median of 34 s (at most 259 s) and 6 model turns. A suite run
  takes about 2 minutes. The whole run took 3 h 24 min.

**Deviations from the pre-registration**, both harness defects found while reading the
results. Each was fixed by re-grading the stored mutation, not by asking the model again,
so every proposal is still the one the model made first.

1. **Migrations never reached the database.** goose doesn't re-apply a version it has
   recorded, and the test database was reused, so every edit to `00001_baseline.sql` was
   tested against the unmutated schema. All 13 came out silent. The harness now drops and
   recreates the database before every suite run, and the 13 were re-graded (`-regrade`):
   12 became marked and 1 stayed silent. The CSV's `regraded_from` column names the
   original outcome.
2. **A panic hid a bound test.** SIG-007's mutation panics in an earlier handler test,
   which kills that package's test binary, so the bound test never ran, and the run graded
   it other. Now any bound test the suite didn't finish is run alone afterwards, and SIG-007
   was re-graded to marked. A check over every stored run confirms each bound test now has a
   pass or fail.

The classifier and the prompt are unchanged. Rules graded before fix 1 ran against the
right schema, because their mutations were in Go code.

**Limits.** This is one private fixture whose tests were written under cinch, by the same
author as its rules. One model proposed each mutation, and once only: a different mutation
of the same rule might slip past a test that caught this one, so 96% is an upper bound on
"the bound test catches the obvious violation", not on every violation. Frontend rules have
no bindings here at all, so the measurement doesn't cover them. A rule bound to several
tests (RT-001 has 7) counts once.

**Reproduce.** `make binding-strength` needs a `project_deltadocs` checkout and pi configured
with the model above. It also needs a Postgres container of its own, so it never touches the
developer's DB or whatever else holds port 5432:
`docker run -d --name cinch-binding-pg -p 55432:5432 -e POSTGRES_USER=<user> -e POSTGRES_PASSWORD=<pass> postgres:15-alpine`,
with the credentials from DeltaDocs' `backend/.env`. Use
`BENCH_ARGS='-rules SIG-019'` for one rule. Runs resume: a rule that already has a result in
the cache is skipped. `BENCH_ARGS='-regrade ID,ID'` re-runs the suite on stored mutations
without asking the model again.
