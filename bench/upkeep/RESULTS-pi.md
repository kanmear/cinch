# Upkeep ablation, stage 1: pi with Qwen

The stage-1 result for the pi engine (`Qwen3.8-27B-UD-Q4_K_M`, thinking high, pi 1.4.0), N = 5
per task and arm, 90 runs, started 2026-10-28 on the pin `cinch-bench/upkeep-v1` (`2907c92`),
cinch 0.13.2, seed `20261028`. The design is in [DESIGN.md](DESIGN.md). Every number below
comes from the CSVs named here and from the output of the analysis tool:

```
go run ./bench/upkeep/analyze bench/upkeep/<csv>
go run ./bench/upkeep/analyze -transcripts ~/.cache/cinch-upkeep/runs/pi bench/upkeep/results-pi.csv
```

All 90 runs are graded and valid; there is one attempt per run and nothing was rerun. No run
bypassed a hook.

## Which grading is which

| Grading | File | Status |
|---|---|---|
| Keys of amendment 1 (`prereg/upkeep-amend1`) | `results-pi-amend1.csv` | Registered primary for pi. A copy of `results-pi.csv` as it stood before the amendment-2 regrade. |
| Keys and grader of amendment 2 | `results-pi.csv` | Sensitivity result for pi. Written after the per-arm results were seen. |
| Keys at `prereg/upkeep`, grader of amendment 2 | `results-pi-keys-prereg-upkeep.csv` | Sensitivity result (`-regrade -keys-ref prereg/upkeep`). |
| Keys at `prereg/upkeep-amend1`, grader of amendment 2 | `results-pi-keys-prereg-upkeep-amend1.csv` | Sensitivity table (`-regrade -keys-ref prereg/upkeep-amend1`). It differs from `results-pi-amend1.csv` in one run, `C1-arm3-r3`, which the amendment-2 retirement rule passes. |

Amendment 2 (DESIGN.md § Amendments) came after all 90 runs were graded and after the per-arm
results were seen, so for pi it cannot be the registered primary. Behavior is graded once and
is the same in every row. Only the corpus half of upkeep changes between rows.

## Primary: arm 3 against arm 2

Upkeep pass (behavior and corpus), stratified by task, one-sided (arm 3 above arm 2). The
registered statistic is the plain CMH; the corrected and exact p values are for comparison.

| Keys | Arm 3 | Arm 2 | Difference | CMH z | CMH p | Corrected p | Exact p | Look 1 (p < 0.005, >= 15 points) |
|---|---|---|---|---|---|---|---|---|
| Amendment 1 (registered) | 21/30 | 25/30 | -13.3 points | -1.414 | 0.9214 | 0.9615 | 0.9646 | not detected |
| Amendment 2 (sensitivity) | 25/30 | 26/30 | -3.3 points | -0.429 | 0.6659 | 0.8043 | 0.8095 | not detected |
| `prereg/upkeep` keys (sensitivity) | 20/30 | 22/30 | -6.7 points | -0.788 | 0.7846 | 0.8813 | 0.8856 | not detected |
| `prereg/upkeep-amend1` keys, amendment-2 grader | 22/30 | 25/30 | -10.0 points | -1.053 | 0.8539 | 0.9199 | 0.9237 | not detected |

The `prereg/upkeep` row uses the amendment-2 grader, so it passes `C1-arm3-r3`. With the
original grader as well, that row would be 19/30 against 22/30 for arms 3 and 2. The tool
was not run on that combination; the figure is the row's 20/30 less that one run.

Under every grading, arm 3 is below arm 2 in the pooled rate, so the question "does arm 3 beat
arm 2" gets a clear no at this N. The differences between gradings come from S1 (below).

## Per task

Upkeep passes out of 5 runs (behavior passes in brackets). Arm 1 is not graded on corpus.

| Task | Arm 1 behavior | Arm 2, amend 1 | Arm 3, amend 1 | Arm 2, amend 2 | Arm 3, amend 2 | Arm 2 behavior | Arm 3 behavior |
|---|---|---|---|---|---|---|---|
| S1 | 5/5 | 4/5 | 2/5 | 5/5 | 5/5 | 5/5 | 5/5 |
| S2 | 5/5 | 5/5 | 5/5 | 5/5 | 5/5 | 5/5 | 5/5 |
| S3 | 5/5 | 5/5 | 5/5 | 5/5 | 5/5 | 5/5 | 5/5 |
| C1 | 5/5 | 3/5 | 1/5 | 3/5 | 2/5 | 4/5 | 3/5 |
| C2 | 3/5 | 3/5 | 3/5 | 3/5 | 3/5 | 3/5 | 4/5 |
| C3 | 5/5 | 5/5 | 5/5 | 5/5 | 5/5 | 5/5 | 5/5 |
| All | 28/30 | 25/30 | 21/30 | 26/30 | 25/30 | 27/30 | 27/30 |

Amendment 2 changed five corpus grades, all fail to pass: `S1-arm2-r1`, `S1-arm3-r0`,
`S1-arm3-r3` and `S1-arm3-r4` under the new S1 key, and `C1-arm3-r3` under the retirement rule.
Each keeps its first grade in `corpus_first` and `upkeep_first`. No other run changed.

Ceiling tasks (100% upkeep in arms 2 and 3):

- Amendment 1: C3, S2, S3. Without them: arm 3 6/15, arm 2 10/15 (-26.7 points).
- Amendment 2: C3, S1, S2, S3. Without them: arm 3 5/10, arm 2 6/10 (-10.0 points).

The ceiling tasks have no spread, so removing them leaves the CMH z and p unchanged.

## Secondary measures

From `results-pi.csv`; none of these depends on the key set.

| Arm | Runs | Behavior | Committed | Bypass | Timed out | Hook blocks | Median turns | Median wall (min) | Median output tokens | Suite pass |
|---|---|---|---|---|---|---|---|---|---|---|
| 1 | 30 | 28/30 | 29/30 | 0 | 1 | 17 | 33 | 6.8 | 14,600 | 30/30 |
| 2 | 30 | 27/30 | 27/30 | 0 | 3 | 14 | 36 | 8.8 | 19,094 | 28/30 |
| 3 | 30 | 27/30 | 26/30 | 0 | 4 | 6 | 35 | 7.9 | 16,962 | 26/30 |

All eight timeouts are in C1 (arm 1: one, arm 2: three, arm 3: four). None of the eight
committed. A run that times out is graded on its working tree as it stands, so a timeout is
not itself a fail: `C1-arm2-r0`, `C1-arm2-r3` and `C1-arm3-r3` timed out and passed upkeep.

## How the agents used cinch and the docs

From each run's `transcript.jsonl` (`-transcripts`). 90 of 90 transcripts were found. A count
is a number of runs.

| Arm | Runs | Invoked any `cinch` | Read anything under `.docs` | Read a workflow file | Edited a doc | Timed out without committing |
|---|---|---|---|---|---|---|
| 1 | 30 | 0 | 10 (probes: arm 1 has no `.docs`) | 0 | 0 | 1 |
| 2 | 30 | 0 | 30 | 2 (no workflows exist in arm 2) | 27 | 3 |
| 3 | 30 | 19 | 30 | 11 | 23 | 4 |

Arm-3 runs by `cinch` subcommand the agent ran itself: `check` 11, `index` 10, `workflows` 5,
`help` 4 (`cinch --help`), `impact` 2, `workflow` 2, `rules` 1. A run can appear in several
rows. Hooks run `cinch` too; those calls are not in this count because the agent didn't type them.

Cross-check: runs with a `.docs` hunk in `final.patch` are 0, 27 and 23 for arms 1, 2 and 3,
the same as the "edited a doc" column. Eleven of the 30 arm-3 runs never ran `cinch`
themselves, so the tool was available in arm 3 and used in 19 runs.

## Reading

**Not detected at N = 5.** Arm 3 did not beat arm 2 under any of the four gradings: the pooled
difference is -13.3 points under the registered keys (21/30 against 25/30), and -3.3 points
under the amendment-2 keys (25/30 against 26/30). The one-sided CMH p is 0.92 and 0.67. The
power table in DESIGN.md puts the detectable difference at about 38 points for 30 against 30
runs at α = 0.005, so this design could not have seen a small positive effect, and the data
says nothing against one. It does not support the prediction of a win of 30 points or more
for pi. The registered result is "not detected at N = 5".

**C1's arm-3 deficit is bounded by the 30-minute cap, and is not a difference between the
arms.** Four of the five arm-3 C1 runs (`r0`, `r2`, `r3`, `r4`) timed out without committing.
Three of those four failed upkeep: `r2` and `r4` fail the hidden tests, and `r0` passes them but
leaves CAT-002 as it was. `r3` passed the amended corpus (retired in place). The fifth,
`r1`, committed at 1560 s and passed. Arm 2 hit the cap too, in three of five C1 runs, and two of
those passed. C1 is therefore a task this model often could not finish in 30 minutes in either
arm (8 of 15 C1 runs timed out; no run of any other task did), and the 2/5 against 3/5 gap
under amendment 2 is one run. All five arm-3 C1 transcripts name CAT-002 as a rule to deal
with before they edit it. `C1-arm3-r4`: "CAT-002 needs to be updated: category color is derived
from the name." `C1-arm3-r3`: "Update CAT-002 in `.docs/domain/categories.md` ... replace with a
derived-color rule." `C1-arm3-r1`: "CAT-002 should be updated to describe the derived color
rule." `C1-arm3-r0` plans to "either remove it or replace it". `C1-arm3-r2` is the weakest
case: it plans to reword CAT-002 ("a stored color string ... round-trip losslessly") and keep
it, not to retire it. In the four runs that ran out of time, the agent knew about the rule and
had not got to the edit; this is a statement about the cap, not about whether cinch's
impact advisory worked, since a timed-out run never reaches the commit where impact fires.

**Three tasks at ceiling (four under amendment 2).** S2, S3 and C3 each pass upkeep in 10 of 10
runs across arms 2 and 3, and arm 1 passes their behavior tests in 15 of 15. Under the amended S1
key S1 joins them. Those tasks cannot separate the arms. C2 is the only task without a cap
effect where upkeep is not at the ceiling (3/5 in both arms), and there arm 3 and arm 2 are
equal.

**S1 is a grading result, not a model result.** S1's arm-2 and arm-3 behavior is 5/5 in both
arms. The upkeep numbers for S1 move from 4/5 and 2/5 (amendment 1) to 5/5 and 5/5
(amendment 2) and 1/5 and 0/5 (`prereg/upkeep`) only because of how a correct rule rewrite
was matched. The S1 numbers do not carry information about the arms.

**cinch's own checks did not fail an agent's commit in these runs.** In the 30 arm-3 runs no
tool result contains "checks      failed" (cinch's own check step in the pre-commit hook) or a
"rules error" finding. The hook blocked a commit in five arm-3 runs, six times in all
(`hook_blocks`): four failures of `make check`'s formatter or analyzer (`S1-arm3-r0`, `r1`,
`r2`, `S3-arm3-r3`) and two rejections of the commit message by the `commit.pattern` check
(`S3-arm3-r0`, `S3-arm3-r3`). The pattern check is cinch's `commit-msg` hook in arm 3 and a
plain hook printing the same message in arms 1 and 2 (8 and 5 rejections in their transcripts),
so a bad subject costs the same in every arm. These runs hold no instance of cinch's checks
catching a stale or orphaned rule. (Earlier planning notes put it at "once in 30 arm-3 runs";
the transcripts give zero for the checks and two for the commit message.)

**Stage 2 for pi is stopped.** Plan 29 (§ Immediate decisions) stops Qwen stage 2 of 25: the
registered result for pi is "not detected at N = 5". Stopping without a win doesn't inflate
α (DESIGN.md § Analysis). For Claude, stage 1 runs on the registered design under the
amendment-2 keys, and those are its keys of record.

**Limits.** These are 90 runs of one local model, one fixture and six tasks, with five runs per
task and arm and a 30-minute cap that bound one task. The result is "no detected effect at
N = 5 on this engine". It is not evidence that cinch has no effect, and it says nothing about
Claude.
