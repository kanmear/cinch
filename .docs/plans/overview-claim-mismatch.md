# cinch — settle the `overview.md` claim: template vs checker

Status: **proposed** — not started.

## Context

cinch's shipped `docs-audit-coverage.md` template claims (line 14):

> its rule-marker check walks every file in `{{paths.docs}}/` except `overview.md`

But the checker does no such thing. `scanRuleDocs` (`internal/cinch/rules.go:110-123`)
walks **every** `.md` under the docs root with no `overview.md` exclusion, and
`scanRuleMarkers` similarly. The template asserts a behavior that doesn't exist.

In `project_deltadocs` this is currently latent — its overview lives at
`.agent/domain/overview.md` and contains no `1. **ID-NNN**` rule items — but it's
a real paper cut: the workflow tells the auditor "overview.md isn't scanned",
which is false. If a rule ever lands in overview.md, the checker *will* pick it
up, and the workflow's instruction pointed the auditor in the wrong direction.

This is the same class of "template tells the model something that isn't
decidably true" that the coverage workflow otherwise works hard to avoid.

## Two facts to reconcile

1. **`docs-maintain-domain.md` template (lines 31, 37, 134) explicitly says
   overview is "a domain map, not a rule list — do not add rules there."** So
   the *intent* is: overview.md is structural prose, not a rules container.
2. **The checker scans it anyway.** Consistent with intent for now (no rules
   there), but it means the "except overview.md" clause in `docs-audit-coverage.md`
   is a lie by default.

So there are two internally-consistent worldviews to choose between:

- **Worldview A — overview is scanned like anything else.** The
  `docs-audit-coverage.md` clause is *wrong* and must be deleted/rewritten. If a
  rule is ever added to overview.md, it's scanned and marked like any other rule
  (and `docs-maintain-domain`'s "don't put rules here" is guidance, not
  enforcement). Smallest possible change: fix one template's prose.
- **Worldview B — overview is deliberately excluded, and that's enforced.**
  Add a real exclusion to `scanRuleDocs` so the template's claim becomes true,
  and add a *guard* that a rule item in overview.md is a structural violation
  (matching `docs-maintain-domain`'s prohibition) — otherwise you've just
  created a place rules can hide, which is principle 6's exact failure mode
  (a rule invisible to the checker).

## Decision needed

Which worldview? The deciding question is: **should overview.md be a place a
rule can legally live?**

- Over **A**: a rule in overview.md is scanned, must carry a marker/ignore like
  any other — nothing special. `docs-maintain-domain`'s "do not add rules there"
  becomes pure guidance. Fix = one template line. No new check state.
- Over **B**: overview is *defined out* of the rule table — matching the shipped
  doc's own assertion — so a rule that lands there is a *violation*, not a rule.
  That's the only reading that makes the "except overview.md" claim truthful AND
  keeps the no-proxy-metre principle intact (the exclusion is itself enforced,
  so nothing can hide).

The principles tilt **B** as the only honest one: the template already commits
cinch (and the auditor workflow) to "overview isn't a rules file", and principle
6 forbids letting that claim be a silent, self-licensing gap. But **B** is a
behavior change (overview can no longer carry a rule ID even if it wanted to),
where **A** is a pure prose fix. This is a real judgment call — decide, don't
default.

Note: this is *agent guidance* ambiguity, not a hittable correctness bug today.
Gate the change on whether the consumer (or cinch itself) treats overview.md as
a rules container at all.

---

## Step 1 (do regardless) — make the template truthful

Rewrite the `docs-audit-coverage.md` clause to whichever worldview won:

- **A** → drop "except `overview.md`"; say the check walks every markdown file
  under the docs root, full stop.
- **B** → keep the clause but state the consequence ("a rule item there is a
  structure violation, not a rule") so the instruction goes the same direction
  as `docs-maintain-domain`.

## Step 2 (only under B) — enforce the exclusion

`internal/cinch/rules.go`:

- Skip `overview.md` **at the docs root** (`<paths.docs>/overview.md`) in
  `scanRuleDocs` — not nested equivalent files elsewhere, to match the template's
  "overview.md" (singular, top-level). Confirm whether `project_deltadocs`'
  `.agent/domain/overview.md` counts (it isn't `<paths.docs>/overview.md`, so
  under a strict-path reading it is *not* excluded — that's a finding to surface
  and resolve in the decision).
- Add a structural guard: a `1. **ID-NNN**` item in overview.md → finding
  "overview.md is a domain map, not a rule list — move the rule to its domain
  file". This keeps the exclusion from being a hiding place.

### Tests — `rules_test.go`

- `TestRules_OverviewDocExcludedFromScan` (B only).
- `TestRules_RuleItemInOverviewFires` — the guard's mutation fixture (revert the
  guard → fixture red).

---

## Verification

1. cinch `make test` green.
2. Re-render + re-check in `project_deltadocs` — `cinch check` still clean, and
   `cinch workflows`/`cinch index` unchanged.
3. Confirm no other shipped template relies on overview being a rules file.

### Critical files

- `internal/cinch/docs/templates/docs-audit-coverage.md`
- `internal/cinch/rules.go` (B only)
- `internal/cinch/rules_test.go` (B only)

### Relationship to other plans

Independent. Small; candidates for bundling with another docs-semantics fix only
if a session is already in the templates.
