# 30. Convention packs: portable conventions, layered and overridable

**Tier 3 — decide before investing. Status: draft, not approved.** Recovered on 2026-11-01 from
a Claude Code session in `~/.config` on 2026-10-09. The second draft was rejected and the
session stopped there, with nothing built. The source plan file is
`~/.claude/plans/lovely-humming-tome.md`. This file is that draft with your review folded in
and reconciled with what landed afterwards. No cost estimate yet.

## Provenance

**Field data.** A conventions sweep of every repo you author in `~/code` and `~/work`, on
2026-10-09 (summary at the end of this file). The same sweep is the source of
[28](28-absorb-shared-hook-scripts.md). Nothing here comes from the audit or the benchmarks.

## Why

The same conventions are copied into five to seven repos, and the copies drift:

| Copied artifact | State on 2026-10-09 |
|---|---|
| `post-commit-version-bump.sh` | 4 variants in 5 copies (5 in 5 by 2026-10-27, see 28) |
| `check-squash-merge-msg.sh` | 3 variants |
| `check_error_codes.sh` | 4 variants |
| `release.sh` | 4 variants |
| `.claude/hooks/inject-agents.sh` | identical everywhere, a hand-rolled copy of the cut `cinch context` |
| `AGENTS.md` / `.agent/` skeleton | rewritten by hand in each repo |

The goal is to make these conventions portable as **optional packs** that cinch installs:

- Packs layer: a `general` pack plus stack packs.
- A project can override them. This matters for repos other people also work on.
- Anyone can replace them with their own.

## Decisions already taken (review of the first draft, 2026-10-09)

1. Packs are synced between machines as a git submodule of `~/.config`, because you work
   across two machines.
2. Stack conventions are in scope, not just general ones.
3. The base pack is called `general` (or `global`), not after the author.
4. A project needs a way to override packs from `~/.config` with its own packs, for repos
   where you aren't the only one working. A better mechanism than the one below is welcome.
5. The `AGENTS.md` pointer should push agents harder to use all of cinch and to start each
   session with it.
6. cinch's thesis may change. It is a personal tool first, and it is allowed to grow.

Decision 6 means this item deliberately widens cinch from "checks docs against the repo" to
"carries a repo's conventions and checks them". It contradicts two entries in the
[overview](00-overview.md): the `cinch context` cut, whose reopen condition isn't met, and
the "no more workflow prose" line. Approving this item means rewriting both entries.

## What changed since the draft

- **Plan 28 took the two git scripts.** Its Part A is this draft's squash-aware commit check,
  with the same keys. Its Part B builds the version bump into cinch as config, instead of the
  managed script this draft put in the `general` pack. 28 is newer and was re-measured, so
  this item defers to it. Packs carry neither script.
- **Plan 28 keeps some scripts in the repos.** It keeps `release.sh` and `check_error_codes.sh`
  out of cinch. This draft
  ships the first two as managed pack files. That is still open, see below.
- **Rule IDs moved.** The draft proposed CINCH-011 to CINCH-015, but CINCH-011 to CINCH-014
  now exist. New rules take the next free IDs.
- **Plan numbers moved.** The draft named plan files 26, 27 and 28 for its three parts. Those
  numbers now hold other plans. This file covers the packs and `cinch session` parts. 28
  covers the commit check.

## Design

### 1. Where packs live: a packs repo, mounted as a submodule of `~/.config`

- **New repo `kanmear/cinch-packs`**, laid out as `packs/<name>/pack.yml` plus
  `packs/<name>/files/…`. It's separate from the cinch source, so another user forks the
  packs, not the tool. If you meant the cinch repo itself as the submodule, only the mount
  path changes.
- **Mounted at `~/.config/cinch`** as a submodule on every machine branch. The branches then
  share one pack history instead of cherry-picking files between them.
- cinch resolves a bare pack name through `$CINCH_PACKS`, then
  `${XDG_CONFIG_HOME:-~/.config}/cinch/packs/<name>`. A path or git URL also works.

### 2. Packs are vendored, so a clone never needs your `~/.config`

- `cinch pack add <name|path|url>` copies the pack into `.cinch/packs/<name>/`, which is
  committed.
- It records the pack in `.cinch/packs.lock`: name, source, pack version, source commit and a
  content hash.
- Rendering and the `generated` check read only the vendored copy. Teammates, CI and cloud
  sessions work offline, and CINCH-007 still holds.
- `cinch pack update [name]` re-copies from the source and re-renders. It refuses when the
  vendored copy no longer matches the lock hash, meaning someone edited it in the repo. The
  refusal points at `--force` or `fork`.
- `cinch pack fork <name>` sets the source to `local`. The pack becomes project-owned and is
  never updated from `~/.config` again.
- `cinch session` (section 6) reports packs whose source has a newer version.

### 3. Layering and overrides (decision 4)

`cinch.yml` lists packs in order, and a project pack goes last:

```yaml
packs: [general, go, sveltekit, go-sveltekit-errors, project]
vars:
  go.root: backend
pack.general.skip: [scripts/release.sh]
```

- **Variables.** Pack defaults, then later packs, then `cinch.yml` `vars`. The last wins.
- **Files and blocks.** For the same destination, the later pack wins. `pack.<name>.skip`
  drops single files.
- **Hooks.** Pack entries join `hooks.*` keyed `<pack>/<entry>`. A repo entry with the same
  key replaces the pack's, and `run: ''` disables it.
- **Project pack.** `.cinch/packs/project/` is authored in the repo with `source: local`. On a
  shared project (fsed-odin-docs, geobyte), team conventions live there, override your user
  packs, and get reviewed like any other code.
- **`requires:`** in `pack.yml`. For example, `go-sveltekit-errors` requires `go` and
  `sveltekit`. A missing requirement is a `cinch check` finding.

### 4. Three file modes

| Mode | Owner after install | Checked by `generated` | Use for |
|---|---|---|---|
| `managed` | cinch, header-stamped | yes | scripts, Claude hooks, lint helpers |
| `seeded` | the repo, copied once and never overwritten | no | `conventions.md`, `.prettierrc`, `analysis_options.yaml`, `.claude/settings.json` |
| `block` | cinch owns a `<!-- cinch:begin pack/id -->` … `<!-- cinch:end -->` region in a repo-owned file | the region only | `AGENTS.md` sections each pack contributes, and the cinch pointer |

Block mode lets a stack pack added later still add a "working in backend, read …" line to an
existing `AGENTS.md`.

### 5. Core changes

- **Pack engine**, in a new file under `internal/cinch/`.
  - `renderAll` (`internal/cinch/render.go:76`) gains pack sources after the embedded
    templates. `substitute` and `substitutionVars` are reused as they are.
  - Orphan removal (`orphanedGeneratedFiles`, CINCH-002) covers pack outputs.
  - `dispatchHooks` (`internal/cinch/hook.go:183`) merges pack hook entries.
- **Manifest.** `flattenSequence` accepts only scalar items, so `packs:` stays a scalar list
  and per-pack options use `pack.<name>.*` keys.
- **Commands.** `cinch pack add|update|fork|list`.
- **Depends on 28 Part A** for the commit-check keys the `general` pack sets defaults for.

New rules, at the next free CINCH IDs, each bound by a marker:

- Packs render only from the vendored copy.
- A seeded file is never overwritten.
- Render rewrites only the inside of a block; text outside it is untouched.
- `pack update` refuses when the vendored copy's hash doesn't match the lock.
- Later layers win.

### 6. Make agents use cinch: `cinch session` and a stronger pointer (decision 5)

**`cinch session`** brings back what `inject-agents.sh` re-implements by hand, and absorbs
`check-git-setup.sh`. It prints:

1. The branch, with a warning on `dev` or `main` or when `core.hooksPath` is unset.
2. Plans that carry a `Status:` line.
3. The workflow trigger table.
4. A rule-corpus summary.
5. Staged paths, with a `cinch impact` preview.
6. Packs that are outdated, or whose vendored copy was edited.

**The `AGENTS.md` pointer becomes a cinch-owned block**, updated on every render. Today
`ensureAgentPointer` (`internal/cinch/agents.go:41`) writes it once and then skips. Its text
tells agents to:

- start every session with `cinch session`;
- run `cinch impact <paths>` before editing, to load the rules those paths touch;
- use `cinch workflow <name>` for features, bugs and plans;
- use `cinch rules --json` and `cinch index` instead of grepping for docs;
- treat a passing `cinch check` as the bar for committing.

The `general` pack makes this automatic for Claude Code: it seeds `.claude/settings.json` with
a SessionStart hook that runs `cinch session`. Other agents rely on the `AGENTS.md` block.

### 7. Pack contents (v1)

**`general`**, for git flow and the agent harness.

- Seeded: `git-conventions.md`, and a cross-cutting `conventions.md`. The latter covers making
  from the repo root (TOOL-001 in the consumers), no direct commits to dev or main (GIT-001),
  dependency approval, no comments unless necessary, money as integer minor units, and a
  locale-policy line to fill in.
- Seeded: `docs-philosophy.md`, the tiered `AGENTS.md` skeleton and a manifest stub.
- Managed: `scripts/release.sh`, with tag format `{{release.tag-format}}` (`v{version}` or
  `{service}-v{version}`). Open, see below.
- Pack defaults for `commit.pattern`, `commit.squash.*` and `version.*` (28's keys), and for
  the hook entries.

**`go`**

- Seeded `backend/conventions.md`, merged from the best of deltadocs, ortho and lucky:
  method-pattern stdlib routes, repository interface plus private struct, `slog`, the
  `{"error":{"code"}}` envelope, the migration owns the schema while the Go struct owns the
  wire shape, and `*Context` database calls with `r.Context()`.
- Managed `scripts/check-go.sh`: `go vet` plus a read-only `gofmt -l` gate. This fixes
  deltadocs' check, which runs `go fmt` and rewrites files.
- A pre-commit hook with `when: {{go.root}}/`.

**`sveltekit`**

- Seeded `frontend/conventions.md`: runes only, script and import order, the API-call and
  load-error pattern, i18n.
- Seeded `.prettierrc` and `eslint.config.js`, matching what the repos already share.
- A pre-commit hook with `when: {{sveltekit.root}}/`.

**`flutter`**

- Seeded: budgetron's `.docs/conventions.md` (cubit first, sealed Equatable states, an
  injected `Clock`, enums stored by name, gen_l10n strings) and its stricter
  `analysis_options.yaml`.
- Managed `tool/check_hardcoded_strings.dart`, with a seeded allowlist.

**`go-sveltekit-errors`**, which requires `go` and `sveltekit`.

- Managed `scripts/check_error_codes.sh`, parametrized by paths and locale list. Open, see
  below.
- A seeded rule snippet stating the contract between error codes and translations.

## Implementation order

Each step is a `feature/` branch off `dev`, squashed as `merge: feature/…`.

1. **cinch:** 28 Part A, the squash-aware commit check. Rewrite the two contradicted overview
   entries in the same branch.
2. **cinch:** the pack engine, with vendoring, the lock, layering, the three modes, the hook
   merge and the `pack` commands (`feature/packs`).
3. **cinch:** `cinch session` and the block-mode `AGENTS.md` pointer
   (`feature/cinch-session`).
4. **cinch-packs:** create the repo and the five packs, built from the most evolved copies:
   deltadocs for general, Go and Svelte, budgetron for Flutter.
5. **`~/.config`**, in a worktree if another session is active there:
   - `git submodule add git@github.com:kanmear/cinch-packs.git cinch`;
   - whitelist `!/cinch` and `!/.gitmodules` in `.gitignore`;
   - add an `EXTERNAL-SETUP.md` entry for `git submodule update --init` and cinch on `PATH`;
   - repeat the submodule commit on every machine branch.
6. **Migrate the repos**, deleting their copied scripts. Start with ortho-store, the
   smallest. Then lucky, deltadocs, budgetron and cinch itself. Last is fsed-odin-docs with a
   `project` pack, which is the shared-repo override case.

## Verification / definition of done

1. **Unit tests**, table-driven, for: layer precedence across variables, files, hooks and
   skip; a seeded file never overwritten; a block region rewritten without touching the text
   around it; `pack update` refusing on a lock-hash mismatch. 28 owns the commit-check tests.
2. **Fresh repo.** `git init && cinch init && cinch pack add general && cinch pack add go`.
   Then: `cinch check` is green; editing a managed script gives a `generated` finding; editing
   a seeded doc gives none; editing inside the `AGENTS.md` block gives a finding and editing
   outside it doesn't.
3. **Fresh clone** of that repo with `XDG_CONFIG_HOME` pointed at an empty directory:
   `cinch check` and `cinch session` both work.
4. **Real flow in a migrated ortho-store.** Commit on a topic branch. Squash onto dev with
   `merge: feature/x`, and see the version bumped and amended. Merge dev into main with
   `--no-ff`, and see it exempt. Run `make release`, and see the tags. Start a Claude Code
   session, and see it receive the `cinch session` output.
5. **Override case.** In fsed-odin-docs, a `project` pack overrides the `general` commit-scope
   list, and `cinch pack update general` leaves that override intact.

## Open

- **`release.sh` and `check_error_codes.sh`: pack or repo?** 28 keeps both in the repos, as a
  rarely drifting script and a project contract. This draft makes both managed pack files.
  The sweep counted 4 variants of each, which argues for packs.
- **Override mechanism.** Decision 4 asked for "or a better mechanism". The project pack
  plus keyed hook replacement is the only candidate written down.
- **The sweep's data is from 2026-10-09.** Only the two git scripts were re-measured, on
  2026-10-27 by 28. Re-run the sweep before building the packs.

## Sweep summary, 2026-10-09

**Scope.** Twelve repos where you wrote most of the commits:

- `~/code`: budgetron, cinch, deltadocs_landing, lucky_to_have_you, odin-landing,
  ortho-store, project_deltadocs.
- `~/work`: fsed-odin-docs, fsed-odin-mono, geobyte-ots-frontend (about 88% of its commits are
  yours), odin_operator, odin_web.

Left out: geobyte-ots-backend and odin, where you wrote few or no commits, and the obsidian
notes vault.

**Three generations of agent docs**, each replacing the one before:

| Generation | Repos | Agent docs |
|---|---|---|
| 1 | odin_web, odin_operator, geobyte | One `CLAUDE.md` or `.clinerules` of 450–500 lines, with a project tree and generic advice |
| 2 | budgetron, cinch | A short `AGENTS.md` plus `.docs/`, managed by cinch |
| 3 | deltadocs, ortho, lucky, fsed-* | A tiered `AGENTS.md`, plus `.agent/` with a manifest, per-area docs, rules with IDs and `cinch:rule` markers, and the plan, fix and execute workflows |

**Shared across generation 3.** Go stdlib `net/http` with method-pattern routes, repository
interface plus private struct, `slog`, the `{"error":{"code"}}` envelope translated on the
frontend, and an error-code sync check. SvelteKit 5 with runes, Bun, and one `.prettierrc`
(tabs, single quotes, width 100). `make` from the repo root only, money as integer minor
units, minimal comments.

**Where the repos diverge, with the sweep's pick:**

1. **Git flow.** dev/main with squash-merges and auto bump (cinch, budgetron, lucky, ortho,
   deltadocs); trunk with manual tags (fsed-*); main with a bump on every commit (odin_web).
   Pick: trunk and manual tags for team repos; dev/main only for solo repos that ship
   versioned builds.
2. **Copied scripts drift**, as in the table above. Pick: plan 28.
3. **`make check`.** ortho, lucky and fsed-mono fail on `gofmt -l`. deltadocs runs `go fmt
   ./...`, which can never fail and rewrites unstaged files. budgetron's check also runs the
   tests. Pick: check is static, fast and read-only; test is separate.
4. **Commit types and scopes.** `ui`/`perf` against `misc`/`wip`; scopes `back`/`front`,
   `backend`/`frontend` or domain names; some repos validate scopes. Pick: one type list,
   `back`/`front` scopes, scope validation optional.
5. **Release tags.** Per-service prefixes (deltadocs), `v0.10.0+5` (budgetron), `v0.36.0`
   (lucky). lucky documents a bare hotfix letter, which cinch's docs say isn't valid semver.
   Pick: per-service prefixes only for multi-service repos, cinch's `+letter` form
   everywhere.
6. **Dependencies.** lucky has no frontend runtime dependencies and an explicit
   dependency-approval rule; the others share DaisyUI, `sveltekit-i18n`, Vitest, ESLint and
   Postgres with goose. Pick: keep the shared stack, but adopt lucky's approval rule
   everywhere.
7. **Locale policy** is stated four different ways. Pick: one explicit line per repo.
8. **Go tests.** testify (ortho, deltadocs) against stdlib only (lucky, fsed); no
   `t.Parallel()`. No backend passes a context to its database calls: about 490
   `Query`/`Exec` calls against almost no `QueryContext`. Pick: stdlib tests are fine; fix the
   context gap in all four backends.
9. **Flutter.** budgetron's conventions and lints are the strongest; odin_operator and fsed
   mobile use bare `flutter_lints`. Pick: port budgetron's.
10. **Stale bits.** fsed-odin-mono pins cinch to exactly `0.5.0+b` and is superseded, so
    archive it. geobyte's `.clinerules` says `pnpm` beside a `package-lock.json`. odin_web's
    `CLAUDE.md` carries a pasted 200-line Backlog.md manual. lucky has 26 local branches.
    ortho and deltadocs have the `check-git-setup.sh` SessionStart hook; lucky doesn't.

**The sweep's ranking:** absorb the squash check and bump into cinch (now 28); fix
deltadocs' `go fmt` check; pass contexts to database calls; make generation 3 the only
template; unify the dependency rule, locale line, commit types and tag format; port
budgetron's Flutter conventions.
