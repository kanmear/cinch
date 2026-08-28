# 2. `cinch upgrade`

**Tier 1 — trust the checker. Cost estimate: an afternoon.**

## Provenance

Audit finding **F5**, corroborated by the benchmark: `~/code/cinch_bench/lib.sh`'s
provisioning step has to run `cinch render` and bump the pin by hand in *every* freshly
cloned tree before the baseline `cinch check` goes green — the exact ritual this item
automates, done manually 84+ times to keep the benchmark's control valid.

## What's wrong

Bumping a consumer to a new cinch release is four manual steps, spread across two files, with
nothing that prompts any of them:

1. Reinstall the `cinch` binary.
2. Run `cinch render` (`internal/cinch/render.go`, `CmdRender`) to regenerate
   `.docs/workflows/*.md` and the hook shims from the newly embedded templates.
3. Confirm `require.cinch` in `cinch.yml` is satisfied — `render` already calls
   `syncRequireCinch` at the end of `CmdRender`, which auto-bumps an **exact** pin
   (`require.cinch: 0.2.0`) to the installed version, but leaves `>=` ranges and unset pins
   untouched by design (`render.go:171-211`). A range pin that's gone stale relative to a
   *minimum* the consumer actually wants raised has no automated path.
4. Review and commit the diff `render` produced.

Nothing in cinch invites a consumer to do any of this. Both of the audit's real-world
consumers sat red for three releases as a result — not because of a real incompatibility, but
because nobody ran the ritual. A checker that's red for reasons the user didn't cause is a
checker people learn to skip, and the benchmark's finding sharpens this: a real defect (the
T3 injected bug) sat in the same noisy `cinch check` output as unrelated pin staleness, in
every provisioned tree, until `lib.sh` was patched to force the render+bump step during setup.
If a *benchmark harness* needed to special-case this, real consumers need the tool to do it
for them.

## Current building blocks (already correct, just not wired to one command)

- `CmdRender(root)` (`render.go:134-166`) — regenerates every template under
  `docs/templates` plus `docs-philosophy.md` plus the three hook shims
  (`pre-commit`/`commit-msg`/`post-commit`), and already calls `syncRequireCinch` at the end.
- `syncRequireCinch(root)` (`render.go:171-211`) — walks the YAML AST for `require.cinch`,
  patches only an **exact** pin in place (preserving quote style via `patchScalarLine`), and
  is a no-op for `>=` ranges, unset pins, or a `dev`-Version binary. This is the right
  behavior for `render` itself (it shouldn't silently loosen a range someone chose
  deliberately) — `upgrade` needs to layer on top of it, not replace it.
- `checkPin` (`pin.go`) — already tells the consumer exactly what's wrong
  ("installed cinch %s does not satisfy require.cinch >=%s — reinstall and re-run
  cinch render") when a range pin fails, and is a no-op when `require.cinch` isn't set at
  all (opt-in, not configured).

## Proposed shape

A new `cinch upgrade` subcommand (wire into `main.go`'s switch alongside `render`, `check`,
`init`) that:

1. Reports the currently installed `Version` and, if reachable, what's newer (scope
   decision: comparing against a remote/release feed is optional for v1; even a purely local
   "you're on X, here's what render+pin-sync would change" pass is the whole value of this
   item — see Scope below).
2. Runs the render step (reuse `renderAll` + the file-write loop from `CmdRender`, not a
   shelled-out call to itself) and captures **before/after diffs** per rendered file instead
   of only printing `output.Step("rendered %s", ...)` — the audit's core ask was visibility
   into *what changed*, not just that something did.
3. Runs `syncRequireCinch` (already does the right thing for exact pins) and, when the pin is
   a `>=` range that's already satisfied, prints a note rather than staying silent — "range
   pin >=X.Y.Z is satisfied by the installed A.B.C; no change" — so the "nothing happened"
   case is legible, not just "nothing printed."
4. Ends by running the same check set `preCommitChecks` runs (`links`, `rules`, `retirement`,
   `generated`, `core`) so the operator sees green (or a real, non-pin-related red) in the
   same invocation, instead of running `cinch upgrade` and then separately wondering whether
   `cinch check` will now pass.

## Scope decision to make before implementing

Whether `cinch upgrade` needs network access (checking a release feed for "is a newer cinch
available") or is purely local (assume the binary on `PATH` is already the target version,
just make the *consumer repo* consistent with it). The audit's F5 and the benchmark's
provisioning pain are both about the **local** half — a repo that has the right binary
installed but a stale rendered/pinned state. Recommend building the local half first; it's
the whole afternoon-sized estimate, and it's the part that was actually measured. Network
awareness can follow if it turns out consumers also forget to reinstall the binary itself.

## Verification

- New integration-style test (mirroring `render_test.go`'s pattern): a fixture tree with a
  stale exact `require.cinch` pin and a template variable that would change under render;
  assert `CmdUpgrade` leaves `cinch check`'s `core`, `generated`, and `rules` checks green
  afterward.
- Manual: reproduce the benchmark's exact failure — clone `project_deltadocs` at its pinned
  SHA with the historical (pre-hotfix-aware) `require.cinch: 0.2.0` and four stale rendered
  workflows, run `cinch upgrade`, confirm `cinch check` goes green in one command instead of
  the three `lib.sh` currently runs by hand (`cinch render` + manual pin bump + `git config
  core.hooksPath`).
