# cinch — per-harness skill pointers: decide the model, then fix the drift

Status: **proposed** — not started.

## Context

`project_deltadocs` chose the slash-command affordance cinch's README explicitly
argues against ("Commands instead of per-harness skills or a persisted index").
It maintains **four** harnesses × seven workflows = 28 thin `SKILL.md`/`SKILL.md`
pointer files that each just say "read `.agent/workflows/<name>.md`". The
analysis found these have drifted:

- `.cursor/skills/` and `.opencode/skills/` are **missing `fix-bug`** (claude/pi
  have it) — AGENTS.md:49-51 promises parity that doesn't exist.
- `.cursor/skills/{rules,check-rules}` and `.pi/skills/{rules,check-rules}`
  still reference **`.agent/business/`** (renamed to `.agent/domain/`) — a path
  that points nowhere.
- There is **no check** that a workflow's harness pointers exist or reference a
  live path — so this drifted silently.

This is the cost of the per-harness-skill model, materialized. The plan is in two
parts: **(1) pick the model** (the decision), **(2) fix the drift** (small and
deferred only on decision). The fix genuinely depends on the decision — don't
ship the fix first.

## Decision needed — the model (this is the judgment call)

Three coherent options. Decide, don't default.

### Option A — Delete the pointers; rely on `cinch workflows`/`cinch workflow NAME`
The README's own position. `project_deltadocs`' AGENTS.md:46 already advertises
the commands; the slash-command affordance (`/plan-feature` etc.) is the only
thing lost. Requires deleting ~24 gitignored-but-some-tracked files, editing
AGENTS.md prose ("same-named slash command across…"), and — important — fixing
the `.gitignore`/tracked mismatch (below) so the repo stops ignoring dirs it
committed. Lowest ongoing cost; zero cinch change. But drops the harness-native
UX the consumer deliberately adopted.

### Option B — Keep pointers; generate them as `cinch render` outputs
cinch owns them → the `generated` check (byte-sha) verifies them, tamper-evident,
exactly like hook shims. Real fix, but:
- grows the schema — a new render target (principle 5 risk: "shape is the real
  variance"), and a new manifest knob (e.g. which harnesses to emit).
- the per-harness *content* is mostly boilerplate ("read this file"), which is
  exactly the duplication cinch's README calls "all of it duplicated boilerplate
  that only ever says read this file".
- Alternates a stability / version-skew concern: it becomes another generated
  artifact a consumer must render after upgrade.

### Option C — Keep pointers; add a verifier
A light check that each shipped workflow has a pointer in each harness and that
each pointer's referenced path exists under `paths.docs`. Cheapest *retaining*
option — but it's a **proxy metric**: "a pointer exists" doesn't mean the pointer
is correct or load-bearing, and it enshrines the very duplication the design
rejects. Principle 6 argues strongly against this.

Recommendation leans **A** (aligned, cheapest long-term, matches the README
design) or **B** if the slash-command affordance is genuinely load-bearing for
the user's workflow. C is the option to argue *against*. The user should decide
A vs B; this plan is written to be executable under either.

---

## Step 1 — fix the tracked/ignored mismatch (do regardless of model)

`project_deltadocs` root `.gitignore` lists `.claude/`, `.cursor/`, `.opencode/`,
`.pi` — yet 35 files under them are tracked (committed with `-f` historically).
`git check-ignore -v .cursor/skills/newthing/SKILL.md` confirms **any *new* file
under those dirs is silently ignored** — which is exactly why `fix-bug` is
missing from cursor/opencode: it could never be committed without `-f`, and
`git status` never surfaced it.

Fix: remove those four entries from `.gitignore` (they're actively tracked, so
ignoring them is wrong); keep only genuinely-machine-local ignores (e.g.
`.claude/settings.local.json`, `.pi-lens.json` is tracked? verify). After
removal, `git status` should show the real gaps. **This step must land
regardless of A/B/C** — it's a latent trap either way.

## Step 2 — under A: delete and re-point

- `git rm` the 28 (or subset) skill files; remove the four `.gitignore` entries
  (overlaps Step 1 — fold together).
- AGENTS.md:49-51: drop the "same-named slash command across…" sentence; point
  squarely at `cinch workflows` / `cinch workflow <name>` (which AGENTS.md:46
  already does). Ensure `docs-philosophy`/`dev-task-primitive` reference-only
  wording survives.
- Verify `.githooks`/harness session-start hooks (`.claude/hooks/inject-agents.sh`)
  don't depend on the skill files existing.

## Step 2' — under B: generate

- cinch `render.go`: emit per-harness pointer files under a new manifest-scoped
  path (e.g. `paths.skills`), each with a generated header + body-sha so
  `checkGenerated` covers them. Freed of duplication boilerplate via the one
  dictating source (workflow name + harness name).
- New manifest knob (absence-selected): which harnesses to emit; default emit
  the ones present. Principle-5 mitigation to document: the *shape* (flat
  filename → one-line body) is fixed; only the set varies.
- `project_deltadocs`: adopt, re-render, re-check; add the missing `fix-bug`
  pointers across cursor/opencode via the generator (they become render
  outputs, so "missing" stops being a hand-editing question).

## Step 3 — sweep and close

- `grep -rn 'business/' .cursor .claude .opencode .pi` must return nothing after
  the fix (today: cursor+pi stale-path references alive).
- Confirm `cinch workflows` still lists 9 and AGENTS.md commands match.

---

## Verification

- A: no harness skill files tracked; `git status` clean after deletion;
  `grep business/` empty; citations in AGENTS.md resolve.
- B: `cinch render` twice → byte-identical; edit one generated pointer → `cinch
  check` fails (generated); the `fix-bug` gap closed by generation.
- Either: `make test` green (cinch), consumer `cinch check` clean.

### Critical files

- `project_deltadocs/.gitignore`
- `project_deltadocs/AGENTS.md`
- `project_deltadocs/.cursor/skills/*`, `.pi/skills/*` (stale `.agent/business/`)
- cinch `internal/cinch/render.go` + `manifest.go` (B only)

### Relationship to other plans

The **tracked/ignored fix (Step 1)** is independent of the model and could be a
small standalone change; the model decision gates the rest. Do **not** run the
fix under "delete" while the user is leaning "generate" — this is the plan the
decision actually gates.
