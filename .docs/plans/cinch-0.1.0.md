# cinch 0.1.0 — init, self-enforcement, manifest-driven extension

Status: approved — not started

## Context

cinch today is Stage 2: three checks (`links`, `rules`, `coupling`), `render`
of philosophy + eight workflow templates, and a flat `.docs/manifest`. It is a
*checker you must remember to run*. Three gaps block it from being the tool the
brief describes:

1. **No entry point.** A new project has to hand-create `.docs/manifest`, guess
   the directory layout, and know that `render` exists.
2. **Nothing enforces cinch.** The README claims "a re-render diff proves
   tampering" — but nothing ever runs that diff, and nothing runs `cinch check`
   on commit. `bin/cinch check` currently exits **1 against its own repo** (8
   findings), so cinch cannot even gate itself.
3. **No extension seam.** `project_deltadocs` adopted cinch and then hand-built
   everything around it that cinch didn't cover: `.githooks/{pre-commit,
   commit-msg,post-commit}`, `scripts/docs-index.sh` (a re-implementation of a
   `cinch index` the rebuild dropped — its own header says so), a second
   `manifest.yml`, and per-harness `SKILL.md` shims. Each of those is a repo
   paying rent for a gap in the tool.

0.1.0 closes all three while staying stack- and harness-agnostic: a project
runs `cinch init`, gets a doc structure, a manifest, rendered workflows, a
generated doc map, and git hooks that run cinch on every commit — and can hook
its *own* checks (`scripts/check_error_codes.sh`) to those hooks through the
manifest, scoped to the paths that should trigger them.

This file supersedes and replaces the three session plans it was built from —
they were committed in `72089ce` and deleted here; git is the archive.
Folded in: `self-check-findings.md` (F1–F4), `workflow-cli.md` (the
`workflows`/`workflow` commands), `deltadocs-extraction.md` (§1 list values,
§5 commit pattern; §2–§4 deferred, see *Deferred*).

## Surface after 0.1.0

```
cinch init                 scaffold + render + activate hooks (idempotent)
cinch check [MSGFILE]      links, rules, coupling, generated  (+ commit pattern)
cinch render               workflows, doc index, hook shims
cinch hook <event> [args]  git-hook dispatcher (pre-commit, commit-msg)
cinch workflows            print the workflow trigger table
cinch workflow NAME        print one workflow
cinch ignores              list cinch:ignore declarations
```

New manifest keys, all optional, all absence-based:

```
paths.docs  = .docs        # existing
paths.hooks = .githooks    # where generated hook shims land

hooks.pre-commit.error-codes.run  = scripts/check_error_codes.sh
hooks.pre-commit.error-codes.when = frontend/src/lib/api/, backend/errors/
hooks.pre-commit.frontend.run     = make check-frontend
hooks.pre-commit.frontend.when    = frontend/

commit.pattern = ^\[[a-z-]+\] .+
```

---

## Flag for discussion — where this pushes on `.docs/PRINCIPLES.md`

These are decisions to accept explicitly, not to discover later.

1. **`hooks.*` is shape living in a value store (principle 5).** The manifest is
   supposed to bind *values*, and shape variance is answered by absence, not by
   growing the schema. `hooks.<event>.<name>.{run,when}` is a small table. The
   mitigations: the file format does not change (still flat
   `dotted.key = value`, hand-written parser, no nesting, no dependency), the
   field set is closed at two, grouping is by key prefix, and absent keys mean
   the hook runs only `cinch check`. It is still schema growth. The alternative
   — markers inside the scripts — was considered and rejected in favour of
   "bind project specifics in one place."
2. **cinch cannot mutation-fixture a consumer's script (principles 1 and 6).**
   `check_error_codes.sh` is opaque to cinch. What cinch owns is *decidable and
   fixtured*: did the `when` prefixes match the staged set, did the command run,
   was its exit code propagated. The README must state that boundary — "cinch
   guarantees dispatch, not your script's correctness" — rather than let the
   green light imply more, per the principles' own instruction to state the
   boundary rather than pretend a check closes it.
3. **The `generated` check couples a green tree to the binary version.**
   Upgrading cinch reddens every consumer until `cinch render` is re-run. The
   green state is reachable and the action is exact (principle 6 satisfied), but
   it is a real ergonomic cost. deltadocs' `harness_version` pin + warn was the
   alternative; rejected — a warn whose close action is fuzzy is the proxy
   metric principle 6 kills.
4. **A generated doc index is a copy of a machine-readable fact (principle 2).**
   The direction rule says a doc that copies a machine fact should be *deleted*,
   not validated. The defence: it is never authored, it is a build artifact,
   and it is verified by byte-equality rather than by a semantic check — nobody
   maintains it, so there is nothing to go stale by hand. It is also load-
   bearing right now: five references in the shipped templates already point at
   `{{paths.docs}}/index.md`, a file cinch stopped generating, which is exactly
   why deltadocs wrote `scripts/docs-index.sh`.
5. **Hook shims are generated unconditionally**, not absence-selected. They are
   inert until `core.hooksPath` is set, and self-enforcement is the release's
   point. A consumer opts out by not activating them.
6. **`cinch init` writes `AGENTS.md` only when absent, and no check ever asserts
   its contents.** "AGENTS.MD initialized" was the first proxy metric ever cut
   (principle 6) — it stays cut. When the file exists, init prints the line to
   add and touches nothing.
7. **Fixed, not deferred:** `manifestPath` was pinned to `.docs/manifest`, so
   reading it couldn't depend on the value it defines. A repo that sets
   `paths.docs = .agent` (deltadocs does) would then end up with two
   directories — the exact outcome the pin's own comment said it avoided.
   Resolved by relocating the manifest to the repo root as `cinch_manifest`
   (Step 0), fully decoupled from `paths.docs`: the manifest's own location
   no longer depends on a value it defines, and `paths.docs` now exclusively
   controls where rendered/authored docs live.

---

## Step 1 — make cinch clean against itself (prerequisite for everything)

`bin/cinch check` exits 1 on this repo. Self-enforcement is impossible until it
exits 0. Fixes F1 from `.docs/plans/self-check-findings.md`, plus the residual
that plan left open.

**`internal/cinch/rules.go`**
- `scanRuleMarkers` (line 127): track `inFence` with the existing `fenceRe`,
  same toggle `parseRuleItems` and `checkLinksInFile` already use. Kills the
  `docs/templates/audit-domain.md:129` finding, which currently ships to every
  consumer.
- `markerRe` (line 17): anchor the ID's end — `` `//\s*cinch:rule\s+([A-Z0-9]+-[0-9]+)\b` ``
  so `SIG-0NN` no longer truncates to a bogus `SIG-0`.

**Test fixtures** (`rules_test.go`, `coupling_test.go`): the remaining 7
findings are cinch's own test fixtures containing literal marker text. Add a
helper that composes marker text at runtime so the source never contains a
scannable literal:

```go
// marker returns a // cinch:rule line for id, composed at runtime so this
// file's own source carries no literal marker for the scan to find.
func marker(id string) string { return "// cinch:" + "rule " + id }
```

**New unit tests (mutation fixtures):** `TestRules_FencedMarkerIsIgnored`
(fenced marker in a non-docs `.md` → no finding), `TestRules_TruncatedIDNotMatched`
(`SIG-0NN` → no finding).

**Verify:** `./bin/cinch check` in this repo → exit 0.

## Step 2 — coupling correctness for hook use (F2, F3)

**`internal/cinch/coupling.go`**
- `gitChangedFiles` (line 153): union `git diff HEAD --name-only` with
  `git ls-files --others --exclude-standard`, deduplicated. A rule's marker in a
  brand-new untracked test file currently false-positives a block finding.
- `checkCoupling` (line 31): before the walk, resolve `docsDir` and `repoRoot`
  to absolute paths; if `filepath.Rel` escapes the repo, return
  `NoOp: "coupling: paths.docs is outside the repository — check did not run"`.
  Today that case is a silent pass, contradicting the no-silent-pass contract.

**Tests:** `TestCoupling_UntrackedMarkerFileCountsAsChanged`,
`TestCoupling_DocsRootOutsideRepoAnnouncesNoOp`.

Note for the README: coupling's window stays working-tree-vs-HEAD, so a
pre-commit run sees unstaged edits too. Same compromise deltadocs' pre-commit
makes (gate on staged paths, run checks on the tree).

## Step 3 — manifest accessors

**`internal/cinch/manifest.go`**
- Preserve declaration order: add an `order []string` field to `Manifest`,
  appended on first sight of a key in `parseManifestFile`. Hook entries then run
  in the order the author wrote them rather than alphabetically.
- `func (m *Manifest) List(key string) []string` — comma-split, trim, drop
  empties, nil for a missing key (deltadocs-extraction §1).
- `func (m *Manifest) Names(prefix string) []string` — distinct next path
  segments under `prefix`, in declaration order.
  `Names("hooks.pre-commit")` over the example manifest → `["error-codes", "frontend"]`.

`substitute` is untouched — every value stays an opaque string to templates.

**Tests:** split/trim/empty-item/missing-key for `List`; declaration order and
distinctness for `Names`.

## Step 4 — genericize the shipped templates

The templates hardcode deltadocs' structure — `{{paths.docs}}/backend/troubleshooting.md`,
`/frontend/e2e.md`, `/api/`, `/models/`, `/overview.md`, `/conventions.md` — in
the one artifact every consumer receives. Keep only the paths cinch itself
owns: `workflows/`, `plans/`, `plans/fix/`, `index.md`.

Rewrote 15 references (the `{{paths.docs}}/(backend|frontend|api|models)`
pattern the mechanical guard below enforces) to route through the doc map
instead — e.g. "the troubleshooting doc for the affected area (find it in
`{{paths.docs}}/index.md`)". The doc map is the indirection that lets workflow
prose stop guessing at a project's layout. They landed in `fix-bug.md`,
`task-primitive.md`, `sync-docs.md`, and `plan-feature.md` — not the file
list originally guessed here; `audit-docs.md`, `audit-domain.md`, and
`maintain-domain.md` had no `backend|frontend|api|models` references to begin
with.

**Mechanical guard (`render_test.go`):** `TestTemplates_NoStackSpecificPaths` —
walk the embedded templates and fail on
`{{paths.docs}}/(backend|frontend|api|models)`. A decidable check that keeps
the leak from coming back.

## Step 5 — doc map as a render output

**`internal/cinch/render.go`**
- `buildDocMap(files []renderFile, docsRoot, root string) (renderFile, error)`
  → `<paths.docs>/index.md`: `# Doc index` plus one `` - `rel` — <H1> `` line per
  markdown file, sorted by path.
- **Source set = union of** the `.md` files already on disk under `docsRoot`
  **and** the dests in `files`. Without the union the first render emits an
  index missing the workflows it is writing in the same pass and a second
  render differs — idempotency breaks.
- Excludes `plans/` (transient work artifacts) and `index.md` itself.
- No H1 → render fails with the file named: "every doc needs a `# Title` line."
  Same behavior deltadocs' generator settled on, and it gives the index a real
  failing state.
- Reuse `titleAndTrigger`'s H1 scan; the first `# ` line wins, so the generated
  header above a rendered body is skipped correctly.

This retires `scripts/docs-index.sh`, `make docs-index`, and `make
docs-index-check` in any consumer.

**Tests:** index lists authored docs and rendered workflows; `plans/` excluded;
second render byte-identical (extend `TestRenderAll_IdempotentReRender`);
missing-H1 fires.

## Step 6 — the `generated` check

**New `internal/cinch/generated.go`** — `checkGenerated(root string) generatedResult`
(same `Findings` / `NoOp` shape as `couplingResult`):

- No `cinch_manifest`, or the workflows dir doesn't exist → `NoOp` on stderr
  ("cinch render has not run — nothing to verify"), never a silent pass.
- Otherwise call `renderAll` in memory and compare against disk:
  missing file, or bytes differ → `Finding{Check: "generated", Level: "error"}`
  with the action verbatim: "run `cinch render` (or revert the edit)".
- Orphans: any file under the render dirs whose first line starts with
  `headerPrefix` but is not in the expected set → finding.

Wire into `CmdCheck` (`check.go:64`) alongside links/rules/coupling.

This is the check that makes the README's "a re-render diff proves tampering"
true, and it is what makes the hook shims tamper-evident: you cannot edit a
generated pre-commit hook to skip cinch without `cinch check` failing.

**Mutation fixtures:** render → clean; flip one byte → fires; delete one file →
fires; drop a cinch-headered orphan into the workflows dir → fires; unrendered
repo → no-op, not a pass.

## Step 7 — `cinch hook` and generated shims

**Shims** (`render.go`, alongside `buildIndex`/`buildDocMap`): for each supported
event, a `renderFile` at `<paths.hooks>/<event>` (default `.githooks/`) with
body `exec cinch hook <event> "$@"`.

Two mechanical changes this needs:
- `renderFile` gains `Mode os.FileMode` (default `0o644`, hooks `0o755`) and a
  comment style. `header()` becomes style-aware: markdown keeps the
  `<!-- ... -->` form; shell emits shebang first, then a `#`-comment header —
  a shebang cannot sit below a comment. `bodyHash` stays over `Body`.
- Bodies never change when the manifest changes, because the dispatch table is
  read at runtime by the binary. Manifest edits take effect immediately with no
  re-render, and the shims stay byte-stable.

**New `internal/cinch/hook.go`** — `CmdHook(root, event string, args []string) int`:

- `pre-commit`:
  1. staged set ← `git diff --cached --name-only --diff-filter=ACMR`
  2. run `CmdCheck("")` in-process
  3. for each `hooks.pre-commit.<name>`: run when `when` is empty, or when any
     staged path has one of the `when` values as a **path prefix** (anchored at
     the start, exactly what deltadocs' `case "$f" in frontend/*` did — prefix
     matching, not globs; documented as such)
  4. execute via `sh -c` from the repo root, streaming stdout/stderr
  5. run every entry, report every failure, exit 1 if any failed (deltadocs'
     `status=1` accumulate pattern, not fail-fast)
- `commit-msg`: `CmdCheck(args[0])` — feeds the `rule-reword:` escape hatch —
  then any `hooks.commit-msg.*` entries (`when` is ignored there and the README
  says so).
- Unknown event → exit 2. Skipped entries are announced on stderr, so a hook
  that ran nothing says so.

**`commit.pattern`** (deltadocs-extraction §5, `internal/cinch/commit.go`):
when `MSGFILE` is given *and* the key is present, a non-matching message is a
`Finding{Check: "commit", Level: "error"}`. Absent key → no behavior change.
Deliberately excluded: branch conventions, auto version bumping, `post-commit` —
app-specific, and out of scope for a docs-integrity tool.

**Tests:** dispatch unit tests (matching / non-matching `when`, empty `when`,
exit-code propagation, multi-failure accumulation); CLI test driving a real
`git commit` in a temp repo with a failing registered script → commit blocked;
same repo with the script's paths untouched → commit succeeds.

## Step 8 — `cinch init`

**New `internal/cinch/init.go`** — `CmdInit(root string) int`, idempotent and
non-destructive to anything authored:

1. `cinch_manifest`: keep if present; otherwise write a starter with
   `paths.docs`, `paths.hooks`, and a commented `hooks.pre-commit.*` example.
2. `mkdir` `<paths.docs>/` and `<paths.docs>/plans/` (with `.gitkeep`).
3. Render everything (workflows, doc map, hook shims).
4. Git repo → `git config core.hooksPath <paths.hooks>`; not a git repo →
   generate the shims anyway and say on stderr that they were not activated.
5. `AGENTS.md`: absent → write a minimal one carrying the workflows line;
   present → print the exact line and touch nothing. Never checked (flag 6).

**The load-bearing CLI test:** `cinch init` in an empty git repo, then
`cinch check` → **exit 0**. And `init` twice → byte-identical tree. If those
two hold, the release works.

## Step 9 — `cinch workflows` / `cinch workflow NAME`

Implement `.docs/plans/workflow-cli.md` as written — `internal/cinch/workflow.go`
with `listWorkflowNames` / `CmdWorkflows` / `CmdWorkflow`, the shared
`workflowsSubdir` constant replacing `render.go:79`'s inline literal, flat
dispatch in `main.go`, and its exit-code table (1 for "render hasn't run", 2 for
argument shape). Its full test list carries over, including
`TestWorkflow_CustomDocsRootFromManifest`.

The point is the consumer's `AGENTS.md` line becomes a command, not a path, so
it cannot go stale under `paths.docs`:

```
Workflows: run `cinch workflows` to see what's available, `cinch workflow <name>` to load one.
```

## Step 10 — cinch self-hosts, and the docs land

- Add this repo's `cinch_manifest` (`paths.docs = .docs`, plus
  `hooks.pre-commit.build.run = make test`), run `cinch init`, commit the
  rendered tree and `.githooks/`, set `core.hooksPath`.
- **README.md:** new commands and manifest keys; the extension boundary from
  flag 2 stated plainly; the version-skew note from flag 3; the AGENTS.md line
  from step 9; `## Known limitation` rewritten (F4) — the self-check is clean
  now, and the residual is that a repo legitimately quoting marker syntax
  *outside* a fence is still scanned.
- **AGENTS.md:** its hand-enumerated `internal/cinch/*.go` list gains
  `generated.go`, `hook.go`, `init.go`, `workflow.go`, `commit.go`.

---

## Deferred, with reasons

- **Seam guard** (deltadocs-extraction §3) — the working artifact is a Claude
  Code `PreToolUse` Python hook. Generating harness-specific hooks breaks
  agnosticism. The agnostic form is a command (`cinch seam <role> --allows`)
  that any harness shims in three lines; 0.2.0, after `hooks.*` proves the
  manifest table pattern.
- **SessionStart injection** (§4) — same reason. The agnostic form is
  `cinch context`, printing AGENTS.md + branch + plan status for any harness's
  session-start mechanism. Cheap, but 0.1.0 is already at the holdability line
  (principle 3).
- **`owns:` front-matter check** (§2) — a genuinely good structural check with
  no dependency on anything here. Nothing blocks it; it just isn't required by
  init / self-enforcement / extension.
- **Auto version bumping, branch-name conventions, `post-commit`** — app-specific.
  A consumer keeps hand-writing those, as deltadocs does.

## Verification

Each step lands and is verified independently; `make test` after each.

1. **Self-clean:** `./bin/cinch check` in this repo → exit 0 (was exit 1 with 8
   findings). This is the gate for steps 7–10 meaning anything.
2. **Mutation fixtures** for every new check (`generated`, `commit`) and for the
   hook dispatcher: a passing fixture and a one-change failing fixture, in the
   test file, as `links_test.go` / `rules_test.go` already do.
3. **Idempotency:** `cinch render` twice → byte-identical, including the doc map
   and the hook shims.
4. **End-to-end in a scratch repo** (the real proof):
   - `git init` → `cinch init` → `cinch check` exits 0
   - edit one rendered workflow by hand → `cinch check` exits 1 with a
     `generated` finding naming `cinch render`
   - add a doc without an H1 → `cinch render` fails naming the file
   - register `hooks.pre-commit.demo.run = false` with
     `when = src/`; commit a change under `src/` → blocked; commit a change
     under `docs/` → clean
   - move `paths.docs` to `mydocs`, re-render, `cinch workflow maintain-domain`
     still resolves with no path argument anywhere
5. **Against `project_deltadocs`** (the strain test the README calls Stage 3):
   `cinch init` there, confirm `scripts/docs-index.sh`, `make docs-index`,
   `make docs-index-check`, and the pre-commit doc-index branch can all be
   deleted, and that `check_error_codes.sh` reattaches as three manifest lines.
