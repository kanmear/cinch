# Testing
<!-- harness-template v2 -->

## Commands

`make test` runs every suite: `go test -count=1` over every package except the gitignored
`/hidden` ones (below). `make check` is the static, read-only gate (go vet, gofmt) and runs no
tests. `TESTFLAGS` passes flags to `go test`: `make test TESTFLAGS='-run TestName -count=50'`.

| Suite | Where | Target | Tier |
|---|---|---|---|
| Checks, commands, rules | `internal/cinch`, `internal/mdscan`, `internal/output`, `internal/semver`, `main_test.go` | `make test` | unit |
| Benchmark harness code | `bench/*/main_test.go`, `bench/leak`, `bench/upkeep`, `bench/recover`, `bench/field` | `make test` | unit |
| Catch matrix (seeded defects) | `TestSeededDefect*` in `internal/cinch` | `make test`; `make catch-matrix` regenerates the table | unit |
| Cost curve, binding strength | `bench/costcurve`, `bench/binding` | `make cost-curve`, `make binding-strength` | benchmark run, not part of `make test` |
| Upkeep, leak and recover runs | `bench/upkeep`, `bench/leak`, `bench/recover` | `go run ./bench/<name>` per its README | benchmark run, model-driven and slow; never part of `make test` |

`TestCatchMatrixIsCurrent` fails `make test` when the committed `bench/catch-matrix.md` is stale:
run `make catch-matrix` and commit the result.

CI (`.github/workflows/test.yml`) runs build, `make check`, `go test -race ./...` and
`go run . check` from source.

## The `/hidden` exclusion

`bench/leak` keeps hidden grading tests of a private consumer repo under `tasks/*/hidden/` and
`private/tasks/T4/hidden/`. They are gitignored (publication is the owner's decision) and import
that consumer's backend module, so they only compile inside a copy of it. A plain `go test ./...`
or `go vet ./...` on a machine that has them fails on those packages, so `make test` and
`make check` list packages with `go list ./... | grep -v /hidden`, and gofmt takes only tracked files
(`git ls-files '*.go'`). A clean clone has no such packages and the filter changes nothing.

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
