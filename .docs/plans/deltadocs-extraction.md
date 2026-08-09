# Plan: extract five patterns from `project_deltadocs` into cinch

Status: proposed — not started

## Context

On 2026-08-09, `cinch` was compared against `project_deltadocs`, a repo that
adopted cinch (Aug 2026, "adopt rebuilt cinch — Stage 2 render") and then
built a layer of hand-maintained tooling around it to cover gaps cinch
doesn't fill: per-harness skill shims, a `SessionStart` context-injection
hook, a full-tree doc index with a staleness gate, a richer hand-authored
`manifest.yml` alongside cinch's own manifest, and a role-scoped Bash
sandbox ("seams") for an auditor subagent. Two items from that comparison
(generated skill shims, full-tree doc index) are already covered by
`workflow-cli.md` and are natural `render`/`check` extensions respectively.

This plan covers the remaining five, ranked by leverage vs. effort, in the
order they should land — #2 is a prerequisite for #3 and #4:

1. Richer manifest values (list support) — no new file format, just what a
   value can hold.
2. `owns:` front-matter + a structural check.
3. Manifest-driven "seams" as a generated hook.
4. SessionStart context-injection as a generated template.
5. Commit-message format check (scoped-down version of deltadocs' git-hook
   pipeline — auto-versioning is deliberately left out, see that section).

Each keeps cinch's existing constraints: no new dependencies, no template
engine growth (still `{{key}}` substitution only), generated files are
header-stamped and idempotent, and every new check ships with a mutation
fixture (per `.docs/PRINCIPLES.md` principle 1).

---

## 1. Richer manifest values (list support)

### Problem
`Manifest.Vars` (`internal/cinch/manifest.go:24`) is `map[string]string` —
one value per key. deltadocs' `seams.auditor.allow` (a list of allowed Bash
commands) and `taxonomy.layers` (a list of layer names) can't be expressed
in cinch's own manifest today, which is why deltadocs ended up hand
-maintaining a second, richer `manifest.yml`.

### Design decision
Don't add a second file format or nested structure — that's exactly the
"shape growth" principle 5 warns against. Instead, let a single manifest
*value* be interpreted as a comma-separated list by consumers that need
one. The stored format doesn't change (`Vars map[string]string`, still one
line, still `dotted.key = value`); only a new accessor is added:

```go
// internal/cinch/manifest.go

// List splits key's value on commas, trimming surrounding whitespace and
// dropping empty items. Consumers that need a list of things (seam
// allow-rules, taxonomy layers) use this instead of a second, structured
// value shape. Missing key returns nil.
func (m *Manifest) List(key string) []string
```

`{{key}}` substitution in `render.go` is untouched — `substitute` still
treats every value as an opaque string; only new, non-template consumers
(the seam-guard generator in §3) call `.List()`.

### Tests
`internal/cinch/manifest_test.go`:
- `TestManifest_List_SplitsAndTrims` — `"a, b ,c"` → `["a","b","c"]`.
- `TestManifest_List_MissingKeyReturnsNil`.
- `TestManifest_List_EmptyItemsDropped` — `"a,,b"` → `["a","b"]`.

---

## 2. `owns:` front-matter + a structural check

### Problem
deltadocs' domain docs declare `owns: [path/to/file.go, path/to/other.go]`
in YAML front-matter, tying a doc to the source it governs — but nothing
checks that those paths still exist. It's an authored claim with no
verification, the exact gap cinch's `links` check already closes for
markdown links.

### Design decision
A new check, same category as `links` (structural: "a referent exists"),
not a full YAML parser — consistent with manifest.go's own "no dependency,
hand-written parser" precedent. Only the two lines cinch needs:

- A file's front-matter is the block between a `---` line at byte 0 and the
  next `---` line.
- Inside it, an `owns:` line followed by a bracketed, comma-separated list:
  `owns: [backend/handler.go, frontend/Tab.svelte]`.

### Implementation

New file `internal/cinch/owns.go`:

```go
// parseOwns extracts the owns: list from a file's front-matter, if any.
// Returns (nil, false) when the file has no front-matter or no owns: key —
// not an error; owns: is optional per file.
func parseOwns(body string) (paths []string, ok bool)

// checkOwns walks docsRoot, and for every file with an owns: front-matter
// entry, verifies each listed path exists relative to repoRoot. A missing
// path is an error-level finding — same severity as a broken link, since
// it's the same failure mode (a stale referent).
func checkOwns(docsRoot, repoRoot string) []Finding
```

Wire into `CmdCheck` (`internal/cinch/check.go:64`) alongside
`checkLinks`/`checkRules`, `Check: "owns"`.

### Tests
`internal/cinch/owns_test.go`, mirroring `links_test.go`'s structure:
- `TestCheckOwns_ExistingPathsPass`.
- `TestCheckOwns_MissingPathFires` — the mutation-fixture pair
  `.docs/PRINCIPLES.md` principle 1 requires: one fixture where the check
  passes, one single-character mutation (delete the owned file) where it
  must fail.
- `TestParseOwns_NoFrontMatterReturnsNotOK`.
- `TestParseOwns_FrontMatterWithoutOwnsReturnsNotOK`.

---

## 3. Manifest-driven "seams" as a generated hook

### Problem
deltadocs' `auditor-bash-guard.py` is a hand-written `PreToolUse` hook that
reads `seams.auditor.allow` from its separate `manifest.yml` at runtime and
denies any Bash command not an exact allow-list match. It's a real,
working access-control mechanism with no cinch equivalent — every consumer
repo that wants a sandboxed subagent has to write this script itself.

### Design decision
Once §1 lands, a manifest can express `seams.auditor.allow = git status,
git diff, make test, ...`. `cinch render` gains a second output *class*
alongside docs: harness artifacts, written outside `paths.docs` (this
already breaks the "everything lives under paths.docs" assumption on
purpose — a hook has to live where Claude Code's settings expect it,
`.claude/hooks/`). Use **absence-based selection** (principle 5): if no
`seams.*` keys exist in the manifest, nothing is generated — this stays
opt-in, not a new default surface for consumers that don't use subagent
sandboxing.

`renderAll` (`render.go:72`) needs a second loop, keyed off manifest keys
rather than embedded template files: for each distinct `<role>` found in
`seams.<role>.allow`, embed a static guard-script template
(`docs/templates/seam-guard.py.tmpl` or similar, Python to match Claude
Code's hook contract) with a `{{seam.allow}}` token substituted with the
role's list, JSON-encoded as a Python literal. Output to
`.claude/hooks/<role>-guard.py`, still header-stamped for idempotency.

Wiring the hook into `.claude/settings.json`'s `PreToolUse` array is
**not** auto-generated — cinch has no business rewriting a file it doesn't
own and can't safely merge. README documents the one-line addition the
same way the `AGENTS.md` Workflows line is documented (§ of README.md
"Workflow index instead of per-harness skills").

### Tests
- Unit: given `seams.auditor.allow = git status, git diff`, the generated
  script's embedded allow-list matches exactly those two commands.
- CLI: `TestRender_SeamGuardGeneratedWhenSeamsKeyPresent`,
  `TestRender_NoSeamGuardWhenNoSeamsKey` (absence-based selection,
  regression-tested explicitly since it's easy to accidentally always-emit).

---

## 4. SessionStart context-injection as a generated template

### Problem
deltadocs' `inject-agents.sh` prints `AGENTS.md` plus live orientation
(current branch, active `.docs/plans/*.md` status) into context at session
start, ending with a verifiable marker line
(`[session-start hook ✓ AGENTS.md + repo orientation injected]`) printed by
the script itself — not the model — so the injection can't be faked by an
agent that skipped reading the docs. This is a strong, reusable pattern
with no cinch equivalent.

### Design decision
Ship it as a static generated artifact, not manifest-templated (it needs
no consumer-specific values — it just cats `AGENTS.md` and runs `git`):
`docs/templates/session-start.sh` embedded, rendered verbatim (like
`doc-philosophy.md` is copied verbatim today) to
`.claude/hooks/session-start.sh`, executable bit set. Same settings.json
caveat as §3 — the hook script is generated, its registration in
`.claude/settings.json` is a documented one-time manual step.

Scope it to reading `AGENTS.md` (fixed, root-relative — same assumption
cinch's README already makes) and listing any files under
`<paths.docs>/plans/*.md` — reuse `docsPathValue`/`ResolveDocsRoot`'s
resolved root so the script doesn't hardcode `.docs` independently of
`paths.docs` (the exact staleness bug `workflow-cli.md` exists to fix,
avoided here by generating the path into the script rather than
hardcoding it by hand).

### Tests
- CLI: after `cinch render` with `paths.docs = mydocs`, the generated
  `.claude/hooks/session-start.sh` references `mydocs/plans`, not `.docs/plans`
  (mirrors `TestWorkflow_CustomDocsRootFromManifest`'s load-bearing intent).
- Shell-level smoke test (`tests/cli_test.go` can shell out): run the
  generated script in a scratch repo, assert it prints `AGENTS.md`'s
  content and the verifiable marker line.

---

## 5. Commit-message format check (scoped down from deltadocs' git-hook pipeline)

### Problem
deltadocs hand-built a full `.githooks/{pre-commit,commit-msg,post-commit}`
pipeline: commit/branch convention enforcement plus automatic per-service
semantic-version bumping on merge.

### Design decision — deliberately narrow the scope
Auto-versioning is app/service-specific (which services exist, what "merge"
means, semver policy) — out of scope for a docs-integrity tool, and adding
it would be exactly the kind of speculative, project-specific feature
`.docs/PRINCIPLES.md` warns against building into a generically-consumed
binary. **Propose only the part that's genuinely doc-adjacent and already
has a natural home**: `cinch check` already reads an optional `MSGFILE`
(`check.go:56`) for coupling's `rule-reword:` escape hatch — the same
message is a natural place to validate a commit-message *format*, if the
consumer opts in.

Add an optional manifest key `commit.pattern` (a single regex string, e.g.
`^\[[a-z-]+\] .+`), consulted only when `MSGFILE` is given and the key is
present (absence-based: no key, no check — zero-config consumers see no
new behavior). On mismatch, emit a `Finding{Check: "commit", Level:
"error"}` against the message file itself.

Explicitly **not** proposed: branch-name conventions, auto version
bumping, or a `post-commit` hook — those stay app-specific, and a consumer
that wants them keeps hand-writing that piece the way deltadocs does today.

### Tests
- `internal/cinch/commit_test.go`: pattern present + message matches → no
  finding; pattern present + message doesn't match → finding; no
  `commit.pattern` key → no finding regardless of message content
  (zero-config default preserved).
- CLI: `cinch check badmsg.txt` in a repo whose manifest sets
  `commit.pattern`, asserting exit 1 and the finding text.

---

## Verification

- `make test` after each numbered section lands independently (these are
  five separable changes — land and verify one at a time, not as a single
  commit).
- For §2 and §5 (new checks): confirm the mutation-fixture pair required by
  `.docs/PRINCIPLES.md` principle 1 — a passing fixture and a one-change
  failing fixture — exists in the test file, same as `links_test.go` /
  `rules_test.go` already do.
- For §3 and §4 (new generated artifacts): confirm `cinch render` run
  twice produces byte-identical output (idempotency, same property
  `TestRender_EndToEndAndIdempotent` checks for existing outputs), and
  confirm absence-based selection — no `seams.*` key means no guard script
  is written, no flag needed to suppress it.
- README.md: document the two new opt-in manifest keys (`seams.<role>.allow`,
  `commit.pattern`) alongside the existing `paths.docs` documentation, and
  the two new one-time manual settings.json wiring steps (§3, §4) alongside
  the existing AGENTS.md Workflows line in "Workflow index instead of
  per-harness skills".
