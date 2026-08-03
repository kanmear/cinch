# P1 handoff — binding layer landed, templates not started

Status: **Phase 1 incomplete.** The manifest/binding side is done and green (C1/C3/C4/C12); the
defining deliverable — templates ported *as templates*, rendered into the consumer — has not begun.
A chore session runs before P1 completion (see below).

## Done

**[project] `manifest.yml` completed** (P1 session 1, `531021f` in `~/code/project_deltadocs`)

- commands (kebab-case, D040), test-dir layout, layer + test-tier taxonomy, `harness_version: 0.1.0` (D008)
- `seams:` declared (D038); C12 green
- both schema constraints hold: every tier `cmd` resolves under `development.commands` (C3); no `domains:` key

**[cinch] core commands working**

- `render` — D005 literal substituter; an undefined variable is an error (render.go)
- `index` — H1 extraction → `.agent/index.md` (D029)
- `check` — wave one C1/C2/C3/C4/C5/C9/C11/C12

**Index ownership moved to cinch** (D041): `scripts/gen-agent-index.sh` deleted in projectX;
`docs-index` and the pre-commit guard route through `cinch index`. The guard git-inits its
staged-tree snapshot first (D042).

**Decisions logged:** D040 (kebab-case command keys are the template contract), D041, D042.

## Not done — P1 blockers

1. `templates/` is empty (only `.gitkeep`). No workflow ported; `render` has nothing to render.
2. **Exit criterion unmet:** projectX `.agent/workflows/*.md` are still hand-authored, not rendered output.
3. **Distribution (D006/D008):** cinch is not at `~/.cinch` — it lives at `~/code/cinch`, and projectX's
   Makefile comment claims `~/.cinch` while `CINCH ?=` points at `~/code/cinch` (comment and wiring
   disagree). Render resolves templates from `$CINCH_HOME/templates`, then `<binary>/../templates`,
   then `~/.cinch/templates`.
4. **D010:** per-domain owning code paths not in `business/<domain>.md` front-matter.
5. **Red checks:** C5 (overview.md omits the Categories domain), C9 ×2 (plans missing `Status:` line),
   C11 ×3 (placeholder links in `workflows/rules.md`).
6. **KICKOFF.md** — §4 relevance table to fold into ROADMAP, then delete the file (P1-era cleanup).

## Pending chore (before P1 completion)

*TBD — the chore session runs before the template-porting session; list it here when known.*

## Next P1 session — recommended order

1. Complete the chore above; log anything it changes (decisions.jsonl is append-only).
2. Port templates one at a time, smallest first (candidates: `task-primitive`, `check-rules`). Each
   must render into projectX and stay green under C2 before the next is ported (D033: nothing lands
   that isn't rendering into a real project).
3. Close C5/C9/C11 so `make check-harness` goes fully green.
4. Add D010 front-matter (owning code paths) to the business docs.
5. Settle distribution: move/alias to `~/.cinch`, or fix the Makefile comment to match `~/code/cinch`.
   Template resolution falls back to `~/.cinch/templates`, so this is the moment to fix the path.
6. Fold KICKOFF §4 into ROADMAP, delete KICKOFF.md.
7. Exit check: workflows are rendered output, C2 guards the render step, `make check-harness` green.
