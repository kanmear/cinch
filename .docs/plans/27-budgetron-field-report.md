# 27. Budgetron field report, 2026-10-11

**Status: shipped, all six items, in 0.10.3–0.13.0. Kept for provenance.**

## Provenance

**Field data.** An agent using cinch 0.10.1 in `budgetron` (a Flutter repo) reported six
issues; four were observed directly, two (5, 6) partly inferred. Budgetron's `cinch.yml`
registers `make-check` (static checks ending in a plain `cinch check`), `make-test` and the
[19](19-field-log.md) field-log script as `hooks.pre-commit` scripts. cinch's own hook checks
already read the index ([14](14-pre-commit-checks-index.md)); every problem below came from
the project scripts, from plain `cinch check`, or from output.

## Items

| # | Report | Cause | Shipped |
|---|---|---|---|
| 1 | `rules error .:1: failed to scan: open lib/…dart` after an unstaged `rm` | Working-tree marker scan listed tracked files via `ls-files --cached` and opened a deleted one; the error was reported at the repo root | `fix/check-skips-deleted-files`: missing files and symlinks to directories are skipped, other scan errors are reported at the file. **CINCH-011** |
| 2 | Hook scripts judged the whole dirty tree while splitting commits | Scripts run in the working tree by design | `feature/hook-script-context`: a warning with file counts, and `CINCH_HOOK=<event>` in the script environment; a plain `cinch check` inside `pre-commit` checks the staged content. **CINCH-012** |
| 3 | `make-check` failed, then `make-test`'s output buried it | Scripts ran on after a failure (needed: the field log must see blocked commits) with nothing at the end | Same branch: every step still runs, and the hook ends with one row per step. **CINCH-013** |
| 4 | `rules A, B, C +5 more` hid IDs | `capList` capped both lists ([15](15-impact-one-line-per-doc.md)) | `feature/impact-full-list`: only the hook caps, and says when it did; `cinch impact` lists everything |
| 5 | No way to land a rule before its test | Workflow templates never mentioned the placeholder | `fix/workflow-rule-placeholder`: `dev-plan-feature` and `docs-maintain-domain` describe `cinch:ignore: test lands in <plan> T<n>` |
| 6 | `Merge branch 'dev'` rejected without a hint | `checkCommit` printed only the pattern | `feature/commit-merge-hint`: git's default merge subjects get a hint to pass `-m` |

## Verification

A scratch clone of budgetron, with `make check`/`make test` swapped for quick stand-ins, replayed
each item through real commits on the shipped build. A build from before item 1 reproduced the
reported `.:1` error on the same commit.

## Open

- The post-commit hook now prints its "all ok" summary on every commit. If that reads as noise,
  print the summary only when a step failed or was skipped.
- The `checks` row reads `ok` when `pre-commit-checks: []` turns the built-in checks off.
