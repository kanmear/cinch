# 31. Conventions sweep, 2026-11-01: drift, variants, and what belongs in a pack

**Tier 3 — decide before investing. Status: data, not a plan.** This is the re-run of the
2026-10-09 sweep that plan 30 asked for before building anything. Six read-only passes over
17 repos (git flow and scripts, Makefile loop, Go backend, SvelteKit and Flutter, agent
harness and docs) plus an adversarial review of plans 28, 30 and the Forum CLI proposal
(`~/code/forum/.docs/plans/forum-cli.md`, `.agent/` on the day). Hashes and grep counts were taken on the day;
the picks are proposals for you to accept, reject or merge. Each practice lists its variants
with the repos that use them, then one pick.

Repos: `~/code` budgetron, cinch, lucky_to_have_you (lthy), ortho-store (ortho),
project_deltadocs (dd), forum, verus, talk-workbench, deltadocs_landing, odin-landing;
`~/work` fsed-odin-docs, fsed-odin-mono, geobyte-ots-frontend (Angular), geobyte-ots-backend
(Python), odin_operator, odin_web, odin. forum and verus were copied from lthy on
2026-11-01, so they count as lthy's lineage.

## 1. Drift: what actually differs, and whether it hurt

Byte-identical everywhere they appear: the three `.githooks` shims, `cinch-log.sh`,
`inject-agents.sh`, `.claude/settings.json`, `mise.toml`, `docs-philosophy.md` (8 of 9), the
prettier config (7 of 8 Svelte repos), the sv-create eslint config (4 of 4 that have it).

| Artifact | Variants | What differs | Harm |
|---|---|---|---|
| `check-squash-merge-msg.sh` | 3 in 6 | dd: comments and the doc path in one error line. cinch: 36-line rewrite with no `main`/hotfix branch. | none felt; cinch's skips squashes onto unlisted branches |
| `post-commit-version-bump.sh` | 6 in 7 | ortho gates on `frontend/` not `frontend/src/`; dd runs node without the mise PATH; verus is single-service; cinch and budgetron use a different bump table (`refactor` = patch, `chore` = none, direct `fix:` = hotfix letter) | a config-only frontend change bumps in ortho; two bump semantics coexist, which is a decision, not drift |
| `release.sh` | 5 in 7 | tag template per service or plain `v`; cinch has `set -euo pipefail` under `#!/bin/sh` (works only because sh is bash here); budgetron has the `set -eu` fix | latent, on another shell |
| `update-version.js` | 5 in 6 | dd formats the hotfix letter without `+` and tags `backend-v0.2.0a`, which is not semver; lthy and ortho fixed it; dd's docs even document the wrong form | real: dd's tags are wrong, nothing consumes them yet |
| `check_error_codes.sh` | 6 in 6 | three families (typed `errors.ts` sync; `@throws` JSDoc sync; fsed's ru-only plus mobile map); ortho's regex admits digits, dd's does not | project contracts; dd misses codes with digits |
| `step.sh` | 2 in 3 | one comment word | none |
| `check-git-setup.sh` | 3 in 5 | the doc path in the warning text | none |
| `.claude/.gitignore` | 2 in 5 | ortho and dd lack `worktrees/` | accidental |
| rendered workflows | 3 sets | ortho and dd stale on `docs-audit-coverage` (two lines); fsed-mono fully old at pin `0.5.0+b` | ortho and dd would fail `generated` on re-render |
| `AGENTS.md`, `conventions.md`, `git-conventions.md` | hand-edited everywhere | 40 to 160 lines between any two | intended; the skeleton is shared |
| Makefile lthy to forum | 161 of 471 lines | parameters, dropped seed targets, added token targets; forum's `deploy-setup` scps `.env.production`, which does not exist (lthy uses `.env.production.example`) | forum deploy will fail until fixed |
| dev ports | | forum kept lthy's 8080/5173, so the two cannot run side by side without overrides | will be felt the first time both run |
| `budgetron` `commit.pattern` | | accepts `merge: <branch>` at any time, so a squash committed as `feat:` never bumps | latent (plan 28's motivating case) |
| `~/.local/bin/cinch` | | a stale 0.6.1 beside the 0.13.2 mise shim; the shim wins on PATH, but a hook shell without mise finds the old one | latent |

Verdict: one wrong-tag bug (dd), one deploy defect (forum), a handful of latent shell issues,
and zero incidents in any git log attributable to a drifted copy. The copies change for local
reasons that are correct where they are (per-service versions, bun vs node, ru-only locales).

## 2. Doc-versus-code drift (the drift cinch's thesis is about, and does not catch)

| Repo | Doc says | Code does |
|---|---|---|
| forum `architecture.md:55` | deletes are soft (`deleted_at`) | 0 hits for `deleted_at` (copied from lthy) |
| forum `conventions.md:80` | every list `ORDER BY` ends in `rowid` | `ORDER BY … id` in `models/tokens.go:153`, `users.go:108`; 0 `rowid` |
| lthy `conventions.md` | same `rowid` rule | 7 of 15 `ORDER BY` lines have no tie-break (`wishes.go:87`, `tasks.go:76`, `plans.go:87`, `pictures.go:90`, `categories.go:112`) |
| lthy `architecture.md:19` | `devdata` is a pure package | `devdata/seed.go` makes 91 `models.` references |
| lthy and forum `architecture.md:4` | structure follows deltadocs | dd uses Postgres, bcrypt, testify, WebSocket, sentinel errors, no contexts; only the layer names match |
| dd `conventions.md` | never call `os.Getenv` directly | `scripts/seed/seed.go:80`, `seed_ru/seed.go:85`, `import-docs/main.go:66` |
| dd, fsed-docs, fsed-mono | (nothing stated) | 140, 107 and 77 database calls without a context; lthy, ortho, forum, verus have 0 |
| ortho `conventions.md` | sentinels live in `errors/app_errors.go` | `models/customer.go:28` declares `ErrClaimEmailSent` |
| cinch `.docs/conventions.md` | repo-root param is always `repoRoot`; one grouped import block; `t.Run` in every table test; spelled-out names | 43 signatures use bare `root` (`upgrade.go:10`, `init.go:29`, `impact.go:204`); 5 bare imports (`selfupdate_exec_unix.go:5`); `selfupdate_test.go`, `impact_test.go` loop without `t.Run`; `rules.go:475` has `buf` |
| budgetron `conventions.md` | no state created in `build` | `category_color_selection_popup.dart` creates a `ValueNotifier` in `build` |
| odin_operator `CLAUDE.md` | never `DateTime.parse`, always `DateFormatter` | 4 call sites still parse; 76 `DateTime.now()` |
| fsed-docs `frontend/conventions.md` | import groups `core, components, stores, api, i18n, local` | 11 of 71 labelled files break the order; an unlisted `// utils` group in 10 |
| fsed-mono `conventions.md` | `ru/` and `en/` both required | fsed-docs is `SUPPORTED_LOCALES = ['ru']`; mono is the stale copy |
| dd frontend | bun with `--frozen-lockfile` | a tracked `package-lock.json` from 2026-06-08 beside `bun.lock` |
| geobyte `.clinerules` | signals, `input()`/`output()`, `@if`, pnpm | 301 `@Input`/`@Output`, 125 files with `*ngIf`, npm lockfile; rules cover new code only |

This is plan 10's territory (`kind: normative vs descriptive`): convention docs are the
part of the corpus nothing checks. The marker closure covers rule docs only.

## 3. Decision lists

Each line: the practice, the variants with their repos, then the pick. Mark your choice.

### 3.1 Git flow and scripts

**Branch model.** (A) dev/main, topic squash as `merge: <branch>`, release `--no-ff` to main,
`hotfix/` off main: lthy, forum, verus, ortho, dd. (B) dev/main without a hotfix branch, a direct
`fix:` on dev bumps the letter: cinch, budgetron. (C) trunk on main with prefixed branches: fsed
pair, odin_operator, odin_web, geobyte. Pick: A for solo repos that ship versioned builds, C for
team repos; B is A without the hotfix branch and can be a parameter.

**Commit grammar.** (A) nine types `feat fix ui docs style refactor test chore perf`, any
`[scope]`, enforced by script: lineage, cinch. (B) same plus `!`, scope `[a-z+-]+`, enforced by
`commit.pattern`: budgetron, odin branches. (C) `feat fix chore docs refactor test wip misc
style` with `!`: fsed. (D) none: geobyte, odin_*. Pick: A plus `!` from B; ortho and dd docs
still describe an older `<code location> - <desc>` form and should be corrected.

**Bump table.** (A) `fix` = patch, `feature|refactor|chore` = minor, `hotfix/` = letter: lineage.
(B) `fix|refactor` = patch, `feature` = minor, `chore|docs` = none, direct `fix:`/`refactor:` on
dev = letter: cinch, budgetron. (C) every commit bumps patch, rolls to minor at 10:
odin_operator, odin_web. Pick: B. A refactor is not a feature and a chore should not bump.
This means changing the lineage scripts, so decide it before plan 28 Part A ships its
defaults.

**Version storage.** (A) four integer files per service: lineage, cinch. (B) `pubspec.yaml`
`x.y.z+build`: budgetron. (C) one `version.ts`/`version.dart`: odin_*. Pick: A for code repos,
B for Flutter; storage stays per repo in every design.

**Release tag.** (A) `<svc>-vX.Y.Z[+a]`: lthy, forum, ortho. (B) `<svc>-vX.Y.Za`: dd (bug).
(C) `vX.Y.Z[+letter]`: cinch; `v0.12.1+8` budgetron. (D) `verus-vX.Y.Z`: verus. Pick: A for
multi-service, C for single-service; fix dd.

**Squash check script.** (A) lineage 68-line: dev and main windows, `MERGE_HEAD` exempt.
(B) cinch 36-line: dev only. Pick: A with `!`, then retire both through plan 28 Part A.

**Version-bump script.** (A) lthy/forum 107-line multi-service with mise PATH. (B) verus
single-service. (C) cinch/budgetron single-version with the B bump table. Pick: A as base with
the bump table as a case block; or skip the merge and go straight to plan 28 Part A plus
leaving Part B's logic in the scripts.

**release.sh.** Pick: lthy's (preconditions, `IN_SHIP`, `git branch -f dev main`),
parametrised by tag template and services; apply budgetron's `set -eu` to cinch's.

**check-git-setup.sh.** Pick: the ortho/dd text citing a rule ID, with no repo-specific doc
path, so the file can be identical everywhere.

### 3.2 Makefile and developer loop

**help.** (A) hand-written grouped echo block: lthy, forum, verus, ortho, dd, fsed, odin_web,
talk-workbench. (B) same with `.DEFAULT_GOAL := help`: landings. (C) script-generated: geobyte.
(D) none: budgetron, cinch, geobyte-backend, odin_*. Pick: A plus B's default goal.

**dev / down.** (A) tmux split, `-j2` fallback without a TTY, `GO_PORT`/`VITE_PORT`, session
named by port, `down` kills session and compose: lthy, forum. (B) tmux split, fixed session
name, no `down`: ortho, dd, fsed, odin_web. (C) one foreground process with `PORT`: verus,
landings. (D) backgrounded with `trap 'kill 0'`: talk-workbench. (E) node orchestrator with
PID file: geobyte. (F) compose only: odin, geobyte-backend. Pick: A for monorepos, C for
single-process repos; the rest is stack-driven.

**check / test / format.** (A) `check` static and read-only (`svelte-check`, `go vet`,
`gofmt -l`, error-code sync), `test` separate, `format` writes: lthy, forum, verus, ortho.
(B) `check` runs `go fmt` and rewrites files: dd. (C) `check` includes tests: budgetron (via
merge-time script), geobyte-backend. Pick: A; fix dd.

**Hook PATH and harness targets.** (A) exported PATH prepend (mise, `~/.local/bin`, bun, go),
`CINCH ?= cinch`, `setup-hooks`, `render`, `check-harness`, `ignores`, `install` chaining: lthy
lineage, ortho. (B) PATH without mise: dd. (C) `command -v cinch` guard: fsed. (D) none:
budgetron, cinch, others. Pick: A plus C's guard.

**Git flow in make.** (A) `merge-dev`, `release-cut`, `push`, `ship` with `step.sh` quiet
lines: lthy lineage. (B) `release` only, manual merge: ortho, dd. (C) `release` plus
`push-release`: budgetron, cinch. Pick: A; `step.sh` has no repo content and can be identical.

**Deploy.** (A) `deploy-setup` (bare repo on lab, `.env`), `deploy` (clean-tree and main gate,
pre-deploy backup, `git push -f lab`, compose rebuild, 90 s health poll), `prod-*` helpers,
`backup-pull`: lthy, forum, verus. (B) prod compose overlay: geobyte-backend. (C) Django
pipeline: odin. Pick: A, parametrised by host, path, port, env file name.

**Data targets.** SQLite: lthy's `create-owner`, `reset-password`, `reset-dev-data [yes=1]`,
`seed`, `fresh-dev-data`. Postgres: ortho/dd's goose-pinned `dev-db`, `stop-db`, `drop-db`,
`migrate-*`, `add-test-data`. Pick: both, by DB; no merge needed.

**Env.** Pick: lthy's exported absolute `DB_PATH`, `ENV_FILE=$(CURDIR)/.env`, `.env.example`
copied on install, a separate `.env.production.example`. Fix forum's `deploy-setup`.

### 3.3 Go backend

**Routing.** (A) stdlib mux, a `[]Route{Pattern, Handler, Public, Access|Scopes}` table, `Mount`
gates by default, route-gate test: lthy, forum. (B) `RegisterRoutes` with per-line wrapper
closures: ortho, dd. (C) `RequireAuth` around the mux with a public map: fsed. Pick: A.

**Layering.** (A) closure handlers, `apperrors.NotFound` returned, `inTransaction(ctx, fn)` in
models, `now int64` passed in, viewer scope as an add-on: lthy, forum. (B) handler structs,
`(nil, nil)` for not found, `time.Now()` inline: ortho, dd, fsed. Pick: A.

**Error contract.** (A) typed `Code` implementing `error`, `StatusFor(code)` in one table,
short names: lthy, forum, verus. (B) string consts with the status passed at each call,
domain-prefixed names, sentinel errors in `app_errors.go`: ortho, dd, fsed. Pick: A; keep
domain prefixes past roughly five domains.

**Logging.** (A) 16-line slog package, validated `LOG_FORMAT`: lineage. (B) 224-line package
with colour console, `LOG_FILE`, daily rotation: ortho, dd, fsed-docs. Pick: A; add file
output only where a deployment needs it.

**Database.** SQLite family: modernc, embedded `NNN_*.sql` under `PRAGMA user_version`, text
UUID, unix seconds, `deleted_at`, `rowid` tie-break, one writer, WAL. Postgres family: lib/pq,
goose, serial ids, `TIMESTAMPTZ` pinned to UTC, `id` tie-break. Context calls: lthy 109/0,
ortho 128/0, forum 27/0, verus 7/0, dd 1/140, fsed-docs 0/107, fsed-mono 0/77. Pick: always
`*Context`; everything else follows the DB.

**Config.** (A) own `.env` parser in `config/`, `ENV_FILE`, never overrides set vars, nothing
else reads env: lineage. (B) godotenv in `main.go`, `.env.local` then `.env`: ortho, dd, fsed.
Pick: A.

**Auth.** (A) httpOnly Lax cookie, sha256-stored session, scrypt, exact `Origin` check, Go
serves the SPA with a hashed CSP: lthy, forum (plus bearer scopes). (B) cookie, bcrypt, CORS to
a separate frontend host: ortho, dd, fsed. Pick: A where same-origin; B follows from a separate
frontend host.

**Realtime.** (A) in-memory SSE hub, 25 s ping, audiences only when visibility differs:
lthy, forum. (B) gorilla WebSocket per project: dd. (C) Centrifugo: fsed. Pick: A.

**Tests.** (A) stdlib, `cases`/`tc` with `t.Run`, `tests/<pkg>/`, markers above the
assertion, `t.Context()`, no `t.Parallel`: lthy, forum, verus. (B) testify, markers above the
`func`: ortho, dd. Pick: A; Postgres repos keep `RequireDB`/`SetupTestDB`.

**Style doc.** Only cinch's `.docs/conventions.md` is detailed; app repos say "no comments
unless necessary"; fsed-docs has the strongest comment rule (a why, an invariant, or a rule
pointer). Pick: cinch's plus fsed's comment rule, and fix cinch's own violations (section 2).

**Subcommands and backup.** (A) one `main.go` switch: `serve`, `healthcheck`, `create-owner`,
`reset-password`, `backup` (`VACUUM INTO`): lineage. (B) separate `cmd/` or `scripts/<name>`
binaries, no backup, `GET /api/health`: ortho, dd. Pick: A for SQLite; Postgres keeps `cmd/`
and needs a `pg_dump` target if backups are wanted.

### 3.4 SvelteKit

**Rendering.** (A) adapter-static, `fallback: index.html`, `ssr=false`: lthy, forum,
talk-workbench. (B) adapter-node with `ssr=false`: dd, fsed-docs. (C) adapter-node with real
SSR and `kit.csp`: ortho (storefront SEO). (D) adapter-vercel: fsed-mono. (E) prerendered
static: landings. Pick: A for anything a Go or Bun backend serves; C only for SEO.

**Tests.** (A) `bun:test`, no vitest; forum adds a `bunfig.toml` svelte preload for component
tests: lthy, forum, talk-workbench. (B) vitest 4 with jsdom and `$app` stubs: ortho, dd, fsed.
(C) Playwright: ortho (smoke), dd (smoke and deep), talk-workbench. Pick: A, with C where an
e2e tier exists.

**Styling.** (A) hand CSS, `tokens.css`, no hex outside tokens, `data-theme`, self-hosted
fonts, in-house `Icon`: lthy, forum. (B) Tailwind 4 plus DaisyUI with a custom theme: ortho.
(C) Tailwind 4 plus DaisyUI, Figma-generated `ui-*` tokens, migrating off DaisyUI: dd.
(D) Tailwind 4 only with hand `ui-*` tokens: talk-workbench. (E) Tailwind plus DaisyUI built-in
themes: fsed. Pick: D as the merge (keeps A's token rule, drops DaisyUI). Give lthy and forum
the shared prettier and eslint files; they have neither today.

**API client and errors.** (A) `createHttpClient(fetch)`, typed `ErrorCode` const, unknown
code becomes `INTERNAL`, `createSubmitter()` with real `<form onsubmit>`, `invalidateAll`:
lthy, forum. (B) same client plus `@throws {ApiError} CODE` JSDoc and `throwLoadError`: ortho,
dd. (C) B plus `ENDPOINTS` consts and snake_case wire types: fsed. Pick: A merged with B's
`@throws` and `throwLoadError`.

**i18n.** (A) typed `Dict` in `en.ts`/`ru.ts`, parity enforced by the compiler: lthy, forum.
(B) sveltekit-i18n, en only: ortho. (C) sveltekit-i18n, en and ru: dd, talk-workbench,
fsed-mono. (D) sveltekit-i18n, ru only: fsed-docs. (E) ru source, en optional: landings.
Pick: A for apps without route-scoped namespaces; locale policy stays one explicit line per
repo.

**Realtime client and PWA.** lthy and forum are identical and the only ones. Pick: as is.

**Components.** (A) `ui/`, `layout/`, `components/<feature>/`, `ui/index.ts` barrel: lthy,
forum. (B) `ui/` and `domain/` plus route-local `components/`, barrels only for api, utils,
constants, types, `ROUTES` const with `resolve()`: ortho. (C) `ui/`, `layout/`, route-local,
promote on the second route, `[name].svelte.ts` logic classes, dev galleries: dd. Pick: A plus
C's promotion rule and B's `ROUTES` const.

**Style rules.** (A) labelled import groups `core, api, stores, components, utils, local` and
script order `imports, interfaces, $props, $state, $derived, $effect, functions`: dd, ortho,
talk-workbench. (B) `core, components, stores, api, i18n, local`: fsed. (C) none: lthy, forum.
Pick: A.

**Dependencies.** (A) zero runtime deps, approval rule: lthy, forum. (B) approval rule with
sveltekit-i18n, lucide, DaisyUI: ortho, budgetron. (C) free: dd, fsed. Pick: the approval rule
everywhere; zero deps stays an lthy/forum choice.

### 3.5 Flutter

**State.** (A) Cubit-first, sealed Equatable states, failures as enum keys: budgetron.
(B) Bloc for multi-trigger lists, Cubit for detail, `AuthRedirectException`: fsed mobile.
(C) Bloc and HydratedBloc only, no sealed states: odin_operator. Pick: A, with B's split
stated as the same rule.

**Clock.** (A) `package:clock` injected: budgetron. (B) own `Clock` abstraction: fsed.
(C) 76 `DateTime.now()`: odin_operator. Pick: A.

**Lints and strings.** (A) `flutter_lints` 6 plus six extra rules and
`tool/check_hardcoded_strings.dart`: budgetron. (B) `flutter_lints` 6 bare: fsed.
(C) `flutter_lints` 4 bare: odin_operator. Pick: A, ported.

**Storage, l10n.** Per product (ObjectBox, sqflite, prefs; en-only vs ru-only). Carry over only
enums by name and money as integer minor units.

### 3.6 Agent harness and docs

**Entry file.** (A) tiered `AGENTS.md` (Critical Rules, every session, working in an area,
domain work, before implementing, fixing bugs, orientation, documentation structure): lthy,
forum, ortho, dd, fsed. (B) `cinch init` pointer block, 10 to 14 lines: budgetron, cinch.
(C) read-order list: verus. (D) flat notes or a 450-line style guide: landings, talk-workbench,
odin_operator. (E) `.clinerules`: geobyte. Pick: A with the B block inside it, plus
`CLAUDE.md` containing `@AGENTS.md` (lthy's `CLAUDE.md` holds only the model line and does not
import `AGENTS.md`).

**Docs root.** (A) `.agent/`: lthy, forum, verus, ortho, dd, fsed. (B) `.docs/`: budgetron,
cinch. Pick: B for new repos (it is `cinch init`'s default and dd's TODO already plans the
rename); do not rename live repos for its own sake.

**manifest.yml.** Present in seven repos, read by nothing since plan 17 trimmed the workflow
pack; every `cinch.yml` says so. Pick: drop it from new repos; ports and env vars live in
`architecture.md` and the Makefile.

**Rules.** Pick: lthy/forum's scheme (`rule_prefix` plus `owns`, drafts as bullets in
`plans/v1-domain-rules.md`, every `cinch:ignore` with a reason); fsed's "never ignore to paper
over a gap" stated once.

**Plan files.** (A) gen-2 task block with Layers, Atomicity rationale, Context: lthy, forum.
(B) current template: Status, Depends on, Files, Rules, Verify, `## Handoff`: ortho, dd,
budgetron, cinch. (C) one-line task header: verus. (D) checklist bullets: fsed-docs. Pick: B
(it is what `cinch workflow dev-plan-feature` prints); forum's `AGENTS.md` says "Session
Handoff" where the template says `## Handoff`.

**Status line.** `Status: complete` (lthy lineage, fsed), `**Status:** complete` (ortho, dd,
budgetron), `**Status: shipped in …**` (cinch). The hook accepts all three. Pick: one form,
`**Status:** <word>`, so a cross-repo reader needs one regex.

**Workflows.** Pick: rendered only; move fsed's `onboard.md` and dd's `figma-restyle.md` out
of `workflows/`; re-render ortho and dd; archive fsed-mono.

**Claude Code config.** Pick: forum's `.claude/` (settings, two hooks, `.gitignore` with
`plans/` and `worktrees/`), with `check-git-setup.sh` reduced to one text.

**Session discipline.** The "Sonnet implements, Opus plans" line is in forum, verus and lthy's
`CLAUDE.md` only. The one-task-per-session block is in forum and verus `AGENTS.md`. "Ask before
choosing" and the subagent rules are in gladius only. Pick: one shared block in the template
`AGENTS.md`. Resolve first: forum and verus `AGENTS.md` say "commit, update the handoff, stop";
the rendered `dev-execute-plan.md` says "propose a commit message, don't commit unless asked".

**Docs philosophy.** The rendered `docs-philosophy.md` is the only full text; five
`conventions.md` files paraphrase it. Pick: the rendered file alone.

**Backlog.** (A) root `TODO.md`: dd. (B) roadmap `## Deferred` and `## Open questions` with a
default per question: forum, verus. (C) `00-overview.md` with "cut, and why": cinch.
(D) `plans/deferred-scope.md` with reopen triggers: fsed. Pick: B, with C's reopen conditions.

**Stated in more than one place.** Git rules (conventions.md § Git, git-conventions.md, two
script headers, cinch.yml comments, check-git-setup.sh text, GIT-001). Service facts
(manifest, architecture.md, Makefile, compose, .env.example). `AGENTS.md` is injected by the
hook and read natively by the host, so forum and verus load it twice.

## 4. Pack, template, or repo-local

| Candidate | Where it belongs | Parameters it would need |
|---|---|---|
| `step.sh`, `cinch-log.sh`, `inject-agents.sh`, `.claude/settings.json`, `mise.toml`, prettier, eslint | template, identical copies | none |
| `check-git-setup.sh` | template, once the text cites a rule ID | none |
| `check-squash-merge-msg.sh` | cinch config (plan 28 Part A) | squash prefixes, hotfix on/off, types, `!` |
| `post-commit-version-bump.sh` | stays a script; bump table as a case block | bump table, service path gates, runner |
| `release.sh` | template with two variables | tag template, services, version command |
| `update-version.js` / `.sh` | repo-local | storage differs per repo |
| `check_error_codes.sh` | repo-local | project contract |
| Makefile | template with a parameter block | name, ports, services, dev runner, host, env file |
| `AGENTS.md` skeleton, `CLAUDE.md`, sessions block, `.docs/` skeleton, roadmap and task templates | template, seeded once | docs path, stack |
| backend, frontend, Flutter conventions docs | template, seeded once, built from lthy (Go, Svelte) and budgetron (Flutter) with the merges in section 3 | stack |
| `git-conventions.md` | template, seeded once | branch model variant, bump table |

Three file modes (managed, seeded, block) are only needed if a mechanism keeps copies in sync
after seeding. With a template, every file is seeded; what the repo edits afterwards is the
repo's. The one cheap mechanism worth considering is a template version line in seeded files
so `cinch check` can warn (not fail) that a seeded file predates the template; that is the
`generated` check in warn mode, not a pack engine.

## 5. Review: what the adversarial pass concluded

Read before deciding anything in section 3. Its claims about the stale binary and the plan
counts were verified by hand.

- The drift table in section 1 is mostly comments and local parameters. No git log shows a
  failure caused by a drifted copy. The felt costs are elsewhere: a wrong tag format in dd, a
  deploy that will fail in forum, a port collision, and 13 plans marked draft or in-progress
  in fsed-odin-docs (9 more in the superseded fsed-odin-mono).
- Making a new repo is one agent task: forum's harness copy was trimmed in one session, verus's
  in one. That is a template problem, not an engine problem. Plan 30's engine (vendoring, lock,
  three modes, layering, `requires`, four commands, five rules, a packs repo, a submodule on two
  machines) is weeks of work with no failure it prevents, and decision 4 (overrides for shared
  repos) is the single requirement that forces most of it, for repos where your conventions
  should not be the base anyway.
- Plan 30 decision 5 (push agents to run `cinch session`, `cinch impact`, `cinch rules --json`
  instead of grep) contradicts `~/code/cinch_bench/FINDINGS.md` §2 and the overview's "if grep
  can answer it, don't build it". Decision 6 (widen cinch to carry conventions) changes the
  tool that plan 29 is still measuring.
- Plan 28 Part A is small, tier 1, and closes a real hole (budgetron's pattern, cinch's
  unlisted-branch skip). Keep it. Part B moves drift from bash to YAML and the writers stay
  per repo regardless; defer it until a bump actually misfires.
- The Forum CLI is right in shape but every path is F1's API, which is not designed. Keep the
  plan, mark it blocked on F1, cut `today` (the phone list already is that), and demote plan
  sync to an open line until F5 and V3 exist.
- What was missing from the list: a cross-repo "where am I" that reads each repo's branch,
  dirty flag, last commit and the in-progress plan's status and resume lines. That is the
  dashboard, it is a script in `~/.config`, and it needs neither Forum nor a cinch change. If
  Forum later wants it, the same script posts the same lines.
- Decide explicitly which repos get aligned: the solo ones and fsed-odin-docs. geobyte and
  odin_* belong to teams.

Suggested order: answer plan 29's paid questions; delete `~/.local/bin/cinch`; write the
`where` script; fix dd's tag format, forum's `deploy-setup` and ports, dd's `go fmt` check;
build the template from forum's and verus's trees with section 3's picks folded in; ship plan
28 Part A; mark fsed-odin-docs' plans honestly; return to the Forum CLI after F1.

## 6. Decisions (2026-11-01, user's picks)

The picks from section 3 stand as marked on the review page, with these merges and answers:

**Git flow.** Branch model A. Commit grammar: nine types plus `!`, scope `[a-z+-]+`;
enforced by `commit.pattern` once plan 28 Part A ships, by the script until then. Bump
table B, extended: `fix/`, `refactor/`, `perf/`, `ui/` = patch; `feature/` = minor;
`chore/`, `docs/` = none; direct `fix:`/`refactor:` on dev = hotfix letter. Release tag
`<svc>-vX.Y.Z[+a]` for multi-service repos, `vX.Y.Z[+a]` for single-service; one template
parameter. `release.sh` is lthy's with `set -eu` and the tag template parametrised.
`check-git-setup.sh` cites the rule ID, no repo path. Plan 28: Part A ships with the grammar
and table above as defaults; Part B deferred, the bump stays a script.

**Makefile.** help A with `.DEFAULT_GOAL`; dev/down A for Go+SvelteKit repos, C for
single-process repos; a port pair per repo assigned in the template (lthy keeps 8080/5173);
check static and read-only; PATH prepend plus a `command -v cinch` guard; deploy and env as
lthy. Dev ports, one pair per Go+SvelteKit repo: lthy 8080/5173 (unchanged), forum 8081/5174,
deltadocs 8082/5175; ortho sets none in its Makefile; verus 3300 (no frontend).

**Go.** All A. Database calls: `*Context` is a rule for new code; each repo with context-less
calls (dd, fsed) gets a backfill task in its own plan, done when that repo is next worked on.
Style doc: cinch's conventions.md plus the comment rule (a why, an invariant, or a rule
pointer).

**SvelteKit.** Rendering A. Tests: bun:test; Playwright opt-in where e2e exists. Styling D:
Tailwind 4 with hand `ui-*` tokens; DaisyUI dropped from the approved list, stays only where
it is until migrated. API client A plus `@throws` JSDoc and `throwLoadError`. i18n: typed
`Dict` for small apps (lthy, forum, verus), sveltekit-i18n en+ru for large ones (ortho, dd,
fsed, talk-workbench); one locale line per repo. Components: route-local first, promote to
`components/<feature>/` on the second route, `ui/index.ts` barrel, `ROUTES` const with
`resolve()`, `[name].svelte.ts` logic classes; no `domain/`, no dev galleries. Style rules A.
Dependencies: approval rule everywhere, approved list sveltekit-i18n and lucide.

**Flutter.** All A.

**Harness.** Tiered AGENTS.md with the `cinch init` pointer block as one tier. Docs root
`.docs/`; forum and verus rename `.agent/` now and drop `manifest.yml` (ports and env vars
move to architecture.md and the Makefile). Plan task block and status line as the current
template. Session discipline: propose a commit message, do not commit unless asked; forum
and verus AGENTS change to match. Backlog: roadmap Deferred with cut, why and reopen
condition, plus Open questions with a default each.

**Scope.** Aligned: cinch, lthy, forum, verus, deltadocs, budgetron, gladius, ortho,
fsed-odin-docs (its 13 draft/in-progress plans get triaged). Not aligned: geobyte, odin_*.

**Plan 30.** Cut. Replaced by a template repo at `~/code/harness-template` (built 2026-11-02:
`apply.sh`, `common/`, `stacks/{go,go-svelte,flutter}`), pinned as a `~/.config` submodule,
applied once per repo by `make apply REPO=… STACK=… NAME=…`; seeded files carry a template
version line so `cinch check` can warn (not fail) when a seeded file predates the template.
Reopen condition: a git log in any aligned repo showing a failure caused by a drifted copy.

**Forum CLI plan.** Accepted as blocked on F1; `today` cut; plan sync demoted to an open
line. Same-binary and the F2 redefinition stay under "Decisions this needs" until F1 lands.
CAP-008 stays draft.

**Where am I.** A `where` script in `~/.config/bin`, terminal output: per repo, branch,
dirty state, ahead/behind, plan status and handoff line. No Forum involvement until F5.

**Plan 29.** Judge calibration (about 156 Sonnet calls) approved. Full judging and the Sonnet
stage 1 of the upkeep ablation still wait. Before any push, private-repo content in
`bench/recover/questions/deltadocs.json`, `bench/drift/private/`, `bench/field/hook-log.csv`
and `bench/drift/README.md` is moved out of the tree and the README cites the public fixture.

**Bug fixes.** Now, in cinch: `release.sh` shebang versus `set -o pipefail`, the 43 bare
`root` and 5 bare-import signatures, the two tests without `t.Run`, `buf`; and the stale
`~/.local/bin/cinch` 0.6.1 is deleted. The deltadocs, ortho, budgetron and forum fixes are
recorded in each repo's roadmap Deferred section and done in that repo's next session.

### Order

1. Plan 29 judge calibration (approved); scrub the private bench files.
2. cinch's own fixes and the stale binary; plan 28 Part A with the defaults above.
3. `where` script.
4. Template repo from forum/verus trimmed trees with the merges above; forum and verus
   `.agent/` to `.docs/`, manifest dropped, AGENTS commit text changed.
5. Roadmap Deferred entries for the other repos' bugs; fsed-odin-docs plan triage.
6. Forum F0/F1; the CLI plan reopens after F1.

## Session Handoff

**Last completed:** 2026-11-02. Steps 1 to 5 of the order above are built; nothing is committed
in any repo (the user commits). The cinch work sits on two branches:

- `feature/commit-squash-merge` (this branch, off `dev`): plan 28 Part A (`commit.squash`,
  `commit.merge`, rules CINCH-015 and CINCH-016, `scripts/check-squash-merge-msg.sh` deleted,
  the `commit:` block in `cinch.yml`), the step-2 fixes (bare `root` parameters, bare imports,
  `buf`, the two tests without `t.Run`, `release.sh` on `set -eu`), the bump table in
  `scripts/post-commit-version-bump.sh` (`perf/`, `ui/` = patch), docs (git-conventions, rules,
  README, plans 00, 28, 30, 31) and the regenerated catch matrix. `go vet`, `go test` and
  `cinch check` pass. Committed; squash onto `dev` as
  `merge: feature/commit-squash-merge` (minor bump):
  `feat [hooks]: squash- and merge-aware commit check (plan 28 Part A); conventions fixes`.
- `chore/bench-upkeep-ablation`: step 1 (judge calibration under `prereg/recover-amend0`, the
  private scrub into gitignored `private/` dirs, `bench/leak` style fixes) is committed there
  as `chore [bench]: judge calibration passed under prereg/recover-amend0; private-repo content
  moved to gitignored private/`, tagged `prereg/recover-amend0`. Never push that branch (its
  history holds the scrubbed files); squash-merge it. Plan 29's Status paragraph and `bench/recover/README.md` § 0 carry
  the result (156 rows, set (a) 98.5% after amend0).
- `.git/info/exclude` holds local lines that hide the held and private bench material on
  branches that lack the bench branch's `.gitignore` files. Keep them; they are not committed.
- The installed binary (`make install`, mise shim) is a dev build with Part A, reporting
  0.13.2; a release cut is what carries Part A to the other machines.

Other repos, all uncommitted: forum and verus (`chore [harness]: docs root .docs, manifest
folded into docs, propose-only sessions`), `~/code/harness-template` (initial commit; then
create `lab:harness-template.git`, push, and `git submodule add` it in `~/.config`),
`~/.config` (`bin/where`, `where/roots`; the user adds `fish_add_path -aP ~/.config/bin` to
fish config), fsed-odin-docs plan triage, and the Deferred entries in ortho, budgetron and
deltadocs. The template README's "Choices made while building" and "Gaps" sections list what
the agents decided on their own; read them before the first real `make apply`.

**Resume at:** both cinch branches are committed (2026-11-02, the bench stash popped, tag
`prereg/recover-amend0` set). Squash-merge them onto `dev`, this one first so `dev` gets Part A
before the bench branch's cinch.yml excludes land on top. Then step 6: Forum F0/F1, after
which the Forum CLI plan reopens. Still waiting on an explicit go-ahead: plan 29's full
judging (about 1,030 Sonnet calls) and the Sonnet stage 1 of the upkeep ablation (90 runs);
the free Qwen recoverability runs may start any time.
