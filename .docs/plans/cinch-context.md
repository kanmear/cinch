# cinch — `cinch context`, the session-start command

Status: **proposed** — not started.

## Context

The daily loop is: start session → re-orient → pick workflow → execute →
commit. cinch can deterministically compress the first two steps, because
every input to them is decidable — current branch, open plans and their
status, the workflow trigger table, staged paths. No model, no judgment, no
new state.

This was deferred during 0.1.0 on holdability grounds ("0.1.0 is already at
the holdability line"; the plan was deleted in `0ed1ebd`, see git history).
That argument has served its purpose: 0.1.0 has since proven itself on two
real repos, and the deferral was about *when*, not *whether*.

The strategic case is that cinch's center of gravity currently sits on the
verification side while the thing being protected is passive — `cinch
workflow <name>` prints markdown and gets out of the way. A checker earns its
keep invisibly, by the gaps that never opened; that makes it easy to stop
running. `cinch context` is the one command that gives cinch a *visible* face
in the daily loop without adding anything to verify.

Harness-neutral by construction, in the same way the AGENTS.md pointer lines
are: a harness with a session-start hook calls it; a harness without one has
AGENTS.md say "run `cinch context`." The distribution is the point — it lives
in the repo, versioned, invocable by any harness or by a human, rather than
in per-harness config that evaporates with the session.

## Not a check

Stated up front so it doesn't drift back: `cinch context` produces no
`Finding`, adds no entry to the five-check list, and has no exit-code
semantics beyond 0 / a read failure. A `cinch context` that reports a red
state is a sixth check wearing a different hat, and a checker whose green
state is "you have read your context" is exactly the proxy metric principle 6
forbids.

## What it prints

All decidable from the tree at call time, nothing persisted:

- Current branch.
- Plan files under `<paths.docs>/plans/`, each with its title and its
  `Status:` header line.
- The workflow trigger table.
- Staged paths.

## Decision needed — plan status

Every current plan carries a freeform status line (`Status: **proposed** —
not started.`, `Status: **partially shipped** — …`). Does context print that
line verbatim, or parse a fixed status vocabulary?

Recommend **verbatim**. Parsing means minting a status vocabulary, which is a
new convention, which needs enforcing, which means a check — the exact
expansion this command is supposed to avoid. Verbatim degrades gracefully: a
plan with no status line prints its title alone.

## Decision needed — last check result

There is no persisted check state, deliberately: 0.1.0 removed every
persisted index for the reason that nothing persisted can drift. Two options:

- **(a) Leave it out.** Context reports what the tree says; whether the tree
  is green is what `cinch check` is for, and the hooks already run it on
  every commit.
- **(b) Run the checks inline.** Accurate, but a session-start command that
  silently runs five checks is a checker again, and it makes the command slow
  enough to skip.

Recommend **(a)**.

## Reuse, don't rebuild

Named explicitly so the implementation doesn't grow a second copy of any of
them:

- `ResolveDocsRoot(root)` — `internal/cinch/check.go:31`. Honors
  `paths.docs`; the one place the docs root is resolved. (Note: see
  `absolute-docs-path.md` — an absolute value currently resolves differently
  here than on the render side. Context reads, so it follows this function.)
- `workflowsTable(docsRoot)` — `internal/cinch/workflow.go:41`. Already
  produces the trigger table `cinch workflows` prints.
- `titleAndTrigger(body)` — `internal/cinch/title.go:12`. Shared H1
  title extraction; use it for plan titles rather than a second parser.
- `indexList(docsRoot)` — `internal/cinch/index.go:23`. **Not** directly
  reusable for plans: it deliberately excludes `plans/` (`index.go:42`), so
  context needs its own small directory scan. Read it for the walk-and-title
  pattern, don't call it.
- `output.Step` / `output.Fail` — `internal/output/output.go:76,54`.

Branch and staged paths come from git; `internal/cinch/coupling.go` already
shells out to git for the working-tree-vs-HEAD comparison — follow whatever
invocation shape it uses rather than introducing a second one.

## Step 1 — implement

- `internal/cinch/context.go`: `CmdContext(root string) int`.
- `main.go`: a dispatch case mirroring `index`'s zero-argument shape
  (`main.go:114-118`), plus the `usage` string entry.
- Outside a git repo, or with no `plans/` directory: degrade to printing what
  is available, saying on stderr what was skipped and why. Same contract the
  coupling check and `cinch init` already follow for the not-a-git-repo case
  — a stated skip, never a silent one.

## Step 2 — document

README entry alongside `cinch workflows` / `cinch index` (the "computed on
demand, nothing persisted" paragraph at `README.md:133-138` is where this
belongs — context is a third member of that family). AGENTS.md's session-start
section becomes "run `cinch context`."

## Verification

- `make test` green, including a CLI-level test in `tests/cli_test.go`.
- `./bin/cinch context` in this repo prints the current branch, this repo's
  plan files with their status lines, and the workflow trigger table.
- Run it in a scratch repo with no `plans/` directory: it degrades with a
  stated skip rather than erroring.
- Run it outside a git repo: same.

### Critical files

- `internal/cinch/context.go` (new), `internal/cinch/context_test.go` (new)
- `main.go` (dispatch + usage)
- `tests/cli_test.go`
- `README.md`, `AGENTS.md`

### Relationship to other plans

Independent of everything currently open. Pairs with
`workflow-postconditions.md` as the two halves of making cinch the
*runtime* for the workflows rather than their warehouse — this one is the
entry point, that one is the exit criterion.
