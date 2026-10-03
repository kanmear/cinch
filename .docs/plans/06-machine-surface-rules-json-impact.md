# 6. The machine surface: `rules --json` + `owns:` → `cinch impact`

**Status: shipped in `f7f94ac`.**

**Tier 2 — make green mean something. Cost estimate: days.**

## Provenance

**Both.** Audit recommendation #5 ("the inventory as data" — `cinch rules --json`) and
`FINDINGS.md` §4.1 ("read the frontmatter you already write — `cinch impact`") independently
arrived at the same underlying gap: cinch's corpus carries structured metadata — rule IDs,
`owns:`/`rule_prefix:`-style frontmatter — that nothing in the tool ever parses or exposes.
The roadmap calls this out explicitly: "one parser serves both."

## What exists and what's missing, grounded in code

**What exists:** `parseRuleItems` (`internal/cinch/rules.go:54-95`) already extracts
everything needed for an inventory — `ruleItem{id, file, line, text, hasIgnore,
ignoreReason, ignoreLine}` — and `scanRuleMarkers` (`rules.go:193-249`) already builds the
inverse map, `id → []markerLoc{file, line}`, across every git-tracked file outside
`docsRoot`, using a bounded worker pool. `CmdIgnores` (`rules.go:333-357`) already
demonstrates the pattern this item generalizes: scan, filter, print — just human-formatted
instead of structured, and limited to the ignore subset rather than the full inventory.

**What's missing:**

1. **No JSON emission.** `CmdIgnores` prints with `fmt.Printf`. There is no
   `--json` flag anywhere in `main.go`'s command dispatch, and no serialization of `ruleItem`
   or the marker map.
2. **No frontmatter parsing at all.** A repo-wide search for `owns:` or `frontmatter` across
   `internal/cinch/*.go` (excluding tests) returns nothing. Whatever `owns:`/`rule_prefix:`
   keys a corpus author writes in a doc's YAML frontmatter (a convention documented and used
   in `project_deltadocs`'s `.agent/`, per the benchmark writeup) are inert — cinch has no code
   path that reads them. `manifest.go`'s YAML parsing (`parseManifestBytes`,
   `flattenMapping`) is scoped to `cinch.yml` specifically, not to arbitrary per-doc
   frontmatter, so this is new parsing, not a reuse of the manifest loader (though the same
   `gopkg.in/yaml.v3` dependency applies).
3. **No diff-to-rules mapping, no pre-commit advisory.** `preCommitChecks`
   (`check.go:35-37`) runs `links`, `rules`, `retirement`, `generated`, `core` — all
   pass/fail checks. Nothing surfaces "this diff touches files that `owns:` maps to
   RULE-IDs X, Y, Z — did you mean to update them?" at commit time, which is the one moment
   an agent or author is guaranteed to be looking at output.

## Proposed shape, as three layered pieces

### 6a. `cinch rules --json`

Emit the existing `[]ruleItem` (plus the marker map, or a merged view — decide based on what
consumers actually need: probably `{id, file, line, text, markers: [{file, line}, ...],
hasIgnore, ignoreReason}` per rule) as JSON on a new `--json` flag to the existing `rules`-
adjacent surface, or a new `cinch rules` command if one doesn't cleanly exist yet (check
`main.go`'s switch — today `ignores` is the only rules-scoped subcommand; `rules --json`
either extends that surface or introduces a sibling). This alone replaces the audit's
observation that its own workflow had to derive the rule inventory by hand — a single `jq`-
able command does it mechanically.

### 6b. Parse `owns:` frontmatter

Add a small YAML-frontmatter extractor (delimited `---\n...\n---` block at the top of a doc,
parsed with the same `yaml.v3` library already a dependency) that reads `owns:` — a list of
path globs or explicit paths the doc's author is asserting the doc is authoritative over — and
`rule_prefix:` if that's a separate declared convention (confirm the exact key set against
`project_deltadocs`'s actual usage before finalizing the schema, since this item's job is to
finally *read* a convention that grew organically without cinch's involvement). Surface it
through the same `--json` output as 6a, per-doc.

### 6c. `cinch impact <files...>` (or diff-scoped, no args = staged diff)

Given a set of changed files (default: `git diff --cached --name-only`, matching the shape
`checkCommit`/hook plumbing already reads git state), cross-reference against the `owns:` map
from 6b and the marker map from `scanRuleMarkers`, and print which rule IDs are plausibly
affected — by direct marker presence in a changed file, or by `owns:` glob match. This is the
"advisory at the one moment an agent is guaranteed to be listening" piece: wire it into the
pre-commit hook path (`hook.go`/`hooks.go`, alongside the existing `preCommitChecks` launch
list in `check.go:35-37`) as **advisory output, not a blocking finding** — it should never
fail the commit, only print "this diff touches SIG-019, SIG-020 — confirm the docs still
match" so the agent sees it inline.

## Why this is tier 2, and why it's ordered after items 4 and 5

`cinch impact` is only trustworthy once green means something: if bindings are frequently
false (item 4 unmeasured) or the corpus it's cross-referencing isn't reliably navigable (item
5 unshipped), an advisory built on top of `owns:`/markers would be advising against unreliable
data. This item is also explicitly the mechanism the roadmap frames as **means to item 10**
(`kind:` normative/descriptive) — a machine-readable inventory is a precondition for
classifying 76+ rules by hand or by tooling; you can't build the admission-test-as-mechanism
without first being able to enumerate and query what exists.

## Evidence this is worth building now rather than later

The benchmark's T2 task (extending a stranding confirm-gate to a new endpoint, spanning
roles/permissions/signatures/frontend/i18n) measured this gap directly: **zero of six control
runs recorded a new rule while shipping a feature that clearly warranted one**, and even the
documented (`with cinch`) arm missed binding the marker in one of two runs. The failure isn't
capability — per `FINDINGS.md`, agents invented the correct-looking rule ID unprompted when
they thought to write one at all — it's **timing**: nothing prompts the thought at the moment
it would land. A pre-commit advisory is a direct fix for a timing gap, which is a different
(and cheaper) intervention than trying to improve agents' judgment about when documentation
matters.

## Verification

- 6a: `cinch rules --json` output round-trips through a JSON schema/struct matching
  `ruleItem`'s fields; spot-check against `project_deltadocs`'s known 76-rule inventory for
  count parity with the existing `rules: ok (N rules, ...)` detail line
  (`rulesCheckResult`, `rules.go:264-275`).
- 6b: fixture docs with and without `owns:` frontmatter; confirm the parser extracts the list
  correctly and doesn't choke on docs with no frontmatter block at all (the common case today).
- 6c: a fixture diff touching a file matched by one doc's `owns:` glob and containing one
  known marker; confirm `cinch impact` names both the glob-matched and marker-matched rule
  IDs, and confirm it's non-blocking (exit 0) even when it has something to say.
- End-to-end: re-run (a slice of) the benchmark's T2 task with the pre-commit advisory wired
  in, and check whether the "zero of six" finding improves — this is the item's own
  acceptance test, since it exists specifically to close that gap.
