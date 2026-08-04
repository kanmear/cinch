# P5.4 handoff — the self-audit's one proposal is applied: HARNESS-004's C2 gap closed

Status: the P5.3 handoff's candidate 2 is done. The check-rules audit's only suggestion —
`TestCheckRenderedTamper` — landed with its stale-branch companion, giving the C2
distinguish-tamper-from-stale contract first-class unit coverage (D072). Closure
script-clean: `go test ./...` green, self-check exit 0 with the two known C11 warns, no
C8 findings, no C10 self-question.

## Done — [P5.4 session]

**HARNESS-004's branch test gap closed (D072).** `TestCheckRenderedTamper` and
`TestCheckRenderedStale` in `check_test.go`, hermetic mini-repos with the render_test.go
machinery (`CINCH_HOME` temp dir, one template, `renderAll` for the pristine body).
Tamper: a committed file whose body no longer matches its own header hash → C2
"hand-edited", never misreported as stale. Stale: header matches own body but body
differs from a fresh render (manifest moved on) → "stale against the manifest", never
misreported as tamper. Marker placement unchanged — HARNESS-004's marker stays above
`checkRendered` (check.go), the C12/TestCheckSeamsAllow precedent the audit named.

**Verification.** `go test ./...` green (the two pre-existing unformatted files —
`introspect_test.go`, `rule_test.go` — are untouched; the touched file is gofmt-clean).
Self-check exit 0 with the two known C11 warns, no C8 findings, `cinch ignores`
unchanged (HARNESS-001). No C10 self-question: test files are not `owns:` paths.
D072/E015 logged; roadmap P5 annotated.

## Not done

- **The real `/check-rules` audit in project_deltadocs** — still the remaining
  unverified path: full closure → classification → report as an interactive skill
  invocation. Start a fresh Claude Code session there so `.claude/agents/auditor.md` is
  seen.
- **The envelope's first live workout** — the cinch auditor agent
  (`.opencode/agent/auditor.md`) has still only been verified at config level (E014's
  method); the audit has not run *inside* the envelope. Restart opencode here with the
  auditor agent selected and invoke check-rules.
- **O005** (org vs personal repo) — dormant until the repo is pushed.
- **D020 reconsideration** — no fan-out exists in either consumer; nothing to decide.
- **Envelope asymmetry** — opencode patterns are static; the Claude Code guard stays
  tighter (chaining/expansion rejection). cinch's own envelope carries the same
  asymmetry (D071). Revisit when a third harness appears.

## Next session

Nothing is blocked. Candidates:

1. **Run the real `/check-rules` audit in an interactive Claude Code session**
   (project_deltadocs) — the remaining unverified path. Fresh session so
   `.claude/agents/auditor.md` is seen.
2. **Run the audit in cinch with the auditor agent selected** — the envelope's first
   live workout: open an opencode session here with `.opencode/agent/auditor.md`,
   invoke check-rules.
3. **O005 / D020** — both dormant by design (repo push; real fan-out).

## Notes for future work

- **The D067 binding pattern still applies to cinch's own seam.** Every command a
  workflow instructs the auditor to run must exist as a manifest command, a Makefile
  target (cinch's commands are `./bin/cinch <sub>`), and an envelope allowance — any
  future script-instructed subcommand needs its allow pattern added to
  `.opencode/agent/auditor.md` in the same change.
- **Cinch's envelope is static patterns.** If a manifest command literal changes shape
  (flags, quoting), re-run the config-level resolution (E014's method): every
  `seams.auditor.allow` id must still match an allow pattern, and no non-seam command
  may.
- **opencode config is loaded at startup** — new agents/skills under `.opencode/` need
  a restart to be selectable (the skill loader scans on start).
- **The self-audit loop is self-consuming**: the audit's semantic half surfaces a
  strengthening (E014 → D072) and the executor applies it next session — the workflow
  now has one complete proposal→applied cycle to its name.
- **The guard's auto-allow output contract** (project-side, Claude Code): verified
  working on v2.1.221 with `hookSpecificOutput.hookEventName: "PreToolUse"` wrapping
  `permissionDecision`; top-level `permissionDecision` is deprecated and silently
  ignored.
- **`.claude/` is root-gitignored but partially tracked** in project_deltadocs
  (historical `git add -f`), same as `.opencode/` there. cinch's `.gitignore` only
  ignores `bin/`, so its `.opencode/` files track normally.
- **The self-check pattern held again**: this session's test + decision + handoff edits
  produced no C10 self-question (test files are not `owns:` paths; `docs/` and
  `decisions.jsonl` are not either), and the self-check stayed exit 0 with the two
  known C11 warns.
