# 22. Catch matrix and cost curve

**Tier 4 — evidence. Cost estimate: hours for the matrix, about an hour for the curve. No
models involved.**

## Provenance

**Audit** (published). Claims #1 and #6 in [21](21-benchmark-suite.md). Both are
deterministic, so they go first: nothing about them depends on agent behavior or on the public
fixture.

## Catch matrix (claim #1: the checker catches what it says it catches)

The seeded-defect tests already exist (`TestSeededDefect*` in `internal/cinch/rules_test.go`,
`links_test.go`, `checks_misc_test.go` and `introspect_test.go`, from item
[03](03-rule-grammar-holes-and-seed-defects.md)). This item publishes them as a table.

1. Enumerate every seeded defect class, along with the check that owns it.
2. For each one, record **caught** (test name plus the finding line it asserts) or
   **documented non-catch** (the test that pins the blind spot, plus the reason it's out of
   scope). An example of the second is the reachability half dropped in item
   [05](05-navigability-check-class.md), with its comment in
   `TestSeededDefectLinksTargetUnreachableFromRoot`.
3. Add the deliberate blind spots that have no test yet. The headline example is a hollowed
   test, item 04's T3 shape: the marker resolves but the test no longer exercises the rule.
   Add each as a seeded test asserting `ok`, so the non-catch is pinned and can't silently
   become a catch, or the reverse, without the table changing.
4. Generate `bench/catch-matrix.md` from the tests rather than by hand, for example with
   `go test -run TestSeededDefect -v` plus a small extractor, so the table can't drift from the
   suite.

## Cost curve (claim #6: cheap enough for every commit)

1. Generate synthetic repos at 1k, 10k and 100k files with a fixed share of docs, rules and
   markers.
2. Time `cinch hook pre-commit` on each one, on both paths of `stagedSnapshot`
   (`internal/cinch/staged.go`): the fast path (clean worktree, so `fsRoot == repoRoot`) and the
   snapshot path (an unstaged or untracked change forces `git checkout-index`).
3. Take the median of N runs per cell, and state the hardware.
4. Cross-check against the real `ms` column from [19](19-field-log.md)'s log once it exists.

Report the result honestly, and keep it out of the headline.

## Verification / definition of done

- `bench/catch-matrix.md` and `bench/cost-curve.csv` are committed along with the scripts that
  produce them.
- Every row in the matrix names a test that passes in `go test ./...`.
- The new blind-spot tests are in the suite, and `cinch check` still passes.
