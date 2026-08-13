# cinch — pass the commit message path to `hooks.commit-msg.*` entries

Status: **proposed** — not started.

## Context

`cmdHookCommitMsg` (`internal/cinch/hook.go:49-71`) receives `args[0]` — the
message-file path git hands the `commit-msg` hook — and forwards it to
`CmdCheck(msgFile)` for the `commit`/`coupling` checks. But the *registered*
extension seam (`hooks.commit-msg.*` manifest entries) gets **no** such path:
`dispatchHooks` → `runHookCommand` runs each command as a bare `sh -c` with no
arguments (`hook.go:81-127`).

The consequence surfaced in `project_deltadocs`' `scripts/check-squash-merge-msg.sh`
(doc comment, lines 15-16): it can't receive the in-progress message path, so it
re-derives it by reaching into git internals —

```sh
# Reconstruct it the same way git wrote it: $GIT_DIR/COMMIT_EDITMSG
msg=$(head -n 1 "$git_dir/COMMIT_EDITMSG")
```

That works but is a workaround: the seam should hand the script the same input
cinch's own checks already receive. `COMMIT_EDITMSG` also carries a trailing
blank line and isn't the canonical interface git exposes to hooks (the file
*argument* is).

**Explicitly out of scope:** changing the `commit-msg` `when`-is-ignored rule
(that's a separate, deliberate design point), and branch-aware commit
conventions (see `commit-convention-decision.md`).

## Decision needed — how the path is handed to the script

Option A — **append as `$1`** (recommended). `runHookCommand` gains an optional
`args []string` appended to the `sh -c` command, so `hooks.commit-msg.<name>.run`
scripts read the path from `$1`, mirroring git's own hook contract and cinch's
own `CmdCheck(msgFile)`.

- Pro: minimal, matches git semantics, zero ambiguity.
- Con: a script currently ignoring args (`$0`-independent) keeps working; a
  script that authors positionally has `$1` available.

Option B — **environment variable** (e.g. `.out` the path in an env var like
`CINCH_MSG_FILE`). Con: adds a secret-ish global contract only the seam sees,
less discoverable than git's own positional convention.

Recommend **A**, and update the consumer's script to prefer `$1` when non-empty,
falling back to `$GIT_DIR/COMMIT_EDITMSG` only for backwards-compat with an
already-generated shim.

---

## Step 1 — cinch: pass args through `dispatchHooks`/`runHookCommand`

`internal/cinch/hook.go`:

- `runHookCommand(root, command string, args ...string) error` — build
  `exec.Command("sh", "-c", command, args...)`. Note `sh -c` treats the first
  post-command arg as `$0`, so to make the path land in `$1` the command's
  first `$0` slot must be a placeholder. Simplest correct shape:

  ```go
  cmd := exec.Command("sh", "-c", command+" \"$1\"", "hook", args[0])
  ```

  only when `len(args) == 1`. `pre-commit` keeps calling `runHookCommand(root,
  cmd)` with no args (unchanged behavior).
- `dispatchHooks(root, m, event, staged, args ...string)` — forward `args`
  through to `runHookCommand`.
- `cmdHookCommitMsg` — pass `msgFile` as the arg for the `commit-msg` dispatch.

`header()`/shims are unaffected (the dispatch table is read at runtime, so no
re-render; shims stay byte-stable).

### Tests — `hook_test.go`

- `TestHookCommitMsg_PassesMessagePathToEntry` — temp git repo, register
  `hooks.commit-msg.dump.run = printf '%s' "$1" > /tmp/msgpath` (or assert via a
  script writing its `$1` to a file), invoke `CmdHook(root, "commit-msg", []string{msgfile})`,
  assert the file's contents equal the path.
- `TestHookPreCommit_PassesNoArgs` — assert a pre-commit entry receives the
  same behavior as today (no `$1`), so nothing regresses.

---

## Step 2 — consumer: use `$1` in `check-squash-merge-msg.sh`

`project_deltadocs/scripts/check-squash-merge-msg.sh`:

```sh
msgfile="${1:-}"
if [ -n "$msgfile" ]; then
	msg=$(head -n 1 "$msgfile")
else
	git_dir=$(git rev-parse --git-dir)
	msg=$(head -n 1 "$git_dir/COMMIT_EDITMSG")
fi
```

Preserves the pre-patch behavior for a shim that predates this change's
re-render.

---

## Verification

1. `make test` in the cinch repo — new + full suite green; the new tests
   hand-verified as genuine (revert the `$1` wiring, fixture fails).
2. In `project_deltadocs`: squash-merge onto a scratch `dev` with a bad message
   → blocked; a good `merge: feature/…` → passes; a normal commit on a feature
   branch with `SQUASH_MSG` absent → untouched.
3. README: document the `$1` contract under `hooks.commit-msg.*` briefly.

### Critical files

- `internal/cinch/hook.go`
- `internal/cinch/hook_test.go`
- `project_deltadocs/scripts/check-squash-merge-msg.sh`
