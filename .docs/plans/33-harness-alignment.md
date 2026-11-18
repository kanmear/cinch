# 33. Harness alignment: trunk flow, one version, one make contract, template v2

**Status:** in-progress

Written 2026-11-17 from a read-only sweep of every repo in `~/code`, `~/work/fsed-odin-docs`
and `~/.local/share/gladius` (five parallel passes, load-bearing claims re-checked by hand).
The owner asked for aligned make commands (`make down` kills the dev tmux session, `make dev`
opens tmux when it needs several shells), aligned versioning, merging, releasing and deploying,
and `.docs` as the docs root in every personal repo. The decisions in § 2 were confirmed in that
session. A fresh session can work from this file, [plan 32](32-plan-lifecycle.md) and
[its alignment steps](32-plan-lifecycle-alignment.md).

## 1. What the sweep found

### 1.1 The template is a template in name only

- `~/code/harness-template` has no commits, no remote and isn't the `~/.config` submodule
  [plan 31](archive/31-conventions-sweep.md) § 6 promised.
- It is applied once and never again ("seeded once, then the repo's own"). Three repos were
  seeded (budgetron, pugio); the six older ones never got it: forum and verus (its own
  source trees), lthy, ortho, dd, cinch.
- Fixes travel in neither direction. All three seeded repos rewrote the same template text by
  hand (`make ship` = deploy, "production is the user's", the HTTP error envelope, the TOOL-001
  tool list). lthy fixed the tmux pane index (`347ba4c`) after the template copied the old one.
  budgetron fixed the flutter hotfix semantics in four files; the template still has the old text.

### 1.2 The release half of the git flow is unused

| Repo | `main..dev` | Last release cut | Deploys from |
|---|---|---|---|
| forum | 77 | never, no tags | `dev` (`make deploy branch=dev`) |
| verus | 36 | never, no tags | `dev` |
| pugio | 27 | never; **never pushed** | — (sideloaded APK) |
| cinch | 23 | 2026-10-06; GitHub's newest release is **v0.9.0** | goreleaser on tag |
| ortho | 18 | never; **no remote** | — |
| lthy | 0 | 2026-11-15 | `main` |
| dd | 0 | 2026-10-22 (tags `0.2.0a`, not semver) | — |
| budgetron | 0 | 2026-11-05 | sideloaded APK |

Six of nine never cut a release. The `dev`/`main` split, the `--no-ff` cut, the dev
fast-forward, the hotfix letter and the sync merge exist for a flow two thirds of the repos don't
run. In every repo `dev` contains `main` (`dev..main` = 0, checked 2026-11-17).

### 1.3 Conventions are documented, not enforced

- **GIT-001** (no direct commits on `dev`/`main`): broken in forum (`aec9352`), verus (`4423a1c`,
  `362495d`), cinch (`ab07279`), budgetron (`e3920e9`, `6ce4e4e`, `5a97c88`). cinch checks the
  subject's shape, never the branch.
- **Squash only**: verus merged three feature branches with `--no-ff` (`54c380f`, `73f85c2`,
  `2957a47`, 3 parents each, no `version/` change), so three minor bumps were lost. The template's
  `commit.merge: exempt` lets the same thing through.
- **Three bump tables** coexist: the old lineage (forum, verus, lthy, ortho, dd: `chore/` and
  `refactor/` bump minor, no `perf/`/`ui/`, direct `fix:` bumps nothing), template B (budgetron,
  pugio), and cinch's variant (B, but never bumps on `main`).
- **Two commit checks**: the `commit:` block (template, cinch, budgetron, pugio) and the
  retired `check-squash-merge-msg.sh` (forum, verus, lthy, ortho, dd).
- **Floors**: `>=0.9.0` (forum, verus, lthy, dd), `>=0.10.0` (ortho), `>=0.13.2` (seeded).
  No GitHub release satisfies `>=0.13`.

### 1.4 The make verbs mean different things per repo

| Repo | `dev` | `down` | `check` read-only |
|---|---|---|---|
| forum, lthy | tmux, session `<name>-dev-<port>`, `-j2` fallback; inside tmux splits the current window | compose down + kill session (misses panes split inside tmux) | yes |
| ortho, dd, fsed | tmux, fixed session name, no fallback | **absent** (`stop-db` only) | dd: **no**, runs `go fmt` |
| verus | foreground | absent | yes |
| budgetron, pugio | `run` | absent | yes |
| cinch | — | — | no `check` target |

Other per-repo variance: `install` vs `install-all`; `push` vs `push-release`; `release`,
`release-cut`, `push`, `ship` as four steps of one act; `version-show` vs `version-*-backend`;
no `help` in cinch.

### 1.5 Ports clash

| Port | Used by |
|---|---|
| 8080 | forum, lthy, dd backends |
| 5173 | forum, lthy vite (dd's vite defaults to it without `strictPort`) |
| 5432 | dd Postgres (ortho moved to 5433) |

Lab host ports 3100 (lthy), 3200 (forum), 3300 (verus) are distinct, but nothing records them;
lab's own model servers hold 8080 and 8081 there. The template's port table already disagrees
with plan 31 (dd at 8080 vs 8082).

### 1.6 Dead weight

1. `check-harness`, `render`, `ignores` and `setup-hooks` wrap one cinch command each.
   `cinch check` already runs in every pre-commit hook; they exist because TOOL-001 says
   "always `make`".
2. AGENTS.md is loaded twice per session in every repo: `CLAUDE.md` is `@AGENTS.md`, and
   `.claude/hooks/inject-agents.sh` also `cat`s it.
3. `check-git-setup.sh` runs `make setup-hooks` (a re-render) at every session start, so a
   session can open on a dirty tree after a cinch upgrade.
4. Git rules are written in about six places: AGENTS.md, `conventions.md` § Git,
   `git-conventions.md`, the script headers, `cinch.yml` comments, `check-git-setup.sh`.
5. `conventions.md` § Documentation Philosophy paraphrases the rendered `docs-philosophy.md`;
   AGENTS.md § Models and sessions repeats the owner's global `~/.claude/CLAUDE.md`.
6. `manifest.yml` (lthy, ortho, dd, fsed) is read by nothing.
8. Two "run it like prod" paths: `preview` and `docker-build`/`docker-up`/`docker-down`.
9. `make help` is 40+ lines. The owner's shell history shows `make` typed by hand one or two
   times per target in 860 commands: agents are the main readers.

### 1.7 Template defects found on the way

- The `go` stack assumes a server; cinch is a CLI and stripped it. `go/conventions.md`
  is a copy of the go-svelte backend doc (`backend/`, `models/`).
- `VERSION_CMD` is a bun script in pure-Go stacks. `version-major` is cited but missing in `go`
  and `go-svelte`. `CONTAINER_PORT` is really the host port. Prettier is seeded but nothing runs
  it. `select-pane -t 0` breaks with `pane-base-index 1`. README says the remote is "always
  `lab`"; `push` targets `origin`. README § Gaps lists files that now exist.
- The template's forum "fix" (`deploy-setup` copies `.env.production.example`) rests on a stale
  premise: forum tracks a secret-free `.env.production` on purpose (`81785c8`).
- `conventions.md` asks every task for layers; `TEMPLATE-plan.md` has no Layers field.
- Shared by template and forum: `make down` can't stop a `make dev` started inside tmux.

## 2. Decisions

**D1. Trunk with tags, in every personal repo.** `main` is the only long-lived branch.
- Topic branches (`feature/`, `fix/`, `refactor/`, `perf/`, `ui/`, `chore/`, `docs/`) branch off
  `main` and land with `make merge`: a squash commit `merge: <branch>`, whose post-commit hook
  bumps the version.
- `make release` tags `v<version>` on `main` and pushes `main` and the tag to `origin`.
  `make ship` is `release` and then the stack's delivery (deploy, install-release, or the tag
  push that runs goreleaser).
- Gone: `dev`, `release-cut`, the `--no-ff` cut, the dev fast-forward, the sync merge, `hotfix/`,
  and the hotfix letter (a hotfix is a `fix/` branch and a patch).
- *Why:* § 1.2. The split buys nothing for one person who deploys from `dev` anyway, and every
  bump hole in § 1.3 lives in its 2-parent edge cases.
- fsed-odin-docs is a team repo with its own trunk flow and is out of D1.

**D2. One version per repo.** Tag `vX.Y.Z`.
- Per-service versions only when services release independently; none do today. Existing
  `<svc>-v…` tags stay as history; verus uses `v`, not `verus-v`.
- **Storage:** a root `VERSION` file holding `x.y.z` (one line). Flutter keeps `pubspec.yaml`
  `x.y.z+N` (every bump raises `N`, budgetron's semantics); Android keeps `version.properties`.
  Go binaries get it through `-ldflags -X`, the SPA through a vite `define`. The four
  integer files existed so sync merges wouldn't conflict; D1 removes sync merges.
- **Bump table (on `main`, squash commits only):** `feature/` = minor; `fix/`, `refactor/`,
  `perf/`, `ui/` = patch; `chore/`, `docs/` = none; major by hand (`make version-major`).
  A squash bumps only if it touches the repo's version-relevant paths.
- **First trunk version:** the highest current version in the repo with minor + 1 and patch 0
  (forum 0.40.0 → 0.41.0, lthy 0.62.0 → 0.63.0, ortho 0.12.0 → 0.13.0, dd 0.17.1 → 0.18.0,
  verus 0.14.0 → 0.15.0). It sorts above every old tag and absorbs verus's lost bumps.

**D3. One make contract.** Same verb, same meaning, in every stack:

| Verb | Meaning |
|---|---|
| `help` | default goal; contract verbs first, repo extras after |
| `install` | dependencies, `.env` from `.env.example` if missing, `cinch init`; idempotent |
| `dev` | everything needed to work locally (see below) |
| `down` | stop what `dev` started: kill the session, stop dev containers. Exists everywhere |
| `check` | static and read-only: vet, formatting, type checks, prettier, repo contract checks |
| `test` | the test suites; slow or device tiers stay separate (`test-e2e`, `test-device`) |
| `format` | rewrites files |
| `build` | the artifact (binary, SPA, APK) |
| `merge` | from a topic branch: squash onto `main` as `merge: <branch>` |
| `release` | tag `v<version>` on `main`, push `main` + tag |
| `ship` | `release` + delivery |
| `deploy` | server stacks: push `main` to `lab`, rebuild, health-poll; `branch=` previews a topic branch |
| `logs` | the running thing's logs (`prod-logs` on servers) |
| `version`, `version-major` | print the version; the manual major bump |
| `prune` | delete local topic branches whose squash already landed on `main` |

- **`dev`:** several processes → a tmux session `<name>-dev-<PORT_BASE>`. Inside tmux it
  creates the session detached and `switch-client`s to it, never splitting the current window,
  so `down` always has exactly one session to kill. An existing session is attached or switched
  to. No TTY or no tmux → `$(MAKE) -j2` in the foreground. One process → foreground. Mobile →
  build, install and start on the device.
- **Removed:** `setup-hooks`, `render`, `check-harness`, `ignores` (§ 1.6 item 1), `release-cut`,
  `push`, `push-release`, `version-show`, `docker-build`/`docker-up`/`docker-down` (`preview`
  stays). `check-frontend`/`check-backend` stay for hook scoping, listed under "more" in help.
- **TOOL-001 narrows** to checking, testing, formatting, building and running the dev server.
  cinch and git are called directly.

**D4. Every project owns a block of 10 ports.** One registry, `ports.md` in harness-template;
each Makefile has `PORT_BASE` and derives the rest. `strictPort` everywhere, and `dev` fails
early naming the process that holds a port.

| Offset | Use |
|---|---|
| +0 | app / backend |
| +1 | vite |
| +2 | database |
| +3, +4 | e2e backend, e2e vite |
| +5 | auxiliary (fake model server, ssh tunnel) |

| Block | Repo | Lab host port |
|---|---|---|
| 4100 | lthy | 3100 |
| 4110 | forum | 3200 |
| 4120 | verus | 3300 |
| 4130 | ortho | (3400 when deployed) |
| 4140 | dd | (3500 when deployed) |
| 4150 | talk-workbench | — |
| 4770 | fsed (keeps its 47xx block; team repo) | — |

Lab-side reserved: 8080, 8081 (model servers). A second stack in a worktree overrides
`PORT_BASE` with an unassigned block.

**D5. Template v2: managed files and seeded files.**
- **Managed:** byte-identical in every repo of a stack, overwritten by `make sync REPO=`. They
  take their parameters as arguments from the call site, never by editing the file: `scripts/`
  (`version.sh`, `post-commit-version-bump.sh`, `release.sh`, `step.sh`, `prune.sh`,
  `pre-commit-merge-test.sh`), `.claude/settings.json`, `.claude/hooks/*`, `.claude/.gitignore`.
- **Seeded:** written once, then the repo's: `Makefile`, `cinch.yml`, `AGENTS.md`, `CLAUDE.md`,
  `.docs/*`, lint configs. `make diff REPO=` shows drift against a fresh render.
- A seeded `.harness` file records `stack`, `name` and the template version; `sync` and `diff`
  read it.
- The template gets its commit history and a remote, and its `VERSION` goes to 2.
- *Why this reopens part of plan 30:* plan 31's reopen condition was a failure caused by a
  drifted copy. § 1.3 has three (dd's tags, the old bump table, verus's merges past the old
  check script). This is still a list of files and a copy, not plan 30's engine.

**D6. Slimming.** § 1.6 items 1–9 are done in template v2:
- the wrappers are removed (D3);
- `inject-agents.sh` prints only the orientation (branch and live plans);
- `check-git-setup.sh` runs `cinch init` only when `core.hooksPath` is unset, and otherwise
  only warns;
- `git-conventions.md` becomes the one git doc. AGENTS.md cites GIT-001 and links to it;
  `conventions.md` § Git shrinks to the rule items;
- the Documentation Philosophy and Models sections and `manifest.yml` are dropped;
- help is reordered.

**D7. `.docs` everywhere personal.** lthy, ortho and dd rename `.agent` → `.docs` in their pass.
fsed-odin-docs keeps `.agent` (team repo).

**D8. Remotes.** `origin` is GitHub (backup, CI, goreleaser). `lab` is only a deploy mirror.
Every personal repo has an `origin` that is pushed. pugio's `origin=lab` / `github=github`
pairing flips.

**D9. Scope.**
- **Full:** cinch, forum, verus, lthy, ortho, dd, budgetron, pugio.
- **Partial (team repo):** fsed-odin-docs. It gets `down`, the dev fallback, `.DEFAULT_GOAL`,
  `install` and the doc-drift fixes. It does not get D1, D2 or D7.
- **Light:**
  - talk-workbench: AGENTS.md + `CLAUDE.md` = `@AGENTS.md`, `down`, the 4150 block, the commit
    grammar.
  - deltadocs_landing: merge `chore/align-conventions`, fix the README's English section.
- **Out:** switch-fidget, pixelwaves, deltadocs-itas2026, cinch_bench, odin-landing (client
  site), gladius (its own CLAUDE.md + PLAN.md workflow), geobyte and odin_* (team repos).

**D10. Sequencing.**
1. Plan 32 § 0 as written.
2. Part A here (cinch guards) and a GitHub release.
3. Template v2 (Part B).
4. One session per repo (Part C) that does plan 32a's triage, the docs-root rename and the
   template sync together, so each repo is visited once.

## 3. Rules

- **Existing:**
  - CINCH-015 (squash subjects): `commit.squash.main` replaces `commit.squash.dev` in every
    `cinch.yml`; the rule itself doesn't change.
  - CINCH-016 (merge commits): extended by CINCH-020.
  - CINCH-006 and CINCH-012 (pre-commit reads the staged tree): the guards run in commit-msg,
    so they don't apply.
- **New, proposed text** (numbered and written into [rules.md](../rules.md) in T1):
  - CINCH-020 (proposed): `commit.merge: reject` fails every commit made while
    `$GIT_DIR/MERGE_HEAD` exists, naming `make merge` as the way to land a branch. `exempt` and
    a pattern behave as CINCH-016 says.
  - CINCH-021 (proposed): with `commit.protect: [<branch>…]` in `cinch.yml`, a commit on a
    listed branch passes only while `$GIT_DIR/SQUASH_MSG` exists, or while `MERGE_HEAD` exists
    and `commit.merge` isn't `reject`. The first commit on an unborn branch passes. A
    `--no-verify` commit (the version bump's amend) isn't checked, by git's design. Without the
    key, nothing changes.

## 4. Test plan (Part A)

- [x] CINCH-020: with `commit.merge: reject` and `MERGE_HEAD` present, the commit-msg check
  fails and its message names `make merge`. With `exempt`, it passes as before.
  — `internal/cinch/checks_misc_test.go`
- [x] CINCH-021: on `main` with `commit.protect: [main]`, a plain commit fails, a squash commit
  passes, and a commit on an unlisted branch passes. — `internal/cinch/checks_misc_test.go`
- [x] CINCH-021 edge cases: an unborn `main` passes; a detached HEAD passes; the key absent
  changes nothing. — same file
- [x] The rendered docs and `git-conventions.md` describe both keys. — `cinch check` (links,
  rules)

## 5. Tasks

### Part A — cinch: guards for the trunk flow

#### T1: Write CINCH-020 and CINCH-021
**Status:** [x]
**Depends on:** plan 32 merged to `dev` (§ 0 step 1 of 32a)
**Files:** `.docs/rules.md` (two items, each with a `cinch:ignore` naming its task),
`.docs/git-conventions.md` (the two keys)
**Rules:** CINCH-020, CINCH-021
**Verify:** `cinch check` green

#### T2: `commit.merge: reject`
**Status:** [x]
**Depends on:** T1
**Files:** `internal/cinch/checks_misc.go` (`commitPatternFor`, `checkCommit`), its test
**Rules:** CINCH-020
**Verify:** `go test ./internal/...`; the T1 ignore is replaced by the marker

#### T3: `commit.protect`
**Status:** [x]
**Depends on:** T1
**Files:** `internal/cinch/checks_misc.go` (`readCommitState` reads the branch on every commit,
not only when squashing; unborn and detached cases), its test
**Rules:** CINCH-021
**Verify:** `go test ./internal/...`; marker in place

#### T4: Release cinch and publish it on GitHub
**Status:** [x]
**Depends on:** T2, T3
**Files:** none (git). On the old flow, one last time: squash onto `dev`, cut the release to
`main`, `make push-release`. The `v*` tag runs goreleaser.
**Verify:** `gh release view` shows the new version. `install.sh` on a clean `$HOME` installs
a binary that satisfies `>=0.16.0`.

### Part B — harness-template v2 (`~/code/harness-template`, rules in its `README.md`)

#### T5: History and remote
**Status:** [ ] — history done (`1132099` import, `eea6f58` plan 32 T6); the remote is
deferred by the owner (2026-11-17)
**Depends on:** plan 32 T6 (which makes the first commit)
**Files:** none. The owner creates the private GitHub repo; the template is then pushed as
`origin` (D8). Until then the template is local only, and T6–T11 proceed without it.
**Verify:** `git log` shows the T6 commit on `origin/main`

#### T6: Managed/seeded split in `apply.sh`
**Status:** [x]
**Depends on:** T5
**Files:**
- `apply.sh`: `--sync` overwrites only managed paths and refuses a target without `.harness`;
  `--diff` prints a unified diff per seeded file against a fresh render using the target's
  `.harness`.
- `common/managed.txt`, `stacks/<stack>/managed.txt`.
- `common/.harness` (`stack={{STACK}}`, `name={{NAME}}`, `template=<VERSION>`).
- `Makefile` (`sync`, `diff`).
**Verify:** `make check` extended: a sync into an applied temp repo changes nothing, a managed
file edited in the temp repo is restored by sync, and diff on a fresh apply prints nothing.

#### T7: Managed scripts and hooks
**Status:** [x]
**Depends on:** T6
**Files:**
- `common/scripts/version.sh`: `show|major|minor|patch` on `VERSION`, POSIX sh. Flutter and
  Android override it with managed stack copies: pubspec `+N`, and pugio's `version.properties`
  + versionCode script.
- `post-commit-version-bump.sh`: D2's table, on `main` only, squash commits only; the path
  patterns arrive as arguments from `cinch.yml`.
- `release.sh`: tag + push. It refuses off `main`, on a dirty tree, or with an unpushed merge
  in progress.
- `prune.sh`: list branches whose `git merge-tree --write-tree main <b>` equals `main^{tree}`;
  delete only with `confirm=1`.
- `pre-commit-merge-test.sh`: budgetron's, gated on `SQUASH_MSG`.
- `step.sh`: unchanged.
- `inject-agents.sh` without the AGENTS.md `cat`.
- `check-git-setup.sh`: non-writing (D6); warns on `main` instead of `dev`/`main`.
**Verify:** `make check`; `sh -n` on every script; a temp-repo run of
merge → bump → release with a fake remote.

#### T8: Make contract in every stack
**Status:** [x]
**Depends on:** T7
**Files:** `stacks/*/Makefile`:
- the D3 verbs;
- `PORT_BASE` with derived ports and a `require-free-ports` guard (D4);
- the tmux `switch-client` dev; `down` everywhere;
- `merge`/`release`/`ship`;
- `deploy` from `main` with `branch=` preview, and `HOST_PORT` (renamed from `CONTAINER_PORT`);
- version stamping through `-ldflags`;
- help reordered.

Absorbed:
- lthy's prettier gate in `check-frontend` and `format`;
- forum's deploy safety: `last-live` + `rollback`, `migrate-check`, `restore-drill` in `ship`;
- the no-container backup guard in every server stack;
- a commented Postgres block from ortho/dd: `dev-db`, `stop-db`, `drop-db`, `migrate-*`, port
  pausing.

**Verify:** `make check` parses every stack Makefile. In a temp repo, `make -n dev` plans one
session and `make down` kills it, inside and outside tmux, checked by hand once.

#### T9: Stacks
**Status:** [x]
**Depends on:** T8
**Files:**
- **New `stacks/go-cli`**, from cinch:
  - `run ARGS=`, `build` to `BIN_DIR`, `install`, `TESTFLAGS`;
  - an optional seeded `.goreleaser.yml` + `.github/workflows/release.yml`;
  - its own conventions doc.
- **`stacks/go`:** its own conventions doc for a root-package service (from verus), verus's
  `.dockerignore`.
- **`stacks/flutter`:** build-number semantics.
- **`stacks/android`:**
  - `device-serial.sh` + `WITH_DEVICE` on every adb target;
  - `assembleDebugAndroidTest` in `test`;
  - generic `update-version.sh` as the managed `version.sh`.
- **Stack variables** for the "production/phone/publishing is the user's" paragraph and the
  error-format section.

**Verify:** `make check` applies every stack, including `go-cli`.

#### T10: Docs, AGENTS.md, registry, README
**Status:** [x]
**Depends on:** T8
**Files:**
- `common/.docs/git-conventions.md`: rewritten for D1/D2, the only git doc.
- `common/.docs/conventions.md`:
  - § Git becomes GIT-001 (no commits on `main` except via `make merge`; `commit.protect`
    enforces it) and a link;
  - TOOL-001 narrowed (D3);
  - Documentation Philosophy dropped.
- `common/AGENTS.md`: Models section dropped, harness section for D3/D6, stack-variable
  production paragraph.
- `common/cinch.yml`:
  - `commit.squash.main`, `commit.merge: reject`, `commit.protect: [main]`;
  - the bump script's arguments;
  - `plans.lifecycle: enforce`;
  - a live `pre-commit: make check` entry;
  - `require.cinch` ≥ T4's version.
- `TEMPLATE-plan.md` is gone (plan 32 T6); the Layers field is noted in
  `cinch workflow dev-plan-feature`'s output instead, as a cinch follow-up.
- `ports.md` (D4).
- `README.md`: apply/sync/diff, managed vs seeded, choices, gaps.

**Verify:** `make check` green (links, rules, index in each applied stack)

#### T11: Template `make check` guards the contract
**Status:** [x]
**Depends on:** T9, T10
**Files:** `Makefile`. Three checks:
- every stack Makefile defines every D3 verb (`make -n` per verb);
- managed files are byte-identical across stacks where shared;
- `ports.md` blocks don't overlap.

Bump `VERSION` to 2.
**Verify:** `make check` green; deliberately breaking each guard turns it red.

### Part C — one session per repo

Each task follows § 5.1 plus its repo's notes, and plan 32a's section for the same repo (its
paths read `.docs` once step 2 has run). The session prepares the branch and proposes the
commit; the owner lands it with § 5.2.

#### T12: cinch (stack `go-cli`; dogfood first)
**Status:** [x] — landed 2026-11-17 (`1789be3 merge: chore/harness-v2`), released as v0.18.0
**Depends on:** T4, T11
**Files:** per § 5.1.
- `VERSION` replaces `internal/version/*` (ldflags already in the Makefile).
- Add `help` and `check` (vet + gofmt); take `vet` out of `build`.
- Drop `.githooks` from `.gitignore`.
- AGENTS.md tiers + `CLAUDE.md` + `.claude/`.
- `require.cinch: any` stays, with its reason.
- Plan 32 T5 triage.
- Worktree `.claude/worktrees/cli-visual-pass`: merge or drop first.

**Verify:** § 5.1 step 7

#### T14: forum (stack `go-svelte`)
**Status:** [x] — landed 2026-11-18 (`ac20601 merge: chore/harness-v2`), released as v0.41.0; `make deploy` waits for lab
**Depends on:** T12
**Files:** per § 5.1.
- Block 4110. Version 0.41.0.
- Delete `check-squash-merge-msg.sh`.
- `.env.production` → `.env.production.example` (it holds no secrets; uniform with the
  template), and drop the stale roadmap Deferred note.
- Keep `install-agent`, `create-token`, `vapid-keys`, `seed`, `reindex`, `prod-restore`,
  `verify`, `outdated`, `icons` as repo extras.
- Check `~/code/forum-wt` (a separate clone) and retire it if it holds nothing unmerged.

**Verify:** § 5.1 step 7; `make deploy` by the owner afterwards

#### T15: verus (stack `go`)
**Status:** [x] — landed locally 2026-11-18 (`a4805d8 merge: chore/harness-v2`); `make release` (v0.15.0) and `make deploy` are the owner's
**Depends on:** T14 (verus pulls forum's contract fixtures)
**Files:** per § 5.1.
- Block 4120 (`tunnel` → 4125).
- Version 0.15.0, tag `v`.
- `PROD_BACKUP_CMD` gets the guard.
- Keep `prompts-lock`, `contract-pull`, `tunnel`.
- `TOOL-001` moves from `testing.md` to `conventions.md`.
- Roadmap gains Open questions.
- Check `~/code/verus-wt`.

**Verify:** § 5.1 step 7

#### T16: lthy (stack `go-svelte`)
**Status:** [x] — landed locally 2026-11-18 (`784dbd8 merge: chore/harness-v2`); `make release` (v0.63.0) and `make deploy` are the owner's
**Depends on:** T12
**Files:** per § 5.1 with the rename (step 2).
- Functional references: `scripts/archive-plans.sh`, which is deleted (superseded by
  `cinch plan archive`; its merge-dev call goes too), and `.dockerignore:10`.
- `manifest.yml` folded into `architecture.md`.
- Block 4100.
- Version 0.63.0.
- Binary name `us` stays as `NAME`.

**Verify:** § 5.1 step 7

#### T17: ortho (stack `go-svelte` + Postgres block)
**Status:** [x] — landed locally 2026-11-18 (`7c00247 merge: chore/harness-v2`); `make release` (v0.13.0) is the owner's
**Depends on:** T12
**Files:** per § 5.1 with the rename.
- Commit the dirty roadmap first (32a).
- Add a GitHub `origin`.
- Block 4130 (backend 8090, vite 5180, pg 5433, e2e 8091/5181 → 4130–4134).
- Version 0.13.0.
- `check-error-codes` into `check`.
- Keep `check-placeholders`, `test-e2e`, the migrations, and CI on make targets.

**Verify:** § 5.1 step 7; `make test-e2e`

#### T18: dd (stack `go-svelte` + Postgres block)
**Status:** [ ]
**Depends on:** T12
**Files:** per § 5.1 with the rename.
- Commit `TODO.md` first (32a); its rename checklist (`TODO.md:101-144`) is the step 2 list.
- Functional references: `.pi-lens.json`, `.pi/extensions/auditor.ts`,
  `.claude/agents/auditor.md`.
- Block 4140 (8080, 5173, 5432, 8180/5273 → 4140–4144).
- Version 0.18.0.
- `check` stops running `go fmt`.
- The appended cinch block and its comment go.
- `TODO.md` → `.docs/roadmap.md` `## Deferred`.

**Verify:** § 5.1 step 7; `make test-e2e`

#### T19: budgetron (stack `flutter`)
**Status:** [ ]
**Depends on:** T12
**Files:** per § 5.1.
- Keep the ObjectBox `setup-test`, `perf` and `coverage`.
- `pre-commit-merge-test.sh` becomes the managed copy.
- Remove the `budgetron-recurring-payments` worktree first (retired, plan 32 Q5).
- Branches `feat/sms-import`, `backup/*` and `claude/*` go through `make prune` or are deleted
  by hand.

**Verify:** § 5.1 step 7; `make build-release`

#### T20: pugio (stack `android`)
**Status:** [ ]
**Depends on:** T12
**Files:** per § 5.1.
- Merge or park `ui/lock-cover-look` first.
- Remotes flip (D8), then the first push.
- `WITH_DEVICE` on `install-release`, `test-device`, `startup`.
- Drop the stale "once P0/P2" AGENTS lines.
- Keep `startup` and `push-config`.
- Delete the `worktree-agent-*` branches.

**Verify:** § 5.1 step 7; `make build-release`

#### T21: fsed-odin-docs (partial, D9)
**Status:** [ ]
**Depends on:** T11
**Files:**
- Root `Makefile`: `down` (kill `odin-upsr-dev`, `stop-db`, Centrifugo), the dev fallback with
  `$(MAKE)`, `.DEFAULT_GOAL`, `install` (`submodule-init` + `cinch init`).
- `cinch.yml`: `commit.merge` set deliberately (team `--no-ff` merges).
- Doc drift: `AGENTS.md:54,71,126`, `git-conventions.md:28,79`.
- Plan 32a § fsed.
- No D1, D2, D7.

**Verify:** `make check`; `cinch check`

#### T22: Light repos
**Status:** [ ]
**Depends on:** T10
**Files:**
- talk-workbench: AGENTS.md (from its CLAUDE.md), `CLAUDE.md` = `@AGENTS.md`, `down`,
  `PORT_BASE` 4150, the commit grammar in AGENTS.md. No cinch.
- deltadocs_landing: merge `chore/align-conventions`, fix README § English.

**Verify:** `make check` in each

### 5.1 Per-repo procedure (Part C)

0. **Preconditions:** plan 32 § 0 merged in this repo, a clean tree, on `dev`. In-progress topic
   branches are parked; they rebase onto `main` after the switch.
1. Branch `chore/harness-v2` off `dev`.
2. **`.agent` repos only:**
   - `git mv .agent .docs`, then set `paths.docs: .docs` in `cinch.yml`.
   - `grep -rn '\.agent' --exclude-dir=.docs --exclude-dir=node_modules .` and fix every hit.
     Functional ones are named in the repo's task.
   - `cinch check --only links`.
3. Plan 32a § 1 and § <repo> (archive, status words, `roadmap.md`, `manual-checks.md`).
4. **Template:**
   - `make -C ~/code/harness-template apply REPO=<repo> STACK=<stack> NAME=<name>` seeds what
     is missing, including `.harness`.
   - `make -C ~/code/harness-template sync REPO=<repo>` writes the managed files.
   - `make -C ~/code/harness-template diff REPO=<repo>` lists the seeded drift. Port by hand:
     - the Makefile verbs, keeping the repo's extras;
     - `cinch.yml` (T10's keys, the bump script's path arguments, pre-commit entries);
     - AGENTS.md / CLAUDE.md, `git-conventions.md` and `conventions.md`.
5. **One version (D2):** write `VERSION`, delete the old version files, and wire `-ldflags` and
   the vite `define` in the Makefile and Dockerfile. Old tags stay.
6. **Ports (D4):** `PORT_BASE`, vite `server.port` + `strictPort`, `.env.example`, compose and
   test configs.
7. **Verify:** `make check`, `make test`, `cinch check` all green. `make help` lists the D3
   verbs. `make dev` then `make down` leaves no session and no dev container, both inside and
   outside tmux. Propose the commit
   `chore [harness]: harness v2 — trunk flow, one version, make contract`.

### 5.2 Landing the switch (owner)

`dev` contains `main` in every repo (§ 1.2), so `main` moves by pointer. The installed cinch
must be T4's or newer: the staged `cinch.yml` has `commit.squash.main` and `commit.protect`.

```
git branch -f main dev                  # on chore/harness-v2; main catches up with dev
git checkout main
git merge --squash chore/harness-v2
git commit -m "merge: chore/harness-v2" # hooks: make check, commit-msg, no bump (chore/)
make release                            # tags v<VERSION>, pushes main + tag to origin
gh repo edit --default-branch main      # where origin is GitHub and the default was dev
git push origin --delete dev
git branch -D dev
git rebase main <parked-branch>         # for each parked topic branch
make prune                              # lists squashed branches; confirm=1 deletes
```

Server repos then `make deploy`. Then check the next session start: branch `main`, and live
plans only.

## 6. Ordering and risk

1. Plan 32 § 0 (merge, install, upgrade pass), unchanged.
2. Part A, then T4. Every Part C landing needs that release.
3. Part B (T5–T11).
4. Part C: T12 cinch, T13, then the rest in any order. T15 comes after T14. T21 and T22
   only need Part B.

Risks:
- A repo landed before T4 is installed fails its `merge:` commit on `main`. § 5.2 names the
  floor, and `require.cinch` in the seeded `cinch.yml` catches it too.
- Parked branches cut from `dev` keep old renders and old scripts; rebasing onto `main` brings
  the new ones.
- Changing lthy's ports loses dev localStorage and the PWA install at the old origin, which is
  acceptable.
- forum and lthy's deploy step is unchanged (`lab` receives `main`), but their first deploy
  after the switch is the first one from `main` in forum's case.

## 7. Separate question for cinch: committed renders

Every repo commits the 8 rendered workflow files. The `generated` check therefore turns every
repo red on any cinch template change, which is why plan 32 § 0 needs a single sitting.
Agents could read workflows with `cinch workflow <name>` (AGENTS.md already names it) instead
of committed copies. That would change what `generated`, `init` and the AGENTS pointer
promise, so it gets its own plan. Not decided here; Part B keeps the committed renders.

## 8. Recorded, not in scope

- **gladius:** not on the harness (D9). It is 199 commits ahead of `lab`. Pushing it is the
  owner's.
- **Housekeeping the owner can do any time:**
  - fast-forward odin-landing's local `main` (13 behind `origin/main`);
  - fix cinch_bench's README Status paragraph;
  - commit or ignore deltadocs-itas2026's `talk/`.
- **Doc-versus-code drift in convention docs** (plan 31 § 2, plan 10). Untouched.

## Handoff
**Last completed:** T17 (ortho, `~/code/ortho-store`). A Sonnet builder did the work, and this
session verified it on 2026-11-18. At the owner's request it landed locally with § 5.2:
- `e4eca6b` is the branch commit. `main` then moved to `dev`, and `7c00247 merge:
  chore/harness-v2` was squashed onto it.
- Every hook passed, the merge test included (with `make dev-db` up). `chore/` doesn't bump, so the
  version stays 0.13.0.
- Nothing is pushed or tagged; `dev` is still there. `origin` is
  `git@github.com:kanmear/ortho-store.git`, a private repo that is still empty.

**What changed:**
- **Rename and triage.** `.agent` → `.docs`, `paths.docs: .docs`.
  - The dirty roadmap rode along and became `.docs/roadmap.md` through `cinch mv`. Its Deferred
    section keeps only the DaisyUI entry; this pass resolved the other four.
  - Nine complete plans plus `foundation.md` are archived as `**Status:** complete — …`.
  - `fix/crm-claim-verification.md` is deleted.
  - The new `manual-checks.md` lists four click-throughs the handoffs owed: password reset,
    stale orders, honest checkout, CRM claim.
  - `manifest.yml` is folded into `architecture.md` and `conventions.md`, then deleted.
- **Template.**
  - `apply` and `sync` ran, and the drift was ported by hand.
  - `cinch.yml` has the v2 keys, `require.cinch >=0.17.0`, and the bump arguments `'backend/*'
    'frontend/src/*'`.
  - `check-squash-merge-msg.sh`, `update-version.js` and the integer version files are deleted.
- **Version.** `VERSION` is 0.13.0; it was backend 0.11.0 and frontend 0.12.0. The backend module
  is `ortho_backend` with `version.Version` stamped by `-ldflags`. The SPA reads
  `__APP_VERSION__` through the vite `define`. Both Dockerfiles take `ARG APP_VERSION`.
- **Ports.** Block 4130: backend 4130, vite 4131 (`strictPort`), Postgres 4132, e2e 4133/4134.
  The builder also edited the owner's gitignored `backend/.env` and `frontend/.env` to the new
  ports. Any other checkout's existing `.env` needs the same edit.
- **Makefile.** The go-svelte render with the Postgres block uncommented.
  - Extras: `check-placeholders`, `check-error-codes` (inside `check`), `test-e2e`,
    `install-e2e`, `install-frontend`/`install-backend` (for CI), `migrate-*`, `create-staff`,
    `add-test-data`, `clean-and-add-test`, `dev-db`/`stop-db`/`drop-db`. Air stays.
  - `backend/Makefile` and `frontend/Makefile` are deleted. The root carries every target, and
    TOOL-001 now forbids `cd` into a service.
  - `backend/docker-compose.yml` became the root `docker-compose.dev-db.yml`, postgres only. It
    keeps `name: ortho_backend` so the dev volume survives. The root `docker-compose.yml` stays
    as the deploy shape.
- **No deploy yet.** `deploy`, `logs`, `rollback`, `restore-drill`, `preview`, the backups and
  `create-owner` are dropped. `ship` is `release` plus a note that there is no deploy target.
  When a host exists (lab 3400), add `deploy` and `logs` from the template.
- **`down` differs from the template.** In tmux, a plain `kill-session` left Air and the backend
  holding 4130. `down` now sends TERM to each pane's process, waits up to 5 s, then kills the
  session.
- **CI.** It triggers on `main` and PRs, and installs cinch v0.18.0 through cinch's pinned
  `install.sh`. It runs `cinch check`, `make check`, `make test-*` and `make test-e2e`.
- **Dropped seeds:** `frontend/bunfig.toml`, `tests/support/*` and the root `.dockerignore`
  (images are built per service). ortho's own eslint config stays.

**Verified here:**
- `make check` is green (about 24 s; it is now the whole pre-commit).
- `cinch check` is green (links, rules, index, plans, generated, core, hooks).
- `make test` is green with `make dev-db` up: 119 vitest tests, and the Go packages `email`,
  `handlers`, `middleware` and `models`. Without the database, `test-backend` fails with "Try
  'make dev-db' first". That was true before the port too, but the squash's merge-test now runs
  `make test`, so start `make dev-db` before `make merge`.
- The retired-names grep is clean outside `plans/archive`. One leftover `.agent/api/` in
  `backend/conventions.md` was fixed here.

**The builder also checked:**
- `make test-e2e`: 5 passed on 4133/4134.
- `make dev` without a TTY ran the `-j2` fallback on 4130/4131. Inside tmux it ran the detached
  session `ortho-dev-4130`, and `make down` left no session, no listener and no container.
- `make version` prints 0.13.0, and `make build` stamps it into `bin/ortho` and the bundle.
- Both images build with the version.
- A second `sync` is a no-op.
- `make -n release|merge|ship` plan sensibly.

**Not verified:** `make dev` from a real TTY outside tmux (the attach path); a real
merge, release or ship; CI (never run, since nothing is pushed); booting the compose stack.

**Owner still to do:**
1. **ortho (T17):** finish § 5.2 on `main`.
   - `make release` is the first push to the empty GitHub repo, with tag v0.13.0. CI runs then;
     check it.
   - `git branch -D dev`. The 11 topic branches and `chore/harness-v2` all have their squash and
     can go through `make prune`.
   - Start `make dev-db` before any `make merge`, because the squash's merge-test runs
     `make test`.
2. **lthy (T16):** finish § 5.2 on `main`.
   - `make release`, which tags v0.63.0 and pushes `main` and the tag.
   - `gh repo edit --default-branch main`, if GitHub's default is `dev`.
   - `git push origin --delete dev && git branch -D dev`. `lab` also has a `dev` branch.
   - `make deploy` once lab is up. The PWA origin changes in dev only.
   - Then `make prune`.
   - Optional: the untracked `.claude/settings.local.json` still allows `make merge-dev`,
     `release-cut` and `push`.
3. **Verus:** on `main`, `make release`, which tags v0.15.0 and pushes `main` and the tag. Then
   `git push origin --delete dev && git branch -D dev`: `dev` holds nothing `main` lacks. Once lab
   is online, `make deploy`, the first deploy from `main` and with the new Dockerfile. Then
   `rmdir ~/code/verus-wt`, an empty directory with no repo, and `make prune`. The ~24 old dev-flow
   branches, `chore/harness-v2` included, all have their squash on `main` and can be deleted.
4. **Forum (T14):** `make deploy` once lab is back. Then prune: `scripts/prune.sh --confirm`,
   plus about 55 older dev-flow branches by hand. Then `rmdir ~/code/forum-wt`.
5. **cinch:** roadmap § "Open from plan 31" still says the Forum CLI waits on Forum F0/F1. Both are
   done, and the CLI remainder is in forum's § Deferred, so that item can go.

**Resume at:** T18 (dd), then T19 (budgetron) or T20 (pugio). All of them depend only on T12. T16
and T17 are the precedents for a go-svelte `.agent` repo. dd is the closest to T17 (Postgres
block, e2e, not deployed).

**Notes:**
- **Template defect, from T15:** the go stack's `Makefile` build and its `conventions.md` stamp
  `-X main.version`, but verus and forum both use a `version` package. Fix it upstream, or give
  the stack a variable for the package path.
- **Template gap, from T16:** the go-svelte stack has no `DEPLOY_NAME` for a deploy name that
  differs from `NAME`. Its seeded `bunfig.toml`, eslint config and `tests/support` assume eslint
  and the svelte preload.
- The template stack branches are still unmerged (`feature/contract-guards` → … →
  `chore/plan-lifecycle`), and T5's remote is still deferred.
- Template gap from T13: a repo-local layer for managed `.claude/settings.json`.
- **Template gaps, from T17:**
  - The go-svelte stack needs a variant for a Node frontend with Postgres: no `DB_PATH`,
    `create-owner`, deploy block or `ENV_FILE`.
  - `down` should send TERM to the panes before `kill-session`, or an Air or `go run` backend
    survives.
