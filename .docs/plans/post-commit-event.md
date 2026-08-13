# cinch — `post-commit` event (dispatch table + shim + consumer adoption)

Status: **proposed** — not started.

## Context

cinch's extension seam dispatches exactly two events: `pre-commit` and
`commit-msg` (`internal/cinch/hook.go:16-25`). `project_deltadocs` — cinch's
first real consumer — still hand-writes the third hook its workflow depends on:
`.githooks/post-commit` is a 2.3 KB bespoke shell script that auto-bumps the
per-service version fields and amends the triggering commit (lock file +
reentry guard + `git commit --amend --no-edit --no-verify`). It's documented in
`.agent/git-conventions.md` but sits entirely outside cinch: no shim, no
manifest dispatch, no tamper-evident header, no `generated`-check coverage.

Everything else in that repo's hook layer migrated behind cinch; `post-commit`
is the one holdout — the exact "paying rent for a gap in the tool" pattern
`cinch-0.1.0.md` describes.

## Why `post-commit` is a good fit for the seam

The version-bump *logic* is app-specific (which field, amend, reentry guard) and
stays in the registered script — that's fine, the extension boundary says cinch
guarantees *dispatch*, not correctness. What cinch should own is exactly what it
owns for the other events: the shim, the manifest `hooks.post-commit.*`
dispatch, and tamper-evidence via the `generated` check.

## Decision needed — dispatch semantics for `post-commit`

`post-commit` runs *after* the commit, so there is no staged set. Two concerns:

1. **`when` scoping.** `project_deltadocs`' bump only fires when the new commit
   touched `frontend/` or `backend/`. Two options:
   - **A. `when`-scope on the committed paths** (recommended): compute the
     changed set from `git diff --name-only HEAD^ HEAD` (or the current commit
     for a root commit) and run `when`-prefix matching with the *patched* set —
     same semantics as pre-commit, but against committed files. This lets the
     bump logic drop its own path triage and keep only *bump-type* logic
     (branch → minor/patch/hotfix, amend, guard).
   - **B. `when`-ignored, run every entry** (mirrors `commit-msg`): the script
     keeps its own path triage. Simpler, but leaves the seam doing less than it
     could and duplicates path logic in a repo that already expresses it in the
     manifest.
   Recommend **A**, treating "committed set" as the natural `post-commit`
   analogue to pre-commit's "staged set". Note: for a `--amend` (which
   re-triggers `post-commit`), `HEAD^ HEAD` is correct — the amend's parent is
   the pre-amend commit, so it still diffs the *payload* change. Verify this
   explicitly in the mutation fixture.

2. **Reentry/amend loop.** The script amends the commit, re-firing `post-commit`.
   That guard is *correctness* of the script, not dispatch — it must stay in the
   script (lock file), not be baked into cinch. State this in the README
   extension-boundary note: cinch runs the entry once per event; a script that
   re-triggers its own event owns the loop guard.

---

## Step 1 — cinch: `post-commit` in the event table + committed-set dispatch

`internal/cinch/hook.go`:

```go
case "post-commit":
	staged, err := gitOutputLines(root, "diff", "--name-only", "HEAD^", "HEAD")
	// fall back to `git diff-tree --no-commit-id --name-only -r HEAD` for a root commit,
	// i.e. when `git rev-parse HEAD^` fails (no parent) — a root commit changes everything in HEAD.
	ok := dispatchHooks(root, m, "post-commit", committed) // `when`-prefix match, same as pre-commit
```

`CmdHook` switches `"post-commit"` to run `dispatchHooks` only (no `CmdCheck` —
checks already ran on pre-commit; re-running `generated`/`links` here is noise
and git runs post-commit for *any* commit path, including ones pre-commit
sandboxed). Unknown event still exits 2.

`render.go`: add `"post-commit"` to the supported-events set so a shim is
generated at `<paths.hooks>/post-commit` with `exec cinch hook post-commit "$@"`
— byte-stable, read-at-runtime dispatch, exactly like the other two shims.

### Tests — `hook_test.go`

- `TestHookPostCommit_DispatchesByCommittedWhen` — temp repo, commit under
  `src/`, register `hooks.post-commit.dump.run = touch /tmp/f`, `when = src/`;
  run `CmdHook(root, "post-commit", nil)`; assert runs. Same repo, commit under
  `docs/`, same harness → assert skipped (announced on stderr).
- `TestHookPostCommit_AmendStillScopes` — commit payload under `src/`, then
  amend (touching only a message or a separate file), assert the committed set
  `HEAD^ HEAD` still includes `src/…`. Guards decision concern 1.
- `TestHookPostCommit_RootCommitFallsBackToTree` — repo with a single root
  commit changing `src/`; assert entry runs despite no `HEAD^`.

---

## Step 2 — consumer: migrate the version bump behind the seam

`project_deltadocs`:

1. Move the bump logic's *executable* body out of `.githooks/post-commit` into
   `scripts/post-commit-version-bump.sh` (or keep `scripts/update-version.js`
   orchestration there), retaining: branch→bump-type map, amend, lock-file
   reentry guard, per-service path triage (now redundant with `when`, can
   simplify or keep defensively).
2. Register in `cinch.yml`:

   ```yaml
   hooks:
     post-commit:
       version-bump:
         run: scripts/post-commit-version-bump.sh
         when: [frontend/, backend/]
   ```

3. Run `cinch render` to regenerate the `post-commit` shim (replacing the
   hand-written file), and delete the old bespoke `.githooks/post-commit`.

Behavioral caveat to confirm in rehearsal: the amend re-fires `post-commit`
through the shim → script's lock guard must still gate. The `when`-scope now
fires on the amend's `HEAD^ HEAD` too, so the guard inside the script remains
the single source of truth for "did I already bump".

---

## Verification

1. cinch repo: `make test` green; new fixtures hand-verified genuine (revert
   committed-set fallback → root-commit fixture fails).
2. `project_deltadocs` scratch branch (never pushed):
   - `make setup-hooks`/`cinch render` → `.githooks/post-commit` now a cinch
     shim with header + body-sha.
   - squash-merge a `fix/…` branch touching `backend/` → `backend/version/patch.go`
     bumps (auto via hook), frontend untouched.
   - commit under only `.agent/` → no bump (when-scope, no `$SQUASH` needed).
   - `make release` path (2-parent merge) → still skipped.
3. README extension-boundary note: `post-commit` dispatch + reentry ownership.

### Critical files

- `internal/cinch/hook.go`
- `internal/cinch/render.go`
- `internal/cinch/hook_test.go`
- `project_deltadocs/cinch.yml`
- `project_deltadocs/.githooks/post-commit` (delete)
- `project_deltadocs/scripts/post-commit-version-bump.sh` (new)
- `project_deltadocs/.agent/git-conventions.md` (update)

### Relationship to other plans

- Depends on nothing. **Adjacent** to `hook-commit-msg-args.md` (both touch
  `dispatchHooks` signature) — sequence them so the args change lands first or
  fold the diff-scoping into the same `dispatchHooks` refactor.
