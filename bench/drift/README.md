# Drift census (plan 29, item A)

The question: in repos where `cinch check` stayed green for months, what share of rules is stale
(the rule text contradicts the code or a marked test), or under-enforced (no marked test would
fail if the code broke a clause of the rule)?

## Method

1. **Audit, one agent per repo.** Each agent ran `cinch workflow docs-audit-coverage` read-only
   against a pinned commit: Budgetron `0fe3070` (dev); the two private repos' pins are recorded
   in their reports under `private/`.
2. **Mapping before markers.** The auditor read every test with the `cinch:rule` comments
   stripped and wrote one line per test saying what it pins (`*-mapping.md`), before it read any
   marker. Only then did it compare the markers and give each rule a verdict: covered, tension,
   under_enforced, marker_misplaced, missing or na. It also judged every existing `cinch:ignore`
   reason.
3. **Independent adversarial verification.** A separate agent re-derived every non-covered
   row from the code and tests. Each row is confirmed, refuted or downgraded, with verbatim
   file:line evidence. The verifier also checked the ignore verdicts and 8 covered rows per repo,
   spread across docs. Anything new the verifier found is labelled as its own finding. Results
   are in `*-verified.csv` and in the "Verification" section of each report.
4. **Owner-confirmed rows** were taken as given and not re-verified:
   - Budgetron SMS-002 and SMS-008 (tension).
   - lucky: one tension.
   - DeltaDocs: one tension and one under_enforced row.

**Definitions:**
- **tension:** a reader of the rule would predict behaviour that the code or a marked test
  refutes.
- **under_enforced:** a quoted clause is pinned by no test at all. If a test pins it but carries
  no marker, the verdict is marker_misplaced instead.
- **missing:** no test enforces the rule anywhere.

**Rates:**
- **(a)** tensions ÷ rules.
- **(b)** (tension + under_enforced + missing) ÷ rules. This is plan 29's registered "stale or
  under-enforced".

marker_misplaced is a traceability defect, not drift, so it's in neither rate.

**Denominators:**
- Budgetron: all 39 rules (all testable).
- lucky: 95 rules minus 7 that were written ahead of their code on an in-progress branch,
  with sanctioned `test lands in ... T<n>` ignores.
- DeltaDocs: the 91 testable rules.

The private-repo reports, with their rule IDs, descriptions and pins, are in `private/`
(gitignored; sha256 in `../private-manifest.txt`). This file cites only Budgetron, the public
fixture, by rule.

## Results

These are verified counts, verifier findings included. Intervals are Wilson 95%.

| Repo | Rules | Tension | Under-enf. + missing | Misplaced | Refuted / downgraded | (a) stale text | (b) stale or under-enforced |
|---|---|---|---|---|---|---|---|
| Budgetron | 39 | 3 | 6 | 3 | 1 covered → under_enforced | 3/39 = 7.7% [2.7, 20.3] | 9/39 = 23.1% [12.6, 38.3] |
| lucky_to_have_you | 88 | 3 | 12 | 4 | 3 (one covered → tension; one ignore → tension; one ignore verdict softened) | 3/88 = 3.4% [1.2, 9.5] | 15/88 = 17.0% [10.6, 26.2] |
| DeltaDocs | 91 | 4 | 14 + 1 | 7 (+9 found among covered rows) | 2 (one → covered, one → na) | 4/91 = 4.4% [1.7, 10.8] | 19/91 = 20.9% [13.8, 30.3] |
| **Pooled** | **218** | **10** | **33** | | | **10/218 = 4.6% [2.5, 8.2]** | **43/218 = 19.7% [15.0, 25.5]** |

Counting only audit rows the verifier confirmed, without its new findings, the pooled rates are
(a) 7/218 = 3.2% [1.6, 6.5] and (b) 39/218 = 17.9% [13.4, 23.5].

**The verifier's new findings:**
- Budgetron: SMS-006 is a tension, and SET-001 is under_enforced.
- lucky: two rules the audit called covered or ignored are tensions.

## Pre-registered reading

Plan 29 §A set two lines: under 5% confirmed drift, green means something; over 20%, green is
hollow and the audit is where the work is.

**Rate (b), the registered measure, lands at the line, not on either side of it.**
- Pooled it is 19.7%, with an interval of 15–25.5%.
- Per repo it is 17.0%, 20.9% and 23.1%, and every interval crosses 20%.
- It is far above 5%, so a green `cinch check` doesn't mean the rules are enforced: about one
  rule in five has a clause no test would defend.
- It isn't clearly over 20% either, and the under-enforced rows are mostly low severity.

**What goes unpinned is mostly the same kind of clause:**
- startup wiring in `main` (Budgetron SET-001, SET-002);
- UI rendering with no widget or component test (Budgetron SMS-007; three DeltaDocs rules and
  one lucky rule);
- a parenthetical mechanism (Budgetron SMS-001's key, MONEY-001's annotation);
- the second half of a two-path rule (Budgetron SMS-003 save versus remove; one lucky rule).

The checker can't see any of these, because it checks that a marker exists, not what the marked
test asserts.

**Rate (a), stale rule text, is low: 4.6% pooled, with an interval of 2.5–8.2%.**
- The point estimate is under the 5% line, but the interval doesn't rule out more.
- The 10 tensions are 8 distinct discrepancies, since three of the DeltaDocs tensions are one
  gap stated in three docs.
- Only 4 of the 10 are owner-confirmed. The rest were confirmed or found by the verifier and
  still need the owner's sign-off.

Read together: the text mostly stays true, but the marked tests guard a good deal less of it than
a green check suggests. The work is in what the marked tests assert, not in rewriting rules.

## Limits

- **Both auditor and verifier are models,** from the same family. The verifier was told to be
  adversarial and read every cited line itself. But it shares the auditor's blind spots: it can
  miss a gap neither of them looks for, or call a clause unpinned when a test pins it
  indirectly.
- **The owner confirmed only a subset:** 5 rows, listed under Method, step 4. Every other
  verdict is model-confirmed. The rows the verifier found itself (Budgetron SMS-006 and SET-001,
  two lucky rows) have had no second read.
- **lucky was audited on a moving branch.** The owner was committing while the audit ran. Both
  passes read the pinned commit through `git show`, not the working tree, but the 7 rules in
  flux on that branch had to be excluded by hand.
- **Small n.** There are three repos and 218 rules, from one author's projects. The intervals
  are wide, and per-repo rates of 17–23% can't be told apart.
- **Severity is not weighted.** A UI parenthetical counts the same as a stranding gate that
  never fires for the editor.
- **Denominators differ slightly:** lucky's 88 includes rules under `cinch:ignore`, and
  DeltaDocs' 91 excludes N/A rules.

## Seed set for claim #5 (confirmed stale rules)

Rows the owner hasn't confirmed are marked †. Descriptions for the private repos are in
`private/lucky-verified.csv` and `private/deltadocs-verified.csv`.

**Budgetron (public):**
- **SMS-002:** "a template error ... skips the message". In fact the next enabled template is
  tried and the message is still imported.
- **SMS-008:** "an amount followed by a three-letter code". The code requires two decimals.
- **SMS-006 †:** "Scans run at most once a minute". The manual scan button is never throttled,
  even though the cubit's own comment says manual scans are.

**lucky_to_have_you (private):** 3 rows, 1 owner-confirmed.

**DeltaDocs (private):** 4 rows, 1 owner-confirmed; three of them are one gap stated in three
docs.
