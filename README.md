# cinch

A referential integrity checker for the operational documentation that
governs a repository — the rules, workflows, and conventions code must
conform to, read by humans and executed by agents.

## Overview

Operational documentation — the rules, conventions, and workflows a codebase
expects contributors (human or AI) to follow — tends to drift from what the
code actually does. Nobody remembers to update the troubleshooting doc after
fixing the bug it describes; a "business rule" written down a year ago
quietly stops being enforced by any test; a workflow file tells an agent to
read a path that hasn't existed since the last refactor. The failure mode is
always the same: the doc *looks* authoritative and isn't, and nothing catches
the gap until someone — increasingly, an autonomous agent — trusts it and
acts on stale information.

cinch closes that gap by making a repo's operational docs *checkable*, not
just readable. It doesn't understand what a rule means — it enforces that
every rule is bound to something concrete and verifiable:

- a **rule** in a doc must have a matching `// cinch:rule <ID>` marker
  somewhere in the source that enforces it (or an explicit, reasoned
  `cinch:ignore` declaration that it's untestable) — so "documented" and
  "enforced" can't quietly diverge
- a **relative link** between docs must resolve on disk
- a rule ID present at the previous commit must still be present (or
  explicitly retired) at this one — deleting a rule silently isn't a no-op
- the **rendered output** — shared workflow guidance, git hook shims — must
  match what re-generating it right now would produce, byte for byte, so a
  hand-edit or tampering attempt is caught the same way a code change would be
- an optional **commit message convention** can be enforced the same way
- the git hooks that run all of the above must actually be **activated**,
  not just generated

`cinch check` runs all of it in one command, in tens of milliseconds, with no
configuration required by default. `cinch init` scaffolds a new consumer end
to end: a manifest, rendered onboarding workflows (bug fixing, feature
planning, domain-rule maintenance, doc sync, and more — generic, with no
assumption about a project's stack or directory layout), and activated git
hooks that run `cinch check` on every commit.

The point is to make "the docs are accurate" a property a script verifies on
every commit, the way a linter verifies formatting — not a hope that has to
survive contact with a deadline.

## Install

```
make install    # go install -ldflags "-X main.Version=$VERSION" . — puts `cinch` on your PATH via GOBIN
```

`VERSION` comes from `internal/version/VERSION` via `scripts/update-version.sh
show` — see `make version-show`. `cinch version` prints the installed
binary's version (`dev` for a build that skipped `make`'s `-ldflags`, which
never gets checked against `require.cinch` — see § `core` below, so a local
dev build never reddens a consumer that pins a release).

The generated git hook shims run `exec cinch hook <event>` — a bare PATH
lookup, deliberately, so a shim never hardcodes one machine's path. The
consequence is that the binary has to be *installed*, not just built: a
plain `go build` produces a local binary for iteration, but a repo whose
hooks are activated needs `cinch` on `PATH`, or every commit fails with
`cinch: not found` (exit 127) before a single check runs.

Re-install after changing cinch itself, too. The workflow templates are
compiled into the binary (`go:embed`), so a stale installed binary re-renders
stale templates and the `generated` check reports a mismatch that isn't
really there.

## Adopting cinch

1. **Install the binary** — see § Install above.

2. **`cinch init`** in the target repo. Interactively (if stdin is a
   terminal) or with defaults otherwise, it writes `cinch.yml`, renders the
   workflows and hook shims into `paths.docs`/`paths.hooks`, and — in a git
   repo — activates the hooks with `git config core.hooksPath`.

   `cinch init` refuses to overwrite a `core.hooksPath` that's already set to
   something other than `paths.hooks` — if another tool (husky, lefthook,
   pre-commit) owns your hooks, point `paths.hooks` at that same directory or
   resolve the conflict by hand first.

3. **Point `paths.docs` somewhere else, if `.docs` doesn't suit** — set it in
   `cinch.yml` *before* the first `cinch init` or `cinch render`. It must
   name a directory inside the repo; a value that escapes it is refused when
   the manifest loads. `paths.hooks` carries the same constraint.

4. **Write the rules you want enforced** under `paths.docs`: numbered
   `1. **ID-NNN** ...` items in a markdown doc, a `// cinch:rule ID-NNN`
   marker above the test that enforces each one, and
   `<!-- cinch:ignore: <reason> -->` under any rule no test can cover. See §
   Status for the exact shapes. Nothing here is cinch-specific vocabulary —
   pick your own ID prefixes.

5. **Register your project's own checks** as `hooks.<event>.<name>.run`
   entries with a `when` path scope, so they run on the commits that touch
   the relevant paths and nowhere else. This is where project-specific
   automation belongs — never in cinch itself.

6. **`cinch check` exits 0.** That's the adoption criterion. Until you've
   written rules, `links` and `rules` pass trivially (reporting a skip, not a
   silent green — see § Status); `retirement`/`commit`/`core` announce
   themselves as skips rather than passes until their opt-in manifest keys
   are set; `hooks` already reports ok, since `cinch init` just activated it.

## Status

`cinch check` runs seven checks against the current directory. The docs root
defaults to `.docs`; a project can point it elsewhere with `paths.docs` in
`cinch.yml` — anywhere **inside the repository**. A value that escapes it,
absolute or `../`-relative, is refused when the manifest loads, naming the
key: cinch renders into that directory and scans it, and a root outside the
repo sends those two resolutions to different places, leaving every check
green against a corpus none of them can see. `paths.hooks` carries the same
constraint. `cinch render` substitutes the same value into templates as
`{{paths.docs}}` — one root, so a rule doc a workflow tells an agent to write
is guaranteed to be one `cinch check` actually scans.

- **links** (error) — a relative markdown link under the docs root that
  doesn't resolve on disk. Fence-aware: link syntax inside a ```` ``` ````
  block is not scanned. Reports `ok (N docs, M links checked)`, or a skip
  when the corpus has no markdown at all — "the links check enforces
  nothing" rather than a silent pass over zero files.

- **rules** (error) — a rule ID (`1. **ID-NNN** ...`) with no `// cinch:rule`
  marker anywhere in tracked (or untracked-but-not-gitignored) source, or a
  marker that resolves to no rule ID. A rule can declare `<!-- cinch:ignore:
  <reason> -->` under itself instead of a marker — a *declaration* that the
  rule is outside a test's domain, not a suppression; it must carry a
  reason, and a rule that's both ignored and marked is a contradiction and
  is itself a finding. `cinch ignores` lists every declaration with its
  reason. Reports `ok (N rules, M rule docs, K ignores)`, or a skip when the
  corpus has no rule IDs at all — "the rules check enforces nothing," so a
  green run can't be mistaken for a covered one when the corpus is simply
  empty. The marker itself is `// cinch:rule <ID>` — line-comment syntax
  only; see § Known limitations.

- **retirement** (error, opt-in) — a rule ID present at `HEAD^` is absent at
  `HEAD` with no recognized tombstone in its place. A tombstone convention is
  project-defined: set `retirement.pattern` in `cinch.yml` to a regexp, and
  an ID counts as tombstoned when a match against the file's `HEAD` content
  contains the ID's own text. Absent the key, the check is a skip — opt-in,
  not configured, same as `commit.pattern`. Bounded to one commit back; see §
  Known limitations.

- **generated** (error) — the render output on disk doesn't match what
  `cinch render` would produce right now: a missing file, altered bytes, an
  orphaned generated file (one no longer produced by the current render), or
  a render that fails outright (an undefined `{{key}}` in a template, say) —
  that last one is a finding naming the manifest, not a silent skip. A
  re-render diff proves tampering — this is the check that makes that claim
  true, and it's what makes hook shims tamper-evident: you cannot edit a
  generated `pre-commit` hook to skip cinch without `cinch check` failing.
  Also scans the docs/hooks roots implied by `cinch.yml` as of `HEAD`, not
  just the working copy's current value, so an uncommitted `paths.docs` edit
  that wasn't paired with moving the directory leaves the old root's
  generated files orphaned and caught. A skip, not a pass, when render
  genuinely hasn't run yet.

- **hooks** (skip) — always runs, no manifest key to configure. `git config
  --get core.hooksPath` must match `paths.hooks`. When it's unset or points
  elsewhere, nothing in the repository actually enforces anything at commit
  time no matter how clean the rest of `cinch check` reports — this check
  exists so that state is visible rather than silently assumed.

- **commit** (error, opt-in) — when `MSGFILE` is given and the manifest's
  `commit.pattern` key is set, the commit's subject line must match it.
  No `SQUASH_MSG`/`MERGE_HEAD` awareness — a project whose git flow includes
  merge commits or squash-merge subjects that don't fit `commit.pattern`
  should either scope the pattern to exclude those, or dispatch its own
  `commit-msg` script via `hooks.commit-msg.*` instead of setting this key.

- **core** (error, opt-in) — when the manifest's `require.cinch` key is set,
  the running binary's version must satisfy it. Accepts an exact version
  (`0.1.0`) or a `>=` minimum (`'>=0.1.0'`); a bare version is exact-match.
  Prefer `>=` unless you specifically need to pin a consumer to one release —
  an exact pin means every cinch release forces a commit in every consumer
  just to bump the number, even when nothing the consumer uses changed.
  Absent the key, no behavior change; an unreleased (`dev`) binary skips the
  comparison so a local dev build never reddens a consumer that pins a
  release. A mismatch is a finding naming both versions and the fix —
  reinstall cinch, then `cinch render`.

`cinch render` writes into `paths.docs`'s `workflows/` subdirectory (default
`.docs/workflows/`): `docs-philosophy.md` (copied verbatim — no variables, no
renderer needed) and one file per template under
`internal/cinch/docs/templates/` (value-substituted against `cinch.yml`).
And it writes a thin shim per supported git hook event under `paths.hooks`
(default `.githooks/`): `exec cinch hook <event> "$@"` — the dispatch table
lives in the manifest and is read at runtime, so editing it takes effect
immediately with no re-render. Every generated output is stamped with a
header carrying a body-sha. Substitution is `{{key}}` literal replacement
only — no loops, no conditionals — and an undefined variable is an error,
never a blank. The render is idempotent: running it twice produces
byte-identical output, so a re-render diff proves tampering.

Workflow and doc navigation are *not* part of the render output — nothing is
persisted, so nothing can drift or need re-rendering. `cinch workflows`
computes and prints the workflow trigger table from whatever is under
`workflows/` right now; `cinch index` computes and prints every doc's path
and title from whatever is under `paths.docs` right now (excluding
`plans/`). Both read the current tree at call time.

### `cinch init` and self-enforcement

`cinch init` is the entry point: run it once in a repo (a fresh one, or an
existing one adopting cinch) and it writes a starter manifest if none
exists, renders everything, and — in a git repo, if `core.hooksPath` is
unset or already points at `paths.hooks` — runs `git config core.hooksPath
<paths.hooks>` to activate the generated hooks. If `core.hooksPath` is set
to something else, `cinch init` stops and says so rather than silently
taking over another tool's hooks. Outside a git repo, the shims are still
generated but init says on stderr that they weren't activated.

Once activated, `.githooks/pre-commit`, `.githooks/commit-msg`, and
`.githooks/post-commit` all `exec cinch hook <event> "$@"`, which runs any
`hooks.<event>.<name>` entries a project has registered in its manifest,
scoped by `when` — a comma-separated list of path prefixes (not globs).
`pre-commit` matches `when` against the staged set; `post-commit` matches
against the committed set (`git diff --name-only HEAD^ HEAD`, falling back
to the commit's full tree for a root commit with no parent). `pre-commit`
and `commit-msg` also run `cinch check` first; `post-commit` does not — the
checks already ran before the commit existed, and git fires `post-commit`
for commits `pre-commit` never gated (e.g. `git commit --amend`). Every
registered entry runs regardless of an earlier one's failure (failures
accumulate, not fail-fast), and a skipped entry says so on stderr, so a hook
run that did nothing is never silent. `when` is ignored for `commit-msg` — a
commit message has no changed paths to scope against. A
`hooks.commit-msg.*` entry's command receives the commit message file path
as `$1`, the same argument git itself hands the `commit-msg` hook;
`pre-commit`/`post-commit` entries get no positional args.

**Extension boundary:** cinch guarantees *dispatch* — did the `when`
prefixes match the staged/committed set, did the registered command run, was
its exit code propagated — not your script's correctness. What a registered
script does is opaque to cinch; a green `cinch check` means dispatch worked,
not that the script itself is bug-free.

**Worked example.**

```yaml
# cinch.yml
paths:
  docs: .docs           # docs corpus root
  hooks: .githooks       # where generated hook shims land

hooks:
  pre-commit:
    error-codes:
      run: scripts/check_error_codes.sh
      when: [frontend/src/lib/api/, backend/errors/, frontend/src/lib/translations/]
    frontend:
      run: make check-frontend
      when: [frontend/]

commit:
  pattern: '^\[[a-z-]+\] .+'

retirement:
  pattern: '<!--\s*retired:.*-->'

require:
  cinch: '>=0.1.0'      # minimum core version; a lower version is a `core` finding
```

### Commands instead of per-harness skills or a persisted index

Don't hand-maintain a `SKILL.md`/slash-command wrapper per workflow per agent
harness — that's one file per workflow times one per harness, all of it
duplicated boilerplate that only ever says "read this file." And don't
persist a doc/workflow index either — a written-to-disk index is one more
render output that can drift and one more thing `cinch check` has to verify.
Compute both on demand instead:

```
Workflows: run `cinch workflows` to see what's available, `cinch workflow <name>` to load one.
Docs: run `cinch index` to see every doc's path and title.
```

Both are commands, not paths, so neither can go stale under a customized
`paths.docs`. Any agent that reads its own onboarding doc — not just one
harness's proprietary skill system — can run these commands directly.

Exit codes: `0` clean, `1` findings (`check`) or a render/dispatch failure,
`2` usage error.

## Layout

- `main.go` — CLI dispatch (`help`, `version`, `init`, `check`, `ignores`,
  `render`, `hook`, `workflows`, `workflow`, `index`); the only `package
  main` file — everything else lives in `internal/cinch`, a private package
  the Go compiler forbids other modules from importing.
- `internal/cinch/check.go` — the `finding`/`checkResult` model and
  `CmdCheck` orchestrator; runs every check concurrently and streams status
  lines as they finish.
- `internal/cinch/links.go`, `internal/cinch/rules.go`,
  `internal/cinch/retirement.go`, `internal/cinch/generated.go`,
  `internal/cinch/hooks.go`, `internal/cinch/commit.go`, `internal/cinch/pin.go`
  — one file per check; `internal/cinch/semver.go` holds the version-parsing
  and comparison logic `pin.go`'s `core` check calls into.
- `internal/cinch/render.go` — the `{{key}}` substituter, header/body-sha,
  hook shims, and `CmdRender`; embeds `docs/docs-philosophy.md` and the whole
  `docs/templates/` directory via `go:embed` (single static binary, no
  runtime template resolution) and iterates it, so adding a template needs no
  code change.
- `internal/cinch/title.go` — H1 title/trigger extraction, shared by
  `workflow.go` and `index.go`.
- `internal/cinch/manifest.go` — the `cinch.yml` parser and its accessors
  (`list`, `names`, declaration order).
- `internal/cinch/hook.go` — `cinch hook`'s dispatcher: staged-set
  computation, `when` prefix matching, command execution.
- `internal/cinch/init.go` — `cinch init`.
- `internal/cinch/workflow.go` — `cinch workflows` / `cinch workflow NAME`,
  computed on demand from `workflows/` on disk.
- `internal/cinch/index.go` — `cinch index`, computed on demand from
  `paths.docs` on disk.
- `internal/cinch/docs/docs-philosophy.md` — copied verbatim into every
  consumer.
- `internal/cinch/docs/templates/*.md` — the workflow templates cinch ships,
  grouped by a shared filename prefix: the feature/bug dev lifecycle
  (`dev-plan-feature`, `dev-fix-bug`, `dev-execute-plan`, `dev-task-primitive`)
  and the docs-corpus toolkit (`docs-audit-quality`, `docs-audit-coverage`,
  `docs-maintain-domain`, `docs-sync`); each renders to
  `<paths.docs>/workflows/<name>.md`.
- `.docs/conventions.md` — the Go style rules this codebase follows;
  `.docs/git-conventions.md` — branch model, commit-message shapes, and the
  auto-versioning scheme in `scripts/`.
- `Makefile` — `build`, `install`, `setup-hooks`, `version-show`,
  `version-major`, `release`.

## Known limitations

**`retirement`'s window is bounded to the single most recent commit.** It
only ever diffs `HEAD^` against `HEAD`: a rule ID removed several commits
back is only caught if `retirement` happened to run (as a pre-commit hook,
typically) on the very next commit after the removal — once a second commit
lands on top, the removal is no longer the `HEAD^`→`HEAD` transition and
drops out of view for good. Consistent, regular use of the pre-commit hook
is what keeps the window from ever skipping a commit; a repo that only runs
`cinch check` sporadically (e.g. once per PR in CI) can lose coverage this
way.

**The marker scan is line-comment syntax only (`//`).** A rule enforced by a
test in Python, Ruby, shell, SQL, or any language whose comment leader isn't
`//` currently can't carry a `// cinch:rule` marker in idiomatic form.

**Pre-commit checks the working tree, not the git index.** A partially
staged commit is verified against content that isn't actually being
committed. Stage everything you mean to commit before relying on the
pre-commit hook.

**Templates are not yet overridable.** The workflow pack embedded in the
binary is one opinionated set; there is no override mechanism to swap in a
project's own prose while keeping the verification pipeline (rendering,
generated-file checking, header stamping). A project that wants different
prose today either accepts cinch's, or maintains its own hand-authored
workflow file outside `cinch render`'s reach entirely (not verified by the
`generated` check).
