# project_deltadocs — stop hand-duplicating the auditor seam guard per harness

Status: **proposed** — not started.

Target repo: `project_deltadocs` (planned here per the "all in cinch plans
dir" precedent this repo's other consumer-targeted plans already use).

## Context

The "seams" pattern (see `manifest-seams-skeleton.md`) declares, in
`.agent/manifest.yml`, which `development.commands` entries the `auditor`
persona may run via Bash. Enforcement is per-harness, and today there are
two independent implementations that hand-parse the same manifest with the
same regex-based mini-YAML reader and the same allow/deny semantics:

- `.claude/hooks/auditor-bash-guard.py` (Claude Code `PreToolUse` hook)
- `.pi/extensions/auditor.ts` (Pi's `tool_call` handler)

Line-for-line correspondence:

| Concern | Python | TypeScript |
|---|---|---|
| Command-line regex | `COMMAND_LINE` (`auditor-bash-guard.py:16`) | `COMMAND_LINE` (`auditor.ts:26`) |
| Chain/expansion charset | `CHAIN` (`:14`) | `CHAIN` (`:25`) |
| Parse `development.commands` | `load_commands` (`:20-38`) | `loadCommands` (`:119-143`) |
| Parse `seams.auditor.allow` | `load_allow` (`:41-59`) | `loadAllow` (`:145-174`) |
| Allow/deny + fail-closed | `main` (`:67-102`) | `tool_call` handler (`:59-116`) |

Both files implement the identical algorithm: find the `commands:` block,
find the `seams:` → `auditor:` → `allow:` list, resolve ids to command
literals, then exact-match-or-prefix-with-space against the incoming Bash
command, deny on any chaining/expansion character in the remainder. This is
not project-specific business logic — it's harness-integration glue for a
generic pattern (declare an allowlist in a manifest, enforce it per-harness).
The duplication already caused one real drift incident this session
(`consumer-auditor-seam-staleness.md` — stale C-number references had
diverged between the two copies' surrounding comments), and every future
change to the seam semantics (a new escape hatch, a bug fix in the YAML
mini-parser, a new allowance shape) has to be applied twice, by hand, with no
automated check that the two stay behaviorally identical.

A third harness variant (a hypothetical Cursor guard, or any future harness)
would mean porting this a third time.

## Decision needed — where the single source of truth lives

- **A — Extract a shared parser/decision module both guards call.** Since the
  two languages differ (Python hook vs. Node/TS Pi extension), "shared code"
  means either (a) a tiny CLI cinch-adjacent tool both shells out to (a Node
  or Python script that takes manifest path + command string, prints
  allow/deny), or (b) accept the two languages can't literally share source
  and instead share a **behavioral parity test** that runs both against the
  same fixture manifests and same command list, failing if they disagree.
  (a) is the real fix but adds a new artifact + calling convention both
  harnesses must invoke correctly (subprocess overhead per Bash call, a new
  thing to keep on PATH). (b) is cheaper and doesn't touch the hot path, but
  doesn't eliminate the duplicate-maintenance burden — it just makes drift
  detectable instead of silent.
- **B — Pick one language as canonical, generate/vendor the other.** E.g.
  hand-maintain the Python version, and either accept the TS port stays
  hand-synced (status quo) or write a small generator that emits the TS
  version's parse functions from the Python source (fragile, over-engineered
  for ~120 lines).
- **C — Leave duplicated, document the burden.** Cheapest; explicitly punts.
  Reasonable if a third harness is unlikely and the parity test in (b) isn't
  worth the setup cost either.

Recommend **A(b)** — a parity test — as the immediate, low-cost fix: it
directly targets the actual failure mode observed (silent drift between two
copies), without introducing a new runtime dependency into either harness's
hot path. Revisit **A(a)** only if a third harness needs the same guard,
at which point the subprocess-overhead cost is worth paying to stop a third
hand-port.

## Step 1 — write the parity test

A test script/harness (language: whichever is more natural to invoke both
from — a small Python or Node test runner, or even a shell script) that:
1. Constructs a set of fixture `manifest.yml` snippets — valid, missing
   `seams`, missing `commands`, allow-list referencing a nonexistent command
   id, malformed YAML, an allowed literal followed by a chaining character.
2. Runs `load_commands`/`load_allow` from `auditor-bash-guard.py` and
   `loadCommands`/`loadAllow` from `auditor.ts` (import/exec each, both
   already structured as importable functions — see the `export function`s in
   `auditor.ts:119,145` and the plain top-level `def`s in
   `auditor-bash-guard.py:20,41`) against every fixture.
3. Asserts identical output (same commands map, same allow list, same
   None/null-vs-empty-list semantics) for every fixture, and identical
   allow/deny decisions for a shared table of (manifest, command) pairs.

Wire it into `make check-harness` (or wherever harness-level checks already
run) so drift is caught the same way any other check in this repo is caught
— not left to a human audit to rediscover.

## Step 2 — fix current drift, if the parity test finds any

Run the new test now; if it finds any behavioral divergence between the two
guards beyond what's already known, fix the two files to agree before
merging the test (a red parity test on day one, for a known-good pair of
files, defeats the point).

## Verification

- New parity test fails when one guard's fixture behavior is hand-edited to
  diverge from the other (fixture-genuineness check — temporarily break one
  side, confirm red, revert).
- `make check-harness` (or equivalent) runs the parity test and is green on
  the current (fixed, if Step 2 applied) pair of files.
- Existing auditor personas (`pi --auditor`, the Claude Code auditor agent)
  still function identically in a manual smoke test — this change adds a
  test, it does not change either guard's runtime behavior (unless Step 2
  found and fixed real drift).

### Critical files

- `project_deltadocs/.claude/hooks/auditor-bash-guard.py`
- `project_deltadocs/.pi/extensions/auditor.ts`
- new parity test file (location TBD — wherever `project_deltadocs` already
  keeps harness-level test tooling)
- `project_deltadocs/Makefile` (wiring into `make check-harness` or similar)

### Relationship to other plans

Depends conceptually on `manifest-seams-skeleton.md` (documents the pattern
these two files enforce) but is independently executable — the duplication
and its fix don't require that documentation plan to land first. If a third
harness ever needs the same guard, revisit the **A(a)** shared-module option
this plan deliberately deferred.
