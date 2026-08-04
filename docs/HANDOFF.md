# P5.2 handoff — `cinch ignores` binding landed; the audit is runnable end-to-end

Status: **the one gap between the seam and the top next-session candidate is closed.** The
check-rules audit's step-1 instruction (`cinch ignores` for the N/A inventory) was denied by both
envelopes — fail-closed guard, deny-by-default opencode agent. It is now a manifest command
(`{{commands.ignores}}`) both consumers declare, so the audit's "never enumerate by hand"
instruction is executable inside the seam (D070). What remains is dormant or consumer-side: the
interactive audit itself (now unblocked), O005, D020, and the third-consumer question.

## Done — [P5.2 session]

**The binding (D070).** `templates/check-rules.md` steps 1 and 2 bind the literal `` `cinch
ignores` `` to `{{commands.ignores}}`; `scaffold/manifest.example.yml` gains `ignores: make
ignores` so `templates_test.go`'s variable contract stays honest. Both consumers declare the
command: project_deltadocs `ignores: "make ignores"` with a new `ignores:` Makefile target
(`$(CINCH) ignores`), cinch's own `.agent/manifest.yml` `ignores: ./bin/cinch ignores`.
`seams.auditor.allow` gains `ignores` in both manifests; the opencode auditor envelope gains
`"make ignores*": allow`; both agent bodies' prose now names the bound shape.

**The Claude Code guard needed zero glue edits** — the D069 payoff, demonstrated live: adding the
id to `seams.auditor.allow` widened the seam by data alone. Unit-runs against both manifests:
`make ignores` allowed (quoted literal), `./bin/cinch ignores` allowed (unquoted), bare
`` `cinch ignores` `` denied as unresolvable, chaining after the allowed literal denied, unrelated
commands denied, non-Bash passes.

**Verification.** Both consumers re-rendered and C2-green: project check-harness exit 0 with the
two designed C6/C7 warns; cinch self-check exit 0 with the two known C11 warns plus the designed
C10 self-question on this session's own template edit (answer: no harness rule changed).
`make ignores` lists CAT-003 and MEMB-006 in the project. `make test` green including
TestTemplatePurity. Decisions logged: D070, E013 (cinch), project E002 (event). Roadmap §P5
permissions bullet annotated.

## Not done

- **The real `/check-rules` audit** — the P5.1 handoff's top candidate, now unblocked: step 1
  (closure + inventory) is script output the seam permits, the semantic half is the model's job
  in the auditor agent. Still hasn't run end-to-end as an interactive skill invocation.
- **O005** (org vs personal repo) — dormant until the repo is pushed.
- **D020 reconsideration** — no fan-out exists in either consumer; nothing to decide.
- **Envelope asymmetry** — the Claude Code guard is tighter than the opencode envelope (exact
  literals; opencode's `make check*` also admits `make check-frontend`/`check-backend`). Noted in
  D069; revisit when a third harness appears.
- **A second phase-5-ish consumer with real harness glue** — cinch's own repo has no `.opencode/`
  or `.claude/`; its seams are declared-and-validated only.

## Next session

Nothing is blocked. Candidates:

1. **Run the real `/check-rules` audit in an interactive Claude Code session** — the envelope
   paths are proven (E012) and step 1's inventory is now seam-permitted (D070), so the full
   closure → classification → report run is the remaining unverified path. A Claude Code session
   started before the agent existed may need a restart to see `.claude/agents/auditor.md`.
2. **A second phase-5-ish consumer** if cinch ever runs in a harness with permission envelopes —
   cinch's own seams are declared-and-validated only, and the guard/agent glue would recur there.
3. **O005 / D020** — both dormant by design (repo push; real fan-out).

## Notes for future work

- **The guard's auto-allow depends on the PreToolUse output contract.** Verified working on
  v2.1.221 with `hookSpecificOutput.hookEventName: "PreToolUse"` wrapping `permissionDecision` —
  top-level `permissionDecision` is deprecated and silently ignored. If an older CLI stops honoring
  the allow (it degrades to a normal permission prompt, never a widened envelope), add the
  `Bash(<literal>:*)` rules to the personal `settings.local.json`; keep the project-shared
  `.claude/settings.json` hooks-only.
- **`.claude/` is root-gitignored but partially tracked** (historical `git add -f`), same as
  `.opencode/`. Any *new* glue file there needs `git add -f` at commit time; edits to
  already-tracked files stage normally. This session only edited tracked files.
- **The guard is a 60-line stdlib script parsing a machine-shaped YAML.** C12 guarantees the ids
  resolve, so the parse risk is the command-literal shapes (quoted vs unquoted, inline comments) —
  the project's `"make ignores"` (quoted) and cinch's `./bin/cinch ignores` (unquoted) are both
  verified; a future manifest shape change must re-run the guard's cases.
- **The `{{commands.ignores}}` binding follows the D067 pattern**: every command the workflow
  instructs the auditor to run must exist as a manifest command, a Makefile target, and an
  envelope allowance. Any future script-instructed command (e.g. a new `cinch` subcommand) should
  ship as all three in the same change, or the workflow will again instruct a denied command.
- **The self-check pattern held again**: the session's own template edits produced the designed
  C10 warn, resolved by answering "no harness rule changed" — no C10/C2 question required any
  harness rule edit.
