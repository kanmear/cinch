# cinch

A CLI that verifies the operational docs in a repository (rules, workflows, conventions) actually match the code, and enforces that on every commit via git hooks.

## What it checks

`cinch check` runs seven checks, each reported as `ok`, `skip`, or a finding:

| check | enforces |
|---|---|
| `links` | every relative markdown link under the docs root resolves on disk |
| `rules` | every documented rule ID has a `// cinch:rule <ID>` marker in source, or an explicit `<!-- cinch:ignore: <reason> -->` |
| `retirement` | (opt-in) a rule ID present at `HEAD^` is still present at `HEAD` or matches your tombstone pattern |
| `generated` | rendered output (workflows, hook shims) is byte-identical to what `cinch render` produces now — hand-edits and tampering fail here |
| `hooks` | `git config core.hooksPath` actually points at `paths.hooks` |
| `commit` | (opt-in) the commit subject matches `commit.pattern` (checked from a `commit-msg` hook) |
| `core` | (opt-in) the installed cinch version satisfies `require.cinch` |

## How it works

- Rules are numbered items in a doc (`1. **ID-NNN** ...`). Each must be bound to a `// cinch:rule ID-NNN` line-comment marker in code that enforces it, or declared untestable with a reasoned `cinch:ignore`. So "documented" and "enforced" can't silently diverge.
- `cinch render` writes the workflow docs into `paths.docs/workflows/` and a thin shim per git hook event into `paths.hooks` (`exec cinch hook <event>`). Substitution is literal `{{key}}` replacement against `cinch.yml`; the render is idempotent and each file is stamped with a body-sha, which is what makes `generated` able to detect tampering.
- `cinch init` writes a starter `cinch.yml`, renders, and sets `git config core.hooksPath` to `paths.hooks` (it refuses to clobber a hooks path owned by another tool).
- `cinch hook <event>` is the dispatcher the shims exec into: it runs every `hooks.<event>.<name>` entry from `cinch.yml` whose `when` path prefixes match the staged/committed files, then runs `cinch check` (pre-commit/commit-msg only). Registered scripts are opaque to cinch — it guarantees dispatch, not script correctness.
- `cinch workflows` / `cinch workflow NAME` / `cinch index` compute the workflow table and doc index from disk on demand — nothing is persisted, so nothing drifts.

## Install

```
curl -fsS https://raw.githubusercontent.com/kanmear/cinch/main/install.sh | bash
```

Pin a version: `CINCH_VERSION=v0.2.0 curl -fsS https://raw.githubusercontent.com/kanmear/cinch/main/install.sh | bash`

Installs a prebuilt binary from [GitHub Releases](https://github.com/kanmear/cinch/releases) to `/usr/local/bin` (or `~/.local/bin` if that isn't writable). Windows: download the `.zip` from Releases directly.

Building from source (contributors / Go users):

```
make install   # go install with version ldflags → cinch on PATH
```

The binary must be *installed* (not just built): hook shims look up `cinch` on `PATH`. Re-install after changing cinch itself — the workflow templates are `go:embed`ed, and a stale binary re-renders stale output that the `generated` check will flag.

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

## Commands

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

## Known limitations

- `retirement` only diffs `HEAD^` → `HEAD`; a removal is missed if a second commit lands before the next check.
- Rule markers use `//` line-comment syntax only — `#`-comment languages can't mark rules idiomatically.
- Pre-commit verifies the working tree, not the index — fully stage what you mean to commit.
- The embedded workflow template pack is not overridable.
