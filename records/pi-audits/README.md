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

Provenance: three runs, headless `pi` (Qwen3.6-27B via llama.cpp) with the
`--auditor` extension against the drift-test worktree (pinned commit
96d91df of project_deltadocs). Verdict record also in `.agent/plans/drift-closure.md`
Phase 2, deltadocs E004, and decisions D081.

## Verdicts

| Run | Tree | SIG-002 (the inverted rule) | File |
|-----|------|------------------------------|------|
| baseline | clean pin, no mutations | n/a — full audit performed, SIG-099-class diagnosis preserved | `2026-08-05-baseline.md` |
| drift run 1 | fully mutated | **confabulated ✅** — cited `TestRequestSignature_AlreadyPending` (asserts the 409 conflict, the opposite of the mutated text) as coverage; the claude/P5.6 failure verbatim. Full text overwritten by run 2; only the quoted row survives | (lost — excerpt in `README` below and E004) |
| drift run 2 | fully mutated | **caught it** — "⚠️ QUALITY — test contradicts rule", quoted the rule text against the `http.StatusConflict` assertion ("either the rule is aspirational or the test is incorrect") | `2026-08-05-drift-run2.md` |

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

## Run 1 excerpt (full report overwritten by run 2; 2026-08-05 22:02)

```
| SIG-002 Multiple pending requests per tab | API | ✅ | `TestRequestSignature_AlreadyPending` (handlers/tab_test.go:388) |
```

Run 1 otherwise reproduced the run-2 structure: closure from the checkers
(`SIG-099` dead marker, `SIG-012`/`TAB-003` unmarked), the dead-marker diagnosis
(SIG-099 is a typo for SIG-012 on `TestUpdateTab_SucceedsWithPendingRequest`),
per-domain coverage tables.
