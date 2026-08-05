# Drift closure — C14, a blinded auditor, and the drift-test fixture

Status: planned

Purpose: the P5.6 adversarial drift test (ten mutations injected one per commit into a scratch
branch of project_deltadocs, scored by one headless auditor run, then reverted) proved the
mechanical layer honest and scoped — 9/9 rows caught by exactly their named checker, no
overreach — and found the semantic pass fails by **confabulation, not oversight**: it read the
inverted rule, read the marked test, quoted the contradicting assertion, and invented a rule
fragment to keep the ✅ (the SIG-002 row). This plan mechanizes the one remaining drift class
(C14), blinds the semantic pass so it cannot preserve a prior, and turns the mutation set into
the harness's regression suite. Plus one incident the test exposed: C10's boundary is a single
git moment, and it should either widen or say so.

Executed in five phases, one side per phase ([project] / [cinch] / [both]); stop at every gate —
the user reviews and commits between phases, no chaining.

## Context — why the shape

**Task-framing failure, not capability failure.** The same audit, on mutation 1, correctly
distinguished a mistyped marker from a missing test — a subtler call than the contradiction it
missed. Reading comprehension was fine; the failure is the frame, which is why "add an
instruction to think harder" is the wrong fix.

**The leading question.** check-rules step 3 asks *here is a rule, here is its marked test, is
it covered?* — expected answer yes. Every green ✅ before the inverted rule reinforced the prior;
asking a model to verify a link that already exists produces verification. The restate-the-rule-
and-flag-tension instruction is still worth keeping — as the weakest of the four fixes, because
it asks the model to catch itself. The other three don't.

## Phase 1 — Sync project_deltadocs `.agent` after the reorg *(landed 2026-08-05 — no commit: the premise was wrong)*

deltadocs last landed 2026-08-05 00:18 (the P5.6 commit 96d91df); the reorg (D074–D080) landed
after it, and its Phase 4 dedup changed `templates/doc-philosophy.md`. The consumer's rendered
workflows are therefore stale — C2 red — and the fixture pins a deltadocs commit, so a stale
baseline would poison the expectation table's known-warn rows before the fixture exists.

- Re-render the consumer corpus, C2 green, `make check-harness` exit 0 at the known baseline
  warns (the two designed C6/C7), commit.
- Check for anything else the reorg left stale in the consumer's `.agent` while there.

**Exit:** deltadocs HEAD C2-green; `make check-harness` exit 0 at the baseline; diff reviewed by
the user.

**Record.** The staleness premise did not hold: D080 (the Phase 4 dedup) resolved to keep
`templates/doc-philosophy.md` unchanged — "no re-render, C2 untouched, consumers re-render on
their own schedule" — and `git diff 616ea61..HEAD -- templates/ scaffold/` is empty. Verified
empirically against a fresh `make build`: `make render` + `make docs-index` produced **zero
diff** on the consumer (C1/C2 green by construction), `make check-harness` exits 0 with exactly
the two designed baseline warns, and no stale artifacts remain in the consumer's `.agent` (no C13
refs, no deleted-docs refs, manifest is a superset of the scaffold contract). Exit met with an
empty diff; nothing to commit.

## Phase 2 — Restore the mutation set as `make drift-test` *(landed 2026-08-05 — fixture in project_deltadocs)*

Resolves the open "Home decided at implementation": mutation patches, scorer, and expectation
table are consumer data (deltadocs paths, SIG-002-class rule IDs), so per the boundary rule they
belong in project_deltadocs, not cinch. The runner: build a throwaway `git worktree` of the real
consumer at the pinned commit (Phase 1's HEAD — D061: no toy fixtures), apply the mutations, run
the check binary, score against the expectation table, tear the worktree down.

- **Restore, not rebuild** (verified 2026-08-05): `scratch/checker-drift-test` is reflog-reachable
  from the P5.6 commit 96d91df, with nine committed mutations in 77d5e60..a83e4e1; the tenth row
  (C10) was applied uncommitted by construction. Recover each commit's diff against its parent as
  the patch file; the C10 row becomes an apply-uncommitted step.
- Mutations as patches, one per row; the C10 row applied *uncommitted* by construction (its window
  is `git diff HEAD`); the C1 row committed with hooks bypassed (`-c hooksPath=/dev/null`) — the
  hook's own catch is part of the C1 row's record, not a failure.
- The scoring table is data: per row, the primary expected checker, the explicit negative (the
  semantic row), and the collateral findings (C11 ×3 on the C1 row, C2-stale-check-rules on the
  C4 row), so a regression in a *secondary* checker also fails the run.
- Mechanical rows fully scripted. The semantic row needs a model: an optional gate that invokes
  headless claude with the auditor agent and asserts the SIG-002-class drift is reported. Until
  Phase 4 lands, that gate is expected to fail — which is the point: the fixture makes C14 and
  the blinded auditor testable in an afternoon.
- Rebuild the expectation table from the recovered diffs and this plan's P5.6 record (9/9 rows
  caught by exactly their named checker; SIG-002 the one miss).

**Exit:** `make drift-test` mechanical rows reproduce the P5.6 score table on today's binary —
the regression proof for the next person who touches `check.go`; semantic gate red-by-design,
documented.

**Record.** All exit criteria met, verified on today's binary (2026-08-05): `make drift-test`
exit 0 — 16 findings, exact-match scored, every row's named checker with counts, zero overreach;
`make drift-test-semantic` mechanical PASS + semantic gate FAIL exit 1 (red by design, report at
`/tmp/drift-test-semantic-report.md`); `make check-harness` stays exit 0 with the two designed
warns.

- **Home, decided at implementation: `.agent/drift/`, not `scripts/`.** First attempt at
  `scripts/drift/` poisoned the consumer's own C8 — `scanRuleMarkers` walks the whole repo and
  the patch files *contain* the mutation's `// cinch:rule` lines (SIG-099 phantom: "does not
  resolve to a rule"), plus the removed SIG-012/TAB-003 marker lines would mask real unmarked
  warns. `.agent/` is in the marker scan's skip list and C1/C11 read only `.md`, so the fixture
  is invisible to every checker there. Decision entry still lands at Phase 5.
- **Recovery:** nine patches extracted byte-faithful from the reflog-reachable commits
  (77d5e60..a83e4e1, `git show <c> --format=`); row 7 (C10) reconstructed — the original
  working-tree edit died with the worktree (never committed, no dangling blob), same shape:
  one-line behaviour change in `backend/handlers/signature_events.go` (owned by exactly one
  domain → exactly one C10 warn), rule text left.
- **Apply mechanics:** committed in the P5.6 *historical* commit order, not score-table row
  order (the score table's row order differs from commit order; row 8's `index.md` hunk was
  generated pre-rename, so it must land before row 5's doc move). Row 5 (C1) commits with
  `-c core.hooksPath=/dev/null` — the pre-commit index guard rightly rejects it, and that catch
  is part of the row's record; note the `core.` section is required (bare `-c hooksPath=` fails
  git's -c parsing). Rows 1/2 exercise the consumer's own pre-commit hook (`make check-backend`
  passes through cleanly). Row 7 applies uncommitted, after all commits.
- **Scoring:** one end-state `$CINCH check` run (the P5.6 audit shape), exact-match on
  (checker, severity, message substring, count) — every finding must be expected, every expected
  finding present. Row 9 is the explicit negative (nothing structural may fire on the SIG-002
  inversion). `check-rules.md` staleness is dual-attributed (rows 4 and 6 both stale it; the
  end-state run can't decompose — row 4's unique signal is task-primitive.md, row 6's is C4);
  the attribution note lives in expectations.json.
- **Incident — git 2.55.0 quirk:** the worktree add/remove cycle flips `core.bare=true` in the
  shared config, leaving the main checkout unworkable ("this operation must be run in a work
  tree"). Reproduced live (fires on every fixture run; did not reproduce in scratch repos — the
  trigger state is something in this repo's gitdir). The runner's cleanup now verifies
  `core.bare` after teardown and restores it with a WARNING; the consumer was recovered
  (`core.bare=false`).
- **Semantic gate, red by design — confirmed 3/3 failures.** Two headless audits today, both
  marked SIG-002 ✅: run 1 deferred the contradiction to a "note below" that never appeared in
  the report; run 2 cited the contradicting test (`TestRequestSignature_AlreadyPending`, which
  asserts the 409 conflict) as coverage outright. The P5.6 original makes it three for three —
  the confabulation is not a fluke of one session. Both today's runs preserved the SIG-099-class
  diagnosis (SIG-012 reported "mismarked — marker reads SIG-099, a typo; test itself is
  correct"), the Phase 4 capability baseline.
- **Dual-driver semantic gate — the pi probe (2026-08-05, first local-model audit).** The gate
  now runs against a second driver: headless pi (Qwen3.6-27B via llama.cpp) with a pi-native
  `--auditor` extension (persona + manifest-derived bash allowance, the D067/D069 envelope
  pattern bound to a third harness; consumer glue in project_deltadocs `.pi/extensions/auditor.ts`,
  fixture modes `--semantic-pi` / `--pi-baseline` in `scripts/drift-test.sh`). Baseline probe on
  the clean pin: the 27B performs the full audit (closure from the checkers, per-domain report,
  SIG-099-class diagnosis preserved). Drift probe, two runs: run 1 confabulated SIG-002 ✅ citing
  `TestRequestSignature_AlreadyPending` — the claude failure verbatim; run 2 flagged it
  "⚠️ QUALITY — test contradicts rule", quoting the mutated rule text against the
  `http.StatusConflict` assertion. Net: **claude 3/3, pi 1/2** — "the auditor confabulates a ✅"
  is model-run-dependent, not a fixed property, so the Phase 4 blinding is what makes the catch
  reliable instead of lucky, and the blinded protocol must be verified against **both** drivers
  (the fixture's dual-driver gate is the standing policy; D081). Runner findings: pi never exits
  after writing its report (shutdown stall — the fixture terminates it after 120s of post-output
  quiet, 45-min backstop, orphan-safe trap); the pi gate scores contradiction wording, not just
  MISSING. Reports: `records/pi-audits/` (temporary home; also `~/Documents/cinch-first-audits`).
- **Forward notes:** the fixture needs the go toolchain (rows 1/2 run the consumer's hook);
  `scripts/drift-test.sh` needs `claude` for `--semantic` and `pi` for `--semantic-pi` /
  `--pi-baseline` (each skips with a message when absent); the pin lives in `.agent/drift/pinned-commit`
  (96d91df full hash) and must be bumped deliberately when deltadocs advances (the fixture fails
  loudly if the pin goes unreachable); the semantic gate is dual-driver by policy (D081) — both
  drivers run per audit round and their verdicts are compared, not averaged.

## Phase 3 — C10/C14 boundary decision, then C14 [cinch] *(landed 2026-08-05 — D082, two-window C14, fixture row 9 re-scored)*

**Decision first** (one entry; read D011/D065 before writing it; applied to C10 and C14
together): C10 only fires pre-commit — its window is `git diff HEAD`, so a committed change is
invisible — and mutation 5 went through with `--no-verify`. "Warns on behaviour change without
doc update" reads like a property of the system rather than of one git moment. Either widen the
boundary to a commit range (needs a ref input; reconcile with D011's no-commit-based-threshold
stance) or state plainly in the code comment and roadmap that it is a hook-only check whose
working-tree window is deliberate — it self-corrects when code and doc land in the same change
(D065).

Then **C14** in `diff.go`, mirroring C10: warn when a commit changes a rule's text and the file
carrying that rule's marker does not. Lateral under D009 — it binds an authored rule text to its
marked test, validating no doc's copy of a machine-readable fact.

- The same twenty-line shape as C10: parse which numbered items changed in `domain/<d>.md`
  (existing `parseDomainDoc`), resolve their marker files (existing `scanRuleMarkers`, extended
  to record *where* each marker lives), intersect with `gitChangedPaths`. Core never parses
  project source — nothing new to parse.
- Honest scope: it does not catch a rule wrong from birth; it catches *drift*, the realistic
  failure — rules rot as behaviour changes, they rarely start false.
- Warning, not error, same philosophy as C10: a rule-text clarification with no test change is
  legitimate; the value is asking the question when the answer is cheapest.
- The semantic row changes meaning once C14 exists: the drift *signal* becomes mechanical; the
  semantic pass keeps the content question — does the test's assertion actually enforce the
  rule's *meaning*? Encode this split in the fixture's expectation table (Phase 2 data).

**Exit:** `go vet ./...` and `go test ./...` green; `cinch check` green on cinch's own repo and
on project_deltadocs; `make drift-test` all mechanical rows green including the C14 row.

**Record.** All exit criteria met, verified on today's binary (2026-08-05): `go vet ./...` +
`go test ./...` green with 15 new C14 cases; cinch self-check exit 0 at the known baseline;
deltadocs `make check-harness` exit 0 with exactly the two designed warns — **no C14 noise on
the clean pin** (the P4.1 migration commit 20abbe6 authored the IDs fresh, so no text-change
detections at rest; cinch's e10555f changed only the ignored rule); `make drift-test` exit 0,
exact-match PASS with row 9 re-scored: the SIG-002 inversion now fires exactly one C14 warn and
nothing else structural.

- **The decision, D082, first:** C10 stays working-tree-only — hook-only by design, stated
  plainly in diff.go's comment and the roadmap (widening would warn permanently on test-only
  commits under owns: paths and re-baseline the fixture's rows 1/2); C14 gets two ref-free
  windows — the working tree (C10's) and the **last commit that touched the rule's doc**
  (`git log -1 -- <doc>`), the only window that sees the fixture's row 9 at the end-state
  audit, since row 9 is committed mid-history (`git diff HEAD` and `git diff HEAD^ HEAD` both
  miss it). Persistent warn by choice: a committed rule-text change without its marker stays
  flagged until the doc is touched again — the falsifying commit is the fact. D011 reconciled:
  one deterministic commit, not a threshold.
- **C14** in `diff.go`: `checkRuleDiffCoupling` plus three small git helpers (gitShow /
  gitLogLast / gitDiffPaths, 10s timeouts, same shape as gitChangedPaths); `scanRuleMarkers`
  extended to record where each marker lives (map[string]string) so the marker file is
  nameable; marker-less and cinch:ignore rules drop out (C8's domain); a rule newly given an
  ID is not a text change (C8 owns the closure); renames self-correct (marker or doc renamed
  in the same commit is silent).
- **Fixture-data self-poison, caught by cinch's own self-check:** the first fixture text put
  literal `// cinch:rule SIG-002` tokens in diff_test.go, which scanRuleMarkers reads as real
  markers → C8 errors on cinch's own repo. Fixtures now build markers at runtime (the mkMarker
  convention from rule_test.go) — the same poison class as the P5.6 patch files, one level
  closer to home.
- Both C14 windows probed live on the pinned deltadocs tree: "changed in the working tree"
  and "changed in commit \<sha\> but its marker file backend/tests/handlers/tab_test.go did
  not" — message names the rule, doc, marker file, and window.

## Phase 4 — Blind the semantic pass, then the full fixture run [both]

Rewrite `templates/check-rules.md` step 3 (Fix 2, plus Fix 3's framing and Fix 4's restate-and-
flag step): give the auditor the rule set and ask *which test enforces this?* — or the test set
and ask *what does this enforce?* — then diff its answers against the recorded marker mapping.
Disagreements surface as conflicts rather than requiring the model to volunteer doubt; the model
never sees the expected mapping, so there is no prior to preserve and no link to verify.

- Frame it as a search with nonzero expected yield: "find rules whose tests do not enforce them"
  beats "confirm each rule is enforced".
- Keep the restate-and-flag instruction as the weakest layer: for every marked test, restate the
  rule and the test's assertion, flag any tension.
- Land in the template, re-render into both consumers, C2 green. Verify live in both consumers'
  envelopes (the E017 interactive path), not just headlessly.
- The SIG-099 diagnosis is the capability baseline: the auditor's correct call — a mistyped
  marker, not a missing test — is the reading-comprehension bar the new protocol must preserve;
  if the blinded protocol regresses it, the fixture's semantic gate will say so.

Then the **full fixture run**: with C14 and the blinded protocol, the semantic row must now be
caught. If it is still missed, the loop still has a hole at its most important point — the
fixture is what proves either way.

**Exit:** `make drift-test` fully green including the semantic row; SIG-099-class call preserved;
both consumers' rendered check-rules.md C2-green; one live auditor run per envelope.

## Phase 5 — Close [both]

Log decisions and events (the fixture-home decision, the C10/C14 boundary decision, the C14
entry), refresh the roadmap status line, close this plan per the completion ritual (feature
plans persist with `Status: complete`).

## Verification

- Phase 2: fixture reproduces the P5.6 score table on today's binary, before C14 lands.
- Phase 3: `go vet ./...` and `go test ./...`; `cinch check` green on cinch's own repo and on
  project_deltadocs.
- Phase 4: both consumers' rendered check-rules.md C2-green; one live auditor run per envelope
  (the E017 interactive path); C14 row: rule-text change → C14 warn + semantic gate fires;
  SIG-099-class call preserved.

## References

- The P6 verdict this plan supersedes: `docs/HANDOFF.md` as of commit 7ae3aa8 (deleted in the
  reorg, D074 — git history is the archive; the P5.6 score table is also reproduced as data in
  project_deltadocs `.agent/drift/expectations.json`)
- E016/E017 — auditor envelope workouts; D065 — C10; D067 — the checker closure is authoritative;
  D073 — step 3 hardened to trust the closure; D061 — no toy fixtures; D074–D080 — the reorg
  (Phase 1's stale-consumer premise came from D080's dedup, which resolved to leave the template
  unchanged)
- `templates/check-rules.md`; `diff.go`; project_deltadocs `.agent/drift/` (the fixture: patches,
  expectations.json, pinned-commit; the mutation set formerly at `scratch/checker-drift-test`,
  branch deleted, commits reflog-reachable at 96d91df^..a83e4e1)
