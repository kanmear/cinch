# project_deltadocs — bind `check_error_codes.sh` so its check is enforced

Status: **proposed** — not started.

Target repo: `project_deltadocs` (planned here per the "all in cinch plans dir"
decision).

## Context

`scripts/check_error_codes.sh` is a deterministic referential-integrity checker:
it validates that every frontend `@throws {ApiError} CODE` resolves to a
`backend/errors/codes.go` constant **and** has a translation key in every
locale's `common.json`. That's precisely the class of cross-file integrity cinch
exists to enforce — but today it's **manual-only**: TODO.md:60 says "Wire
`scripts/check_error_codes.sh` into `make check` / CI (currently manual-only)",
and `.agent/frontend/conventions.md:173` merely *documents* running it. A
documented check no mechanism ever runs.

cinch already gives this repo the exact seam to close it with **zero cinch
code**: a manifest-registered `hooks.pre-commit.*` entry scoped by `when`
path prefixes. Nothing else in the repo needs to change; the discipline is in the
scoping and the doc bookkeeping.

## Decision needed — where to bind it

The script checks three regions: `frontend/src/lib/api/**` (the `@throws`
source), `backend/errors/codes.go` (the constants), and the locale
`common.json` files. The `when` gate must cover all three, or a change to one
side goes unverified:

```yaml
hooks:
  pre-commit:
    error-codes:
      run: scripts/check_error_codes.sh
      when:
        - frontend/src/lib/api/
        - backend/errors/
        - frontend/src/lib/translations/
```

**Decision points:**

1. **Scope vs. always-run.** Because the `@throws` codes, the constants, and the
   translations can each change independently, a partial `when` would let a lone
   change on the unpinned side through. Three options:
   - **A (recommended): pin all three prefixes** (above). Any change touching any
     of the three surfaces re-verifies the whole cross-file contract. Slightly
     over-runs (an unrelated translation tweak re-checks codes) but the check is
     fast and deterministic.
   - **B: fold into the existing `frontend`/`backend` entries.** Riskier — a
     backend-only change wouldn't re-check the frontend `@throws` side unless the
     backend `when` also fires it, and the cross-file nature means the check
     really needs all three upstreams.
   - **C: run unconditionally (empty `when`).** Simplest to reason about; the
     check is cheap and only reads. Downside: runs on docs-only commits, which
     the repo's current pre-commit philosophy deliberately avoids ("a
     docs/skills-only commit runs no checks"). Reject unless that philosophy
     changes.
   Recommend **A**.

2. **Also wire into `make check` / CI?** TODO says "into make check / CI" — but
   the repo has no CI yet (TODO "CI pipeline — no `.github/` yet"). Recommend
   the **hook binding now** (it's the enforcement), and separately note that
   once CI lands, `make check-backend` or a dedicated `make check-error-codes`
   target should include it. Decide whether to add the Makefile target now so
   an interested session can run it on demand.

3. **Doc drift.** `.agent/frontend/conventions.md:173` says "Run
   `scripts/check_error_codes.sh`" with no hint it's enforced. Update the wording
   so it is not read as the only way to run it, and update TODO.md:60 (delete the
   "currently manual-only" item once bound). A `git-conventions.md` pre-commit
   blurb would also be accurate — decide how much of the hook inventory that doc
   should describe (see `consumer-git-hook-docs.md`).

---

## Step 1 — register the hook

Edit `cinch.yml` per option A above (decision 1). Run `cinch render` to confirm
nothing changes (the dispatch table is read at runtime — the shims are
byte-stable, so this is a pure config edit), then `cinch check` to confirm the
tree stays clean.

## Step 2 — provenance/verification

- Reproduce the check's own correctness as a fixture so it's not a proxy: with a
  made-up `@throws {ApiError} NOPE` added to a stubbed `frontend/src/lib/api`
  file, the hook must fail; with a matching constant + translation added, it must
  pass. (The script is a project-owned script — cinch guarantees dispatch, not
  the script's correctness — so the fixture validates the *binding* actually
  fires, which is the part cinch owns.)
- Confirm a docs/skills-only commit skips it (stderr shows the skip, not a silent
  pass) per the current pre-commit philosophy.

## Step 3 — docs and Makefile bookkeeping

- Update `conventions.md`/TODO.md per decision 3.
- If a `make check-error-codes` target is wanted (decision 2), add it and include
  it in `check` composition (or call it out as CI-time-only).

---

## Verification

- `cinch check` clean in the consumer.
- A scratch change introducing a dangling `@throws` blocks `git commit` (through
  the pre-commit hook); the same change with the constant + translation present
  is allowed.
- Docs-only commit → skip announced on stderr.

### Critical files

- `project_deltadocs/cinch.yml`
- `project_deltadocs/scripts/check_error_codes.sh` (unchanged, referenced)
- `project_deltadocs/.agent/frontend/conventions.md`
- `project_deltadocs/TODO.md`
- `project_deltadocs/Makefile` (optional decision-2 target)

### Relationship to other plans

Independent of cinch code — pure manifest binding. This is the highest-value,
lowest-cost consumer change from the analysis (closes a real TODO, zero cinch
work). Execute before the CI plan exists. Inconsistent only if the user later
changes the "docs-only runs no checks" philosophy, which would be decided in
`cinch-version-pin.md`'s or another plan's scope, not here.
