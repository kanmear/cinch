# Upkeep v2 and sequential drift: design (draft for pre-registration)

Plan [29](../../.docs/plans/29-benchmark-program-v2.md), items **C** (upkeep v2) and **E**
(sequential drift), claim #3: *cinch keeps docs true as code changes.* Part C is
[25](../../.docs/plans/25-upkeep-ablation.md) rebuilt around the six flaws plan 29 lists for its
stage 1 (`bench/upkeep/RESULTS-pi.md`). Part E runs C's tasks one after another on a single tree
per arm, then audits what is left.

This draft has no tasks, hidden tests, code or runs yet. It becomes the pre-registration in two
steps, both before any counted run, as in `bench/leak`:

1. **`prereg/upkeep2-pilot`**: this design; the candidate tasks with prompts, hidden tests,
   references, naive references, keys and judge probes; the grader; the judge rubric with its
   calibration passing; the pilot rule. No agent model call comes before it.
2. **`prereg/upkeep2`** and **`prereg/seqdrift`** (one commit, two tags): the pilot outcome
   applied mechanically (kept tasks per engine, the cap per task), E's sequence and orders, E's
   calibration audits, and `PINS` filled. Nothing else may change between the tags. A change to
   a prompt or hidden test after a pilot voids the pilots, and they are rerun.

E gets its tag before C's first counted run, so E's design can't be fitted to C's results.

---

# Part C. Upkeep v2

## Question

When an agent changes code in a way that makes a **far** rule false or puts it at risk, does
cinch's tooling make the agent keep that rule true, beyond what the docs and the `AGENTS.md`
pointer already do? A far rule is one the change wouldn't lead the agent to. And which piece of
the tooling does the work?

Stage 1 of 25 answered the first half on near rules for pi/Qwen: not detected at N = 5. Arm-2
agents found and edited the docs in 27 of 30 runs on the pointer alone, and cinch's own checks
never failed an agent's commit. Plan 29 reads that as the pointer doing the work wherever the
rule is obvious. It also notes that nothing has measured cinch where the checker could matter:
rules the agent wouldn't go looking for. C measures there.

## What v1 got right, kept unchanged

The following are carried over from `bench/upkeep/DESIGN.md` and its harness (`sandbox.go`,
`provision.go`, `engine.go`), except where a section below says otherwise:

- **Fixture and pin.** Budgetron, the scrubbed mirror `~/code/budgetron.scrubbed.git`, tag
  `cinch-bench/upkeep-v1` → `2907c92cdf886eb269d7bb91573e6c0841a9022a`, with `make gen` and
  `make setup-test` run before the agent starts (and `mkdir -p build`, C1's note). Census A
  audited `0fe3070` and census B writes its questions from it. That commit has the same `.docs/`
  and the same rules as `2907c92` (`bench/recover/DESIGN.md` § Corpora and pins), so A's
  verdicts and B's list apply to this pin as they stand.
- **Sandbox.** The bubblewrap sandbox is built the same way in every arm. The repo is at `/work`
  and home is empty apart from the engine's credentials. `~/code`, the task files, the hidden
  tests and the harness cache don't exist inside it. The Flutter SDK and the pub cache are
  copy-on-write overlays thrown away after each run. Setup refuses to start if an arm can reach a
  cinch it shouldn't have (arms 1 and 2: none; arm 4: anything but `impact`).
- **Gate parity.**
  - Budgetron's `make check` minus `cinch check` runs as a plain `pre-commit` hook in every arm
    without cinch's hooks. Format, l10n, analyze and the hardcoded-strings check fire everywhere.
  - Commit-message parity: a plain `commit-msg` hook checks the pin's `commit.pattern` and prints
    cinch's rejection text, so a bad subject costs the same in every arm.
- **Neutral history.** The arm-stripping commit has one message in every arm (`chore: prepare
  workspace`). The agent works on branch `feature/task`, so the version-bump hook never fires.
- **Hidden tests.** They are copied to `test/_bench/<task>/` after the run and run alone. A build
  failure counts as a fail. The agent never sees them.
- **Bypass detection.** Any of `--no-verify`, `git commit -n`, an edit to `core.hooksPath`, or an
  edit under `.githooks/` or `.git/hooks/` counts. It's read from the transcript, and in arm 3
  also from the field log. A bypassed run is kept, graded, and counted per arm.
- **Validity.** A run is invalid, logged and rerun once if it reads a path under
  `~/code/cinch/bench/`, runs a cinch it doesn't have, or never reaches the task (an engine error
  before the first edit). Invalid runs are listed, never dropped silently.
- **Two-look group-sequential analysis** with O'Brien–Fleming-style boundaries and a 15-point
  floor (below).
- **Amendment discipline.** Every amendment is its own tagged commit (`prereg/upkeep2-amendN`)
  saying what changed, why, and after how many graded runs per engine. A change made after any
  per-arm result has been seen can't become the registered primary for that engine (25's
  amendment-2 rule). It is a labeled sensitivity result.
- **Infrastructure checks before the tag**, extended to four arms (§ Infrastructure checks).
- **Tasks are features Budgetron will never build** (owner veto per task). Benchmark output is
  never merged into Budgetron.

## The six flaws and the mechanism that fixes each

| # | Flaw in v1 | Mechanism in v2 | Section |
|---|---|---|---|
| 1 | The 30-minute cap bound C1 in every arm | Per-task cap from pilots, by a formula fixed now. The primary grades timeouts as they stand; "conditioned on finished runs" is a pre-registered sensitivity, and a task that turns cap-bound during the study is flagged and reported both ways | § Pilot and cap |
| 2 | S2, S3 and C3 sat at the ceiling | Arm-2 pilots per task and engine; a task is kept only at 20–80% upkeep pass | § Pilot and cap |
| 3 | Regex keys on prose: three rounds of false negatives | Structural signals graded mechanically, plus a blinded judge from the other model family grading meaning against a truth statement written before any run, plus a blind owner audit sample. No regex on prose anywhere in the registered grade | § Grading |
| 4 | The docs impact was obvious | Tasks target far rules only, drawn from census B's normative list and census A's under-enforced and misplaced rows by a procedure fixed now. Each task's far type and impact reach are established mechanically | § Task selection |
| 5 | N = 5 detects only about 38 points | Four kept tasks, N = 10 at look 1 and 15 at look 2. 80% power at about 22 points | § Power |
| 6 | Arm 3 bundles workflows, `impact` and the check | Arm 4: docs, pointer and the `impact` reminder only. The arms nest (1 ⊂ 2 ⊂ 4 ⊂ 3), so each adjacent contrast adds one layer | § Arms |

## Arms

The arms nest: each adds one layer to the one before it. Prompt, pin, toolchain, sandbox and
non-cinch gates are the same in all four.

| | Arm 1: no docs | Arm 2: docs + pointer | Arm 4: docs + pointer + impact | Arm 3: docs + cinch |
|---|---|---|---|---|
| `.docs/` domain, architecture, conventions, testing | removed | present | present | present |
| `.docs/workflows/` | removed | removed | removed | present |
| `AGENTS.md` | Project section only (`bench/upkeep/arms/arm1`) | pointer without cinch commands (`bench/upkeep/arms/arm2`) | **identical to arm 2** | as at the pin |
| `cinch.yml`, `.githooks/`, `core.hooksPath` | removed | removed | removed | present |
| `pre-commit` | plain: `make check` minus `cinch check` | plain, same | plain, same, **then the impact advisory** | cinch's hook (`make check` with `cinch check`, impact advisory) |
| `commit-msg` | plain pattern hook | plain pattern hook | plain pattern hook | cinch's |
| `cinch` on `PATH` | no | no | **impact-only binary** | full (pinned version) |
| `cinch:rule` markers in tests | kept | kept | kept | kept |

**Arm 4 in detail.** The agent's `PATH` gets `cinch`, a binary built from this repository
(`bench/upkeep2/impactonly`, importing `internal/cinch`, docs root fixed to `.docs`, so arm 4
needs no `cinch.yml`). It has one subcommand, `impact`. Every
other subcommand prints `cinch: only 'impact' is available in this workspace` and exits 2. The
plain `pre-commit` hook runs the non-cinch checks first and then this binary's capped hook mode.
That prints the same advisory block arm 3's hook prints, byte for byte, including `confirm the
docs still match`, and it never blocks. `AGENTS.md` is arm 2's, word for word. The agent learns
of impact the way an arm-3 agent learns of it: from the hook's output when it commits. The
advisory's last line points at `cinch impact`, and the agent can run it.

**Why there's no `AGENTS.md` line about impact.** With such a line, arm 4 against arm 2 would
measure a prose reminder plus a tool, and the R0 stratum below couldn't separate them. Arm 3
keeps the pin's `AGENTS.md`, so it stays comparable with v1. The variant with an impact line in
both arms is an open decision.

### What each contrast attributes

| Contrast | Adds | Attributes | What it can't separate |
|---|---|---|---|
| **2 vs 1** | the corpus and the pointer | whether having the docs changes **behavior** on a far rule's clause (arm 1 isn't graded on corpus) | reading the rule from reading neighbouring prose; arm 1 keeps markers and code comments, so the gap is conservative |
| **4 vs 2** | the commit-time impact advisory, and `cinch impact` on demand | the reminder alone: a list of owning docs, marked rules and "confirm the docs still match" | the list's content from the generic nudge. The R0 stratum (tasks where impact names no key rule or doc) separates them: there, any arm-4 gain comes from the nudge, not the list |
| **3 vs 4** | `cinch check` (orphan, stray marker, links, generated), the rendered workflow pack, and `cinch index`, `workflows`, `rules` | everything in cinch except impact | the check from the workflows. A follow-up arm (arm 3 without `.docs/workflows/`) is pre-registered to run only if 3 beats 4 at the winning engine |
| **3 vs 2** | the whole layer | v1's primary question, on far tasks | which part of the layer did it (the two rows above split it) |

**The acknowledgment gate.** Plan 29 also proposes an arm 5: a commit that touches owned code
without its doc must carry a `Rules-checked: <IDs>` trailer. That is a proposed feature, not
what ships, so it is not an arm here. It is pre-registered as a follow-up that runs at an engine
only if both of these hold:

- arm 4 does not beat arm 2 there;
- the transcripts show **reach without reaction**: in at least half of the arm-4 runs on R1 and
  R2 tasks, the advisory named the key rule or its doc, and in most of those runs no corpus edit
  followed.

## Task selection

### Inputs

- **B: the normative list.** `bench/recover/normative-budgetron.json`, from census B.
  **TODO: input, not yet produced** (B is pre-registered at `prereg/recover` and has not run). B's
  designer predicts 3 of 39 Budgetron rules normative (BUDGET-007, ENTRY-006, SMS-003's
  re-application clause).
- **A: the drift census rows**, fixed now, from `bench/drift/budgetron-verified.csv` (verifier
  findings included, as A's headline counts them):

| Set | Rules | Use here |
|---|---|---|
| A_U, under-enforced | MONEY-001, SET-001, SET-002, SMS-001, SMS-003, SMS-007 | eligible, except SET-001 and SET-002 (their unpinned clause lives in `main()`, which a hidden test can't reach) |
| A_M, marker misplaced | ENTRY-005, ENTRY-006, SMS-004 | eligible: the marked tests don't pin the clause, so neither the marker channel of `impact` nor the orphan check connects it |
| A_T, tension | SMS-002, SMS-008, SMS-006 | **excluded as targets**: they are already false at the pin, and a task on them mixes upkeep with fixing old drift |

### Procedure (applied in order; each step's outcome is written to `tasks/SELECTION.md`)

1. **Eligible rules.** E = (B ∪ A_U ∪ A_M) minus A_T minus {SET-001, SET-002}. Without B, the A
   part is {MONEY-001, SMS-001, SMS-003, SMS-007, ENTRY-005, ENTRY-006, SMS-004}.
2. **Tiers.** Tier 1 = in B and in A (normative and under-enforced: the rule is worth protecting
   and nothing protects it). Tier 2 = in B only. Tier 3 = in A only.
3. **Sketch tasks.** Up to two task sketches per eligible rule. A sketch qualifies only if all of
   the following hold:
   - **(a) Never built.** It is a feature Budgetron will never build (owner veto).
   - **(b) Far**, of at least one of two types, both checked mechanically and recorded in
     `key.json`:
     - *by ownership*: no `owns:` path of the key rule's doc matches a non-test file that the
       naive reference changes;
     - *by side effect*: the prompt contains none of the focus words the key lists for the
       rule's clause (case-insensitive, whole word), and the naive reference breaks that clause.
   - **(c) Breakable.** A competent code-only solution, `reference-naive.patch`, passes the
     feature's hidden tests (F) and fails the clause's hidden tests (K), or (change kind) leaves
     the rule's text false. A sketch whose obvious solution keeps the rule true by accident has
     nothing to measure.
   - **(d) Gradable.** Hidden tests can pin the clause on the new path, and the change can be
     stated as a truth statement of one or two sentences for the judge.
   - **(e) Fixed interface.** The prompt fixes the public interface the hidden tests call and
     never mentions docs, rules or cinch. It ends with v1's closing sentence.
   - **(f) Size.** The reference's code diff (outside `.docs/` and `test/`) is at most about 400
     lines, so the cap stays a cap.
4. **Rank** the qualifying sketches:
   - first by tier;
   - then by far type (both types ahead of ownership only, ahead of side effect only);
   - then by kind (change ahead of leave-and-comply);
   - then preferring rules in B whose focus clause B's code arm got WRONG ("misleading"), ahead
     of ABSTAIN and PARTIAL.

   At most one task per rule, and at most two per rule doc.
5. **Draft the top 8** as full candidates (prompt, hidden tests, both references, key, probes)
   and calibrate them mechanically (§ Calibration). A candidate that fails calibration is replaced
   by the next sketch before `prereg/upkeep2-pilot`, never after.
6. **Kind mix.** At least half of the candidates are **change** kind: the far rule's text must
   change. Leave-and-comply tasks (the rule stays true; the new code must obey it) measure reach
   more than upkeep and may fill at most half.
7. **Reach diversity.** Among the drafted candidates, at least two are expected R0 and at least
   two R1 or R2 (reach classes below), so the impact contrast has a negative control.
8. **Shortfall.** B's design says that if B finds fewer than four normative Budgetron rules, C
   records the shortfall and takes the rest from DeltaDocs as a private arm, without relabelling
   descriptive rules. This design admits A's rows as a separate source, labelled tier 3 and never
   called normative. So the DeltaDocs fallback fires only if fewer than four candidates from
   tiers 1–3 survive the pilot at some engine. **This reading of B's rule needs the owner's
   approval** (open decision 1).

### Impact reach class (mechanical, per task, recorded at calibration)

The grader's cinch runs `cinch impact` on the file list of `reference-naive.patch` (code and
tests, no `.docs/`). That is what the advisory would tell an agent who wrote the obvious code.

- **R0, unreached.** Impact names neither the key rule (no marker in a changed file) nor its doc
  (no `owns:` match). Only reading the docs can save the rule. In arms 3 and 4 impact is either
  silent or points at other docs.
- **R1, marker-reached.** A marker of the key rule sits in a file the naive patch changes.
- **R2, owns-reached.** The key rule's doc owns a file the naive patch changes. That makes the
  task far by side effect only.

The orphan check reaches a task only if the naive patch deletes or renames every test that
carries the key rule's markers. This is recorded as **O1** (reached) or **O0** (not), and is
expected to be O0 almost everywhere: v1 showed that check never firing.

### Provisional sketches (designer's, before B; not yet checked against the pin's interfaces)

These seed step 3. Only the paths and markers below were checked at the pin, with `git grep` on
the mirror. The interfaces and whether the naive solution really breaks the clause are checked
when a sketch is drafted.

| Id | Sketch | Key rule and clause | Kind | Far | Expected reach | Tier (pre-B) |
|---|---|---|---|---|---|---|
| U1 | New installs default to the "Recent" entries sort; existing stored choices are kept | ENTRY-006 "(the default)", today amount descending | change | ownership: the default is set in `lib/cubit/settings_cubit.dart:49`, which `settings.md` owns, not `entries.md`; the tests that pin the default carry no ENTRY-006 marker | R0 (impact names `settings.md`) | 3; 1 if B confirms |
| U2 | The rule-keyword suggestion on confirm drops trailing store numbers ("SHOP SOSEDI 123" → "SHOP SOSEDI") | SMS-004 "(the form offers the whole merchant)" | change | ownership: `lib/routes/pages/entry/cubit/entry_form_cubit.dart:110` is owned by no doc; the rule is in `sms-import.md` | R1 if the agent edits `entry_form_cubit_test.dart` (2 SMS-004 markers), else R0 | 3 |
| U3 | Paste a list of "keyword → category" lines to create category rules in bulk | SMS-003 "re-applied … whenever one is saved … in the same transaction as the rule write" (only the remove path is pinned) | leave-and-comply | side effect: the prompt is about bulk input; the obvious loop of rule writes skips the re-apply or runs it outside the transaction | R2 (`lib/logic/sms`) | 3; 1 if B confirms |
| U4 | Import messages from an SMS backup file; a message already read from the inbox is never imported twice | SMS-001 "(provider id plus receive time)": backup ids differ from inbox ids, so the identity must change | change | side effect: the prompt talks about duplicates, not about the key | R2 (`lib/models/sms/sms_message.dart:16`) | 3 |
| U5 | Recategorize many selected entries at once | ENTRY-005: usages move with a category-changing edit, atomically (misplaced: pinned only under ENTRY-004's marker) | leave-and-comply | side effect | R2 (`lib/logic/entry`) | 3 |
| U6 | Planned entries: a future-dated entry with an account and a category, posted when its date arrives | ACCT-004's list of what an account delete clears; XFER-001's list of what export carries | change (two far rules) | ownership: new code under a new folder; `accounts.md` and `data-transfer.md` own neither | R0 unless the agent edits the account or transfer code | **not eligible** unless B marks ACCT-004 or XFER-001 normative (open decision 2) |
| U7, U8 | reserved for B's normative rules that U1–U5 don't cover | | | | | 1 or 2 |

U1–U4 are small, so the cap shouldn't bind for Qwen. U3 and U5 are leave-and-comply, and U6 is
the classic side-effect task: enumerations that go stale. Census A found that rules restating
code (enumerations among them) are not where the drift is, which is why U6 waits on B.

## Grading

The grader runs after the agent exits, on the final working tree, committed or not. It uses its
own cinch, outside the agent's environment, so every arm is graded the same way. Every component
below is computed for every run. Arm 1 has no corpus and gets behavior only.

### 1. Behavior (mechanical)

`hidden/*.dart` holds two groups, named by prefix:

- **F**: the feature works;
- **K**: the far rule's clause holds on the new path, and on any old path the change could
  reach.

The **behavior pass** is F and K all passing. F-only and K-only are recorded too.

### 2. Structural signals (mechanical)

These are read from `cinch rules --json` on the final tree, compared with the pin and the run's
diff. For each rule the key names:

| Signal | Definition | In corpus pass? |
|---|---|---|
| **G1 present** | the rule exists. A rule the key marks retirable may instead be absent, or present with text matching `(?i)\bretired\b` (v1 amendment 2), or carry cinch's retire marker if the pinned cinch version parses one | yes |
| **G2 text changed** | change kind: the text differs from the pin after whitespace normalization. Leave kind: not required (a rewording is judged) | yes, change kind |
| **G3 marked test changed** | a file carrying the rule's marker (at the pin or at the end) changed in the diff, or a new marker for the rule was added | **no**: reported as "bound upkeep". A correct fix can legitimately leave the marked tests alone (U1's marked test never pinned the default) |
| **G4 marker on a test** | the rule has at least one marker in a file under `test/`. For a new rule (add kind), some ID absent at the pin has one | yes |
| **G5 no collateral orphan** | every rule present at the pin and not retirable still exists with at least one marker | yes |

Two more measures are secondary: **collateral edits** (rules outside the key whose text changed)
and **inherited state** (did `cinch check` pass on the final tree; this applies in every arm,
since the grader runs it).

### 3. Meaning: a blinded judge from the other model family

- **Who.** Claude Sonnet (`JUDGE_MODEL`, `claude -p`) judges the pi/Qwen runs. pi/Qwen judges the
  Claude runs, as in bench/leak, subject to Qwen passing calibration as a judge. If it fails, the
  owner picks another non-Claude judge before the tag. Each call runs in an empty directory with
  no tools.
- **What it sees**, filled into `rubric.md` (adapted from `bench/recover/rubric.md`):
  - the **truth statement**: what the code does after the change, on the rule's clause, written
    in `key.json` before any run;
  - the **required clauses** and the **not-required** ones;
  - the rule's text at the pin;
  - the rule's final text.

  In both texts the list number and bold ID are stripped, and so are the words `cinch`,
  `workflow` and `.docs/` paths. The judge never sees the arm, the engine, the run ID, the
  transcript, the code, the diff or the hidden-test result.
- **Verdicts:**
  - **CORRECT**: the final text is true of the changed behavior on the clause the task changed,
    and keeps every still-true clause the key requires. Wording doesn't matter.
  - **PARTIAL**: nothing false, but a required clause is missing or too vague to check.
  - **STALE**: it still asserts the pin's behavior on the changed clause.
  - **WRONG**: it asserts something the truth statement contradicts.

  For leave kind, CORRECT means the final text says what the pin says. An unchanged text gets
  STALE (change kind) or CORRECT (leave kind) mechanically, with no judge call. A retired or
  absent retirable rule is CORRECT when the key allows it.
- **Order.** Candidates from every arm, task and rep are shuffled with `SCHEDULE_SEED` and the
  engine name, and judged after that engine's last run. Pilot runs are judged as they finish
  (they are excluded from every result).
- **Calibration before `prereg/upkeep2-pilot`**, for each judge:
  - Per task:
    - the reference's rule text → CORRECT;
    - a paraphrase of it → CORRECT;
    - the pin's text → STALE;
    - a text that adds the new behavior but keeps the old contradicting clause → WRONG;
    - a text that drops a required still-true clause → PARTIAL;
    - for a retirable rule, the retired text → CORRECT.
  - **v1 transfer set.** Every *changed* SMS-006 and CAT-002 text from v1's 20 arm-2 and arm-3
    pi runs of S1 and C1 (unchanged texts are graded mechanically and left out). Each carries the
    verdict the owner settled at `prereg/upkeep-amend2` (both tasks' `NOTES.md`), and is judged
    against truth statements written now for S1 and C1. At least 90% must match.

  Every per-task probe must get its expected verdict. A failure fixes the rubric before the tag.
- **Agreement checks:**
  - **Self-agreement.** A seeded 10% of candidates are judged twice, and agreement on CORRECT
    versus not is reported.
  - **Blind owner audit.** Per engine, the owner sees 40 judged candidates, or all of them if
    fewer, drawn as a seeded random sample stratified by judge verdict. The owner sees the truth
    statement, the required clauses and both texts, and never the arm or the verdict.
  - Below 85% owner–judge agreement on CORRECT versus not, the corpus grade is reported as
    judge-unreliable. An amendment then revises the rubric and re-judges every candidate. Agent
    runs are never repeated for a judge change.

### 4. Outcomes

- **Corpus pass** (registered): G1, G2 (change kind), G4 and G5 hold, and every key rule's judge
  verdict is CORRECT.
- **Upkeep pass** (primary; arms 2, 3 and 4) = behavior pass **and** corpus pass.
- **Sensitivities:**
  - PARTIAL counts as CORRECT;
  - structural-only corpus (no judge);
  - G3 added to the corpus pass ("bound upkeep").

### 5. Transcript measures (secondary; arms 3 and 4 unless noted)

- **impact fired**: the hook printed an advisory that names the key rule or its doc;
- **impact acted on**: a `.docs/` edit after the first such advisory;
- the cinch subcommands the agent ran itself;
- whether the agent read the key rule's doc (all arms with docs), and when, relative to its first
  code edit;
- hook blocks by cause;
- turns, tokens, wall clock.

### Publication

Budgetron is public, so everything is published with the results:

- transcripts and final patches;
- the judge's inputs and outputs, and the owner audit;
- the per-run CSV.

Anyone can regrade.

## Calibration (before `prereg/upkeep2-pilot`, no agent model)

For each candidate, in every arm that grades it, through the harness's `-apply` path:

1. The untouched pin fails F.
2. `reference.patch` passes F and K, corpus passes (structural, and the judge says CORRECT), and
   `make check` is green, `cinch check` included.
3. `reference-naive.patch` (code only, no `.docs/` hunks) passes F, and fails K or fails corpus.
   This is the breakability criterion (c).
4. `reference.patch` with its `.docs/` hunks reverted passes behavior and fails corpus (change
   kind).
5. The hidden tests are green 10 times in a row on `reference.patch`.
6. The reach class (R0/R1/R2), the orphan reach (O0/O1) and the far type are computed and written
   to `key.json` and `NOTES.md`.
7. **Shuffled-arm grading check**: the reference's final tree, provisioned as arms 2, 3 and 4
   (arm-specific files normalized), gets identical structural signals.

## Pilot and cap (between the two tags)

### Pilot runs

Pilot runs are graded through the full pipeline, judge included, and **excluded from every
result**.

- **Per candidate and engine:** 5 runs in arm 2, plus 1 timing run in arm 3, at a provisional cap
  of 90 minutes.
- **Two-stage rule for the 20–80% filter:**
  - 2 or 3 upkeep passes of 5: keep.
  - 1 or 4 of 5: run 5 more arm-2 pilots and keep if 2–8 of 10 pass.
  - 0 or 5 of 5: drop. The task is reported as floor or ceiling.
- **Cap-bound drop.** A candidate is also dropped if 2 or more of its first 5 arm-2 pilots fail
  to commit before 90 minutes. Its arm-3 timing run must commit too; if it doesn't, one more is
  run, and the task is dropped if that one doesn't either.
- **Engine order.**
  - pi pilots run first on every candidate.
  - Claude pilots run only on candidates pi did **not** drop at the ceiling. A task Qwen passes
    5 of 5 in arm 2 is assumed to sit at Sonnet's ceiling too. That assumption is labelled in the
    report.
  - Claude pilots are paid: the owner approves them separately.
- **Per-engine task sets.** Each engine's primary uses the tasks its own pilot kept. A
  cross-engine comparison is secondary and restricted to the tasks both engines kept.
- **Minimum.** At least 3 kept tasks per engine; otherwise that engine's primary doesn't run and
  the owner decides on replacements under a new pilot tag (§ Task selection, step 8). At most 4
  are kept per engine: if more qualify, the 4 highest in the ranking.

### The cap per task

C = the larger of 30 minutes and 1.5 × the slowest committing pilot (arm-2 pilots and the arm-3
timing run), rounded up to 5 minutes, at most 90.

- The same C applies to every arm of that engine. Each engine gets its own C, since Qwen and
  Sonnet differ several-fold in speed.
- Claude also gets `--max-turns 80`.

### During the study

- A timed-out run is graded on its tree as it stands, and never retried.
- If more than 25% of a task's runs in any compared arm time out, the task is flagged
  **cap-bound**. The primary is then also reported without it.

### Conditioned on finished runs (pre-registered sensitivity, not the primary)

The upkeep-pass contrasts are recomputed over committed runs only. This is not the primary
because finishing is itself a post-treatment outcome. If arm 3's workflow reading slows agents
down, conditioning on finishing selects different agents in different arms. The calibrated cap is
the primary fix; conditioning shows how much the cap still matters.

## Analysis (pre-stated)

### Primary

Per engine: the **upkeep pass rate**, stratified by task (Cochran–Mantel–Haenszel, one-sided),
for three contrasts tested in a fixed sequence. Each is tested only if the one before it won:

1. **H1: arm 3 > arm 2.** The whole layer, v1's question on far tasks. It goes first because it
   has the largest expected effect (arm 3 contains arm 4).
2. **H2: arm 4 > arm 2.** The impact reminder alone.
3. **H3: arm 3 > arm 4.** Check, workflows and the CLI beyond impact.

Each contrast uses v1's two looks and boundaries:

- **Look 1** at N = 10 per task and arm. A win needs p < 0.005 and a pooled difference of at
  least 15 points.
- **Look 2** at N = 15. A win needs p < 0.048 on all 15 and at least 15 points.
- If H1 hasn't won at look 1, the owner may stop that engine: the result is "not detected at
  N = 10". All arms stop or continue together. No arm is dropped between the looks.
- The exact stratified (permutation) p is reported beside the CMH p.
- `power.py` (written before the tag) simulates the sequence with the looks and checks that the
  family-wise one-sided α stays at or below 0.05.

### Key secondaries, reported with the primary

- **4 vs 2 by reach class.** R1+R2 against R0. Reading rule:
  - a gain only in R1+R2 is the advisory's content working;
  - an equal gain in R0 is the generic "confirm the docs" nudge;
  - no gain in either is no effect.
- **2 vs 1 on behavior** (Fisher per task, CMH pooled, one-sided): do the docs help the agent
  keep a far rule's clause in code?
- **3 vs 2 on R0 tasks** alone: there, only the check and the workflows can help, and the check
  can't see an R0 rule unless the agent orphans it.

### Secondary

- behavior and corpus separately, per arm;
- the sensitivities of § Grading 4 and the conditioned-on-finished rows;
- the transcript measures;
- bypass rate, commit rate, timeouts;
- collateral edits per arm, which is the cost side that v1's S3 measured;
- per-task tables;
- the cross-engine comparison on shared tasks.

### Reading (pre-stated)

- **H1 not detected on both engines.** Together with v1, claim #3 is closed as "not detected"
  on near and on far rules, at N = 15 with a detectable difference of about 22 points. That goes
  on the front page.
- **H1 wins and H2 wins, H3 doesn't.** Impact carries the effect, and the check and workflows add
  nothing detectable. That supports plan 29's "small agent surface" direction.
- **H1 wins, H2 doesn't.** The effect is in the check or the workflows, and the arm-3-without-
  workflows follow-up runs next.
- **H2 null with reach without reaction** (§ Arms): the acknowledgment-gate arm runs next.

## Engines

Analyzed separately and never pooled. The prediction differs by engine.

- **pi with the local Qwen model**: v1's stack (`PI_*` in `PINS`), the lab's single slot, serial.
  Free.
- **Claude Code with `claude-sonnet-5-5`** (`claude -p`, `--max-turns 80`, plus the wall cap).
  Paid: the owner approves the Claude pilots, then the counted Claude runs, separately, each at a
  cost measured on the stage before it. No Claude run starts without approval.

## Power and budget

### Power

The simulation used v1's CMH and two looks with the 15-point floor. The 4 kept tasks were given
arm-2 rates spread over 0.3–0.7, and every task got the same added difference. With N = 10 at
look 1 and 15 at look 2, one contrast at one-sided α = 0.05:

| True difference | Look 1 | Overall |
|---|---|---|
| 0 | <0.01 | 0.04–0.05 |
| 15 points | 0.12 | 0.53 |
| 20 points | 0.24 | 0.75 |
| 25 points | 0.45 | 0.91 |
| 30 points | 0.64 | 0.97 |

So the design detects a difference of **about 22 points with 80% power**, against 38 points at
v1's N = 5. Look 1 alone only catches 30 points or more. Two more figures:

- With 3 kept tasks: 64% at 20 points, 82% at 25.
- With Bonferroni co-primaries at α = 0.025 (open decision 3): 63% at 20 points, 86% at 25.

The 15-point floor caps power near 50% for a true 15-point effect, whatever the N.

### Runs per engine (4 kept tasks)

| | Arm 1 | Arm 2 | Arm 4 | Arm 3 | Total |
|---|---|---|---|---|---|
| Look 1 (N = 10) | 40 | 40 | 40 | 40 | 160 |
| Look 2 (+5) | 0 | 20 | 20 | 20 | 60 |

- Arm 1 stops at N = 10: its only contrast (2 vs 1, behavior) is secondary.
- Pilots: pi up to 8 × (5–10 + 1–2), about 50–95 runs. Claude: the same per candidate pi didn't
  drop at the ceiling.

### Hours

pi runs are serial. v1's median was 7–9 minutes, and far tasks are a little larger, so plan on a
15-minute mean until the pilots measure it:

| | Hours |
|---|---|
| Pilots | about 15–25 |
| Look 1 | about 40 |
| Look 2 | about 15 |

Claude runs on a pool of 2, capped by host RAM.

### Cost

- pi runs: none.
- Sonnet judge calls on pi-run rule texts: a few hundred short calls.
- Claude runs: the pilots measure the cost per run, and 220 × that figure goes to the owner.
- The Qwen judge on Claude runs: free.

### Schedule and build

- **Run order:** randomized per engine with `SCHEDULE_SEED` (proposed `20261103`) and the
  engine's name, interleaving tasks and arms. Look 2 continues the same seed.
- **Build:** about five sessions:
  - the harness: v1's, with arm 4, the impact-only binary, F/K grading, the structural signals,
    and recover's judge path adapted;
  - one session per two candidates: prompt, hidden tests, both references, key, probes,
    calibration.

## Infrastructure checks (before `prereg/upkeep2`, outside the results)

- **The `-engine fake` sandbox probe in all four arms:**
  - arms 1 and 2 can't reach any cinch;
  - arm 4's `cinch impact` works, and `cinch check`, `cinch index` and `cinch workflows` are
    refused;
  - arm 3 has the full binary at the pinned version.
- **Advisory parity.** The same staged diff (each reference's code) in arms 3 and 4 gives a
  byte-identical impact block on the hook's stderr.
- **Apply checks:**
  - `-apply` (every reference passes behavior in all arms, and corpus in arms 2–4);
  - `-apply-naive` (behavior or corpus fails, as calibration step 3 says);
  - `-apply-strip-docs` (corpus fails for every change-kind task).
- **Smoke-prompt sessions.** One per engine and arm, with fixed shell probes and no task: arms
  1–4 for pi; arms 1 and 4 for Claude (paid, a few cents).
- **Commit-msg parity:** a bad subject is rejected with the same text in every arm.

None of these runs counts.

---

# Part E. Sequential drift

## Question

Over a sequence of changes, how much drift builds up in a rule corpus under each arm? Is it more
than the sum of the single-task misses C measures?

Plan 29 calls this the **truth** factor: P(a rule is still true | the code it describes changed),
accumulated. C measures it one change at a time from a clean pin. E lets every miss stay in the
tree, where it can mislead the next task, and then grades the final corpus with census A's audit.

## Design

- **Tasks.** Six, the same six for both engines. They are the top six candidates in C's ranking
  that passed calibration and were not dropped as cap-bound at the pi pilot. The 20–80% filter
  doesn't apply: E doesn't need each task to discriminate. If fewer than six qualify, E runs with
  what there is (at least four) and says so.
- **Overlap.** At least two pairs among the six share a rule doc (U2, U3 and U4 all live in
  `sms-import.md`), so a stale rule from one task can meet a later one. Those pairs are named in
  `sequence.json` at the tag, and the compounding measure is reported on them separately.
- **Arms:** C's four, same provisioning, same prompts, the same per-task cap C per engine.
- **Chain:** one tree per arm and replicate.
  - Task k starts from the tree task k−1 left. Each task is a fresh agent session: only the tree
    carries anything from one task to the next.
  - Before task k, the harness commits whatever the previous agent left uncommitted, with message
    `chore: prepare workspace` and hooks disabled (the same in every arm). The branch stays
    `feature/task`.
  - Hidden tests never enter the chain. Every grading runs on a copy.
- **Ordering: seeded, paired.** Replicate r uses order π_r, the same for all four arms, so arms
  are compared on identical sequences. The pi orders are the 6 rows of a Williams design for six
  tasks (each task in each position once, first-order carry-over balanced), randomized from
  `ORDER_SEED` (proposed `20261104`). `sequence.json` holds the orders at the tag.
- **Replicates:**
  - **pi: R = 6**, one Williams square. That is 4 arms × 6 replicates = 24 chains, 144 task
    runs.
  - **Claude: R = 6 on the same square if the owner approves; otherwise R = 3**, the first three
    rows, which makes the Claude results descriptive only (a sign-flip test on 3 pairs can't
    reach 0.05).
  - A second square for pi (R = 12) is an owner decision **before** the tag, never after a
    result.

### A failure in task k

The chain always carries on from the agent's tree as it stands. Nothing is repaired: any repair
would put the harness's code or docs into the arm's tree.

- **Behavior failure:** the next task starts there anyway. Task k's failure counts in its step
  grade and in the end-state measures.
- **Timeout:** the tree as it stands, committed by the harness, is task k+1's start.
- **Build break.** `flutter analyze` reports errors, or the visible suite fails to compile, at
  the start of task k+1. The chain continues, since that is what a developer inheriting the tree
  meets. It is flagged **broken at k**. The primary includes broken chains; a sensitivity drops
  every replicate with a broken chain in any arm (pairing preserved).
- **Inherited gate failure, arm 3.** The harness's commit skips hooks, so a tree can carry an
  orphan or stray marker into the next task. The next agent's commit is then blocked until it
  fixes it. That is the CI pressure cinch is meant to exert, so it is counted ("inherited
  blocks"), not prevented.
- **Invalid run** (an engine error before the first edit, or a forbidden path read): rerun once
  from the same starting tree. If it is invalid again, task k counts as failed, and the chain
  continues from the unchanged tree.

### Grading

**Per step** (a copy of the tree after each task): C's whole pipeline for task k (behavior,
structural, judge). This gives per-step upkeep, comparable with C's independent runs.

**End state**, on a copy of the final tree:

1. **Every task's hidden tests**, on the final tree:
   - **end-state behavior**: tasks passing, out of 6;
   - **regressions**: tasks that passed at their own step and fail at the end.
2. **Census A's audit**, blinded. The tree is normalized before the auditor sees it, so it
   can't tell the arm:
   - `.docs/workflows/`, `cinch.yml` and `.githooks/` are removed, `AGENTS.md` is replaced with
     arm 2's, and there is no `.git` (an exported tree, so no history or field log).
   - **Arm 1 has no corpus.** The pin's `.docs/` is restored onto its final code ("arm 1,
     restored"). This is the **unmaintained** baseline: the code changed, nobody touched the
     docs.
3. **The audit itself**, as bench/drift ran it:
   - The auditor runs `docs-audit-coverage`, whose text and the precomputed `cinch rules --json`
     come from the grader, since the tree has no cinch. It writes the marker-free mapping before
     it reads any marker, then gives each rule a verdict: covered, tension, under_enforced,
     marker_misplaced, missing or na.
   - An independent **adversarial verifier** re-derives every non-covered row and 8 covered rows
     from the code, with file:line evidence, and may add findings of its own.
   - `AUDIT_MODEL` and `VERIFY_MODEL` are pinned in `PINS` (open decision 7). Trees from all
     arms and replicates of an engine are audited in a seeded shuffled order after that engine's
     last chain.
   - The owner blind-checks a seeded sample of 20 confirmed rows per engine, stratified by
     verdict, without the arm.

### "Accumulated drift" as a number

For tree (a, r), with the audit verified and verifier findings included (as A's headline):

- **Drift count** D_{a,r} = the number of rules in the final corpus with verdict tension,
  under_enforced or missing. That is census A's rate (b). marker_misplaced is in neither count,
  as in A.
- **Pin baseline** D_pin = the same count on the pin tree with the same instrument: the mean of
  two independent audits of the pin.
- **Net introduced drift** I_{a,r} = D_{a,r} − D_pin. A task that fixes old drift by the way
  lowers it, which is legitimate.
- **Drift rate per arm**: Σ_r D_{a,r} / Σ_r (rules in the final corpus of (a, r)), with a Wilson
  95% interval.
  - The interval treats rules as independent, which they aren't within a tree. It is reported
    next to a percentile bootstrap over replicates, and the bootstrap is the one read for
    differences.
  - Rate (a), tension only, is reported the same way.
- **Touched versus untouched.**
  - A rule is **touched** if any of these holds: it is a key rule of any task; its doc owns a
    path changed in the chain; a file with its marker changed; or it is new.
  - D is split into touched and untouched rules.
  - Drift appearing on untouched rules is **instrument noise**, and is reported as such. Its
    rate on the two pin audits and on the reference chain (below) sets the floor.

### Primary contrast

pi: **arm 3 against arm 2, net introduced drift per tree.**

- **Test.** Paired by replicate, one-sided (arm 3 lower), with an exact sign-flip permutation
  over the 6 replicate differences (64 sign patterns), α = 0.05. The difference in mean I and
  its bootstrap interval are reported beside it.
- **Power,** simulated with Poisson counts and a shared per-order difficulty:

  | Arm 2 | Arm 3 | Power |
  |---|---|---|
  | 4 introduced rows per tree | 1 | 0.72 |
  | 4 | 2 | 0.34 |

  At R = 12 the 4 → 2 case rises to about 0.6.
- **Claude.** The same test if R = 6 is approved; at R = 3, estimates and intervals only.

E's primary can only detect large effects. Mostly, E is an estimate.

### Secondary

- **4 vs 2 and 3 vs 4**, same test.
- **2 vs arm 1-restored.** How much maintenance there is at all; this is the scale every other
  number is read on.
- **Retention**: 1 − I_a / I_{arm 1-restored}.
- **Compounding.** Per arm: the observed number of key rules failing the corpus at the end,
  minus the number predicted from C's independent per-task corpus-fail rates for that engine and
  arm (the sum over the six tasks). It is reported overall, on the overlap pairs, and per step,
  against C's rates (position effect).
  - Positive compounding means a miss early in the chain makes later misses likelier. C can't
    see that.
- **Collateral drift**: drift on touched rules outside every task's key. These are rules nobody
  anticipated, which C's keys can't see.
- **End state**: end-state behavior, regressions and broken chains per arm.
- **Inherited blocks** (arm 3), and the per-step transcript measures from C.

### Calibration of E (before `prereg/seqdrift`, no agent model; the audits are paid)

1. **Reference chains.** For each order π_r, the references applied in sequence:
   - every task's hidden tests pass at the end, and from its own step onward;
   - `make check` is green, `cinch check` included.

   A pair of tasks that doesn't commute gets a precedence constraint in `sequence.json`, and the
   Williams rows are redrawn under it.
2. **Naive chain.** The naive references in order π_1, docs untouched: every K group fails as
   designed, and the tree compiles.
3. **Instrument checks** (four audit-plus-verify passes):
   - **The pin, twice.** Reproducibility: agreement of the two non-covered sets, and the match
     with A's verified Budgetron rows (9 of 39, rate (b)).
   - **The reference chain's final tree, once.** Introduced drift on touched rules must be 0 or
     1. More than that means the instrument over-calls, and the auditor prompt is fixed before
     the tag.
   - **The naive chain's final tree, once.** It must flag at least 4 of the change-kind key
     rules as tension, under_enforced or missing. Fewer means the instrument can't see the drift
     E exists to count; it is fixed before the tag, or E doesn't run.

### What a replicate costs

One replicate is 4 chains:

- 24 agent runs;
- 24 per-step gradings, about 24–40 short judge calls;
- 4 final hidden-test runs;
- 4 audits, each an auditor session plus a verifier session.

| | pi/Qwen | Claude Sonnet |
|---|---|---|
| Agent runs per replicate | 24, free; about 6 h of the lab slot at a 15-minute mean (at most 24 × C) | 24 × C's measured mean cost per run |
| Judge per replicate | about 30 Sonnet calls, cents | Qwen, free |
| Audits per replicate | 4 × (auditor + verifier), paid | the same |
| R | 6 (one square) | 6 if approved, else 3 |
| Total | 144 runs (about 36 h), 24 audits, plus 4 calibration audits | 144 or 72 runs, 24 or 12 audits |

**The cost of an audit isn't known yet.** Census A didn't record its model or tokens. The four
calibration audits measure it. Then the per-engine total (24 or 12 audits × the measured cost,
plus the Claude runs) goes to the owner before any counted chain. Claude runs and every audit
are paid, and nothing paid starts without approval.

## What E can and cannot attribute

**What E can attribute, compared with C:**

- **Accumulation and interaction.** C grades each task from a clean pin, so a stale rule never
  meets a later change. E lets it, and the compounding measure compares E's end state with C's
  rates. E's excess over C is attributable to the sequence.
- **Drift outside the keys.** C's keys name the rules the designer foresaw. E's audit covers
  every rule, so collateral drift (a rule the agent made false, or added without a test that pins
  it) is visible only here.
- **Under-enforcement introduced by agents.** An agent that adds or rewrites a rule and marks a
  test that doesn't pin it passes `cinch check` and C's G4. Only an audit sees it. This is the
  one-rule-in-five gap census A found in the field, now measured as it forms.
- **The arm comparison is causal**, as in C: arms share orders and tasks, and differ only in
  provisioning.

**What E cannot attribute:**

- **Which task introduced a drift row.** The audit sees only the final tree. Per-step key grades
  and `git blame` on the rule line give a heuristic attribution, labelled as one. A row on a rule
  two tasks touched belongs to neither for certain.
- **Small differences.** At R = 6, rules within a tree are correlated and the unit is the tree,
  so only large arm differences are detectable. C answers per-change questions with far more
  power. E's role is the estimate and the compounding check.
- **A field drift rate.** The tasks are chosen to break far rules, so E's rate is a stress rate
  for those rules. It is not comparable to census A's 19.7%, which is observational, over months,
  and covers every kind of change. Six changes are not months, and a curated sequence is not a
  roadmap.
- **The instrument is a model.**
  - The auditor and verifier are one family. For Claude-engine trees that is the agent's family
    as well; the owner sample and the arm-blind normalization are the checks, not a cure.
  - The pin and reference-chain audits bound the noise; they don't remove it.
- **Corpus genesis.** E starts from a mature 39-rule corpus. Whether workflows write good rules
  from nothing (density and truth of the residue) is F's question. E sees rule-writing only where
  a task's change needs a new rule.
- **Engine capability against the docs.** As in C, a Qwen chain that breaks the build at step 2
  measures Qwen more than the arm. Broken chains are flagged, and the sensitivity drops them.

---

## Prediction

### Designer's (2026-11-01, the agent that drafted this design, before B's list or any pilot)

**C, upkeep pass pooled over the kept tasks, behavior pass in brackets.** The pilot holds arm 2
near 50% by construction, so the contrasts carry the content.

| Arm | pi/Qwen | Claude Sonnet |
|---|---|---|
| 1 (behavior only) | [40%] | [55%] |
| 2 | 45% [55%] | 50% [70%] |
| 4 | 50% [55%] | 62% [72%] |
| 3 | 47% [53%] | 60% [72%] |

| Contrast | pi/Qwen | Claude Sonnet |
|---|---|---|
| H1, 3 vs 2 | +2 points: not detected | +10: not detected (about a 30% chance of a win) |
| H2, 4 vs 2 | +5 (not tested confirmatorily) | +12 |
| H3, 3 vs 4 | −3 | −2 |
| 4 vs 2, R1 + R2 tasks | +8 | +18 |
| 4 vs 2, R0 tasks | 0 | +3 |
| 2 vs 1, behavior | +15 | +15 |

Reasons:

- **The impact advisory arrives at commit time, after the commit has passed.** The agent has to
  choose to act on stderr from a successful command. Qwen usually commits once and stops; in v1
  only 2 of 30 arm-3 runs ran `impact` themselves. Sonnet reads hook output more often and
  amends.
- **On R0 tasks impact points at the wrong doc or at nothing**, so arms 2–4 should be close
  there.
- **Arm 3's workflow reading costs a local model minutes** and a check that rarely fires buys
  little, so arm 3 ≤ arm 4 on both engines.
- **The docs help behavior on far clauses** (2 vs 1) mostly where an agent that read the doc
  meets the rule while working.

**E, net introduced drift per final tree** (rules, of about 40):

| Arm | pi/Qwen | Claude Sonnet |
|---|---|---|
| 1, restored | 5.0 | 5.0 |
| 2 | 3.0 | 2.0 |
| 4 | 2.7 | 1.5 |
| 3 | 2.8 | 1.6 |

- **E primary (3 vs 2, pi): not detected.**
- **Compounding:** about +0.3 rules per tree in arm 2, and less elsewhere. Most tasks hit
  disjoint clauses, so misses rarely meet.
- **Collateral drift** is mostly under-enforcement: agents' new or reworded rules bound to tests
  that don't pin them, about one per tree in every arm with docs. cinch's check can't see it, so
  arm 3 doesn't reduce it.
- **Regressions:** at most 0.5 tasks per tree.

### The owner's

*(to be added before `prereg/upkeep2-pilot`)*

## Amendments

Every amendment is its own commit, tagged `prereg/upkeep2-amendN` or `prereg/seqdrift-amendN`. It
says what changed, why, and after how many graded runs (or chains) per engine.

- Hidden tests, truth statements and required clauses never change after the first counted run.
  A defect found in one voids that task for that engine (in C), or is reported against every
  chain (in E). The task's results are reported under the original material.
- A rubric change re-judges every candidate. An audit-prompt change re-audits every tree. Results
  are reported under both. Agent runs are never repeated for a grading change.
- v1's amendment-2 rule applies: a change made after per-arm results were seen is a labelled
  sensitivity result for that engine, never its primary.

## Open decisions (owner)

1. **B's shortfall rule.** This design admits census A's under-enforced and misplaced rows as
   tier 3, beside B's normative list. B's text, read literally, sends any shortfall below four
   normative rules to DeltaDocs. Approve tier 3, or require B-only targets and accept a DeltaDocs
   private set, which would be a larger build: Go, Postgres and bench/leak's harness.
2. **Enumeration rules (U6).** Admit side-effect tasks on rules B calls descriptive, such as
   ACCT-004's and XFER-001's lists, as a labelled tier 4? They are the textbook far rule, but the
   code restates them.
3. **Multiplicity.** The fixed sequence H1 → H2 → H3 (as drafted), or H2 first (impact is the
   mechanism under test), or Bonferroni co-primaries H1 and H2 at 0.025 (86% instead of 91%
   power at 25 points).
4. **Arm 4's reminder surface.** Hook output only (as drafted), or an added `AGENTS.md` line about
   `cinch impact` in arms 4 and 3, which breaks arm 3's comparability with v1.
5. **The acknowledgment-gate follow-up** (arm 5) and its trigger as drafted.
6. **The judge for Claude runs:** pi/Qwen (bench/leak's decision), contingent on calibration, and
   the fallback if it fails.
7. **E's auditor and verifier models** (`AUDIT_MODEL`, `VERIFY_MODEL`), and whether the verifier
   should come from another family than the auditor.
8. **Replicates:** pi R = 6 or 12; Claude R = 6 or 3; arm 4 in E or not (dropping it saves a
   quarter of E).
9. **Paid runs.** The Claude pilots, the counted Claude runs at the measured cost, the Sonnet
   judge calls, and every audit in E. Each is approved separately.
10. **Task vetoes** for every sketch, U1–U8 (features Budgetron will never build).
11. **The cinch version at the tag.** If a newer release parses a retire marker, arm 3's
    workflows are re-rendered with it, and the tree hashes are committed, as in bench/leak.
12. **Seeds:** `SCHEDULE_SEED` `20261103`, `ORDER_SEED` `20261104`.
13. **The owner's prediction,** before `prereg/upkeep2-pilot`.
