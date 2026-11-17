# Architecture
<!-- harness-template v2 -->

## System Overview

cinch is one Go binary (`main.go` at the repo root, module `cinch`) with no server, no database and
no network use beyond the self-update lookup in `selfupdate.go`. It reads a consumer repo's docs,
`cinch.yml` and source markers, and reports findings; the same binary renders the workflows and the
git-hook shims into a consumer and runs as the hook dispatcher those shims call.

| Part | Where | What it holds |
|---|---|---|
| Command switch | `main.go` | usage text, `switch os.Args[1]`, `var Version = "dev"` (stamped by `-ldflags`); logic is not here |
| Features | `internal/cinch/` | every command and check: `check.go` (the check runner and the staged-index snapshot in `staged.go`), `links.go`, `rules.go`, `plans.go`, `checks_misc.go` (commit, core, hooks), `render.go`, `init.go`, `upgrade.go`, `hook.go`, `impact.go`, `move.go`, `archive.go`, `selfupdate.go` |
| Embedded content | `internal/cinch/docs/` | the workflow templates (`templates/`) and the docs philosophy that `cinch render` writes into a consumer; product content, not this repo's own docs |
| Mechanism | `internal/gitutil`, `internal/mdscan`, `internal/semver`, `internal/concurrency`, `internal/output` | pure, silent helpers with no dependency on `cinch`'s types (`output` is terminal formatting and error printing) |
| Benchmarks | `bench/` | one directory per claim cinch makes (`catchmatrix`, `costcurve`, `binding`, `upkeep`, `leak`, `recover`, `field`, `drift`); their own `main` packages, results committed beside them. Read `bench/README.md` first |
| Release | `.goreleaser.yml`, `.github/workflows/` | goreleaser builds static binaries when a `v<version>` tag is pushed; `test.yml` runs build, vet, tests and `go run . check` |

`internal/cinch` is consumed only by `main.go`; the package-boundary rule is in
[Conventions](conventions.md#package-boundaries). What cinch guarantees about itself is in
[Rules](rules.md) (`CINCH-001` and on), each bound to a test.

## How checks, hooks and render fit

- `cinch check` runs the checks (links, rules, index, plans, generated, commit, core, hooks) over
  the docs root, the source tree and `cinch.yml`. `--staged` checks the index snapshot, which is what
  the pre-commit hook uses.
- `cinch render` writes `.docs/workflows/*` from the embedded templates and `.githooks/*` shims,
  each stamped as generated; the `generated` check fails on a hand-edited copy. `cinch init` does a
  first render, sets `core.hooksPath` and adds the AGENTS.md pointer; `cinch upgrade` re-renders
  after a reinstall.
- Each shim execs the installed `cinch hook <event>`. cinch runs its own checks for the event, then
  the scripts registered under `hooks.*` in `cinch.yml` (here `make check`, the squash-only test
  run, and the post-commit version bump).
- cinch checks itself: this repo's `cinch.yml` and `.docs/` are the proof, and `go run . check` runs
  them from source in CI. The hooks run the installed binary, so `make install-bin` after changing
  checks or templates.

## Environment

No ports and no environment variables of its own. `cinch.yml` holds the repo's settings; the Makefile
parameter block holds the binary name and `BIN_DIR`.
