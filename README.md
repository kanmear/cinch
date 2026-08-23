# cinch

A portable, project-agnostic convention enforcer for operational docs — with
first-class support for agentic workflows. Cinch is two things: a convention
for writing operational docs that stay true by construction, and a CLI that
enforces the convention on every commit via git hooks.

## The docs convention

- A **rule** is a numbered list item in a doc: `1. **PREFIX-NNN** <text>`
  (the prefix is per-doc). Every rule ID must be bound to a
  `// cinch:rule PREFIX-NNN` line-comment marker in the code that enforces
  it, or explicitly declared untestable with a reasoned
  `<!-- cinch:ignore: <reason> -->`. So "documented" and "enforced" can't
  silently diverge — a rule with neither a marker nor an ignore is a finding.
- Markdown links between docs must resolve on disk; nothing in the corpus is
  allowed to point at a moved or deleted file.
- A set of embedded **workflow templates** (planning a feature, fixing a bug,
  syncing docs after a code change, auditing rule coverage/quality,
  maintaining domain docs) get rendered into `paths.docs/workflows/` by
  `cinch render`, `{{key}}`-substituted against `cinch.yml`. They're the
  process docs an agent or contributor follows; the docs convention is what
  keeps them honest.

## The CLI

`cinch check` runs seven checks, each reported as `ok`, `skip`, or a finding:

| check | enforces |
|---|---|
| `links` | every relative markdown link under the docs root resolves on disk |
| `rules` | every documented rule ID has a marker or an explicit ignore |
| `retirement` | (opt-in) a rule ID present at `HEAD^` is still present at `HEAD` or matches your tombstone pattern |
| `generated` | rendered output (workflows, hook shims) is byte-identical to what `cinch render` produces now — hand-edits and tampering fail here |
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

### Commands

```
cinch init              scaffold a consumer (cinch.yml, render, activate hooks)
cinch check [MSGFILE]   run all checks (MSGFILE = in-progress commit message)
cinch render            re-render docs/workflows and hook shims from cinch.yml
cinch hook EVENT [ARGS] git-hook dispatcher run by the generated shims
cinch workflows         print the workflow trigger table
cinch workflow NAME     print one rendered workflow
cinch index             print every doc's path and title
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
- **Rule markers beyond `//`** — support `#`, `--`, and block-comment marker
  syntax so `#`-comment languages can bind rules idiomatically.
- **Index-aware pre-commit checks** — check the staged index rather than the
  working tree, so a partially-staged edit can't slip past pre-commit.
- **Overridable template packs** — let a project supply its own workflow
  templates instead of the fixed set embedded in the binary.
