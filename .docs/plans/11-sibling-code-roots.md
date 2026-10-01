# 11. `rules.roots` and hook-baseline overrides

**Tier 1 — trust the checker. Cost estimate: ~150 lines (rules.go/manifest.go/check.go +
tests), landed in one session.**

## Provenance

**External consumer report**, not from the F1–F12 audit/benchmark corpus: splitting
`fsed-odin-mono` (a real cinch consumer) into four sibling repos — `backend`, `frontend`,
`mobile`, and a `.agent` docs/harness repo — hit a wall neither source anticipated. `rules` and
the generated `pre-commit` hook both assume the docs corpus and the code it governs share one
repo. Once `.agent` no longer contains `backend`/`frontend` source, `rules` cannot pass — not a
config mistake, a structural limitation — and there was no `cinch.yml` key to scope what the
hook itself runs (only to add *extra* steps), so the consumer's docs repo couldn't even commit
its own changes once hooks were active.

## Confirmed directly in this repo

`checkRules(repoRoot, docsRoot)` (pre-change: `internal/cinch/rules.go:339`) always scanned only
`repoRoot`'s own git worktree via `markerScanFiles` → `gitutil.ScannableFiles(repoRoot)`
(`rules.go:213`, falling back to `filepath.WalkDir(repoRoot, ...)`) — `repoRoot` is always `"."`
in practice (`check.go`'s `runChecks(".", ...)`), so a docs-only checkout has zero candidate
files. Reproduced against the actual consumer: `cinch check` in the original `fsed-odin-mono`
monorepo reports `rules: ok (82 rules, 6 rule docs, 8 ignores)`; the identical command in the
split-out `.agent` repo reported 74 "no // cinch:rule marker" findings, one per documented rule —
100% false, since the enforcing tests simply aren't in that repo anymore.

Separately, `preCommitChecks`/`commitMsgChecks` (`check.go:37-43`, pre-change) hardcoded their
`only` list in Go source (`"links", "rules", "index", "generated", "core"` /
`"commit"`) with no parameter or manifest read anywhere in `cmdHookPreCommit`/`cmdHookCommitMsg`
(`hook.go`) — `hooks.<event>.<name>` entries can only add steps, never scope the baseline. So even
knowing `rules` couldn't pass, there was no `cinch.yml`-level way to exclude it from the hook that
blocks every commit.

## Proposed shape (shipped)

Two additive, backward-compatible manifest keys — nothing about `paths.docs`/`paths.hooks`
changed, since a docs-only repo already passes every *other* check standalone.

**`rules.roots`** — a list of sibling-relative paths (`..` explicitly allowed) also scanned for
`// cinch:rule` markers:
```yaml
rules:
  roots: [../backend, ../frontend, ../mobile]
```
`markerScanFiles` now takes a `ruleScanRoot{path, display}` and returns `scannedFile{fsPath,
display}` pairs instead of bare relative strings, so a sibling's files open from the right
filesystem path (`fsPath`) but report under the configured prefix (`display`, e.g.
`../backend/errors/codes.go`) — `resolveRuleScanRoots` (`rules.go`) splits configured roots into
present/missing by `os.Stat`, and `scanRuleMarkers` scans every present root, merging results.
`manifest.go`'s `validate()` gained a second, narrower loop for `rules.roots`: unlike
`repoLocalPathKeys`' `filepath.IsLocal` gate (writes must stay inside the repo), this only rejects
absolute paths — reading a named sibling is the point. If every configured root is absent (a
docs-only clone, or CI that checked out one repo), `checkRules` reports a single `rules: skip:
configured code roots not present (...)` instead of one misleading "no marker found" per rule.
`cinch rules --json` (`buildRulesInventory`) also honors `rules.roots`, for the same
"machine-readable inventory should match what `check` sees" reason; `cinch impact` deliberately
does not — it only ever matches a marker's file against a locally-changed/staged path, which by
definition can never live in a sibling repo, so threading roots through there would be pure
overhead with zero possible payoff.

**`hooks.pre-commit-checks` / `hooks.commit-msg-checks`** — optional overrides of the hook
baseline:
```yaml
hooks:
  pre-commit-checks: [links, index, generated, core]
  commit-msg-checks: [commit]
```
`preCommitChecks`/`commitMsgChecks` each load their own manifest copy (matching
`runChecks`/`ResolveDocsRoot`'s existing style of reloading rather than threading one through) to
look up the override, falling back to today's hardcoded default when absent — a malformed
`cinch.yml` is *not* treated as an error here, since `runChecks`'s own load already surfaces that
properly via the `generated` check. With `rules.roots` configured correctly this override isn't
needed for the motivating case (`rules` legitimately passes or cleanly skips) — it's a general
escape hatch for a consumer who wants a check excluded from the *hook* specifically for some other
reason, which turned out to be nearly free to add alongside `rules.roots` since both touch the
same corner of `check.go`.

## Why tier 1

`rules` is "exercised daily and is solid" (00-overview.md) — this item is about keeping that true
for a docs repo that's structurally separated from its code, not adding a new kind of check or
deciding a design question first. Every change is additive and off by default; existing consumers
see zero behavior change (confirmed: the full existing test suite passes unmodified against the
refactored `markerScanFiles`/`scanRuleMarkers`).

## Verification

- `go test ./...` — full suite green, including the pre-existing battery unmodified.
- New coverage: `TestScanRuleMarkersPrefixesSiblingRootFindings`, `TestCheckRulesScansSiblingRoot`,
  `TestCheckRulesSkipsWhenAllRootsMissing`, `TestCheckRulesPartialRootsStillChecksPresentOnes`,
  `TestCheckRulesUnaffectedWithoutConfiguredRoots` (`rules_test.go`); `TestManifestValidateAllows
  DotDotInRulesRoots`, `TestManifestValidateRejectsAbsoluteRulesRoots`, plus two IsLocal regression
  tests since `validate()` had zero prior coverage (`manifest_test.go`); `TestPreCommitChecksHonors
  ManifestOverride`, `TestCommitMsgChecksHonorsManifestOverride` (`check_test.go`).
- End-to-end against the real consumer: added `rules: {roots: [../backend, ../frontend,
  ../mobile]}` to `fsed-odin-agent`'s `cinch.yml` (built from this branch) and ran `cinch check`
  from within it — `rules: ok (82 rules, 6 rule docs, 8 ignores)`, exact parity with the original
  monorepo's result. Temporarily renamed all three sibling checkouts away and re-ran: `rules:
  skip: configured code roots not present (../backend, ../frontend, ../mobile)` — clean skip, zero
  false findings.
