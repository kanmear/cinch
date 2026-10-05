# 15. `cinch impact`: one line per doc, not one per rule

**Status: shipped in `ed08e9e` (`fix/impact-attribute-by-doc`). Kept for provenance.**
Depended on nothing.

## Why

`impact` prints during every pre-commit. Touching one unrelated file under an `owns:` path
listed every rule in that doc, one line each — 8–20 lines per commit on real corpora, and a
list that is always long stops being read.

Correctness bug as well: `buildImpact` matched IDs with
`strings.HasPrefix(id, rule_prefix)`, so a doc with `rule_prefix: AUTH` also claimed
`AUTHZ-001`.

## Change (`internal/cinch/impact.go`)

1. Attribute rules to docs by file: `rulesByDoc` maps a doc to the IDs declared in it. An
   `owns:` match on a doc means that doc's rules; the `rule_prefix == ""` skip went away. A
   doc with `owns:` and no rules produces nothing. `rule_prefix` is still parsed and
   emitted by `rules --json`, where it is informational.
2. Marker hits stay per rule and gain the line number.
3. Output: marker lines first, then one owns line per doc, each list capped at three items
   plus `+N more`, then a single `confirm the docs still match`. Doc paths are
   repo-relative. Nothing is printed when there are no hits.

The shipped owns line reads
`.docs/rules.md — owns <files> — rules <IDs>` rather than the `N rules (…)` shape
originally drafted.

## Tests (`impact_test.go`)

Collapses per doc; attribution by doc, not prefix (`AUTH` never pulls in `AUTHZ-`); marker
line carries `file:line`; `printImpact` matches a golden string; the hook's
never-blocks and error-swallowed tests still pass.
