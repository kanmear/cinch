# project_deltadocs — purge stale pre-rebuild references from auditor seams

Status: **proposed** — not started.

Target repo: `project_deltadocs` (planned here per the "all in cinch plans dir"
decision).

## Context

The auditor-seam files — the per-harness entry points agents actually read when
running the rule-coverage audit — still carry references to cinch's **deleted,
pre-rebuild** design and to a retired make target. None of these were swept up in
the earlier "strip dangling references" commits; they survived by living in
committed-but-ignored dirs that `git status` and the sweep greps missed.

Concretely:

1. **Dead C-check numbers (old cinch).** `.claude/agents/auditor.md:15`,
   `.pi/extensions/auditor.ts:51`, `.opencode/agent/auditor.md:17` all say
   "`make check-harness` — **C8** warns every rule lacking a marker". `C8` is a
   pre-rebuild check ID (the numbered check set `C1..C15` was deleted; `PRINCIPLES.md`
   explicitly says the old design is git history). The current rules check is
   `rules` and reports *errors*, not "C8 warns".
2. **Retired make target.** `.opencode/agent/auditor.md:11` whitelists
   `"make docs-index*": allow` — but `docs-index` was retired in `a31d980`
   ("retire the local doc-index generation backfill"). It grants a command that
   no longer exists.
3. **Enforcement inconsistency.** Claude's auditor derives its Bash allow-list
   from `manifest.yml` `seams.auditor.allow` at runtime (via
   `.claude/hooks/auditor-bash-guard.py`); pi's does the same (`.pi/extensions/auditor.ts`).
   OpenCode's `.opencode/agent/auditor.md:7-14` **hardcodes** an allow-list — and
   its persona text (line 17) still claims "Your allowance list (manifest
   `seams.auditor.allow`) is enforced by this agent's permission envelope", which
   is false for opencode.

None of these are load-bearing *logic*, but they mislead the only humans/models
who run the audit, and #3 is a real enforcement-vs-declaration mismatch.

## Decision needed — how far to normalize opencode

- **A — Make opencode derive its allow-list the same way claude/pi do
  (recommended).** That means wiring the manifest-derived check into opencode's
  permission model so declaration and enforcement can't drift — mirror claude's
  guard (a small pre-`tool_call` hook reading `manifest.yml` `seams.auditor.allow`
  → `development.commands.*`), or whatever opencode's native permission
  mechanism supports. Then the persona text becomes true.
- **B — Accept the hardcode but fix it (drop `docs-index*`, sync the list to
  `seams.auditor.allow`), and edit the persona text** to say the list is
  hardcoded (not manifest-derived), so at least it stops lying. Cheaper, but
  keeps the two-source drift risk.

The rest of the per-harness files (the `C8` reference, the `docs-index` grant)
are unambiguous: fix or delete. This decision is specifically about opencode's
enforcement source.

## Step 1 — breathe the current rule language in

Replace "`C8` warns every rule lacking a marker" with the accurate current
mechanism, e.g. "`cinch check`'s `rules` check flags every unmarked rule; `cinch
ignores` lists the N/A inventory". Apply identically across
`.claude/agents/auditor.md`, `.pi/extensions/auditor.ts`, `.opencode/agent/auditor.md`
(and any other place the same sentence survives).

## Step 2 — drop the `docs-index*` grant

Remove `"make docs-index*": allow` from `.opencode/agent/auditor.md`. Also grep
for `docs-index`/`index` allowances in `.pi`/`.claude` seams and confirm none
survive (the `.claude/hooks/auditor-bash-guard.py` derives from the manifest, so
nothing to strip there unless `development.commands` still names a dead id —
check `manifest.yml` `development.commands` for any retired `docs-index`
entry).

## Step 3 — opencode enforcement (per decision A or B)

- **A:** implement the manifest-derived allow-list check for opencode; confirm a
  `make docs-index` call and a non-allowlisted `make` both get denied/filtered.
- **B:** hardcode to exactly the resolved `seams.auditor.allow` command literals,
  and edit the persona to not claim manifest derivation.

## Step 4 — sweep the same class everywhere

The analysis flagged other stale references in the same family:

-  the `C`-number mentions inside `scripts/drift-test.sh` comments (row-07-c10 …)
-  **`.agent/manifest.example.yml` (D6)** — its header still cites "checked: C3" /
   "checked: C5" (old-design check numbering) and "Foundation §1" (a deleted doc).
   It's a committed *example copy* of `manifest.yml`, so it can drift from the
   real manifest with no signal — per the direction rule, a copy with no check is
   a future stale duplicate. Decide: **delete the example** (and have the real
   `manifest.yml`'s comments carry the schema notes) or **regenerate it** from the
   real manifest at update time; either way strip the `C3/C5`/`Foundation` prose.

Grep the whole consumer:
`grep -rn "C[0-9]\|docs-index\|business/" .claude .pi .opencode .cursor scripts .agent`
and reconcile every hit against the current design.

---

## Verification

- `grep -rn "C8\|docs-index" .claude .pi .opencode` → empty (unless intentional).
- Auditor personae describe the current `rules` check, not `C8`.
- Under A: opencode auditor denies a non-allowlisted `make` — the enforcement
  matches the declaration; under B: the persona text matches the hardcode.
- The auditor can still run the full `docs-audit-coverage.md` workflow end to end
  (closure via `cinch check`/`ignores`).

### Critical files

- `project_deltadocs/.claude/agents/auditor.md`
- `project_deltadocs/.pi/extensions/auditor.ts`
- `project_deltadocs/.opencode/agent/auditor.md`
- `project_deltadocs/manifest.yml` (`seams.auditor.allow`, `development.commands`)
- `project_deltadocs/.agent/manifest.example.yml` (D6 — stale `C3/C5`/`Foundation`)
- (A only) opencode permission wiring
- `project_deltadocs/scripts/drift-test.sh` (stale comments)

### Relationship to other plans

The stale-`C-number`/`docs-index` cleanup is independent and cheap. The opencode
enforcement decision (A vs B) is a harness-internal matter, orthogonal to the
larger `harness-skill-pointers.md` model decision — but sequencing it after that
decision (delete-vs-generate) is cleaner so the seams aren't edited twice. If the
`drift-test.sh` staleness is handled by `consumer-dead-references.md` (Item 1 B/
deletion), coordinate so this and that plan don't both touch the same lines.
