# 17. Cut the workflow pack to ~650 lines; render removes stale generated files

**Status: shipped in `09ef586` (`refactor/trim-workflow-pack`). Kept for provenance.** Also
the commit that effectively closed item 7's cheap half — see
`07-workflow-prose-value-and-f1-scope.md`. Depended on 13; ran before 18.

## Why

- The pack was 1,287 template lines plus a 38-line philosophy file, and a workflow
  ablation (24 runs) found no measurable read-time effect from it.
- Of 15–16 real plan files across both consumers, 0–1 used the full task ceremony.
- About 20 places told agents to read a `manifest.yml` cinch never creates
  (`taxonomy.*`, `paths.tests`, `development.commands`, `make <cmd>`).
- `{{paths.docs}}` was the only template variable.
- The one workflow idea with supporting data is "plan before code"; the one gap shown in
  real use is the missing rule — DeltaDocs' worst bug, a realtime content leak, sat in a
  green 76-rule corpus because no rule said who receives a realtime event.

## Part A: render removes files it generated and no longer produces

Deleting a template would otherwise leave every consumer with an orphaned-generated-file
finding. Orphan detection moved into a helper shared by `checkGenerated` and
`CmdRender`/`CmdUpgrade` (and so `CmdInit`), with the same rules as before: current render
directories plus those implied by `cinch.yml` at HEAD, regular files only, no recursion,
header-stamped only. Render removes orphans and says so; a file without cinch's header is
never touched. Bound by **CINCH-002**.

## Part B: rewrite the pack

Hard constraints, the checkable ones enforced by `internal/cinch/templates_test.go`:

1. `dev-task-primitive.md` deleted; what was worth keeping folded into the plan/execute
   workflows in at most 25 lines.
2. No `manifest.yml`, `manifest.`, `taxonomy`, `development.commands`, test-tier tables or
   `make <cmd>` assumptions; commands are "the project's test command".
3. No layout assumptions (`api/`, `models/`, `domain/<x>.md`).
4. `{{paths.docs}}` stays the only variable.
5. Templates plus philosophy at most 650 lines.
6. Every template keeps an H1 whose first following sentence is the trigger
   (`titleAndTrigger`).
7. Each dev workflow states an explicit entry gate.
8. A missing-rule step in `dev-plan-feature` and `dev-fix-bug`.
9. `docs-audit-coverage` inventories rules from `cinch rules --json`.
10. Redline cuts: compaction-anchor section, duplicated lists, long disclaimers.
11. No new workflows; `docs-philosophy.md` byte-identical.
12. Rendered output creates no rules or markers in consumer repos.

## Verify

Fresh `cinch init`: green check, eight workflow rows with sensible triggers. Upgrade path
from a pre-change binary removes `dev-task-primitive.md` and stays green.
