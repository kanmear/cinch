# 32a. Plan lifecycle: what to align in each repo

**Status:** in-progress

These are the owner's steps for plan 32's T5 to T18, one section per repo. They were written on
2026-11-17 from a fresh inventory of every consumer: branch, dirty files, the installed
`generated`/`core` result, and what `cinch check --only plans` reports with the key on. The
decisions behind each step (D1–D8, Q1–Q8) are in [32-plan-lifecycle.md](32-plan-lifecycle.md).

How to use it: start a session in the repo and point it at this file: "align this repo per
`~/code/cinch/.docs/plans/32-plan-lifecycle-alignment.md` § <repo>". The session proposes the
triage list, you confirm it, and the session does the moves and proposes a commit message. Tick
the repo off here when it's merged.

**Changed 2026-11-17 by [plan 33](33-harness-alignment.md):** § 0 stays as written and comes
first. The per-repo triage (§ 1 and § 2, step 3 below) no longer runs on its own. It is step 3
of plan 33's per-repo pass (§ 5.1 there), after the harness-template v2 work, so each repo is
visited once. In that pass:
- lthy, ortho and dd rename `.agent` → `.docs` first, so read their paths below as `.docs`.
- The branch is plan 33's `chore/harness-v2`, not `chore/plan-lifecycle`.
- The triage commit is folded into plan 33's single commit.

harness-template (T6 here) is unchanged and comes before plan 33's Part B.

## 0. Once, before any repo: install, then upgrade every repo in the same sitting

The new cinch changes the rendered workflows. From the moment it's installed, the pre-commit
`generated` check fails in every repo that hasn't run `cinch upgrade`. So steps 1 and 2 belong
together. Step 3 (the triage) can then happen per repo, on any day.

1. **cinch:** squash-merge `feature/plan-lifecycle` onto `dev`, with the subject
   `merge: feature/plan-lifecycle`. That's a minor bump, to 0.15.0. Then run `make install`;
   `cinch version` should print 0.15.0.
   - If you commit in these repos on the other machine too, give it 0.15.0 there as well:
     `make release` and `make push-release`, or build from source. Step 2 raises every repo's
     floor to 0.15.0, so the `core` check fails on an older binary.
2. **Upgrade pass, every repo in the checklist below:**
   - On a `chore/` branch off `dev` (fsed: its own flow), run `cinch upgrade`. It finds no newer
     GitHub release (the latest is v0.9.0), so it leaves the local build alone. It re-renders the
     workflows and raises `require.cinch` to `>=0.15.0`.
   - Stage only the rendered files and `cinch.yml`, then commit
     `chore [harness]: cinch upgrade (plan lifecycle workflows)` and merge.
   - dd and ortho are red on `generated` today; this step fixes them.
   - **Branches and worktrees cut before this step** still carry the old renders, so their
     commits fail `generated` until they merge `dev` or run `cinch render`. Known ones:
     - pugio's current branch `ui/lock-cover-look`;
     - fsed's worktree `fsed-odin-docs-feedback-wt` (`docs/client-feedback-page`);
     - cinch's `.claude/worktrees/cli-visual-pass`.
3. **Triage, per repo:** the common steps in § 1 plus that repo's section.

Checklist (tick when merged):

| Repo | Docs root | Step 2: upgrade | Step 3: triage |
|---|---|---|---|
| cinch | `.docs` | [x] (T5: nothing to render) | [ ] |
| harness-template | `common/.docs` | n/a | [x] (T6, `eea6f58`) |
| budgetron | `.docs` | [x] | [ ] |
| forum | `.docs` | [x] | [x] (plan 33 T14, `ac20601`) |
| verus | `.docs` | [x] | [ ] |
| pugio | `.docs` | [x] | [ ] |
| limen | `.docs` | [x] | [x] (plan 33 T13, `74f7f43`) |
| lthy (`lucky_to_have_you`) | `.agent` → `.docs` | [x] | [x] (plan 33 T16, `784dbd8`) |
| ortho (`ortho-store`) | `.agent` | [x] | [ ] |
| dd (`project_deltadocs`) | `.agent` | [x] | [ ] |
| fsed-odin-docs (`~/work`) | `.agent` | [x] | [ ] |

Some repos are left out:
- budgetron-recurring-payments and limen-bench are retired (Q5).
- fsed-odin-mono stays as it is (Q3).
- limen-hortus-preview no longer exists: no directory and no worktree. Their `generated` turns
  red after the install, and nothing is done about it.

## 1. Common steps in every repo (step 3)

`<docs>` is the repo's docs root.

1. Branch off `dev` (fsed: its own flow) as `chore/plan-lifecycle`. Leave unrelated dirty files
   alone.
2. Add the key to `cinch.yml`:
   ```yaml
   plans:
     lifecycle: enforce
   ```
   If `cinch.yml` has an explicit `hooks.pre-commit-checks` list, add `plans` to it. Only cinch
   has one.
   Run `cinch check --only plans`. Its findings are the work list, and they come back green when
   the work is done.
3. Triage every file in `plans/` and `plans/fix/` with the repo's section below. The session
   proposes, and you confirm the list.
   - **Finished plan:** its status becomes `**Status:** complete — <short note>`.
   - **Dropped plan:** `**Status:** cut — <reason>`.
   - **Live plan:** `planned` or `in-progress`, in the bold form, with nothing glued to the word.
     `in-progress.` and `Complete` fail, and so does plain `Status:`.
4. Owed human checks go to `<docs>/manual-checks.md`, before archiving.
   - Create the file with a `# Manual checks` title if it's missing.
   - Add one line per check: what to check, and where (a `testing.md` section if one exists).
   - Link the plan **at its current path**. The archive step then rewrites that link to
     `plans/archive/`.
   - For a fix plan, name the fix instead of linking it, because the file gets deleted.
5. `cinch plan archive <docs>/plans/a.md <docs>/plans/b.md …` for every finished or cut plan,
   in one command. Plans that link to each other then end up as sibling links in `archive/`.
6. Delete finished fix plans from `plans/fix/` once their checks are in `manual-checks.md`.
7. The non-plans (D4):
   - **Roadmap or overview:** `cinch mv <docs>/plans/<file> <docs>/roadmap.md`. Do this before
     step 5, so the archive step rewrites the roadmap's links to the plans it archives.
   - **Spec or contract:** `cinch mv` it to a durable place under `<docs>/`.
   - **Register:** fold its entries into the roadmap's `## Deferred`, each with a reopen
     condition. Fix the links to it by hand (`cinch index --links-to plans/<file>` lists them),
     then delete it.
   - **`TEMPLATE-plan.md`:** delete it. `cinch workflow dev-plan-feature` prints the plan shape.
8. Fix references from outside the docs root, which `cinch mv` doesn't see: `grep -rn
   'plans/' AGENTS.md CLAUDE.md README.md`. The usual one is AGENTS.md's "read
   `<docs>/plans/v1-roadmap.md`" line, which becomes `<docs>/roadmap.md`.
   - `.claude/hooks/inject-agents.sh` needs no change (D7). Its `plans/*.md` glob never reaches
     `archive/`.
9. `cinch check` should be green. Commit
   `chore [plans]: plan lifecycle — archive finished plans, roadmap to docs root` (adjust it to
   what changed), then merge.
10. Check the next session's start. The hook's "active plans" list should show only `planned`
    and `in-progress` plans.

## 2. Per repo

### cinch (`~/code/cinch`): plan 32 T5

- `cinch.yml`: add the key, and add `plans` to `pre-commit-checks: [links, rules, index, core]`.
- `cinch mv .docs/plans/00-overview.md .docs/roadmap.md` **first**. Then fix the README link to
  `.docs/plans/00-overview.md`; it's outside the docs root.
- 14 plans have no status line, and most others use the old `**Status: shipped in X.**` form.
  This proposal follows the overview's table; confirm each:
  - **complete:** 01, 02, 03 (Part A and the seeded battery; anything still open goes to the
    roadmap), 04, 05 (H1 shipped, reachability dropped), 06, 08, 11–18, 22, 23 (decided:
    Budgetron), 26, 27, 28 (Part A shipped; Part B to the roadmap's Deferred), 31 (decisions
    applied — first copy its open steps into the roadmap: release cut, Forum F0/F1, plan 29's
    paid runs).
  - **cut:** 30 (cut 2026-11-01, replaced by harness-template).
  - **Your call:**
    - 07: the cheap half is done and the expensive half is open. Keep it `planned`, or move the
      open half to Deferred and archive it.
    - 09, 10, 20, 24: open. Each is `planned` or goes to Deferred.
    - 21: the umbrella for 22–25. Archive it as complete or keep it.
  - **in-progress:** 19 (field log collecting), 25 (Sonnet stage pending), 29, 32, and this file
    until every repo is done.
- Carry plan 32 § 8's "One docs root; no `.conventions/`" bullet into the roadmap's "What's
  deliberately not on this list".
- No `manual-checks.md` is needed unless a plan names a human check.

### harness-template (`~/code/harness-template`): plan 32 T6

- It has no commits yet. First commit the tree as it is (Q4), so the changes below show up as
  their own diff.
- `common/cinch.yml`: add the key, and raise `require.cinch` to `">=0.15.0"`.
- Delete `common/.docs/plans/TEMPLATE-plan.md`.
- Move `common/.docs/plans/v1-roadmap.md` to `common/.docs/roadmap.md` with `git mv`. The
  `{{DOCS}}` placeholders keep `cinch mv` out of this repo. Keep its `## Deferred` and
  `## Open questions`, and drop the status line, because a doc at the docs root needs only a
  `# ` title.
- Add a `common/.docs/manual-checks.md` skeleton with a `# Manual checks` title and one
  placeholder line.
- Update the references:
  - `common/AGENTS.md:21`: the roadmap path becomes `{{DOCS}}/roadmap.md`.
  - `common/AGENTS.md:42`: "plan files start from `TEMPLATE-plan.md`" becomes "`cinch workflow
    dev-plan-feature` prints the plan shape".
  - `README.md:82`, the layout block.
  - Any `stacks/*/files.txt` or `apply.sh` lines that name the removed files.
- `make check` should be green, with every stack applying and nothing missing.

### budgetron (`~/code/budgetron`): on `dev`, clean, floor `>=0.13.2`

- `cinch mv .docs/plans/00-overview.md .docs/roadmap.md` first. README.md and AGENTS.md both
  reference `plans/`; fix them.
- Plans 01–16 are all finished. Most say "device checks pending", and those checks go to
  `manual-checks.md` (new).
  - Words to normalise: 02 `Complete.`, 04 `complete.`, 05 `Done except T11`, 07–09 `Complete`.
  - 05's T11 (retire the legacy columns, a later release) goes to the roadmap's Deferred.
  - 11's "signed-release verification (T3, T5)" goes to `manual-checks.md`.
  - 09 and 10 had optional tasks skipped. A note after ` — ` is enough.
- Archive 01–16 in one `cinch plan archive` run. Their "Depends on [plan N]" links become
  sibling links.
- Delete `TEMPLATE-plan.md`, and check AGENTS.md for a mention.

### forum (`~/code/forum`): on `dev`, clean, floor `>=0.9.0`

- `cinch mv .docs/plans/v1-roadmap.md .docs/roadmap.md` first.
  - It still says "their manual checks on lab are…"; move those lines to `manual-checks.md`.
  - AGENTS.md lines 19–20 point at the old path.
- f0–f4 are done, so mark them `complete` and archive them.
  - f0 uses plain `Status:`.
  - f2's manual check is pending, and f3's agent-stopped case is open. Both go to
    `manual-checks.md`.
  - AGENTS.md lines 45–46 cite `f0-scaffold.md` and `f1-captures.md`. Fix those paths after
    archiving.
- `forum-cli.md` is blocked on F1 (plan 31). Make it `**Status:** planned — blocked on F1`, or
  move it to the roadmap's Deferred.
- `v1-domain-rules.md` (draft rules) and `assistant-contract.md` go to durable homes with
  `cinch mv`.
  - Suggested targets: `.docs/domain/draft-rules.md`, and `.docs/api/assistant-contract.md`
    beside `api/assistant.md` (merge them if they overlap).
  - AGENTS.md line 36 cites `v1-domain-rules.md`.
- `later.md` is a register: fold it into the roadmap's `## Deferred` and delete it.

### verus (`~/code/verus`): on `dev`, clean, floor `>=0.9.0`

- `cinch mv .docs/plans/v1-roadmap.md .docs/roadmap.md` first, and fix AGENTS.md.
- `v0-scaffold.md` says "done (merge into dev pending)". Confirm it's merged, then mark it
  `complete` and archive it.
- `v1-on-forum.md` is `in-progress`, already in the right form. It stays.
- `v1-domain-rules.md` only has AI-009 left, as a draft with an ignore. Move it to a durable
  draft-rules doc with `cinch mv`, or fold AI-009 into `domain/ai.md` as a draft.

### pugio (`~/code/pugio`): on `ui/lock-cover-look`, clean, floor `>=0.13.2`

- Merge or park `ui/lock-cover-look` first; see § 0 step 2.
- `cinch mv .docs/plans/v1-roadmap.md .docs/roadmap.md` first, and fix AGENTS.md.
- p0–p7, p9 and p10 are `complete` with phone checks owed, so those checks go to
  `manual-checks.md` (new).
  - Link the `testing.md` sections where they exist, e.g. p10 links `testing.md` § Lock cover.
  - Then archive all ten in one run.
- `p8-daily.md` is `in-progress`, and the phone week is yours. It stays.
- `fix/review-findings.md` and `fix/second-review.md` say "code complete; kept until the phone
  checks pass". Move their checks to `manual-checks.md`, naming each fix, then delete both.
- `review-decisions.md` is a closed record: `**Status:** complete — closed 2026-11-10`, then
  archive it.
- Delete `TEMPLATE-plan.md`.
- Afterwards, the session start should list only `p8-daily.md` (D7).

### limen (`~/code/limen`): on `dev`, clean, floor `>=0.13.2`

- `cinch mv .docs/plans/v1-roadmap.md .docs/roadmap.md` first, and fix AGENTS.md.
- **Archive as complete:**
  - l0–l3.
  - `mouse-select.md`: `complete.` with a period; normalise it.
  - `l5-tui.md`: "built, reviewed, merged, checked"; confirm.
- **`working-line.md`:** "built on `ui/ui-tweaks`, live checks and merge still yours". If it's
  merged, the checks go to `manual-checks.md` and it's archived as complete. If not, it's
  `in-progress`.
- **Live, normalise:** `native-tools.md` and `theme-picker.md` say `in-progress.` with a
  period; make it `**Status:** in-progress — …`.
- **Already passing:** at-file-mentions, calm-transcript, l6, l7, ui-tweaks and
  ux-conveniences (`planned …`).
- Delete `TEMPLATE-plan.md`.

### lthy (`~/code/lucky_to_have_you`): docs `.agent`, on `dev`, clean, floor `>=0.9.0`

- `plans/archive/` already exists with 15 files. m1–m8 there have no status line; add
  `**Status:** complete` near the top of each.
- `cinch mv .agent/plans/v1-roadmap.md .agent/roadmap.md` first, and fix AGENTS.md.
- `hidden-items-layout.md` and `lists.md` say "complete (manual verification pending)". The
  checks go to `manual-checks.md` (new), then archive both.
- `m10-go-backend.md` says `Status: complete (on dev; release and deploy pending)`. The release
  is a roadmap fact, so make it `**Status:** complete — …` and archive it.
- `m9-deploy.md` says `Status: in-progress`; make it the bold form.

### ortho (`~/code/ortho-store`): docs `.agent`, on `dev`, floor `>=0.10.0`

- **Dirty:** `.agent/plans/roadmap.md` holds plan 31 step 5's Deferred entries, never
  committed. Commit them first.
- `cinch mv .agent/plans/roadmap.md .agent/roadmap.md` first. At the docs root its status line
  is just prose.
- Archive the 10 complete plans, plus `foundation.md` (`done`, kept as the foundation record),
  as `**Status:** complete — …`.
- `fix/crm-claim-verification.md` is complete; delete it.

### dd (`~/code/project_deltadocs`): docs `.agent`, on `dev`, floor `>=0.9.0`

- **Dirty:** `TODO.md` holds plan 31 step 5's Deferred entries, never committed. Commit them
  first.
- Archive `email-invites`, `project-admins` and `tab-editor-live-editing` as complete.
- **Normalise to the bold form:**
  - `action-bar-state-model-refactor.md`: `Status: open` becomes `planned`.
  - `page-entry-declined-signature-dot.md`: `Status: in-progress`.
  - `realtime-target-scoping.md`: lowercase `status: in-progress`.
- Optional: plan 31 picked a roadmap with `## Deferred` for the backlog. Moving root `TODO.md`
  into `.agent/roadmap.md` can happen here or later.

### fsed-odin-docs (`~/work/fsed-odin-docs`): docs `.agent`, on `main`, floor `>=0.13.0`

- **Dirty:** the `mobile` submodule pointer, plus untracked `.agent/client-feedback/` and
  `dist/`. Leave them.
- `manual-checks.md` already exists, and there are 112 inbound links from 19 docs. `cinch mv`
  and `cinch plan archive` rewrite them.
- 47 files. Many use plain `Status:`.
- **Archive as complete:**
  - 13, 19, 20, 21, 22, 23 (the pointer bump goes to the roadmap or `manual-checks.md`)
  - akb-install-executor-pick, analytics-by-person, assigned-field, centrifugo-chat,
    channel-marks
  - e2e-tier, equipment-placement (its manual check goes to `manual-checks.md`),
    equipment-return, general-chat, meeting-quick-wins (pointers)
  - mobile-credential-cache (device run owed), mobile-ui-overhaul, multi-task-creation,
    password-change-admin, placement-form-ux, planned-movement-units
  - rule-coverage-audit, rule-implementation-audit, sap-sync-button, short-names-tables,
    stock-intake-topup, task-driven-movements, task-notifications, task-review,
    tasks-server-paging
- **Archive as cut:** 17, 18, release-tooling, square-colors.
- **Live, normalise:**
  - `in-progress`: 15 (Part 1 open), auth-bootstrap, chat-photos, client-feedback-2026-11,
    line-equipment-counting, map-layering, mobile-offline-tasks.
  - `planned`: mobile-office-tasks.
- **Non-plans:**
  - `deferred-scope.md` and `assigned-field-scope.md` (registers): fsed has no roadmap.
    Either create `.agent/roadmap.md` with a `## Deferred` section and fold them in, or
    `cinch mv` both to the docs root as they are, which keeps their links working.
  - `meeting-2026-10-29.md` (standing index): archive it as complete, or `cinch mv` it under
    `.agent/business/`.
  - `tech_spec.md`: `cinch mv .agent/plans/tech_spec.md .agent/tech_spec.md`.

## Handoff

**Last completed:** § 0 steps 1 and 2 (2026-11-17). cinch 0.15.0 is installed. Every repo in
the checklist ran `cinch upgrade` on `chore/cinch-upgrade-0.15` and merged it (fsed onto `main`).
cinch needed no commit because its renders were already current. Branches cut before the upgrade
(pugio's `ui/lock-cover-look`, fsed's `docs/client-feedback-page` worktree, cinch's
`cli-visual-pass` worktree) still carry the old renders.
**Resume at:** T6 (harness-template), then step 3 per repo inside plan 33's § 5.1 pass.
