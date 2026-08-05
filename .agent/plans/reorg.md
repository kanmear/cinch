# Cinch reorganization — pin the plan, execute one phase at a time

Status: planned

Purpose: cinch has outgrown its own grasp — docs/ mixes living spec with dead
artifacts, the Go code is grouped by chronology not concept, naming is
inconsistent (harness at three levels, C13 is not a checker), and there is no
orientation artifact. This plan reorganizes and cuts. The fundamentals
cross-check follows in Phase 4, after scope is settled.

## Pinned definition (adopted verbatim — do not edit)

> A referential integrity checker for agent documentation, plus the binding
> layer that makes it portable across repositories.

Mechanism version, second paragraph:

> Cinch renders portable workflow templates against a per-repository manifest,
> and verifies the resulting corpus is referentially sound — that every path,
> link, command, generated artifact, and rule-to-test binding resolves to
> something that exists.

**What cinch is not:** a memory system; a semantic
validator — even at its best it detects *divergence* (C10/C14), never
*correctness*. The database connotation of "referential integrity" carries
the limitation for free. Keep the definition static: the divergence ceiling
lives in "What cinch is not," not in the mechanism line.

## Hard constraints

- **Cinch-only this session(s).** No consumer-visible identifier changes:
  template variable keys (`{{commands.*}}`, `{{paths.*}}`), checker IDs
  (C1–C12), and subcommand names (`render`, `index`, `check`, `ignores`,
  `version`) all stay stable. project_deltadocs stays C2-green untouched.
- The agent never runs `git commit` — the user reviews and commits between
  phases (.agent/conventions.md).
- **One phase at a time, stop at every gate.** No chaining into the next
  phase without user go.
- Append decision entries to `decisions.jsonl` for load-bearing decisions;
  keep ceremony low (2–3 entries, not one per move).

## Phase 0 — Pin

- This file. Done when written and Status: planned.

## Phase 1 — Scope & taxonomy (keystone) *(landed 2026-08-05 — D074–D076)*

Definition first; it justifies every cut.

Two deltas from the sketch below, both settled in review: (1) AGENTS.md was
trimmed beyond line 1 — its "What this repo is" bullets shrank to
session-relevant notes, because the audience seam (injected into every session
vs read on demand) makes README the owner of the layout map, glossary, and
registry; (2) the README's `.agent/` bullet reads "cinch's own consumer corpus
— not a harness of its own", closing the harness confusion at its last new
occurrence (the glossary's harness entry keeps the consumer sense).

- **README.md** (new, root): the pinned definition (one-liner + mechanism
  version); a "What cinch is not" section; one-paragraph layout map of the
  repo; **glossary** (harness = the consumer's `.agent/` system, cinch = the
  tool that builds and verifies it; binding layer; corpus; referential
  integrity; template; workflow; fragment; manifest; checker; rule ID;
  marker; domain doc; seam; auditor; scaffold); **checker registry** — the
  one-fact-one-place table currently scattered across comments in
  `check.go`/`rule.go`/`diff.go`: C#, what it checks, direction
  (doc-upstream/lateral/structural), severity, file.
- **AGENTS.md line 1**: replace "A portable agent harness: templates plus a
  Go CLI…" with the new definition — the entry point of this repo is the
  single worst offender of the harness confusion this plan fixes.
- **Decision entries** (2–3 in `decisions.jsonl`):
  1. docs/ taxonomy: spec (PHILOSOPHY) / contract (INTROSPECTION) /
     plan+status (ROADMAP) only; historical artifacts are deleted — git is
     the archive; session handoffs never live in docs/.
  2. C13 retired from the checker numbering (it is a unit test,
     `TestTemplatePurity`, not a checker — HARNESS-005's permanence spirit
     applies to C-numbers; a retired number stays retired).
  3. Go layout split (Phase 3).

**Exit criteria:** README reads true against the corpus; glossary terms used
consistently; AGENTS.md opens with the new definition. Stop for user review
of the README wording.

## Phase 2 — Cut

- **Delete outright** (git history is the archive):
  - `docs/RATIONALE.md` — superseded by ROADMAP v3; its phases are the same
    ones now marked [landed].
  - `docs/FINDINGS-render-audit.md` — P1-era worklist; items resolved into
    the D-log.
  - `docs/HANDOFF.md` — P6 session state; its work plan moves to
    `.agent/plans/drift-closure.md`.
  - `docs/decisions-log.md` — doc-downstream copy of the consumer's
    events-log schema; the exact smell the direction rule forbids.
- **Rescue HANDOFF's plan** → `.agent/plans/drift-closure.md`
  (`Status: planned`): drift-test fixture first, C14 in `diff.go`, blinded
  auditor rewrite of `templates/check-rules.md` step 3, C10-boundary
  decision, verification. Distill from HANDOFF.md's Work plan section before
  deleting it.
- **Slim ROADMAP**: phase sections become one-line statuses; strip D/E
  number soup from annotations; update the "Session loads" table to the new
  Go layout (Phase 3 paths); point forward work at the plan files.
- **AGENTS.md**: docs taxonomy, handoff rule (handoffs go to events/plan
  files, never docs/), test-location convention (unit tests live in
  `internal/` after Phase 3).
- Read `.githooks/commit-msg` before committing anything in this phase.

**Exit criteria:** dead docs gone; both plan files C9-clean (Status: line);
`./bin/cinch check` green (C1 unaffected — plans/ is index-skipped; C2
unaffected — no template changes). Stop for user review of the ROADMAP
rewrite (flagged: it is a real rewrite, not a trim; phase detail is
historical record — keep the phase list and statuses, drop D/E annotations).

## Phase 3 — Go restructure

```
cmd/cinch/main.go      — usage, dispatch, repoRoot/agentDir        (from main.go)
internal/manifest/     — manifest.go, manifest_test.go
internal/render/       — render.go, render_test.go, templates_test.go
internal/index/        — index.go, index_test.go
internal/check/        — check.go (C1–C7, C9, C11, C12), rule.go (C8),
                         diff.go (C10), introspect.go, report.go,
                         check/rule/diff/introspect tests
internal/version/      — version logic (from version.go); versionDir unchanged
version/               — generated store, untouched
tests/cli_test.go      — unchanged (exercises the built binary)
```

One checker package (not eight) — C8/C10 share `report` and helpers;
splitting further is churn-for-ceremony.

- **C13 fix everywhere**: `harness.md` HARNESS-003, `rule.go` comment,
  `templates_test.go` comment, `.agent/domain/overview.md` → cite
  `TestTemplatePurity`, never "C13"; grep for stragglers.
- Update `harness.md` `owns:` paths to the new layout (keeps C10 coupling
  live).
- Makefile: `go build -o bin/cinch ./cmd/cinch`; binary stays at `bin/` so
  binary-relative `templatesDir`/`versionDir` resolution is untouched.
- Update the ROADMAP "Session loads" table paths (if not already done in
  Phase 2).

**Exit criteria:** `make build test` green (`go vet ./...`, `go test
./...`); `./bin/cinch check` green on cinch's own repo (C1: `./bin/cinch
index` first if .agent changed; C2: re-render if any template changed —
templates are unchanged by this phase, so C2 should stay green); HARNESS
markers still resolve (they travel with the moved test files). Stop for user
review of the package layout.

## Phase 4 — Fundamentals cross-check

- **Dedup PHILOSOPHY ↔ doc-philosophy**: the template is the canonical
  operational statement of the derivability gate; PHILOSOPHY's "Code is the
  documentation" section shrinks to a pointer. Template change → re-render
  cinch's own copy (C2 green); the consumer re-renders on its own schedule.
- **North-star → mechanism map**: signal-to-noise → task-primitive context
  manifests; direction rule → checker registry; rules→tests→tasks →
  C8/C10/plan file; why-framing → templates. Confirm each north star has a
  live mechanism.
- **Direction-rule audit**: classify every checker in the registry; confirm
  none is doc-downstream.
- **Closure check**: HARNESS-001..005 + TestRuleIDsUnique resolve after the
  moves; AGENTS.md standing rules still true.
- Output: findings as decision entries + ROADMAP status; act only on
  confirmed issues.
- Close this plan per the completion ritual (Status: complete; keep the
  file — feature plans persist).

**Exit criteria:** no confirmed north star without a mechanism; registry
classifications match implementation; `./bin/cinch check` green; findings
logged.

## Verification (each phase)

`go vet ./...` and `go test ./...` after Go changes; `./bin/cinch check`
green on cinch's own repo after every phase; `./bin/cinch index` + C1 green
whenever `.agent/` markdown changes; no dangling links in any new/edited
docs (C11).

## References

- `docs/PHILOSOPHY.md` — the spec (north stars + foundation + forward vision)
- `docs/ROADMAP.md` — plan + status, to be slimmed in Phase 2
- `docs/INTROSPECTION.md` — the C6/C7 JSON contract (stays as-is; it is a
  contract, not philosophy)
- `decisions.jsonl` — append-only D-log; Phase 1 appends the taxonomy
  entries
- `.agent/domain/harness.md` — cinch's own domain rules (HARNESS-001..005)
- `AGENTS.md` — repo conventions; line 1 + taxonomy updates in Phases 1–2
