# AGENTS.md
<!-- harness-template v2 -->

cinch is a Go command-line tool that checks the operational docs governing a repository (links,
numbered rules bound to tests, plans, rendered workflows and hook shims, commit shapes) and renders
the workflows and git hooks that keep them honest; it runs on its own repo. One binary (`main.go`
plus `internal/`), no server, published as GitHub releases built by goreleaser from a `v<version>`
tag.

## Critical Rules

- **TOOL-001** (`.docs/conventions.md`) — run checks, tests, formatting, builds and the dev server
  via `make <target>` from the project root. Never invoke the underlying tool (`go`)
  directly. `cinch` and `git` are called directly. `make help` lists every target.
- **GIT-001** (`.docs/conventions.md` § Git) — never commit to `main` directly. Work on a
  `feature/`, `fix/`, `refactor/`, `perf/`, `ui/`, `chore/` or `docs/` branch off `main`, with
  `<type> [<scope>]: <description>` commit messages, and land it with `make merge`. This holds
  for cloud and remote sessions too: if the session is assigned `main` as its working branch,
  create a topic branch instead and tell the user.

## For AI Agents - always follow this

**Every session — read these before doing anything:**
- `.docs/roadmap.md` — status, decisions, milestones, what is next; then the plan of the
  current milestone in `.docs/plans/`.
- `.docs/architecture.md` — the packages, the commands, how checks, hooks and render fit
- `.docs/conventions.md` — git, commands, dependency rule, Go style

**Working in an area** — also read that area's docs. Run `cinch index` for what exists.
- Go code → `.docs/stack-conventions.md` (layout, errors, tests, release) and `.docs/conventions.md`
  (naming, package boundaries)
- tests → `.docs/testing.md`
- a rule or a check's behavior → `.docs/rules.md` (cinch's own numbered rules, `CINCH-nnn`)
- git, versions, releases → `.docs/git-conventions.md`

**Before writing any implementation code for a milestone** — follow the feature-planning workflow in
`.docs/workflows/dev-plan-feature.md`: validate the rules with the user, write the test plan, and
decompose into atomic tasks (`cinch workflow dev-plan-feature` prints it). Planning only
decomposes tasks — it does not implement them; once a plan file has a `## Tasks` section, work
through it via `.docs/workflows/dev-execute-plan.md` (`cinch workflow dev-execute-plan`).

**Fixing bugs / debugging — follow the bug-fix workflow in `.docs/workflows/dev-fix-bug.md`.**

**Orientation** (the cinch pointer; `cinch init` leaves this file alone while it mentions cinch):
- `cinch index` lists every doc with its title; `cinch workflows` lists the workflows, `cinch workflow <name>` prints one.
- Business rules are numbered items with a bold ID like **AUTH-001**. The test that enforces a rule carries a `cinch:rule <ID>` comment.
- Changing a behavior a rule describes means updating the rule and its test together. A new invariant a reader couldn't recover from the code gets a new rule.
- `cinch check` must pass before you commit.

## Harness

- **Session start.** `.claude/settings.json` runs two hooks. `inject-agents.sh` prints the
  branch and each plan's status line, nothing else (this file is already loaded through
  `CLAUDE.md`). `check-git-setup.sh` runs `cinch init` only when `core.hooksPath` is unset, and
  otherwise only warns: `cinch` missing, `core.hooksPath` set elsewhere, or the session on `main`.
- **Git hooks** (cinch, configured in `cinch.yml`). `pre-commit` runs `cinch check` and
  `make check`, and on a squash `make test`; `commit-msg` enforces the `commit:` block
  (`<type> [<scope>]: <description>`, `merge: <branch>` for a squash onto `main`, no merge
  commits, no other commit on `main`); `post-commit` bumps the version after a squash on `main`.
  Details: `.docs/git-conventions.md`. The hooks exec the installed `cinch`, so after changing
  checks or templates run `make install-bin` to test them against the new build.
- **Commands.** Before a commit run `make check` (static, read-only) and `make test`; for a docs
  change `cinch check`. TOOL-001 applies to the make targets.
- **Publishing is the user's.** `make release` and `make ship` push `main` and a `v<version>` tag to
  `origin`, and the tag push runs goreleaser in CI and publishes the GitHub release; the user runs
  them. An agent session builds and tests locally and never releases or pushes; when a change needs
  a release, say so in the handoff.

## Sessions

No new dependency, runtime or dev, without explicit approval (see `.docs/conventions.md`).

Handoffs are manual. A session does one task of the current plan: take the first task whose
dependencies are done, implement it, run its Verify, **propose a commit message and don't run
`git commit` unless asked**, update the task status and the plan's `## Handoff`, and stop. The
user reads the handoff and starts the next session. Never continue into the next task in the same
session, and never start a task in another repo.
