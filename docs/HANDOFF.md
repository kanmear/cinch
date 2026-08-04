# P4 handoff — admission test and deletion mandate landed, first-run prune executed

Status: **Phase 4 is complete.** All four P4 exit items are live on both consumers: rule IDs and
markers (4.1, C8), the diff-coupling warning (4.2, C10), the derivability admission test (4.3),
and optimize-docs' deletion mandate (4.4) — which deleted something on its first run. The
roadmap's P4 exit line is fully annotated. Remaining work is Phase 5, plus O005. See Next session.

## Done — [P4.3/4.4 session]

**Decision first (D066)** — the gate is two questions and the pass removes. D012 names the gate
and the residue but not where residue content goes or what survives a retroactive prune; D066
settles both: recoverable-from-source → not written, point at source; not recoverable but
falsifiable → business rule, `domain/` doc with a rule ID (C8/C10-guarded); otherwise → prose.
The pruning pass carves out `domain/` rule docs (the guarded residue) and generated workflows
(C2-owned), and a removal must be spot-checked recoverable from source before committing.

**Authoring (cinch templates)**

- `templates/doc-philosophy.md` — "The Admission Test" added at the head of the doc (after The
  Core Insight, before the principles): the two-question test above, stated as a write-path
  procedure. Principles and the ❌/✅ teaching lists unchanged.
- `templates/optimize-docs.md` — new step 2 "Prune first — the admission test applied
  retroactively": gate-failing content is **removed, not trimmed, not refreshed**; the report
  names removals and confirms recoverability; the fix step makes removal the default for
  gate-failing content (trim/cross-reference only for pass-the-gate redundancy).

**Re-render + verification (D033)**

- Both consumers re-rendered and C2-green. cinch self-check exit 0 with the two known C11 warns;
  the C10 warn it fired on the session's own uncommitted template edits is the loop working as
  designed — no harness rule changed, no harness.md update needed. Project `make check-harness`
  exit 0 with the two designed C6/C7 warns. `make test` green including TestTemplatePurity.

**First-run deletion (project side)**

- `.agent/analysis/v4-flash-review.md` removed: a dated point-in-time review of the pre-P3
  `.agent/` system, unindexed (`analysis/` is index-skipped) and unreferenced by any doc, every
  finding since superseded by the phase history (guard hole → cinch pre-commit guard, rule
  citations → C8 IDs, Claude Code flavor → generic templates, index → cinch index). Its reasoning
  survives in git history and decisions.jsonl (E001–E008). `analysis/` is gone; index unchanged;
  check-harness still exit 0. The roadmap's P4 exit bar — "`optimize-docs` has deleted something
  on first run" — is met.
- **Decisions logged:** D066 (gate shape, residue routing, carve-outs, recoverability
  spot-check), E009 (session event). Roadmap §P4.3/4.4 bullets and the P4 exit line annotated.

## Not done

- **O005** (org vs personal repo) — still open; dormant until the repo is pushed.

## Next session

The roadmap's session-loads row for Phase 5:

> roadmap §P5 · `manifest.yml` · workflow list · D014 D015 — seams, tiering, permissions. Needs
> P1, P2. Order within: declare seams → wire the Auditor verification → Executor fan-out →
> permissions.

Phase 5 is **[both]** and benefits from P3, which is complete: the Auditor seam was already
changed by C8's mechanized enumeration (D014), doc-maintainer narrows to judgment, and fan-out
may arrive before the phase is formally reached (D039). The anytime tracks (session-start
signals, episodic memory) are independent project-side work. P5 needs the workflow list and
`manifest.yml` — the seams section already exists in the project's manifest (C12-checked); the
work is declaration-to-enforcement, and permissions enforcement is per-harness glue.

## Notes for future work

- **C10's boundary is `git diff HEAD`**, so its scope is the working set, not history — a clean
  committed tree is always silent, and `cinch check` is unchanged as a CI gate. Untracked files
  stay invisible until staged; the extension point if it ever bites is
  `git ls-files --others --exclude-standard`.
- **The self-check has now asked the session's own question twice** (check.go in P4.2, template
  edits in P4.3) — both times the designed answer was "no rule changed", resolved in the same
  change. The cheap fix path holds; a rule change would be the expensive-but-correct case.
- **Stale artifacts hide in the unindexed dirs.** `analysis/` held the first-run deletion;
  `plans/` was spared because its files conform to the convention (Status: open/complete). An
  optimize-docs run should look there first — C1/C5/C11 give no signal about those files.
- **The admission gate is a write-time procedure and a prune-time verdict**, not a checker: the
  question "can an agent recover this?" is judgment, so it stays prose in the templates (D058's
  judgment-residue principle) and is enforced by the write path and the optimize-docs run, never
  by a C-check.
