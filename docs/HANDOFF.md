# P5.1 handoff — Claude Code seam enforcement landed; episodic memory track opened

Status: **both next-session candidates from the P5 handoff are done.** The Claude Code half of
per-harness seam enforcement exists (agent envelope + manifest-derived PreToolUse guard, D069) and
the episodic-memory anytime track is live in the project (`.agent/events/events.jsonl`). What
remains is all dormant or consumer-side: the `cinch ignores` binding gap (deferred, see below),
O005, and the D020 reconsideration once fan-out is real.

## Done — [P5.1 session]

**Claude Code auditor seam (D069).** `.claude/agents/auditor.md` in the project — the same envelope
shape as the opencode agent: `tools: Read, Grep, Glob, Bash` (no Edit/Write/Agent), body mirroring
the seam prompt — plus an agent-scoped `PreToolUse` hook running `.claude/hooks/auditor-bash-guard.py`.
The guard is the enforcement piece and it *consults the manifest*: it resolves `seams.auditor.allow`
ids to command literals through `development.commands` at runtime (quoted and unquoted literal
shapes, inline comments — both consumers verified), auto-allows a match only if its remainder is
free of chaining/expansion (`; | & \` $`, control chars), and exits 2 (deny) on everything else,
**failing closed** when the manifest is unreadable, the seam lacks `allow:`, or no id resolves.
Adding an id to the seam's `allow:` now widens the Claude Code seam with zero glue edits — the
manifest is the single source of truth, the handoff's "live property" is closed for this harness.

**Routing + stale-path pass.** The `.claude/skills/check-rules/SKILL.md` now routes the audit to
the auditor agent (mirroring the opencode skill) and the pre-P4 `.agent/business/` paths are gone:
`check-rules`/`rules` in `.claude/`, `rules` in `.opencode/`, and the project's AGENTS.md.
Correction to the P5 handoff: **plan-feature/fix-bug/optimize-docs/sync-docs skills were already
clean** — the pass was narrower than listed.

**Episodic memory (roadmap anytime).** `.agent/events/events.jsonl` seeded with E001 (this
session's landing) and a recording rule in the project's AGENTS.md (decisions, dead ends, accepted
risks, surprises → one append-only JSONL event; schema pointer to cinch's `docs/decisions-log.md`).
Verified index-inert: `make docs-index` regenerates the same 63-entry index.

**Verification.** Guard unit-run against both consumers' manifests (allow: the three project
checkers + cinch's `./bin/cinch check`; deny: `make test`, `git push`, `rm -rf .`, `ls`, `make
check-frontend`; chaining: `&&` / `|` after an allowed literal; fail-closed: missing manifest,
unresolvable ids). Agent frontmatter parses to the expected structure. Project `make check-harness`
exit 0 with the two designed C6/C7 warns, unchanged. **End-to-end smoke test completed** (E012):
the earlier headless `claude --agent auditor` failure was a VPN outage, not the glue; once auth
worked, the deny path blocked `ls`/`cat`/compounds live, and the allow path exposed one real bug —
the guard emitted `permissionDecision` at top level, but PreToolUse requires it inside
`hookSpecificOutput` with `hookEventName: "PreToolUse"` (top-level is deprecated). With that fix,
`make check-harness` ran prompt-free inside the auditor agent and the model interpreted the harness
output in the seam role. Verified against Claude Code v2.1.221.

**Decisions logged:** D069 (Claude Code enforcement design), E011 (session event), project E001
(event). Roadmap §P5 permissions bullet, status line, and anytime episodic-memory bullet annotated.

## Not done

- **`cinch ignores` binding gap (deferred, by design).** `check-rules.md` step 1 references
  `cinch ignores`, but no manifest command, Makefile target, or harness envelope allows it — it was
  unreachable in the opencode envelope too. The N/A inventory stays derivable by reading the
  `cinch:ignore` markers in the domain docs. Fixing it means a manifest command + Makefile target +
  template binding (`{{commands.*}}` instead of the literal) + both envelopes — a cinch-side change
  with a re-render; parked unless it bites.
- **O005** (org vs personal repo) — dormant until the repo is pushed.
- **D020 reconsideration** — the roadmap's anytime section says to live with the event log before
  deciding on the multi-slot handoff; no fan-out exists, so nothing to decide yet.
- **Envelope asymmetry** — the Claude Code guard is tighter than the opencode envelope (exact
  literals; opencode's `make check*` also admits `make check-frontend`/`check-backend`, which the
  project's auditor allowance does not name). Noted in D069; revisit when a third harness appears.

## Next session

Nothing is blocked. Candidates, in rough order of value:

1. **Run the real `/check-rules` audit in an interactive Claude Code session** — the headless
   smoke test (E012) proved both envelope paths, but the full audit (closure → classification →
   report) hasn't run end-to-end as a skill invocation yet. The `.claude/agents/` directory was
   created this session — a running Claude Code session started before it may need a restart to see
   the agent.
2. **The `cinch ignores` binding** if the audit run shows the missing N/A inventory actually hurts.
3. **A second phase-5-ish consumer** — cinch's own repo has no harness glue (no `.opencode/` or
   `.claude/` at all); its seams are declared-and-validated only. If cinch sessions ever run in a
   harness with permission envelopes, the glue question recurs there.

## Notes for future work

- **The guard's auto-allow depends on the PreToolUse output contract.** Verified working on
  v2.1.221 with `hookSpecificOutput.hookEventName: "PreToolUse"` wrapping `permissionDecision` —
  top-level `permissionDecision` is deprecated and silently ignored. If an older CLI stops honoring
  the allow (it degrades to a normal permission prompt, never a widened envelope), add the
  `Bash(<literal>:*)` rules to the personal `settings.local.json`; keep the project-shared
  `.claude/settings.json` hooks-only.
- **`.claude/` is root-gitignored but partially tracked** (historical `git add -f`), same as
  `.opencode/`. The agent and hook files were force-added this session; any *new* glue file there
  needs `git add -f` at commit time or it silently never ships. Edits to already-tracked files
  stage normally.
- **The guard is a 60-line stdlib script parsing a machine-shaped YAML.** C12 guarantees the ids
  resolve, so the parse risk is the command-literal shapes (quoted vs unquoted, inline comments) —
  both verified, but a future manifest shape change must re-run the guard's cases.
- **The self-check pattern held again**: this session touched no harness rule, only glue and docs,
  so no C10/C2 question even arose on the cinch side — the cheapest resolution yet.
