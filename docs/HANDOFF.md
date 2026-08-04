# P1 handoff — first templates landed, eight to go

Status: **Phase 1 in progress.** The binding side is done and green (C1/C3/C4/C12); the template
port has started — `check-rules` and `task-primitive` are authored *as templates*, render into
project_deltadocs, and are C2-guarded. The remaining eight workflows are still hand-authored.
`make check-harness` is **fully green** ("harness ok").

## Done

**[project] `manifest.yml` completed** (P1 session 1, `531021f` in `~/code/project_deltadocs`)

- commands (kebab-case, D040), test-dir layout, layer + test-tier taxonomy, `harness_version: 0.1.0` (D008)
- `seams:` declared (D038); C12 green
- both schema constraints hold: every tier `cmd` resolves under `development.commands` (C3); no `domains:` key

**[cinch] core commands working**

- `render` — D005 literal substituter; an undefined variable is an error (render.go)
- `index` — H1 extraction → `.agent/index.md` (D029); now skips leading YAML front-matter (D048)
- `check` — wave one C1/C2/C3/C4/C5/C9/C11/C12; C11 ignores links inside fenced code blocks (D046)

**Index ownership moved to cinch** (D041): `scripts/gen-agent-index.sh` deleted in projectX;
`docs-index` and the pre-commit guard route through `cinch index`. The guard git-inits its
staged-tree snapshot first (D042).

**Ops/chore session ran** (after the previous handoff): versioning system ported as
`cinch version show|bump` (D043), git-conventions moved out of docs/ (D044), `conventions/`
renamed to `ops/`. Nothing pending there.

**[P1 session 2 — this one]**

- `templates/check-rules.md` + `templates/task-primitive.md` authored as templates (D045
  conventions: vars from the scaffold contract only, examples genericized, no project names) and
  rendered into projectX — C2 green on both. Header + body hash guard active.
- Red checks closed: C5 (overview.md now references every domain, incl. Categories), C9 ×2 (both
  plans gained `Status:` lines — `open` for the action-bar stub, `complete` for the schema review),
  C11 ×3 (rules.md placeholder links, via D046).
- **D010 front-matter** (`owns:` code paths) landed on all eight `business/<domain>.md` docs.
- **Distribution settled** (D047): projectX keeps `CINCH ?= $(HOME)/code/cinch/bin/cinch`; the
  Makefile comment that claimed `~/.cinch` now describes the actual wiring. A `~/.cinch`
  distribution clone is release-time work (first tag), not now.
- **KICKOFF.md deleted**; its §4 relevance table folded into ROADMAP as the "Session loads"
  section (D035 still referenced).

**Decisions logged:** D045 (template porting conventions), D046 (C11 fence-skip), D047
(distribution), D048 (index skips front-matter), D049 (template-purity Go test, not a checker or
name blocklist), E001 (P1 progress event).

**Template purity test added** (D049): `templates_test.go` (`go test` / `make test`) now catches
command-shaped inline tokens (`` `/foo` ``), runner-config dotdir paths (`.claude/`-shaped,
excluding `.agent/`), and stack literals hardcoded from `scaffold/manifest.example.yml` instead of
referenced as `{{commands.<key>}}` — all structural, no maintained name list. `make`'s `test`
target no longer swallows `go test` failures.

## Not done — P1 blockers

1. **Eight workflows still hand-authored** in projectX; `render` only covers the two ported ones.
   Port order (smallest first) and per-file notes:

   | Workflow | Portability notes |
   | --- | --- |
   | `optimize-docs` | clean — manifest references only; port next |
   | `execute-plan` | clean; genericize the commit-conventions calibration examples (they name `back`/`front` scopes) |
   | `plan-feature` | clean; genericize the rule-interaction-table example (signatures/perms/members names) |
   | `sync-docs` | `make docs-index` → `{{commands.docs-index}}`; genericize quirks examples (`.env.local`, `json:"-"`) |
   | `fix-bug` | `test-backend`/`test-frontend` literals → tier-`cmd` mapping prose or `{{commands.*}}`; troubleshooting paths are structural, keep |
   | `rules.md` | procedure is portable but the §2 exception table names real API resources (access/auth/users) — that table is project data and must move into a project-side doc (candidate: `business/overview.md` or `conventions.md`) before the template is written |
   | `doc-philosophy` | principles are portable; the ✅/❌ example lists name this stack (Svelte runes, session-auth-vs-JWT) — genericize while keeping the concrete-vs-abstract contrast |
   | `figma-restyle` | hardest — names `frontend/src/lib/api/*.ts`, `action-bar.md`, and calls `make check-frontend` which is **not declared in the manifest** (project Makefile has it; add `check-frontend` to `development.commands` project-side first) |

   Each port must render into projectX and stay C2-green before the next (D033).
2. **Exit criterion unmet:** all ten workflows must be rendered output. C2 then guards the whole
   set; `make render` is the only writer.
3. **Distribution (D006/D008):** resolved as D047 for now (dev clone). The roadmap's "cloned to
   `~/.cinch`" line becomes true at first release — add an install/refresh step to release.sh
   then, or keep consumers on the dev clone and update the roadmap wording.
4. **Red checks:** none — `make check-harness` green.

## Next P1 session — recommended order

1. Port `optimize-docs` and `execute-plan` (smallest, cleanest). Render, `make check-harness`,
   then the next.
2. Port `plan-feature`, `sync-docs`, `fix-bug` — same discipline, genericizing examples per D045.
3. `rules.md`: first move its exception table project-side (see table above), then port the
   procedure. The `<related>.md`-style placeholder links in its example are now safe under D046.
4. `doc-philosophy`, then `figma-restyle` last (needs `check-frontend` declared in the manifest
   first — project-side edit).
5. Exit check: ten rendered workflows, C2 green, `make check-harness` green, no `{{` left
   unreplaced in projectX workflows.

## Notes for the next session

- **Harness-neutrality is part of the port (D045).** The remaining eight hand-authored workflows all
  open with an italic "Claude Code exposes this as …" line and reference workflows by slash command
  (`/execute-plan`, `/rules`, …). Both are Claude Code bindings: drop the line and rewrite the
  references as `.agent/workflows/<name>.md` paths. The consumer's `.claude/skills/` shims already
  frame the workflows as "agent-neutral procedures" — the workflow body naming the harness
  contradicts that framing. `make test` (D049) now catches the slash-command references and any
  hardcoded stack commands mechanically as each port lands — it will **not** catch the italic
  "Claude Code exposes this as …" line itself, since that's prose with no structural token; still
  needs eyes during each port.
- Templates resolve next to the binary (`~/code/cinch/templates`) — edit them in the cinch repo,
  then run `make render` in projectX. No sync step exists; do not introduce a second copy.
- `templates/` may be nested (render walks recursively) — keep flat unless a port needs grouping.
- The scaffold `manifest.example.yml` is the variable contract (D040/D045): a template may use any
  key it declares. New keys needed by a port must be added to the example *and* the consumer
  manifest, and C3/C4-style schema thinking applies.
