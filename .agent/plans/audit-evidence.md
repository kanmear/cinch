# Audit evidence — C15 mechanically checks the coverage report

Status: planned

Purpose: the audit report is the only output nothing validates. Every row must carry
verbatim evidence — the rule's text and the test's assertion line — and a new checker
(C15) verifies both quotes are non-empty and appear in their source files. That kills
vacuous rows, fabricated test names, and paraphrase-as-evidence mechanically, at both
tiers: the check never asks a model anything, so a 27B report and a frontier report
pass or fail the same way. This also resolves the deferred (b) fork from drift-closure
Phase 4 — iterate the restate-and-flag layer — as the mechanical path: the report
format becomes an artifact contract instead of a wording judgment.

Executed in three phases, one side per phase ([cinch] / [both]); stop at every gate —
the user reviews and commits between phases, no chaining.

## Context — why the shape

The recorded failures are all report-shape failures, not verdict failures:

- run2 (08-06): vacuous row `| SIG-002 | API | handlers/tab_test.go | handler test | ✅ |`
  — no rule text, no test name (records/pi-audits/2026-08-06-drift-run2.md)
- run1 (08-06): matched SIG-002 to SIG-004's test by name; rule text never engaged
- baseline (08-06): rule-text summaries are loose paraphrases (SIG-002 labeled
  "requires edit" — SIG-003's text)

C15 requires each row to quote verbatim the rule text and the specific assertion line,
then validates both quotes are non-empty and appear as substrings in their source
files. It cannot judge a well-quoted wrong link — that stays the semantic pass's
content question — but it kills the three classes above at both tiers. The report is a
committed artifact with a manifest home; artifacts get checked (C1/C2 shape). Severity
is error: a row that can't back its claim is the D027 false-checkmark class.

## Phase 1 — C15, template step 4, tests [cinch]

- **C15** in `internal/check/audit.go` (wired into `Run`): reads the report at the new
  manifest key `paths.audit` (e.g. `.agent/audit/coverage.md`); undeclared → skip
  (C5/C12 absence semantics); C4 covers existence. Parses the per-domain tables
  (`## <domain>.md` headings, then rows). Per row:
  1. rule ID resolves in `domain/*.md` (reuse `parseDomainDoc`) — else error
  2. rule-text cell non-empty and a substring of that rule's full numbered-item body
     (multi-line safe) — else error
  3. status ∈ ✅ / ⚠️ TENSION / ⚠️ MISSING / N/A — else error
  4. ✅/TENSION rows: test file resolves (repo root, else under any declared
     `paths.tests.*`); test-name cell non-empty and a substring of the file; assertion
     cell non-empty and a substring of the file — else error
  5. MISSING/N/A rows: no test evidence required; rule-text quote still required
  6. completeness: every closure rule must have a row — a report with no rows is a
     vacuous artifact (added after the pi probe: its format-deviant report passed every
     per-row check by having none)
- Cells tolerate markdown-escaping of interior backticks (`\`` → `` ` ``): a model
  wrapping a backticked rule text in a code span escapes it, and the escape is the
  faithful copy.
- C11 skips the manifest-declared audit report: its cells quote rule texts verbatim, and
  a link inside a quote resolves from the report's own directory, not the source's (D046
  class; same treatment as `records/` in scanRuleMarkers).
- **Template** `templates/check-rules.md` step 4: per-domain tables gain the columns
  `| Rule | Rule text (verbatim) | Testable | Covered | Test file | Test | Assertion (verbatim) |`.
  Cells are exact copies — no wrapper quotes, no paraphrase, no ellipsis; the report is
  machine-checked (C15). Example cells use placeholders, never literal assertion syntax
  (stack-literal rule; templates_test.go guards).
- **Tests** `internal/check/audit_test.go` (mkMarker fixture convention): each failure
  class, N/A/MISSING exemption, multi-line rule bodies, backticked rule text, test-file
  resolution under paths.tests.*, undeclared path → skip.
- `scaffold/manifest.example.yml` documents the key; README registry row; ROADMAP status
  touch; plan file (this file).

**Exit:** `go vet ./...` + `go test ./...` green; template purity green; cinch
self-check exit 0 at the known baseline (paths.audit not yet declared in cinch's
manifest — skip is the declaration).

## Phase 2 — Consumer round, dual-driver [both]

- Re-render deltadocs `check-rules.md`, C2 green; deltadocs `manifest.yml` gains
  `paths.audit`.
- **Two fresh baseline audits on the clean pin, one per driver — claude and pi.** The
  claude report is committed at `paths.audit` (strong-tier verdict is canonical; 75/75
  rules, all evidence verbatim, C15 green). The pi report failed C15 mechanically —
  "has no coverage rows": the 27B produced an entirely improvised format (no canonical
  tables, invented domains) — archived at `records/pi-audits/` with its C15 verdict, the
  tier-boundary evidence recorded as a check result, not a judgment call.
- Cinch's own corpus: `.agent/manifest.yml` gains `paths.audit`; commit cinch's report
  — all five HARNESS rules are cinch:ignore (D062), so N/A rows with verbatim
  rule-text quotes, fully determined by the corpus.
- **Fixture**: the pin now carries the report, so the end-state `$CINCH check` on the
  mutated tree sees it — row 9's inverted SIG-002 text breaks the committed quote → C15
  fires mechanically on the semantic row (C14's history, one level up). `expectations.json`
  row 9 re-scored (C14 warn + C15 stale-quote error); pin bumped to the
  report-carrying commit.

**Exit:** deltadocs `make check-harness` exit 0 at baseline (C2 green, C15 green on the
committed report); cinch self-check exit 0 with its own N/A report; both driver reports
recorded with C15 verdicts.

## Phase 3 — Close [both]

`make drift-test` exit 0 with re-scored row 9; `make drift-test-semantic` exit 0
(claude's TENSION report is C15-clean — SIG-002 quoted verbatim against the mutated
text). Decision entry (D084: C15 shape, severity, `paths.audit` contract, dual-driver
record), roadmap refresh, plan closed complete.

## Verification

- Phase 1: `go vet ./...` and `go test ./...`; `make test`; cinch self-check.
- Phase 2: deltadocs check-harness exit 0 at baseline; dual-driver reports recorded.
- Phase 3: `make drift-test` and `make drift-test-semantic` exit 0.

## Notes

- A rule-text change makes the committed report stale → C15 errors until re-audit —
  intended, the C2-staleness analog; the executor flow (audit → apply → commit report)
  absorbs it.
- The report's quote cells can't span lines in a table, so multi-line rule texts quote a
  single-line fragment — substring matching covers it.
- The completeness row check was marked out of scope in the plan draft; the pi probe's
  format-deviant report (no canonical rows) passed C15 with zero findings, so
  completeness became part of the check: a report covering no rules is the vacuous
  artifact class the check exists for.
