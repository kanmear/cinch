# Harness philosophy

Status: **advisory.** The complete, standalone spec for building and maintaining a **personal agent harness** — a framework/language-agnostic setup driven by any model tier, from a local 27B Q4 at ~90k context to a frontier agent at 1M. The tier is a configuration, not a fork. This doc is self-contained: the `## North stars` are the invariant principles, the
`## Foundation` restates the portable mechanics the harness builds on, and the `## Forward vision` is the harness-specific design work. It depends on no other file.

---

## North stars — the invariant core

The invariant principles the harness is built on — harness-grade and portable across any repo.
Only the project-specific *bindings* around them are not portable; those are isolated into the
manifest and rendered away at install time (see **Foundation** below).

### Context is signal-to-noise

The standout of the whole system, and the thing most setups get wrong. Irrelevant material in the
window degrades output even when it fits comfortably: attention is finite independently of context
length, and noise costs accuracy at a 1M window exactly as it does at 90k. The frame is **signal-
to-noise, not budget** — a context manifest is a relevance filter, not a budget allocation, sized
by what a task genuinely needs (source files included), and it earns its keep at any window size.
The ~90k local model is the harness's first consumer, not the reason the rule exists.

The mechanics follow from that frame. Per-task context manifests with "load only this, nothing
else" are how signal is selected; the one-layer atomicity rule keeps a task small enough that its
signal is identifiable; the heuristic that *if the manifest starts to feel like "load everything,"
the task is too large and must be split* applies the same test at the boundary — "load everything"
is the maximum-noise configuration, whatever the window; and the cold-resume Session Handoff stops
signal being re-derived from nothing. Reading these as budget workarounds is precisely what lets a
frontier consumer conclude it is licensed to load everything. It is not — which is what makes the
same harness serve a 27B consumer and a frontier one: the rule is tier-independent, so the tier is
a configuration, not a fork. Embodied mechanically by the shared task primitive (see **Foundation**
below).

### Code is the documentation; one fact, one place; scale by splitting

Docs exist only for what can't be derived from reading the source. Single source of truth —
cross-reference, never duplicate. Adding features adds *files*, not length to existing ones.

Crucially: **apply this to the harness itself.** A setup breaks its own rule the moment it grows
300+ line workflows that duplicate one another, or an `AGENTS.md` that inlines the directory tree
and so duplicates the filesystem. Both are addressed mechanically by the Foundation below (the
shared task primitive collapses the duplicated workflows; the generated index replaces the inlined
tree). The principle is what makes those the right moves.

### rules → tests → tasks traceability

A real integrity system, not a folder of docs. Business rules are the root; every rule derives a
test; every test maps to an atomic task. `check-rules` audits the closure **semantically** — by
what a test exercises, not by name — which is what makes it a closed loop rather than a checklist.

### The doc-maintenance loop is a cycle, not a folder

`sync-docs` (code→docs), `optimize-docs` (anti-rot audit), and `check-rules` (rules→tests coverage)
form a maintenance *cycle*. This is the thing that keeps the docs from rotting into the very state
the philosophy warns against — and it's why the docs can be trusted as the single source of truth.

### Why-framing over intimidation

Small models follow rationale better than threats — threat-framing tends to work *worse* on them.
Give the reason for a rule, not a penalty for breaking it. This is why the Foundation relocates the
health signal out of the model (deleting any "MANDATORY / protocol violation" compliance ritual): a
proxy metric backed by intimidation is doubly wrong for a weak local model, which will emit the
token and ignore the instruction.

---

## Foundation — the portable `.agent/` layer the harness builds on

Before the harness work below is meaningful, the base setup must be **portable**: framework- and
language-agnostic, driven by any agent (Claude Code, a local 27B Q4 at ~90k context, or anything
else), and droppable into a new repo by editing one file. These are the mechanics that get it
there. They are stated here as what the foundation *is* — the harness assumes all six are in place.

### 1. One manifest, bound at install time

`manifest.yml` is the single source of every project-specific binding. Portable workflow
*templates* reference variables; the docs the runtime model actually reads are **rendered** from
those templates once per repo, with the literals inlined.

Binding at **install time**, not runtime, is deliberate — on deterministic grounds, not capability
ones. Runtime resolution would make `manifest.yml` a mandatory context load on every workflow run:
portability paid for on every session, at any tier, whether or not the indirection is ever
exercised. And literals are *statically checkable*: a rendered `make check` can be verified against
the manifest by the staleness/tamper checker and the command-resolution checker, while an indirect
instruction can only be verified by watching a model resolve it. Determinism, not condescension —
that a weak model also follows literals more reliably than indirection-plus-lookup is a bonus, not
the load-bearing argument. So:

- **Templates** (portable, shipped with the harness) reference manifest variables:
  `{{commands.check}}`, `{{paths.tests.integration}}`, `{{taxonomy.layers}}`.
- An **install/render step** (a small script, or a capable model running an `install` workflow once
  per repo) produces flat, literal, project-bound workflow files from templates + manifest.
- The **runtime model** reads only rendered docs — `make check`, `backend/tests`, no indirection.

Portability lives in the generator; the runtime model gets literals. The one new maintenance duty is
re-rendering after a manifest edit, guarded by the same staleness check as the generated index (§4).

The manifest holds *all* bindings — commands, the test-dir layout, the layer taxonomy, the
test-tier taxonomy, and the seam tiers (Forward vision):

```yaml
taxonomy:
  layers: [backend-model, backend-handler, frontend-state, frontend-ui]  # the atomicity axis
  test_tiers:                        # data-driven tiers; no hardcoded classification
    - {id: integration, tdd: rule-tests, cmd: test-backend}   # TDD-mandatory only when a test enforces a business rule
    - {id: unit,        tdd: optional,   cmd: test-frontend}
    - {id: e2e,         tdd: never,      opt_in: true, cmd: test-e2e, notes: frontend/e2e.md}
    - {id: manual,      automatable: false}

paths:
  business: .agent/business          # the domain list IS this directory's files (minus overview.md)
  tests: {integration: backend/tests, unit: frontend/tests, e2e: frontend/e2e}
# commands live under development.commands (check / test / build / migrate-* / …)
```

Two schema constraints: **every `cmd` id resolves** to a key under `development.commands`; and there
is **no `domains:` key** — the domain list is already the filesystem (`paths.business/*.md` minus
`overview.md`), and a manifest copy would drift the same way an inlined tree does. Templates that
need the domain list derive it by listing `paths.business`.

### 2. One shared task-decomposition primitive

The machinery common to feature-planning, bug-fixing, and plan execution lives in exactly one
template — `task-primitive.md` — and the workflows compose it: atomicity rules, context-manifest
rules, verification tiers, the task template, the pre-flight gate, and the completion ritual. The
completion ritual is **parameterized, not flattened**, preserving a real lifecycle distinction:

- *fix plans*: plan file **deleted** at completion (the plans dir holds active work only; git
  history is the archive) + a troubleshooting-doc update.
- *feature plans*: plan file persists with status `complete`.

The planning workflows shrink to just their Phase 1/2 fronts (domain-rules + test-plan for features;
reproduce + root-cause for bugs) and compose the primitive; the execution workflow points its
cross-references at the primitive rather than at section numbers in sibling files. The cost
duplication imposes is **drift**, not tokens — a session loads only one workflow, so extraction saves
roughly zero runtime context; its value is one place to generalize instead of three to keep in sync.

### 3. Agnostic test taxonomy

Test tiers are **data in the manifest** (§1's `taxonomy.test_tiers`), not baked into the procedure.
Each tier declares its id, TDD policy, opt-in-ness, and command. A repo with a different stack
redefines its tiers in YAML and the templates are unchanged. Three details a naive four-key row gets
wrong:

- **TDD conditionality.** The rule is not "integration ⇒ TDD-mandatory"; it is TDD-mandatory *when a
  test enforces a business rule*. Encode `tdd: rule-tests`, not `tdd: mandatory`.
- **Per-tier prose survives.** Operational guidance no YAML row can hold (spec auto-discovery, "keep
  the e2e surface thin — smoke, not a per-rule mirror", the e2e-vs-manual judgment call) lives in the
  task primitive (§2) or a per-tier doc named by the tier's `notes:` field.
- **Opt-in as data.** `opt_in: true` lets *any* agent — whatever its own house rules about, say,
  browser automation — know mechanically that a tier is never a completion gate and runs only on
  explicit request. The tier's status is declared in the repo's data, not assumed from one agent's
  config.

### 4. Generated doc index, not an inlined tree

The session-injected `AGENTS.md` carries no hand-maintained copy of the filesystem. The doc map is a
**generated** index with a staleness check — generation is the mechanism, not an option; an
on-demand index that is still hand-maintained just relocates the rot.

- **Built from the files**: each entry's one-line description comes from the file's H1 / first line /
  front-matter, preserving the per-file summaries a bare `ls -R` loses.
- **Regenerated** by a make target or hook; **guarded** by a staleness check that fails when the
  index and the filesystem disagree, so it *cannot* drift the way a hand-edited tree does.
- **Loaded on demand** (or kept as a small generated, injected file) instead of paid for up front.

Every place that maintained or depended on the inlined tree (the `sync-docs` / `rules` / `optimize-
docs` notes about updating `AGENTS.md`) becomes either "regenerate the index" or a reference to it.

### 5. Health signal in the hook, not the model

Session-start injection health is verified by the hook printing its own confirmation line to the
user — zero model tokens, no compliance theater. Any mandatory `"…initialized"` emission by the
model is a proxy metric (it proves the model read the top of one file, nothing more) and a weak
model will emit the token while ignoring the instructions it claims to enforce. Model compliance is
instead verified through **observable behavior**: did the session load the right manifest, run the
project's checks, honor its context manifest.

### 6. Branch/plan awareness at session start

The SessionStart hook injects, alongside the doc index, `git branch --show-current` and the
`Status:` lines of the active plan files (`.agent/plans/*.md` and `.agent/plans/fix/*.md`). A few
lines of shell; works for any agent the hook can reach. Cold-resume handoff covers *within* a plan;
this covers *finding* the plan, so orienting to in-flight work isn't left to the user's prompt or to
the model rediscovering it.

---

## Forward vision — what makes it a *personal harness*

The core above is portable but still assumes **one model doing everything** — a single slot at a
single tier. A "scaling harness" usually means more than that. These are the pieces to design if
the harness is the destination.

### Model/role tiering & sub-agent seams

The workflows already *are* the natural sub-agent seams — each has a sharp, minimal context
boundary that's effectively already spec'd. The seams and their model tiers are **declared as data
in the manifest** (`seams:`), not baked into prose (D038): "27B everywhere", "frontier everywhere",
and mixed are three configurations of one design, not three designs. The declaration ships early
(P1), guarded by the C12 schema check (P3) — every seam declares a known tier, and `auditor` must
be `strong`. Enforcement is the later piece (P5) and is per-harness glue; a declaration gates
nothing by itself.

```yaml
seams:
  planner:        {tier: strong, max_task_layers: 1}
  executor:       {tier: any,    max_task_layers: 1}
  doc_maintainer: {tier: mid}
  auditor:        {tier: strong}   # semantic judgment; never cheap, at any target
```

Tier vocabulary is `cheap | mid | strong | any`. The context boundaries below are the same whatever
the tier — which is what makes per-seam model assignment viable rather than aspirational:

| Seam           | Workflow                     | Context boundary                |
|----------------|------------------------------|---------------------------------|
| Planner        | `plan-feature` / `fix-bug`   | domain rules + interaction table|
| Executor       | `execute-plan` per-task loop | one task's manifest only        |
| Doc-maintainer | `sync-docs` / `optimize-docs`| git diff + target doc           |
| Auditor        | `check-rules`                | `business/` + tests             |

The assignments, briefly. Planner at `strong`: rule reasoning and decomposition, the one seam where
raw capability earns its keep. Executor at `any`: the narrowest boundary in the system, one task's
manifest only — and `max_task_layers` is the one knob whose right answer genuinely differs by tier
(task granularity), which is exactly why it is data rather than procedure: expressed as data, the
procedure stays identical across tiers. Doc-maintainer at `mid`: after P3 the routing is mechanical,
checker work; the core act — "can an agent derive this from source?" — is judgment.

Auditor at `strong`, always. That assignment deserves its own justification, because the naive
version contradicts a north star above: `check-rules`' whole value is that it audits
**semantically** — that is what makes the rules→tests→tasks loop closed rather than a checklist.
Deciding whether `TestUpdate_SignaturePending` truly covers "a tab with a pending signature cannot
be edited" is judgment, and a false ✅ anywhere in that loop *silently corrupts the integrity
loop* — the worst place in the system to be wrong. C8 has already removed enumeration from the
model entirely (a script lists every rule and every test), so the seam is strong-model
*verification* over a script-produced list — cheaper and stronger than any model-split version.
And the assignment was never about the weak tier: a frontier model's false ✅s are more convincing,
which makes them more dangerous, not less.

### `manifest.yml` as a permission gate

The manifest already knows the project's shape (services, commands, and — via the binding layer
(Foundation §1) — paths and taxonomy). It is the natural place to *declare* what a given seam is
allowed
to run, touch, or invoke: bindings and permissions want to live in the same file. But a YAML file
gates nothing by itself — it is the **source** an enforcing layer reads. Enforcement lives in
whatever the harness provides (Claude Code settings/hooks permissions, a local runner's
allowlist), and mapping the manifest's declarations onto each harness's enforcement mechanism is
per-harness glue that must be built, not assumed.

### Branch/plan awareness at session start

Folded into the Foundation (§6) — it's a foundation-layer mechanic (a few lines in the existing
SessionStart hook), not harness vision, so it shouldn't wait behind the harness work.

### Multi-slot Session Handoff

The handoff is currently single-slot — the template says "overwrite, don't append." Scope this
correctly before designing anything: the handoff lives **per plan file**, so parallel work across
two *plans* already has two independent handoffs. The single slot bites only for parallel lines
*within one plan* — which is exactly what a harness that fans a plan's independent tasks out to
sub-agents (see the seams above) would create. That, and only that, is the case a multi-slot
handoff needs to serve.

---

## Where this points

The invariant core (North stars) is worth preserving as-is. The Foundation makes it portable. This
vision — seams with model tiers, the manifest as the permission source, branching handoff — is what
turns a portable doc system into a *personal agent harness*. Sequence it after the Foundation is in
place: you want the agnostic binding layer built before you start handing seams to different models.
