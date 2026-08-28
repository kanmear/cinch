# 9. Calibrate every heuristic against real history before shipping it

**Tier 3 — decide before investing. Cost estimate: days.**

## Provenance

**Benchmark** (measured), applied prospectively — this item is a methodology constraint on
future work, specifically flagging staleness signals, single-binding warnings, and ownership
conflicts (the `owns:` machinery from item 6) as candidates that need it, since none of them
exist yet as shipped checks.

## What's actually being proposed

None of "staleness," "single-binding warning," or "ownership conflict" exist in the codebase
today — confirmed by grep: no match for `staleness`/`stale` anywhere in `internal/cinch/*.go`.
These are candidate heuristics raised elsewhere in this build order (staleness in
`FINDINGS.md` §4.4 — "staleness signal from git alone"; single-binding and ownership conflicts
as natural companions to item 4's binding-strength work and item 6's `owns:` machinery). This
item isn't a feature to build; it's a **gate every one of those heuristics must pass before
shipping**, and the mechanism for the gate.

## The mechanism: replay against `main`

For each candidate heuristic, before merging it as a live `cinch check` finding:

1. Implement it as a detector (not yet wired into `runChecks`'s launch list in `check.go`).
2. Walk every commit on `main` (`git log`, the same kind of git plumbing `retirement.go`
   already uses via `gitOutputLines`/`gitShow` — `git.go` presumably centralizes this; reuse
   it rather than writing new subprocess-calling code) and, at each commit, run the detector
   against the tree as it stood.
3. Count: how many commits would the heuristic have flagged? Of those, how many are
   legitimate, ordinary changes a human reviewer would not have wanted blocked or warned
   about — false positives from the detector's perspective, not from the codebase's.
4. Report a false-positive rate before anyone sees the check in a real `cinch check` run.

## Why this matters, in cinch's own terms

`checkResult` (`check.go:18-22`) has three states, not two: `findings` (real problems),
`noOp` (the check ran but had nothing to enforce — e.g. `checkPin`'s "require.cinch is not
set... opt-in, not configured", `pin.go:14-15`), and implicitly, "the check ran and is wrong."
cinch's existing checks are all deterministic and structural — a marker resolves or it
doesn't, a link target exists or it doesn't. A staleness signal is necessarily probabilistic
("this doc looks like it hasn't been touched since the code near it changed a suspicious
amount") and probabilistic heuristics have false-positive rates by nature. The roadmap's
framing is exact: "a check that flags 40% of legitimate commits gets disabled, and a disabled
check is worse than an unshipped one" — worse specifically because a disabled check still
occupies the mental slot of "this is handled," while doing nothing, which is a strictly worse
state than never having built it (nobody's relying on it, but everybody thinks they might be).

## Concrete candidates and what calibration would look like for each

- **Staleness** (`FINDINGS.md` §4.4, "staleness signal from git alone"): likely shape is "doc
  D references file F; F changed N commits ago; D hasn't changed since." Calibration question:
  what N, and what counts as "F changed" (any touch, or a touch to the specific lines/symbols
  D discusses)? Replay against `main` at several candidate thresholds and see which threshold
  produces a defensible flag rate — this is exactly the kind of parameter that can't be chosen
  a priori.
- **Single-binding warnings**: a rule with exactly one `// cinch:rule` marker might be
  under-enforced (only one code site knows about it) or might be completely correctly scoped
  (a rule that only ever applies in one place). Calibration: replay against `main`'s history of
  rules that started single-bound and stayed correct for a long time vs. ones that turned out
  to need a second binding — if most single-bound rules are fine forever, this heuristic's
  flag rate needs to be very conservative or it's noise.
- **Ownership conflicts** (once item 6's `owns:` ships): two docs both claiming `owns:` over
  overlapping paths. Likely lower false-positive risk than the other two (an actual conflict
  is closer to a structural fact than a judgment call), but still worth a replay pass to
  confirm real corpora don't have legitimate overlapping ownership this would misfire on.

## Why tier 3

Every one of these is speculative until calibrated — building the detector logic is not the
expensive part; discovering after shipping that it's noisy (and then having to walk it back,
which costs more trust than never shipping it) is. This is placed in "decide before investing"
because the decision isn't "should we build staleness detection," it's "we don't yet know if
staleness detection *can* be built at an acceptable false-positive rate, and the way to find
out is cheaper than shipping and finding out from users."

## Verification / definition of done

- Per-heuristic: a replay script (reusable — see if `retirement.go`'s per-commit `git show`
  pattern generalizes into a shared "walk main, run detector at each commit" harness worth
  factoring into `git.go`, rather than writing three bespoke replay scripts) run against
  `project_deltadocs`'s full `main` history (the corpus this whole build order is grounded in)
  and, ideally, one or two other real cinch-consuming repos if available, to avoid calibrating
  against a single codebase's idiosyncrasies.
- A written false-positive rate per heuristic, with the threshold(s) tested, before any of
  them is wired into `runChecks`'s launch list.
- A stated go/no-go bar (e.g., "under 5% of legitimate commits flagged" — pick a real number
  before running the replay, not after, to avoid rationalizing whatever number comes out).
