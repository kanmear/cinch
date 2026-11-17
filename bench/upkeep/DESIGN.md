# Upkeep ablation: design (draft for pre-registration)

Plan [25](../../.docs/plans/25-upkeep-ablation.md), claim #3: *cinch keeps docs true as code
changes.* This file becomes the pre-registration once the open decisions at the end are
settled. It gets committed together with the task specs, hidden tests, grader and prediction,
and tagged `prereg/upkeep`, before any model call.

## Question

When an agent changes code, does cinch's tooling make it keep the rule corpus true, beyond
what the docs and the `AGENTS.md` pointer already do on their own? The earlier upkeep result
(6 of 6 against 0 of 6, `~/code/cinch_bench/FINDINGS.md` §2) compared docs plus cinch against
nothing. This one compares them against docs without cinch.

## Fixture

Budgetron (`github.com/kanmear/budgetron`, Flutter/Dart, ObjectBox, local-only), pinned to
tag `cinch-bench/upkeep-v1` at `2907c92`. That commit comes after a history rewrite that
replaced real bank SMS details in test fixtures with made-up ones, before publishing. At that tag it has 39 rules in 8 domain docs, every rule bound
to a test, and 874 tests that run in about 11 s.

Budgetron needed one fix of its own before tagging. At `6ce4e4e`, `make check` was red
against cinch 0.13.2 because `.docs/workflows/docs-audit-coverage.md` predated the
`fix/rule-item-boundary` template. A `cinch render` commit fixed it, and the tag sits on that
commit. A fresh clone also needs `make gen` (`objectbox.g.dart` and the l10n files are
gitignored) and `make setup-test`. The harness runs both before the agent starts.

DeltaDocs runs the same arms as a private data point, with its own tasks. That set is drafted
after this one and pre-registered on its own.

## Arms

All three arms see the same prompt, the same pinned commit, the same toolchain, and the same
non-cinch gates. A plain git `pre-commit` hook runs Budgetron's `make check` minus `cinch
check`, so format, l10n, analyze and the hardcoded-strings check fire in every arm. The only
thing that varies is the cinch layer.

| | Arm 1: no docs | Arm 2: docs + pointer | Arm 3: docs + cinch |
|---|---|---|---|
| `.docs/` domain, architecture, conventions, testing | removed | present | present |
| `.docs/workflows/` (rendered by cinch) | removed | removed | present |
| `AGENTS.md` | Project section only | docs section without cinch commands, plus Project section | as-is |
| `cinch.yml`, `.githooks/`, `core.hooksPath` | removed | removed | present |
| `cinch` on `PATH` | no | no | yes |
| `cinch:rule` comments in tests | kept | kept | kept |

- In arm 2, the docs section of `AGENTS.md` keeps every instruction except the tool:
  "Business rules are numbered items with a bold ID. The test that enforces a rule carries a
  `cinch:rule <ID>` comment. Changing a behavior a rule describes means updating the rule and
  its test together. A new invariant a reader couldn't recover from the code gets a new
  rule." What arm 2 loses is `cinch index`, `cinch workflows` and the `cinch check` gate.
- Arm 1 is a repo that lost its docs, not one that never had them. Test markers and code
  comments that mention rules stay in place. That biases the comparison toward arm 1, so any
  measured gap is conservative (as in FINDINGS §6).
- The arm-stripping commits use one neutral message in every arm (`chore: prepare
  workspace`), so the log doesn't reveal the arm.
- Budgetron's `commit.pattern` binds in every arm. In arm 3, cinch's `commit-msg` hook
  enforces it. In arms 1–2, a plain `commit-msg` hook checks the same pattern from the
  pin's `cinch.yml` and prints cinch's rejection text, so a bad subject costs the same in
  every arm.
- Every agent runs in a bubblewrap sandbox (`sandbox.go`) built the same way in every arm,
  except the cinch binary and its PATH entry, which exist only in arm 3. The repo is mounted
  at `/work`. Home is empty except for the engine's own credentials and settings. `~/code`,
  the task files, the hidden tests and the harness cache don't exist. The Flutter SDK and
  the pub cache are copy-on-write overlays that get thrown away after each run. Setup
  checks each arm's sandbox and refuses to start if arms 1–2 can reach a cinch. The network
  is shared, so the transcript scans for `cinch` and benchmark paths still apply.
- The agent works on branch `feature/task`, so Budgetron's version-bump hook (which fires
  only for merges onto `dev`) never amends its commits.

**What arms 2 and 3 can't separate:** arm 3 adds the workflows and the hooks together. If
arm 3 wins, a follow-up arm (arm 3 without `.docs/workflows/`) attributes the effect. It is
pre-registered here but runs only after a win.

## Tasks

There are six tasks, and every one is a feature Budgetron will never build. That keeps them
out of its roadmap, so later Budgetron work can't contaminate a rerun (plan 23 § Pinning).
Benchmark output is never merged into Budgetron.

Three tasks are simple: one doc, an obvious rule, small code. They set the ceiling and
measure the cost side, such as blocked commits, bypasses and needless doc edits. The other
three are complicated in a way that one of cinch's mechanisms could catch. In arm 3 those
mechanisms are:

- **impact** (advisory, at commit): names every doc that owns a staged file, with its rules,
  and says "confirm the docs still match".
- **orphan** (blocks): a rule left with no test marker fails `cinch check`.
- **stray marker** (blocks): a marker naming a rule that doesn't exist fails `cinch check`.

Each spec fixes the public interface the hidden tests call, so a correct solution in any
style can be graded. No task mentions docs or rules. The prompt is a plain feature request
ending in: "Follow the repository's conventions. Add or update tests. Commit your work on the
current branch when done."

### Simple

| # | Kind | Task | Rules at stake | Corpus key |
|---|---|---|---|---|
| S1 | change | Background SMS scans run at most once every **13 minutes** instead of once a minute | SMS-006 | SMS-006 contains `13`, and no longer contains `once a minute` |
| S2 | change | Category names are unique **ignoring spaces**: `Eating Out` and `EatingOut` collide on add and on rename, throwing `DuplicateCategoryName` and writing nothing | CAT-001 | CAT-001 contains `space` |
| S3 | leave | `formatMoneyRoman(Money)` (top-level, in `lib/models/money_format.dart`) formats the major units as Roman numerals, dropping minor units and prefixing negatives with `-`. Nothing calls it yet | MONEY-002 must stay true | Every rule unchanged in meaning: MONEY-002 keeps `never changes stored values` |

S3 is there to measure false alarms. Its commit touches `lib/models/money_format.dart`, so
arm 3's impact names `money.md`, while the correct corpus edit is none. Rule-text edits that
the task didn't need are counted per arm (secondary).

### Complicated

| # | Kind | Task | Rules at stake | What cinch could catch | Corpus key |
|---|---|---|---|---|---|
| C1 | change (rule retires) | Category colors are no longer stored: each category's color is derived from its name (a fixed hash into a fixed palette). The `color` field goes, export stops writing it, and importing an older file that has it still works | CAT-002 (round-trip becomes false: remove or rewrite it); XFER-005 (derived values are never exported); XFER-001, XFER-004 must stay true | **impact**: the diff touches the category model and the exporter, so arm 3 names `categories.md` and `data-transfer.md`. **orphan** only fires if the agent deletes CAT-002's tests. Those test color-string conversion, which accounts still use, so they can stay valid and leave the false rule bound and green. Impact is therefore C1's main pull (builder's finding, 2026-10-28) | No rule still claims color round-trips: CAT-002 is gone, or it no longer contains `round-trip` and mentions the name. The XFER rules keep their tokens |
| C2 | change, across docs | Halved entries: an entry flagged `isHalved` counts at half its amount (rounded toward zero, in minor units) in account balances and in budget spent. Its stored amount stays whole, and the flag is exported | ACCT-001 (balance formula) and BUDGET-002 (spent formula) must both change; XFER-001 (the flag round-trips) and XFER-005 (halved sums are derived, never exported) must stay true | **impact**: the diff touches `balance_calculator.dart` (accounts.md) and `budget_calculator.dart` (budgets.md), so arm 3 names both docs. Updating one formula and forgetting the other is the expected miss | ACCT-001 **and** BUDGET-002 each match `half\|halv` |
| C3 | add, cross-cutting | Sealed categories: a category whose name starts with `#` is sealed. No write path may add an entry to it, move one into it, or edit or delete one in it: `EntryService` create, edit and delete; SMS confirm throws `SealedCategory`; confirm-all skips it. Renaming a category to or from `#…` seals or unseals it | A new rule expected (the invariant spans entry and SMS write paths, like the realtime leak); ENTRY-005 and SMS-004 must stay true | **orphan / stray marker**: a new rule written without a marker blocks, and so does a marker for a rule that was never written. **impact** names `sms-import.md` when the confirm path is touched | A rule ID absent at the pin exists, has a marker on a test file, and matches `seal` |

C3's hidden tests hit every write path separately. A solution that guards only
`EntryService.create` fails on behavior, whatever the corpus says.

Interfaces and file locations get checked against the pinned commit when each task's hidden
tests and reference solution are written. A task whose premise doesn't hold there gets
replaced before the pre-registration commit, not after.

## Grading (mechanical, no model in the loop)

The grader runs after the agent exits, on the final working tree, committed or not. It uses
its own `cinch` binary, outside the agent's environment, so it grades every arm the same way.

1. **Behavior.** The hidden tests for the task (`bench/upkeep/hidden/<task>/*.dart`) are copied
   to `test/_bench/` and run alone. A build failure counts as a fail. Hidden tests cover both
   the new behavior and every "must stay true" rule on the task's new code path. The agent
   never sees them.
2. **Corpus** (arms 2 and 3 only; arm 1 has no corpus), read from `cinch rules --json` on
   the final tree:
   - **change**: the named rule still exists with at least one marker. Its text contains
     the key's required tokens and none of its forbidden tokens. Tokens are literal, chosen
     so the change introduces them (for example S1 requires `13` and forbids `once a
     minute`). A rule the key allows to retire (C1's CAT-002) may instead be absent.
   - **add**: at least one rule ID absent at the pin now exists, carries a marker on a test
     file, and its text contains the key's required token (for example C3 `seal`).
   - **leave**: every rule present at the pin still exists with at least one marker, and
     each named rule still contains its key tokens (for example S3 MONEY-002 `never changes
     stored values`).
   - **All kinds**: no rule present at the pin has lost all its markers, unless the key
     lets it retire.
3. **Upkeep pass** (the primary outcome, for arms 2 and 3) is behavior pass **and** corpus
   pass.
4. **Bypass.** Any `--no-verify`, `git commit -n`, edit to `core.hooksPath`, or edit under
   `.githooks/` or `.git/hooks/` counts. It's detected from the transcript, and in arm 3
   also from the field log. A bypassed run is kept and counted as graded, and the bypass
   rate is reported per arm.
5. **Validity.** A run is invalid, and rerun once, if its transcript reads any path under
   `~/code/cinch/bench/`, runs `cinch` in arm 1 or 2, or never reaches the task (an engine
   error before the first edit). Invalid runs are logged, never silently dropped.

**Calibration before the pre-registration commit:** each task's reference solution
(`bench/upkeep/reference/<task>.patch`, with corpus edits) must pass behavior and corpus. The
untouched pin must fail behavior. A reference solution with the corpus edits reverted must
pass behavior and fail corpus. Keys get loosened only in the control's favour (FINDINGS §6),
and only before the commit.

## Analysis (pre-stated)

- **Primary:** the upkeep pass rate in arm 3 against arm 2, per engine, stratified by task:
  Cochran–Mantel–Haenszel, one-sided. It's a two-look group-sequential design, with
  O'Brien–Fleming-style boundaries and an overall one-sided α of 0.05 per engine:
  - **Look 1, at N = 5:** arm 3 beats arm 2 if p < 0.005 and the pooled difference is at
    least 15 points. Otherwise the owner either stops (the result is "not detected at
    N = 5") or runs stage 2 for that engine. Stopping without a win never inflates α, so the
    stop decision may use the stage-1 results.
  - **Look 2, at N = 10** (stage 2 is 5 more runs per task and arm, on the same pin, prompt
    and grader): arm 3 beats arm 2 if p < 0.048 on all 10 runs and the pooled difference is
    at least 15 points.
  - Nothing else changes between the looks. A stage-2 run that needs a grader or task fix
    voids the design for that engine, and the change gets reported.
- **Secondary:** the behavior pass rate across all three arms (arm 1 is the floor); corpus
  pass broken down by kind; bypass rate; commit rate; turns, tokens and wall-clock per arm.
  The ceiling tasks (100% in every arm) are reported separately, so they don't dilute the
  pooled rate (FINDINGS §6).
- **Reading,** from plan 25: if arm 3 doesn't beat arm 2, the docs and the pointer do the
  work and the checker is optional. That gets published on the front page. If arm 3 wins,
  that's the headline, and the arm-3-without-workflows follow-up runs next.

## Engines

There are two engines, split 50/50, and each is analyzed on its own. The prediction differs
by engine, so the primary test runs once per engine, never pooled across them.

- **pi with the local Qwen model** (pinned in `PINS` at the pre-registration commit).
- **Claude Code with `claude-sonnet-5-5`** (`claude -p`, with `--max-turns` as well as the
  wall-clock cap).

## Budget, power and schedule

Stage 1 runs N = 5 per task, arm and engine: 90 runs per engine, 180 in all. Each engine's
comparison pools 30 runs against 30. These are one-sided tests at 80% power, with arm 2 near
50%:

| Stage | N per task, arm and engine | Runs per engine | Detectable difference |
|---|---|---|---|
| 1 | 5 | 90 | about 38 points at α = 0.005 |
| 1 + 2 | 10 | 180 | about 23 points at α = 0.048 |

- **Run order:** randomized per engine with a seed fixed in this commit, interleaving arms
  and tasks. A time-varying condition (server load, an API change) then can't line up with
  an arm.
- **Parallelism:** the engines run side by side. pi/Qwen runs on one server slot, at the
  lab's standing config (`PI_SERVER` in `PINS`), so its runs are serial. A two-slot test on
  2026-10-28 found one usable config: two slots of about 64k context each, with roughly 1.6×
  the combined throughput. It cut each run's context from 92k to 64k and made individual
  runs slower against the 30-minute cap. It was declined, so the context budget and per-run
  speed stay as the model normally runs. Claude runs on a small worker pool, capped by host
  RAM (each run builds and tests a Flutter tree).
- **Infrastructure checks (before this commit, outside the stages):** the `-engine fake`
  sandbox probe in all three arms; `-apply` (all 18 runs pass behavior, and corpus passes in
  arms 2–3); `-apply-strip-docs` (corpus fails for S1, S2, C1, C2, C3 and passes for S3);
  and one `-smoke-prompt smoke-prompt.md` session per engine (Claude in arm 1, $0.03; pi in
  arms 1 and 3). The smoke prompt runs fixed shell probes and never sees a task. None of
  these runs counts toward the results.
- **Limits:** wall-clock per run is capped at 30 minutes. Timeouts are graded as they stand,
  never retried.

## Amendments

Every amendment to this pre-registration is its own commit, tagged `prereg/upkeep-amendN`. It
says what it changed, why, and after how many graded runs. A key amendment applies to arms
2 and 3 the same way, because arm 1 isn't graded on corpus. `-regrade` then re-evaluates every
run graded before it from the run's saved `rules.json`; behavior is never regraded. Each
changed run keeps its first grade in `corpus_first` and `upkeep_first`. The report gives the
primary result under the amended keys, and next to it the result under the keys at
`prereg/upkeep` (`-regrade -keys-ref prereg/upkeep`).

1. **`prereg/upkeep-amend1`, 2026-10-28, after 1 graded run (pi, `S1-arm2-r2`).**
   - **S1:** the `once a minute` forbid failed a correct SMS-006 that kept UI scans at one
     minute. The key now forbids only the pin's sentence, and a one-minute interval in the
     same sentence as "background".
   - **S2:** the `exact match` forbid had the same failure mode, so it's dropped. The
     `space|whitespace` match already fails the untouched rule.
   - Details and checked texts are in the tasks' `NOTES.md` files, and the same texts are
     pinned in `TestKeyAmendments`. Calibration is unchanged under the amended keys: `-apply`
     still passes corpus in all 12 arm 2–3 runs, and `-apply-strip-docs` still fails S1, S2,
     C1, C2 and C3 and passes S3.
   - The pi runs that started before this commit were graded by the harness at
     `prereg/upkeep`, which reads each key from disk when it grades. They're regraded here
     too. No Claude run had started.
2. **`prereg/upkeep-amend2`, 2026-11-01, after all 90 pi runs were graded.** It came after the
   per-arm results were seen. For pi, the amendment-1 result therefore stays the registered
   primary, and the amendment-2 grading is a labeled sensitivity result next to it. No Claude
   run had started, so for Claude the amendment-2 keys are the keys of record.
   - **S1:** the key is now SMS-006 matching `13 minutes`, with no forbid. The amendment-1
     forbids still rejected four correct texts (`S1-arm2-r1`, `S1-arm3-r0`, `S1-arm3-r3`,
     `S1-arm3-r4`): three state the background interval and a one-minute interval for other
     scans in one sentence, and one keeps "Scans run at most once a minute" beside a
     13-minute background rule. The hidden tests, not the key, grade what the code does.
   - **Grader:** a rule in a key's `retire` whose final text matches `(?i)\bretired\b` counts as
     retired even when `ifPresent` fails. cinch's rule-maintenance workflow retires a rule by
     keeping its item and saying it is retired, as `C1-arm3-r3` did with CAT-002. This
     applies to every task's key; only C1 has a `retire` entry.
   - The task set, prompts, hidden tests and references are untouched. Texts are in the S1 and
     C1 `NOTES.md` files and in `TestKeyAmendments`. `-regrade` re-evaluated every pi run from
     its saved `rules.json`; the sensitivity tables use `-regrade -keys-ref prereg/upkeep` and
     `-keys-ref prereg/upkeep-amend1` (the second needs the grader of this commit, since the
     retirement rule is in the code, not the key).

## Prediction (owner, 2026-10-28)

- **pi/Qwen:** a large win for arm 3 over arm 2, 30 points or more.
- **Claude Sonnet:** a modest difference, 15–30 points.

## Open decisions (settled before the commit)

1. ~~Engines and N~~: pi/Qwen and Claude Sonnet, 50/50; N = 5, with an optional stage 2.
2. ~~Arm 2~~: without workflows.
3. ~~Task vetoes~~: all six are features Budgetron will never build (2026-10-28).
4. ~~Lab slots~~: serial on the standing config, not two 64k slots (2026-10-28).
5. ~~Schedule seed~~: `20261028`.
