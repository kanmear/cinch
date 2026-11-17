# 32. Plan lifecycle: archive, one status vocabulary, owed checks out of plans

**Status:** planned

Written 2026-11-17 from a review of how cinch is used across `~/code` and `~/work`
(survey taken 2026-11-12, re-checked on 2026-11-17). The user asked for one
improvement in particular: `plans/` is messy, and maybe archived plans should get their own folder.
They should. But moving finished plans won't stay fixed unless the three causes below are
fixed at the same time. This file holds the evidence, the decisions with their reasons, the
rules, the tests and the tasks. A fresh session can work from this file alone.

## 1. What the survey found

There are 15 cinch consumers: 13 in `~/code` (cinch, budgetron, budgetron-recurring-payments,
forum, verus, pugio, lucky_to_have_you (lthy),
ortho-store (ortho), project_deltadocs (dd), harness-template/common) and 2 in `~/work`
(fsed-odin-docs, fsed-odin-mono). Together they hold about 235 plan files. The installed cinch
was 0.13.2.

### 1.1 Finished plans pile up

| Repo | Files in `plans/` | Actually live |
|---|---|---|
| fsed-odin-docs | 47 | 8. The rest: 31 complete or done, 4 cut, 3 registers or indexes, 1 spec |
| cinch | 32 | about 3 |
| lthy | 19 | 1 (`m9-deploy.md`) |
| budgetron | 18 | 0 (16 complete, `00-overview.md`, `TEMPLATE-plan.md`) |
| pugio | 16 | 1 (`p8-daily.md`) |
| budgetron-recurring-payments | 15 | 0 (the same 01–14 as budgetron) |
| ortho | 12 | 1 (`roadmap.md`). One complete fix plan was never deleted |
| forum | 10 | 0–2 (f0–f4 done; a proposal, a draft, a register) |

**Cause:** the rendered `dev-execute-plan.md` says, under Completion: *"A feature plan stays,
with `**Status:** complete`. A fix plan is deleted; git history is the archive."* Every repo
follows it, so `plans/` only ever grows.

### 1.2 Sessions pay for the clutter

The shared session-start hook `.claude/hooks/inject-agents.sh` (9 copies in two variants:
4 identical and 5 identical) prints every file under `plans/*.md` and `plans/fix/*.md`
that has a status line in its first 15 lines, all under the heading "active plans". In pugio
on 2026-11-12 that came to 16 lines, and only one of them was a plan in progress. The list
also included `TEMPLATE-plan.md` and a 600-character roadmap status. Every session in every
repo pays for this.

### 1.3 "Complete" usually means "complete, but you still owe a check"

The most common real state is "code done, owner's device or phone check still owed": 10 of 11
pugio phase plans, most of budgetron and budgetron-recurring-payments, several lthy and fsed
plans, and pugio's two `fix/` plans. pugio's fix plans say so outright: *"code complete; kept
until the owner's phone checks pass, then deleted"*. The owed check is the only reason these
files stay in `plans/`, and the status line is the only place it's recorded. fsed already has
a single list of owed checks (`.agent/manual-checks.md`, linked from `testing.md:111`).
No other repo does.

### 1.4 Status lines vary, so neither people nor scripts can read them reliably

- **Format.** Repos mix `**Status:** x` (ortho, dd, budgetron, pugio), `Status: x`
  (lthy lineage, much of fsed, forum, verus) and `**Status: shipped in …**` (cinch). In cinch,
  14 of 32 plans have no status line at all.
- **Words.** complete, Complete, done, Done, shipped, closed, open, proposal, draft, built,
  planned, in-progress, cut, "code complete", "only AI-009 is left".
- Plan 31 already picked one form, `**Status:** <word>`
  ([31-conventions-sweep.md](31-conventions-sweep.md) § 3, "Status line"). That pick was
  never applied, because nothing enforces it.

### 1.5 Files in `plans/` that aren't plans

- **Roadmaps and overviews:** `v1-roadmap.md` (lthy, pugio, forum, verus,
  harness-template), `00-overview.md` (cinch, budgetron, budgetron-recurring-payments),
  `roadmap.md` (ortho).
- **Templates:** `TEMPLATE-plan.md` (budgetron, pugio, harness-template).
  `cinch workflow dev-plan-feature` already prints the plan shape.
- **Registers:** fsed `deferred-scope.md` and `assigned-field-scope.md`, forum `later.md`.
  Plan 31 § 3 "Backlog" picked the roadmap's `## Deferred` section, with a reopen condition
  per entry, as their home.
- **Records and indexes:** fsed `meeting-2026-10-29.md`, pugio `review-decisions.md`, and
  cinch's own [31-conventions-sweep.md](31-conventions-sweep.md) ("data, not a plan").
- **Specs and contracts:** fsed `tech_spec.md`, forum `assistant-contract.md`,
  `v1-domain-rules.md` (forum, verus).

### 1.6 Durable docs link into `plans/`

fsed has 112 links into `plans/` from 19 docs outside it (`business/*.md`, `glossary.md`,
`manual-checks.md`, `mobile/architecture.md`, `frontend/conventions.md`, …). budgetron's plans
link to each other through their `Depends on [plan N](NN-….md)` lines. cinch's README links
`.docs/plans/00-overview.md`. So deleting finished plans would break links, and moving them
means rewriting links. cinch's `links` check already catches every link inside the docs
root that a move breaks.

### 1.7 How cinch treats `plans/` today

- `checkIndex` (`internal/cinch/introspect.go`, `checkIndex`) and `indexList` skip anything
  under `plans/`, so plans need no `# ` title and stay out of `cinch index`.
- The `index` check's `--changed` gating ignores markdown under `plans`
  (`internal/cinch/check.go`, the `planned` map).
- The link graph (`docLinks`, `internal/cinch/links.go`) deliberately includes `plans/`, both
  as link source and as link target.
- The rule parser reads every doc under the docs root, plans included. That's why the planning
  workflow forbids copying a rule item into a plan.
- No check reads plan status. The hook's `case` pattern is the only thing that parses it.

### 1.8 Other findings from the same survey (handled in the sweep or recorded in § 8)

- `cinch check` fails in **dd** and **ortho**: their `.agent/workflows/docs-audit-coverage.md`
  no longer matches a fresh render. Plan 31 noted this ("re-render ortho and dd") and it was
  never done.
- **fsed-odin-mono** is still pinned at `0.5.0+b`. It has 9 `generated` errors, `core.hooksPath`
  unset, and its last commit was on 2026-09-10. Plan 31 decided to archive it, and that never
  happened.
- **Floor pins:** dd, lthy, forum, verus and budgetron-recurring-payments require `>=0.9.0`. A
  floor lets a repo's rendered workflows fall behind quietly until `generated` turns red, as in
  dd and ortho.

## 2. Decisions and their reasoning

**D1. Finished plans move to `<docs>/plans/archive/`. They aren't deleted, and they don't go
to a sibling `<docs>/archive/`.**
- *Why not delete:* finished feature plans are linked from durable docs (fsed: 112 links) and
  record the "why" of decisions (cinch's own plans say "kept for provenance"). The fix-plan rule
  stays as it is: fix plans are deleted, and git history is their archive.
- *Why not only filter by status:* the directory listing is what agents and the hook actually
  read. A status filter would make every reader parse status. A directory makes `ls plans/`
  and `plans/*.md` correct without parsing anything.
- *Why under `plans/` rather than at the docs root:* the index exclusion, the title-check
  exclusion and the `--changed` gating all match on the `plans/` prefix, so an archive under
  `plans/` needs no change to them. The hook globs `plans/*.md` and `plans/fix/*.md`, so it
  stops listing archived plans with no edit at all.

**D2. One status line in one form, from a closed set of words.** The plan's status is
`**Status:** <word>` within the file's first 15 lines (the hook already uses that window; the
per-task `**Status:** [ ]` lines come later). Prose may follow after ` — `.
- Live words (in `plans/` and `plans/fix/`): `planned`, `in-progress`.
- Archived words (in `plans/archive/`): `complete`, `cut`.
- *Why strict:* it's plan 31's pick, a reader across repos needs only one regex, and the
  folder and the word confirm each other. Plain `Status:`, capitalised words and synonyms
  (done, shipped, closed, open) are all rejected.
- *Why not frontmatter `status:`:* about 200 existing files and the hook use the inline bold
  line, which also shows when the markdown is rendered. Frontmatter would mean migrating every
  file and every reader to get the same result.

**D3. Owed manual checks leave the plan.** When a plan's code tasks are done, every check its
Verify lines leave to a person becomes one line in `<docs>/manual-checks.md`, linking back to
the plan. The plan is then marked `complete` and archived right away. *Why:* "complete (checks
owed)" is the most common state (§ 1.3), and it's the reason plans linger. One list of owed
checks across the whole repo is easier to work through than ten status lines, and fsed
already works this way.
- *Fix plans:* a fix plan is deleted (D1), so its owed-check lines cite the fix commit, or the
  `testing.md` section the check comes from, instead of linking the plan. A link to the deleted
  file would break every time.
- *Leaving the list:* a check that passes deletes its line, and git history keeps it. A check
  that fails is a bug and goes through `dev-fix-bug`.
- *Rejected alternative:* a third live word, `verify`, so the plan stays in `plans/` until
  its checks pass. That's honest but keeps the clutter: pugio would show 10 `verify` plans at
  session start. **Confirmed at the checkpoint (Q1).**

**D4. Non-plans leave `plans/`.**
- Roadmap or overview → `<docs>/roadmap.md`. It's a durable doc with a title, so it appears in
  `cinch index`.
- Registers → the roadmap's `## Deferred` section, each with a reopen condition (plan 31).
- Records and indexes whose work is done → `plans/archive/` with `complete`.
- Specs and contracts that describe how things are → a durable doc at the docs root.
- `TEMPLATE-plan.md` → deleted.
Each file is decided during its repo's sweep. The agent proposes, the owner confirms. Each move
is done with `cinch mv` (D6), because a roadmap is linked from 5 to 14 files per repo (pugio
14, lthy 12, cinch 11, forum 10). Moving it from `plans/` to the docs root changes every
relative path to it, and the paths differ by depth: `plans/x.md` needs `../roadmap.md` and
`plans/archive/x.md` needs `../../roadmap.md`.

**D5. cinch enforces D1 and D2 with a new `plans` check, enabled by a manifest key:
`plans.lifecycle: enforce`.** Without the key, the check does nothing. *Why opt-in:* cinch is
installed globally through the mise shim, and `require.cinch` is only a floor. A check that is
on by default would turn all 14 consumers red on the next `make install`, before any of them
is migrated. Instead, each repo enables the key in its own sweep. Making it the default is a
later decision, outside this plan.

**D6. Two commands do the moves.**
- **`cinch mv <src> <dst>`** moves any `.md` file under the docs root to a new path under it.
  It rewrites every link to the file from docs in the link graph, plus the moved file's own
  relative links.
- **`cinch plan archive <file>...`** checks the status word, then does the same move into
  `plans/archive/`.

*Why:*
- fsed alone needs 35 archive moves and about 112 link rewrites, and budgetron's "Depends on"
  chains link plan to plan.
- D4's moves (roadmaps, specs, contracts) need the same rewriting at a changing depth.
- Doing that by hand is error-prone. `links` would catch what's missed, but only after the
  fact.

Both commands share one move routine. It uses cinch's existing link parser (so links inside
inline code stay untouched, as CINCH-005 requires) and its existing graph
(`index --links-to`).
- *Limit:* links from outside the docs root (`README.md`, `AGENTS.md`) aren't in the graph.
  The commands list none of them and the `links` check doesn't see them, so each repo's sweep
  greps for them.
- **Confirmed at the checkpoint (Q2):** build it, rather than moving files by hand with
  `git mv`. **Widened (Q7):** a general `cinch mv`, with `plan archive` built on it.

**D7. The hook needs no change.** Its glob never reaches `plans/archive/`. Once the non-plans
are gone, everything it lists is live. Check this in pugio after its sweep.

**D8. Plan naming stays as it is.** The prefixes `NN-`, `pN-`, `mN-`, `lN-`, `fN-` and plain
names all stay. With 1–8 live files per repo, order barely matters. The one wart, `p10`
sorting before `p2`, only shows up in the archive.

## 3. Rules

- **Existing:** CINCH-005 (link syntax inside inline code is not a link; the move's link rewrite
  must keep to it). CINCH-006 and CINCH-012 (pre-commit checks staged content; the new check
  must read the staged tree like the others). CINCH-008 (`cinch upgrade` raises a floor only
  when the rendered output changed; T4 changes the rendered output, so T5a's upgrade raises
  every floor). CINCH-001 and CINCH-002 (re-rendering after the template change).
- **New, to be written into [rules.md](../rules.md) after the checkpoint** (proposed text,
  not yet numbered items):
  - CINCH-017 (proposed): With `plans.lifecycle: enforce` in `cinch.yml`, every `.md` file
    directly in `<docs>/plans/` or `<docs>/plans/fix/` has a `**Status:**` line in its first
    15 lines whose status word is `planned` or `in-progress`. Every `.md` file in
    `<docs>/plans/archive/` has one whose status word is `complete` or `cut`. The status word is
    the text after `**Status:** ` up to the first whitespace or the end of the line, so
    `complete — shipped in X` and `complete (X)` pass and `complete, X` doesn't. Each violation
    is one `plans` finding. Without the key the check enforces nothing and prints a no-op line.
  - CINCH-018 (proposed): `cinch mv` moves a `.md` file under the docs root to a target under
    the docs root that doesn't exist yet. After the move, every relative link in the docs that
    resolved to the file, and every relative link inside it, still resolves. A refusal changes
    no file.
  - CINCH-019 (proposed): `cinch plan archive` moves only a plan whose status word is
    `complete` or `cut`, and only from `<docs>/plans/` (not from `fix/`, not from `archive/`),
    into `<docs>/plans/archive/`, with CINCH-018's guarantees. A refusal changes no file.
- **Ambiguities, settled at the checkpoint (Q6):** `plans/fix/` accepts both live words,
  `planned` and `in-progress`. A markdown file in any other subfolder of `plans/` is ignored;
  only the three named folders are checked.

## 4. Test plan

- [ ] CINCH-017: key unset, with a live plan that has no status line → no findings, and the
  no-op line is printed. `internal/cinch/plans_test.go`
- [ ] CINCH-017: live plan `**Status:** complete` → one finding naming the file and the
  allowed words. `plans_test.go`
- [ ] CINCH-017: archived plan `**Status:** in-progress` → one finding. `plans_test.go`
- [ ] CINCH-017: no status in the first 15 lines (the only one is on line 16) → finding. A
  per-task `**Status:** [x]` further down isn't read as the plan's status. `plans_test.go`
- [ ] CINCH-017: `Status: complete`, `**Status:** Complete`, `**Status:** done` and
  `**Status:** complete, shipped` in `archive/` → four findings. `**Status:** complete —
  shipped in abc123` and `**Status:** complete (abc123)` pass. `plans_test.go`
- [ ] CINCH-017: a file in `plans/fix/` is held to the live words (`planned` passes there).
  A statusless file in another subfolder, e.g. `plans/notes/x.md`, is not a finding.
  `plans_test.go`
- [ ] CINCH-017 with CINCH-006: the plan is valid on disk but staged with a bad status →
  `check --staged` reports it. `internal/cinch/staged_test.go`
- [ ] Gating (no rule): `check --changed` runs `plans` when markdown under `plans/` changed,
  and skips it with a skip line otherwise. `internal/cinch/check_test.go`
- [ ] CINCH-018: `cinch mv` moves the file. It rewrites an inbound link from a doc in another
  folder, a sibling's link, and the moved file's own outbound relative link, and keeps a
  `#fragment`. A link inside inline code is untouched. `checkLinks` is clean afterwards.
  `internal/cinch/move_test.go`
- [ ] CINCH-018: a move that changes depth (`plans/roadmap.md` → `roadmap.md`) rewrites links
  from `plans/` and from `plans/archive/` to their own correct relative paths. `move_test.go`
- [ ] CINCH-018: refusals for an existing target, and for a source or target outside the docs
  root. Each exits non-zero and leaves the tree byte-identical. `move_test.go`
- [ ] CINCH-019: archiving a `complete` plan lands it in `plans/archive/` with its links
  resolved. Refusals for an `in-progress` plan, a `fix/` plan, a path outside `plans/`, and an
  existing target leave the tree byte-identical. `internal/cinch/archive_test.go`
- [ ] Template (no rule): `templates_test.go` still passes. The rendered Completion section
  names `plans/archive/` and `manual-checks.md`. The rendered quality audit leaves out
  `plans/` and `roadmap.md`.

## 5. Tasks

### T1: Write CINCH-017, CINCH-018 and CINCH-019
**Status:** [x]
**Depends on:** the checkpoint (§ 7)
**Files:** `.docs/rules.md`. Add the three rules as confirmed, each followed by
`<!-- cinch:ignore: test lands in plan 32 T2 -->` (or `T3`).
**Rules:** CINCH-017, CINCH-018, CINCH-019
**Verify:** `cinch check` is green with the three ignores in place.

### T2: The `plans` check
**Status:** [x]
**Depends on:** T1
**Files:**
- New `internal/cinch/plans.go`: a status-line parser shared with T3, and `checkPlans`.
- `internal/cinch/check.go`: register `plans` in the `planned` map, `checkLaunchOrder` and
  `defaultPreCommitChecks`, with `--changed` relevance for markdown under `plans/`.
- `main.go`: add `plans` to the `--only` names in the usage text.
- README: add a row to the check table.
- The manifest key `plans.lifecycle` is read with `manifestVar`.
- Remove T1's ignore on CINCH-017.
**Rules:** CINCH-017 (with CINCH-006)
**Verify:** `go test ./...`. The CINCH-017 tests in § 4 carry `cinch:rule CINCH-017`.

### T3: `cinch mv` and `cinch plan archive`
**Status:** [x]
**Depends on:** T2 (status parser)
**Files:**
- `internal/cinch/move.go`: the move, plus link rewriting through `docLinks` and the
  `links.go` parser.
- `internal/cinch/archive.go`: the status and location checks, then the move.
- `main.go`: a `mv` command, and a `plan` command with an `archive` subcommand. Add both to
  usage and `commands`.
- README: document both commands.
- Remove T1's ignores on CINCH-018 and CINCH-019.
- Neither command stages or commits. Each prints one line per moved file and per rewritten
  file.
**Rules:** CINCH-018 (with CINCH-005), CINCH-019
**Verify:** `go test ./...`. The tests carry `cinch:rule CINCH-018` and `cinch:rule CINCH-019`.

### T4: Workflow templates describe the lifecycle
**Status:** [x]
**Depends on:** T2, T3
**Files:**
- `internal/cinch/docs/templates/dev-execute-plan.md`, Completion:
  - Owed manual checks go to `{{paths.docs}}/manual-checks.md`, one line each. The line links
    the plan; for a fix plan it cites the fix commit instead (D3).
  - Then set `**Status:** complete` and run `cinch plan archive`.
  - Fix plans are still deleted. Cut plans are archived with `cut` and a reason.
  - A passing check deletes its line; a failing one goes through `dev-fix-bug`.
- `dev-plan-feature.md` § 6: the status vocabulary, `planned` for a plan written ahead of its
  work, and the line that only plans live in `plans/` (roadmap → `{{paths.docs}}/roadmap.md`,
  registers → its `## Deferred`).
- `dev-fix-bug.md`: the same vocabulary.
- `docs-audit-quality.md`:
  - Steps 2 and 3 cover the docs `cinch index` lists, which already leaves out `plans/` and
    its `archive/`.
  - Leave out `workflows/` (owned by cinch) and `{{paths.docs}}/roadmap.md` (it records
    intent).
  - *Why:* step 3 now reads every `.md` except `workflows/`. Principle 5 would then delete the
    archived plans D1 keeps, and flag the forward-looking roadmap D4 moves into the corpus.
- The overview's "no more workflow prose" line binds here. Replace the old Completion text
  rather than adding beside it, and keep each template's growth to the lines the new
  vocabulary needs.
- Re-render `.docs/workflows/`.
**Rules:** none (prose). CINCH-001 and CINCH-002 govern the re-render.
**Verify:** `go test ./...` (templates_test) and `cinch check` green.

### T5: Install, and dogfood on cinch
**Status:** [ ]
**Depends on:** T4
**Files:**
- `make install`; `cinch version` shows the new build through the mise shim.
- `cinch.yml`:
  - Add `plans.lifecycle: enforce`.
  - Add `plans` to the explicit `pre-commit-checks` list. Without it, cinch's own hook never
    runs the check; CI would, because it runs the full set from source. cinch is the only
    consumer with an explicit list.
- `.docs/plans/`: give the 14 plans without one a status line. Normalise the rest
  (`**Status: shipped in X.**` → `**Status:** complete — shipped in X`). Archive everything
  shipped, done or cut.
- `cinch mv .docs/plans/00-overview.md .docs/roadmap.md`, then fix the README link, which is
  outside the graph; grep for it.
- Carry § 8's "one docs root" bullet into the roadmap's "What's deliberately not on this list".
- Plan 31 needs a call: its decisions are applied, but its Session Handoff is still a resume
  point. Archive it once its open steps are listed in the roadmap.
- This plan stays live.
**Rules:** CINCH-017, CINCH-018, CINCH-019 in use
**Verify:** `cinch check` green; `ls .docs/plans/` shows only live plans plus `archive/` and
`fix/` (if any); `grep -rn 'plans/00-overview' README.md AGENTS.md .docs` finds nothing.

### T5a: Upgrade pass across the consumers, right after the install
**Status:** [ ]
**Depends on:** T5, in the same sitting
**Files:** in every repo of the sweep table except the dropped ones (T11, T15, T19):
1. Note any uncommitted changes and leave them alone.
2. On a `chore/` branch off `dev`, run `cinch upgrade`. Decline its offer to install a
   release, because T5 already installed the build. The upgrade re-renders the workflows and
   raises the floor (CINCH-008).
3. Stage only the rendered files and `cinch.yml`, then propose
   `chore [harness]: cinch upgrade (plan lifecycle workflows)`.

No triage and no `plans.lifecycle` key yet.

*Why (Q8):* T4 changes the rendered workflows, and the pre-commit hook runs `generated` in
full: `checkGenerated` compares byte for byte with the installed binary's render, and it isn't
gated on changed files. From T5's `make install` onward, every commit in an unswept repo fails
until that repo upgrades. Upgrading every repo at once removes that window, so the triage
sessions below can come in any order.
**Rules:** CINCH-008
**Verify:** `cinch check --only generated,core` is green in every repo, which includes dd and
ortho, red today on their stale renders.

### T6: harness-template carries the lifecycle
**Status:** [ ]
**Depends on:** T5, and the template's first commit (Q4: commit the current tree as it is
first, so T6 shows up as its own diff).
**Files:** in `~/code/harness-template/common`:
- `cinch.yml` gets `plans.lifecycle: enforce`.
- Delete `.docs/plans/TEMPLATE-plan.md`.
- Move `.docs/plans/v1-roadmap.md` to `.docs/roadmap.md`, keeping its `## Deferred` and
  `## Open questions` sections.
- Add a `.docs/manual-checks.md` skeleton.
- `common/AGENTS.md`: line 21 (`plans/v1-roadmap.md` → `roadmap.md`) and line 42 (plan files
  start from `TEMPLATE-plan.md` → `cinch workflow dev-plan-feature` prints the shape).
- `README.md:82`: the layout block lists both removed files.
- `apply.sh` targets that copy the removed files.
**Rules:** none
**Verify:** `make check` in harness-template is green, and all three stacks apply with nothing
missing.

### T7–T18: Sweep each consumer (T11 and T15 dropped)
**Status:** [ ]
**Depends on:** T5a, T6 for repos created from the template
**Files:** one task per repo, ideally one session each. The same steps in every repo:
1. Note any uncommitted changes and leave them alone unless the owner says otherwise.
2. T5a already ran `cinch upgrade` here. Run it again only if cinch's templates changed since.
3. Set `plans.lifecycle: enforce`.
4. Triage every file in `plans/` using D3 and D4. Propose the list and get the owner's OK.
5. Move owed checks into `manual-checks.md`.
6. `cinch plan archive` the finished plans, `cinch mv` the D4 moves, and delete finished fix
   plans.
7. Grep `README.md`/`AGENTS.md` for `plans/` links.
8. `cinch check` green, then propose a commit message (the repos' AGENTS say don't commit
   unless asked).

The per-repo notes are in [32-plan-lifecycle-alignment.md](32-plan-lifecycle-alignment.md),
rebuilt from a fresh inventory on 2026-11-17. The owner runs T5, T5a, T6 and this sweep from
that file, one repo per session.

**Rules:** CINCH-017 green in each repo
**Verify:** in each repo `cinch check` is green, the live `plans/` holds only `planned` and
`in-progress` plans, and the session-start hook (where there is one) lists only those.

### T19: fsed-odin-mono — dropped
**Status:** [x]
**Depends on:** —
**Files:** none. Q3 answer: leave it alone. It stays a broken cinch consumer (pin `0.5.0+b`,
`generated` red, hooks off) outside the sweep. Plan 31's "archive fsed-mono" is not carried
out here.
**Rules:** none
**Verify:** nothing to verify.

## 6. Ordering and risk

T1–T5 are cinch-only. The new check is safe to install because of D5: nothing changes in
another repo until that repo sets the key. The re-render is not safe to install on its own.
T4 changes rendered output, so after T5's `make install` every consumer's pre-commit
`generated` check fails until that repo runs `cinch upgrade` (CINCH-008). T5a closes that
window: it upgrades every consumer in the same sitting as the install. After that, the
triage tasks have no ordering constraint. Start with the repos that have the most sessions
(fsed, pugio) only because they gain the most.

## 7. Checkpoint answers (2026-11-17)

- **Q1.** When a plan's code is done, archive it right away and move its owed checks to
  `manual-checks.md`. There's no `verify` status. (D3)
- **Q2.** Build `cinch plan archive`. (D6, T3, CINCH-018)
- **Q3.** Leave fsed-odin-mono alone. T19 is dropped.
- **Q4.** Give harness-template its first commit before T6.
- **Q6.** `plans/fix/` accepts `planned` and `in-progress`. Other subfolders of `plans/`
  are ignored.
- **Q7** (review, 2026-11-17). Make the move general: `cinch mv` for any doc under the docs
  root, with `plan archive` built on it. (D6, T3, CINCH-018 and CINCH-019)
- **Q8** (review, 2026-11-17). Upgrade every consumer right after the install, before any
  triage. (T5a)

## 8. Recorded, not in scope

- **Floor pins.** The sweep raises them all. Running `cinch upgrade` across every repo on a
  regular schedule (the repo list in `~/.config/where/roots` could drive it) would stop
  renders going stale again. That's a follow-up.
- **Two variants of `inject-agents.sh`** (4 and 5 copies). D7 needs neither changed.
  Reconciling them belongs to harness-template.
- **Making the `plans` check default-on,** after the sweep has proven the format holds.
- **One docs root; no `.conventions/` (decided 2026-11-17).** Splitting `.docs/` into business
  and architecture docs plus a `.conventions/` root for conventions and workflows was
  considered and rejected:
  - Every check runs over the one `paths.docs` root, and convention docs hold rules too
    (TOOL-001, GIT-001). A second root would change nothing about what gets caught.
  - Docs are grouped by area, not by kind. forum's `backend/` holds `architecture.md` and
    `conventions.md`, with about 20 links between the two kinds in both directions.
  - Where a file came from is already marked on the file: rendered (`workflows/` plus a
    header stamp), seeded (`harness-template vN`) or repo-written.
  - D4's `roadmap.md` and `manual-checks.md` fall on neither side.

  Reopen if conventions start being shared live (a submodule or a synced directory), which is
  plan 30's reopen condition, or if cinch needs to check the kinds differently (plan 10,
  `kind:`). T5 copies this bullet into the roadmap.
- **Doc-versus-code drift** in convention docs (plan 31 § 2, plan 10). Untouched here.

## Handoff

**Last completed:** T4. T1–T4 are committed on `feature/plan-lifecycle` (`5df2a33`).
**Verified:**
- `go vet . ./internal/...`, `go test ./internal/... .` and `go run . check` all pass,
  including `generated`.
- On a copy of pugio's docs with the key set, `plans` flagged the 15 expected files.
- `plan archive` moved four plans there, rewrote the domain docs' links into `archive/`, left
  `links` unchanged, and refused the live `p8-daily.md`.

**Resume at:** [32-plan-lifecycle-alignment.md](32-plan-lifecycle-alignment.md) § 0. The owner
merges, installs, and runs the upgrade pass in every repo in one sitting, then triages each repo
when convenient.
**Notes:**
