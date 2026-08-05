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

## Items

1. **Fixture first, while the mutation set is fresh.** `drift-test` applies the mutation set to
   a throwaway `git worktree` of the real consumer (project_deltadocs at a pinned commit —
   D061: no toy fixtures), runs `bin/cinch check`, scores the findings against the expectation
   table, tears the worktree down.
   - Mutations as patches committed in the fixture, one per row; the C10 row applied *uncommitted*
     by construction (its window is `git diff HEAD`); the C1 row committed with hooks bypassed
     (`-c hooksPath=/dev/null`) — the hook's own catch is part of the C1 row's record, not a failure.
   - The scoring table is data: per row, the primary expected checker, the explicit negative (the
     semantic row), and the collateral findings (C11 ×3 on the C1 row, C2-stale-check-rules on the
     C4 row), so a regression in a *secondary* checker also fails the run.
   - Mechanical rows fully scripted. The semantic row needs a model: an optional gate that invokes
     headless claude with the auditor agent and asserts the SIG-002-class drift is reported. Until
     item 3 lands, that gate is expected to fail — which is the point: the fixture makes C14 and
     the blinded auditor testable in an afternoon.
   - Home decided at implementation: mutation patches and scorer in `scripts/` with a Makefile
     target, or a `cinch drift-test` subcommand if the scorer needs Go. Log the decision.
   - Verify the mechanical rows reproduce the P5.6 score table on today's binary — the regression
     proof for the next person who touches `check.go`.
2. **C14** in `diff.go`, mirroring C10: warn when a commit changes a rule's text and the file
   carrying that rule's marker does not. Lateral under D009 — it binds an authored rule text to
   its marked test, validating no doc's copy of a machine-readable fact.
   - The same twenty-line shape as C10: parse which numbered items changed in `domain/<d>.md`
     (existing `parseDomainDoc`), resolve their marker files (existing `scanRuleMarkers`, extended
     to record *where* each marker lives), intersect with `gitChangedPaths`. Core never parses
     project source — nothing new to parse.
   - Honest scope: it does not catch a rule wrong from birth; it catches *drift*, the realistic
     failure — rules rot as behaviour changes, they rarely start false.
   - Warning, not error, same philosophy as C10: a rule-text clarification with no test change is
     legitimate; the value is asking the question when the answer is cheapest.
   - Boundary: inherits item 4's decision — C10 and C14 share one answer.
   - The semantic row changes meaning once C14 exists: the drift *signal* becomes mechanical; the
     semantic pass keeps the content question — does the test's assertion actually enforce the
     rule's *meaning*? The fixture's expectation table must encode this split.
3. **Blind the semantic pass to the marker** (Fix 2, plus Fix 3's framing and Fix 4's restate-
   and-flag step). Rewrite `templates/check-rules.md` step 3: give the auditor the rule set and
   ask *which test enforces this?* — or the test set and ask *what does this enforce?* — then diff
   its answers against the recorded marker mapping. Disagreements surface as conflicts rather than
   requiring the model to volunteer doubt; the model never sees the expected mapping, so there is
   no prior to preserve and no link to verify.
   - Frame it as a search with nonzero expected yield: "find rules whose tests do not enforce them"
     beats "confirm each rule is enforced".
   - Keep the restate-and-flag instruction as the weakest layer: for every marked test, restate the
     rule and the test's assertion, flag any tension.
   - Land in the template, re-render into both consumers, C2 green. Verify live in both consumers'
     envelopes (the E017 interactive path), not just headlessly.
   - The SIG-099 diagnosis is the capability baseline: the auditor's correct call — a mistyped
     marker, not a missing test — is the reading-comprehension bar the new protocol must preserve;
     if the blinded protocol regresses it, the fixture's semantic gate will say so.
4. **C10/C14 boundary decision.** C10 only fires pre-commit: its window is `git diff HEAD`, so a
   committed change is invisible — and mutation 5 went through with `--no-verify`. "Warns on
   behaviour change without doc update" reads like a property of the system rather than of one git
   moment. Either widen the boundary to a commit range (needs a ref input; reconcile with D011's
   no-commit-based-threshold stance) or state plainly in the code comment and roadmap that it is a
   hook-only check whose working-tree window is deliberate — it self-corrects when code and doc
   land in the same change (D065). One decision entry, applied to C10 and C14 together.
5. **Full fixture run.** With C14 and the blinded protocol, the semantic row must now be caught.
   If it is still missed, the loop still has a hole at its most important point — the fixture is
   what proves either way.
6. **Close.** Log decisions and events, refresh the roadmap status line, close this plan per the
   completion ritual (feature plans persist with `Status: complete`).

## Verification

- Fixture reproduces the P5.6 score table on today's binary, before C14 lands.
- `go vet ./...` and `go test ./...` after Go changes; `cinch check` green on cinch's own repo
  and on project_deltadocs.
- Both consumers' rendered check-rules.md C2-green; one live auditor run per envelope (the E017
  interactive path).
- C14 row: rule-text change → C14 warn + semantic gate fires; SIG-099-class call preserved.

## References

- `docs/HANDOFF.md` — the P6 verdict this plan supersedes (git history is the archive)
- E016/E017 — auditor envelope workouts; D065 — C10; D067 — the checker closure is authoritative;
  D073 — step 3 hardened to trust the closure; D061 — no toy fixtures
- `templates/check-rules.md`; `diff.go`; project_deltadocs (the fixture's real consumer)
