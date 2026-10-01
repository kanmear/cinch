# 12. Skip directory entries (submodule gitlinks) in the primary marker scan

**Tier 1 — trust the checker. Cost estimate: ~10 lines + a test.**

## Provenance

**External consumer report**, same source as item 11: converting `fsed-odin-docs` (the
sibling-repo split's docs/harness repo) to use real git submodules for its three nested service
repos, rather than plain gitignored directories, surfaced a latent bug in the `rules` check's
primary marker scan.

## Confirmed directly in this repo

`markerScanFiles`'s `gitutil.ScannableFiles` branch (`internal/cinch/rules.go:267-276`, pre-fix)
took every path `git ls-files` reported and opened it as a file:

```go
if files, err := gitutil.ScannableFiles(root.path); err == nil {
    var out []scannedFile
    for _, f := range files {
        sf := toScanned(f)
        if !underPath(sf.fsPath, docsRoot) {
            out = append(out, sf)
        }
    }
    return out, nil
}
```

`git ls-files` lists a submodule as a single gitlink entry (mode `160000`) — a path that is
actually a **directory** on disk, not a blob. `scanFileMarkers` (same file) then does a bare
`os.Open(path)` and reads it as a file; `open()` on a directory succeeds on Linux, but the first
`read()` returns `EISDIR`, surfacing as a scan error. Reproduced directly: a fixture repo with a
real `git submodule add`ed directory containing its own `// cinch:rule` marker made
`scanRuleMarkers` return `read .../sub: is a directory` before the fix, confirmed by temporarily
reverting the fix and re-running the new test (`TestMarkerScanSkipsSubmoduleGitlink`).

This never surfaced before item 11 landed, because a gitignored path never appears in `git
ls-files` output at all — only a *tracked* directory-shaped entry (a submodule gitlink) does, and
nothing in this codebase or its consumers used real submodules until now.

## Proposed shape (shipped)

Skip any candidate whose resolved filesystem path is a directory, right where `underPath` is
already checked — one `os.Lstat` + `IsDir()` guard:

```go
if files, err := gitutil.ScannableFiles(root.path); err == nil {
    var out []scannedFile
    for _, f := range files {
        sf := toScanned(f)
        if underPath(sf.fsPath, docsRoot) {
            continue
        }
        if info, statErr := os.Lstat(sf.fsPath); statErr == nil && info.IsDir() {
            continue
        }
        out = append(out, sf)
    }
    return out, nil
}
```

The `filepath.WalkDir` fallback branch (used only when `root.path` isn't a git repo) needs no
change — it already branches on `d.IsDir()` and recurses into directories rather than treating
them as files, so a submodule's working tree there is walked normally via the filesystem
(redundant with `rules.roots` but harmless, not a bug).

This is a general robustness fix, not submodule-specific plumbing: any git-tracked path that
resolves to a directory on disk (a submodule, or a hypothetical future tracked-directory
construct) hits the same failure mode.

## Why tier 1

Same reasoning as item 11: this is about making `rules` — "exercised daily and is solid"
(00-overview.md) — actually solid for a shape of repo (one using submodules) that a consumer
just adopted. Purely subtractive (skips a candidate that was always nonsensical to open as a
file), zero behavior change for any consumer without submodules, full existing suite passes
unmodified.

## Verification

- `go test ./...` — full suite green, including the pre-existing battery unmodified.
- New test `TestMarkerScanSkipsSubmoduleGitlink` (`rules_test.go`): a real `git submodule add`ed
  fixture with its own marker — confirms the primary scan no longer errors and doesn't see the
  submodule's marker (by design — only `rules.roots` reaches into it), and a second assertion in
  the same test confirms `rules.roots: [sub]` *does* find it, so the two scans compose correctly.
- Confirmed the test actually catches the bug: reverted the fix, re-ran the test, got the exact
  predicted failure (`read .../sub: is a directory`); restored the fix, test passes.
- End-to-end against the real consumer: `fsed-odin-docs`'s `cinch check` (`rules` check
  specifically) passes cleanly once its backend/frontend/mobile nested repos become real
  submodules and this fix is installed.
