# P6 handoff — the drift test's verdict: the semantic pass confabulates; plan: C14, a blinded auditor, and the drift-test fixture

Status: P5.6 closed, and the follow-up it invited is now planned. An adversarial drift test — ten
mutations injected one per commit into a scratch branch of project_deltadocs, scored by one headless
auditor run (`claude -p --agent auditor` → `/check-rules` → `make check-harness`), then reverted —
proved the mechanical layer honest and scoped (9/9 rows caught by exactly their named checker, no
overreach) and found the semantic pass fails by **confabulation, not oversight**: it read the
inverted rule, read the marked test, quoted the contradicting assertion, and invented a rule
fragment to keep the ✅. The fixes below are the plan the next session executes: a mechanized
rule-level coupling checker (C14), a generative auditor protocol that never shows the model the
expected mapping, and a scripted fixture that turns the mutation set into the harness's regression
suite. Plus one incident the test exposed: C10's boundary is a single git moment, and it should
either widen or say so.

## What the drift test found

**The experiment.** Ten mutations, one per commit (nine committed; the C10 row left uncommitted by
construction — its window is `git diff HEAD`, so a committed change is invisible to it). One audit
run at the end on the fully-mutated branch: the semantic pass had to find the one true semantic
drift inside a pile of mechanical drift. Score against the expectation table:

| Mutation | Expected | Result |
|---|---|---|
| Rename marked test (marker rewritten to SIG-099) | C8 | caught — error (unresolved marker) + warn (SIG-012 unmarked) |
| Delete marked test (TAB-003) | C8 | caught — warn |
| Hand-edit rendered workflow | C2 tamper | caught |
| Manifest command change, no re-render | C2 staleness | caught |
| Move doc, no reindex | C1 | caught (+3 collateral C11 warns — the consumer's own pre-commit hook caught it first; the commit needed `--no-verify`) |
| `paths.tests.models` → dead dir | C4 | caught (+1 collateral C2 stale on check-rules.md — its classification fragment binds `{{paths.tests.models}}`, so the cascade is *correct* behavior) |
| Handler behaviour change, rule text left | C10 | caught — warn (uncommitted state) |
| Domain file added, not in overview | C5 | caught |
| **Rule meaning inverted (SIG-002), ID + marker kept** | **nothing structural; semantic pass only** | **missed — see below** |
| `models/` field contradicts the struct | C6 | caught |

**Mechanical layer: exactly as scoped.** Nine of nine rows fired the named checker and no other.
Nothing structural fired on the semantic mutation — the checkers check what they claim and nothing
more. The only surprise was a *correct* cascade (C4 → C2 via the composed classification fragment).

**The semantic failure is worse than a miss.** The auditor read the inverted rule ("multiple pending
requests never conflict"), read the marked test (asserts 409 `SIGNATURE_PENDING` on a duplicate),
quoted the contradicting assertion in its own report — and reported SIG-002 as ✅ covered,
rationalizing the contradiction as "complements the never-conflicts-across-different-tabs half." **A
half that does not exist.** That is not overlooking something. That is confabulating a premise to
preserve a conclusion.

**Task-framing failure, not capability failure.** The same audit, on mutation 1, correctly
distinguished a mistyped marker from a missing test — a subtler call than the contradiction it
missed. Reading comprehension was fine. The failure is the frame, which is why "add an instruction
to think harder" is the wrong fix.

## Why "think harder" is the wrong fix

Step 3 of check-rules is a leading question: *here is a rule, here is its marked test, is it
covered?* The expected answer is yes. Every green ✅ before SIG-002 reinforced the prior, and asking
a model to verify a link that already exists produces verification. The restate-the-rule-and-flag-
tension instruction is still worth adding — but as the weakest of the four fixes below. It asks the
model to catch itself. The other three don't.

## Fix 1 — C14: mechanize the drift class (this one surprised me)

The mutation was rule text changed, test unchanged. That is **C10's shape one level down**:
diff-coupling at the rule level rather than the domain level. A checker that warns when a commit
changes a rule's text and the file carrying that rule's marker does not would have caught mutation 9
outright.

- **Lateral under D009** — it binds an authored rule text to its marked test, validating no doc's
  copy of a machine-readable fact.
- **The same twenty-line shape as C10** in `diff.go`: parse which numbered items changed in
  `domain/<d>.md` (existing `parseDomainDoc`), resolve their marker files (existing
  `scanRuleMarkers`, extended to record *where* each marker lives), intersect with `gitChangedPaths`.
  Core never parses project source — nothing new to parse.
- **Honest scope:** it does not catch a rule that was wrong from birth. It catches *drift*, which is
  the realistic failure — rules rot as behaviour changes, they rarely start false.
- **Warning, not error**, same philosophy as C10: a rule-text clarification with no test change is
  legitimate; the value is asking the question when the answer is cheapest.
- **Boundary:** inherits the C10 decision below — the two should share one answer.
- **The semantic row changes meaning once C14 exists.** The drift *signal* becomes mechanical; the
  semantic pass keeps the content question — does the test's assertion actually enforce the rule's
  *meaning*? The fixture's expectation table must encode this split.

## Fix 2 — blind the semantic pass to the marker

Give the auditor the rule set and ask *which test enforces this?* — or the test set and ask *what
does this enforce?* — then diff its answers against the recorded marker mapping. Disagreements
surface as conflicts rather than requiring the model to volunteer doubt. This converts a
confirmatory task into a generative one, which is where models are reliable: the model never sees
the expected mapping, so there is no prior to preserve and no link to verify.

- Land in `templates/check-rules.md` (step 3 rewrite), re-render into both consumers, C2 green.
- Verify live in both consumers' envelopes (the E017 interactive path), not just headlessly.

## Fix 3 — frame it as a search with nonzero expected yield

"Find rules whose tests do not enforce them" beats "confirm each rule is enforced." A task expecting
zero findings trains toward zero findings. This is a re-wording of Fix 2's step, not a separate
mechanism, but it is the phrasing the template should use — and it is free.

## Fix 4 — the restate-and-flag instruction (weakest, keep it anyway)

For every marked test: restate the rule and the test's assertion in your own words; flag any
tension. Cheap, occasionally catches what 2 and 3 miss, and asks the model to catch itself — which
is exactly why it is not the primary fix.

## The drift-test fixture — the regression suite for the harness

The mutation set currently exists as a discarded branch and a file in `/tmp` (`/tmp/opencode/score.md`,
`/tmp/opencode/audit-claude.log`). It is the most valuable artifact the exercise produced, and it
must not evaporate. Rebuild it as a scripted command:

- **`drift-test`**: applies the mutation set to a throwaway `git worktree` of the real consumer
  (project_deltadocs at a pinned commit — D061: no toy fixtures), runs `bin/cinch check`, scores the
  findings against the expectation table, tears the worktree down.
- **Mutations as patches** committed in the fixture (one per row), the C10 row applied to the
  worktree *uncommitted* by construction, the C1 row committed with hooks bypassed
  (`-c hooksPath=/dev/null`) — the hook's own catch is part of the C1 row's record, not a failure.
- **The scoring table is data**: per row, the primary expected checker, the explicit negative (the
  semantic row), and the collateral findings (C11 ×3 on the C1 row, C2-stale-check-rules on the C4
  row) so a regression in a *secondary* checker also fails the run.
- **Mechanical rows fully scripted.** The semantic row needs a model: an optional gate that invokes
  headless claude with the auditor agent and asserts the SIG-002-class drift is reported. Until the
  Fix-2 protocol lands, that gate is expected to fail — which is the point: the fixture makes C14
  and the blinded auditor testable in an afternoon.
- **Home:** the mutation patches and scorer in `scripts/` with a Makefile target, or a `cinch
  drift-test` subcommand if the scorer needs Go; decide at implementation, log the decision.
- **Why it exists:** without it, the next person who touches `check.go` has no way to know they
  broke C2's tamper branch. C14 and the blinded auditor are hypotheses until the fixture says
  otherwise.

## C10's boundary — widen, or state plainly

The test exposed an incidental weakness: C10 only fires pre-commit. Its window is `git diff HEAD`,
so a committed change is invisible — and mutation 5 was pushed through with `--no-verify`. If the
hook is ever bypassed or checks run in CI post-merge, **C10 never fires at all**. "Warns on
behaviour change without doc update" reads like a property of the system rather than of one git
moment. Either widen its boundary to a commit range (needs a ref input; reconcile with D011's
no-commit-based-threshold stance), or state plainly in the code comment and roadmap that it is a
hook-only check whose working-tree window is deliberate — it self-corrects when code and doc land
in the same change (D065). One decision entry, applied to C10 and C14 together.

## Work plan (next session)

1. **Fixture first, while the mutation set is fresh.** Encode the ten rows, the collateral
   findings, the explicit negative, the hook-bypass and uncommitted-C10 mechanics. Verify the
   mechanical rows reproduce the score table above on today's binary.
2. **C14** in `diff.go`, mirroring C10; extend `scanRuleMarkers` to record marker locations; add
   the C14 row to the fixture (rule text change → C14 warn + semantic gate).
3. **Fix-2/3 template rewrite** (`templates/check-rules.md`), re-render both consumers, C2 green;
   live interactive verification in both envelopes (the E017 path).
4. **C10/C14 boundary decision** — widen or document-hook-only; log it; apply to both.
5. **Full fixture run**: with C14 + the blinded protocol, the semantic row must now be caught. If
   it is still missed, the loop still has a hole at its most important point — the fixture is what
   proves either way.
6. Log decisions (D074+) and events, refresh the roadmap status line, update this handoff.

## Not done / notes

- **The SIG-099 diagnosis is the capability baseline.** The auditor's correct call — a mistyped
  marker, not a missing test — is the reading-comprehension bar the new protocol must preserve;
  if the blinded protocol regresses it, the fixture's semantic gate will say so.
- **P5.6's own notes carry forward** (D073 closure-trust amendment, envelope asymmetry, the C11
  template-source baseline, the `.claude/` partial-tracking quirk) — see the git history of this
  file for the full list; nothing in them conflicts with this plan.
- **Nothing here is project-specific.** C14, the protocol rewrite, and the fixture bind cinch to
  its real consumer; no stack or harness literal crosses the boundary. The fixture's patches are
  consumer-shaped but live in cinch as the harness's own regression data.
