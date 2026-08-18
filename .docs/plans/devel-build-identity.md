# cinch — give `devel` builds a real identity

Status: **proposed** — not started.

## Context

From the "Checking the Checker" audit (2026-08-18), CONTRA-01 + OPT-01.

`pin.go`'s `checkPin` deliberately no-ops the `core` check whenever the running
binary's `Version` is `"devel"`: "an unreleased (devel) binary skips the
comparison outright: a local dev build must never redden a consumer that pins a
release." That's the right call for the case it names (an iterating cinch
developer's `go build` shouldn't fail *other* people's pinned consumers) — but
it means a stale `bin/cinch` on the machine actually iterating on cinch itself
can pass `require.cinch` against any pin, including a wrong one, by definition
alone. Combined with the README's own admission (§ Install) that a stale
installed binary "re-renders stale templates and the `generated` check reports
a mismatch that isn't really there," this is cinch's own version of a
false-green state — the exact shape the tool exists to catch elsewhere.

The audience most exposed is precisely the audience most likely to hit it: a
cinch contributor running `make build` (→ `bin/cinch`, `Version=devel`) instead
of `make install`, checking a consumer or cinch's own self-hosted tree, and
seeing green from a binary several commits stale.

## Design

Stamp a short git SHA into `devel`-tier builds too, not just released ones, and
let a binary compare its own build SHA against the working tree's current HEAD
when both are knowable — a strictly local, opt-in signal, not a new manifest
key aimed at consumers (`require.cinch` stays a *version* pin; this is a
same-repo staleness check, a different property).

- `Makefile`'s `LDFLAGS` already stamps `main.Version`. Add a second `-X` for a
  short SHA (`git rev-parse --short HEAD`, empty string if not in a git repo —
  `make build`/`make install` both already run inside the cinch repo, so this
  is always available at build time there).
- `cinch version` prints the SHA alongside `devel` when set: `devel (a1b2c3d)`
  instead of bare `devel`.
- Scope this to self-hosting only: when cinch is run against its *own* repo
  (the tree `go.mod`/`main.go` live in — cinch already self-hosts its own
  `cinch check`), a `devel` binary whose stamped SHA doesn't match the current
  `git rev-parse HEAD` is a `core` finding: "cinch binary built from a stale
  checkout (<SHA>) — run `make install` before trusting this check." Do **not**
  extend this to arbitrary consumers: a `devel` binary is legitimately used
  against any repo during cinch development, and only cinch's own tree can
  know "the SHA I was built from" should equal "the SHA I'm checking."

This directly closes CONTRA-01 without touching `checkPin`'s existing consumer-
facing logic at all — it's a new, narrower check, not a change to the current
one's semantics.

## Steps

1. Add the short-SHA `-X main.BuildSHA=$(shell git rev-parse --short HEAD
   2>/dev/null)` to `Makefile`'s `LDFLAGS` (guard the shell call so a
   non-git tarball build doesn't fail — empty string is a valid "unknown"
   value).
2. Thread `BuildSHA` through the same path `Version` already takes into
   `internal/cinch` (see `pin.go`'s comment on why `main()` sets `pin.Version`
   before dispatch — `BuildSHA` needs the same treatment).
3. `cinch version` output gains the SHA when present and `Version == "devel"`.
4. Add the self-hosting staleness check described above, gated on running
   inside cinch's own repo (detect via `go.mod`'s module name, or a marker
   file — decide during implementation which is more robust against a
   renamed fork).
5. Update `README.md` § Install to mention the new signal as the concrete
   mitigation for the paragraph that currently only says "re-install after
   changing cinch itself, too."

## Verification

- `make build && ./bin/cinch version` shows `devel (<short-sha>)` matching
  `git rev-parse --short HEAD`.
- Mutation fixture (principle 1): stamp a binary with a SHA one commit stale
  against a synthetic HEAD, confirm the new finding fires; confirm it's silent
  when the stamped SHA matches.
- Confirm the existing `checkPin` consumer-facing behavior (release version
  mismatch, opt-in absence) is unchanged — this plan adds a check, it doesn't
  touch that one.

### Critical files

- `Makefile` — `LDFLAGS`.
- `main.go` — `Version` var and dispatch, needs a `BuildSHA` sibling.
- `internal/cinch/pin.go` — where the analogous `Version` package var and
  `checkPin` logic live; the new check is a sibling, not a modification.
- `README.md` § Install.

### Relationship to other plans

Independent. Narrower than it might sound — deliberately does not attempt a
general provenance/lockfile system (that was `agent-harness-design-notes.md`'s
proposal, and STR-06 in the audit specifically credits cinch for *not*
building that).

New capability: on completion, mark status `complete` and keep the file — the
scoping decision (self-hosting only, not a consumer-facing feature) is worth
keeping as a record against a future temptation to generalize it.
