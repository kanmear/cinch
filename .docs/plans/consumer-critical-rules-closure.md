# project_deltadocs — reconcile AGENTS.md "Critical Rules" with cinch's scope

Status: **proposed** — not started.

Target repo: `project_deltadocs` (planned here per the "all in cinch plans dir"
decision).

## Context

`AGENTS.md:9` carries the repo's most important operational rule:

> **Always run checks/tests/formatting via `make <target>` from the project root.**
> Never `cd` into a subdirectory and run subcommands directly.

This is a **documented, unenforced, un-declared convention**. It sits at the repo
root, **outside** `paths.docs` (`.agent`), so cinch's rules/links/coupling checks
never see it — no rule ID, no `// cinch:rule` marker, no `cinch:ignore`
declaration. Two separate gaps:

1. **The rule is invisible to cinch.** Even if someone wanted to bind it, cinch
   only scans `.agent/`.
2. **It isn't decidable**, so it can't be a test-enforced cinch rule anyway — "a
   contributor always uses `make` from the project root" is process, not a
   property a script can assert from outside the agent.

Per cinch's own design, a documented-but-untestable rule is not silent — it's
either bound to a marker (testable) or **declared `cinch:ignore` with a reason**
(an explicit, reasoned statement that it's outside a test's domain). This rule
currently falls into a third, forbidden nether-state: enforced by nothing and not
even declared.

## Decision needed — the right home for a process rule

The cleanest cinch-aligned move is to **relocate the rule into the corpus as a
declared rule**, so cinch's closure and `cinch ignores` inventory are honest
about it:

- Put the `make`-from-root rule into `.agent/conventions.md` (it's already the
  cross-cutting conventions doc, and it currently *duplicates* the same guidance
  at `.agent/conventions.md:40`) as a numbered rule item, e.g.
  `1. **TOOL-001** …` with a `<!-- cinch:ignore: process convention, enforced by
  the session start hook and code review, not by a scriptable property -->`
  declaration. Then **`cinch ignores` lists it**, and the AGENTS.md mention can
  cite the rule ID instead of duplicating prose.

Decide which target doc:

- **A — `.agent/conventions.md` (recommended).** It already restates the same
  rule at :40; consolidating removes duplication (principle 2 / derivability).
- **B — keep it only in AGENTS.md** and treat AGENTS.md as outside cinch's scope
  (its own entry point, not part of the scanned corpus). This is defensible but
  leaves the rule silent to `cinch ignores`.

## Decision needed — should cinch ever scan AGENTS.md?

This is the *cinch-core* side of the same question. `paths.docs` is currently a
single root at `.agent`, and AGENTS.md (repo root) is typically where the most
load-bearing process rules live. Options:

- **A — No change now.** Keep AGENTS.md outside the corpus; the fix is purely
  consumer-side (move/declare the rule into `.agent/conventions.md`). Smallest
  footprint.
- **B — A multi-root corpus.** Allow `paths.docs` to be a list (`.agent`,
  `AGENTS.md`), so root-level process rules get scanned. Real but larger: touches
  every check's root resolution, `cinch index`/`workflows`, and `scanRuleDocs`.
  This is a genuine cinch-core feature request with real benefit (the repo's most
  important rules are at the root), but it's out of scope for a single
  consumer-side session and deserves its own plan.

Recommend the **consumer-side fix now (A for decision 1, A for decision 2)**,
and file decision-2-B as a separate cinch-core proposal if the user wants
root-level rules covered.

## Step 1 — declare the rule in the corpus

In `.agent/conventions.md`, under a numbered rule item carrying the `make`-from-
root guidance, add the ID + `cinch:ignore` declaration with a reason. Update the
existing :40 prose to reference the rule ID rather than re-describe it.

## Step 2 — update AGENTS.md

Have AGENTS.md:9 point at the rule ID in `conventions.md` (and the `cinch ignores`
inventory) instead of duplicating the full text — or keep the reading-guide
summary but cite the ID. Ensure AGENTS.md's mention and the rule item don't
drift apart.

## Step 3 — confirm the inventory

Run `cinch ignores` → the rule must appear with its reason. `cinch check` stays
clean. Optionally document the decision (why this rule is ignore-declared, not
marker-enforced) at the top of `conventions.md`.

---

## Verification

- `cinch ignores` lists the new declaration with a nonempty reason.
- `cinch check` clean.
- `grep -c 'TOOL-001' AGENTS.md .agent/conventions.md` ≥ 2 (cited correctly).
- No behavioral change.

### Critical files

- `project_deltadocs/AGENTS.md`
- `project_deltadocs/.agent/conventions.md`
- (if decision-2-B) cinch `internal/cinch/*` + `manifest.go` — deferred to a
  separate plan.

### Relationship to other plans

- The **consumer fix (Steps 1-3)** is standalone and small.
- **Decision-2-B (scan AGENTS.md) is explicitly a cinch-core feature** and
  should be its own proposed plan if pursued — do not fold it into this
  consumer-scoped plan.
