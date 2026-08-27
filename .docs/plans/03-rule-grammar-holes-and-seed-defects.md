# 3. Close the rule-grammar holes, then seed defects to find the rest

**Tier 1 — trust the checker. Cost estimate: days.**

## Provenance

Audit finding **F2** (two specific holes, found by hand) plus the benchmark's methodology
(a seeded-defect battery was how the T3 task and the wrong-corpus study, `~/code/cinch_bench/
FINDINGS.md` §9.1, were built — this item generalizes that technique into a permanent
regression suite for the checker itself).

## Part A — the two known holes

### A1. Near-miss rule IDs are invisible

`checkRulesFrom` (`internal/cinch/rules.go:277-331`) computes `ruleIDs := ruleItemIDs(items)`
— the set of IDs actually declared in a numbered rule item — and cross-checks every marker
against it:

```go
for id, locs := range markers {
    if ruleIDs[id] {
        continue
    }
    for _, loc := range locs {
        findings = append(findings, finding{... "does not resolve to any rule ID"})
    }
}
```

This *does* fire for a marker pointing at an ID that doesn't exist at all. What it doesn't
catch: two IDs that are near-misses of each other — `SIG-019` vs `SIG-019` typo'd as `SIG-091`
in one place and correct in the doc, or a doc typo (`SIG-01` vs `SIG-019`) that happens to
still resolve because *some* marker somewhere matches the typo'd form and *some* rule item
somewhere matches it too, just not the same rule/marker pair the author meant to bind. The
current check is purely set-membership; it has no notion of "this ID is suspiciously close to
a real one and probably a typo." Fix: when a marker ID doesn't resolve (the existing `!ruleIDs[id]`
branch), additionally search `ruleIDs` for near-misses (e.g. edit distance ≤ 2, or same prefix
+ different number) and append them to the finding message as a suggestion — "does not resolve
to any rule ID (did you mean SIG-019?)". This turns a silent gap into a **more useful
finding**, not a new check.

### A2. Duplicate rule IDs are keyed wrong

`ruleItemIDs` (`rules.go:46-52`) builds `map[string]bool` — a set. If two different rule
items across two different docs both declare `SIG-019`, the map just has one `true` entry;
nothing ever notices two authors independently claimed the same ID. The fix: key by
`(id → []file:line)` instead of `(id → bool)` during `checkRulesFrom`, and after building the
per-item findings, scan for any ID with more than one declaring location and emit a
`rules`-check finding — "SIG-019: declared in two places (`domain/signatures.md:12` and
`domain/tabs.md:44`) — rule IDs must be unique". This is a small change to
`checkRulesFrom`'s bookkeeping (it already builds a `docs map[string]bool` from `items`;
extend the same loop to also build `id → []ruleItem` and check `len(...) > 1`).

## Part B — the systematic pass: seed a defect battery

The audit found A1 and A2 by reading the code. That's necessarily incomplete — a human
auditor stops when they've found a plausible number of holes, not when they've covered the
grammar. The benchmark's wrong-corpus study (`FINDINGS.md` §9.1) built exactly this kind of
battery for a different purpose (testing whether *agents* notice a falsified rule) and it's
directly reusable as a **checker** regression suite instead.

Build 25–30 seeded defects, each a minimal fixture tree plus "here's what `cinch check`
*should* report." Categories, grounded in what the current code actually checks
(`internal/cinch/check.go:74-86` launches `links`, `rules`, `retirement`, `generated`,
`commit`, `core`, `hooks`):

- **rules.go grammar**: marker deleted (should fire, and does — baseline case to confirm the
  battery itself works); marker on a hollowed-out test (the exact T3 construction — cinch
  can't detect this today and shouldn't be expected to structurally, but it should be in the
  battery as a documented non-catch, not a silent gap); rule text edited to contradict a
  neighboring rule's ID range; duplicate ID (A2); near-miss ID (A1); `cinch:ignore` with no
  reason (`rules.go:283-288`, already covered — confirm it stays covered); `cinch:ignore`
  co-occurring with a real marker (`rules.go:289-294`, already covered).
- **links.go**: link target moved without updating the link (already covered —
  `checkLinksInFile`, `links.go:48-94`); link target exists but is unreachable from any
  declared root (**not** covered today — this is item 5's job, but the battery should assert
  it's uncovered *now* and covered *after* item 5 ships, giving item 5 a concrete acceptance
  test).
- **retirement.go**: ID present at `HEAD^`, absent at `HEAD`, no tombstone (already covered —
  `retirementFindingsForFile`, `retirement.go:60-87`); ID removed and a tombstone pattern
  matches but references the *wrong* ID (`idIsTombstoned`, `retirement.go:142-149`, matches
  by `strings.Contains(m, id)` against every tombstone match in the whole file — a tombstone
  for `SIG-020` that happens to also contain the substring `SIG-02` would falsely satisfy a
  retired `SIG-02` if that were ever a real ID; low-probability but worth one seeded case).
- **generated.go / header.go**: a rendered file hand-edited after generation (header hash
  mismatch — confirm `hasGeneratedHeader` / `bodyHash` in `header.go:19-22,32-39` catch this;
  it's the mechanism `generated.go` presumably wraps, worth a fixture regardless of which file
  owns the check).
- **owns:/frontmatter class (forward-looking)**: once item 6 lands, `owns:` pointing at a
  moved or deleted file should be a battery case from day one, not bolted on later.

Each fixture becomes a small directory under a new `testdata/defects/` (or similar, following
this repo's existing `_test.go` + fixture conventions — check how `rules_test.go`,
`links_test.go`, `retirement_test.go` currently embed or construct fixtures before deciding
between inline Go literals vs. `testdata/`) with an expected finding list, run as a table
test. No agents, no models — this is a checker unit-test suite, and it runs in seconds.

## Why this is tier 1

Both parts fix the same category of problem tier 1 exists to fix: cases where `cinch check`
reports green (or a misleading finding) for a corpus that is actually broken. Item 1 fixed
"green when it should be red because of a parser bug." This item fixes "green when it should
be red because the grammar has holes." Both are preconditions for tier 2 items (4, 5, 6) to
mean anything — there's no point measuring binding strength or navigability on a checker whose
own pass/fail signal on the *existing* checks can't be trusted.

## Verification

- A1/A2: unit tests in `rules_test.go` — one fixture with a near-miss marker asserting the
  suggestion appears in the finding message; one fixture with a duplicate ID asserting a new
  "declared in two places" finding with both locations named.
- Part B: the battery itself is the deliverable. Success criterion: every category above has
  at least one fixture, the table test passes against current `cinch check` behavior (with
  the two known-uncovered cases — hollowed marker, unreachable-but-existing link — explicitly
  asserted as *not* caught, documenting the gap rather than hiding it), and the suite is
  wired into `make test` / CI so it runs on every future change to any `check*.go` file.
