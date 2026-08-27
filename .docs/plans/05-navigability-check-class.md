# 5. Navigability as one check class

**Tier 2 — make green mean something. Cost estimate: ~30 lines.**

## Provenance

**Both.** Audit finding **F3** (docs without an H1 vanish from `cinch index` silently) plus
the benchmark's independent discovery of the same underlying property from the value-of-corpus
side: an unreachable corpus scored identical to no corpus at all, across every metric in the
retrieval battery (`~/code/cinch_bench/FINDINGS.md`, and the roadmap's framing: "an
unreachable corpus measured identical to no corpus, on every metric"). Two failure modes, one
property — the corpus must be *navigable*, not merely present.

## The two current gaps, grounded in code

### 5a. `links.go` checks existence, not reachability

`checkLinksInFile` (`internal/cinch/links.go:48-94`) resolves every markdown link target with
`os.Stat(resolved)` and reports a finding only if the file **doesn't exist**:

```go
resolved := filepath.Join(filepath.Dir(path), target)
if _, err := os.Stat(resolved); err != nil {
    findings = append(findings, finding{... "link target does not resolve: " + target})
}
```

This says nothing about whether the file being linked to is itself reachable from anywhere an
agent or reader would actually start — `AGENTS.md`, an index, a declared root. A doc can exist,
have valid inbound-link syntax pointing at it from nowhere that matters, and still be
functionally invisible. `links.go` has no notion of a root or a reachability graph at all; it
only ever looks at one file's outbound links in isolation.

### 5b. A doc without an H1 silently vanishes from `cinch index`

`titleAndTrigger` (`internal/cinch/title.go:8-20`) returns `("", "")` if it never finds a line
starting with `# `. `indexList` (`internal/cinch/index.go:20-33`) uses that directly:

```go
title, _ := titleAndTrigger(string(data))
if title == "" {
    return nil, nil  // silently dropped, no finding anywhere
}
```

The doc is skipped from the collected `entries` with no error, no warning — `cinch index`
just doesn't mention it. Since `cinch check` never calls `indexList` or `titleAndTrigger` at
all (`check.go:74-86`'s launch list has no such check), there is currently **no path** by
which a missing-H1 doc produces a finding. It's discoverable only by noticing the doc missing
from `cinch index`'s output, which requires already suspecting something's wrong.

## Proposed shape

Add a `navigability` check (or fold into an existing pass — see the design note below) that
asserts the property both findings are instances of: **every doc under `paths.docs` has a
title, and is reachable by link from a declared root.**

1. **Title requirement.** Reuse `titleAndTrigger` — if it returns an empty title for any
   markdown file under `docsRoot` (excluding whatever `indexList` already excludes, e.g. the
   `plans/` prefix — `index.go:22-23`), emit a `finding` instead of silently continuing. This
   is a small, mechanical addition: wrap the existing `collectMarkdown` walk (already used
   by both `checkRules`/`scanRuleDocs` in `rules.go:105-107` and `indexList` in
   `index.go:20-34`) with a title check.
2. **Reachability requirement.** This is the larger piece: build a directed graph from every
   markdown link (`mdLinkRe`, already defined in `links.go:13`) resolved relative to its
   source file, rooted at whatever the manifest declares as the entry point(s) — likely
   `AGENTS.md` plus `paths.docs` itself, or an explicit new manifest key if there's more than
   one plausible root (check whether `project_deltadocs`'s corpus has a single entry point or
   several before deciding the key shape). Do a reachability walk from the root(s); any doc
   under `docsRoot` not visited is a finding — "unreachable from any declared root: no
   incoming link path from AGENTS.md". This reuses `links.go`'s existing link-extraction
   regex and file-resolution logic; the new part is graph construction and a BFS/DFS instead
   of link-by-link existence checks.

## Design note: new check vs. extending `links`

`links.go` already owns link parsing and target resolution — extending `checkLinks` to also
build the reachability graph (rather than inventing a parallel parser) is likely the smaller
diff, since `checkLinksInFile` already walks every file and resolves every target; reachability
is a second pass over the same resolved-target data rather than new parsing. The title check
is unrelated to link parsing and more naturally sits with `indexList`'s existing walk, or as
its own small check function following this repo's `checkX` naming convention
(`.docs/conventions.md`'s "check functions" rule) — decide based on how `check.go`'s
`launch(...)` list reads afterward; a single `navigability` check bundling both properties
under one name is probably the most legible framing for `cinch check` output, even if the
implementation touches two existing files.

## Why tier 2

This is squarely "make green mean something": today `cinch check` can be fully green on a
corpus that is present in the repo but functionally inert — no different, per the benchmark,
from a repo with no corpus at all. That's not a hypothetical; it's a measured equivalence.
Tier 1 makes the checker's *existing* signals trustworthy; this item makes the checker notice
a failure mode it currently has no way to see, which is a precondition for item 6's `owns:`
work and item 10's `kind:` work meaning anything — a machine-readable rule inventory or a
normative/descriptive split is worthless if some fraction of what it's describing was never
reachable in the first place.

## Verification

- Unit tests: a fixture doc with no `# ` line under `docsRoot` produces a navigability
  finding; a fixture doc that exists, has a title, but is linked from nowhere under the
  declared root(s) produces a finding; a fixture that's properly linked and titled produces
  none.
- Regression case from the benchmark: reproduce the actual unreachable-doc condition found
  during the study (a relative link broken by a directory move, per `FINDINGS.md`'s methodology
  notes about `nowf-clean`'s contaminated strip pattern missing `../workflows/docs-philosophy.md`)
  and confirm the new check catches what `links.go` alone did not.
- Feed into item 3's seeded-defect battery: add "link exists but is unreachable from any root"
  as a battery case, and flip its expected-result from "not caught" to "caught" once this
  ships.
