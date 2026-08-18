# cinch — make coupling's comparison window a bound value

Status: **proposed** — not started.

## Context

From the "Checking the Checker" audit (2026-08-18), GAP-05 + CONTRA-03 + OPT-02.
**Correction to that report's own citation:** GAP-05 cited `README.md § Known
limitations` for coupling's window, but that section (README.md:481-495)
actually documents a different check's limitation — `generated`'s
path-migration detection being "bounded to one commit back" (README.md:185).
Verified against `coupling.go` directly instead: `checkCoupling`'s own
docstring states the real property — "its window is the working tree against
HEAD, so it is invisible post-commit (nothing changed, nothing to compare)."
That's not "one commit back," it's *zero* commits back — the check only ever
sees uncommitted, in-progress changes. The moment a rule/marker mismatch is
committed, `coupling` can never see it again, on any later commit.

The audit's CONTRA-03 names the sharper problem: principle 6 calls a permanent
"known warn" a failed check wearing a costume, and `coupling`'s designed
post-commit behavior is exactly that shape — a permanent, unclosable no-op for
every invocation after the first commit containing the drift, "defended by the
same careful wording ('a no-op, not a pass') principle 6 exists to make
readers distrust everywhere else." The README bullet for `coupling` (line
162-167) is honest about this ("Transition-scoped... so it's a no-op
post-commit... and says so on stderr rather than reporting a silent pass"),
which is real mitigation — but honesty about a permanent gap isn't the same as
closing it.

## Design

OPT-02's proposal: expose the comparison baseline as a manifest-bound value
instead of the hardcoded `HEAD` in `checkCoupling`'s `gitShow(repoRoot,
"HEAD:"+...)` call. Same mechanism (`git show <ref>:<path>` — already
ref-agnostic in the existing code, just always called with the literal string
`"HEAD"`), wider configurability — fits principle 5 (bind a value, don't grow
a checker category).

- New manifest key, e.g. `coupling.since` — a git revision expression
  (`HEAD`, a tag name, `HEAD~5`, anything `git show <ref>:<path>` accepts).
  Default, absent key: current behavior (`HEAD`) — no behavior change for
  every consumer that doesn't set it.
- This *does not* fully close CONTRA-03's structural point — coupling would
  still be blind between whatever `coupling.since` names and the working
  tree, just with a wider, project-chosen window (e.g. "since last release
  tag" catches drift accumulated across many small commits between releases,
  which the current HEAD-only window cannot). A project that wants stronger
  coverage sets a wider window and re-runs at wider intervals (a CI job
  against `coupling.since: <last-release-tag>`, say) rather than depending
  solely on the pre-commit invocation.
- Consider documenting, not building: the honest fix might be adding one
  sentence to the `coupling` README bullet naming the manifest key and the CI-
  job pattern, rather than assuming every consumer wants to configure this.
  Decide after drafting — this is a small enough change that over-engineering
  it (a new sub-command, say) would be the wrong shape for the actual gap.

## Steps

1. Add `coupling.since` as a recognized optional manifest key (absent →
   `"HEAD"`, unchanged behavior).
2. Thread it into `checkCoupling`'s `gitShow` call in place of the literal
   `"HEAD"` string.
3. Handle the failure mode: an invalid or unresolvable ref (typo'd tag,
   branch deleted) should be a named finding ("`coupling.since: <value>` does
   not resolve — check the manifest"), not a silent no-op indistinguishable
   from "nothing to compare."
4. Update the `coupling` bullet in `README.md` (lines 162-167) to name the new
   key and, briefly, the CI-job pattern for catching drift the pre-commit
   window alone can't.
5. Correct the audit's own miscitation while here: nothing in `README.md` §
   Known Limitations currently mentions `coupling` at all — if this plan adds
   the CI-job caveat to the `coupling` bullet instead, that's the more
   accurate home for it than Known Limitations, which is specifically about
   `paths.docs` migration.

## Verification

- Mutation fixture: a rule/marker mismatch committed at `coupling.since`'s
  named ref but not since — confirm it's caught when `coupling.since` points
  before the offending commit, silent (no-op, correctly) when it doesn't.
- `cinch check` with no `coupling.since` set behaves byte-identically to
  today (regression guard on the default).
- `go vet ./...`, `make test`.

### Critical files

- `internal/cinch/coupling.go` — `checkCoupling`, the `gitShow(repoRoot,
  "HEAD:"+...)` call site.
- `README.md` lines 162-167 (the `coupling` bullet).

### Relationship to other plans

Touches the same file as the `identity` checker (`coupling.go`'s git-walk
helpers), added by the now-landed rule-ID-monotonicity plan. That addition is
already in, so this plan modifies `coupling.go` on top of `identity.go`
existing rather than needing to sequence ahead of it.

New capability: on completion, mark status `complete` and keep the file — it
documents why the window is configurable rather than fully closed (the
CONTRA-03 structural point that no window choice eliminates).
