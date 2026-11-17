# 29. Benchmark program v2: measure the premise, not the checker

**Status:** in-progress

**Tier 4 — evidence. Status: in progress. Supersedes the order in
[21](archive/21-benchmark-suite.md); 21's claims and publishing rules still apply. 2026-11-01: A done
(`bench/drift/`), G done (`bench/field/`), B pre-registered (`prereg/recover`), 25 amended
(`prereg/upkeep-amend2`). D is being designed; C, E and F wait on B.**

## Provenance

**Benchmark and field data, read together.** Stage 1 of the upkeep ablation
([25](25-upkeep-ablation.md), 90 pi/Qwen runs) found no advantage for full cinch over docs plus
an `AGENTS.md` pointer. Read next to everything measured before it, that null is the expected
result, and it says the suite has been measuring the wrong layer.

| Claim | Result | Source |
|---|---|---|
| The checker catches what it says | holds | catch matrix, 44 classes |
| A marker means the test bites | holds, 96% | binding strength, 75 rules |
| It's cheap | holds, 44–56 ms median per commit | field logs, 557 hook runs |
| The tooling helps agents *read* docs | null: corpus + pointer +10 points, tooling +4 with a CI across zero | 48-run ablation |
| A corpus with no pointer | worth zero, same score as no docs | same |
| Workflow prose at read time | null | 24-run ablation |
| A wrong rule | not propagated, never reported | 18-run study |
| The checker helps agents *keep* docs true | null at N = 5, design flawed (below) | 25, stage 1 |

The field logs are the only source that observes instead of predicting: 557 hook runs across
Budgetron, DeltaDocs and lucky_to_have_you, four blocks, three of them stale renders. One real
catch: an agent added `PIC-007` with no marker. Zero bypasses. In the 90 upkeep transcripts,
arm 2 agents found and edited the docs in 27 of 30 runs on the pointer alone; cinch's own
checks (`links`, `rules`, `generated`) never failed an agent's commit in 30 arm-3 runs (the
six arm-3 blocks were format, analyze and commit-pattern rejections, which fire in every arm);
and the one task arm 3 "lost" (C1) is one run inside a task where 8 of 15 runs hit the
30-minute cap in both arms, 4 of 5 in arm 3 and 3 of 5 in arm 2, most after naming the stale
rule in their notes. The write-up is `bench/upkeep/RESULTS-pi.md`.

**Reading:** the corpus plus the pointer is the active ingredient. The mechanical checker is
a cheap, correct floor that rarely needs to fire once a competent agent has been pointed at
the docs. That is not a failure of cinch; it is where the value sits, and nothing has measured
it there.

## The premise as something measurable

cinch claims that docs hold what the code can't tell you, and stay true as the code changes,
so agents and people get those things right. Value is the product of three factors:

- **Density.** The share of the corpus that is the `PERM-003` class: rules the code doesn't
  settle, or misleads on. In the retrieval study 11 of 16 questions were answerable from bare
  source, and the whole effect came from the rest.
- **Truth.** P(a rule is still true | the code it describes changed), accumulated over months.
- **Reach.** Whether the agent reads the rule at the moment it matters. The pointer is proven;
  the commit-time `impact` reminder is unmeasured.

Every benchmark so far measured single-session reach. None measured density or cumulative
truth, which are the premise.

## Flaws in the stage-1 upkeep design, fixed before any rerun

1. **The time cap binds.** A 27B local model in a sandbox can't finish C1 in 30 minutes, in
   any arm, so C1 measured the cap, not the docs.
   Calibrate the cap per task on arm-2 pilots, or condition on finished runs, pre-registered.
2. **Ceiling tasks carry no information.** S2, S3 and C3 scored 100% in both arms. Pilot each
   task in arm 2 and keep only those that pass 20–80% of the time.
3. **Regex keys on prose.** Three false-negative rounds. Grade structure mechanically (rule
   text changed, marked test changed, rule present) and grade meaning with a blinded judge from
   a different model family than the agent, transcripts published, or blind human grading.
4. **The docs impact was obvious.** Removing a color field is plainly a category change, so
   arm 2 caught it unaided. The checker can only show value on *far* rules: the rule lives in
   a doc that doesn't own the changed path, or the change breaks it as a side effect.
5. **Power.** 5 per cell detects a 38-point gap. Fewer, harder tasks at N = 10–15.
6. **No attribution arm.** Arm 3 bundles workflows, `impact` and the check.

## The program

Ordered by information per unit of cost. A and B are instruments; everything after reuses them.

### A. Drift census (field, first)

Run `docs-audit-coverage` with a strong model over the repos that have used cinch for months
(Budgetron, DeltaDocs, lucky_to_have_you; odin-docs if the owner wants it), then blind-verify
every TENSION, MISSING and under-enforced row by hand. Output: the share of rules that are
stale or under-enforced today, in repos where `cinch check` was green throughout.

Pre-registered reading: under 5% confirmed drift, the system keeps docs true and the checker's
green means something; over 20%, green is hollow and the audit is where the work is. The
confirmed rows also become claim #5's seed set, from real drift instead of synthetic seeds.
Private-repo reports stay in `bench/drift/private/`, which is gitignored; the public history
carries their hashes in `bench/private-manifest.txt`, and the files land after the owner clears
them.

**Result (2026-11-01, `bench/drift/README.md`):** 218 rules across the three repos, audited by
one agent per repo and verified adversarially by another. Stale rule text: 10/218 = 4.6%
[2.5, 8.2]. Stale or under-enforced: 43/218 = 19.7% [15.0, 25.5], every per-repo interval
crossing the 20% line. Wrong text is rare; a clause no marked test pins is one rule in five.
Four confirmed drifts are the owner's; six await sign-off. One finding is an authorization bug in
DeltaDocs, with the rule's marked tests green on the paths they cover; details are withheld
until it is fixed (`bench/private-manifest.txt` hashes the withheld files). Neither pre-stated reading fires cleanly; the data says the mechanical check holds the
structure and the audit is where the semantic gap is.

### B. Recoverability census (density)

For every rule, write a question whose correct answer is the rule's content and that can't be
answered from the question itself. A code-only agent (docs removed, no pointer) answers; a
judge from a different model family grades against the rule text. The share of rules *not*
recoverable is the normative share. Questions and keys are committed before any run.

This makes [10](archive/10-kind-normative-vs-descriptive.md)'s `kind:` decision empirical, says which
rules are worth protecting, and picks the far rules for C.

**Status (2026-11-01):** questions, keys, `PINS` and the rubric are committed (`prereg/recover`).
The judge calibration ran on Sonnet and passed at 130/132 after five grading notes were loosened
to the rule's own wording (`prereg/recover-amend0`, rubric unchanged; `bench/recover/README.md`
§ 0 has the two known misses). The free Qwen runs may start; the full judging (about 1,030 paid
calls) still waits on the owner.

### C. Upkeep v2

25 with the six fixes above, both engines, and a fourth arm: docs + pointer + `impact` only.
Tasks target rules B marks as non-recoverable and whose owning doc the change wouldn't lead
an agent to. Pre-registered separately, as 25 was.

### D. Leak replay

[24](24-leak-replay.md), unchanged. The missing-rule step is the one piece of workflow prose
with a sharp, mechanically graded showcase.

### E. Sequential drift

Six tasks in sequence on one tree per arm, then census A on each final tree. It measures
accumulation at a fraction of F's cost.

### F. Corpus genesis

[07](archive/07-workflow-prose-value-and-f1-scope.md)'s expensive half: start from no corpus, N tasks
with and without the workflows, grade the residue with A and B. Free on pi. With A and B
built, its grading is mechanical, which is what made it unaffordable before.

### G. Field log and history replay, published now

The data exists. [19](19-field-log.md)'s labeling over the three logs, deduped by head; the
[20](20-rule-growth-replay.md) walk on Budgetron and DeltaDocs; claim #7 answered from both
directions.

## What the data already says cinch should become

- **`impact` as an acknowledgment gate.** A commit touching owned code without touching the
  owning doc needs a `Rules-checked: <IDs>` trailer. The agent must assert it considered them.
  C's fourth arm measures it.
- **Rule assertions.** A rule may carry a machine check (a grep, a command). The wrong-corpus
  study showed agents route around a false rule silently; nothing in cinch can notice a rule
  is lying.
- **Recognize retirement.** An agent wrote `*(retired)*` by hand in C1; the parser should
  accept a retire marker, and the grader already should have.
- **Scheduled audits.** `cinch audit` scaffolding, calibrated by A.
- **A small agent surface.** Reading the workflow pack costs a local model minutes it doesn't
  have. A one-page agent brief would cost less than `cinch workflows`.

## Considered and dropped

Retrieval tooling (null twice). Typed bindings (binding strength closed it). A purpose-built
fixture (Budgetron works). Human-only studies (too slow, and agents are the audience).

## Immediate decisions

- Qwen stage 2 of 25: stop. The registered result is "not detected at N = 5".
- Sonnet stage 1 of 25: run on the registered design after amendment 2 (S1 key, retirement in
  the grader). Sonnet won't hit the cap, so it answers C1 cleanly, and a two-engine null closes
  claim #3 honestly. The owner decides when it starts.

## Verification / definition of done

- `bench/drift/` holds A's reports and the verified drift rate per repo, with the reading
  applied.
- `bench/recover/` holds B's questions (committed before the runs), harness, raw results and
  the per-repo normative share.
- `bench/field/` holds G's labeled log, the rule-growth CSV and the claim #7 line.
- C, D, E and F each have their own pre-registration tag before any task run.
- The README results table carries every number, null results included.
