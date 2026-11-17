---
rule_prefix: CINCH
owns:
  - internal/cinch/checks_misc.go
  - internal/cinch/render.go
  - internal/cinch/init.go
  - internal/cinch/links.go
  - internal/cinch/staged.go
  - internal/cinch/hook.go
  - internal/cinch/rules.go
  - internal/cinch/plans.go
  - internal/cinch/move.go
  - internal/cinch/archive.go
---

# Cinch rules

Behaviour cinch guarantees about itself. Each rule is bound by a
`cinch:rule` marker on the test that enforces it; `cinch check` runs on this
repository, so an unbound rule or a stray marker fails CI.

1. **CINCH-001** A rendered file edited by hand is a `generated` finding.
2. **CINCH-002** A header-stamped file the current render no longer produces
   is a finding, and `cinch render` removes it.
3. **CINCH-003** `cinch init` never overwrites a `core.hooksPath` another tool
   set, and refuses before writing any file.
4. **CINCH-004** Render rewrites an exact `require.cinch` pin and never
   changes a range pin, `any`, or an unset pin.
5. **CINCH-005** Link syntax inside inline code is not a link.
6. **CINCH-006** The pre-commit hook checks staged content; unstaged edits
   neither block nor excuse a commit.
7. **CINCH-007** Cinch sends no repository content over the network.
   <!-- cinch:ignore: a negative property; the only network call is the release lookup in selfupdate.go, verified by review -->
8. **CINCH-008** `cinch init` writes a `>=` floor pin at the installed
   version. `cinch upgrade` raises a floor only when its rendered output
   changed, and never lowers one.
9. **CINCH-009** The marker scan skips binary files (a NUL byte in the first
   8 KB, git's own test), so a marker-shaped string inside one binds nothing.
10. **CINCH-010** Marker locations are reported in file-then-line order, so
    the same tree always gives the same `cinch rules --json` output.
11. **CINCH-011** `cinch check` scans the working tree as it is: a tracked file
    deleted from disk is skipped, not a scan error.
12. **CINCH-012** Inside a pre-commit hook, a script's plain `cinch check`
    checks the staged content.
13. **CINCH-013** A failing hook step doesn't stop later steps, and the hook's
    last lines name every step's result.
14. **CINCH-014** A rule item ends at the next heading or unindented paragraph,
    so a `cinch:ignore` outside its rule's item binds nothing.
15. **CINCH-015** While `$GIT_DIR/SQUASH_MSG` exists, the commit subject is
    checked against `commit.squash.<branch>` for the checked-out branch. A
    branch with no entry, a manifest without `commit.squash`, or no
    `SQUASH_MSG` falls back to `commit.pattern` as for any commit.
16. **CINCH-016** While `$GIT_DIR/MERGE_HEAD` exists, `commit.merge: exempt`
    passes the subject unchecked, `commit.merge: reject` fails it
    (CINCH-020), and any other value is a pattern that replaces
    `commit.pattern`; without the key a merge commit is checked like any
    other, and the key is ignored outside a merge.
17. **CINCH-017** With `plans.lifecycle: enforce` in `cinch.yml`, every `.md`
    file directly in `<docs>/plans/` or `<docs>/plans/fix/` has a
    `**Status:**` line in its first 15 lines whose status word is `planned`
    or `in-progress`, and every `.md` file in `<docs>/plans/archive/` has one
    whose status word is `complete` or `cut`. The status word is the text
    after `**Status:** ` up to the first whitespace or the end of the line.
    Each violation is one `plans` finding; files in other subfolders of
    `plans/` are not checked. Without the key the check enforces nothing.
18. **CINCH-018** `cinch mv` moves a `.md` file under the docs root to a
    target under the docs root that doesn't exist yet. Afterwards every
    relative link in the docs that resolved to the file, and every relative
    link inside it, still resolves. A refusal changes no file.
19. **CINCH-019** `cinch plan archive` moves only a plan whose status word is
    `complete` or `cut`, and only from directly in `<docs>/plans/`, into
    `<docs>/plans/archive/`, with CINCH-018's guarantees. A refusal changes
    no file.
20. **CINCH-020** With `commit.merge: reject`, every commit made while
    `$GIT_DIR/MERGE_HEAD` exists fails the `commit` check, whatever its
    subject, and the finding names `make merge` as the way to land a branch.
21. **CINCH-021** With `commit.protect: [<branch>…]` in `cinch.yml`, a commit
    on a listed branch passes the `commit` check's branch guard only while
    `$GIT_DIR/SQUASH_MSG` exists, or while `$GIT_DIR/MERGE_HEAD` exists and
    `commit.merge` isn't `reject`. The first commit on an unborn branch and
    a commit on a detached HEAD pass the guard. A `--no-verify` commit (the
    version bump's amend) isn't checked, by git's design. Without the key
    the guard does nothing.
    <!-- cinch:ignore: test lands in plan 33 T3 -->
