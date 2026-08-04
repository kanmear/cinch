# P1 handoff — seven templates landed, three to go

Status: **Phase 1 in progress.** The binding side is done and green (C1/C3/C4/C12); the template
port is underway — `check-rules`, `task-primitive`, `optimize-docs`, `execute-plan`,
`plan-feature`, `sync-docs`, and `fix-bug` are authored *as templates*, render into
project_deltadocs, and are C2-guarded. The remaining three workflows are still hand-authored.
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

**[P1 session 2]**

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

**[P1 session 3 — this one]**

- `templates/optimize-docs.md` + `templates/execute-plan.md` authored as templates and rendered
  into projectX — C2 green on both, `make check-harness` clean. Neither needed a `{{commands.*}}`
  substitution (no literal `make` commands in either source); the work was harness-neutrality
  (dropped the "Claude Code exposes this as…" line and every `/slash-command` reference, per D045 —
  sibling workflows are now referenced by their rendered `.agent/workflows/<name>.md` path) plus
  genericizing `execute-plan.md`'s real commit-history calibration examples into a `<scope>`
  placeholder shape.
- `TestTemplatePurity` (D049) passed clean on both new templates on first write — no follow-up
  fixes needed.
- **Bug found and fixed (D050):** `templates/README.md` was getting rendered into projectX as
  `.agent/workflows/README.md` — `renderAll` walks every `*.md` under `templates/` with no
  filename exclusion, so a meta-doc there is indistinguishable from a workflow. Deleted it (its
  content already duplicated `AGENTS.md`); removed the orphaned rendered copy from projectX by
  hand, since `cmdRender` doesn't clean up output whose template disappeared. `AGENTS.md`'s
  `templates/` bullet now says directory conventions live there, not a nested README. Also fixed a
  stray `(D050)` reference in `templates_test.go`'s doc comment (from the D049 commit) that pointed
  at a decision never actually logged — it now cites D049, which is what it was restating.

**[P1 session 4 — this one]**

- `templates/plan-feature.md`, `templates/sync-docs.md`, and `templates/fix-bug.md` authored as
  templates and rendered into projectX — each verified individually (render + `cinch check` C2
  green + `go test` D049) before the next port started, per D033. `make check-harness` clean
  throughout.
- Slash-command tokens and `plan-feature`/`sync-docs`'s italic "Claude Code exposes this as…" lines
  dropped; sibling-workflow prose references (`task-primitive.md`, `rules.md`, `execute-plan.md`)
  rewritten as `.agent/workflows/<name>.md` paths, matching the four prior ports.
- `plan-feature.md`'s rule-interaction-table example (`signatures.md`/`tabs.md`/`perms`/`members`)
  and `sync-docs.md`'s quirks examples (`tParams()`, `.env.local`, `json:"-"`,
  `credentials: 'include'`) genericized into `<placeholder>` shapes, keeping the same illustrative
  structure.
- `fix-bug.md`'s reproduce-method table: the `test-backend`/`test-frontend` row became tier-`cmd`
  mapping prose pointing at `taxonomy.test_tiers` (matching how `task-primitive.md` already treats
  tier-derived values, rather than hardcoding a tier→key assumption the schema doesn't guarantee);
  the `start-backend`/`start-frontend` row — not tier-derived — became a direct
  `{{commands.start-backend}}` / `{{commands.start-frontend}}` substitution. Troubleshooting-file
  paths kept literal per HANDOFF's prior note (structural, `.agent/` is D045's dotdir exception).
- **Scaffold amendment (D051):** `scaffold/manifest.example.yml`'s `development.commands` gained
  `docs-index`, `start-backend`, `start-frontend` — needed by these two ports and previously
  undeclared, which left D045's "a template may assume any key it declares" claim silently false
  (as `task-primitive.md`'s pre-existing `{{commands.test}}` already had — noted, not fixed, this
  session).
- **One correction made mid-port:** `sync-docs.md`'s `[Doc Philosophy](doc-philosophy.md)` markdown
  link was initially rewritten to `.agent/workflows/doc-philosophy.md` to match the prose
  sibling-reference convention, which broke C11 — link resolution in `checkLinks` is relative to
  the linking file's own directory, so `.agent/workflows/doc-philosophy.md` resolved against
  `.agent/workflows/` doubled the path. The sibling-reference convention applies to backtick prose
  only; actual markdown links must stay relative. Reverted to the original relative link.

**Decisions logged:** D045 (template porting conventions), D046 (C11 fence-skip), D047
(distribution), D048 (index skips front-matter), D049 (template-purity Go test, not a checker or
name blocklist), D050 (templates/ holds only templates, no nested README), D051 (scaffold
contract gains docs-index/start-backend/start-frontend), E001 (P1 progress event), E002 (P1
session 3 progress event), E003 (P1 session 4 progress event).

**Template purity test added** (D049): `templates_test.go` (`go test` / `make test`) now catches
command-shaped inline tokens (`` `/foo` ``), runner-config dotdir paths (`.claude/`-shaped,
excluding `.agent/`), and stack literals hardcoded from `scaffold/manifest.example.yml` instead of
referenced as `{{commands.<key>}}` — all structural, no maintained name list. `make`'s `test`
target no longer swallows `go test` failures.

## Not done — P1 blockers

1. **Three workflows still hand-authored** in projectX; `render` now covers the other seven.
   Port order (smallest first) and per-file notes:

   | Workflow | Portability notes |
   | --- | --- |
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

1. `rules.md`: first move its exception table project-side (see table above), then port the
   procedure. The `<related>.md`-style placeholder links in its example are now safe under D046.
2. `doc-philosophy`, then `figma-restyle` last (needs `check-frontend` declared in the manifest
   first — project-side edit).
3. Exit check: ten rendered workflows, C2 green, `make check-harness` green, no `{{` left
   unreplaced in projectX workflows.

## Notes for the next session

- **Harness-neutrality is part of the port (D045).** The remaining three hand-authored workflows
  may still open with an italic "Claude Code exposes this as …" line and reference workflows by
  slash command (`/rules`, …) — check each on read; `fix-bug.md` already had neither, so it's not
  guaranteed every remaining file does. Both are Claude Code bindings: drop the line and rewrite
  the references as `.agent/workflows/<name>.md` paths. The consumer's `.claude/skills/` shims
  already frame the workflows as "agent-neutral procedures" — the workflow body naming the harness
  contradicts that framing. `make test` (D049) now catches the slash-command references and any
  hardcoded stack commands mechanically as each port lands — it will **not** catch the italic
  "Claude Code exposes this as …" line itself, since that's prose with no structural token; still
  needs eyes during each port. **New from session 4:** the sibling-reference rewrite applies to
  backtick prose only — an actual markdown link (`[text](target.md)`) must stay relative to the
  linking file's own directory, since C11's `checkLinks` resolves link targets that way, not
  repo-root-relative; rewriting a real link to a `.agent/workflows/…` form breaks it.
- Templates resolve next to the binary (`~/code/cinch/templates`) — edit them in the cinch repo,
  then run `make render` in projectX. No sync step exists; do not introduce a second copy.
- `templates/` may be nested (render walks recursively) — keep flat unless a port needs grouping.
- The scaffold `manifest.example.yml` is the variable contract (D040/D045): a template may use any
  key it declares. New keys needed by a port must be added to the example *and* the consumer
  manifest, and C3/C4-style schema thinking applies.
