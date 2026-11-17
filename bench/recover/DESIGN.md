# Recoverability census: design (pre-registration)

Plan [29](../../.docs/plans/29-benchmark-program-v2.md), item B: the **density** factor of the
premise. This file, the question files, the rubric and `PINS` are committed together, before any
model call, and the commit is tagged `prereg/recover`. Changes after that are amendments (below).

## Question

For each rule in a corpus: can an agent that has the **code only** (docs removed, no `AGENTS.md`
pointer to them) recover what the rule says? The share of rules it cannot recover is the corpus's
**normative share**: the rules whose content lives only in the docs, the ones worth protecting
([10](../../.docs/plans/archive/10-kind-normative-vs-descriptive.md)). The rest are descriptive: they
restate what the code already tells a reader.

The earlier retrieval battery (`~/code/cinch_bench/FINDINGS.md` §2) found 11 of 16 questions
answerable from bare source, with the whole effect in the rest (the `PERM-003` class: a rule
about something the code doesn't settle, or misleads on). That battery was 16 hand-picked
questions graded by negation-blind regex keys (§6). This census asks one question per rule, over
every rule of two corpora, and grades meaning with a judge from another model family.

## Corpora and pins

| | Budgetron (public) | DeltaDocs (private) |
|---|---|---|
| Questions written from | `~/code/budgetron`, `dev` at `0fe3070f39721c57e91e99c250d51fd4ed75e4af` (2026-10-28) | `~/code/project_deltadocs`, `dev` at `db65536270f56882504a30e053a0dcbc18a72e3b` (2026-10-22) |
| Runs use | the scrubbed mirror `~/code/budgetron.scrubbed.git`, tag `cinch-bench/upkeep-v1` → `2907c92cdf886eb269d7bb91573e6c0841a9022a` | a clone of the same commit; the owner tags it `cinch-bench/recover-v1` before the first run |
| `commit` in the questions file | `2907c92…` | `db65536…` |
| Doc root (`paths.docs`) | `.docs/` | `.agent/` |
| Rules | 39 in 8 domain docs, none ignored | 93 in 10 docs, 5 `cinch:ignore`d (GIT-001, TOOL-001, CAT-003, MEMB-006, RT-003) |
| cinch reading the rules | 0.13.2 (`cinch rules --json`) | 0.13.2 |

The two Budgetron commits are the same tree except 11 files whose SMS fixtures were scrubbed
(test data and one doc comment in `lib/models/sms/sms_template.dart`); `.docs/` and every rule are
identical, checked with `git diff --stat 0fe3070 2907c92`. It is the same tag bench/upkeep pins.

**DeltaDocs is private.** `questions/deltadocs.json` holds its rule text verbatim. The owner
decides before the prereg commit whether that file is committed here or kept out of the public
repo, with only its SHA-256 committed in `PINS` as the timestamped evidence of what was
registered. Its results go under `bench/recover/private/` either way, and are labeled private
wherever they are quoted.

## Arms

Both arms get the same prompt, the same pinned tree, the same sandbox and the same tools. Each
workspace is a fresh copy of the pinned tree made a one-commit repository, so history can't give
back what an arm removed.

| | code | docs |
|---|---|---|
| doc root (`.docs/`, `.agent/`) | removed | kept, except `workflows/` |
| `cinch.yml`, `.githooks/`, `scripts/cinch-log.sh` | removed | removed |
| agent tool config (`.claude/`, `.cursor/`, `.pi/`, `.pi-lens.json`; DeltaDocs only) | removed | removed |
| `AGENTS.md` | Project section only, no pointer to docs | docs pointer (without cinch commands) plus Project section |
| `cinch:rule` markers, code comments, `README.md`, `TODO.md` | kept | kept |
| cinch binary | unreachable | unreachable |

- **Budgetron** uses exactly bench/upkeep's arm 1 (code) and arm 2 (docs) provisioning and
  `AGENTS.md` derivation.
- **DeltaDocs** needs its own, because its docs live in `.agent/` and its `AGENTS.md` is a long
  agent guide that restates TOOL-001 and GIT-001 under "Critical Rules". Its `AGENTS.md` is
  replaced, not derived:
  - code arm:

    ```
    # Agent instructions

    ## Project

    DeltaDocs is a full-stack app in a monorepo: a Go/PostgreSQL backend in `backend/` and a SvelteKit/DaisyUI frontend (Bun, Vite) in `frontend/`.
    ```
  - docs arm: the same, with this section before `## Project`:

    ```
    ## Project docs

    This repo's operational docs live in `.agent/`. Read them before changing behavior they describe.

    - Business rules are numbered items with a bold ID like **AUTH-001**. The test that enforces a rule carries a `cinch:rule <ID>` comment.
    - Changing a behavior a rule describes means updating the rule and its test together. A new invariant a reader couldn't recover from the code gets a new rule.
    ```

  `.claude/` goes in both arms because it holds an agent definition and session hooks that inject
  `AGENTS.md`; its `check-git-setup.sh` is the one piece of code that warns about GIT-001, so
  GIT-001's code-arm result is a little more pessimistic than a plain "docs removed" would be.
- **The code arm is a repo that lost its docs, not one that never had them.** Comments that cite
  rule IDs (Budgetron's `lib/` and `test/` mention a rule ID 339 times, markers included, and many
  `lib/` comments paraphrase the rule), test names and markers stay; so do Budgetron's README (which describes the import backup and SMS dedup) and
  DeltaDocs' `TODO.md`. All of this biases toward *recoverable*, so the measured normative share is
  a **lower bound** for these corpora (as in FINDINGS §6).
- The docs arm is the **validity control**: an agent pointed at the docs that still can't give the
  rule's content means the question is bad, not that the rule is normative.

## Engine and judge

- **Answering agent:** pi with the local Qwen model (free), the stack pinned in `PINS`
  (`PI_VERSION`, `PI_MODEL`, `PI_THINKING`, as bench/upkeep). It runs in the bench/upkeep
  bubblewrap sandbox: the workspace at `/work`, an empty home, no `~/code`, no question files,
  keys or harness. Tools are `read, bash, grep, find, ls`; no `edit` or `write`. One session per
  question, arm and rep, so no answer can lean on another. Wall clock is capped at 10 minutes.
- **Prompt:** the question, a blank line, then the harness's fixed `answerSuffix` (in
  `engine.go`, verbatim at the prereg commit). Nothing in it names docs, rules or the arm.
- **Judge:** Claude Sonnet (`JUDGE_MODEL` in `PINS`) via `claude -p`, a different model family
  from the agent. Each call runs in an empty directory with no tools, so it can't read the repo.
  It sees exactly four things, filled into [rubric.md](rubric.md): the question, the answer, the
  key (rule text verbatim from `cinch rules --json`) and the grading notes. Never the arm, repo,
  doc path or prediction.
- **Blinding the answer.** A docs-arm answer may cite `.docs/domain/x.md` or say "the docs say",
  which would tell the judge the arm. Before judging, the harness replaces in the answer: any path
  under `.docs/` or `.agent/`, any `*.md` path, `AGENTS.md`, and the whole words *doc*, *docs*,
  *documentation*, *documented* (case-insensitive) with `[ref]`. Code-file citations stay (both
  arms produce them). Rule IDs stay, since the code arm sees them in markers too. The judge is
  told to ignore citations.
- **Judge order:** every answer, both arms and both repos, shuffled with `SCHEDULE_SEED` before
  judging. One verdict per answer.

## Question-writing rules (applied to every question)

1. Phrased as a developer new to the repo would ask before changing that area.
2. It must not contain the answer or paraphrase the rule's fixed quantities, actors or outcomes
   ("how often may a background SMS scan run?", never "is it once a minute?").
3. It must be answerable from the repo with the docs present.
4. It names enough context (feature area, entity, endpoint family) that the agent knows where to
   look.
5. No yes/no questions.
6. One question tests one rule. Where a rule has several clauses, it asks about the one a code
   reader is most likely to get wrong, and the notes say so ("Focus").
7. No rule IDs and no doc paths. A rule ID would let the code arm grep the `cinch:rule` marker
   straight to the bound test, which a developer without docs couldn't do; a doc path leaks the
   docs' existence.
8. The notes state what CORRECT, PARTIAL and WRONG mean for that question, which clauses are
   required, and which are "not required". Negation traps are named where the rule's content is
   a negative (SIG-001, SIG-017, PERM-003, BUDGET-008).

Rules that overlap get questions from different entry points so each tests its own content:
MEMB-013 (Members API) and PERM-007 (Permissions API) ask about the confirm gate, SIG-020 about
the stranded request and its recovery; PROJ-003 (owned projects deleted), PROJ-009 (hard versus
soft delete) and MEMB-008 (memberships). Two rules are pointers whose content is another rule's:
CAT-005 (= TAB-002) and PROJ-009 (restates PROJ-003 against PROJ-007). They're asked and counted
like any other, and listed separately in the report.

Questions, keys, notes and a per-rule prediction are in
[questions/budgetron.json](questions/budgetron.json) and
[questions/deltadocs.json](questions/deltadocs.json), one entry per rule, ignored rules included
and marked `"ignored": true`.

## Grading

The judge gives one of four verdicts per answer ([rubric.md](rubric.md)):

- **CORRECT**: states the rule's substantive content as the notes require, including any
  quantities, actors and conditions the rule fixes.
- **PARTIAL**: right direction, misses a fixed quantity, actor or condition.
- **WRONG**: contradicts the rule or invents a behavior it rules out.
- **ABSTAIN**: says it can't tell, or doesn't answer.

A run that timed out or ended without an answer is ABSTAIN with no judge call. An unparseable
judge reply is retried once; a second failure is graded blind by the owner (answer, key and
notes only) and listed.

## Per-rule outcome

Three reps per rule and arm. Only CORRECT counts.

- **recoverable**: the code arm is CORRECT in at least 2 of 3 reps.
- **valid**: the docs arm is CORRECT in at least 2 of 3 reps.
- **normative**: valid and not recoverable.
- **descriptive**: valid and recoverable.
- **bad question**: not valid, whatever the code arm did. A bad question is never rewritten and
  rerun inside this census; it is reported and left out of the share's denominator.
- A bad question whose code arm *is* recoverable is listed separately as **inverted**: the code
  told the agent the rule and the docs didn't, which suggests a stale or misleading rule. Those
  go to the drift census (A) as candidates.

## Analysis (pre-stated)

- **Primary, per repo:** normative share = normative / valid over the **bound** (non-ignored)
  rules, with a Wilson 95% interval. Budgetron and DeltaDocs are reported separately, never
  pooled; DeltaDocs is labeled private.
- **Ignored rules** (DeltaDocs' 5) are asked like the rest and reported as a separate row. They
  are kept out of the primary share because cinch doesn't bind them and upkeep v2 can't target
  them; including them is the sensitivity row.
- **By doc:** normative / valid per rule doc, both repos.
- **The output artifact:** `normative-<repo>.json` (and a CSV with every rule's six verdicts and
  its outcome): the list of normative bound rules, which is the candidate pool for upkeep v2's
  task selection (plan 29 C: tasks target rules marked normative whose owning doc the change
  wouldn't lead an agent to).
- **Secondary:**
  - *lenient share*: recoverable counted as CORRECT-or-PARTIAL in at least 2 of 3. The gap
    between the two shares says how much of "normative" is a missed detail rather than a missed
    rule.
  - *how a normative rule fails in the code arm*: **misleading** when at least 2 of 3 code-arm
    verdicts are WRONG (the code points the other way: the `PERM-003` class proper), **silent**
    when they are ABSTAIN, **partial** when PARTIAL, mixed otherwise.
  - *prediction*: per-rule `predict` against the outcome (a 2×2), and the predicted share
    against the measured one.
  - *stability*: the share of rules whose three code-arm verdicts agree.
  - verdict distribution per arm; turns, tokens and wall clock per arm; timeouts per arm.
- **Judge checks** (reported, not used to change verdicts unless an amendment says so):
  - **calibration, before any agent run.** (a) Every rule's own text (list number, `**ID**` and
    `cinch:ignore` comment stripped) as the answer: must be CORRECT for at least 97% of 132.
    (b) The fixed answer "I could not establish this from the repository." for 10 rules: must be
    ABSTAIN 10 of 10. (c) The negation and partial probes below: each must get its expected
    verdict. A failure here fixes the rubric before the first agent run, committed as
    `prereg/recover-amend0`.
  - **self-agreement**: a random 10% of answers (seeded) judged a second time; agreement on
    CORRECT versus not is reported.
  - **human audit**: the owner grades a stratified random sample of 60 answers blind (answer, key,
    notes; no arm, no judge verdict). If agreement with the judge on CORRECT versus not is below
    85%, the census is reported as judge-unreliable and the rubric is revised under an amendment
    that re-judges every answer. Agent runs are never repeated for a judge change.

| Probe | Answer given to the judge | Expected |
|---|---|---|
| SIG-017 | "A signer who loses eligibility is not removed and the request is never auto-cancelled; they just fail the permission check when they respond, and only the editor cancelling unblocks it." | CORRECT |
| SIG-017 | "A signer who loses eligibility is automatically removed from the request, and if no signers remain it is auto-cancelled." | WRONG |
| SIG-001 | "Editing is never locked by a signature request; the tab stays editable while a request is open." | CORRECT |
| BUDGET-008 | "Computing budget summaries never writes anything, however long since the last visit." | CORRECT |
| MEMB-007 | "Removing a member deletes only the membership row; nothing they created is deleted." | CORRECT |
| PERM-003 | "If no entry exists the role has the default permission, `none`." | WRONG |
| CAT-001 | "Names are unique ignoring case, so `Food` and `food` collide and the rename throws `DuplicateCategoryName`." | WRONG |
| ENTRY-002 | "Date-range filters include the start date and exclude the end date." | WRONG |
| BUDGET-007 | "The interrupted period is kept as a partial period and evaluated against its target prorated to its shortened length." | WRONG |
| SIG-019 | "Once no signer is still pending the request resolves accepted, even if one signer declined." | WRONG |
| XFER-006 | "Import keeps the pending SMS entries and resets the SMS scan point." | WRONG |
| PROJ-007 | "Deleting a project removes its row from the database, and its reads return 404." | WRONG |
| BUDGET-001 | "Weeks start on Monday and months on the 1st." | PARTIAL |
| SMS-006 | "A background scan runs only if the permission is already granted and a scan point exists, never prompts, and scans never overlap." | PARTIAL |

## Prediction (2026-11-01, before any run)

- **Budgetron: 8% normative (3 of 39 bound rules).** Nearly every Budgetron rule is restated in
  a code comment that cites its ID next to a test named after it, so only the long multi-clause
  rules (BUDGET-007, ENTRY-006, SMS-003's re-application clause) should defeat a 27B model inside
  one session.
- **DeltaDocs: 10% normative (9 of 93; 6 of 88 bound rules, 7%).** The predicted normative rules
  are process conventions with no code (GIT-001, TOOL-001), meaning and absence claims (SIG-014's
  "not a quorum", SIG-006's "never a rank", SIG-020's "no recovery path", RT-003's "why"), the
  classic `PERM-003`, and two long rules (SIG-015, MEMB-024). The code has gained explanatory
  comments since the retrieval battery (the seed scripts now explain PERM-003), so I expect fewer
  misses than its 5 of 16.
- Both predictions treat a capability miss as "not recoverable", which is how the census counts
  it. If the measured shares come out well above these, the first suspect is engine capability on
  long rules, not density: the code arm's failures will cluster on rules whose keys run past
  three clauses.

The per-rule predictions are the `predict` fields.

## Reading (pre-stated)

- **Density.** If the normative share's upper Wilson bound is under 15% in both repos, the
  density factor is small: most of each corpus restates code, cinch's protective value sits on a
  handful of rules, and the README says so in those words. If the lower bound is over 30% in
  either repo, that corpus carries substantial content the code can't give back.
- **Where `kind:` lives (plan 10).** If every rule doc is either at most 20% or at least 80%
  normative, a per-doc `kind:` would be right for nearly every rule and per-doc is enough. If any
  doc with 5 or more valid rules sits between 20% and 80%, `kind:` has to be per rule.
- **Upkeep v2 (plan 29 C).** C draws its tasks from the normative bound Budgetron rules. If there
  are fewer than four, C records the shortfall in its own pre-registration and takes the rest
  from DeltaDocs' list as its private arm; it doesn't relabel descriptive rules.
- **Bad questions.** Over 10% bad questions in a repo means the docs arm (or the question set) is
  unreliable there, and that repo's share is reported with that caveat in its headline.

## Budget

| | Budgetron | DeltaDocs | Total |
|---|---|---|---|
| Rules (bound + ignored) | 39 + 0 | 88 + 5 | 132 |
| pi sessions (rules × 2 arms × 3 reps) | 234 | 558 | 792 |
| Judge calls on answers | ≤ 234 | ≤ 558 | ≤ 792 |
| Judge calibration + 10% re-judge | | | about 160 + 80 |

- **pi time.** Serial on the lab's one slot. At an assumed 2–4 minutes per read-only session the
  run takes 26–53 hours; the 10-minute cap bounds it at 132 hours. Free.
- **Judge cost.** About 1,030 `claude -p` Sonnet calls of a few thousand input tokens each:
  roughly $10–25. **This is a paid run**: the owner confirms before the first judge call
  (calibration included).
- **Run order:** every question × arm × rep of a repo, shuffled with `SCHEDULE_SEED` (`20261101`)
  and the engine name. A time-varying condition can't line up with an arm.

## Validity of a run

A run is invalid, logged and rerun once if its transcript touches any path outside `/work` (or
the sandbox's own home), or if the engine fails before its first tool call. A timeout is graded
as it stands (usually ABSTAIN), never retried. Invalid runs are listed, never silently dropped.

## Limits

- **Engine-relative.** "Recoverable" means *this* agent (a local 27B model with 10 minutes and
  read-only tools) recovered it. A stronger agent would recover more, so the normative share is
  an upper bound in that direction, while the kept comments and markers make it a lower bound in
  the other. A Claude code arm on the normative rules alone is the cheap follow-up that separates
  density from capability.
- **One clause per rule.** A multi-clause rule is tested on its focus clause; another clause
  might be recoverable or not independently.
- **One author.** The same person wrote the rules, the questions, the notes and the prediction.
  The commit timestamp is the evidence the questions weren't fitted to the result; the human audit
  is the check on the notes.
- **One judge family.** Sonnet grades Qwen, so the judge shares no family with the agent, but its
  reading of the notes is a single instrument, which is why calibration and the audit
  are pre-stated.

## Amendments

Every change after `prereg/recover` is its own commit, tagged `prereg/recover-amendN`, saying
what changed, why, and after how many judged answers. Questions and keys never change once any
agent run has started; a defective question is reported as one. Rubric or notes changes re-judge
every answer, and the report gives the result under both the amended and the registered rubric.
