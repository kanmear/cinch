# P4.1 handoff — rule IDs landed, C8 live on both consumers, tabID regression fixed

Status: **Phase 4.1 exit items are met.** All 75 domain rules in project_deltadocs carry permanent
IDs and `// cinch:rule` markers; cinch's own harness rules (HARNESS-001..005) are marked or
ignored; C8 ships with the migration (D057) and is green on both consumers. P4.2 (C10), 4.3
(derivability gate), and 4.4 (optimize-docs deletion) remain — see Next session.

## Done — [P4.1 session]

**C8 checker + `cinch ignores` (cinch side, D064)**

- `rule.go` implements C8: rule IDs are read from the docs themselves — `rule_prefix` front-matter
  plus a bold `**PREFIX-NNN**` token at the start of each numbered item in a non-overview
  `domain/*.md`. Forward (rule with no marker) **warns**; reverse (marker resolving to no rule)
  **errors**; a rule both marked and ignored, an ignore naming a nonexistent rule, a rule doc
  with items but no `rule_prefix`, and a numbered item without an ID all error. Rules with no
  marker and no ignore warn.
- `<!-- cinch:ignore <ID> -->` under a rule item declares it untestable-by-design (display
  behavior, future-decision constraints) and drops it out of the coverage question. `cinch
  ignores` lists the inventory with rule text — the enumerable N/A list for the check-rules and
  optimize-docs audits.
- The marker scan reads the **source tree only**: `.agent/`, `templates/`, `docs/`,
  `decisions.jsonl` (D027's own example token lives there forever) and `.git/`/`bin/` are
  skipped. Prose that teaches the marker syntax must use placeholder IDs (`// cinch:rule <ID>`)
  or the reverse check fires.
- Markers for checker-enforced rules sit above the enforcing function (`// cinch:rule
  HARNESS-002` above `checkSeams`, HARNESS-004 above `checkRendered`); test-enforced rules sit
  above the test function as D027 wrote it. `rule_test.go` covers the parser, both scan
  directions, ignore semantics, and overview exemption; `TestRuleIDsUnique` enforces R5.

**P4.1 migration (project_deltadocs)**

- All 8 domain docs gained `rule_prefix` + IDs, one per rule, matching the existing number
  (SIG-001..SIG-020, MEMB-001..013, PROJ-001..012, PERM-001..007, FILE-001..007, CAT-001..006,
  ROLE-001..005, TAB-001..005). Numbers and `(rule N)` prose cross-references untouched —
  nothing renumbered, so the audit's §2 citation class stays valid.
- 73 rules are marked above their primary enforcing test (20 test files; several rules share a
  test — e.g. `TestCreateProject` carries MEMB-005/PROJ-002/PROJ-004/ROLE-002). Two rules are
  `cinch:ignore`'d by design: MEMB-006 (asserts the absence of a flow — no test can enforce a
  nonexistent endpoint) and CAT-003 (display behavior — tab labels derive from a category name).
- cinch's own `.agent/domain/harness.md`: `rule_prefix: HARNESS`, R1–R5 became HARNESS-001..005;
  HARNESS-001 (untestable future-decision constraint, guardrail §7.3) is the cinch-side
  `cinch:ignore` case; R5's "testable, untested" gap is closed by `TestRuleIDsUnique`.

**The tabID regression (pre-existing, found and fixed this session)**

- `make test-backend` was red at HEAD on a fresh database: **20 signature-endpoint tests** failed
  because D056's `{id}`→`{tabID}` rename updated the handlers but not the direct-handler tests,
  which still did `SetPathValue("id", ...)` — the handlers read `tabID` and returned
  `API_INVALID_ID`. The handoff's "make test green" did not survive the P3 rename (check-harness
  was green; the backend suite was not re-run). The 18 stale call sites in `tab_test.go`,
  `tab_permission_test.go`, `realtime_test.go` now set `tabID`, and the full backend suite is
  green. This mattered for the migration: a marker claims coverage, and coverage must be real.
  Also recreated the dev DB volume (`make drop-db` + `dev-db`) — goose tracks migrations by
  version only, and the single baseline file grows in place.

**Templates (authoring side of the migration)**

- `rules.md` teaches the ID scheme: new rules append with the next free ID, never renumber/reuse,
  the new-file template shows `rule_prefix` front-matter and `**<PREFIX>-001**` items, and both
  marker and ignore declarations are spelled out (plus a "Renumbering rules" pitfall).
- `check-rules.md` enumerates by ID, treats `cinch check` (C8) as the mechanical half of the
  closure (its semantic half is confirming the marked test really exercises the rule), and its
  N/A step now says "declare `cinch:ignore` in the doc", not just "skip in the report".
- `plan-feature.md` §1.2/1.3 reference rules by ID.
- Both consumers re-rendered (`cinch render` + `cinch index`); D049 purity test green.

**Verification**

- cinch: `go test ./...` green; self-check exit 0 with the two known C11 warns; `cinch ignores`
  lists HARNESS-001.
- project_deltadocs: `make check-harness` exit 0 with the two designed warns (health,
  indicators); backend suite and frontend suite (332 tests) green; `make check` green.
- **Decisions logged:** D064 (C8 semantics: doc-side IDs, warn-forward/error-reverse, ignore
  mechanism, scan exclusions, marker placement), E007 (session event). Roadmap P3 wave-3, P4.1,
  and P4 exit lines annotated.

## Not done — P4 status

1. **C10 diff-coupling (P4.2, [cinch])** — warn-level check that a commit touching a domain's
   `owns:` paths but not its domain doc asks the question. Not started; nothing in this session
   blocks it.
2. **Derivability gate (4.3, [project])** — the admission test ("can an agent recover this from
   source?") as an explicit practice.
3. **optimize-docs deletion mandate (4.4, [project])** — must *remove* on first run, applying 4.3
   retroactively. Untouched.
4. **O005** (org vs personal repo) — still open; dormant until the repo is pushed.

## Next session

The roadmap's session-loads table for the rest of P4:

> roadmap §P4.2 · `check.go` · D009 — the cinch side is C10 diff-coupling (warn-level);
> roadmap §P4.3/4.4 · `domain/overview.md` · `doc-philosophy.md` · D012 D013 — the project side is
> the derivability gate and optimize-docs' deletion mandate.

P4.1's exit bar is met; nothing in P4.2/4.3/4.4 is blocked on anything pending. C10 needs the
commit boundary (git diff vs HEAD) — the one check that departs from the pure filesystem pattern
of C1–C9, worth a decision entry on direction before building (doc-upstream or lateral only, D009).

## Notes for future template work

- **Rule ID format is data, not prose.** ID = doc's `rule_prefix` + the rule's existing number,
  zero-padded (`SIG-020`). Because the number and the ID are welded, "never renumber" (already
  the de-facto rule) is now mechanically load-bearing: renumbering silently breaks markers and
  citations. A future scheme change (e.g. prefixes per section) would be a full migration.
- **Marker examples in prose must not look like markers.** `// cinch:rule PROJ-014` in any
  non-excluded file is a real token to C8. The append-only decisions log is excluded from the
  scan for exactly this reason (D027's example lives there); prose elsewhere should use `<ID>`
  placeholders.
- **The scan-exclusion set is the definition of "source"** for the closure: `.git/`, `bin/`,
  `.agent/`, `templates/`, `docs/`, `decisions.jsonl`. A consumer whose real markers live in an
  unusual location (not under test dirs) works regardless — only doc surfaces are excluded.
- **The D056 test regression is the canonical "checker green, suite red" trap.** `make
  check-harness` never exercises the backend tests; the handoff's "make test green" claim
  predated the rename. The P4.1 migration re-ran the real suites and paid the debt. Consider
  wiring a suite run into the pre-commit guard or the CI workflow once the repo is pushed (O005).
- `<!-- cinch:ignore <ID> -->` currently carries no justification text checked by C8 — the
  comment's prose is advisory. If the inventory outgrows the manual audit, the check could
  require a reason string; not yet needed.
