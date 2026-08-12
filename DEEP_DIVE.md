# cinch — deep dive

A line-by-line, mechanism-by-mechanism walk of what cinch is, what it does
right now, and how it achieves it. Written from the current tree (`HEAD =
ea3e342`), `make test` green. Source references are `file:line`. This is a
point-in-time exploration artifact, not a spec — the spec lives in
`.docs/PRINCIPLES.md`, and session state lives in plan files and git.

---

## 1. What cinch is, in one breath

cinch is a referential-integrity checker for the **operational documentation**
that governs a repository — the rules, workflows, and conventions code must
conform to, read by humans and executed by agents. It doesn't understand what
a rule *means*; it enforces that documented things are bound to concrete,
*script-decidable* facts on disk and in git:

> "The docs are accurate" is made into a property a script verifies on every
> commit, the way a linter verifies formatting — not a hope that has to
> survive contact with a deadline. (README.md)

Concretely, five checks (`links`, `rules`, `coupling`, `generated`,
`commit`), a `render` command that generates consumer-facing workflow docs
and git-hook shims, an `init` entry point, a `hook` dispatcher, and the
`workflows`/`workflow`/`ignores` readers — all one static Go binary, no
runtime dependencies, no network, one `go.mod` line (`module cinch`, `go
1.22`, zero imports outside the stdlib).

---

## 2. The failure mode it exists for

Operational docs drift from what the code actually does:

- a "business rule" written eighteen months ago quietly stops being enforced
  by any test;
- a workflow file tells an agent to read a path that hasn't existed since the
  last refactor;
- a troubleshooting doc describes a bug that was already fixed.

The doc *looks* authoritative and isn't, and nothing catches the gap until the
next trusted reader (increasingly, an autonomous agent) acts on stale
information. The `.docs/PRINCIPLES.md` file records the hard-won evidence:
the previous design's semantic-audit layer confabulated ✅s fluently, 3/3
pre-blinding, which is why cinch's whole bet is **determinism — a script can
return a false negative only if its decision procedure is wrong, while a model
will happily tell you a wrong thing is right.**

---

## 3. Big-picture architecture

A single Go module with exactly one `package main` file and one internal
package:

```
main.go                     CLI dispatch. The only package main — everything
                            else lives in internal/cinch, which the Go
                            compiler makes un-importable from outside this
                            module (the "internal" rule = enforced privacy).

internal/cinch/
  check.go      the Finding model + the CmdCheck orchestrator
  links.go      check 1 — link resolution
  rules.go      check 2 — rule IDs ↔ source markers
  coupling.go   check 3 — rule text ↔ enforcing test file, transition check
  generated.go  check 4 — render output vs. a fresh in-memory render
  commit.go     check 5 — commit message subject vs. commit.pattern
  render.go     substitution, headers/body-sha, hook shims, CmdRender;
                go:embed carries the shipped content
  title.go      H1 title/trigger extraction, shared by workflow.go and docs.go
  manifest.go   the cinch_manifest parser + List/Names accessors
  hook.go       CmdHook dispatcher (pre-commit / commit-msg)
  init.go       CmdInit — the scaffold entry point
  workflow.go   CmdWorkflows / CmdWorkflow, computed on demand
  docs.go       CmdDocs, computed on demand
  docs/         philosophy + templates, embedded into the binary
  *_test.go     one test file per .go file, each carrying mutation fixtures
tests/
  cli_test.go   end-to-end tests against the built ./bin/cinch binary
```

The five execution domains:

| Command | What it does |
|---|---|
| `cinch init` | Scaffold a whole consumer: manifest, docs/plans, full render, activated git hooks, AGENTS.md lines. Idempotent. |
| `cinch check [MSGFILE]` | Run all five checks against the current directory. `MSGFILE` is the in-progress commit message (commit-msg hook contract). |
| `cinch render` | Regenerate every derived artifact: workflows, hook shims. Idempotent. |
| `cinch hook EVENT [ARGS]` | Dispatcher the generated shims `exec` into. |
| `cinch workflows` / `cinch workflow NAME` / `cinch docs` | Compute and print the trigger table / one workflow / the doc list — nothing persisted. |
| `cinch ignores` | List every `cinch:ignore` declaration with its reason. Always exits 0. |

Exit codes: `0` clean; `1` findings (check) or a render/dispatch/state
failure; `2` usage/argument errors (`main.go:38`). `main.go:41` — every
command is dispatched on the CWD (the root argument is hardcoded `"."`).

---

## 4. The manifest: `cinch_manifest`

### 4.1 Location and format

The manifest binds the values templates and hooks reference. It lives **at the
repo root, pinned as `cinch_manifest`** (`manifest.go:17`), deliberately not
inside `paths.docs` — the manifest is what *defines* `paths.docs`, so reading
it can't depend on a value it defines (`manifest.go:12-16`; this was a
deliberate mid-release fix, see plan flag 7).

Format: a flat, hand-written `dotted.key = value` text parser — no schema, no
nesting, no dependency (`manifest.go:59-95`). Blank lines and `#` comment
lines are skipped; each line is split on the first `=`; a line with no `=`
(or an empty key) is a named, line-numbered error.

The parser preserves **declaration order** via an `order []string` field
(`manifest.go:29`, appended on first sight of a key) — this is why hook
entries run in the order the author wrote them, not alphabetically.

### 4.2 The keys cinch itself reads

| Key | Meaning | Default | Consumed by |
|---|---|---|---|
| `paths.docs` | docs corpus root (relative to repo root, or absolute) | `.docs` | check, ignores, render, workflows, workflow, init |
| `paths.hooks` | where generated hook shims land | `.githooks` | render, init |
| `hooks.<event>.<name>.run` | a command to run for that git hook | — | hook |
| `hooks.<event>.<name>.when` | comma-separated path prefixes that must match the staged set | empty = always | hook |
| `commit.pattern` | regex the commit subject must match | — | commit check |

Only `paths.docs` is referenced by any shipped template value. Every other key
the shared templates mention (`development.commands`, `taxonomy.test_tiers`,
`manifest.paths.tests`, `rule_prefix`... ) appears in shipped-workflow *prose*
as project content the consumer defines — cinch itself never reads them. That
is principle 5 in action: manifest binds values, not shape; a key that would
serve exactly one template is project content, not a manifest key.

### 4.3 Accessors (`manifest.go:100-146`)

- `List(key)` — comma-split, trimmed, empties dropped; `nil` for a missing
  key (distinguishable from empty).
- `Names(prefix)` — distinct next path segment after `prefix.`, in
  declaration order. `Names("hooks.pre-commit")` → `["error-codes",
  "frontend"]` for the README's example.
- `docsPathValue(m)` / `hooksPathValue(m)` — the two path keys with defaults
  (`check.go:16-23`, `render.go:187-194`).
- `loadManifest` (strict: missing manifest is an error — `render` needs it)
  vs `loadManifestOptional` (missing manifest → `(nil, nil)` — `check`,
  `ignores` must be zero-config). Both treat a malformed line as an error.
  (`manifest.go:35-55`).

---

## 5. `cinch check` — the pipeline

### 5.1 Orchestration (`check.go`)

`CmdCheck(msgFile)` (`check.go:56`):

1. `ResolveDocsRoot(".")` — reads the manifest, applies `paths.docs` or the
   `.docs` default, honors absolute values as-is (`check.go:30-40`).
2. Appends findings from each check in order: `links`, `rules`, then
   `coupling` (which carries `Findings` + `NoOp` + `Suppressed`),
   `generated` (Findings + NoOp), and `commit`. NoOp and suppressed messages
   go to **stderr** so silence on stdout is never mistaken for "checked and
   clean".
3. Sorts findings by Check, then File, then Line (`check.go:84-93`).
4. Prints one line per finding: `check level file:line: message`
   (`check.go:95-97`).
5. Exit `1` if any findings, else `0`.

### 5.2 The `Finding` model (`check.go:42-49`)

```go
type Finding struct {
    Check   string // "links" | "rules" | "coupling" | "generated" | "commit"
    Level   string // "error" | "block"
    File    string // repo-relative (or plain path)
    Line    int    // 1-based
    Message string
}
```

Two level semantics today: `error` (a correctness defect) and `block` (used by
`coupling` for the change-synchronization violation). Both currently make the
exit code `1` — the level is present as a classification seam for future
differentiation, but the observable behavior is identical today.

### 5.3 The "no-op, never a silent pass" contract

A check that *didn't run* must say so rather than read as clean — principle 6
("a check that cannot fail tells you nothing") sharpened into "and a check
that didn't even run must not collapse into a pass". `couplingResult` and
`generatedResult` both carry a `NoOp string`; `CmdCheck` prints it to stderr
(`check.go:69-71,78-80`). Examples:

- `coupling: not a git repository — check did not run`
- `coupling: working tree matches HEAD — nothing to compare, check did not run`
- `generated: cinch render has not run — nothing to verify`

The tests assert these messages — a missing no-op message is a test failure
(e.g. `generated_test.go:202-227`).

---

## 6. The five checks, in detail

### 6.1 `links` — structural (`links.go`)

**What it verifies:** every relative Markdown link under the docs root
resolves on disk.

**How it achieves it:** `checkLinks` (`links.go:19`) walks the docs root for
`*.md` files (a missing docs dir is a *quiet* pass — there's nothing to
scan, and this is intended: an un-initialized repo is not an error by
default). Per file, `checkLinksInFile` (`links.go:36`) scans line by line:

- skips fenced code blocks (tracked with `fenceRe`, `` ^\s*``` ``) — example
  prose must not be treated as real links;
- matches `` `mdLinkRe` `` = `\[[^\]]*\]\(([^)]+)\)`;
- **exempts** (`links.go:86`): empty targets, `scheme://` URLs, `#anchor`
  fragments, `mailto:` — these are correctly "unresolvable on disk" by design;
- strips any `#fragment` from an otherwise-relative target before resolving;
- resolves the target against the link's own directory
  (`filepath.Join(filepath.Dir(path), target)`) and `os.Stat`s it; failure →
  `links error` finding naming the broken target.

**Direction:** structural (principle 2) — a doc's own asserted
cross-reference must resolve; this is not validating a duplicate of a
machine-readable fact.

**Mutation fixture:** `links_test.go` — broken link fires, resolving link is
clean, fence/URL/anchor/mailto exempted, missing docs dir is quiet.

### 6.2 `rules` — lateral, bidirectional (`rules.go`)

**What it verifies:** the rule-ID ↔ `// cinch:rule <ID>` marker closure holds
*both ways* — every documented rule has an enforcing marker (or an explicit,
reasoned N/A declaration), and every marker resolves to a real rule ID.

**How it achieves it:** three regexes (`rules.go:13-18`):

- `ruleItemRe` `^\s*\d+\.\s+\*\*([A-Z0-9]+-[0-9]+)\*\*` — a rule is a
  numbered list item with a bolded ID (`1. **SIG-001** text`). Note the ID
  shape `<PREFIX>-<number>` with a trailing `\b` anchor so `SIG-0NN` can't
  truncate to a bogus `SIG-0`.
- `anyItemRe` `^\s*\d+\.\s` — any numbered item; used to bound rule text.
- `ignoreRe` `<!--\s*cinch:ignore\s*(?::\s*(.*?))?\s*-->` — the N/A
  declaration.
- `markerRe` `//\s*cinch:rule\s+([A-Z0-9]+-[0-9]+)\b` — the source marker.

`parseRuleItems(file, content)` (`rules.go:45`) parses **item-scoped text**:
the ID line through the last line before the next numbered item, then
whitespace-normalized (`strings.Join(strings.Fields(...)," ")`), so a
whitespace-only edit doesn't count as a text change. It's fence-aware. It's a
function of `(file, content)` rather than a path so the coupling check can
feed it a `git show HEAD:...` blob.

`scanRuleMarkers(repoRoot, docsDir)` (`rules.go:127`) walks **the whole repo
minus `.git`, `bin`, and the docs dir itself**, reads every file (up to 4 MiB
each), and collects `// cinch:rule` occurrences by ID — again fence-aware, so
a marker quoted in example prose is skipped.

`checkRules(docsDir, repoRoot)` (`rules.go:185`) then:

- rule without ignore and without marker → `no // cinch:rule marker (or
  cinch:ignore declaration)` (error);
- ignore with no reason → error (a *declaration* without a reason);
- ignore **and** marker both present → error (`pick one`) — a contradiction;
- marker with no matching rule ID → `does not resolve to any rule ID`
  (error).

`cinch ignores` (`rules.go:236`) is not a check: it enumerates every
declaration + reason and always exits 0, so the "declared N/A" inventory is
cheap to read periodically.

**Direction:** lateral (principle 4) — the closures are computed, never
hand-listed.

**Why the marker lives in the test file:** deletion is self-correcting —
remove the test and the marker goes with it, and the gap surfaces immediately.

**Mutation fixtures:** `rules_test.go` — missing marker, present marker,
dangling marker, ignore with/without reason, ignore+marker contradiction,
fenced marker ignored, truncated ID not matched, numbered *prose* rejected.

### 6.3 `coupling` — lateral, a transition check (`coupling.go`)

**What it verifies:** a rule's item-scoped *text* changed in the working tree
vs HEAD while the file holding that rule's marker did **not** change → a
`block` finding. Principle 1's own named pattern: *"this text changed and its
enforcing file did not."*

**How it achieves it** (`checkCoupling`, `coupling.go:31`):

1. **Preconditions**, each a distinct `NoOp`: not a git repo; no commits yet
   (`hasHead`); `paths.docs` outside the repo (an escape guard via
   `filepath.Rel`, `coupling.go:47-49`); `git diff HEAD` + untracked search
   failing; **working tree matches HEAD** (nothing to compare).
2. `gitChangedFiles` (`coupling.go:171`) = **union** of `git diff HEAD
   --name-only` and `git ls-files --others --exclude-standard`, deduplicated.
   The untracked half matters: a marker landing in a brand-new un-staged test
   file must count as "the marker file changed," or the check false-positives.
3. For each `.md` under the docs root, pull the **HEAD blob** via
   `git show HEAD:<rel>` and parse both HEAD and working-tree rule items per
   ID; a rule whose normalized text differs and whose marker file is not in
   the changed set is a finding.
4. If the rule ID appears in the `rule-reword:` escape hatch of `MSGFILE`
   (`coupling.go:122`, matched per line), the finding is **suppressed and
   logged** to stderr as `suppressed by rule-reword` — this is the escape for
   a legitimate reword where no test change is warranted. It's wired through
   `cinch check [MSGFILE]`, matching git's own `commit-msg` hook contract.

**Window and lifecycle:** working-tree-vs-HEAD makes it visible *during* a
change (pre-commit sees unstaged edits too — the README calls this the same
compromise a standard pre-commit gate makes) and a no-op right after commit.

**Mutation fixtures:** `coupling_test.go` — text changed/marker untouched
fires (mutating the *second* line proves item-scoping); both changed is
clean; untracked marker file counts; rule-reword suppresses; unrelated rule
unaffected; each no-op path announces itself.

### 6.4 `generated` — the tamper-evidence check (`generated.go`)

**What it verifies:** the render output on disk matches, byte for byte, what
`cinch render` would produce *right now* — missing file, altered bytes,
orphaned generated file, or a render that would fail outright.

**How it achieves it** (`checkGenerated`, `generated.go:22`):

1. No manifest → `NoOp` ("render has not run").
2. Call `renderAll` **in memory** (`generated.go:33`). If that fails (an
   undefined `{{key}}` in a template, say), that's a *decidable defect*, not
   an absence: it's reported as a finding against the manifest —
   "cinch render fails here, so no generated output can be verified."
   Rationale (comment at `generated.go:28-32`): a render failure that stayed
   loud on `cinch render` but silent on `cinch check` — the thing the hook
   runs — would silently disable verification of every generated file in the
   repo. That exact hole was the post-0.1.0 fix.
3. **The "render has not run" gate is the absence of *every* expected output,
   not the absence of one directory** (`generated.go:46-55`). Gating on the
   workflows dir alone meant `rm -rf .docs/workflows` skipped the outputs that
   live *outside* the docs root — the hook shims — so you could edit a shim to
   skip cinch without failing the check. Now: if exactly zero expected files
   exist → `NoOp`; if even one exists, full verification runs.
4. Compare each expected `dst` against `header(source, body, style) + body`
   — missing → "missing — run `cinch render` (or revert the edit)"; altered
   → "does not match a fresh render — ...".
5. **Orphans** (`generated.go:81-107`): for each file *not* in the expected
   set but present in a render directory, if it carries a generated header
   (`hasGeneratedHeader`, first two lines — catches both markdown line 1 and
   shell line 2 after the shebang), it's a finding: "orphaned generated file,
   no longer produced by cinch render — delete it (cinch render never removes
   files)." A hand-authored file dropped in a render directory is *not* an
   orphan (no generated header).

**Why this makes the whole system work:** it's the check that turns "a
re-render diff proves tampering" from a claim into a property. You cannot edit
a generated `.githooks/pre-commit` to skip cinch without this failing, and you
can't dodge it by deleting the docs directory.

**Mutation fixtures:** `generated_test.go` — fresh render clean; byte-flip
fires; delete a file fires; orphan fires; hand-authored orphan ignored;
unrenderable doc (no H1) is a *finding* not a no-op; deleted workflows dir
still verifies the shims; unrendered repo and no-manifest announce no-ops.

### 6.5 `commit` — opt-in convention (`commit.go`)

**What it verifies:** when **both** `MSGFILE` is given *and* the manifest has
`commit.pattern`, the message's **subject line** (first line, up to `\n`)
must match the pattern regex. Absent message file, absent manifest key, or
absent key → no behavior change, no finding. An invalid regexp in the manifest
is itself a finding. Nothing else is enforced — branch conventions, auto
version bumps, and `post-commit` are explicitly out of scope (app-specific;
plan "Deferred").

**Mutation fixtures:** `commit_test.go` — matching clean, non-matching fires,
absent key inert, no msg file inert, only subject line matched.

---

## 7. `cinch render` — what it generates and how

### 7.1 Substitution: `{{key}}`, literal only (`render.go:27-43`)

`varRe` = `\{\{([a-zA-Z0-9_.-]+)\}\}`. `substitute` replaces every token with
its manifest value. **No loops, no conditionals, no template engine.** An
undefined variable is an **error** (all missing keys reported at once via the
second return), never a blank — a silently-empty path in shipped prose would
be a referential-integrity hole in a referential-integrity tool.

### 7.2 Headers and the body-sha (`render.go:45-92`)

Every generated file carries a header stamped with its source and a
**16-hex-char SHA-256 of its body** — `bodyHash`. The marker string is the
comment-syntax-agnostic `headerPrefix = "generated by cinch"`.

Style-aware: a markdown file gets `<!-- generated by cinch from <src> — do
not edit; body-sha: ... -->` on line 1; a shell script *must* keep its
shebang as the literal first line, so the marker becomes a `#` comment on line
2. `hasGeneratedHeader` checks the first two lines, so orphan detection works
regardless of style.

Idempotency is structural: identical embedded source + identical manifest ⇒
identical body ⇒ identical hash ⇒ identical bytes. A hand-edit changes the
bytes but not (via a re-render) the expected bytes, so `cinch check` catches
it; a hand-edit that also tries to patch the header hash fails because the
hash is of the *body*, and re-render regenerates it anyway.

### 7.3 What `renderAll` produces (`render.go:107-155`)

For docs root `D` and hooks dir `H`:

| Output | Source | Notes |
|---|---|---|
| `D/workflows/<template>.md` | every embedded `docs/templates/*.md` | one-per-template; substitution applied; iterates the embed (adding a template needs no code change) |
| `D/workflows/doc-philosophy.md` | embedded `docs/philosophy.md` | copied **verbatim** — has no variables, needs no renderer |
| `H/pre-commit`, `H/commit-msg` | derived | hook shims, mode `0o755`, `exec cinch hook <event> "$@"` |

Files are sorted by destination for stability. Workflow and doc navigation
are **not** render outputs — see §10.

### 7.4 Title/trigger extraction (`title.go`)

`titleAndTrigger` finds the first `# ` line (a generated header above a body
is a comment, so it's skipped correctly) and takes the first sentence after
it, joining hand-wrapped physical lines until sentence punctuation, a blank
line, or a new block (`startsNewBlock`: heading, bullet, fence, table row,
quote, `1.` list). Shared by `cinch workflows` (title discarded, trigger
used) and `cinch docs` (title used, trigger discarded).

### 7.5 The hook shims (`buildHookShims`, `render.go:180-192`)

One thin, mode-`0755` shim per supported event:

```sh
#!/bin/sh
# generated by cinch from generated — do not edit; body-sha: <hash>

exec cinch hook <event> "$@"
```

**The body never encodes the dispatch table** — `hooks.<event>.*` is read at
runtime from the manifest by `cinch hook`. Consequence (deliberate): editing
the manifest takes effect *immediately*, with no re-render, and the shims stay
byte-stable across manifest changes. The `exec` runs a bare `cinch` — a PATH
lookup, deliberately, so a shim never hardcodes one machine's path. That
choice has a cost the README is explicit about: hooks only work once the
binary is **installed** (`make install` → `go install .`), not merely built —
otherwise every commit hits `cinch: not found` (exit 127) before a check runs.

### 7.6 `CmdRender` (`render.go:198-231`)

Requires a manifest (strict `loadManifest` — render has nothing to substitute
without it); calls `renderAll`; `MkdirAll` each directory; writes header+body
with the file's mode (default `0o644`); prints `rendered <dest>` per file.
**It never deletes a file** — removing obsolete outputs is the generated
check's orphan finding's job, not render's.

---

## 8. `cinch hook` — the dispatcher

The shims `exec cinch hook <event> "$@"`; `CmdHook` (`hook.go:14`) switches
on the event.

**`pre-commit`** (`cmdHookPreCommit`, `hook.go:26`):

1. staged set ← `git diff --cached --name-only --diff-filter=ACMR`;
2. `CmdCheck("")` in-process;
3. if a manifest exists, `dispatchHooks(root, m, "pre-commit", staged)`;
4. exit `1` if cinch check *or* any dispatched hook failed.

**`commit-msg`** (`cmdHookCommitMsg`, `hook.go:50`):

1. `CmdCheck(args[0])` — passes the message file, feeding both the `commit`
   check and coupling's `rule-reword:` escape hatch;
2. `dispatchHooks(root, m, "commit-msg", nil)` — `nil` is the sentinel that
   disables `when` filtering entirely (a commit message has no changed paths
   to scope against), so every registered entry always runs.

**`dispatchHooks`** (`hook.go:84`) iterates `m.Names("hooks.<event>")` in
declaration order:

- an entry with no `.run` value is skipped;
- `when := m.List(...)`; `hookWhenMatches(staged, when)` — path-**prefix**
  match (a staged path qualifies if it starts with one of the `when` values;
  empty `when` or `nil` staged always match), explicitly *not* globs;
- a non-matching entry is **announced** on stderr (`skipped ... (no staged
  path under ...)`) so "a hook run that did nothing is never silent";
- matching entries run via `runHookCommand` → `sh -c <command>` from the repo
  root with stdout/stderr streamed straight through;
- **every entry runs regardless of earlier failures** — failures accumulate,
  they don't fail-fast (`hook.go:83-102`); return `false` if any failed.

**The extension boundary, stated in the README:** cinch guarantees *dispatch*
— did the `when` prefixes match, did the registered command run, was its exit
code propagated — not your script's correctness. A green `cinch check` means
dispatch worked, not that the script is bug-free.

**Fixtures:** `hook_test.go` — when-match runs, no-match skips, empty when
always runs, commit-msg ignores when, exit code propagates, multi-failure
accumulates (second still runs), declaration order (`zzz` before `aaa`), and a
table-driven `hookWhenMatches`. The CLI test `TestHook_PreCommitDispatchesByWhen`
(`tests/cli_test.go:348`) proves the whole chain against a **real `git
commit`** — core.hooksPath, generated shim, registered `false` command under
`src/` → commit blocked; under `docs/` → commit succeeds.

---

## 9. `cinch init` — the entry point

`CmdInit(root)` (`init.go:29`), idempotent and non-destructive to anything
authored:

1. **Manifest**: keep if present; otherwise write a starter
   (`paths.docs = .docs`, `paths.hooks = .githooks`, and a commented
   `hooks.pre-commit.example` pair to teach the format).
2. **`mkdir <docs>/plans/` + `.gitkeep`** — the plans directory every
   workflow writes into.
3. **`CmdRender`** — workflows and hook shims (workflow/doc navigation is
   computed on demand — see §10 — not part of render output).
4. **Git repo** → `git config core.hooksPath <paths.hooks>` (activation). Not
   a git repo → shims are still generated, with an explicit stderr note that
   they weren't activated.
5. **AGENTS.md**: absent → write a minimal one carrying the two-line
   `Workflows: run `cinch workflows` ...` / `Docs: run `cinch docs` ...`
   commands; present → print the exact lines to add and touch nothing.
   **No check ever asserts AGENTS.md's contents** — "AGENTS.MD initialized"
   was the first proxy metric ever cut (principle 6), and it stays cut.

Idempotency is load-bearing: `init` twice → byte-identical tree (both unit
`TestCmdInit_TwiceIsByteIdentical` and CLI `TestInit_TwiceIsByteIdentical`).
The CLI-level `TestInit_ThenCheckExitsClean` is *the* release proof: `git
init` → `cinch init` → `cinch check` exits 0 with zero configuration.

---

## 10. `cinch workflows` / `cinch workflow NAME` / `cinch docs`

The point: a consumer's `AGENTS.md` lines are **commands, not paths**, so
neither can go stale under a customized `paths.docs`. Any agent that reads
AGENTS.md — not just one harness's proprietary skill system — can run the
command. Neither command's output is persisted anywhere; both compute from
whatever is on disk at call time, so there's nothing for `cinch check` to
verify and nothing that can drift.

- `workflowsTable` (`workflow.go`) lists `workflows/*.md` (`listWorkflowNames`,
  sorted), reads each file, and builds the `| Workflow | Trigger |` table via
  `titleAndTrigger`. `CmdWorkflows` (`workflow.go`) prints it, exiting `1` if
  there are no workflows — that must not read as "this project has zero
  workflows," but rather "render hasn't run."
- `CmdWorkflow` (`workflow.go`) resolves the docs root from the manifest,
  lists names (`listWorkflowNames`), rejects unknown names with exit `1`,
  and prints `<docs>/workflows/<name>.md` verbatim.
- `docList` (`docs.go`) walks the whole `paths.docs` tree, excludes `plans/`,
  reads each `.md` file, and pulls its H1 via `titleAndTrigger` — a doc with
  no H1 is skipped rather than erroring (this is a convenience listing, not
  a validated build step). `CmdDocs` (`docs.go`) prints `path — title` lines
  sorted by path.
- All three use `ResolveDocsRoot`, so a customized `paths.docs` is transparent
  (CLI test: `TestWorkflow_CustomDocsRootFromManifest`).

---

## 11. The shipped corpus (embedded content)

`go:embed` (`render.go:17-21`) carries everything into the static binary — no
runtime template resolution.

### `docs/philosophy.md` — copied verbatim

"Lean, Scalable Documentation": the **Admission Test** — *can an agent recover
this by reading the source?* If yes it doesn't get written (point at the
source); if no it's earned, and then *could a code change falsify it?* If yes
it's a business rule (belongs in `domain/<domain>.md` with a rule ID, where
the rule-ID closure and change-coupling guard it); if no, it's a decision/why
(prose). Plus six guiding principles: decisions-not-basics, code-is-the-
example, one-fact-one-place, scale-by-splitting, no-speculative-content,
only-quirks-as-examples.

### The eight workflow templates (`docs/templates/`)

| File | H1 | Trigger | Role |
|---|---|---|---|
| `fix-bug.md` | Bug Fix Workflow | produce a structured bug fix plan | Five phases (reproduce → root cause → domain-rule check → regression plan → fix tasks) + a **fast path**; three ⛔ checkpoints; TDD-mandatory regression test; fix plan lives in `plans/fix/<bug-slug>.md` then is **deleted** at completion |
| `execute-plan.md` | Plan Execution Workflow | execute tasks in an already-decomposed plan file | Per-task loop over `T<N>`; SESSION STOP boundaries; **bind-brackets-not-parens** commit conventions; Session Handoff template; compaction awareness |
| `plan-feature.md` | Feature Planning Workflow | produce a structured, atomic feature plan | Phase 1 domain rules (with interaction table), Phase 2 test plan, Phase 3 decomposes via the task primitive; persists `plans/<slug>.md` as `complete` |
| `maintain-domain.md` | Domain Rule Maintenance | add/edit/reorganize domain rules | Where rule IDs get minted (next-free `**PREFIX-0NN**`, never renumber), where `// cinch:rule <ID>` markers and `<!-- cinch:ignore: <reason> -->` declarations are placed; the doc that teaches the marker protocol |
| `audit-docs.md` | Documentation Audit | audit docs for philosophy violations, bloat, staleness | Retroactive admission test — **prune first** (deletion is the fix), then per-file audit against principles; stale refs are cinch's links/rules checks, the residue is judgment |
| `audit-domain.md` | Rule Coverage Audit | audit test coverage against domain rules | The **semantic half** that sits behind the decidable tripwire: derive the test→rule mapping from test *assertions* (not names), then diff against the marker closure; report TENSION/MISSING with verbatim evidence |
| `sync-docs.md` | Documentation Sync | update docs to reflect recent code changes | Routes changes by kind (regular / quirk / common issue) through a routing table that says "find it via `cinch docs`"; never touches domain rules (that's maintain-domain) |
| `task-primitive.md` | Task Decomposition Primitive | shared machinery for atomic, verifiable tasks | Single home for atomicity, context manifests, verification tiers (Tier 1 fail / Tier 2 pass / Tier 3 regression), the task template, pre-flight gate, completion ritual, plan-lifecycle (feature persists / fix deletes), and the compaction anchor |

Two structural features worth naming: the workflows **compose** — plan-feature
and fix-bug use the primitive (task-primitive) and route execution through
execute-plan, keeping shared machinery in one file — and they **indirect
through `cinch docs`** instead of hardcoding consumer paths, which is what
let the templates be genericized away from the original
`backend/`/`frontend/`/`api/`/`models/` assumption (the
`TestTemplates_NoStackSpecificPaths` guard keeps that from regressing).

---

## 12. Self-hosting: cinch on itself

This repo is its own consumer (README "This repo self-hosts, and its own
checks gate its own commits"):

- `cinch_manifest`: `paths.docs = .docs`, `paths.hooks = .githooks`, plus
  `hooks.pre-commit.build.run = make test` — so every commit runs *both*
  `cinch check` and the full test suite via the generated `pre-commit` shim.
- `.docs/` holds `PRINCIPLES.md` (spec), `plans/` (session plans), `workflows/`
  (rendered).
- `.githooks/` holds the two generated shims; `git config core.hooksPath =
  .githooks` is set.
- `AGENTS.md` carries the workflows and docs commands and the internal/
  layout map — and every committed doc in `.docs/workflows/` is itself a
  cinch render output, so editing one by hand trips the `generated` check.

The self-check has no findings exactly because of the textual-scan gotchas the
README documents: the marker-composition `marker()` helper in the test
helpers keeps literal `// cinch:rule` text out of the test files' source, and
fenced example markers are skipped by the fence-aware scanners.

---

## 13. How it's tested (principle 1 + 6 made concrete)

113 test functions across `internal/cinch/*_test.go` and `tests/cli_test.go`;
`make test` = build → `go vet ./...` → `go test ./...`.

**Mutation fixtures are the rule.** Every check ships both a passing fixture
and a small-mutation failing fixture — the proof of life that the check
catches its class (PRINCIPLES 1 and 6: "a checker without its mutation proof
is not done"). Confirmed failing-fixtures were hand-verified as non-vacuous by
reverting the fix under test (`generated`'s dark paths, the coupling
untracked-file fix).

**Fixture hygiene for the textual scans.** The shared helpers in
`links_test.go`:

```go
func writeFile(t *testing.T, path, content string) { ... }          // create + parent dirs
func findingsForCheck(findings []Finding, check string) []Finding   // filter by check
func marker(id string) string { return "// cinch:" + "rule " + id } // compose at runtime
```

The `marker(id)` helper concatenates `"// cinch:" + "rule " + id` so no test
file's own *source* carries a scannable literal `// cinch:rule` — otherwise
cinch's rules check would trip over its own fixtures during the self-check.
`seedCoupledRule` (coupling_test.go:45) commits a doc with two rules and two
marker test files so one can be mutated while the other provably stays
unaffected.

**Test layers:**

- Unit tests in `internal/cinch` call the check functions directly against
  `t.TempDir()` scratch trees (some real git repos for coupling/hook/init).
- CLI tests in `tests/` exec the **built** `./bin/cinch` with
  `cmd.Dir` set to a temp dir; `binPath` returns an absolute path (exec
  resolves relative paths against `cmd.Dir`).
- `gitEnv`/`runGit` (tests/cli_test.go:323-342) bypass real git config,
  set a throwaway identity, and **prepend the bin dir to PATH** so a generated
  shim's `exec cinch hook ...` resolves to the freshly-built binary — this is
  what makes the real-`git commit` dispatch test possible.
- Idempotency fixtures at both layers: `TestRenderAll_IdempotentReRender`,
  `TestRender_EndToEndAndIdempotent`, `TestInit_TwiceIsByteIdentical` (CLI) /
  `TestCmdInit_TwiceIsByteIdentical` (unit), plus an end-to-end scratch-repo
  sha256 comparison at verification time (plan §Verification.3).

**The several load-bearing proofs**, called out as such in comments and plan:

- `TestInit_ThenCheckExitsClean` — init must leave a tree check passes with
  no configuration; "If this holds, the release works."
- `TestHook_PreCommitDispatchesByWhen` — the shim + hooksPath + `when`
  matching wire together against a real commit.
- `TestCheckGenerated_DeletedWorkflowsDirStillVerifiesShims` — the
  "generated can't go dark" fixture that confirms the pre-fix code silently
  passed when the workflows directory alone was deleted.

---

## 14. The six principles, mapped onto the code

| Principle | Embodiment |
|---|---|
| 1. Deterministic checkers over model audits | Every check is a pure-ish, script-decidable procedure over files/git; the "semantic residue" (the audit-domain workflow's TENSION/MISSING judgment, a consumer's registered scripts) is explicitly *demoted* to sit behind a decidable tripwire; each check carries a mutation fixture. |
| 2. The direction rule | `links` = structural, `rules` = lateral, `coupling` = lateral, `generated` = a verification of the tool's own *build* output (never authored → nothing to go stale by hand). |
| 3. Derivability gate on docs; holdability gate on the system | Workflow and doc navigation are computed on demand, not even persisted — the strongest form of "can't go stale" is to never write it; the corpus philosophy deletes anything derivable from source; the suite stays enumerable (one person can hold it — the README/plan describe the line). |
| 4. Rule → test markers, in the test file | `// cinch:rule <ID>` above the enforcing test; closure computed by `checkRules`, never a registry; deletion is self-correcting. |
| 5. Bind project specifics in one place | `cinch_manifest` binds values; templates use only `paths.docs`; shape variance is answered by absence (`when` list vs globs; keys present or absent change behavior) — with the acknowledged `hooks.*`-is-a-small-table tension flagged in the plan. |
| 6. Proxy metrics are worthless | No "AGENTS.md initialized" check; every warn/no-op has a stated green state or is a deliberate no-op-with-reason (never a falsely-green pass); no accepted baselines; every check has a demonstrable failing state (its mutation fixture). |

---

## 15. Build, install, toolchain

- `make build` → `go build -o bin/cinch .` (`.gitignore`'d).
- `make install` → `go install .` (puts `cinch` on PATH via GOBIN) — required
  for activated hooks, since the shims exec a bare `cinch`.
- `make test` → build + `go vet ./...` + `go test ./...`.
- Module: `module cinch`, Go 1.22, **zero third-party dependencies** —
  everything, including the docs corpus, ships in the binary via `go:embed`.

---

## 16. Known limitations and deliberate trade-offs

- **The marker scan is textual, not language-aware** (README "Known
  limitation", `Known limitation`): a repo that quotes marker syntax as a
  string literal *outside* a fenced block is still scanned as if it were a
  real marker. No file-name exclusion is built for it — that would be exactly
  the ad-hoc config the principles avoid. (Fenced markers are skipped; the
  test files use the runtime-composition helper so their own source stays
  clean.)
- **Version skew:** upgrading the cinch binary can redden every consumer's
  `generated` check until `cinch render` is re-run — no version pin or soft
  warn; the green state is reachable and the fix is exact, which is what
  matters (a fuzzy warn was considered and rejected as a proxy metric).
- **`pre-commit` sees unstaged edits too** (coupling's window is working-tree
  vs HEAD) — the same compromise a standard pre-commit gate makes.
- **`render` never deletes files**; obsolete outputs are flagged by the
  orphan half of the `generated` check, and the consumer deletes them.
- **Known pre-existing bug (from the plan's Results, not yet fixed):**
  `CmdRender` writes via `filepath.Join(root, f.Dest)`; an *absolute*
  `paths.docs` gets its output nested under `root` instead of at the absolute
  location the README claims. (`ResolveDocsRoot` itself handles absolute
  values correctly — the gap is the write path specifically.)
- **Extension boundary:** cinch guarantees hook *dispatch*, not consumer-script
  correctness.

---

## 17. History and trajectory

`git log` tells the story of 0.1.0 shippied and then tightened:

- `9aabe75` relocate manifest to root `cinch_manifest` (flag 7)
- `437309e` make cinch clean against itself (the prerequisite for
  self-enforcement)
- `bc99e04` coupling correctness for hook use (untracked files, outside-repo
  no-op)
- `4d11c77` manifest accessors (`List`/`Names`, declaration order)
- `3504b2b` genericize shipped templates + the stack-path regression guard
- `262b9a2` doc map as a render output
- `e052c1a` the `generated` check
- `3cd7f4c` `cinch hook` + generated shims + `commit.pattern`
- `369590f` `cinch init`
- `069193a` `cinch workflows` / `cinch workflow NAME`
- `72ecc0c` cinch self-hosts (its own manifest/render/hooks commit)
- `07b75fb`/`a4eca1e` docs (plan results, README overview)
- `ea3e342` **`fix: the generated check can no longer go dark`** —
  post-0.1.0 hardening: a failing render is a *finding* (via `renderFault`),
  the "render hasn't run" gate is the absence of every expected output (not
  one directory), and `make install` closes the un-runnable-hooks gap.

Next (the README's "Stage 3"): run it against a second, real, straining
consumer repo — the self-host is the proof, not the destination.

## Appendix A — per-file map

| File | Responsibility |
|---|---|
| `main.go` | usage text; arg-shape validation; dispatch to `Cmd*`; exit-code translation |
| `internal/cinch/check.go` | `Finding`, `docsPathValue`, `ResolveDocsRoot`, `CmdCheck` |
| `internal/cinch/links.go` | the `links` check (fence-aware link resolution) |
| `internal/cinch/rules.go` | rule-item parser, marker scanner, `checkRules`, `CmdIgnores` |
| `internal/cinch/coupling.go` | the `coupling` transition check, `rule-reword` escape, git helpers |
| `internal/cinch/generated.go` | byte-compare against fresh in-memory render; orphan scan |
| `internal/cinch/commit.go` | subject-vs-`commit.pattern` check |
| `internal/cinch/render.go` | substitution, headers/body-sha, `renderAll`, hook shims, `CmdRender` |
| `internal/cinch/title.go` | `titleAndTrigger`, `startsNewBlock`, `endsSentence` — shared H1 extraction |
| `internal/cinch/manifest.go` | `cinch_manifest` parser, `List`, `Names`, declaration order |
| `internal/cinch/hook.go` | `CmdHook`, staged-set computation, `when` prefix matching, `sh -c` execution |
| `internal/cinch/init.go` | `CmdInit` scaffold |
| `internal/cinch/workflow.go` | `CmdWorkflows` / `CmdWorkflow`, `listWorkflowNames`, `workflowsTable` — computed on demand |
| `internal/cinch/docs.go` | `CmdDocs`, `docList` — computed on demand |
| `internal/cinch/docs/` | `philosophy.md` + 8 templates, embedded |
| `tests/cli_test.go` | binary-level end-to-end tests |
