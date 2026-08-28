# cinch

A small Go CLI plus a convention for in-repo operational docs. The CLI checks a
few mechanical properties of the docs corpus on every commit (via git hooks):
markdown links resolve, every documented rule is either bound to a test marker
or explicitly declared untestable, and rendered files still match
`cinch.yml`. It also ships a set of workflow documents — step-by-step process
for planning features, fixing bugs, keeping docs in sync, and auditing rule
coverage — that a human or a coding agent can follow.

## The docs convention

- A **rule** is a numbered list item in a doc: `1. **PREFIX-NNN** <text>`
  (the prefix is per-doc). The rule ID must be bound to a
  `cinch:rule PREFIX-NNN` marker in the code or test that enforces it —
  leader `//`, `#`, `--`, `<!--`, `/*`, `%`, or `;`, so `#`-comment,
  SQL-style, and block-comment languages can bind idiomatically too — or
  explicitly declared untestable with a reasoned
  `<!-- cinch:ignore: <reason> -->`. A rule with neither is a finding from
  `cinch check`.
- Markdown links between docs must resolve on disk.

**What the marker check does and doesn't prove.** The check is mechanical: it
verifies that a marker with the right ID exists somewhere in the tracked tree
(or that an ignore exists). It does *not* verify that the marked test actually
asserts the rule's meaning, that the marker sits above the right function, or
that an ignore's reason is genuine — any of those can be faked cheaply to make
the check pass. What cinch guarantees is the inverse: an *unaccounted-for*
rule is a loud, per-rule failure instead of silent drift. Judging whether a
marked test really enforces its rule is deliberately left to the audit
workflows below (and to code review); cinch surfaces the inventory, the audits
judge the semantics.

## Shipped workflow docs

`cinch render` writes a fixed set of embedded workflow templates into
`<paths.docs>/workflows/`, substituting `{{key}}` tokens against `cinch.yml`
(so each workflow references *this* project's docs root, test roots, etc.).
The `generated` check fails if any of them is hand-edited — to customize, edit
`cinch.yml` or (roadmap) supply your own templates.

The workflows are the process docs a contributor or agent follows; they are
written to be executed literally, with explicit checkpoints and output files:

| workflow | when to run | what it does |
|---|---|---|
| `dev-plan-feature.md` | start of a non-trivial feature | Enumerate the domain rules the feature touches, build a rule-interaction table for multi-domain features, derive a test plan (one test per rule, classified by tier), then decompose into atomic tasks. Output: a living plan file under `<docs>/plans/`. No implementation code is written in this workflow. |
| `dev-fix-bug.md` | when a bug is reported | Reproduce, identify root cause, write a regression test, decompose the fix. Output: `<docs>/plans/fix/<slug>.md`. |
| `dev-task-primitive.md` | (shared machinery) | The common decomposition machinery both planning workflows hand off to: atomicity rules, context manifests, verification tiers, the task template, and the completion ritual. |
| `dev-execute-plan.md` | a plan file exists | Work through a plan's tasks one per session, with a session-handoff trail so work can resume cold. |
| `docs-maintain-domain.md` | adding/editing/reorganizing domain rules | Placement, numbering, and style conventions for rule docs. |
| `docs-sync.md` | after a code change | Update the non-rule docs (API, models, architecture, conventions, troubleshooting, `cinch.yml`) to match the code. |
| `docs-audit-coverage.md` | periodically | Start from `cinch check`'s per-rule inventory, classify each rule (API-enforced / model-enforced / N-A), derive a test→rule mapping from the assertions *before* consulting markers, and report per rule with verbatim evidence: covered / TENSION (marked test contradicts the rule's text) / MISSING / N-A. |
| `docs-audit-quality.md` | periodically | Audit the whole docs corpus against `docs-philosophy.md` for bloat, staleness, and violations of the doc philosophy. |
| `docs-philosophy.md` | (reference) | The principles the quality audit enforces; read first by the rule-maintenance workflow. |

The dev workflows and the audit workflows are complementary: the marker check
can be satisfied mechanically, the audits are where that satisfaction gets
questioned.

## The CLI

`cinch check` runs seven checks, each reported as `ok`, `skip`, or a finding:

| check | enforces |
|---|---|
| `links` | every relative markdown link under the docs root resolves on disk |
| `rules` | every documented rule ID has a marker or an explicit ignore |
| `retirement` | (opt-in) a rule ID present at `HEAD^` is still present at `HEAD` or matches your tombstone pattern |
| `generated` | rendered output (workflows, hook shims) is byte-identical to what `cinch render` produces now — hand-edits fail here |
| `hooks` | `git config core.hooksPath` actually points at `paths.hooks` |
| `commit` | (opt-in) the commit subject matches `commit.pattern` (checked from a `commit-msg` hook) |
| `core` | (opt-in) the installed cinch version satisfies `require.cinch` |

`cinch init` writes a starter `cinch.yml`, renders, and points
`core.hooksPath` at `paths.hooks` (refusing to clobber a hooks path owned by
another tool). `cinch hook <event>` is the dispatcher the generated shims
exec into: it runs every `hooks.<event>.<name>` entry from `cinch.yml` whose
`when` path prefixes match the staged/committed files, then runs `cinch
check` (pre-commit/commit-msg only). Registered scripts are opaque to cinch —
it guarantees dispatch, not script correctness. `cinch workflows` / `cinch
workflow NAME` / `cinch index` compute the workflow table and doc index from
disk on demand, so nothing persisted can drift.

`cinch index --links-to PATH` and `--links-from PATH` answer which docs
reference a doc, and which it references. The links check already resolves
every markdown link against its source file's directory to decide whether the
target exists; these flags read the same resolution instead of re-parsing, so
the graph and the check can't disagree. The resolution is the point: a corpus
routinely holds several `architecture.md`, referenced as `../architecture.md`
from one directory and `./architecture.md` from another, and grep can't say
which file any of them means. Pass a bare basename and every doc answering to
it comes back with its link counts; pass a path with a directory in it and
that doc is addressed exactly. Unlike `cinch index`, the graph counts
`plans/` as both source and target — excluding it answers "what is the doc
corpus", but "what links here" would be lying by omission.

### Commands

```
cinch init              scaffold a consumer (cinch.yml, render, activate hooks)
cinch check [MSGFILE]   run all checks (MSGFILE = in-progress commit message)
cinch render            re-render docs/workflows and hook shims from cinch.yml
cinch hook EVENT [ARGS] git-hook dispatcher run by the generated shims
cinch workflows         print the workflow trigger table
cinch workflow NAME     print one rendered workflow
cinch index             print every doc's path and title
cinch index --links-to PATH
                        list the docs that link to PATH
cinch index --links-from PATH
                        list the docs PATH links to
cinch ignores           list every cinch:ignore declaration with its reason
cinch version
```

## Install

```
curl -fsS https://raw.githubusercontent.com/kanmear/cinch/main/install.sh | bash
```

Pin a version: `CINCH_VERSION=v0.2.0 curl -fsS https://raw.githubusercontent.com/kanmear/cinch/main/install.sh | bash`

Installs a prebuilt binary from [GitHub Releases](https://github.com/kanmear/cinch/releases) to `/usr/local/bin` (or `~/.local/bin` if that isn't writable). Windows: download the `.zip` from Releases directly.

## Adopt

```
cinch init     # in the target repo: writes cinch.yml, renders, activates hooks
cinch check    # exits 0 when clean (1 findings, 2 usage error)
```

Then write rules under `paths.docs` and register project-specific checks as `hooks.<event>.<name>` entries.

## `cinch.yml`

```yaml
paths:
  docs: .docs            # docs corpus root; rendered workflows land in <docs>/workflows/
  hooks: .githooks       # where generated hook shims land (must be core.hooksPath)

# Project-specific checks, dispatched by cinch hook at each event.
# `when` is a list of path prefixes (not globs) scoped to the staged set
# (pre-commit) or the committed set (post-commit); omit for always.
# commit-msg entries receive the message file path as $1.
hooks:
  pre-commit:
    error-codes:
      run: scripts/check_error_codes.sh
      when: [frontend/src/lib/api/, backend/errors/]
  post-commit:
    notify:
      run: scripts/notify.sh
      # when: [frontend/]

# Opt-in: commit subjects must match this regexp (commit-msg hook).
#commit:
#  pattern: '^\[[a-z-]+\] .+'

# Opt-in: tombstone convention for deleted rules. A rule ID removed in the
# latest commit is accepted if this regexp matches file content containing
# the ID's text.
#retirement:
#  pattern: '<!--\s*retired:.*-->'

# Opt-in: pin the cinch version (exact `0.1.0` or `>=0.1.0`).
#require:
#  cinch: '>=0.1.0'
```

## Roadmap

- **Multi-commit retirement tracking** — `retirement` currently only diffs
  `HEAD^` → `HEAD`, so a removal can be missed if a second commit lands
  before the next check.
- **Index-aware pre-commit checks** — check the staged index rather than the
  working tree, so a partially-staged edit can't slip past pre-commit.
- **Overridable template packs** — let a project supply its own workflow
  templates instead of the fixed set embedded in the binary.
