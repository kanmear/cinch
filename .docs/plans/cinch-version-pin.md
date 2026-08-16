# cinch — `version` command + optional core-version pin

Status: **partially shipped** — Step 1 (`cinch version` command, scope 2a)
landed, `bf4ef96`. Step 2 (`require.cinch` pin enforced by `cinch check`) was
deliberately deferred, not forgotten: the user chose scope "2a, version
command only" over "2b, also verify the pin in `cinch check`". Reopen this
plan (Step 2 onward) if/when the consumer-side pin should become enforced
rather than a human-checkable convention.

## Context

`project_deltadocs` "pins" the cinch core with prose it can't enforce:
`.agent/manifest.yml:2` reads `harness_version: 0.1.0 # pinned core version;
bump -> re-render -> cinch check`, and the Makefile comment repeats the claim —
but `cinch version` is not a command, and nothing compares the installed binary
against any declared version. The README's "Version-skew note" already concedes
there's no pin or warn: upgrading cinch reddens every consumer's `generated`
check until `cinch render` is re-run, and there's no way to *know* that's what
happened vs. a real tamper.

Two separate gaps:
1. **No version command.** A consumer can't even ask the binary what it is.
2. **The consumer's pin is dead prose.** Nothing verifies installed-version vs
   declared-version, so "bump -> re-render -> cinch check" is a hope, not a
   property.

## Decision needed — scope

2a. **`cinch version` only** (small). The consumer's prose pin stays prose but
becomes *checkable by a human/agent*: they can run `cinch version` and compare
to `harness_version` themselves.

2b. **Also verify the pin in `cinch check`** (the real change). Add an optional,
absence-based manifest key that `check` compares against the binary, turning the
version-skew noise into a *decidable red state* — aligned with principles 1/6
(a real failing state, never a proxy). This is the reasoning that previously
rejected a *fuzzy warn* (flag 3 in the 0.1.0 plan, deleted in `0ed1ebd`); the
difference is this is
a *hard* check with an exact close action ("run `cinch render`"), not a warn
whose green state is mushy.

How the binary knows its own version:
- **A (recommended): build-time `ldflags` injection.** `main` reads a package
  string (default `"dev"`/`devel`), `make install`/`build` set
  `-X cinch/internal/...=0.1.0`. Pro: exact, no VCS dependency at runtime.
  Con: `go install .` without the flag stamps `dev` unless the Makefile always
  sets it.
- **B: VCS at runtime** (`git describe` on the cinch repo — but cinch is
  typically *installed* to GOBIN, not run from its repo, so VCS lookup fails in
  the common case). Not viable as the primary path.

Recommend **A**, with the Makefile (`build`/`install`) always passing the
version so `go install .`-style installs carry it. `harness_version: 0.1.0 in
the consumer already matches the semantic this encodes.

## Decision needed — key name & failure semantics

- Key name: the repo renamed `cinch_manifest` → `cinch.yml`; propose
  `require.cinch = 0.1.0` (absence-based — with no key, no behavior change at
  all; with `devel`/mismatch, a finding). Alternative `core.version`.
- Failure level: **error** (it's decidable and fixable) vs **warn**. Error,
  for the reason above. The green state is reachable and the action exact.
- Semver compare: exact-string or `x.y.z` prefix/semver precedence? Recommend a
  plain semver compare where mismatch fires; `dev`/`devel` treated as
  "unreleased — skip the pin check" so a local dev binary doesn't redden a
  consumer that pins a release. This last choice is a **judgment call to
  confirm**: should an unversioned dev binary respect the pin or bypass it?

---

## Step 1 — cinch: `version` command

- Add `Version` string (package var / `const` default `"devel"`), settable via
  `-ldflags "-X …Version=0.1.0"`.
- `main.go`: `cinch version` prints it plain and exits 0 (no arguments).
- `Makefile`: `/bin` and `install` recipes pass the ldflag (single place to
  bump the version — `VERSION ?= 0.1.0`).

### Tests

- CLI test `TestVersion_PrintsAndExitsZero`. Unit test baked under the ldflag
  path is impractical (ldflags are a build-time string) — assert the command
  shape and the `devel` default.

## Step 2 — cinch check: optional `require.cinch` pin

`internal/cinch/manifest.go` / `check.go`: read `require.cinch`; if absent, a
declared no-op (same contract as `commit.pattern` — absence changes nothing,
and the no-op is *stated*, not silent). If present:

- `devel`/unreleased binary → no-op (per decision above) or error — confirm.
- semver mismatch → `Finding{Check: "core", Level: "error"}` with the exact
  close action: "run `make install`/`cinch render` per the version note".

Wire into `CmdCheck`. Keep the `commit`/`generated`-check one-line-per-check
reporting style.

### Tests — mutation fixtures

- `TestCheckPin_DeclaredMatches → clean`.
- `TestCheckPin_DeclaredMismatch → error`.
- `TestCheckPin_Absent → stated no-op, not a pass`.
- `TestCheckPin_DevelSkips` (if that decision holds).

## Step 3 — consumer: make the pin real

1. Add `require.cinch = 0.1.0` to `project_deltadocs/cinch.yml` (replacing the
   `manifest.yml` prose `harness_version`, or keeping it as documentation).
2. Update `.agent/manifest.yml:2` + Makefile comment to say the property is now
   *enforced* by `cinch check`, not a hope.
3. Confirm `require.cinch` matches the installed binary; `cinch check` clean.

---

## Verification

1. cinch repo: `make test` green incl. new pin fixtures (hand-verify the
   mismatch fixture genuinely fails, i.e. revert compare → fixture red).
2. Consumer scratch branch: add `require.cinch = 0.1.0`, check clean; bump the
   value to `0.2.0` → check errors naming the pin and the fix; remove key →
   stated no-op.
3. README: `version` in the usage block; the version-skew note now points at
   the pin as the consumer-side answer.

### Critical files

- `internal/cinch/*.go` (version + manifest pin)
- `main.go`
- `Makefile` (`VERSION`, ldflags)
- `tests/cli_test.go`
- `project_deltadocs/cinch.yml`
- `project_deltadocs/.agent/manifest.yml`
- `README.md`

### Relationship to other plans

Independent. Consumes the same manifest accessors as other `cinch.yml` work;
sequence alongside `hook-commit-msg-args.md` on build-cleanliness grounds (each
plan's `make test` must stay green).
