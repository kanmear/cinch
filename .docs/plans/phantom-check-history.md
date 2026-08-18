# cinch — commit-keyed phantom-check history

Status: **proposed** — not started.

## Context

Started from a much broader ask: track each check's green/red rate per
project. That framing was rejected on the merits during design, not on
authority of `PRINCIPLES.md`: a per-invocation ledger is dominated by
dev-loop iteration noise (fixing one broken link and re-running `cinch
check` ten times looks like "10% green"), and raw red-rate is a weak proxy
for a check's value anyway — a check that's always green might simply be
doing its job.

One narrower question survived that critique: has a given check *ever*
fired on a real commit in this project? That's answerable from local
history without the noise problem, if the record is keyed by commit SHA
(one record per real commit) instead of by invocation — a commit happens
once, no matter how many times someone ran `cinch check` getting there. A
check with zero red records across a real sample of commits is a phantom
candidate: dead weight, or a checker that structurally can't fire.

Independently cross-checked against a second critique of the same
original proposal (session notes: `~/Documents/cinch/run-log-discussion.md`)
that reached the same rejection of a rate ledger via a different route,
and additionally argued for two refinements adopted here: storing the log
outside the repo (survives reclones; removes the `.gitignore` question
entirely) and treating disagreement between independently recorded runs
at the same commit as a passive flakiness signal, rather than discarding
it as a duplicate. That second critique's "record every run including
dirty, filter later" shape was deliberately **not** adopted — filtering
dirty rows out downstream is a convention a future query can forget;
this design keeps the harder invariant that a dirty-tree run is never
written at all.

Companion plan `.docs/plans/check-twice.md` (`cinch check --twice`, a
same-tree double-run determinism probe) shares this plan's `runChecks`
refactor and its `recordCommitCheck` write path — sequence this plan
first.

## Design

**Data model** (`internal/cinch/check.go`) — one struct for both printing
and storage:

```go
// checkTally is one check's outcome from a single runChecks pass. NoOp is
// print-only (json:"-") — a skipped check doesn't count toward sample
// size, so its reason isn't worth persisting.
type checkTally struct {
    Name    string   `json:"name"`
    Status  string   `json:"status"`            // "green" | "red" | "skip"
    Count   int      `json:"count,omitempty"`   // red only
    Reasons []string `json:"reasons,omitempty"` // red only — Finding.Message values
    NoOp    string   `json:"-"`
}
```

Same green/red/skip precedence `output.CheckStatus` already uses (noOp set
→ skip; else findings → red; else green).

**Storage** (`internal/cinch/phantoms.go`, new) — global, outside the
repo: `$XDG_DATA_HOME/cinch/runs/<key>.jsonl`, falling back to
`~/.local/share/cinch/runs/<key>.jsonl`. `<key>` is the first 16 hex chars
of `sha256(EvalSymlinks(filepath.Abs(root)))` — stable per checkout,
filesystem-safe. Accepted limitation: this key is path-based, not
identity-based, so a second clone or a `git worktree` of the same repo
starts a fresh history rather than merging with the original. A
content-based identity (the repo's root commit) would survive that but
adds real complexity — shallow clones, squashed histories, a repo with no
commits yet — for a benefit that doesn't matter much to one person on one
machine, which is what this is for. Not building that now.

Format: JSONL, one `commitRecord{SHA, Time, Checks []checkTally}` per
line, appended via `O_APPEND|O_CREATE|O_WRONLY`.

Write gate — this is what fixes the noise problem structurally rather
than by convention. `recordCommitCheck(root string, tallies []checkTally)`
only writes when the working tree is verifiably identical to a real
commit:

```go
func recordCommitCheck(root string, tallies []checkTally) {
    if !isGitRepo(root) || !hasHead(root) {
        return // no git, or no commit yet — nothing to key against
    }
    changed, err := gitChangedFiles(root) // already exists, coupling.go:174
    if err != nil || len(changed) > 0 {
        return // dirty tree mid-edit — this run isn't "commit X", stay silent
    }
    sha, err := gitOutputLines(root, "rev-parse", "HEAD") // already exists, coupling.go:198
    if err != nil || len(sha) != 1 {
        return
    }
    path, err := phantomsStorePath(root)
    if err != nil {
        output.Skip("check", "phantoms", "could not resolve history path: "+err.Error())
        return
    }
    // MkdirAll(filepath.Dir(path)), marshal commitRecord{sha[0], time.Now(), tallies}, append.
    // A write failure surfaces once via output.Skip; the function returns
    // void, so it can never affect CmdCheck's exit code.
}
```

Reuses `isGitRepo`, `hasHead`, `gitChangedFiles`, `gitOutputLines` —
already implemented in `coupling.go` for the coupling checker. No new git
plumbing.

No check-before-write dedup, and multiple records per SHA are kept, not
collapsed at write time. Two *agreeing* records for the same SHA are
redundant (checks are deterministic); two *disagreeing* records for the
same SHA — one run on this machine, another in CI, both against the
identical clean commit — are exactly the flakiness signal the reader
surfaces below. Discarding repeats at write time would throw that away.

**Read/report path — `cinch phantoms`:**

```go
const phantomsMinSample = 20 // commits before "never fired" reads as signal, not luck

type checkTallySummary struct {
    Commits, Red, Green, Skip int // Commits = Green+Red; skip excluded from sample size
}

// flakyCandidate is one check whose recorded outcome disagreed across two
// or more runs against the identical clean commit.
type flakyCandidate struct {
    SHA   string
    Check string
    Seen  []checkTally
}

// readCommitChecks groups records by SHA. A SHA with one record, or
// multiple records that agree per check, tallies normally. A SHA with
// disagreeing records for a given check becomes a flakyCandidate instead
// and is excluded from that check's summary — unsettled, not a data
// point either way.
func readCommitChecks(root string) (commits int, order []string, summaries map[string]*checkTallySummary, flaky []flakyCandidate, err error)

func CmdPhantoms(root string) int // always 0 — "not a check", same contract as CmdIgnores
```

Output:

```
$ cinch phantoms
42 commits recorded (~/.local/share/cinch/runs/3f9a1c2b8e7d4f10.jsonl)

links:      active — 3 red / 42 commits
rules:      phantom candidate (never fired) — 0 red / 42 commits
coupling:   active — 1 red / 39 commits (1 flaky)
identity:   not enough data (need 20+) — 0 red / 8 commits
generated:  phantom candidate (never fired) — 0 red / 42 commits
commit:     active — 5 red / 42 commits
core:       phantom candidate (never fired) — 0 red / 42 commits

flaky candidates (same commit, disagreeing outcomes across separate runs):
  coupling @ a1b2c3d: green in one run, red (1 finding) in another
```

The "(N flaky)" suffix and trailing section are omitted entirely when
`flaky` is empty. Empty-log case: `no commits recorded yet — run 'cinch
check' against a clean, committed tree to start building history`. Never
prints a percentage or a target — anomaly candidates only, each with the
run count behind it.

## Steps

1. Extract the per-check accumulation loop currently inline in
   `CmdCheck` (`internal/cinch/check.go`) into `runChecks(docsRoot, root,
   msgFile string) checkOutcome` — pure, no printing, no history write.
   `checkOutcome{findings []Finding, tallies []checkTally, suppressed
   []string}`. `CmdCheck` becomes: resolve docs root → `runChecks` →
   print (`output.CheckStatus` per tally, `output.Skip` per suppressed
   entry, findings to stdout) → `recordCommitCheck` → exit code from
   `len(outcome.findings)`. Behavior-preserving; `check-twice.md` depends
   on this same split.
2. Add `internal/cinch/phantoms.go`: `phantomsStorePath`,
   `recordCommitCheck`, `readCommitChecks`, `CmdPhantoms`.
3. Wire `recordCommitCheck` into `CmdCheck` after printing, per step 1.
4. `main.go`: add `"phantoms"` to `commands` (after `"check"`); add a
   `cinch phantoms` help entry ("Not a check: always exits 0", names
   `$XDG_DATA_HOME/cinch/runs/`); add `case "phantoms":` identical in
   shape to `case "ignores":`.
5. No changes to `hook.go` — `recordCommitCheck`'s own clean-tree gate
   handles every caller uniformly (manual run, pre-commit, commit-msg,
   CI), so no caller-identity threading is needed.
6. Tests (below).

## Verification

- `go test ./...`.
- Manual: in a scratch git repo, run `cinch check` after a few real
  commits, inspect `~/.local/share/cinch/runs/<key>.jsonl`, run `cinch
  phantoms` and confirm the reported per-check counts match; re-clone the
  same repo elsewhere and confirm `cinch phantoms` there sees the same
  history (proves the global-store design actually survives a reclone).
- `cinch help` shows `cinch phantoms`; `cinch phantoms` with an
  unexpected arg exits 2 (mirrors other no-arg commands).

Tests, `internal/cinch/phantoms_test.go` (`t.Setenv("XDG_DATA_HOME",
t.TempDir())` in every test touching storage):
- `phantomsStorePath`: same `root` (even via a different relative path)
  resolves to the same key; a different `root` resolves to a different
  key; honors `XDG_DATA_HOME` when set.
- `recordCommitCheck`: writes when tree is clean at HEAD; no-ops when
  dirty, no HEAD, or outside a git repo; two clean runs at the same SHA
  both append.
- `readCommitChecks`: agreeing same-SHA records tally as one commit;
  disagreeing same-SHA records for one check produce a `flakyCandidate`
  and are excluded from that check's summary while other checks in the
  same records still tally normally; skip doesn't count toward `Commits`.
- `CmdPhantoms` output: below-threshold → "not enough data"; at/above
  threshold with zero red → "phantom candidate"; nonzero red → "active";
  flaky suffix and section present/absent correctly.

CLI tests, `tests/cli_test.go` (existing `binPath(t)`/`cmd.Dir`/
`CombinedOutput` idiom, `cmd.Env` including `XDG_DATA_HOME=<t.TempDir()>`):
- `TestCheck_DirtyTreeDoesNotRecordCommit`
- `TestCheck_CleanCommitRecordsOnce`
- `TestCheck_RepeatedCheckSameCommitNotDoubleCounted`
- `TestPhantoms_NoHistoryYet`
- `TestPhantoms_FlagsNeverFiredCheck` / `TestPhantoms_NotEnoughData` (seed
  the store file directly rather than making 20+ real commits)
- `TestPhantoms_FlagsSameSHADisagreement`

### Critical files

- `internal/cinch/check.go` — `CmdCheck`, the extraction target for
  `runChecks`/`checkOutcome`/`checkTally`.
- `internal/cinch/coupling.go` — `isGitRepo`, `hasHead`,
  `gitChangedFiles`, `gitOutputLines`: the git plumbing this reuses
  rather than duplicates.
- `internal/cinch/phantoms.go` (new) — storage + `CmdPhantoms`.
- `main.go` — `commands`, `helpText`, dispatch.
- `tests/cli_test.go` — existing temp-git-repo test fixtures to build on.

### Relationship to other plans

`check-twice.md` depends on this plan's `runChecks`/`checkOutcome` split
landing first, and calls this plan's `recordCommitCheck` from its own
success path. Sequence this plan before that one.
