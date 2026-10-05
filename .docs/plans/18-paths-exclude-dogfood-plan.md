# 18. `paths.exclude`, then run cinch on cinch — the hand-off plan

**Status: shipped in `9830825` (`feature/paths-exclude-dogfood`). Kept for provenance.** The
implementation hand-off for item 8 (`08-paths-exclude-and-dogfood.md`), which holds the
original argument. Depended on 13 (link fix), 14 (CINCH-006 binds to its tests) and 17 (so
cinch renders the trimmed pack).

## Why

`go run . check` in this repo gave 28 `rules` findings, all from marker-shaped fixtures in
`rules_test.go`, `hook_test.go`, `impact_test.go` and `rules_json_test.go`. Cinch had never
run on itself.

## Part A: `paths.exclude`

- A list key. An entry ending in `/` is a directory prefix; anything else is a
  `path.Match` pattern against the repo-relative slash path (`*` doesn't cross `/`).
- Validated at load: local, not absolute, a valid pattern — otherwise `cinch.yml` fails to
  load naming the entry.
- Applies to the marker scan of the primary root only, so `checkRules`, `rules --json` and
  `impact` all honor it, staged mode included. Docs scanning is unaffected.
- Tests in `exclude_test.go`.

## Part B: dogfood

1. `cinch.yml` at the repo root: `.docs`, `.githooks`, the four fixture files excluded,
   pre-commit checks `[links, rules, index, core]` (`generated` left to CI because the
   shims exec the installed binary), and the two existing scripts registered as
   `commit-msg` and `post-commit` hooks.
2. `.docs/rules.md` with `rule_prefix: CINCH` and CINCH-001…007, each bound to a test in a
   non-excluded file (CINCH-007, no network egress of repo content, carries a
   `cinch:ignore` since it is a negative property verified by review).
3. `make setup-hooks` installs cinch and runs `go run . init`; the rendered workflows and
   shims are committed as generated files.
4. CI runs `go run . check` after the tests.
5. `.docs/git-conventions.md` and the README describe cinch checking itself.

## Since shipping

- `owns:` started as `internal/cinch/` and made the impact advisory list every rule on
  nearly every commit. It was narrowed to the five files the rules cover
  (`chore/quiet-self-hooks`).
- The built-in `commit` check printed a skip on every commit, because the message check is
  the script. `hooks.commit-msg-checks: []` now turns it off, which needed an explicit
  empty list to mean "none" rather than "defaults" (`fix/hook-checks-explicit-empty`).
