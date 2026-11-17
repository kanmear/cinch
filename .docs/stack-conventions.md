# Go CLI Conventions
<!-- harness-template v2 -->

How a Go command-line tool is written and released. Read this before touching Go code. The shape
comes from cinch (a published tool: a thin `main.go`, features under `internal/`, goreleaser).
Where this file and the code disagree, the
code is the bug or this file is stale; say which in your handoff, do not guess.

## Layout

```
main.go              the command switch: parse, call one command, exit with its code
internal/<tool>/     the features: one file per command or check, with its _test.go beside it
internal/<name>/     pure, silent mechanism (path, semver, git helpers, output formatting)
```

- `main.go` only dispatches. It holds `usage`, a `switch os.Args[1]`, and `var Version = "dev"`,
  which the build overwrites (see Release). Logic lives in the packages.
- A file belongs in the feature package if it implements a command or any other behavior a user can
  observe. It belongs in its own sibling package if it is pure, silent mechanism: no dependency on
  the feature package's types and no output of its own.
- A private tool whose packages another repo will import keeps them at the root instead of
  under `internal/`. Pick one layout when
  the repo starts and say which in the architecture doc.
- One executable per repo. A second binary means a second `cmd/<name>/` and a second goreleaser
  build, which is a decision for the architecture doc.

## Commands, output and exit codes

- A command is a function that takes its arguments and returns an exit code or an error; `main`
  turns that into `os.Exit`. Nothing below `main` calls `os.Exit`.
- Exit codes: `0` ok, `1` a failure or findings, `2` a usage error (a bad flag, a missing
  argument, an unknown command). The usage goes to stderr.
- stdout carries the command's result and nothing else, so it can be piped; progress, warnings and
  errors go to stderr. A message names the command: `<binary> <command>: <what went wrong>`.
- A command that needs the network or a model takes the client as a dependency (an interface or a
  function value), so its tests inject a fake.
- `<binary> version` prints `<name> <version>`.

## Config

A file or the environment, read in one place (`config`), validated at start, with defaults in one
line each. A secret has no default and is never logged. A command that needs no config never loads
it, so `version` and `--help` work on a bare machine.

## Errors

Callers branch on sentinel or typed errors with `errors.Is` and `errors.As`; everything else is
wrapped with `fmt.Errorf("...: %w", err)` and returned. Use `err.Error()` or `%s` only when
embedding into a plain string field. Only `main` decides the exit code.

## Tests

Stdlib `testing`, no assertion library. `make test` runs everything (`TESTFLAGS` passes flags to
`go test`).

- Table tests use `cases`, each element `tc`, each run in `t.Run(name, ...)`, even for a short
  table; assertions are `got` and `want`. A bare loop hides which case failed.
- Use `t.TempDir()` and a scratch git repo helper (one per package, not one per test file) for
  anything that touches the filesystem or git; call the helper that exists instead of writing a new
  one. Use `t.Context()`, never `context.Background()`.
- A test that drives a command captures stdout, stderr and the exit code and asserts on all three.
- No test calls the network or a real model: `httptest` servers or injected fakes. A live call is a
  manual step in a plan's Verify.
- New checks and features ship with tests. Backfilling old gaps is tracked separately and is not a
  license to skip tests on new code.
- Rule markers go on the line directly above the assertion they enforce (the form is in the repo's
  testing doc); a regression test names the rule it protects.

## Release

The version is the root `VERSION` file (`x.y.z`), kept by `scripts/version.sh`; there is no
generated Go file. Both builds stamp it into `main.Version`:

- `make build` runs `go build -ldflags "-s -w -X main.Version=$(VERSION)"` into `BIN_DIR`.
- `make ship` is `make release`: it tags `v<version>` on `main` and pushes `main` and the tag. The
  tag push runs `.github/workflows/release.yml`, which runs goreleaser (`.goreleaser.yml`): static
  binaries (`CGO_ENABLED=0`) for linux, darwin and windows, archives named
  `<name>_<version>_<os>_<arch>`, `checksums.txt`, and a changelog from the commit subjects. The
  workflow needs `contents: write` and the default `GITHUB_TOKEN`; it has no other secret.
- A release is the user's (see AGENTS.md). Test a config change with
  `goreleaser release --snapshot --clean` before tagging; nothing is published by it.
- `go.mod` carries the Go version CI builds with (`go-version-file`), so bump it there, not in the
  workflow.

## Style

- **Exports.** Export an identifier (type, func, var, const, field) only if something outside its
  package needs it. Check concretely before exporting. An unexported type gets unexported fields.
- **Struct placement.** Types are declared together near the top of the file, after package-level
  `const`/`var` blocks, not interleaved with the functions that use them. Field order: identity or
  classification, then location (file, line), then payload, then optional flags.
- **Paths.** A parameter holding the repo root is always `repoRoot`; the docs root is always
  `docsRoot`; never a bare `root`. When a function takes both the order is `(repoRoot, docsRoot)`.
- **Spelled-out names.** `buffer`, `temp`, `lineNumber`, not `buf`, `tmp`, `lineNo`. If the codebase
  already has the full word for a concept (`message`, `path`, `pattern`, `result`, `report`,
  `directory`), a clipped local matches it. The short idioms stay: `err`, `ok`, `i`/`j`/`n`,
  single-letter receivers and `r`/`w`/`f`/`d` params, `fn`, `a`/`b` in comparators, `re`, `b` for a
  `strings.Builder`.
- **`Error` suffix, not `Err`,** for functions named after an error-shaped thing
  (`output.UsageError`). `err` stays the local variable name; sentinels are `ErrX`.
- **`checkX` functions** are the entry point for one check. Do not reuse a check's name for an
  unrelated thing.
- **Imports.** One grouped `import (...)` block per file: stdlib, blank line, third-party and local.
  Never a bare `import "..."`.
- **Declarations.** Group two or more package-level declarations of the same kind into one `var (...)`
  or `const (...)` block. Package-level `regexp.MustCompile` for a fixed pattern; inline
  `regexp.Compile` for a runtime pattern. `fmt.Sprintf` over `+` once more than one value is
  interpolated.
- **Comments.** A comment is only a why, an invariant, or a rule pointer. No comment that restates
  the code. Doc comments are optional on exported identifiers; add one only when the why is not
  obvious from the signature.
- **File organization.** Group by feature, not one function per file. A generic unrelated helper goes
  in the package that owns that concern, not the file of its first caller.
- **Quoting.** Single quotes around command references inside user-facing messages: `'make check'`.
- **Dependencies.** No new Go module without explicit user approval. Stdlib and the repo's current
  `go.mod` are the baseline.
