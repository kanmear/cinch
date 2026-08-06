# Pi probe reports — first headless local-model audits of the drift-test fixture

Status: **temporary home.** These reports are consumer data (project_deltadocs
rule IDs, test names, paths) — committed here only because the probe results are
history worth keeping and no durable consumer-side home exists yet. Migrate to
the consumer's fixture data (`.agent/drift/`) or an equivalent when one is
agreed; this directory is not part of cinch's spec/contract/plan corpus.

`records/` is in `scanRuleMarkers`' skip list (internal/check/rule.go): the
reports quote `// cinch:rule <ID>` tokens verbatim, and a record surface must
never resolve as source markers — same treatment as `docs/` and
`decisions.jsonl`.

Provenance: five headless `pi` (Qwen3.6-27B via llama.cpp) runs with the
`--auditor` extension against the drift-test worktree of project_deltadocs
(pinned commit 96d91df for the 2026-08-05 runs; 03ec172 — the Phase-4 re-render
commit — for the 2026-08-06 runs, which exercise the blinded check-rules step 3).
The 2026-08-06 C15 baseline round ran against the live deltadocs HEAD. Verdict
record also in `.agent/plans/drift-closure.md` Phase 2/4, `.agent/plans/audit-evidence.md`,
deltadocs E004/E005, and decisions D081/D083/D084.

## C15 round (2026-08-06) — the evidence requirement is mechanical now

With the C15 checker (D084), the report must carry verbatim rule-text and
assertion quotes, and every closure rule must have a row. The round's dual-driver
baseline (claude + pi, one run each on the clean tree, new step-4 format):

- **claude: C15-green** — 75/75 rules, verbatim quotes, committed at deltadocs
  `paths.audit`.
- **pi: C15-red** — the 27B produced an entirely improvised report (no canonical
  per-domain tables, invented domain names, wrong counts); C15's verdict is a
  single mechanical error ("has no coverage rows"), not a judgment call.
  `2026-08-06-c15-baseline-pi.md`.

The pi tier-boundary evidence changes shape: previously a wording-scored
judgment ("paraphrase errors"), now a check result any executor can reproduce.

## Verdicts

| Run | Tree | SIG-002 (the inverted rule) | File |
|-----|------|------------------------------|------|
| baseline (08-05) | clean pin, no mutations | n/a — full audit performed, SIG-099-class diagnosis preserved | `2026-08-05-baseline.md` |
| drift run 1 (08-05) | fully mutated | **confabulated ✅** — cited `TestRequestSignature_AlreadyPending` (asserts the 409 conflict, the opposite of the mutated text) as coverage; the claude/P5.6 failure verbatim. Full text overwritten by run 2; only the quoted row survives | (lost — excerpt in `README` below and E004) |
| drift run 2 (08-05) | fully mutated | **caught it** — "⚠️ QUALITY — test contradicts rule", quoted the rule text against the `http.StatusConflict` assertion ("either the rule is aspirational or the test is incorrect") | `2026-08-05-drift-run2.md` |
| baseline (08-06) | clean pin 03ec172, blinded workflow | n/a — full audit performed (75 rules, per-domain tables), verdicts and test locations correct; rule-text *summaries* are loose paraphrases (SIG-002 labeled "requires edit" — SIG-003's text) | `2026-08-06-baseline.md` |
| drift run 1 (08-06) | fully mutated, blinded workflow | **missed** — ✅ with the wrong test matched by name (`TestCreateSignatureRequest_SnapshotsContentAndPinsHistory` — SIG-004's test, chosen for its name); rule text never engaged. Full text overwritten by run 2; key rows quoted in `README` below | (lost — excerpts below and E005) |
| drift run 2 (08-06) | fully mutated, blinded workflow | **missed** — vacuous row `| SIG-002 | API | handlers/tab_test.go | handler test | ✅ |`; no rule text, no test name | `2026-08-06-drift-run2.md` |

The blinded protocol (Phase 4, D083) turned the claude record around — claude
1/1 green under blinding vs 3/3 confabulated before — and the pi record around
the other way: **pi is 0/2 under blinding vs 1/2 before.** The 27B slips through
a different hole now: the derive pass spreads attention over 75 rules, and the
model skims — matching tests by name and paraphrasing rule text away without
ever restating it against the marked test's assertions. The dual-driver
comparison (D081) is therefore the standing evidence that the semantic row is a
frontier-tier call: the fixture keeps the pi miss visible per round instead of
hiding it in a single-driver verdict. D081's exit bar ("caught by both, not
one") is NOT met by the pi driver; the fork — accept the 27B tier boundary as
documented, or iterate the template's restate-and-flag layer (the weakest of
the four fixes, now evidenced) — is the cinch-side decision recorded in
drift-closure Phase 4's record.

The claude record for comparison: 3/3 runs confabulated a ✅ on the SIG-002 row
(P5.6 + two headless audits; `drift-closure.md` Phase 2). The local model is
therefore 1/2 on the same row — **flaky, not deterministically blind** — which
falsifies "the auditor confabulates a ✅ here" as a fixed property and makes the
Phase 4 blinding the mechanism that turns the catch from run-dependent to
reliable, for both drivers.

## Runner findings (fixed in the consumer fixture, deltadocs `scripts/drift-test.sh`)

- pi writes the complete report and then **never exits** (shutdown stall after
  the final message). The fixture now monitors the report file for growth and
  terminates pi after 120s of post-output quiet, with a 45-minute backstop and
  an orphan-safe EXIT trap.
- The pi semantic gate scores **contradiction wording**, not just MISSING:
  a 27B model flags the drift as "⚠️ QUALITY — test contradicts rule", not as a
  coverage gap.
- **Extension flag conflict (found 2026-08-06 after the Phase-4 pin bump):**
  the pin (03ec172) carries its own `.pi/extensions/auditor.ts`, which pi
  auto-discovers from the worktree cwd — the fixture's explicit `--extension`
  of the repo copy then registered the `--auditor` flag twice and died with a
  flag conflict. The runner now loads the pinned tree's own extension exactly
  once (`--no-extensions -e "$wt/.pi/extensions/auditor.ts"`).

## Drift run 1 excerpts (08-06; full report overwritten by run 2)

The run otherwise reproduced the run-2 structure: closure from the checkers
(`SIG-099` dead marker, `SIG-012`/`TAB-003` unmarked), per-domain tables.
SIG-012 was misdiagnosed as plainly unmarked — the run did not connect the
SIG-099 typo to it (it guessed the marker should read `SIG-001`):

```
| **SIG-002** | Signature request creation | Testable — Model-enforced | (semantic: `TestCreateSignatureRequest_SnapshotsContentAndPinsHistory`) | `models/tab_test.go` |
| **SIG-012** | Signature request cancellation | **UNMARKED** | — | — |
```

## Run 1 excerpt (2026-08-05; full report overwritten by run 2)

```
| SIG-002 Multiple pending requests per tab | API | ✅ | `TestRequestSignature_AlreadyPending` (handlers/tab_test.go:388) |
```

Run 1 otherwise reproduced the run-2 structure: closure from the checkers
(`SIG-099` dead marker, `SIG-012`/`TAB-003` unmarked), the dead-marker diagnosis
(SIG-099 is a typo for SIG-012 on `TestUpdateTab_SucceedsWithPendingRequest`),
per-domain coverage tables.
