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

## Phase 1 — Sync project_deltadocs `.agent` after the reorg [project]

deltadocs last landed 2026-08-05 00:18 (the P5.6 commit 96d91df); the reorg (D074–D080) landed
after it, and its Phase 4 dedup changed `templates/doc-philosophy.md`. The consumer's rendered
workflows are therefore stale — C2 red — and the fixture pins a deltadocs commit, so a stale
baseline would poison the expectation table's known-warn rows before the fixture exists.

- Re-render the consumer corpus, C2 green, `make check-harness` exit 0 at the known baseline
  warns (the two designed C6/C7), commit.
- Check for anything else the reorg left stale in the consumer's `.agent` while there.

**Exit:** deltadocs HEAD C2-green; `make check-harness` exit 0 at the baseline; diff reviewed by
the user.

## Phase 2 — Restore the mutation set as `make drift-test` [project]

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

## Phase 3 — C10/C14 boundary decision, then C14 [cinch]

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

- `docs/HANDOFF.md` — the P6 verdict this plan supersedes (git history is the archive)
- E016/E017 — auditor envelope workouts; D065 — C10; D067 — the checker closure is authoritative;
  D073 — step 3 hardened to trust the closure; D061 — no toy fixtures; D074–D080 — the reorg
  (the source of Phase 1's stale consumer)
- `templates/check-rules.md`; `diff.go`; project_deltadocs (the fixture's real consumer;
  `scratch/checker-drift-test` reflog is the mutation set)
