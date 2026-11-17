# Recoverability census

Plan 29 item B: for each rule, can an agent with the code only recover what it says? The share it
can't is the corpus's normative share. The pre-registration (question, arms, grading, analysis,
prediction, budget) is [DESIGN.md](DESIGN.md); the judge prompt is [rubric.md](rubric.md); the
questions, keys and per-rule predictions are in [questions/](questions/). DeltaDocs is a private
fixture: its questions and results are labeled private (see DESIGN § Corpora and pins).

## RUN

The harness is `package main` in this directory. Build or `go run ./bench/recover <command>`.
`run` is the default command; `judge` and `summary` follow it.

### Pins and order

`PINS` is filled: `BUDGETRON_SHA` (`cinch-bench/upkeep-v1`, `2907c92c…`) and `DELTADOCS_REF`
(`cinch-bench/recover-v1`) / `DELTADOCS_SHA` (`db655362…`). The harness refuses to run when `<REPO>_REF`
in the fixture does not resolve to `<REPO>_SHA`, when the questions file's `commit` is a different
commit, or while a pin is `TODO`. Question files and `PINS` are committed before any run.

Recommended order: **calibrate the judge first** (step 0, before any agent run), then
**Budgetron** (39 rules x 2 arms x 3 reps = 234 runs), judge and summarize
it, then DeltaDocs (88 bound rules x 2 x 3 = 528 runs; 558 with the 5 ignored rules).

Per-repo settings live in `PINS`, keyed by the repo name upper-cased:

| key | Budgetron | DeltaDocs |
| --- | --- | --- |
| `<REPO>_DOCS`, the docs root | `.docs` | `.agent` |
| `<REPO>_GEN`, run once in the base checkout | `make gen` | empty |
| `<REPO>_OBJECTBOX`, lib copied to `.objectbox/lib` | `~/code/budgetron/.objectbox/lib` | empty |
| `<REPO>_STRIP`, removed in both arms | none | `.claude,.cursor,.pi,.pi-lens.json` |
| `<REPO>_AGENTS_DIR`, replacement `AGENTS.md` (`code.md`, `docs.md`), relative to PINS | none (derived) | `private/arms/deltadocs` |

### 0. Calibrate the judge

```
go run ./bench/recover calibrate -judge claude -model claude-sonnet-5-5 \
  -rubric bench/recover/rubric.md -o bench/recover/calibration-claude.csv
```

DESIGN § Judge checks (calibration, before any agent run), through the same code path as `judge`
(rubric fill, answer scrub, verdict parsing, one retry). About 156 paid calls: confirm first. Three
sets, over both questions files and `calibration-probes.json`:

- **(a) own text.** Every rule's text (list number, bold ID and `cinch:ignore` comment stripped) as
  the answer, 132 rules, ignored ones included. At least 97% must be CORRECT, so 129 of 132.
- **(b) abstain.** The fixed answer "I could not establish this from the repository." for 10 rules
  chosen with `SCHEDULE_SEED`. All 10 must be ABSTAIN.
- **(c) probes.** The 14 negation and partial probes of DESIGN's table, transcribed verbatim into
  `calibration-probes.json` (`rule` and `repo` say which question, key and notes each is judged
  against). Each must get its expected verdict.

It prints PASS or FAIL per set, names every miss, and exits non-zero if any set fails or any row has no
verdict. The CSV (`id, set, answer, expected, verdict, justification`, then `judged_answer`,
`judge_error`, `judge_model`) is also the resume state: run it again to retry failures. It holds
DeltaDocs rule text, so keep it private with that repo's results. A failure fixes the rubric before the
first agent run, committed as `prereg/recover-amend0`. `-judge fake` is a plumbing check only: it
passes (a) and (b) and cannot pass (c).

**Result (2026-11-01, `claude-sonnet-5-5`, 156 calls plus 5 re-judged).** First pass: (a) 126/132
(95.5%, FAIL), (b) 10/10, (c) 14/14. Five of the six (a) misses were grading notes that demanded a
clause the rule's own text never states (a stored form named explicitly, a confirmation mechanism
spelled out, the invite-link alternative, the API's soft-delete as the contrast), so a docs-arm
answer quoting the rule verbatim could not reach CORRECT. Their notes were loosened to accept the
rule's own wording, before any agent run, and only those five rows were re-judged: (a) 130/132
(98.5%, PASS). The two misses that stay are reported as known: budgetron CAT-002 (the judge still
wants the stored form named outright) and a DeltaDocs rule whose text is a pointer to another doc
(ABSTAIN on its own text is the judge reading the rubric correctly; the docs arm answers it from
the target doc). The rubric did not change. This is `prereg/recover-amend0`; the CSV is in
`private/` (it holds DeltaDocs rule text).

### 1. Answer

```
go run ./bench/recover run -questions bench/recover/questions/budgetron.json \
  -repo ~/code/budgetron.scrubbed.git -engine pi -arm code,docs -reps 3 \
  -work ~/.cache/cinch-recover -o bench/recover/results-budgetron-pi.csv

go run ./bench/recover run -questions bench/recover/questions/deltadocs.json \
  -repo ~/code/project_deltadocs -flutter "" -engine pi -arm code,docs -reps 3 -include-ignored \
  -work ~/.cache/cinch-recover -o bench/recover/private/results-deltadocs-pi.csv
```

- `-arm` is `code`, `docs` or `code,docs` (default both). The schedule is the full shuffle of every
  question x arm x rep (seed `SCHEDULE_SEED` and the engine name), so a subset (`-arm`, `-ids`) keeps
  the slots it has in the full order. `-dry-run` prints it.
- `-include-ignored` also asks the questions marked `"ignored": true` (DeltaDocs' 5), which DESIGN
  reports as a separate row. It changes the shuffle, so use it for every run of a repo or for none.
  The results mark them in an `ignored` column.
- `-gen`, `-objectbox` and `-ref` override the PINS values. `-flutter` is bound into the sandbox, on
  PATH and as an overlay, only if the directory exists; pass `-flutter ""` for DeltaDocs.
- `-timeout` defaults to `WALL_CLOCK_MINUTES` (10). A timed-out run keeps whatever answer it had
  (usually none) and is judged as it stands. Nothing is retried.
- A run that already has `result.json` under `<work>/runs/<repo>/<engine>/<run_id>/` is skipped, so
  an interrupted `run` resumes; delete the directory to redo one. Verdicts already in the CSV are kept
  when `run` rewrites it.
- pi has one slot, so `-workers` stays 1. pi gets the read-only tool set `read,bash,grep,find,ls`
  (no `edit`/`write`; bench/upkeep gave it those).
- Engines are `pi` and `fake`. `-engine fake -fake-script FILE` runs a bash script in the same
  sandbox in place of a model; its stdout is the answer and `$1` is the prompt.

What each arm is: a fresh copy of the pinned checkout, made a one-commit repository (so `git log`
cannot give back what was removed), with `AGENTS.md` derived from, or replaced for, the repo.

| | code | docs |
| --- | --- | --- |
| docs root (`.docs/`, `.agent/`) | removed | kept, except `<root>/workflows` |
| `cinch.yml`, `.githooks/`, `scripts/cinch-log.sh` | removed | removed |
| `<REPO>_STRIP` paths (DeltaDocs: `.claude/`, `.cursor/`, `.pi/`, `.pi-lens.json`) | removed | removed |
| `AGENTS.md`, Budgetron | "Project docs" section and every sentence naming the docs root dropped | cinch commands dropped, pointer kept |
| `AGENTS.md`, DeltaDocs | `private/arms/deltadocs/code.md` | `private/arms/deltadocs/docs.md` |
| cinch binary | not reachable | not reachable |

Known residue in the code arm, left on purpose (DESIGN § Arms): `// cinch:rule <ID>` comments in
tests, `cinch` in `README.md` and the `Makefile`, DeltaDocs' `TODO.md`.

### 2. Judge

```
go run ./bench/recover judge -results bench/recover/results-budgetron-pi.csv \
  -rubric bench/recover/rubric.md -judge claude -model claude-sonnet-5-5
```

This is the paid step: confirm before the first call.

- Each answer without a verdict gets one `claude -p --output-format json --tools "" ...` call, run in
  an empty temp directory with only project-level settings, so none of the user's CLAUDE.md or memory
  applies. Answers are judged in an order shuffled with `SCHEDULE_SEED` (`-seed`, `-pins`).
- **Blinding.** The judge is shown a scrubbed answer, not the stored one: paths under `.docs/` or
  `.agent/`, any `*.md` path (`AGENTS.md` included) and the whole words doc, docs, documentation,
  documented become `[ref]`. Code-file citations and rule IDs stay. The CSV keeps `answer` as the agent
  wrote it and `judged_answer` as the judge saw it.
- The rubric is filled with `{{question}}`, `{{answer}}`, `{{key}}` and `{{notes}}` and nothing else:
  never the arm, repo, doc or commit. A rubric missing any of the four is refused.
- The reply must contain exactly one of `CORRECT`, `PARTIAL`, `WRONG`, `ABSTAIN` (upper case; the first
  line that opens with it, optionally after `Verdict:`); the rest is the justification. A run with no
  answer is `ABSTAIN` without a call. A failed or unparseable reply is retried once; if it fails again
  the row keeps no verdict and a `judge_error`, and the next `judge` run retries it (DESIGN has the
  owner grade those blind). The file is rewritten after every call, so an interrupted judge resumes.
- `-judge fake` is a deterministic stub for tests: CORRECT iff the answer contains the first 12
  characters of the key's rule text (list number, ID and `cinch:ignore` comment stripped), ABSTAIN for
  calibration's fixed abstain sentence, else WRONG.

### 3. Summarize

```
go run ./bench/recover summary -results bench/recover/results-budgetron-pi.csv -reps 3 -min-correct 2
```

Only `CORRECT` counts; `PARTIAL` does not. Per rule: recoverable = code arm correct in at least
`-min-correct` of `-reps`; valid = docs arm correct in at least that many; normative = valid and not
recoverable; bad question = not valid. A rule short of `-reps` judged runs in an arm is `incomplete`
and counts nowhere. The normative share is normative / valid, with a Wilson 95% interval, then the
same per doc, the normative rule IDs, the bad questions, and how often the author's `predict` agreed.
These cover the bound (non-ignored) rules; ignored rules get their own row, outside the share. It
writes `normative-<repo>.json` next to the results (`-out-dir` to move it).

### Results columns

`run_id, repo, commit, engine, question_id, doc, ignored, arm, rep, position, seed, predict`; how the run
ended: `status` (ok, timeout, error, empty), `timed_out, exit_code, wall_s, engine_error`; usage:
`turns, input_tokens, output_tokens, cache_read, cache_write, tool_calls, cost_usd`; the content:
`question, key, notes, answer, judged_answer`; the judge: `verdict, justification, judge_error, judge_model`. Each
run directory also holds `answer.txt`, `transcript.jsonl`, `stderr.txt` and `result.json`.

### Tests and the dry run

```
go vet ./bench/recover && go test ./bench/recover
RECOVER_E2E=1 go test -v -run Budgetron ./bench/recover   # runs make gen once; ~1 min
```

The tests need bubblewrap and skip without it. The end-to-end dry run uses
`testdata/dryrun-questions.json` (two Budgetron questions, not part of the pre-registered set),
`testdata/PINS.dryrun` and `testdata/fake-answer.sh` against the scrubbed mirror. A fake-engine run of
the real question files and `PINS` checks a repo end to end without a model:
`recover run -questions questions/<repo>.json -repo <repo> -engine fake -fake-script FILE -reps 1 -ids A,B -o /tmp/x.csv`.
