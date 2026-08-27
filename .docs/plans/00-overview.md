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
| 2 | [`cinch upgrade`](02-cinch-upgrade.md) | 1 — trust the checker | Open |
| 3 | [Close rule-grammar holes, then seed defects](03-rule-grammar-holes-and-seed-defects.md) | 1 — trust the checker | Open |
| 4 | [Measure binding strength before designing for it](04-measure-binding-strength.md) | 2 — make green mean something | Open |
| 5 | [Navigability as one check class](05-navigability-check-class.md) | 2 — make green mean something | Open |
| 6 | [The machine surface: `rules --json` + `owns:` → `cinch impact`](06-machine-surface-rules-json-impact.md) | 2 — make green mean something | Open |
| 7 | [Workflow prose value and F1's scope](07-workflow-prose-value-and-f1-scope.md) | 3 — decide before investing | Open, reframed |
| 8 | [`paths.exclude`, then dogfood](08-paths-exclude-and-dogfood.md) | 3 — decide before investing | Open |
| 9 | [Calibrate every heuristic against real history](09-calibrate-heuristics-against-history.md) | 3 — decide before investing | Open |
| 10 | [`kind:` — normative versus descriptive](10-kind-normative-vs-descriptive.md) | 3 — decide before investing | Open |

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
