# cinch — roadmap (v3)

Status: **advisory.** Supersedes v2. Lives in the **cinch repo**, not in the consuming project
(D032).

v3 changes one thing structurally: **cinch is created first, greenfield** (D031). v2 built the
binding layer and checkers inside the project's `.agent/` and extracted them at Phase 4. That put
three concerns — the project, its harness, and the harness's own rebuild — in one session-injected
context window, and made portability an audit rather than a constraint.

Every phase is annotated **[cinch]**, **[project]**, or **[both]**. A session works one side.

---

## Standing rules

**1. Nothing lands in cinch that is not rendering into the project** (D033). The project is the
consumer from commit one. A template, checker, or manifest key is not done until it runs against
the real repo. This replaces the pressure that extract-later gave for free.

**2. cinch does not plan its own work with the project's `plan-feature`** (D036). It is domain-bound
and structurally inapplicable to a Go CLI with no domain rules. P0 and P1 are planned in-session
against the roadmap; P2 produces the primitive cinch then uses for P3 onward. Separately, D025 still
applies project-side: a phase that modifies a project workflow is not executed via that workflow.

**6. Build sessions assume a frontier model** (D035). The ~90k budget is the consumer's constraint,
not the workspace's. Per-phase file lists are relevance filters; decomposition ceremony is optional
for P0 and P1.

**3. Generated artifacts are committed to the project and guarded twice** (D006, D007). Rendered
workflows carry a `generated — do not edit` header and a body hash. C2 distinguishes stale
(manifest moved on) from tampered (output hand-edited).

**4. Context is signal-to-noise, not budget** (D037). Irrelevant material degrades output even when
it fits. Context manifests are relevance filters, valid at 90k and at 1M — which is what makes one
harness serve both a 27B consumer and a frontier one. Model tier is a manifest binding (D038), not
an assumption baked into prose.

**5. Core is Go and never parses project source** (D003, D028). Type and route introspection are
manifest-declared commands; cinch defines their output contract.

---

## Phase 0 — Split the workspace  [both]

One goal: three concerns become two repos. No harness improvement happens here.

- **0.1** Create the cinch repo. Empty, or seeded via `git filter-repo` if the workflow files'
  history is worth keeping. Give it its own minimal `AGENTS.md` — cinch is a Go CLI plus a template
  set, and needs nothing resembling the project's harness.
- **0.2** Move cinch's design docs out of the project: this roadmap, `docs/RATIONALE.md`,
  `decisions.jsonl`, and **`harness-philosophy.md`** (D032, D037) — the north stars and Foundation
  are portable spec, not project content. The project's `.agent/` keeps only project material.
- **0.3** Delete the compliance ritual from the project harness — `"AGENTS.MD initialized"`,
  `MANDATORY`, `protocol violation`. Pure deletion, no replacement.

**Exit:** two repos. A project session's injected context contains no cinch material; a cinch
session's contains no project domain material.

---

## Phase 1 — Binding layer  [both]

Implements Foundation §1 and §3. Unblocks everything.

**[cinch]** The Go binary: literal substituter (D005 — no template engine; an undefined variable is
an error), `render`, `index`. Templates ported one at a time, authored *as* templates.

**[project]** `manifest.yml` completed: commands, test-dir layout, layer taxonomy, test-tier
taxonomy, `harness_version` pin. Two schema constraints hold — every `cmd` id resolves under
`development.commands`; no `domains:` key. Per-domain owning code paths go in
`domain/<domain>.md` front-matter, not the manifest (D010).

**[project]** `seams:` declared in the manifest — tier and constraints per seam (D038). Declaration
only; enforcement is P5.

**[project]** Distribution: cinch cloned to `~/.cinch`, version pinned in the manifest, rendered
output committed (D006, D008).

Test tiers as data: `tdd: rule-tests` not `tdd: mandatory`; `opt_in: true` as data; per-tier prose
in the primitive or a `notes:`-named doc.

**Exit:** no literal `make`, no hardcoded test paths or tier names in any template. Nine of the
project's ten workflows are rendered output; `figma-restyle.md` stays project-owned prose (D053) —
its content doesn't generalize into a portable procedure. **Do not ship the render step without
C2.**

---

## Phase 2 — Task primitive  [cinch]

Implements Foundation §2. Unblocks P5.

Extract `task-primitive.md` as a template: atomicity, context-manifest rules, verification tiers,
task template, pre-flight gate, completion ritual, **and the compaction anchor** (D039) — the
minimal state a workflow declares must survive a mid-session summarization event: current task ID,
its context manifest, the completion ritual. Cheap now, expensive to retrofit into every template
later. Parameterize the completion ritual rather than
flattening it — fix plans delete their file plus update a troubleshooting doc; feature plans persist
with `Status: complete`. Shrink `plan-feature` / `fix-bug` to their Phase 1/2 fronts; repoint
`execute-plan` at the primitive.

**Exit:** no atomicity, manifest, or verification rule appears in more than one template.
`fix-bug`'s "Differences from Feature Planning" table is deleted, not updated. **And cinch renders
the primitive for its own use** (the template exists, is C2-green in project_deltadocs, and is
proven portable — not a self-manifest or self-render inside cinch's own repo, which stays out of
scope until P3 per D036/standing rule 2) — from P3 onward cinch plans its own work with it (D036),
which is both the planning workflow the project's domain-bound `plan-feature` cannot provide and
the first real portability evidence.

**Justify as drift control, not context savings** — a session loads one workflow, so extraction
saves roughly zero runtime tokens.

---

## Phase 3 — Checker layer  [cinch]

**The direction rule governs what may exist** (D009): only *doc-upstream* (doc is the spec, code
conforms) and *lateral* (binding between two authored artifacts) checks. A *doc-downstream* check
means the derivability gate failed at write time — delete the duplication instead.

Three waves (D030):

**Wave 1** — pure filesystem, YAML, markdown. No adapters, nothing to retrofit.
C1 index ↔ filesystem · C2 render staleness + tamper · C3 `cmd` id resolution · C4 manifest paths
exist · C5 domain files ↔ `overview.md` · C9 plan hygiene · C11 cross-reference integrity ·
C12 seam schema (every seam declares a known tier; `auditor` must be `strong`; `max_task_layers`
a positive integer when declared).
*(C1–C12 ship in the starter package.)*

**Wave 2** — C6 `models/*.md` ↔ real types; C7 `api/*.md` ↔ registered routes. Static introspection
for types, runtime for routes (D028). cinch defines the JSON contract; the project implements the
producers. *(Landed — contract in `docs/INTROSPECTION.md`, C6/C7 in `check.go`, producers in the
project; doc-upstream errors, coverage warns, absent producers skip. D056.)*

**Wave 3** — C8 rule ID → at least one `// cinch:rule <ID>` marker (D027), plus the reverse
check that every marker resolves to a real rule. Blocked on markers existing across the test suite —
a migration, not a build. *(Landed with the P4.1 rule-ID migration — D057/D064; forward warns,
reverse errors, `cinch:ignore` declares exemptions, `cinch ignores` lists them.)*

No time- or commit-based staleness thresholds (D011).

**Exit:** `cinch check` green on a clean tree in the project and in cinch's own repo — the two real
consumers (D061 cut the toy fixtures). Every mechanical audit in `sync-docs` / `optimize-docs`
prose is either a checker or deleted as doc-downstream.

---

## Phase 4 — Semantic integrity  [both]

Needs P1 and P3. Addresses the one drift class nothing else catches: behaviour changes, tests are
updated, `check-rules` still passes — and the rule text is now false.

- **[project] 4.1** Roll out rule IDs and `// cinch:rule` markers. Author-assigned, permanent, never
  reused, prefixed from `rule_prefix` front-matter. Only `domain/<domain>.md` rules get IDs;
  philosophies in `overview.md` do not (D027). *(Landed — 75 rules IDed and marked across
  project_deltadocs; cinch's own harness rules HARNESS-001..005 marked or ignored. D064.)*
- **[cinch] 4.2** C10 diff-coupling: a commit touching a domain's `owns:` paths but not its domain
  doc → **warning**, not error. Value is asking the question when the answer is cheapest.
  *(Landed — C10 in `diff.go`, working-tree boundary (`git diff HEAD`), one warn per domain, skips
  outside git or with no owns lists. D065.)*
- **[project] 4.3** Derivability gate as an explicit admission test: *can an agent recover this from
  source?* If yes, it is not written. Freshness is a write-time constraint, not a sync process
  (D012). *(Landed — the gate is two questions at the head of `doc-philosophy.md`: recoverable → not
  written, point at source; falsifiable → business rule → `domain/` doc with an ID. D066.)*
- **[project] 4.4** `optimize-docs` gets a deletion mandate — it must *remove*, applying 4.3
  retroactively (D013). *(Landed — it prunes first, remove-not-refresh, with `domain/` and generated
  workflows carved out; the first run deleted the superseded v4-flash review. D066.)*

**Exit:** every rule has an ID resolving to a marked test (untestable rules declared
`cinch:ignore`, listed by `cinch ignores` — D064); a behaviour-changing commit that leaves
its domain doc untouched warns *(live — C10, D065)*; `optimize-docs` has deleted something
on first run *(done — the superseded analysis review was pruned, D066)*.

---

## Phase 5 — Seams, tiering, permissions  [both]

Needs P1, P2; benefits from P3.

- **Auditor seam changed by C8.** Enumeration left the model entirely, so the seam is
  strong-verification over a script-produced list — cheaper *and* stronger than the original split
  (D014). *(Wired — check-rules.md takes the rule closure from the harness checkers
  (`{{commands.check-harness}}`: C8 warns + `cinch ignores` inventory); the model's job is the
  semantic half only. D067.)*
- **Doc-maintainer narrows.** After P3 most mechanical routing is gone; mid tier still right, but
  the role is nearly all "can an agent derive this from source?"
- **Fan-out arrives earlier than this phase implies** (D039). Frontier harnesses spawn parallel
  sub-agents natively, so Executor fan-out and the multi-slot handoff question (D020) may become
  real before P5 is formally reached. *(Checked at P5: no fan-out exists in either consumer —
  skills are single-slot, execute-plan runs one atomic task per session, no sub-agent config
  declared. The D020 deferral stands; re-verify when a consumer actually gains fan-out. D068.)*
- **Permissions.** Declare per-seam allowances in `manifest.yml`. Enforcement is per-harness glue
  (Claude Code hook permissions, local runner allowlist) and must be built. No doc may imply that
  declaration is enforcement. *(Enforced — `allow:` command-id lists in both manifests,
  C12-validated (non-empty, C3-resolved). Both glue pieces landed: the project's opencode auditor
  agent envelope (D067) and the Claude Code auditor agent whose PreToolUse guard derives its
  allowance from the manifest at runtime — declaration and enforcement cannot drift (D069).
   The check-rules N/A inventory is itself a bound command (`{{commands.ignores}}`, D070), so the
   audit's step 1 is executable inside the seam. *(cinch's own repo now carries real glue for the
   harness it runs in: `.opencode/agent/auditor.md` envelope + check-rules entry point, and the
   audit ran against cinch's own domain — D071/E014. The audit's one proposal — C2's tamper branch
   had no unit test — is executor-applied: `TestCheckRenderedTamper` + the stale-branch companion
   in check_test.go close the gap (D072/E015).)*

Order within: declare seams → wire the Auditor verification → Executor fan-out → permissions.
*Status at session close: Phase 5 complete — seams declared (P1), Auditor wired, fan-out
verified-and-deferred, permissions declared and enforced in both harnesses (D067/D069) and in
cinch's own opencode seam (D071); the audit has executed inside cinch's own envelope (E016) and —
the last unverified path — interactively in the Claude Code consumer (E017): full closure →
classification → report → proposal → executor-apply, the one real gap closed (PERM-004's
read-access clause, route tests + marker move) and the one misfire (a rule declared unmarked
under a zero-unmarked closure) hardening step 3 of the workflow (D073). The seam now has a
complete workout in every consumer it was built for.*

---

## Anytime tracks  [project]

Independent of the sequence (D034).

**Session-start signals.** Health signal printed by the SessionStart hook, not emitted by the model
(Foundation §5). Branch and active-plan `Status:` lines injected at session start (Foundation §6).
Shell calls the binary and does nothing else (D024).

**Episodic memory.** Append-only `.agent/events/*.jsonl` for project decisions, dead ends, accepted
risks. Minimal schema — see `docs/decisions-log.md` for the convention. Append-only means no merge
semantics, no rewrite path, no pruning pressure, and it cannot drift. *(Landed —
`project_deltadocs` carries `.agent/events/events.jsonl` seeded with E001, plus an AGENTS.md
recording rule; index-inert, so no checker touches it.)*

Then reconsider the multi-slot handoff (D020): the log covers much of the
parallel-lines-within-one-plan case without branching the slot. Decide after living with it, and
only once P5 creates real fan-out.

---

## Non-goals

- **No GROW-style unconditional self-write** (D021).
- **No narration or compliance contracts** (D022).
- **No doc-downstream checkers** (D009).
- **No pre-built index of the code itself** (D023).
- **No plugin system for checkers; no bash beyond the SessionStart hook** (D024).
- **No multi-slot handoff before Executor fan-out exists** (D020).
- **No multi-developer support** (D026). The semantic layer breaks first, not the mechanical one.

---

## Order

```
0  split the workspace         [both]    one goal: two repos
1  binding layer               [both]    unblocks everything
2  task primitive              [cinch]   unblocks 5
3  checker layer, 3 waves      [cinch]   ship C2 with the render step
4  semantic integrity          [both]    needs 1, 3
5  seams, tiering, permissions [both]    needs 1, 2

anytime  session-start signals, episodic memory   [project]
```

**The checker layer precedes the seam design** (D015) — C8 changes what the Auditor seam should be.

---

## Session loads

What a phase session should have in front of it — a relevance filter, not a budget ceiling (D035).
The failure this prevents is a cinch session loading the project's domain rules, or a project
session loading cinch's design history. Anytime-track sessions load the anytime section and nothing
else.

| Phase | Side | Load |
| --- | --- | --- |
| 1 | cinch | roadmap §P1 · `render.go` `manifest.go` · one workflow as the port pilot · D005 D008 |
| 1 | project | roadmap §P1 · `manifest.example.yml` · current `manifest.yml` · D001 D010 |
| 2 | cinch | roadmap §P2 · `task-primitive.md` `fix-bug.md` `execute-plan.md` · D036 D039 |
| 3 | cinch | roadmap §P3 · `check.go` · consumer `manifest.yml` · D009 D011 D027 D028 D030 |
| 4 | project | roadmap §P4 · `domain/overview.md` · one `domain/<domain>.md` · `doc-philosophy.md` · D012 D013 D027 |
| 4 | cinch | roadmap §P4.2 · `check.go` · D009 |
| 5 | both | roadmap §P5 · `manifest.yml` · workflow list · D014 D015 |

---

## Open

**O005** — cinch under a GitHub org or a personal repo. Blocks P0.1, and only that.
