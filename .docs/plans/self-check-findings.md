# Plan: self-check findings — claims audit vs live behavior

Status: handoff — findings recorded, fixes proposed but not implemented

## Context

On 2026-08-09 the README's claims were audited against the code, the test
suites, and live behavior of the built binary (`bin/cinch`). The audit
covered: the three checks (links, rules, coupling), `cinch ignores`, `cinch
render` (philosophy verbatim, eight workflow templates, generated index,
`{{key}}` substitution, header/body-sha, idempotency), exit codes, the
Makefile/test story, and the documented "known limitation" about the marker
scan. Everything that claims to hold was re-verified; three gaps between
claim and behavior were found. This file records them with reproduction
steps and proposed fixes. Point-in-time session state; git is the archive.

## Verified claims

- Layout matches README/AGENTS.md: `main.go` is the only `package main`;
  everything else is in `internal/cinch` (`check.go`, `links.go`,
  `rules.go`, `coupling.go`, `render.go`, `manifest.go`, each `_test.go`
  carrying mutation fixtures); `tests/cli_test.go` exercises the built
  binary; `Makefile` = `build` + `go vet` + `go test`.
- `go vet` clean; every Go test passes (`ok cinch/internal/cinch`,
  `ok cinch/tests`).
- Exit codes 0/1/2 work as documented (verified live, including
  `check` with an unreadable MSGFILE → 2).
- `cinch render` produces exactly the 10 claimed files (8 templates +
  `doc-philosophy.md` copied verbatim + generated `index.md` derived from
  titles and opening lines). `{{paths.docs}}` is the only variable any
  template uses. Undefined variables are an error. Re-render is
  byte-identical (unit and CLI tests). Every output is header-stamped with a
  body-sha.
- `paths.docs` defaults to `.docs`; relative and absolute values both work
  for `check` (CLI-tested).
- Coupling is transition-scoped (working tree vs HEAD), announces its no-op
  on stderr (verified live: "working tree matches HEAD — nothing to
  compare"), and the `rule-reword: <ID>` escape hatch suppresses (tested).
- The known limitation is real: `cinch check` exits 1 against this repo's
  own tree, exactly as documented. The README's specific diagnosis
  (quoted marker syntax in `rules_test.go`) is *confirmed but incomplete* —
  see F1.

## Findings

### F1 — the marker scan's blast radius exceeds the documented known limitation

`scanRuleMarkers` (internal/cinch/rules.go:127) is a line-by-line regex over
every file under the repo root (excluding `.git`, `bin`, and the docs dir).
Unlike `parseRuleItems` and `checkLinksInFile`, it does not skip fenced
blocks, and its regex is unanchored at the end of the ID.

Live `cinch check` on this repo reports 8 findings; the README's limitation
paragraph names only `rules_test.go`'s quoted fixtures:

- `internal/cinch/rules_test.go` (3 findings) — the documented case.
- `internal/cinch/coupling_test.go` (4 findings) — same quoted-fixture
  mechanism, not named in the README.
- `internal/cinch/docs/templates/audit-domain.md:129` (1 finding) — a
  literal `// cinch:rule SIG-0NN` inside fenced example prose in a shipped
  template. Because the regex is unanchored, it is reported as the truncated
  ID `SIG-0` (`[A-Z0-9]+-[0-9]+` matches "SIG-0" and stops before "NN").
  This content ships to every consumer repo. It is also an asymmetry: the
  rules and links scans skip fenced blocks, the marker scan doesn't.

### F2 — absolute `paths.docs` outside the repo silently disables coupling

README advertises `paths.docs` as "relative to the project root, or an
absolute path", and coupling advertises "no silent pass". When `paths.docs`
points outside the repo, coupling is silently a no-op with no finding and no
stderr notice.

`checkCoupling` builds `git show HEAD:<path>` from
`relTo(repoRoot, path)` (internal/cinch/coupling.go:61). For a docs dir
outside the repo that relative spec is `../../...`; `gitShow` fails,
`ok=false` skips the file, and nothing is reported.

Reproduction (verified live): git repo, `.docs/manifest` with
`paths.docs = /tmp/.../ext-docs` (outside the repo), committed rule +
marker, then mutate the rule text only. Expected: a coupling block finding.
Actual: `exit 0`, no output at all.

Note: an absolute `paths.docs` *inside* the repo works correctly
(`relTo` yields a proper `git show` spec) — the gap is only for external
roots.

### F3 — untracked marker file false-positives coupling on manual runs

`gitChangedFiles` uses `git diff HEAD --name-only` (internal/cinch/
coupling.go:153), which excludes untracked files. If a rule's text changes
and its `// cinch:rule` marker sits in a brand-new, un-staged test file,
coupling fires a block finding even though the marker file *did* change —
it just isn't tracked yet.

Reproduction (verified live): repo, commit rule + marker file, then create
`untracked_test.go` (un-staged) containing the marker and mutate the rule
text → coupling block finding, exit 1. After `git add`, the same tree is
clean, exit 0.

Severity is low in the intended usage (the commit-msg hook runs after
`git add`, so staged new files are included in `git diff HEAD`), but a
manual `cinch check` on an in-progress tree false-positives.

### N1 — minor notes, no fix proposed

- `index.md` includes a row for `doc-philosophy.md` (9 rows, not 8). The
  README's "every other rendered file" phrasing is ambiguous but the
  behavior is coherent: the index lists every rendered file, philosophy
  included.
- The philosophy row's trigger spans two sentences ("The Core Insight…" +
  "These docs exist only for…"), because both end in sentence punctuation
  and `titleAndTrigger` keeps reading until `startsNewBlock`. Mechanically
  derived, just longer than a "trigger".

## Proposed fixes

### Fix F1 — fence-aware, token-anchored marker scan

Edit `scanRuleMarkers` (internal/cinch/rules.go):

- Track `inFence` with the existing `fenceRe` (same toggle as
  `parseRuleItems` / `checkLinksInFile`), skipping lines inside fences. This
  kills the `audit-domain.md:129` finding and future fenced examples in
  templates. Go source is unaffected in practice (fence lines are ```,
  which Go never produces).
- Anchor the end of the ID in `markerRe` so an ID token is never truncated
  out of prose: `//\s*cinch:rule\s+([A-Z0-9]+-[0-9]+)\b`. With RE2 the
  previous-match against `SIG-0NN` fails instead of truncating to `SIG-0`.
  (Go's regexp supports `\b`.)

The quoted string literals in `rules_test.go` / `coupling_test.go` remain
visible to the scan — that part of the known limitation is inherent to a
textual scan and stays documented.

### Fix F2 — coupling announces a no-op for out-of-repo docs roots

In `checkCoupling` (internal/cinch/coupling.go), before the walk, determine
whether the docs dir is inside the repo: resolve both `docsDir` and
`repoRoot` to absolute paths and require `filepath.Rel` not to escape.
When the docs root is outside the repo, return
`NoOp: "coupling: paths.docs is outside the repository — check did not run"`
(consistent with the existing no-op contract: said on stderr, never a
silent pass). Links and rules still run against the external root — they
don't need git — so only coupling changes.

### Fix F3 — count untracked files as changed

In `gitChangedFiles`, append `git ls-files --others --exclude-standard`
(untracked, not ignored) output to the `git diff HEAD --name-only` list,
deduplicated. A new un-staged marker file then counts as "the marked file
changed" and the false positive goes away in manual runs; the hook context
is unchanged.

### Fix F4 — correct the README's known-limitation paragraph

Expand `## Known limitation` to state that the self-check failure is not
limited to `rules_test.go`: it also covers `coupling_test.go` fixtures and
(until Fix F1 lands) fenced example prose in
`internal/cinch/docs/templates/audit-domain.md`, with the truncated-ID
behavior. Also record the F2 and F3 behaviors briefly, since both are
undocumented claim edges. If a fix lands, the paragraph describes the
residual, still-true part: quoted marker strings in test fixtures.

## Verification

- `make test` — builds `bin/cinch`, `go vet` clean, all tests pass,
  including updated CLI tests.
- F1: `bin/cinch check` in this repo — finding count drops from 8 to the
  7 fixture-literal findings (3 from `rules_test.go`, 4 from
  `coupling_test.go`), and no `SIG-0` finding remains. Add a unit test
  seeding a fenced `// cinch:rule` line in a non-docs `.md` file and
  asserting it is not reported.
- F2: repeat the external-`paths.docs` reproduction — `exit 0` now with
  `coupling: paths.docs is outside the repository — check did not run` on
  stderr. Add a coupling unit test for the out-of-repo docs dir.
- F3: repeat the untracked-marker reproduction — `exit 0` with the rule
  text changed and the marker in an untracked test file. Extend
  `TestCoupling_*` with the untracked-marker case.
- F4: README paragraph reflects the verified behavior.

## Evidence (commands run on 2026-08-09)

Self-check (`./bin/cinch check` in this repo): 8 findings, exit 1 —
`coupling_test.go:51,52,85,153`, `docs/templates/audit-domain.md:129`
(reported as `SIG-0`), `rules_test.go:28,40,105`; coupling no-op printed on
stderr; `./bin/cinch ignores` exit 0.

F2 reproduction (git repo, `paths.docs` = absolute dir outside the repo,
rule text mutated): `exit 0`, no stderr notice, no finding.

F3 reproduction (git repo, untracked marker file, rule text mutated):
```
coupling block .docs/rules.md:1: CIN-001: rule text changed but marked test file (untracked_test.go) did not — escape via `rule-reword: CIN-001` in the commit message
```
exit 1; after `git add -A`: exit 0.
