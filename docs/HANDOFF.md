# P5 handoff — Auditor wired, fan-out verified-and-deferred, permissions declared with first glue piece

Status: **Phase 5's order is complete**: seams were already declared (P1), the Auditor verification
is wired (D067), Executor fan-out was verified absent and deferred (D068), and permissions are
declared with the first enforcement glue piece (D067). Remaining work is the enforcement remainder
(Claude Code glue), the anytime tracks, and O005 — all project-side or dormant. See Next session.

## Done — [P5 session]

**Auditor seam wired (D067).** `templates/check-rules.md` step 1 no longer has the model read every
domain file and build the rule list by hand — enumeration is script work (D014). The workflow now
takes the closure from the harness checkers: a new `{{commands.check-harness}}` binding declared in
both manifests (project: `make check-harness`; cinch: `./bin/cinch check`) whose C8 report names
every unmarked rule, plus `cinch ignores` for the N/A inventory. The model's job is the semantic
half the script cannot do (marked tests genuinely exercise their rules; unmarked genuinely
untested vs forgotten). No tier prose — that would duplicate C12 (D009).

**Permissions declared, C12 extended (D067).** `seams.<name>.allow` — an optional non-empty list
of `development.commands` ids, resolved like C3 — is validated by C12 in `check.go`
(check_test.go covers five cases). Allowances declared in both manifests (auditor:
check-harness/check/docs-index; doc_maintainer: check/docs-index; planner/executor: fuller sets).
A declaration gates nothing by itself — the manifest comment and the roadmap both say so.

**First glue piece (project).** `.opencode/agent/auditor.md` — the auditor seam's permission
envelope in opencode: `edit` and `task` denied, bash restricted to the checker commands (broad
patterns first — opencode evaluates the last matching rule). The `check-rules` skill entry point
routes the audit to it and fixes its stale `.agent/business/` path. Note for commit time:
`.opencode/` is root-gitignored; the skills were force-added historically, and `auditor.md` needs
the same `git add -f` (nested `.opencode/.gitignore` does not exclude it).

**Fan-out verified-and-deferred (D068).** Neither consumer harness fans out: skills are
single-slot, `execute-plan.md` runs one atomic task per session, no sub-agent config in
`.opencode.json` or `.claude/settings.json`. D020's multi-slot deferral stands; re-verify when a
consumer actually gains fan-out.

**Re-render + verification (D033).** Both consumers re-rendered and C2-green. cinch self-check
exit 0 with the two known C11 warns plus the designed C10 warn on the session's own
check.go/templates edits — answer: no harness rule changed (HARNESS-002's "Enforced: C12" still
holds), same resolution as P4.2/P4.3. Project `make check-harness` exit 0 with the two designed
C6/C7 warns. `make test` green including TestCheckSeamsAllow.

**Decisions logged:** D067 (closure-by-script + allow schema + first glue piece), D068
(fan-out verified-and-deferred), E010 (session event). Roadmap §P5 bullets and order line
annotated.

## Not done

- **Claude Code enforcement glue** — the roadmap's per-harness enforcement "must be built" has
  its opencode half; the Claude Code half (hook permissions / deny rules consulting the seam
  allowances) does not exist yet.
- **O005** (org vs personal repo) — still open; dormant until the repo is pushed.
- **Anytime tracks** — session-start signals are live (the SessionStart hook); episodic memory
  (`.agent/events/*.jsonl`) does not exist yet.
- **Stale skill paths** — `.agent/business/` in the remaining project skills (`rules`,
  `plan-feature`, `fix-bug`, `optimize-docs`, `sync-docs`) in both harnesses still points at the
  pre-P4 name; only `check-rules` was fixed. Glue cleanup, not script-caught.

## Next session

Two independent candidates, both project-side:

1. **Claude Code seam enforcement** — the permissions remainder: per-seam allowances enforced via
   Claude Code hook permissions or deny rules. Load the project manifest's `seams.*.allow` data;
   the auditor agent's envelope in `.opencode/agent/auditor.md` is the shape precedent.
2. **Episodic memory anytime track** — append-only `.agent/events/*.jsonl` per the roadmap's
   anytime section and `docs/decisions-log.md`.

O005 becomes actionable whenever cinch is pushed (needs a GitHub org-vs-personal call and blocks
only P0.1). The stale-skill-path cleanup is a ten-minute glue pass.

## Notes for future work

- **The self-check has now asked the session's own question three times** (check.go in P4.2,
  template edits in P4.3, check.go+templates in P5) — every time the designed answer was "no rule
  changed", resolved in the same change. The cheap fix path still holds.
- **The allowance list is data with a resolution contract (C12), not an enforcement surface.**
  Adding a command id to a seam's `allow:` changes nothing until the harness glue consumes it —
  the roadmap's "declaration is not enforcement" is a live property, not a caveat.
- **opencode's permission objects are last-match-wins**: broad patterns first, narrow last. The
  auditor agent's bash rules are ordered accordingly; a future edit that reorders them silently
  flips the envelope.
- **`.opencode/` is root-gitignored but partially tracked** (skills force-added in an old
  commit). New glue files there need `git add -f` at commit time or they silently never ship.
