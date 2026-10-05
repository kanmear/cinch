# Build order — overview

Source: the combined roadmap (external field-report audit of cinch @ `c98c729`, twelve
findings F1–F12, plus an A/B benchmark and layer ablation — 84+ agent runs measuring
`.agent/`-style corpora against a control on a real consumer, `project_deltadocs`).
Published artifact: `Cinch Build Order` (https://claude.ai/code/artifact/481a31ab-987e-4b3d-8c73-ad6eaab4023a).
Underlying data and reasoning: `~/code/cinch_bench/FINDINGS.md` and `~/code/cinch_bench/report.html`.

Each numbered file below is one build-order item, expanded into something someone could pick
up and implement without re-deriving the "why" — current behavior with file:line references
into this repo, the proposed change, the evidence that justifies doing it now versus later,
and how to verify it landed. Ordered by what unblocks what, not by severity — everything past
tier three is small in isolation but compounds.

| # | Item | Tier | Status |
|---|---|---|---|
| 1 | [Strip build metadata before parsing semver](01-strip-build-metadata-semver.md) | 1 — trust the checker | **Shipped** (`a35cbe1`) — kept for provenance |
| 2 | [`cinch upgrade`](02-cinch-upgrade.md) | 1 — trust the checker | **Shipped** (`cf3edcf`) |
| 3 | [Close rule-grammar holes, then seed defects](03-rule-grammar-holes-and-seed-defects.md) | 1 — trust the checker | Part A and the seeded battery shipped (`284bf40`) |
| 4 | [Measure binding strength before designing for it](04-measure-binding-strength.md) | 2 — make green mean something | Open |
| 5 | [Navigability as one check class](05-navigability-check-class.md) | 2 — make green mean something | H1 half shipped (`2925bf4`); the reachability half was dropped — see the item's resolution and the comment in `TestSeededDefectLinksTargetUnreachableFromRoot` |
| 6 | [The machine surface: `rules --json` + `owns:` → `cinch impact`](06-machine-surface-rules-json-impact.md) | 2 — make green mean something | **Shipped** (`f7f94ac`) |
| 7 | [Workflow prose value and F1's scope](07-workflow-prose-value-and-f1-scope.md) | 3 — decide before investing | Cheap half effectively done by the workflow-template trim (`09ef586`); expensive half still open |
| 8 | [`paths.exclude`, then dogfood](08-paths-exclude-and-dogfood.md) | 3 — decide before investing | **Shipped** (`9830825`) |
| 9 | [Calibrate every heuristic against real history](09-calibrate-heuristics-against-history.md) | 3 — decide before investing | Open |
| 10 | [`kind:` — normative versus descriptive](10-kind-normative-vs-descriptive.md) | 3 — decide before investing | Open |
| 11 | [`rules.roots` and hook-baseline overrides](11-sibling-code-roots.md) | 1 — trust the checker | **Shipped** (`7e5c70d`) — kept for provenance |
| 12 | [Skip directory entries (submodule gitlinks) in the primary marker scan](12-skip-directory-entries-marker-scan.md) | 1 — trust the checker | **Shipped** (`92b99d4`) — kept for provenance |

## Reading order vs. build order

Read 1→10 once for the full argument; *build* in the tier order, since each tier is a
precondition for the next tier's evidence being trustworthy:

- **Tier 1** makes `cinch check` red only for real reasons. Nothing above this tier is worth
  measuring, because right now green and red are both partly noise (build-metadata parse
  failures, four-release-stale consumers, near-miss IDs that don't even surface).
- **Tier 2** makes green mean something once it's trustworthy — binding strength, doc
  reachability, and a machine-readable rule inventory are the difference between "the checker
  passed" and "the checker verified the thing the corpus claims."
- **Tier 3** are the items where doing the work before deciding would be premature investment.
  Each one is a measurement or a design decision that changes the shape of a later
  implementation, not a straightforward bug fix.

## Provenance tags

Each item file states which of the two sources justifies it:

- **Audit (F#)** — found by static reading of the tool; a code-level defect or gap.
- **Benchmark (measured)** — found or confirmed by running agents against the corpus and
  scoring the result; an empirical claim about value or failure mode.
- **Both** — the audit predicted it and the benchmark independently measured its consequence,
  which is the strongest form of evidence available here since the two methods fail
  differently (the audit sees code the benchmark can't reach; the benchmark sees behavior the
  audit can only predict).

## What's deliberately not on this list

Carried from the roadmap's own "what both reads argue against" table — not restated per-item
below, but binding on all ten:

- No MCP server or agent-facing retrieval tool. Agents needed no tool to read docs — grep and
  a pointer sufficed. The useful agent surface is workflow-timed (item 6's pre-commit
  advisory), not retrieval-shaped.
- No more workflow prose, and no vaguer prose in the name of portability, until item 7 is
  resolved. 1,325 template lines substitute exactly one variable today; the failure mode to
  avoid is more tokens buying more ambiguity in a tool whose thesis is narrow, high-signal
  context.
- No new enforcement layered on the corpus as it stands, before item 10. Most rules restate
  what the code already says; machinery over a mostly-redundant corpus mostly generates work.
- Confidence is not distributed evenly across cinch's own checks. `rules` is exercised daily
  and is solid. The plan/workflow ceremony has been exercised once, end to end (item 7). The
  `retirement` check has never been enabled by anyone who wasn't running this benchmark.

## Cut, and why

Three surfaces were removed after shipping. Recorded here because two of the three
were cut with no rationale in the commit, and the `cinch context` question came back
months later and had to be re-derived from the benchmark data from scratch. Each entry
names the commit that holds the code, so restoring is a `git show` away if the stated
condition is ever met.

- **`cinch context`** — the session-start report: branch, plans under `plans/` with
  their `Status:` lines, the workflow trigger table, staged paths. Added `276aedd`,
  status vocabulary added `e50064f`, cut `ec3e8cc` (empty commit body). Three of its
  four sections are covered by `cinch workflows`, `git branch`, and `git status`; the
  fourth needs `manifest.go`'s `List()` restored to order a handful of files. It also
  degrades here, since `58e7457` dropped the self-host files and none of the plans in
  this directory carry a `Status:` line. **Reopen only if** item 8 lands and cinch
  self-hosts again *and* a consumer's `plans/` grows past roughly a dozen files with
  subdirectory grouping — until then `ls` is enough. The first half is now met
  (`9830825`: cinch self-hosts via its own `cinch.yml`, with `paths.exclude` and
  `cinch check` in CI); the `plans/` half is not. The condition names a consumer's
  `plans/`, and this repo's own `.docs/plans/` is 13 flat files with no subdirectory
  grouping. Note that
  `05-navigability-check-class.md`'s resolution still names "eventually `cinch
  context`" as a canonical entry point; that is an aside written before this decision,
  not a commitment.
- **`cinch move-docs NEW-PATH`** — moved the docs root, rewrote `paths.docs`, and
  re-rendered as one atomic operation, so the manifest could never point at a path the
  directory hadn't followed. Added `a45a5ad`, cut `ec3e8cc`. The failure it prevented
  is still caught: `checkGenerated`'s cross-root scan compares against the roots
  `cinch.yml` implied at HEAD, which is the detection half and landed in the same
  commit. What went was the one-shot migration command, exercised roughly once per
  consumer per lifetime. **Reopen only if** a consumer actually corrupts a docs root
  by hand-editing `paths.docs`, which the generated check would now name.
- **The `coupling` check** — transition-scoped rule-text coupling. Cut `712daba`, which
  does state its reasoning: it duplicated what an immersed human review already catches,
  could not tell a punctuation reword from a semantic one, and so blocked legitimate
  changes. The silent-failure class it was aimed at is covered by the identity check
  (rule-ID disappearance) and the `rules` check (unmarked rules). Its git helpers
  survive in `git.go`. **Reopen only if** a measured false-negative shows up that
  neither of those two catches.
- **The `retirement` check** — HEAD^-vs-HEAD diffing of rule IDs against an opt-in
  tombstone pattern, with a bounded worker pool fanning `git show` across doc files.
  294 lines total (153 production, 141 test) for a check `retirement.pattern` never
  turned on in either consumer's `cinch.yml`. Its own `idIsTombstoned` had a known,
  seeded-and-never-fixed substring bug (a tombstone naming `SIG-020` falsely covered
  the retirement of `SIG-02`). The one failure mode it uniquely caught — a doc rule
  entry and its marker disappearing together, cleanly, in one commit — is exactly the
  case a diff review already catches on a repo this small; the other half of "silent
  rule disappearance" (an orphaned marker or an orphaned doc entry) was already a loud,
  unconditional `rules` finding with no opt-in required. **Reopen only if** a
  consumer's rule corpus grows large enough that reviewers stop reliably catching a
  clean same-commit removal in diff review — and fix the substring match before
  reopening it.

### On the no-retrieval-tool rule

`cinch index --links-to/--links-from` is retrieval-shaped and lands after the bullet
above that forbids retrieval features. That is deliberate, not an oversight. The
benchmark's finding is that tooling adds nothing to *reading* the corpus — grep and a
pointer sufficed. The link flags aren't justified on retrieval value; they exist
because relative-path resolution is a thing grep provably cannot do: `project_deltadocs`
holds 12 colliding basenames over 63 docs, and `architecture.md` alone has 20 inbound
references in five spellings resolving to three different files. Where grep *is*
sufficient — matching titles — no flag was added, because `cinch index | grep` already
is one. Apply the same test to the next proposal: if grep can answer it, don't build it.
