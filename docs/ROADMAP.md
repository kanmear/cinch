# cinch — roadmap (v3)

Status: **advisory.** Supersedes v2. Lives in the **cinch repo**, not in the consuming project
(D032).

This is the phase ledger: standing rules, the phase list with landed status, non-goals, and the
session-load table. Phase detail is historical record — git is the archive; forward work lives in
the plan files under Current work.

## Current work

- `.agent/plans/reorg.md` — the reorganization: all four phases landed (scope and taxonomy, cut,
  Go restructure, fundamentals cross-check — D074–D080); plan closed complete, file kept.
- `.agent/plans/drift-closure.md` — the drift-test follow-up, five phases: deltadocs `.agent`
  sync, `make drift-test` fixture, C10/C14 boundary decision + C14, blinded auditor, close.

---

## Standing rules

**1. Nothing lands in cinch that is not rendering into the project** (D033). The project is the
consumer from commit one. A template, checker, or manifest key is not done until it runs against
the real repo.

**2. cinch does not plan its own work with the project's `plan-feature`** (D036). It is domain-bound
and structurally inapplicable to a Go CLI with no domain rules. From P3 onward cinch plans its own
work with the task primitive it extracted for consumers (D036); a phase that modifies a project
workflow is not executed via that workflow (D025).

**6. Build sessions assume a frontier model** (D035). The ~90k budget is the consumer's constraint,
not the workspace's. Per-phase file lists are relevance filters; decomposition ceremony is optional
for P0 and P1.

**3. Generated artifacts are committed to the project and guarded twice** (D006, D007). Rendered
workflows carry a `generated — do not edit` header and a body hash. C2 distinguishes stale
(manifest moved on) from tampered (output hand-edited).

**4. Context is signal-to-noise, not budget** (D037). Irrelevant material degrades output even when
it fits. Context manifests are relevance filters, valid at 90k and at 1M. Model tier is a manifest
binding (D038), not an assumption baked into prose.

**5. Core is Go and never parses project source** (D003, D028). Type and route introspection are
manifest-declared commands; cinch defines their output contract.

---

## Phases

Every phase is annotated **[cinch]**, **[project]**, or **[both]**. A session works one side.
Phase sections are one-line statuses; the detail behind each was logged in `decisions.jsonl` as it
landed.

**Phase 0 — Split the workspace [both]** — Landed: two repos; the project session no longer
carries cinch material and vice versa.

**Phase 1 — Binding layer [both]** — Landed: literal-substitution `render` and `index`, completed
manifest contract (commands, paths, taxonomy, seams), distribution via `~/.cinch` with a
`harness_version` pin; rendered output committed from the first consumer.

**Phase 2 — Task primitive [cinch]** — Landed: `task-primitive.md` extracted with the compaction
anchor and the parameterized completion ritual; no atomicity, manifest, or verification rule
appears in more than one template. Cinch renders its own workflows with it from P3 onward.

**Phase 3 — Checker layer [cinch]** — Landed: C1–C12 in three waves (filesystem/YAML/markdown;
C6/C7 introspect contract in `docs/INTROSPECTION.md`; C8 rule-ID closure). The direction rule
governs what may exist: doc-upstream and lateral checks only. `cinch check` green on both real
consumers; no time- or commit-based staleness thresholds.

**Phase 4 — Semantic integrity [both]** — Landed: author-assigned permanent rule IDs with
`// cinch:rule` markers (warnings both directions, `cinch ignores` for exemptions); C10 diff-
coupling at the domain level (working-tree window only — a hook-only check by design, D065/D082);
C14 rule-level diff-coupling with a committed window (drift-closure Phase 3, D082); the
derivability gate as an explicit admission test; `optimize-docs` deletion mandate exercised.

**Phase 5 — Seams, tiering, permissions [both]** — Landed: auditor seam strong-tier over a
script-produced closure; `allow:` command-id lists declared in both manifests and enforced by both
harness glues (the opencode auditor envelope and the Claude Code PreToolUse guard); the audit has
executed end-to-end in both consumers and inside cinch's own seam. Fan-out verified-and-deferred
until a consumer actually gains it.

---

## Anytime tracks [project]

Independent of the sequence (D034).

- **Session-start signals.** Health signal printed by the SessionStart hook, not emitted by the
  model (Foundation §5); shell calls the binary and does nothing else (D024). Designed; status in
  the consumer not re-verified since P5.
- **Episodic memory.** Append-only `.agent/events/*.jsonl` in the consumer; minimal schema per the
  consumer's events log (the schema copy that lived at `docs/decisions-log.md` was the exact
  duplication the direction rule forbids — deleted, D074). Landed in project_deltadocs (E001 seed,
  recording rule); index-inert, so no checker touches it.

The multi-slot handoff (D020) stays deferred: the events log covers the parallel-lines-within-one-
plan case without branching the slot. Reconsider only once a consumer creates real fan-out.

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

A relevance filter, not a budget ceiling (D035) — what a phase session should have in front of it;
the failure prevented is a cinch session loading the project's domain rules, or a project session
loading cinch's design history. Anytime-track sessions load the anytime section and nothing else.
Paths follow the Phase 3 layout (D076); forward detail lives in the Current work plan files.

| Phase | Side | Load |
| --- | --- | --- |
| 1 | cinch | roadmap §P1 · `internal/render/render.go` `internal/manifest/manifest.go` · one workflow as the port pilot · D005 D008 |
| 1 | project | roadmap §P1 · `manifest.example.yml` · current `manifest.yml` · D001 D010 |
| 2 | cinch | roadmap §P2 · `task-primitive.md` `fix-bug.md` `execute-plan.md` · D036 D039 |
| 3 | cinch | roadmap §P3 · `internal/check/check.go` · consumer `manifest.yml` · D009 D011 D027 D028 D030 |
| 4 | project | roadmap §P4 · `domain/overview.md` · one `domain/<domain>.md` · `doc-philosophy.md` · D012 D013 D027 |
| 4 | cinch | roadmap §P4.2 · `internal/check/diff.go` · D009 |
| 5 | both | roadmap §P5 · `manifest.yml` · workflow list · D014 D015 |

---

## Open

**O005** — cinch under a GitHub org or a personal repo. Blocks P0.1, and only that.
