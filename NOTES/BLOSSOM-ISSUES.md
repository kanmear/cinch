# BLOSSOM issues — consumer-driven improvements

Potential improvements for cinch, collected from an audit of two consumers (2026-08-19):
`odin-upsr` and `project_deltadocs` (both pin `require.cinch: 0.2.0`). Each item: the observed
gap, the consumer evidence, and a suggested change. None are bugs — `cinch check` passes clean
in both repos — they are visibility, robustness, and ergonomics gaps that let shallow adoptions
look healthy.

Ordered by estimated value.

## 1. The `rules` check is silent on a thin corpus — DONE (d55f1fd)

- **Evidence:** odin-upsr has ~69 numbered business rules across 6 docs but only crews.md (8
  rules) carries `**CREW-0NN**` IDs; the other ~61 are prose bullets. `cinch check` still prints
  `rules: ok` — the check enforces almost nothing and nothing says so. A consumer with **zero**
  rule IDs would get the same green.
- **Suggested change:** report coverage in the `rules` result line — e.g. `rules: ok (8 rules,
  1 marked file, 1 ignore)` — and emit a NoOp (or a finding) when the scanned corpus has no rule
  IDs at all: "no rule IDs found — rules check enforces nothing".
- **Resolved in `d55f1fd`** ("feat: surface corpus coverage on rules/links check ok lines"):
  `checkRules`/`checkLinks` now report rule/doc/link/ignore counts as a detail suffix on the ok
  line, and a zero-corpus clean run is an explicit NoOp instead of a silent ok.

## 2. `cinch check` never verifies hooks are active — DONE (b3cb15d)

- **Evidence:** both consumers pass `cinch check` in a clone where `core.hooksPath` is unset —
  i.e. nothing actually enforces anything at commit time. Consumers hand-roll activation
  (`setup-hooks`) and the docs carry the "must run once per clone" burden.
- **Suggested change:** in `check`, compare `git config core.hooksPath` against
  `paths.hooks`; emit a finding (or at least a NoOp hint) when unset or mismatched, with the
  fix spelled out (`cinch init` or `git config core.hooksPath <paths.hooks>`). Skip cleanly when
  not a git repo or when `paths.hooks` is unset.
- **Resolved in `b3cb15d`** ("feat: add hooks check to verify core.hooksPath is active"): a new
  `hooks` check (`internal/cinch/hooks.go`) compares `git config
  --get core.hooksPath` against the resolved `paths.hooks` value and reports a NoOp
  (`hooks: skip: ...`) naming both fixes when unset or mismatched; not-a-git-repo and
  no-`cinch.yml` skip cleanly. Non-blocking (NoOp, not a finding) so existing
  `cinch check`-gated CI does not start failing on repos that have not wired up hooks yet.

## 3. post-commit (and pre-commit) dispatch gives scripts no inputs

- **Evidence:** `commit-msg` gets the message-file path as `$1`, but pre-commit and post-commit
  get no args and no env. project_deltadocs' `scripts/post-commit-version-bump.sh` must
  re-derive the committed set via `git diff`, and `scripts/check-squash-merge-msg.sh` maintains a
  `COMMIT_EDITMSG` fallback for shims that predate the `$1` contract. Every consumer reimplements
  the same git archaeology.
- **Suggested change:** pass the message subject (or message-file path) to post-commit dispatch
  (e.g. `$2` after the event arg, or via env), and export `CINCH_EVENT=pre-commit|commit-msg|post-commit`
  to all dispatched scripts. Also document the `$1`/args contract per event in the hook help.

## 4. Dead config is invisible: `when` on commit-msg entries

- **Evidence:** `hooks.commit-msg.<name>.when` is documented as ignored (dispatch always runs),
  and project_deltadocs declares exactly that. Nothing signals the declaration is a no-op.
- **Suggested change:** during `cinch check`, warn when a `hooks.commit-msg.*.when` list is
  declared — either the entry relies on dispatch semantics, or the config should drop the `when`.

## 5. Opt-in checks are undiscoverable — `retirement.pattern` especially

- **Evidence:** both consumers leave `retirement.pattern` unset; `retirement` prints
  "opt-in, not configured" and is easy to miss. Retired-rule identity is exactly the guarantee a
  rule-ID corpus wants.
- **Suggested change:** make the NoOp lines for `retirement` and the `rules` coverage (item 1)
  one-line callouts with the key to set, so a `cinch check` run reads as a dashboard of what is
  *not* being enforced, not just what passes.

## 6. `require.cinch` mismatch message could say exactly how to fix

- **Evidence:** the `core` finding says "reinstall and re-render" but not how; consumers build
  cinch differently (`make install` in a cinch checkout; a future `~/.cinch` distribution clone
  per project_deltadocs' Makefile comment).
- **Suggested change:** keep the message but append a canonical install hint
  (e.g. "make install in the cinch checkout, then run 'cinch render' here"). Cosmetic, but the
  message is the first contact a version skew surfaces.

Also noted but deliberately **not** recommended: a `move-docs` command re-appears in
project_deltadocs' TODO as a rename-readiness wish; both consumers now derive `paths.docs` from
`cinch.yml` in their session hooks, so the rename story is mostly consumer-side.
