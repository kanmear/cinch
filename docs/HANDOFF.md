# P5.6 handoff — the last unverified path closes: project-side interactive audit ran, PERM-004 closed, step 3 hardened

Status: the P5.5 handoff's remaining candidate executed. The check-rules audit ran *interactively*
in the Claude Code consumer (project_deltadocs, fresh session, auditor agent) — full closure →
classification → report → proposal → executor-apply. The one real gap (PERM-004) is closed with
four route tests; the audit's one misfire (declaring PERM-004 unmarked under a zero-unmarked
closure) became a workflow hardening (D073). Phase 5 is complete: the seam has a live workout in
every consumer it was built for.

## Done — [P5.6 session]

**The interactive Claude Code audit (E017).** Fresh `claude` session in project_deltadocs with the
auditor agent selected; `/check-rules` invoked. Enforcement proven live project-side: the
manifest-derived PreToolUse guard auto-allowed `make check-harness` / `make ignores`, and a probe
denial (tail on the events log) blocked. Closure: zero C8 findings, `cinch ignores` = CAT-003 +
MEMB-006. Semantic half: 64 of 65 testable rules confirmed enforced by their marked tests;
**PERM-004** — "sign grants read but not edit" — was under-enforced: the read-access clause had no
test at all, and the rule's marker sat above a resolver-mapping test
(`TestSignOnlyPermission_CanSignButNotEdit` in `actionbar_states_test.go`), not route-level
enforcement.

**PERM-004 closed (executor).** Four route tests added to
`backend/tests/handlers/tab_permission_test.go` mirroring the existing View-* patterns:
`TestTabPermission_GetByID_SignAllowed` (200 — the missing read-access half),
`TestTabPermission_Update_SignForbidden` (403 — marker's new home),
`TestTabPermission_Delete_SignForbidden` (403), `TestTabPermission_Create_SignCategoryForbidden`
(403). Marker moved off `actionbar_states_test.go` (one marker per rule — the D072 precedent).
`make test-backend` green; the four new tests pass individually.

**C8 wiring confirmed correct (D073).** The report's "is C8 fully wired for permissions.md?" note
resolved as marker-outside-file-set: the PERM-004 marker existed but lived in a UI-state test file
outside `permissions.md`'s `owns:` paths, so the owns-derived semantic pass never read it. C8's
zero-unmarked output was true. The workflow's step 3 now instructs the semantic half to trust the
closure: a gap under a zero-unmarked closure is by definition not a missing marker — search the
tree for the rule's marker and judge marker quality instead of reporting unmarked.
`templates/check-rules.md` amended; re-rendered into both consumers, C2 green.

**Verification.** Backend `go test` green (handlers/models/database). Project `cinch check` exit 0
(two known C6/C7 warns + two designed C10 questions on the uncommitted diff — categories/signatures
owns: paths touched, no rule-text change; clear on commit). cinch self-check exit 0 (two known C11
warns + one designed C10 question on templates/check-rules.md under harness owns: — the amendment
documents procedure, no harness rule text changed; clears on commit). No C8 findings either side.
E017/D073 logged in `decisions.jsonl`; project event E003 logged in its events log; roadmap P5
status line refreshed.

## Not done

- **O005** (org vs personal repo) — dormant until the repo is pushed.
- **D020 reconsideration** — no fan-out exists in either consumer; nothing to decide.
- **Envelope asymmetry** — opencode patterns are static; the Claude Code guard stays
  tighter (chaining/expansion rejection). cinch's own envelope carries the same
  asymmetry (D071). Revisit when a third harness appears.

## Next session

Nothing is blocked. Candidates:

1. **Commit the open work**: project_deltadocs (3 files: the two test files + the re-rendered
   `check-rules.md`) and cinch (template, rendered workflow, decisions, roadmap, handoff) —
   both are uncommitted; the designed C10 questions clear on commit.
2. **O005 / D020** — both still dormant by design (repo push; real fan-out).
3. **The C11 baseline** (optional) — the two known warns' template-side source
   (`templates/sync-docs.md` routing-table rows hardcoding `.agent/frontend/conventions.md` and
   `.agent/backend/troubleshooting.md`) could be genericized to `[service]/…` placeholders; still
   deliberately not scheduled.

## Notes for future work

- **The semantic half has a blind spot the mechanical half doesn't: `owns:`-derived file sets.**
  A marker can live in a test file the domain doc doesn't own (PERM-004's sat in a UI-state file).
  The step-3 amendment (D073) makes the closure authoritative when the two disagree — but the
  practical lesson stands: an auditor reporting a gap should search the tree for the marker before
  naming it unmarked. The checker never misses a rule; a hand extraction can (D067).
- **C10's designed questions fired on both consumers' uncommitted diffs** — the warns are the
  "ask when the answer is cheapest" behavior, the answers were both no-ops (test-only change /
  procedure-doc change), and all clears on commit. Not defects.
- **The self-audit loop is now complete in every consumer**: cinch envelope (E014 → D072, E016
  clean), project opencode/Claude Code seams (E017 → PERM-004 closed, E003). The proposal→apply
  cycle has two full revolutions and one clean-run to its name.
- **The D067 binding pattern still applies to cinch's own seam.** Every command a workflow
  instructs the auditor to run must exist as a manifest command, a Makefile target, and an
  envelope allowance — any future script-instructed subcommand needs its allow pattern added to
  `.opencode/agent/auditor.md` in the same change.
- **The known C11 warns have a template-side source.** `templates/sync-docs.md`'s routing table
  hardcodes `.agent/frontend/conventions.md` and `.agent/backend/troubleshooting.md` (the only
  stack-shaped rows; neighbors use `[service]/…` placeholders). Genericizing them would clear both
  warns; deliberately not scheduled — the baseline is accepted. If the `fix-bug-devservers` /
  `task-primitive-troubleshooting` fragments ever render, expect two more C11 warns with the same
  shape.
- **The guard's auto-allow output contract** (project-side, Claude Code): verified working on
  v2.1.221 with `hookSpecificOutput.hookEventName: "PreToolUse"` wrapping `permissionDecision`;
  top-level `permissionDecision` is deprecated and silently ignored. The interactive run used a
  modern client; a much older `claude` would deny instead of auto-allow (still a valid
  enforcement result).
- **`.claude/` is root-gitignored but partially tracked** in project_deltadocs (historical
  `git add -f`), same as `.opencode/` there. cinch's `.gitignore` only ignores `bin/`, so its
  `.opencode/` files track normally.
- **The self-check pattern held again**: this session's decision + event + handoff + roadmap edits
  produced no C10 self-question (`docs/` and `decisions.jsonl` are not `owns:` paths), and the
  self-check stayed exit 0 with the two known C11 warns.
