# cinch

cinch is a Go CLI that checks a repo's docs against the repo. It runs from
git hooks and fails the commit when:

- a relative markdown link points at a file that doesn't exist
- a rule in the docs has no test marker and no stated reason for lacking one
- a test marker names a rule that doesn't exist
- a file cinch generated from `cinch.yml` was edited by hand

It also ships a set of workflow docs (planning a feature, fixing a bug,
syncing docs, auditing rule coverage) written for a person or a coding agent
to follow step by step.

## Install

```
curl -fsS https://raw.githubusercontent.com/kanmear/cinch/main/install.sh | bash
```

To pin a version, set `CINCH_VERSION=vX.Y.Z` before running the script. It
installs a prebuilt binary from
[Releases](https://github.com/kanmear/cinch/releases) to `/usr/local/bin`, or
to `~/.local/bin` if the first isn't writable. On Windows, download the `.zip`
from Releases.

To build from source: `make build` (output in `bin/cinch`).

## Quick start

In the repo you want to check:

```
cinch init     # writes cinch.yml, renders the workflows and hook shims, sets core.hooksPath
cinch check    # exit 0 clean, 1 findings, 2 usage error
```

`init` asks a few questions when run in a terminal (docs path, hooks path,
commit pattern, version pin, extra pre-commit scripts) and uses defaults
otherwise. It won't overwrite a `core.hooksPath` that another tool already set.

From then on, every commit runs the checks. To update later, run
`cinch upgrade`. It offers to install the latest release, re-renders, updates
an exact `require.cinch` pin, and runs the checks.

## Rules and markers

A rule is a numbered list item whose first token is a bold ID:

```markdown
---
rule_prefix: AUTH
owns:
  - src/auth
---

# Auth rules

1. **AUTH-001** Sessions expire after 24 hours.
2. **AUTH-002** Passwords are never logged.
   <!-- cinch:ignore: covered by log review, not a unit test -->
```

Each rule needs one of two things:

- a `cinch:rule AUTH-001` comment in the code or test that enforces it. The
  comment leader can be `//`, `#`, `--`, `<!--`, `/*`, `%` or `;`, so most
  languages work.
- a `<!-- cinch:ignore: <reason> -->` line under the rule, saying why it
  can't be tested.

The front matter is optional. `rule_prefix` sets the doc's ID prefix, and
`owns` lists the code paths the doc's rules cover (prefix match, not globs).
`cinch impact` reads both.

Here's what `cinch check` prints for the doc above with one mistyped marker:

```
rules: 2 findings
rules error .docs/domain/auth.md:9: AUTH-001: no // cinch:rule marker (or cinch:ignore declaration)
rules error src/auth/s_test.go:1: // cinch:rule AUTH-01 does not resolve to any rule ID (did you mean AUTH-001 or AUTH-002?)
```

Duplicate rule IDs are also findings.

### What this doesn't prove

The check only confirms a marker with the right ID exists somewhere in the
tracked files, or that an ignore exists. It can't tell whether the marked test
actually asserts the rule, whether the marker is above the right function, or
whether an ignore's reason holds up. All of those are cheap to fake.

It also can't find a rule nobody wrote. A behavior the docs never state, such as
who receives a realtime event, has no ID to check, so `cinch check` stays green.
The planning and bug-fix workflows ask you to name missing rules; nothing
mechanical does.

What you do get: a rule nobody accounted for fails loudly, by ID, instead of
drifting quietly. Whether the tests really enforce the rules is a question for
code review and the `docs-audit-coverage` workflow.

## Checks

`cinch check` runs these and prints `ok`, `skip`, or findings for each:

| check | fails when |
|---|---|
| `links` | a relative markdown link under the docs root doesn't resolve |
| `rules` | a rule has no marker or ignore; a marker names an unknown rule; a rule ID appears twice. Also scans `rules.roots` if set |
| `index` | a doc outside `plans/` has no `# ` title |
| `generated` | a rendered workflow or hook shim differs from what `cinch render` would write now |
| `hooks` | `core.hooksPath` isn't set to `paths.hooks` |
| `commit` | (opt-in) the commit subject doesn't match `commit.pattern` |
| `core` | (opt-in) the installed cinch doesn't satisfy `require.cinch` |

The pre-commit hook and `cinch check --staged` check the content being
committed, not your working tree: an unstaged edit can't block a commit, and
an unstaged or untracked marker can't let a broken one through.
`cinch check --changed` limits the run to files with unstaged changes.
`--only links,rules` runs just the named checks.

## Commands

```
cinch init              set up cinch.yml, render, activate hooks
cinch check [MSGFILE]   run all checks (MSGFILE = in-progress commit message)
cinch check --changed [--only NAMES] [MSGFILE]
                        only check unstaged files / only run the named checks
cinch check --staged [--only NAMES] [MSGFILE]
                        check the staged content, as the pre-commit hook does
cinch render            re-render workflows and hook shims from cinch.yml
cinch upgrade           install the latest release, re-render, sync the pin, check
cinch hook EVENT [ARGS] git-hook dispatcher (called by the generated shims)
cinch workflows         print the workflow table
cinch workflow NAME     print one rendered workflow
cinch index             print every doc's path and title
cinch index --links-to PATH
                        docs that link to PATH
cinch index --links-from PATH
                        docs that PATH links to
cinch ignores           list every cinch:ignore and its reason
cinch rules --json      rule inventory as JSON (rules, markers, ignores, owns)
cinch impact [FILES]    rule IDs a change may affect (default: staged files)
cinch version
cinch help
```

A few notes:

- **`hook`**: for `pre-commit` and `commit-msg`, it runs the checks and then
  every script registered under `hooks.<event>` in `cinch.yml` whose `when`
  prefixes match the changed files. cinch only runs these scripts; whether
  they're correct is up to you. The checks read the staged content; these
  scripts, and the `impact` list, see your working tree.
- **`impact`**: lists rules from docs whose `owns` covers a changed file, plus
  rules whose marker is in a changed file. The pre-commit hook prints the same
  list as a reminder. It never fails the commit.
- **`index --links-to` / `--links-from`**: reuses the link check's path
  resolution, so it knows which of several `architecture.md` files a given
  `../architecture.md` refers to, which grep can't. A bare basename matches
  every doc with that name; a path with a directory matches exactly. Unlike
  plain `index`, the link graph includes `plans/`.

## Workflows

`cinch render` writes these into `<paths.docs>/workflows/`, filling in values
from `cinch.yml`. The `generated` check fails if you edit them by hand. To
change them, change `cinch.yml`.

| workflow | use it when | what it does |
|---|---|---|
| `dev-plan-feature.md` | starting a non-trivial feature | Lists the rules the feature touches, plans one test per rule, splits the work into small tasks. Writes a plan under `<docs>/plans/`. No code yet. |
| `dev-fix-bug.md` | a bug is reported | Reproduce, find the cause, write a regression test, plan the fix. Writes `<docs>/plans/fix/<slug>.md`. |
| `dev-task-primitive.md` | (used by the two above) | How to split tasks, what context each task lists, how to verify it, how to close it out. |
| `dev-execute-plan.md` | a plan exists | Work through the plan one task per session, leaving notes so the next session can pick up. |
| `docs-maintain-domain.md` | adding or editing rules | Where rules go, how to number them, how to write them. |
| `docs-sync.md` | after a code change | Bring the non-rule docs (API, models, architecture, `cinch.yml`) in line with the code. |
| `docs-audit-coverage.md` | periodically | Go rule by rule and check whether a test really enforces it. Reports each as covered, contradicted, missing, or not applicable, with evidence. |
| `docs-audit-quality.md` | periodically | Review the docs for bloat, staleness, and departures from `docs-philosophy.md`. |
| `docs-philosophy.md` | (reference) | The principles the quality audit checks against. |

The marker check can be satisfied mechanically. The audits exist to check
whether the markers mean anything.

## `cinch.yml`

```yaml
paths:
  docs: .docs            # docs root; workflows are rendered to <docs>/workflows/
  hooks: .githooks       # where hook shims go; must match core.hooksPath

# Optional: other repos to scan for cinch:rule markers. For a docs repo that
# sits next to the code it describes. Paths may use "..", but can't be
# absolute. Missing roots are skipped.
#rules:
#  roots: [../backend, ../frontend]

# Scripts to run at each hook event. `when` is a list of path prefixes (not
# globs) matched against staged files (pre-commit) or committed files
# (post-commit). Leave it out to always run. commit-msg scripts get the
# message file as $1.
#
# pre-commit-checks / commit-msg-checks change which checks the hooks run.
# Defaults: pre-commit [links, rules, index, generated, core],
# commit-msg [commit].
hooks:
  #pre-commit-checks: [links, index, generated, core]
  #commit-msg-checks: [commit]
  pre-commit:
    error-codes:
      run: scripts/check_error_codes.sh
      when: [frontend/src/lib/api/, backend/errors/]
  post-commit:
    notify:
      run: scripts/notify.sh

# Optional: commit subjects must match this regexp.
#commit:
#  pattern: '^\[[a-z-]+\] .+'

# Optional: required cinch version, exact (0.1.0) or minimum (>=0.1.0).
#require:
#  cinch: '>=0.1.0'
```

## Development

```
make build     # build to bin/cinch and run go vet
make test      # go test ./...
```

Plans for upcoming work, and notes on features that were removed and why, are
in [`.docs/plans/`](.docs/plans/00-overview.md).

## License

[MIT](LICENSE)
