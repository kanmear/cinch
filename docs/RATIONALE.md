# Harness improvement plan

Status: **advisory.** A sequenced improvement plan for the `.agent/` harness, superseding the
`Recommended sequencing` section of `agent-setup-review.md`. Each phase below is scoped to convert
into an executable plan via `/plan-feature`. It assumes `harness-philosophy.md` (north stars +
Foundation §1–6) as the spec and does not re-derive it; where a phase implements a Foundation
section, it cites the section rather than restating it.

The plan adds one thing the earlier review did not have: a **deterministic checker layer**, and a
rule for deciding which checks are legitimate. That rule reorganises the sequencing, so the phases
below are not simply the review's four steps with detail added.

---

## What changed since the review

The review's four-step sequence was correct and remains the spine. Three things have sharpened
since it was written.

**1. A large fraction of the doc-maintenance cycle is a lint job, not a model job.**
`sync-docs` and `optimize-docs` currently spend model tokens auditing properties that are
mechanically decidable. Every such audit is both more expensive and *less* reliable than a script,
because a model audit can return a false ✅ and a script cannot. This is the single largest
unclaimed win in the system and it did not appear in the review at all.

**2. There is a rule for which checks are worth building, and it is about direction.**
If a checker validates that a doc's copy of a fact matches an authoritative machine-readable
source, the fact should not have been copied — delete the duplication instead of policing it. Only
two directions justify a checker:

- **Doc upstream** — the doc is the spec and code must conform (`models/*.md` → types).
- **Lateral** — a binding between two authored artifacts, neither derivable from the other
  (`manifest.yml` ↔ rendered workflows, `api/*.md` ↔ registered routes).

**Doc-downstream checks are a design smell, not a feature.** They are the sign that the
derivability gate in `doc-philosophy.md` was not applied at write time. This rule is what keeps the
checker layer from growing into a maintenance burden of its own.

**3. The freshness question has a design answer, not a process answer.**
"How do you keep this fresh when the codebase changes constantly" is answered by *what gets written
down*, not by how often it is re-synced. Content that a code change can falsify should not be in
the corpus. The one exception is the semantic layer — `business/*.md` encodes rules that a
behaviour change can silently invalidate and that no checker can validate — and that exposure needs
its own mechanism (Phase 4). Everything else is handled by the gate.

---

## Phase 0 — Cheap, independent, unblocking

No dependencies. Do these first because they cost hours, not days, and nothing waits on them.

**0.1 Delete the compliance ritual.** Remove the `"AGENTS.MD initialized"` emission, the
`MANDATORY` framing, and the `protocol violation` language wherever they appear (review §4).
Replace with why-framing per the north star. This is a pure deletion; there is no replacement
artifact.

**0.2 Move the health signal into the hook** (Foundation §5). The SessionStart hook prints its own
confirmation line to the user. Zero model tokens.

**0.3 Branch/plan awareness at session start** (Foundation §6). Same hook: inject
`git branch --show-current` plus the `Status:` lines of `.agent/plans/*.md` and
`.agent/plans/fix/*.md`.

**0.4 Fix the self-containment claim.** `harness-philosophy.md` asserts "It depends on no other
file." True of the principles, false of the substrate — a reader of that file alone cannot discover
`business/`, `models/`, `api/`, or the tiered reading guide. Either scope the claim to the
principles, or add one line pointing at the generated index as the substrate's entry point. The
second is better: it makes the index the discovery mechanism for the whole corpus, which is what it
should be anyway.

**Exit criteria:** no magic-string emission anywhere in the corpus; session start prints branch,
active plan statuses, and a hook-side health line without model involvement.

---

## Phase 1 — The binding layer

Implements Foundation §1 and §3. This is the unlock the review identified and it remains first among
the substantial work, because the checker layer (Phase 3) reads its paths and the seams (Phase 6)
read its permissions.

**1.1 Complete `manifest.yml`** as the single source of every project-specific binding: commands,
test-dir layout, layer taxonomy, test-tier taxonomy. Hold to the two schema constraints already
specified — every `cmd` id resolves under `development.commands`, and there is no `domains:` key.

**1.2 Add per-domain code paths — in the business docs, not the manifest.** Phase 4's diff-coupling
check needs a domain → owning-code-paths mapping. Putting it in `manifest.yml` would reintroduce
the `domains:` key the schema forbids and would drift the same way. Put it in each
`business/<domain>.md`'s front-matter instead:

```yaml
---
rule_prefix: PROJ            # for the rule-ID convention, see 4.1
owns:
  - backend/handlers/projects/
  - backend/models/project.go
  - frontend/src/routes/projects/
---
```

The domain list stays the filesystem; the mapping lives with the domain it describes.

**1.3 Build the render step.** Templates reference `{{commands.check}}` etc.; a render script or an
`install` workflow produces flat, literal, project-bound workflow files. The runtime model reads only
rendered output.

**1.4 Test tiers as data** (Foundation §3), with `tdd: rule-tests` rather than `tdd: mandatory`,
`opt_in: true` as data, and per-tier prose living in the task primitive or a `notes:`-named doc.

**Exit criteria:** no literal `make`, no hardcoded test paths, and no hardcoded tier names in any
template; a fresh repo is onboarded by editing `manifest.yml` and running the render step.

**Risk:** the render step is a new failure mode — rendered files can go stale against an edited
manifest. Checker C2 (Phase 3) exists specifically to close this, so do not ship 1.3 without it.

---

## Phase 2 — The task primitive

Implements Foundation §2. Depends on Phase 1 only in that templates should already be
variable-referencing when they are split, so the split happens once.

**2.1 Extract `task-primitive.md`**: atomicity rules, context-manifest rules, verification tiers,
task template, pre-flight gate, completion ritual.

**2.2 Parameterize the completion ritual** rather than flattening it — fix plans delete their plan
file plus update a troubleshooting doc; feature plans persist with `Status: complete`.

**2.3 Shrink `plan-feature` and `fix-bug`** to their Phase 1/2 fronts and have them compose the
primitive. Repoint `execute-plan`'s cross-references at the primitive rather than at section numbers
in sibling files.

**Exit criteria:** no rule about atomicity, context manifests, or verification tiers appears in more
than one file. `fix-bug.md`'s "Differences from Feature Planning" table is deleted, not updated —
the differences are now structural, expressed as the primitive's parameters.

**Note on value:** the payoff here is drift, not tokens. A session loads one workflow, so extraction
saves roughly zero runtime context. Do not justify this phase on context budget; justify it as one
place to generalize instead of three to keep in sync.

---

## Phase 3 — The checker layer

The largest addition to the original plan. Depends on Phase 1 (checkers read manifest paths).

Build all of these as one target — `make check-harness` — runnable in pre-commit and CI, zero model
tokens, exit non-zero on error.

| # | Checker | Direction | Catches |
|---|---|---|---|
| C1 | index staleness | generated ↔ fs | the generated index disagreeing with the filesystem (Foundation §4) |
| C2 | render staleness | manifest ↔ rendered workflows | rendered docs older than the manifest hash they were built from |
| C3 | cmd-id resolution | manifest-internal | a `cmd:` in `taxonomy.test_tiers` with no key under `development.commands` |
| C4 | path-binding validity | manifest → fs | `paths.*` pointing at directories that don't exist |
| C5 | domain enumeration | fs ↔ `business/overview.md` | a domain file with no entry in overview, or an overview reference to a missing domain file |
| C6 | model ↔ type sync | **doc upstream** | fields declared in `models/*.md` absent from, or contradicted by, the actual structs/types |
| C7 | api ↔ route | lateral | documented endpoints with no registered route, and routes with no doc |
| C8 | rule → test enumeration | doc upstream | a rule ID in `business/*.md` with no test referencing it |
| C9 | plan hygiene | fs ↔ ritual | a completed fix plan still present; a feature plan with a missing or invalid `Status:` |
| C11 | cross-reference integrity | doc ↔ doc | dead internal links — one-fact-one-place depends on cross-refs resolving |

(C10 is Phase 4; it is listed there because it needs the rule-ID decision.)

**C6 and C7 need stack adapters.** Resolve this with the portability tiering you already have: the
*checker* is `verbatim` tier, and the *introspection command* it shells out to is a manifest binding
— `development.commands.introspect-types`, `development.commands.introspect-routes`. A repo with a
different stack supplies its own command; the checker is unchanged. This keeps the highest-value
checks from becoming the least portable ones.

**C8 is the enumeration half of `check-rules`, mechanized.** The philosophy doc assigns enumeration
to a cheap model as part of the auditor split. It is not cheap-model work — it is *no*-model work.
After C8 lands, `check-rules` narrows to the part that actually needs judgment: whether the mapped
test genuinely exercises the rule. That is a strict improvement on the seam design in Foundation's
Forward vision, and it should be reflected there when Phase 6 is written.

**What not to build, and why.** Do not add checkers for documented dependency versions, documented
command existence, or script coverage. Those are doc-downstream — the fact lives authoritatively in
a machine-readable file and the doc's copy should be deleted rather than validated. If such a check
would currently fail, the correct fix is to remove the duplicated content from the corpus.

**Staleness thresholds:** hold off on time- or commit-based staleness warnings. On a personal repo
they fire rarely enough to be noise-free but also catch little; the real drift classes are covered
by C1–C11 deterministically. Revisit only if the harness goes multi-dev (see Deferred).

**Exit criteria:** `make check-harness` green on a clean tree; every mechanical audit currently
performed by `sync-docs`/`optimize-docs` prose is either implemented as a checker or explicitly
deleted from those workflows as doc-downstream. The `.pi/skills/sync-docs` instructions shrink to
the judgment-only remainder.

---

## Phase 4 — Semantic integrity

Depends on Phase 3's harness and Phase 1's per-domain `owns:` front-matter.

This phase addresses the one drift class nothing else catches: a developer changes behaviour, the
tests are updated, `check-rules` still passes because the rule still has a test — and the rule text
is now false. The rules→tests→tasks loop is closed but anchored to a stale root.

**4.1 Adopt a rule-ID convention.** C8 and C10 both need rules to be addressable. Something like
`PROJ-014` in the business doc, referenced from the test by tag or comment rather than by test name
— name-matching is precisely what the semantic audit was designed to avoid, so the reference must be
explicit, not inferred.

**4.2 C10 — diff-coupling.** If a commit touches paths under a domain's `owns:` list and does not
touch that domain's business doc, flag it. Warning, not error: most commits legitimately don't
change rules. The value is that it puts the question in front of a human at the moment the answer is
cheapest.

**4.3 Harden the derivability gate into an admission test.** `doc-philosophy.md` states the
principle; make it a gate the write path runs against. One question, asked before any doc content is
added: *can an agent recover this by reading the source?* If yes, it does not get written. This is
the mechanism that keeps the corpus fresh by construction, and it is worth stating as such in the
philosophy rather than leaving it as an implication of "code is the documentation."

**4.4 Give `optimize-docs` a deletion mandate.** It is currently framed as an anti-rot audit. Rot
includes content that should never have been admitted. Add an explicit pruning pass that applies 4.3
retroactively and *removes* rather than refreshes. Without this, the corpus only grows, and the
context-budget north star erodes silently over months.

**Exit criteria:** every rule in `business/` has an ID; every rule ID resolves to at least one
tagged test; a behaviour-changing commit that leaves its domain doc untouched produces a warning;
`optimize-docs` has removed at least one section on its first run.

---

## Phase 5 — Episodic memory

Independently schedulable; no dependencies. Small.

The Session Handoff is *state* — a single slot describing where work stands. There is no *episodic*
record: why a session went the way it did, what was ruled out, what surprised. `business/` holds
standing rules, git holds diffs, and neither holds rationale-in-flight.

**5.1 Add an append-only event log** — `.agent/events/*.jsonl` or equivalent — written on decisions,
risks, and dead ends. Append-only is the point: no merge semantics needed, no rewrite path, no
pruning pressure, and it cannot drift because it is a record of what was believed at a time rather
than a claim about the present.

**5.2 Reconsider the multi-slot handoff in light of it.** The philosophy scopes the multi-slot
problem correctly — it bites only for parallel lines *within one plan*. But an event log addresses a
good part of that case without branching the slot at all: parallel lines append, and the slot stays
single. Decide whether multi-slot is still needed *after* 5.1 has run for a while. Do not build it
first.

**Exit criteria:** a cold resume can answer "what did the last three sessions rule out?" without
reading diffs.

---

## Phase 6 — Seams, tiering, permissions

Depends on Phases 1 and 2. This is the Forward vision, unchanged in substance; three amendments.

**6.1 Declare the seams and their model tiers** per the philosophy's table, with two corrections:

- **Auditor** — the split changes. C8 removes enumeration from the model entirely, so the seam is no
  longer "cheap enumeration + strong verification" but "strong verification only, over a
  script-produced enumeration." Cheaper *and* stronger than the original design.
- **Doc-maintainer** — after Phase 3, most of the mechanical routing is gone from this seam too. Mid
  tier is still right, but for a smaller and more judgment-dense job: "can an agent derive this from
  source?" is now nearly the whole role.

**6.2 The manifest as permission source.** Declare per-seam allowances in `manifest.yml`; accept that
enforcement is per-harness glue (Claude Code hook permissions, a local runner's allowlist) and must
be built, not assumed. Do not let the declaration imply enforcement in any doc.

**6.3 Sequence within the phase:** declare seams → wire the Auditor split (highest value, lowest
risk, and Phase 3 already did the hard half) → Executor fan-out → permissions. Only after Executor
fan-out exists does the multi-slot handoff question (5.2) become real.

---

## Explicit non-goals

Stated so they don't reappear as good ideas later.

- **No GROW-style unconditional self-write.** An agent that writes a pattern after every task is an
  unbounded write path into memory with no admission gate. On a weak local model it is a drift
  generator. New workflow docs are authored deliberately, not emitted as a side effect.
- **No narration contracts.** "Narrate what you load" is the same proxy metric as
  `"AGENTS.MD initialized"` — the model can emit the string without doing the thing. Compliance is
  verified through observable behaviour.
- **No doc-downstream checkers.** Covered above; the fix is deletion, not validation.
- **No pre-built index of the code itself.** The corpus documents what code cannot answer. Anything
  the agent can grep for is grepped for, live.
- **No multi-slot handoff before Executor fan-out exists.** It solves a problem the harness does not
  yet have.

---

## Deferred: multi-developer

The whole plan assumes a personal harness. If it ever serves a team, three things break that are
currently free: doc ownership (who arbitrates a rule change), merge semantics on prose (git's
line-based resolution is close to useless on rewritten paragraphs), and the economics of C10
(warnings that fire on most commits get ignored). None of this is worth building now. It is worth
knowing that the semantic layer is where it would break first, not the mechanical one.

---

## Open decisions

These block specific phases and are yours to make.

1. **Rule-ID scheme and test-reference mechanism** (blocks 4.1, C8). Tag? Structured comment?
   Registry file? The constraint is that the reference must be explicit rather than name-inferred.
2. **Introspection strategy for C6/C7** (blocks Phase 3). Static parse of source, or a runtime
   command that dumps the route table and type set? Runtime is more accurate and adds a build
   dependency to the check.
3. **Index delivery** (affects Phase 0.4 and C1). Committed generated file, or injected at session
   start by the hook? The philosophy leaves both open.
4. **Event log schema** (blocks 5.1). Minimal is better; the failure mode is a schema rich enough to
   need maintenance.

---

## Sequencing summary

```
Phase 0  cheap deletions + hook signals          independent, do first
Phase 1  binding layer + agnostic taxonomy       unblocks 3 and 6
Phase 2  task primitive                          unblocks 6
Phase 3  checker layer                           needs 1; do not ship 1.3 without C2
Phase 4  semantic integrity                      needs 1 and 3
Phase 5  event log                               independent; small; do whenever
Phase 6  seams, tiering, permissions             needs 1 and 2; benefits from 3
```

The order differs from the review in one substantive way: the checker layer is promoted ahead of the
seam design, because it changes what the seams should be. Designing the Auditor split before C8
exists would bake in an enumeration pass that a script makes obsolete.
