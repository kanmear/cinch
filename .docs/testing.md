# Testing
<!-- harness-template v2 -->

## Commands

`make test` runs every suite: `go test -count=1` over every package. `make check` is the static, read-only gate (go vet, gofmt) and runs no
tests. `TESTFLAGS` passes flags to `go test`: `make test TESTFLAGS='-run TestName -count=50'`.

| Suite | Where | Target | Tier |
|---|---|---|---|
| Checks, commands, rules | `internal/cinch`, `internal/mdscan`, `internal/output`, `internal/semver`, `main_test.go` | `make test` | unit |

CI (`.github/workflows/test.yml`) runs build, `make check`, `go test -race ./...` and
`go run . check` from source.

## Test Structure

Table tests with `cases` and `t.Run(name, ...)`; scratch git repos come from the helper in
`internal/cinch` (`gitTestHelper`), so new tests call it instead of writing another. Style and
helpers are in [Stack conventions](stack-conventions.md#tests). Look at `internal/cinch/archive_test.go`
for a command test and `internal/cinch/links_test.go` for a check test.

## Rule Markers

A numbered rule in [Rules](rules.md) is bound to the test that enforces it by a comment directly
above the assertion: `// cinch:rule <ID>`. `cinch check` picks the marker up in source files with no
extra config, and fails if a rule has neither a marker nor an explicit `cinch:ignore: <reason>` in
its doc (for a negative property or a process rule). Test files that quote marker-shaped text as
fixtures are listed under `paths.exclude` in `cinch.yml`, so they bind nothing.

## Manual Verification

See [Manual checks](manual-checks.md).
