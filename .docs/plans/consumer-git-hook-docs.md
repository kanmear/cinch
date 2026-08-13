# project_deltadocs — fix stale hook documentation in `git-conventions.md`

Status: **proposed** — not started.

Target repo: `project_deltadocs` (planned here per the "all in cinch plans dir"
decision).

## Context

`.agent/git-conventions.md` describes the pre-cinch hand-written hooks — but the
hooks migrated behind cinch (`.githooks/pre-commit` is now a 3-line generated
shim, dispatch via `cinch.yml`), and the doc wasn't updated. The stale claims:

1. **git-conventions.md:21-27** — pre-commit "Prepends common user-level tool
   install locations (`~/.bun/bin`, …) to `PATH` …". That logic moved to the
   Makefile `export PATH` (Makefile:8); the hook itself no longer does it.
2. **git-conventions.md:24-27** — "Skips entirely during a squash-merge
   (detected via `SQUASH_MSG` …) so the source branch's tip commit already
   passed". cinch's dispatch (`hook.go:27-47`) has no `SQUASH_MSG` skip — it runs
   `CmdCheck` then the `when`-scoped entries unconditionally on a pre-commit.
3. **git-conventions.md:19** — "a docs/skills-only commit runs no checks at all".
   False: `cinch check` always runs its five checks as part of pre-commit; only
   the `when`-scoped make targets are skipped. A docs-only commit still gets
   links/rules/coupling/generated.
4. **git-conventions.md:29,43,53** — "Source: `.githooks/pre-commit`" etc. — the
   hooks are now generated shims, not sourced hand-written files; their behavior
   lives in `cinch.yml` + `scripts/`.
5. **git-conventions.md:31-43** (commit-msg) describes the squash-format check,
   which is now a cinch manifest entry running `scripts/check-squash-merge-msg.sh`
   — the prose is roughly right but the "Source" and the mechanism (manifest, not
   `.githooks/commit-msg` directly) aren't stated.

This is the exact **doc-drift cinch exists to catch** — but cinch can't, because
it's prose describing hook *behavior*, not a scanned rule/link. The fix is
human, deliberate, and cheap.

## Decision needed — how much the doc should now say

The doc has a real role (explaining *why* the version-bump scheme works), but the
hooks section is drifting lower-level detail. Decide the target shape:

- **A — Rewrite the hooks section to describe the manifest, not the files.**
  State: hooks are generated cinch shims; `pre-commit`/`commit-msg`/`post-commit`
  dispatch via `cinch.yml`; `when` scoping is prefix matching on staged/committed
  paths; PATH prepend lives in the Makefile; the squash handling is in
  `scripts/check-squash-merge-msg.sh` driven by the commit-msg entry. Accurate,
  survives future drift by pointing at the manifest as the single source.
- **B — Delete the hooks prose, keep only the versioning scheme.** The level of
  detail "99% of tasks won't need" (its own framing) — drop to a pointer to
  `cinch.yml` + the scripts, keep the ## Post-commit — Versioning scheme which is
  genuinely load-bearing. Less to go stale.

Recommend **A** — the version-bump scheme's *behavioral* coupling to
`commit-msg` (SQUASH_MSG → bump type → service) is real and belongs documented;
just re-home it to the manifest reality.

## Step 1 — correct the factual claims

- PATH prepend → "handled in the Makefile (`export PATH`), not the hook".
- Remove/replace the "skips entirely during a squash-merge" and "docs/skills-only
  runs no checks" sentences with the accurate behavior: pre-commit runs `cinch
  check` (its five checks) always, then `when`-scoped make targets only when a
  matching prefix is staged.
- Reword "Source: `.githooks/pre-commit`" → "Generated shim; dispatch in
  `cinch.yml` (`hooks.pre-commit.*`)".
- commit-msg section: note it's a cinch manifest entry → `scripts/check-squash-merge-msg.sh`.

## Step 2 (per decision A) — re-home the mechanism language

Make the manifest the single source of truth the doc refers to. Optionally add a
line pointing readers to `cinch workflows`/`cinch check` for the live state
rather than the hand-maintained table.

## Step 3 — cross-check against the other hook docs

`.agent/conventions.md:5-36` (Git section) also describes the hooks and the
merge strategy. Grep both docs for stale `.githooks/pre-commit` descriptions and
bring them into agreement (they may still say the hand-written hook mounts PATH
or skips squash).

---

## Verification

- `grep -rn "Skips entirely\|Prepends common\|runs no checks at all" .agent/`
  returns nothing (unless the docs intentionally restate philosophy).
- `cinch check` clean (links fine). No functional change — documentation only.
- Cross-referenced with `conventions.md` so the two git sections agree.

### Critical files

- `project_deltadocs/.agent/git-conventions.md`
- `project_deltadocs/.agent/conventions.md`

### Relationship to other plans

Documentation-only; independent. Should land **before or with**
`consumer-bind-check-error-codes.md` and `post-commit-event.md` (both introduce
manifest-driven hook behavior this doc must describe consistently) — sequence
the docs pass last so it describes the settled post-migration state, or run it
now against the current reality and re-touch if the post-commit plan changes the
dispatch.
