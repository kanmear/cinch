# Field log and rule-growth replay

Claim #7, "it doesn't cry wolf", measured on three consumer repos while their owner worked on
them for 3.7 weeks (2026-10-06 to 2026-11-01). Plans [19](../../.docs/plans/19-field-log.md)
(the hook log and its labeling) and [20](../../.docs/plans/20-rule-growth-replay.md) (the rule-growth
walk) are the method; [29](../../.docs/plans/29-benchmark-program-v2.md) item G publishes them.

**Claim #7, one line.** It doesn't cry wolf, and that is as far as the data goes: in 256 commits
cinch's checks blocked 5 attempts with 0 false positives and landed 0 red commits, but 3 of the
5 blocks were a stale generated workflow (Noise), only 2 caught something real (an incomplete
commit, fixed in under two minutes), the replay found 2 in-window authorization bugs in
project_deltadocs that no check saw until a fix wrote the missing rule, and one repo
(budgetron) never produced a single finding, so its silence can't tell a clean repo from a
check that can't see.

**Private repos.** `project_deltadocs` and `lucky_to_have_you` are private and can't be
reproduced from outside. The tracked [hook-log.csv](hook-log.csv) and
[findings.csv](findings.csv) hold the `budgetron` rows only; the full classification of all
three logs (timestamps, branch, short head, actor, exit, ms, class and cinch's finding text, which
names private paths and rule IDs), the DeltaDocs rule-growth CSV (merge subjects are branch
names) and the unabridged field log sit in `private/`, gitignored, with a sha256 of each file in
`../private-manifest.txt`. The counts below were computed over all three logs. `budgetron` is
the public fixture and can be replayed.

## Files

| File | What |
|---|---|
| [main.go](main.go), [hooklog.go](hooklog.go), [rulegrowth.go](rulegrowth.go), [main_test.go](main_test.go) | the tool: `hooklog` and `rulegrowth` |
| [hook-log.csv](hook-log.csv) | one row per budgetron log row, with class, bypass candidate and findings; all repos in `private/hook-log-all.csv` |
| [findings.csv](findings.csv) | every distinct budgetron finding (none); all repos in `private/findings-all.csv` |
| [field-log.md](field-log.md) | plan 19's labels, the bypass and exit-2 investigation, the decision rules; unabridged in `private/field-log-full.md` |
| [rule-growth-budgetron.csv](rule-growth-budgetron.csv) | plan 20's replay, one row per `merge: <branch>` commit; DeltaDocs' in `private/rule-growth-project_deltadocs.csv` |

## Hook log

**Method.** Each repo's `scripts/cinch-log.sh` appended one line per hook run to
`.git/cinch-log.tsv` (time, event, branch, head, actor, exit, ms, findings), running the hook's
default checks (`links,rules,index,generated,core`). `hooklog` classifies every row:

- **pre-commit:** *passed* (exit 0, a post-commit follows on that branch), *blocked* (non-zero,
  no commit follows), *rejected-later* (exit 0, no commit follows: another hook step or an empty
  commit refused it, not cinch) or *failed-landed* (non-zero and the commit landed anyway; none).
- **post-commit:** *clean*, *red-landed* (non-zero), or *superseded* when a later row has the
  same head. A `version-bump` amend fires post-commit twice, so the last row per head is the
  verdict (plan 19's caveat). A bypass candidate is a landed commit with no pre-commit row
  before it on its branch.
- **ms** is the logger's own `cinch check --staged` run, not the whole hook, and leaves out
  superseded rows and project_deltadocs rows before 2026-10-07 (a tracked 12 MB binary added
  about 800 ms until cinch 0.9.1).

**Result.** Snapshot of the logs at 2026-11-01 02:07 +03:00 (the lucky_to_have_you log is live
and has grown since):

| Repo | Rows | Commits | Blocks | Rejected later | Red landed | Commits by human / claude / pi | Pre-commit ms (median, p90) | Post-commit ms (median, p90) |
|---|---|---|---|---|---|---|---|---|
| budgetron (public) | 197 | 89 | 0 | 14 | 0 | 26 / 63 / 0 | 30, 56 | 67, 90 |
| project_deltadocs (private) | 104 | 47 | 1 | 1 | 0 | 4 / 43 / 0 | 28, 44 | 105, 140 |
| lucky_to_have_you (private) | 269 | 120 | 4 | 2 | 0 | 36 / 84 / 0 | 30, 47 | 60, 77 |

| Repo | Superseded post-commit rows | of which exit non-zero | Failed pre-commit, commit landed | Landed with no pre-commit row |
|---|---|---|---|---|
| budgetron | 7 | 0 | 0 | 2 |
| project_deltadocs | 8 | 8 | 0 | 0 |
| lucky_to_have_you | 23 | 0 | 0 | 0 |

- **Nothing red landed.** DeltaDocs' eight post-commit rows with exit 2 are not red content.
  They are the logger failing inside the version-bump amend, where a prepended `PATH` makes
  it run a stale 0.6.1 `cinch` that doesn't know `--staged`. Reproduced on a scratch clone;
  [field-log.md](field-log.md) has the evidence.
- **Five blocks, six TP findings, five Noise findings, no FP.** The labels, with what happened
  next in each repo's history, are in [field-log.md](field-log.md). The blocks that were Noise
  were all `generated`: cinch's own workflow text changed and the consumers' floor pins let
  their renders go stale.
- **Budgetron never blocked.** 0 findings in 197 rows. 14 pre-commit rows exited 0 and still
  produced no commit; the project's `make check` or merge-time tests refused them, not cinch.
  The two landed commits with no pre-commit row are consistent with `--no-verify`
  (cinch's own verdict on both is clean).
- **No pi data.** `pi` appears in no row, so this says nothing about a local model's commits.
  Rows before about 21:50 on 2026-10-06 are all `human` even where an agent committed, because
  actor tagging started then.

**Limits.** Three repos, one owner, 3.7 weeks, and the labeler is the author. The two TPs
were each fixed within two minutes by the same agent session, so what the log shows is "cinch
stopped an incomplete commit", not "cinch prevented a bug". A hook log records what cinch said,
not what it didn't: a Miss is invisible to it. The checks it logged are the pre-commit baseline,
so the commit-msg and hooks checks aren't covered. `rejected-later` rows can't be told apart
from empty commits without the other hooks' output, which isn't logged.

## Rule growth

**Method.** For each `merge: <name>` commit reachable from the repo's `dev` (squash merges onto
a local `dev` can sit behind a `merge: dev` sync commit, so the walk is not first-parent
only; `merge: dev` itself is skipped), check the commit and its first parent out in a detached
worktree and run `cinch rules --json` (cinch 0.13.2). A row records the rule count, rules with
a marker, rules with `cinch:ignore`, whether the merge touched product code, the branch type,
and the rules added, changed (same ID, different text after stripping list numbering) and
removed. The diff is against the first parent, which is what the merge did; the CSV also keeps
the diff against the previous row (`prev_row_*`), and the two agree in every row but one:
budgetron's `0fe3070`, where a direct fix commit between the two merges edited a rule. Product code is `lib/` in budgetron, and `backend/` or `frontend/` outside any
`.agent/` in project_deltadocs. Historical commits with no rule corpus, whose `cinch rules` fails
(`lstat .docs: no such file or directory`), keep a row with the error recorded and are left out
of the headline: DeltaDocs has 41 such rows, 40 merges before the corpus and `feature/cinch`
(2026-09-01), which created it.

**Headline** (plan 20): the share of `feature/` and `fix/` merges that touched product code
and added or changed a rule. The window was fixed before the numbers were looked at: the
*rule era*, merges whose first parent already had rules. The field-log window (from 2026-10-06)
is shown beside it.

| Repo | Merges | Feature/fix touching product code | Added or changed a rule | Share | Rules added / changed |
|---|---|---|---|---|---|
| budgetron, rule era | 10 | 5 | 3 | 60% | 6 / 6 |
| project_deltadocs, rule era | 23 | 14 | 5 | 36% | 17 / 7 |
| project_deltadocs, field-log window | 11 | 6 | 4 | 67% | 13 / 7 |

- **Rule growth is not close to zero, so plan 20's decision rule is not triggered.** Cut the
  dev workflows to the missing-rule step and the entry gate only if the share stays near zero
  over the push. It sits at 36% to 67%. That answers plan 07's open question in the direction
  of keeping the missing-rule step: it produced rules on the merges where it should have.
- **The share is lumpy.** In project_deltadocs the corpus stayed at 79 to 80 rules from 09-02
  to 10-19 across 9 feature/fix merges that touched product code (the one rule added was in a
  chore merge), then grew by 13 rules in the four merges of 10-21 to 10-22. Three large merges
  (5,755, 3,323 and 559 changed product lines) changed no rule.
- **Fix merges that added rules are the Misses.** Three DeltaDocs fix merges (09-02, 10-21,
  10-22) each needed rules the corpus didn't have; see [field-log.md](field-log.md).
- **Budgetron's count is a floor.** Its 10 squash merges are all the walk sees. Its largest
  feature (SMS import, ten commits, the first of which added the SMS rules) and the two
  `fix [..]` commits after the last merge landed outside the `merge:` convention and are not
  rows.

**Limits.** "Touched product code" is a path test, not a judgement that the change needed a
rule: a 66-line sidebar fix with no rule change counts against the share. A merge that adds a
rule in a separate later merge counts as a zero here. Two repos, one owner, and the squash
convention starts 2026-10-09 in budgetron. Early DeltaDocs rows before 2026-09-01 are
out because the corpus doesn't exist, not because the replay failed.

## Reproduce

The hook logs are in `.git/` and unversioned, so `hooklog` only reproduces on this machine:

```
go run ./bench/field hooklog ~/code/budgetron/.git/cinch-log.tsv \
  ~/code/project_deltadocs/.git/cinch-log.tsv ~/code/lucky_to_have_you/.git/cinch-log.tsv
```

`rulegrowth` adds and removes a worktree in the repo it is given. The published numbers came
from `git clone --no-local` copies so the originals were never touched:

```
go run ./bench/field rulegrowth -tmp /tmp ~/clones/budgetron dev
go run ./bench/field rulegrowth -tmp /tmp ~/clones/project_deltadocs dev
```

Budgetron can be replayed from its public repo. `go test ./bench/field` covers the
classifier, the delta, the headline and a walk over a throwaway git repo with a stub rule reader.
