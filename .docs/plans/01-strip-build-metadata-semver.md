# 1. Strip build metadata before parsing semver

**Tier 1 — trust the checker. Status: shipped in `a35cbe1`. Kept here for provenance and
because item 2 (`cinch upgrade`) exists partly *because* this bug was expensive to hit
manually.**

## Provenance

Audit finding **F4**. Cost estimate at the time: one line.

## What was wrong

`parseSemver` (`internal/cinch/semver.go`) split on `.` and called `strconv.Atoi` on each
part. Hotfix releases are stamped `X.Y.Z+<letter>` (build metadata, per the semver spec) —
e.g. `0.2.3+a`. `Atoi("3+a")` fails, `parseSemver` returns `ok=false`, and every caller that
depends on it — `semverAtLeast`, `semverEqual` — silently fell back to **string equality**.

The consequence lands in `checkPin` (`internal/cinch/pin.go`): a consumer with
`require.cinch: ">=0.2.0"` and cinch `0.2.3+a` installed should pass. Instead
`semverAtLeast("0.2.3+a", "0.2.0")` returned `false` (string paths don't match), `checkPin`
emitted a `core`-level error, and the remediation text it printed — "reinstall and re-run
cinch render" — could not fix it, because the *next* release would also carry a hotfix
suffix. Both of the audit's real-world consumers were red for three releases for exactly this
reason (see item 2's rationale, F5).

## The fix

```go
func parseSemver(s string) (v semver, ok bool) {
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	// ... unchanged Atoi-per-part logic on the stripped string
}
```

One `IndexByte` + slice before the existing split. `semverAtLeast` and `semverEqual` needed
no changes — they already delegate to `parseSemver` and already fall back to string equality
when parsing fails for an unrelated reason (a real `dev` build or a non-numeric pre-release
tag), so the fallback path stays intact for those cases.

Shipped alongside a `semver_test.go` case pinning `0.2.3+a` >= `0.2.0` to `true`, and a
one-line change to `internal/version/hotfix` (unrelated to the parser fix, bundled in the same
commit).

## Why this had to go first

Range pins (`require.cinch: ">=X.Y.Z"`) are the mechanism that is supposed to make
`cinch upgrade` (item 2) unnecessary between compatible releases — a consumer pinned to a
range shouldn't need to touch its manifest on every patch bump. If the range-pin comparison
itself is broken on the exact version format the release process produces, nothing built on
top of pins — including the whole premise of item 2, which assumes pins can be trusted to
signal real incompatibility — is trustworthy. This is why it's item 1 in a list ordered by
"what unblocks what," not by severity: a one-line fix, but everything downstream in tier 1
was noise until it landed.

## Verification

- `internal/cinch/semver_test.go` covers `parseSemver`, `semverAtLeast`, and `semverEqual`
  against a `+`-suffixed version on both sides of the comparison.
- Manual check against the historical failure: `checkPin("0.2.3+a", m)` with
  `require.cinch: ">=0.2.0"` in `m` returns a clean `checkResult{}` (no findings).
- No further action needed; recorded here so the build order's provenance stays legible even
  though the fix landed out of band, before this document existed.
