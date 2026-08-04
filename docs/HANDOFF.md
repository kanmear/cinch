# P1 handoff — nine of ten workflows rendered; template-porting goal met

Status: **Phase 1's template-porting exit criterion is met.** The binding side is done and green
(C1/C3/C4/C12); nine workflows — `check-rules`, `task-primitive`, `optimize-docs`, `execute-plan`,
`plan-feature`, `sync-docs`, `fix-bug`, `rules`, and `doc-philosophy` — are authored *as
templates*, render into project_deltadocs, and are C2-guarded. `figma-restyle.md` is intentionally
**not** ported (D053) — its content doesn't generalize into a portable procedure, so it stays
project-owned prose, same status as any project doc with no derivable-elsewhere content.
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

**[P1 session 3]**

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

**[P1 session 4]**

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

**[P1 session 5 — this one]**

- `templates/rules.md` and `templates/doc-philosophy.md` authored as templates and rendered into
  projectX — each verified individually (render + `cinch check` C2 green + `go test` D049) before
  the next port started, per D033.
- **`rules.md`'s exception table moved project-side first (D052):** the three bullets naming
  `api/access.md`/`api/auth.md`/`api/users.md` as having no `business/*.md` counterpart moved into
  `business/overview.md` § Structure Convention (an `**Exceptions**` sub-list extending the
  existing section, not a new one). The template now points at that section by path
  (`{{paths.business}}/overview.md`) instead of hardcoding the table; the file-creation block's
  `<Resource>`/`<related>` placeholders were already generic (D046) and needed no further change.
- `doc-philosophy.md`'s stack-specific examples — Go error handling / Svelte reactivity /
  session-vs-JWT auth (principle 1), the bcrypt/`backend/handlers/auth.go` example (principle 2),
  `models/session.md`/`api/auth.md` (principle 3), the `as never` i18n workaround (principle 6),
  and the ❌/✅ lists' Go/Svelte/`.env.local`/MCP-for-Svelte-docs mentions — were genericized into
  placeholder shapes, keeping the concrete-vs-abstract contrast the principles illustrate (same
  treatment session 4 gave `sync-docs.md`'s quirks examples).
- **`figma-restyle.md` scoped out of the port entirely (D053), on direction mid-session:** its
  per-component state-inventory methodology is tied to a specific `frontend/components/` tree, a
  worked example naming `action-bar.md`, and an assumed `frontend/conventions.md` § Documenting
  Complex Component States anchor — project-specific enough that a template would either lose the
  concrete guidance or need a manifest key (e.g. a frontend-component-docs path) serving only this
  one workflow. It remains project-owned prose, not a P1 blocker.
- **`check-frontend` manifest key reverted:** added to `scaffold/manifest.example.yml` and the
  project manifest in prep for the `figma-restyle` port (mirroring D051's pattern), then reverted
  from both once that port was scoped out — no other template needs it, and D045's contract is
  that only what a template actually uses belongs in the scaffold's variable contract.
- **Phase 1's roadmap exit line revised:** "the project's workflows are rendered output" now reads
  "nine of the project's ten workflows are rendered output; `figma-restyle.md` stays project-owned
  prose (D053)". `make check-harness` clean throughout; no other P1 exit item is outstanding
  (distribution remains the accepted D047 deferral, not a blocker).

**Decisions logged:** D045 (template porting conventions), D046 (C11 fence-skip), D047
(distribution), D048 (index skips front-matter), D049 (template-purity Go test, not a checker or
name blocklist), D050 (templates/ holds only templates, no nested README), D051 (scaffold
contract gains docs-index/start-backend/start-frontend), D052 (rules.md exception table moves to
business/overview.md § Structure Convention), D053 (figma-restyle stays hand-authored, not
ported), E001 (P1 progress event), E002 (P1 session 3 progress event), E003 (P1 session 4 progress
event), E004 (P1 session 5 progress event).

**Template purity test added** (D049): `templates_test.go` (`go test` / `make test`) now catches
command-shaped inline tokens (`` `/foo` ``), runner-config dotdir paths (`.claude/`-shaped,
excluding `.agent/`), and stack literals hardcoded from `scaffold/manifest.example.yml` instead of
referenced as `{{commands.<key>}}` — all structural, no maintained name list. `make`'s `test`
target no longer swallows `go test` failures.

## Not done — P1 status

1. **Template porting is done.** Nine of ten workflows are rendered output; `figma-restyle.md`
   stays hand-authored by deliberate scope decision (D053), not because it's blocked — there's no
   further prep or port to schedule for it. `check-rules`, `task-primitive`, `optimize-docs`,
   `execute-plan`, `plan-feature`, `sync-docs`, `fix-bug`, `rules`, `doc-philosophy` are all
   C2-green.
2. **Exit criterion met** (revised): "nine of the project's ten workflows are rendered output" —
   see `roadmap P1`'s Exit line. `make render` is the only writer; C2 guards the rendered set.
3. **Distribution (D006/D008):** resolved as D047 for now (dev clone). The roadmap's "cloned to
   `~/.cinch`" line becomes true at first release — add an install/refresh step to release.sh
   then, or keep consumers on the dev clone and update the roadmap wording.
4. **Red checks:** none — `make check-harness` green.

## Next session

P1's template-porting work is closed out. What's left in Phase 1 is the D047 distribution deferral
(not a blocker, revisit at first release) — otherwise Phase 1's stated exit criteria hold. The
roadmap's session-loads table names Phase 2 (`task-primitive.md` extraction, D039 compaction
anchor, D036 dogfooding) as the next numbered phase; note that `templates/task-primitive.md`
already exists from P1 session 2; the next session should check the roadmap P2 section for what
Phase 2 still expects (shrinking `plan-feature`/`fix-bug` to their P1/P2 fronts, deleting rather
than updating `fix-bug`'s differences table) before assuming P2 starts from zero.

## Notes for future template work

Template porting is done for P1, but these mechanics apply to any future edit of an existing
template or a revisit of `figma-restyle.md`:

- **Harness-neutrality is part of any port (D045).** A hand-authored source may open with an
  italic "Claude Code exposes this as …" line and reference workflows by slash command (`/rules`,
  …) — both are Claude Code bindings: drop the line and rewrite backtick-prose references as
  `.agent/workflows/<name>.md` paths. `make test` (D049) mechanically catches slash-command
  references and hardcoded stack commands; it will **not** catch the italic line itself (prose, no
  structural token) — still needs eyes. **The sibling-reference rewrite applies to backtick prose
  only** — an actual markdown link (`[text](target.md)`) must stay relative to the linking file's
  own directory, since C11's `checkLinks` resolves link targets that way, not repo-root-relative;
  rewriting a real link to a `.agent/workflows/…` form breaks it (session 4's `sync-docs.md`
  lesson, confirmed again reading `rules.md`'s `[Doc Philosophy](doc-philosophy.md)` link this
  session — left untouched, correctly).
- **A scaffold-contract key earns its place by a landing template, not a prospective one (session
  5 lesson).** `check-frontend` was added to `scaffold/manifest.example.yml` in prep for
  `figma-restyle.md`, then reverted when that port was scoped out — don't add a manifest key ahead
  of the template that will actually use it; add it in the same session the template lands (D051's
  original pattern), or not at all.
- Templates resolve next to the binary (`~/code/cinch/templates`) — edit them in the cinch repo,
  then run `make render` in projectX. No sync step exists; do not introduce a second copy.
- `templates/` may be nested (render walks recursively) — keep flat unless a port needs grouping.
- The scaffold `manifest.example.yml` is the variable contract (D040/D045): a template may use any
  key it declares. New keys needed by a port must be added to the example *and* the consumer
  manifest, and C3/C4-style schema thinking applies.
