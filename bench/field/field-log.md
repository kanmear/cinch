# Field log: labels

Plan [19](../../.docs/plans/19-field-log.md)'s weekly labeling, done once over the whole log.
Source: the three hook logs (`.git/cinch-log.tsv`), snapshotted 2026-11-01 02:07 +03:00
(budgetron 197 rows, project_deltadocs 104, lucky_to_have_you 269), classified by
`go run ./bench/field hooklog`. Budgetron's rows are in [hook-log.csv](hook-log.csv); the rows
of all three repos, the distinct findings and the unabridged version of this file (branch
names, heads, file paths, rule IDs and what each fix was) are in `private/`, gitignored, because
`project_deltadocs` and `lucky_to_have_you` are private. This file keeps the labels and the
counts. Labels are TP (real problem, fixed), FP (cinch wrong, or the block wasn't worth having),
Noise (correct but not worth stopping for) and Miss (a bug fix that needed a new rule). This
file uses tables and bullets, not numbered lines with bold IDs, so nothing here parses as a
rule.

**The log in one paragraph.** 256 commits landed with the hook installed (budgetron 89,
project_deltadocs 47, lucky_to_have_you 120). Cinch's checks blocked 5 attempts, all in the
two private repos. Nothing red landed: 0 post-commit rows with a real non-zero verdict. Of the
5 blocks, 2 caught something real and 3 were a stale generated workflow. Budgetron never
blocked: 0 findings in 197 rows.

## The five blocks

Every non-zero pre-commit row with no commit after it on that branch. Time-to-clear is from the
blocked row to the next landed commit that contained the fix.

| When (+03:00) | Repo | Actor | Findings | Next in history | Label |
|---|---|---|---|---|---|
| 10-12 16:33:37 | lucky | claude | `generated` x2: two workflows stale | 10 s later a `chore [harness]` re-render commit; the feature commits followed | Noise |
| 10-12 16:35:30 | lucky | claude | `generated` x2 (same files), `rules` x14: tests carry markers for two rule IDs no rule defines | 106 s later the re-render commit, then at 16:37:31 one commit landed the code, the tests and the two new rules together | TP for `rules`, Noise for `generated` |
| 10-18 22:08:41 | lucky | claude | `rules`: a new rule has no marker and no `cinch:ignore` | 60 s later a commit landed the rule with tests that carry its marker; four more commits on the branch | TP |
| 10-19 00:55:03 | project_deltadocs | claude | `generated` x2 (the same two workflows) | 111 s later a `chore [harness]` re-render on a separate branch; the blocked work landed 290 s after the block | Noise |
| 11-01 01:55:27 | lucky | claude | `generated`: `docs-audit-coverage.md` stale | 14 s later a `chore [harness]` re-render commit | Noise |

Why the three `generated` blocks are Noise rather than TP. Each time the owner's installed
cinch had moved ahead of the version that last rendered the workflows, because every consumer
pins a floor (`>=0.9.0`), not an exact version. The re-render commits show the template text
changing: the `cinch:ignore: test lands in <plan> T<n>` paragraph in `dev-plan-feature` and
`docs-maintain-domain` (cinch `fix/workflow-rule-placeholder`, 10-11) and the "rule item up to
the next heading" sentence in `docs-audit-coverage` (cinch `fix/rule-item-boundary`, 10-27).
The check is right that the file differs from a fresh render, and the fix is one command and
10 to 111 seconds. But nothing was wrong with the repo: the cost is one extra commit each time a
cinch release touches a bundled workflow, in every consumer that commits next. That is plan
19's own example of Noise ("a stale render").

Why the rules blocks are TP. In the second block the agent staged tests whose markers cited
two rules that were not yet in a rule doc in the staged tree. The commit that landed a minute
later had the rule text and the tests together. In the third block the agent staged a new rule
without a test or an ignore line; the next commit bound it to tests. Both are the
dangling-reference state the `rules` check exists for, fixed before landing. The caveat is
size: both were fixed in the same agent session within two minutes, and either would have
shown up at the next `cinch check`. The log shows cinch stopped the incomplete commit, not that
a bug was prevented.

## Distinct findings

Same check, file and message is one finding (line numbers move). Eleven distinct findings, all
in the private repos; the per-file list is in `private/findings-all.csv`.

| Repo | Check | Message | Findings | Rows | Label |
|---|---|---|---|---|---|
| project_deltadocs | generated | stale render (two bundled workflows) | 2 | 2 | Noise |
| lucky_to_have_you | generated | stale render (three bundled workflows) | 3 | 5 | Noise |
| lucky_to_have_you | rules | a `cinch:rule` marker resolves to no rule (five test files, two IDs) | 5 | 5 | TP |
| lucky_to_have_you | rules | a rule has no marker or ignore | 1 | 1 | TP |

Totals: 6 TP (from 2 blocking events), 5 Noise (from 4 events: the three Noise-only blocks plus
the `generated` half of the second block), 0 FP. The five marker findings are one event with 14
marker lines.

## Every red-landed, bypass and non-landing row

- **Red-landed: none.** No post-commit row in any repo carries a real non-zero verdict. The only
  non-zero post-commit rows are the eight below, and they are not content.
- **project_deltadocs, eight post-commit rows with exit 2 (10-09 to 10-22): a tooling
  artifact, label Noise, not red content.** Findings column empty, `ms` 11 to 15 (a real check
  takes 100 ms or more), and every one is the first of two rows with the same head, the second
  being exit 0. The eight heads are exactly the eight `merge: <branch>` squash merges onto
  `dev` that touch `frontend/` or `backend/`, which are the ones the `version-bump` script
  amends. The amend re-fires post-commit, and the nested run is the one that failed.
  The cause, reproduced on a scratch clone (`git clone --no-local` into the session scratchpad;
  the real repo was not touched): `scripts/post-commit-version-bump.sh` exports
  `PATH="$HOME/.bun/bin:$HOME/.local/bin:$HOME/go/bin:...:$PATH"` before it amends. The
  nested hook, and the `cinch-log.sh` it runs, therefore resolve `cinch` to
  `~/.local/bin/cinch`, which is 0.6.1 (installed 2026-09-15; the mise shim on the normal PATH
  is 0.13.2). 0.6.1 predates `--staged`, so the logger's `cinch check --staged --only ...`
  prints `cinch: check: cannot read message file: --staged` and exits 2 (the usage-error code).
  Running `~/.local/bin/cinch check --staged --only links` directly gives the same line.
  Reproduction: replaying the last of the eight squashes on a `dev` branch of the clone logged
  `post-commit dev <head> exit 2, 11 ms` (the nested run, with that message in the captured
  output) and then `exit 1` from the outer run (the clone's pins differ, so `generated`
  was stale there; the outer row is the real verdict). The other two repos are unaffected:
  lucky_to_have_you's bump script puts the mise shims first on its `PATH`, and its 23
  superseded rows are all exit 0. The tooling lesson, not a cinch bug: a hook script that
  prepends to `PATH` changes which `cinch` the nested hooks run, so a stale binary in
  `~/.local/bin` produces a wrong-verdict row. The classifier drops the earlier row of a
  duplicated head (a "superseded" row), so these eight never count as red.
- **Bypass candidates: two, both budgetron, both label "bypass, cinch checks clean".** A bypass
  candidate is a landed commit whose post-commit row has no pre-commit row before it on its
  branch. `58b9c6f` and `608ed0b` on `feature/sms-import-improvements` (10-11 14:39:27 and
  14:41:54, claude) landed with no pre-commit row, while every normal commit there shows the
  project's `make check` (about 19 s) between commit start and the pre-commit row. That is
  consistent with `--no-verify`, but the reason is not recorded. Cinch's post-hoc verdict on
  both is exit 0, so no cinch check was bypassed into red. The skipped part was the project's
  own `make check`. No other repo has a candidate.
- **Rejected after cinch passed: 17 pre-commit rows (budgetron 14, project_deltadocs 1,
  lucky_to_have_you 2).** Exit 0, then no commit landed on that branch from that attempt. Not
  cinch blocks (plan 19's caveat): another step in the hook refused it (budgetron's
  `make check` and merge-time tests, an empty commit, a commit-msg rejection). The log doesn't
  record which. In 12 of 17 a commit landed on the same branch within 79 s, so they were
  retries, not abandoned work; two more landed after 81 s and 18 min; the other three
  (budgetron `main`, 10-08 and 10-11 twice) had no later commit on that branch in the log.
- **Commits with no log row, checked against `git log --all`.** Budgetron's 9
  `feat/sms-import` commits of 10-07 18:30 to 19:06 are rewritten copies (a `filter-branch`
  stripped co-author trailers, backup branch `backup/pre-coauthor-strip`), and the originals
  have rows. Two project_deltadocs commits of 10-07 01:30 were made on a feature branch before
  the merge that brought the logger into that branch. All other unlogged commits are
  two-parent `merge: dev` / `Merge branch` commits, which don't fire post-commit. No
  unexplained gap.
- **Failed pre-commit with the commit landing anyway: none** in any repo.

## Misses

The hook log cannot see a Miss; the rule-growth replay can (a fix merge that added or changed a
rule means the corpus lacked a rule the bug needed). The DeltaDocs replay is in
`private/rule-growth-project_deltadocs.csv`.

| Fix merge | Date | Rules it needed | Bug | Label |
|---|---|---|---|---|
| first | 09-02 (before the log) | 3 new realtime rules | events broadcast to an audience wider than the rule later drew | Miss (pre-window, the plan's own example) |
| second | 10-21 | 3 new rules in three docs | an ID from another project was acted on through the URL's project | Miss |
| third | 10-22 | 1 new rule, 1 reworded | an operation was allowed to a caller who didn't hold the authority the rule later required | Miss |

Two Misses inside the log window, both in project_deltadocs and both authorization-shaped, so
the corpus had no rule for a boundary that mattered. Neither was visible to any check: cinch
passed every commit that carried the bug. Budgetron shows none (its `fix/` merges changed no
rules; two direct `fix [..]` commits on `dev` after the last merge, one of which edited a rule's
text, sit outside the walk).

## Plan 19's decision rules

- **Wrong blocks (more than one FP or bypass a week for any check: fix that check first).**
  Not triggered. FP: 0 in 5 blocks over 3.7 weeks. Bypasses: the two budgetron candidates
  are both in the week of 10-05 to 10-11, which is more than one, but they bypassed the whole
  hook rather than being a wrong block by any check, and the cinch verdict on both is clean,
  so there is no check to fix. If bypasses count against a check regardless, the rule fires
  once, for the project's `make check`, not for cinch. The one thing worth fixing before
  anything else is outside cinch's checks: the stale `~/.local/bin/cinch` that mislabeled eight
  rows, and the 3 Noise blocks that a floor pin plus a template change will keep producing.
- **Binding strength (above about 90% biting: close; below about 70%: build a mutation-check
  step).** Closed. Plan 04 measured 72 of 75 bindings biting (96%) on project_deltadocs; see
  `bench/README.md`.
- **Rule growth (close to zero merges adding rules: cut the dev workflows to the missing-rule
  step and the entry gate).** Not triggered. Share of `feature/` and `fix/` merges that touched
  product code and added or changed a rule: project_deltadocs 5 of 14 (36%) over the rule era,
  4 of 6 (67%) inside the field-log window; budgetron 3 of 5 (60%). See the README.
- **Overall value (zero TPs and some FPs: shrink cinch to `links`, `generated` and the rule
  inventory).** Not triggered. 6 TP findings from 2 events, 0 FP. The caveat above applies: the
  TPs are both "incomplete commit stopped before landing", and the labeler is also the author.
