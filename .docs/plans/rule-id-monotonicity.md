# cinch — detect silent rule-ID disappearance across history

Status: **proposed** — not started.

## Context

From the "Checking the Checker" audit (2026-08-18), GAP-01 + OPT-03 — the single
highest-priority item in that report's closing verdict.

`project_deltadocs/.agent/rules.md:137` declares rule IDs permanent: append-only,
never renumbered, tombstoned in place when retired. Over 50 live citations across
`business/`, `api/`, plans, and tests depend on that promise holding. Nothing in
cinch verifies it — `checkRules` (`internal/cinch/rules.go`) only checks that
*currently present* rule items have a marker and vice versa; it has no memory of
what existed at a previous commit. A rule ID silently deleted from a doc today
(as opposed to tombstoned per the consumer's own convention) produces zero
findings: `rules` sees a smaller, still-consistent doc; `coupling` only compares
working tree vs HEAD for *text changes on IDs that still exist* (`coupling.go`
`checkCoupling`), so an outright removal isn't "text changed," it's "item gone,"
which the loop in `checkCoupling` never visits.

This is exactly the shape principle 1 exists for: a real, decidable property
(rule IDs are stable) with no checker behind it.

## Design

Reuse `coupling`'s existing machinery rather than adding a new checker category:
it already resolves `git show HEAD:<path>` for docs-dir markdown files and
already runs `parseRuleItems` to get IDs. Extend that walk one step further back
than HEAD-vs-working-tree — HEAD vs its parent — and compare rule-ID sets per
file:

- An ID present at `HEAD^` and absent at `HEAD`, with no tombstone marker
  recognized in its place, is a finding.
- A "tombstone" is project-defined — DeltaDocs' `rules.md` uses its own
  convention for retired IDs, and cinch must not hardcode one project's format
  (principle 5: bind values, not shape). A new manifest key, e.g.
  `rules.tombstone`, holds a regex or literal marker text (a plain string
  value, same shape as `commit.pattern`); absent key means no tombstone
  convention is recognized and *any* ID disappearance is a finding — a stricter
  default, matching how absent `commit.pattern` means no convention enforced
  rather than no convention possible.
- Runs at the same point `coupling` runs (pre-commit / `cinch check`), since it
  needs the same git plumbing and the same "no-op outside a git repo / before
  first commit" handling `coupling` already has.

Open question to resolve before writing tasks: same checker (`coupling`) with a
second finding class, or a new checker name (e.g. `identity`) sharing
`coupling.go`'s internal helpers. Given the direction rule and principle 5, lean
toward a new checker name — "coupling" already means something specific
(text-changed-without-marker-changed) and overloading it muddies both the
`cinch check` output and this plan's own mutation fixture.

## Steps

1. Add the manifest key `rules.tombstone` (optional, opt-in — same absence
   contract as `commit.pattern` and `require.cinch`).
2. Add a checker (working name: `identity`) that, for each docs-dir markdown
   file, diffs the rule-ID set at `HEAD^` against `HEAD` (skip when there's no
   parent commit — root commit — same guard shape as `hasHead` in
   `coupling.go`) and reports an ID that vanished without a recognized
   tombstone.
3. Wire it into `CmdCheck` (`internal/cinch/check.go`) alongside the other five.
4. Update `README.md` § Status (the five-check list) to a six-check list, and
   add the check to § Known limitations if the `HEAD^`-only window carries the
   same one-commit-back caveat `generated`'s path-migration detection has —
   likely yes, and worth stating rather than discovering later.
5. Per principle 1, a mutation fixture proving the checker: a fixture commit
   that removes a rule ID with no tombstone must fail; the same removal with a
   recognized tombstone must pass; a normal edit that doesn't touch IDs must
   pass.

## Verification

- `make test` — new checker's mutation fixture passes/fails as designed
  (Tier 1/2 shape: the fixture fails before the checker exists conceptually,
  passes after).
- `cinch check` in `project_deltadocs`, once its `cinch.yml` is updated with
  its actual `rules.tombstone` convention (a follow-up consumer-side commit,
  not part of this plan) — confirms the new checker is silent against 75 live
  rule IDs and their real tombstone history.
- `go vet ./...` clean.

### Critical files

- `internal/cinch/coupling.go` — the git-walk and rule-item-diff machinery to
  reuse, not duplicate.
- `internal/cinch/rules.go` — `parseRuleItems`, the parser this reuses.
- `internal/cinch/check.go` — where the new checker wires into `CmdCheck`.
- `internal/cinch/manifest.go` — where `rules.tombstone` becomes a recognized
  optional key, if manifest keys need explicit registration (confirm against
  current code — `commit.pattern` and `require.cinch` are read ad hoc via
  `m.Vars[key]`, so this may need no manifest.go change at all).
- `README.md` §§ Status, Known limitations.

### Relationship to other plans

Independent of the others below. Consider sequencing before
`coupling-window-config.md` — both touch `coupling.go`'s neighborhood, and
landing the new checker first avoids two plans conflicting over the same file's
git-walk helpers.

This is new capability, not a fix: on completion, mark status `complete` and
keep the file as the durable record of the tombstone-convention design
decision (why a manifest key rather than a hardcoded format).
