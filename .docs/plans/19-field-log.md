# 19. Field log during the DeltaDocs MVP push

**Tier 4 — evidence. Cost estimate: one script per consumer repo now, about 10 minutes a week
of labeling, and one afternoon of analysis at the end.**

## Provenance

**Field data** (prospective). Every earlier item was justified by an audit or a benchmark,
and both of those predict behavior. This item records cinch's behavior on real commits while
`project_deltadocs` (and the odin repos) push to MVP. It's the first source in this build order
that observes what happens instead of predicting it.

## Why git history alone isn't enough

A commit cinch blocked never lands. A `--no-verify` bypass looks like any other commit. Those
are exactly the cases that most need counting, so something has to record them as they
happen. This item has to be installed **now**, before the push, which is why it's numbered
ahead of the benchmarks in [21](21-benchmark-suite.md) even though it's analyzed last.

## What it answers

| Question | Source | Effort |
|---|---|---|
| Does cinch catch real problems? | Hook log, then weekly labeling | Automatic, plus ~10 min/week |
| Does it block wrongly, or get bypassed? | Hook log | Automatic |
| Do rules get written as you go (is the missing-rule step working)? | [20](20-rule-growth-replay.md), at the end | One script, once |
| Does a passing check mean the rules are actually tested? | [04](04-measure-binding-strength.md) | One local run, overnight |

## 1. The hook log (no cinch changes needed)

Put one script in each consumer repo and register it for two hook events. `cmdHookPreCommit`
(`internal/cinch/hook.go:27`) dispatches project scripts even when its own checks failed, so
blocked attempts get logged too.

```sh
#!/bin/sh
# scripts/cinch-log.sh <pre-commit|post-commit> — one line per run, in .git/ (never committed)
event=$1
gitdir=$(git rev-parse --git-common-dir)
log="$gitdir/cinch-log.tsv"
# The hook's default baseline (no hooks.pre-commit-checks override here), so a
# logged pass or fail is the same verdict the hook gave.
checks=links,rules,index,generated,core

start=$(date +%s%3N)                     # GNU date
if [ "$event" = post-commit ]; then
  # Check exactly what landed: a throwaway index built from HEAD.
  idx="$gitdir/cinch-head-index"; rm -f "$idx"
  GIT_INDEX_FILE="$idx" git read-tree HEAD
  out=$(GIT_INDEX_FILE="$idx" cinch check --staged --only $checks 2>&1); code=$?
  rm -f "$idx"; head=$(git rev-parse --short HEAD)
else
  out=$(cinch check --staged --only $checks 2>&1); code=$?; head=-
fi
ms=$(( $(date +%s%3N) - start ))

findings=$(printf '%s\n' "$out" | grep -E '^[a-z]+ (error|block) ' | tr '\n' '|')
printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$(date -Iseconds)" "$event" \
  "$(git branch --show-current)" "$head" "${CINCH_ACTOR:-human}" "$code" "$ms" "$findings" >> "$log"
exit 0
```

```yaml
# cinch.yml
hooks:
  pre-commit:
    field-log:
      run: scripts/cinch-log.sh pre-commit
  post-commit:
    field-log:
      run: scripts/cinch-log.sh post-commit
```

Two dependencies to know about:

- The post-commit branch relies on `GIT_INDEX_FILE` being honored by `cinch check --staged`.
  That's item [14](14-pre-commit-checks-index.md)'s snapshot design, covered by
  `internal/cinch/staged_test.go`.
- The `grep` matches the finding format printed at `internal/cinch/check.go:326`
  (`<check> <level> <file>:<line>: <msg>`). That only works with colour off. It is: colour is
  decided per stream by `isColorTerminal` (`internal/output/output.go:35`), so stdout captured
  by `$(...)` is never painted. Confirmed from the code on 2026-10-06, so no field check is
  needed. If that detection ever changes, set `NO_COLOR=1` in the script.

Where to install it: `~/code/project_deltadocs` (its `cinch.yml` already has
`hooks.pre-commit` entries, so add `field-log` alongside them), and each odin repo that
consumes cinch (only `~/code/odin-landing` is checked out locally).

### How to read it

- A failed pre-commit with no post-commit after it means cinch blocked a commit. Each of these
  needs a label.
- A post-commit with a non-zero result means red content landed. It was either bypassed with
  `--no-verify`, or it slipped through. This works because `--no-verify` skips pre-commit and
  commit-msg but not post-commit.
- The `ms` column tells you what cinch costs per commit. The pre-commit run re-checks what
  cinch just checked, which doubles a step that takes tens of milliseconds, so it isn't worth
  worrying about.

### Telling agents from humans

Set `CINCH_ACTOR` when launching each agent, for example `alias pi='CINCH_ACTOR=pi pi'`, and the
same for Claude Code. The log then separates human commits from agent commits without
guessing from environment variables.

## Installed (2026-10-06)

| Repo | Commit | Before installing |
|---|---|---|
| `~/code/project_deltadocs` | `e9df406 merge: chore/cinch-field-log` on `dev` | `cinch upgrade`, pin 0.6.1 → 0.9.0 plus re-render (`45e1275`) |
| `~/code/lucky_to_have_you` | `27bc718 merge: chore/cinch-field-log` on `dev` | none, already pinned to 0.9.0 |
| `~/code/budgetron` | swept into an agent's `8934fd8 test [rules]: …` on `main` (see below) | none |
| `~/work/fsed-odin-docs` | `70b4d55` on `main` | `cinch upgrade`, pin 0.8.3 → 0.9.0 (`0b0ebe5`) |

Nothing is pushed. `~/work/fsed-odin-mono` is left out: its `core.hooksPath` is unset, so
cinch doesn't run there at all, and turning hooks on would change enforcement, not just
observe it. Before the upgrades, DeltaDocs and odin-docs failed `core` on every commit
(DeltaDocs also failed `generated`), so every commit there was blocked or bypassed. That
was the "version pin" Noise this item predicted, cleared before logging started rather
than logged.

Changes from the design above, and why:

- **`core` is in the logged checks.** It's in the hook's default baseline, and none of the four
  repos override `hooks.pre-commit-checks`. Without it, a commit blocked by the pin would be
  logged as a pass.
- **Actor tagging** is a fish `conf.d` file, `~/.config/fish/conf.d/cinch-actor.fish`, that wraps
  `pi` and `claude` and sets `CINCH_ACTOR`. Rows before about 21:50 on 2026-10-06 are all
  `human`, including eight budgetron commits that were made by an agent.

Lessons from the install:

- **Don't stage into a checkout an agent is using.** In budgetron, an agent session's
  `git commit` picked up the staged `cinch.yml` and script into its own commit. Use a
  worktree.
- **Worktrees don't work for repos that use `rules.roots` with nested repos.**
  `fsed-odin-docs` keeps its markers in nested `backend`/`frontend`/`mobile` repos that a
  fresh worktree doesn't contain, so every rule shows as unmarked there. Work in the main
  checkout.

## Reading caveats found on day one

- **Duplicate post-commit rows.** In DeltaDocs and lucky_to_have_you, the `version-bump`
  script amends the commit with `--no-verify`, which fires post-commit again. A bumped
  commit therefore logs two post-commit rows with the same `head`. Dedupe by `head`.
- **A pre-commit row with exit 0 and no post-commit after it is not a cinch block.** Git
  runs pre-commit before it checks for an empty commit, and a project hook script or
  commit-msg can still reject the commit afterwards. Only a non-zero pre-commit exit counts
  as cinch blocking. budgetron's row at 2026-10-06T21:46:02 is an example: an empty commit
  attempt.
- **DeltaDocs' early `ms` values are inflated.** It took about 0.9–1.0 s per run, while the
  other three took 40–100 ms. The cause was a tracked 12 MB binary (`backend/bin/server`)
  that the marker scan read line by line. Cinch 0.9.1 skips binary files (CINCH-009) and
  brought DeltaDocs to about 100 ms. Rows before 2026-10-07 carry the extra ~800 ms, so
  exclude them from any cost figure.

## Pins (2026-10-07)

All four repos pin `require.cinch: '>=0.9.0'`, a floor. Installing 0.9.1 showed the
problem with exact pins: `0.9.0` in DeltaDocs `dev` and odin-docs (and `0.9.1` on DeltaDocs'
feature branch) made `core` block every commit until someone re-pinned. lucky_to_have_you
was moved to a floor outside this session. DeltaDocs: `cde4bd3` on `dev`, merged into
`feature/tab-editor-live-editing` as `293e3a0`. odin-docs: `ae7ef80`.

## 2. Weekly labeling (~10 minutes)

Every distinct finding in the log gets one label. Keep the labels in
`~/code/cinch_bench/field-log.md`, next to the benchmark, and **not** in a consumer's docs: a
numbered line there with a bold rule ID would be parsed as a rule.

- **TP**: a real problem, and you fixed it.
- **FP**: cinch was wrong, or the block wasn't worth having.
- **Noise**: technically correct, but not worth stopping for. A stale render or the version
  pin are typical.
- **Miss**: a bug found later that a rule should have caught. This is the realtime-leak kind,
  and it's the most important label. Record it whenever a bug fix turns out to need a new
  rule.

Also record the bypasses from the log, and note why each one happened.

## 3. Decision rules (pre-registered)

These are written down before any data exists, following item
[09](09-calibrate-heuristics-against-history.md)'s discipline. This file's commit timestamp is
the evidence that they came first.

- **Wrong blocks.** If any check produces more than one FP or bypass a week, fix that check
  before anything else.
- **Binding strength** ([04](04-measure-binding-strength.md)). Above about 90% biting, close
  the question. Below about 70%, build a mutation-check step into the coverage audit.
- **Rule growth** ([20](20-rule-growth-replay.md)). If close to zero merges add rules over the
  MVP push, cut the dev workflows down to the missing-rule step and the entry gate.
- **Overall value.** If the run ends with zero TPs and some FPs, shrink cinch to what
  demonstrably held up (`links`, `generated`, the rule inventory) and say so in the README.

## Caveat

This is a few weeks of one person's work, so most of the result will be qualitative. The Miss
and bypass entries will probably teach more than any rate. That's still better than another
review, which can only predict this behavior.

## Verification / definition of done

- The script and `cinch.yml` entries are committed in each consumer, and the first week's
  log has non-empty rows from both events.
- `~/code/cinch_bench/field-log.md` has a labeled entry for every distinct finding and every
  bypass at the end of the MVP push.
- A results note applies each decision rule above to the data and states the outcome. It feeds
  claim #7 in [21](21-benchmark-suite.md).

**State (2026-10-06):** `~/code/cinch_bench/` is not on this machine yet; it will be available
later. The hook log doesn't need it, so install the script now. The labels file is created there
once the directory is back, and the log in `.git/cinch-log.tsv` keeps everything until then.
