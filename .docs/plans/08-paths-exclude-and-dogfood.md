# 8. `paths.exclude`, then dogfood

**Tier 3 — decide before investing. Cost estimate: ~20 lines.**

## Provenance

Audit finding **F9**: thirteen false findings, all traced to `ABC-1`-style marker-shaped
strings living in cinch's own test fixtures, are the only thing standing between cinch and
running `cinch check` on its own repository.

## Confirmed directly in this repo

```
$ grep -rn "ABC-1\b" internal/cinch/*_test.go | wc -l
32
$ grep -rln "ABC-1\b" internal/cinch/*_test.go
internal/cinch/rules_test.go
```

`markerScanFiles` (`internal/cinch/rules.go:125-161`) determines what gets scanned for
`// cinch:rule` markers: every git-tracked file (`gitScannableFiles`) **outside** `docsRoot`,
with no other exclusion. `rules_test.go` uses `ABC-1`-shaped strings as literal test data —
exercising `markerRe`/`ruleItemRe` against realistic-looking input — and every one of those
32 occurrences is a false positive candidate the moment `docsRoot` doesn't also declare a
matching rule item for `ABC-1` (which it structurally can't, since `ABC-1` isn't a real rule
in cinch's own corpus — it exists only as test fixture data).

This is the exact failure mode F9 named in `project_deltadocs`, just smaller in this repo (32
occurrences here vs. the audit's thirteen there) — same mechanism, same root cause: **any**
consumer whose test suite contains marker-shaped fixture strings hits this, because
`markerScanFiles` has no notion of "this path is test data, not source."

## Proposed shape

Add a `paths.exclude` manifest key — a list of glob patterns, following the existing pattern
of `paths.docs`/`paths.hooks` (`internal/cinch/paths.go:7-12`, both simple string vars with a
default) but as a list, so it reads through `manifest.list(key)` (already implemented,
`manifest.go:165-176`, used elsewhere for list-valued config) rather than
`manifestVar`. Wire it into `markerScanFiles`: after collecting the git-scannable file list
and filtering out `docsRoot` (`rules.go:129-130`), additionally filter out any path matching
a `paths.exclude` glob. `filepath.Match` (stdlib) is likely sufficient for the glob semantics;
confirm against what shape of pattern a real exclude list needs (e.g. `**/*_test.go` needs
`path.Match`-style double-star handling that `filepath.Match` doesn't natively support — check
whether a simple single-glob-per-path-component semantics is enough, or whether a small doublestar
dependency is warranted, before implementing).

`checkRules` (`rules.go:251-262`) is the only caller of `scanRuleMarkers` →
`markerScanFiles` in the check path; confirm no other caller needs the unfiltered list before
changing `markerScanFiles`'s signature or default behavior — a quick grep of
`markerScanFiles`/`scanRuleMarkers` call sites is a five-minute check before implementation.

## Then: dogfood

Once `paths.exclude` ships, add a `cinch.yml` to this repo itself (`project_deltadocs` and the
benchmark fixture already have one; cinch's own repo currently doesn't — confirmed by `find .
-name cinch.yml` returning nothing), set `paths.exclude` to cover `internal/cinch/*_test.go`
(or more precisely, whatever pattern excludes the `ABC-1` fixtures without excluding real
markers, if this repo ever grows real `cinch:rule` markers in its own `.docs/`), and run
`cinch check` on cinch itself. This item is explicitly named "dogfood" in the roadmap because
running cinch on its own repository is both a real validation of `paths.exclude` (does it
actually silence the 32 false positives without hiding true ones) and a forcing function for
noticing what else about cinch's own repo doesn't yet fit its own conventions.

## Why tier 3

This is small (~20 lines for the exclude mechanism itself) but it's placed in "decide before
investing" because the dogfooding half is open-ended — once `cinch check` runs cleanly on
cinch's own repo, it's a live question how much of the rest of this build order (item 6's
`owns:`, item 10's `kind:`) cinch should also apply to *itself*, which isn't a decision this
item alone should make. The mechanical `paths.exclude` piece is unambiguous and can ship
immediately; the "how much do we dogfood" question is the tier-3-shaped part.

## Verification

- Unit test: a fixture repo with a `paths.exclude` pattern matching a file containing an
  `ABC-1`-shaped marker string, and confirm `checkRules` no longer reports it once the glob is
  set, while a marker in a *non*-excluded file still fires normally.
- Direct regression test using this repo's actual fixture: add `paths.exclude:
  ["internal/cinch/rules_test.go"]` (or a tighter pattern) to a throwaway `cinch.yml`, run
  `cinch check` against this repo, and confirm the 32 `ABC-1` occurrences no longer produce
  findings.
- Dogfood milestone: `cinch check` exits 0 (or with only real, expected findings) when run
  against cinch's own repository, once a permanent `cinch.yml` is committed here.
