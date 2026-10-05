# 7. Workflow prose value and F1's scope

**Tier 3 — decide before investing. Cost estimate: cheap half is small; expensive half is
unscoped until the open question below resolves.**

## Provenance

**Both**, and this item's status has moved since the roadmap artifact was published — see
"What's changed since the roadmap" below before doing anything here. Audit findings **F1**
(the workflow templates reference a `manifest.yml` cinch never renders, so a consumer whose
setup diverges from the template's assumptions gets silent improvisation rather than an error)
and **F11** (the same document calls the whole workflow ceremony "a hypothesis with n=1").
Benchmark: two studies, both discussed in `~/code/cinch_bench/FINDINGS.md` §9.2 and §10.

## What F1 actually found

`renderAll` (`internal/cinch/render.go:75-118`) substitutes `{{var}}` tokens
(`varRe`, `render.go:26`) into every embedded template under `docs/templates`, and **fails
the whole render** if any template references a manifest variable that isn't set — see
`substitute`'s `missing` tracking (`render.go:38-49`) and `renderAll`'s `errorLines`
accumulation (`render.go:100-109`), which turns any missing variable into a hard error, not a
silent skip. That part already works as F1 would want it to, for the templates cinch ships.

F1's actual complaint is different: the *content* of the rendered workflow prose (not the
template substitution mechanism) instructs an agent to consult things like a `manifest.yml`
that describes project-specific structure the template can't know in advance — and if that
referenced file doesn't exist or doesn't match what the prose assumes, nothing in cinch's
render-time or check-time machinery catches it. The agent just improvises. This is a
content/instructional-design problem, not a template-substitution bug — closing it fully means
teaching the workflow prose to fail loudly when its *runtime* assumptions (not just its
render-time variables) don't hold, which the roadmap originally estimated as "the taxonomy
plumbing" — a larger investment than the template-variable check.

## What's changed since the roadmap: the ablation ran, and answered a narrower question than intended

The roadmap's original text for this item said: "Ship the cheap half of F1 now... Hold the
expensive half until a fifth arm says whether the workflow prose carries measurable value."
That fifth arm ran — `~/code/cinch_bench/wfablate.sh`, 24 runs across `full` /
`nowf-clean` / `nowf-dangle` arms — and found **no measurable read-time effect**: pi was
identical across all three arms on rule-added/marker-bound/docs-edited scoring; DeepSeek was
noisy in both directions. Full results in `FINDINGS.md` §9.2.

**That result does not settle F1-vs-F11, and the roadmap's framing of it as settling was
wrong — corrected in `FINDINGS.md` §10.** The ablation kept the *corpus* — 76 rules, IDs,
markers, the whole accumulated product of months of following the workflow ceremony —
present and identical in every arm; only the workflow *prose* was removed. What it measured is
"does an agent need the ceremony's instructions to consume a corpus that already exists,"
and the answer is no. It did not and structurally could not measure "does the ceremony's
prose produce a better corpus over time than working without it would have" — that's a
production-time claim, and the study only varied a read-time input.

Read this way, study 1 (the original with/without corpus A/B benchmark) is where the
workflows' value, if any, already shows up: the 95.8%-vs-85.4% with/without gap is a
measurement of what following the ceremony produced, banked and attributed to the corpus it
left behind. Studies 1 and 7's ablation don't conflict; they measured different halves of the
same lifecycle.

## The decision this item still needs

**F1's expensive half (teaching workflow prose to fail loudly on runtime-assumption
mismatches, not just render-time variable gaps) is still unscoped**, because the study that
would justify or deprioritize it hasn't run. What would settle it:

A **longitudinal** study, not another snapshot ablation: start from a repo with no corpus (or
a fresh, empty domain inside an existing one), run N feature tasks with the workflow prose
present versus absent, and grade **the corpus that accumulates** — rule count against rule
durability, ID collision rate, marker-binding rate, whether the admission test's rule/no-rule
boundary was respected, whether docs drift out of sync with code over the N tasks. This is
expensive (N sessions per arm, not one), but it's the only construction where the thing being
measured (corpus quality) isn't also the thing held constant across arms (which is what
happened in the ablation that already ran).

**Until that study runs, item 7's recommendation is unchanged from the roadmap's original
text on the cheap half, for an independent reason** — not because the ablation validated it,
but because it's a small, self-justifying improvement regardless of F1-vs-F11:

## The cheap half — do this regardless of the open question

Render-time failure when a template references an undefined key **already exists** for
manifest-variable substitution (`substitute`/`renderAll` above). What's actually missing and
still cheap: audit the *shipped* workflow templates under `docs/templates` for any place they
instruct an agent to consult a file, path, or structure that isn't itself a `{{var}}`-templated
(and therefore render-time-checked) reference — i.e., a hardcoded assumption about project
layout baked into prose rather than substituted. Where found, either template it (so a missing
value becomes the existing render-time error) or make the instruction conditional/discoverable
rather than assumed. This doesn't require the longitudinal study; it's tightening what already
ships to actually get the render-time guarantee F1 wants, for the specific gap F1 named.

**Resolution (effectively done, `09ef586`).** The workflow-template trim removed the
`manifest.yml`, taxonomy, `development.commands` and `dev-task-primitive` references from the
shipped prose, leaving `{{paths.docs}}` as the only substituted variable.
`TestWorkflowsNameNoUncreatedConfig` fails if any of those strings returns, and
`TestWorkflowReferencesResolve` fails if a template names a `workflows/*.md` the render
doesn't produce (both in `internal/cinch/templates_test.go`). It differs from the definition
of done below in two ways: the guard is a denylist of the known-bad strings plus workflow
cross-reference resolution, not a general check for hardcoded structural assumptions, and no
seeded-defect case was added to item 3's battery. The expensive half is untouched.

## Why tier 3, and why it stays open

This is the item the roadmap explicitly frames as "decide before investing" — the expensive
half is a multi-week design-and-build effort (a taxonomy for what counts as a runtime
assumption, a mechanism to check it, work across every shipped template) and committing to it
without evidence the ceremony produces a better corpus than working ad hoc would be exactly
the kind of premature investment items in tier 3 are meant to avoid. The correction in
`FINDINGS.md` §10 doesn't resolve the open question — it un-resolves a premature closure of it.

## Verification / definition of done

- **Cheap half**: an audit of `docs/templates/*` producing a list of any non-templated,
  hardcoded structural assumption in the rendered prose; each either templated (with the
  existing render-time failure now covering it) or reworded to degrade gracefully.
  Verification: seed a defect (a template instructing the agent to read a file that doesn't
  exist in a fixture repo) and confirm it now fails at `cinch render` or is clearly
  conditional, per whichever fix was chosen — add this as a case to item 3's battery.
- **Expensive half**: stays undecided until the longitudinal study described above runs.
  This item's remaining action is scoping and running that study, not writing code.
