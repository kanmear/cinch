# cinch — `cinch check --twice` determinism probe

Status: **proposed** — not started.

## Context

Companion to the phantom-check-history proposal (`.docs/plans/phantom-check-history.md`).
That design and an independent second critique of the same original
proposal (session notes: `~/Documents/cinch/run-log-discussion.md`) both
identified flakiness — nondeterminism in a supposedly-deterministic
checker — as a second legitimate need hiding inside "track check
outcomes," and both converged on the same fix: it needs no history at
all. Run the full suite twice against the identical tree and diff. Two
runs suffice regardless of sample size; a deterministic checker must
never disagree with itself on the same input, so a single disagreement is
proof, not evidence. This is runtime enforcement of `PRINCIPLES.md`
principle 1 (deterministic checkers) in production, not just against a
mutation fixture — arguably the highest-value check the system can run
on itself, at zero storage cost when the two runs agree.

Depends on `phantom-check-history.md` landing first: both plans share the
`runChecks`/`checkOutcome`/`checkTally` data model that plan introduces,
and this plan calls that plan's `recordCommitCheck` on its own success
path.

## Design

Run `runChecks` (from the companion plan's refactor) twice against the
same tree, compare the two `checkOutcome`s. Disagreement is a bug in a
checker that's supposed to be a pure function of the tree — report it
directly instead of silently accepting whichever result happened to run.

```go
func diffOutcomes(a, b checkOutcome) []string {
    var diffs []string
    for i := range a.tallies {
        ta, tb := a.tallies[i], b.tallies[i]
        if ta.Status != tb.Status || ta.Count != tb.Count || !reflect.DeepEqual(ta.Reasons, tb.Reasons) {
            diffs = append(diffs, fmt.Sprintf("%s: run 1 = %s (%d), run 2 = %s (%d)",
                ta.Name, ta.Status, ta.Count, tb.Status, tb.Count))
        }
    }
    return diffs
}

func CmdCheckTwice(msgFile string) int {
    docsRoot, err := ResolveDocsRoot(".")
    if err != nil {
        return output.Fail("check", err)
    }
    first := runChecks(docsRoot, ".", msgFile)
    second := runChecks(docsRoot, ".", msgFile)

    if diffs := diffOutcomes(first, second); len(diffs) > 0 {
        fmt.Fprintln(os.Stderr, "cinch: check --twice: nondeterminism — two runs against the identical tree disagreed:")
        for _, d := range diffs {
            fmt.Fprintln(os.Stderr, "  "+d)
        }
        fmt.Fprintln(os.Stderr, "cinch: check --twice: not recording to phantom history — an unreliable result isn't a fact about this commit")
        return 1
    }

    printOutcome(first) // shared with CmdCheck: CheckStatus lines, suppressed notices, findings
    recordCommitCheck(".", first.tallies)
    output.Step("check --twice: two independent runs agreed (%d findings)", len(first.findings))
    if len(first.findings) > 0 {
        return 1
    }
    return 0
}
```

Exit code on disagreement: reuse `1` (cinch's existing "not clean" code)
rather than inventing a fourth code — the stderr message carries the
specific meaning. `helpText`'s `exit codes: 0 clean, 1 findings, 2 usage
error` line gets a one-line addendum noting `--twice` also returns 1 on
detected nondeterminism.

Deliberately does **not** record to the phantom store on disagreement — a
result that isn't reproducible isn't a fact about that commit worth
persisting; recording it would let an unreliable checker quietly poison
the companion plan's own ledger.

`CmdCheck` (single-run path) is refactored to the same `runChecks` +
`printOutcome` shape as part of the companion plan, so plain `cinch
check` behavior is unchanged by this addition — verify by not modifying
any existing CLI test's expected output.

## Steps

1. Land `phantom-check-history.md`'s `runChecks`/`checkOutcome`/
   `checkTally` refactor first (this plan builds directly on it).
2. Add `printOutcome(checkOutcome) `to `check.go` — the shared
   CheckStatus/suppressed/findings printing `CmdCheck` and
   `CmdCheckTwice` both call.
3. Add `diffOutcomes` and `CmdCheckTwice` to `check.go`.
4. `main.go` `case "check":` gains hand-rolled `--twice` flag scanning
   (matches the codebase's existing no-`flag`-package, manual-argv style
   — `grep -n 'flag\.' main.go` currently returns nothing):
   ```go
   case "check":
       args := os.Args[2:]
       twice := false
       var rest []string
       for _, a := range args {
           if a == "--twice" {
               twice = true
               continue
           }
           rest = append(rest, a)
       }
       if len(rest) > 1 {
           os.Exit(output.UsageErr("check: too many arguments"))
       }
       msgFile := ""
       if len(rest) == 1 {
           msgFile = rest[0]
           if _, err := os.Stat(msgFile); err != nil {
               os.Exit(output.UsageErr("check: cannot read message file: " + msgFile))
           }
       }
       if twice {
           os.Exit(impl.CmdCheckTwice(msgFile))
       }
       os.Exit(impl.CmdCheck(msgFile))
   ```
5. `helpText`: extend the existing `cinch check [MSGFILE]` entry with a
   `--twice` line: "run every check twice against the identical tree and
   report any check whose result differs between runs — a determinism
   self-test."
6. Tests (below).

## Verification

- `go test ./...`.
- Manual: `cinch check --twice` in a scratch repo against a clean tree
  (reports agreement, exit 0) and against the existing broken-link
  fixture (reports agreement, exit 1, same findings as plain `cinch
  check`).
- `cinch help` shows the `--twice` flag.

Tests, `internal/cinch/check_test.go`:
- `diffOutcomes`: identical tallies → no diffs; a status/count/reasons
  mismatch on one check → exactly one diff naming that check. This is
  the right boundary to test nondeterminism *detection* at — the actual
  checkers are (and must stay) deterministic, so there's no way to force
  a genuinely flaky result end-to-end without adding a fake
  nondeterministic checker, which is out of scope.

CLI tests, `tests/cli_test.go`:
- `TestCheckTwice_CleanTreeAgrees`: exits 0, prints "two independent runs
  agreed", matches plain `cinch check`'s finding output.
- `TestCheckTwice_RedTreeAgrees`: same against the broken-link fixture —
  exits 1, same findings as plain `cinch check`.
- `TestCheckTwice_RecordsOneCommit`: `cinch check --twice` against a
  clean HEAD, then `cinch phantoms` reports exactly 1 commit (not 2) —
  confirms the double run doesn't double-write.
- `TestCheckTwice_TooManyArgs`: `cinch check --twice a b` → exit 2.

### Critical files

- `internal/cinch/check.go` — `runChecks`/`checkOutcome` (from the
  companion plan), plus this plan's `printOutcome`, `diffOutcomes`,
  `CmdCheckTwice`.
- `internal/cinch/phantoms.go` — `recordCommitCheck`, called from this
  plan's success path.
- `main.go` — `--twice` flag parsing in `case "check":`, `helpText`.
- `tests/cli_test.go`.

### Relationship to other plans

Depends on `phantom-check-history.md`. Do not implement standalone —
`runChecks`/`checkOutcome`/`checkTally` and `recordCommitCheck` are
defined there.
