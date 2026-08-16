# cinch — workflow postconditions belong in `cinch.yml`

Status: **proposed** — not started.

**This plan ships no cinch code.** It is a discipline, adopted using
machinery that already exists. If executing it grows a manifest key or a
sixth check, it has been executed wrong.

## Context

A workflow is prose until something can decide whether it was followed.
`cinch workflow <name>` prints markdown; whether the agent that read it
actually ran the tests, or actually marked the plan shipped, is currently
unverifiable — the workflow's postconditions live only in the workflow's own
sentences.

The dispatch machinery for fixing that already shipped and needs nothing
added: `hooks.<event>.<name>.run` with `when` path-prefix scoping, three
events (`pre-commit`, `commit-msg`, `post-commit`), failures accumulating
rather than fail-fast, skips stated on stderr rather than silent — all
documented at `README.md:158-175` and implemented in
`internal/cinch/hook.go`. The path scoping in particular is exactly what
workflow postconditions want: a postcondition that only applies when the
commit touches a given subtree.

This is the rule→test marker pattern one level up. A rule doc claims
something and a marker ties it to a test; a workflow claims something and a
manifest entry ties it to a command. In both cases the tie is structural
rather than semantic — which is a real limit, not a hidden one (see the
honest accounting in `assessment-direction.md` §3.1), and it is still better
than prose alone.

## The discipline

> When a workflow is adopted or written, its checkable postconditions get
> written into `cinch.yml` in the same commit.

"Checkable" is load-bearing. Most of what a workflow says is judgment and
stays judgment. The subset worth encoding is the subset a script can decide:

| The workflow says | The manifest entry |
|---|---|
| fix-bug: tests must pass before the fix lands | `hooks.pre-commit.<name>.run`, `when` scoped to the source subtree |
| a plan is marked shipped before its work merges | a `hooks.commit-msg.<name>.run` script |
| docs under a subtree are re-rendered when touched | `hooks.pre-commit.<name>.run` with `when: <subtree>` |

The gain is not that the script is clever — cinch guarantees dispatch, not
your script's correctness (`README.md:177-181`, the extension boundary). The
gain is that the postcondition stops being a sentence an agent may or may not
have read, and becomes something the commit boundary decides.

## Decision needed — where the rule lives

A convention with no home is precisely the drift cinch exists to catch, so
this is the one real choice here.

- **`AGENTS.md` under `## Rules`** — where this repo's other standing
  conventions live, and what an agent reads first.
- **A `README.md` section** beside the hooks documentation — where a new
  consumer would look for it.
- **A numbered `**ID-NNN**` rule under `.docs/`** with a `// cinch:rule`
  marker, which would subject the convention to the `rules` check itself.

Recommend the **README hooks section plus one `AGENTS.md` line**, and
explicitly *not* minting a rule ID yet. A rule ID demands a marker, and the
only marker available today would sit above a test that asserts something
trivial about dispatch — which is the weakness `assessment-direction.md` §3.1
names, imported deliberately. Mint the ID when there is a second consumer
actually following the discipline, and mint it against whatever that
consumer's practice turns out to need.

## Non-goals

Stated so the next session doesn't reopen them:

- **No new manifest keys.** No `hooks.<event>.<name>.workflow` back-reference
  tying an entry to the workflow it enforces; the entry's `<name>` carries
  that, and a back-reference is a schema whose only consumer would be a
  check that verifies it.
- **No seam guard** — cinch verifying that registered entries are the ones a
  workflow expects. That deferral is correct for the reason given in
  `assessment-direction.md` §7.1: it re-implements at commit granularity what
  a harness already does at action granularity.
- **No workflow-template changes.** The templates describe process; the
  postconditions live in the consumer's manifest, because which ones are
  checkable is a per-repo fact.

## Step 1 — write the convention down

Add it in the two places chosen above. Keep it to a paragraph: the rule
sentence, one worked example, and a pointer to the hooks documentation for
the mechanics.

## Step 2 — self-host it

This repo's `cinch.yml` currently registers exactly one entry
(`hooks.pre-commit.build.run: make test`), which predates the convention and
enforces no workflow in particular. Pick one shipped workflow whose
postcondition is genuinely decidable, register it, and have that workflow
name the entry that enforces it. One real example beats the paragraph.

If no shipped workflow turns out to have a postcondition worth encoding, that
is the finding — record it here and stop, rather than inventing one to make
the convention look used.

## Verification

- This repo's `cinch.yml` carries at least one workflow postcondition entry
  beyond `build`, and the workflow it enforces names it. Self-hosting is the
  proof, same as everywhere else in this repo.
- Deliberately break the postcondition and confirm the commit is blocked, and
  that the failure message identifies the entry. A postcondition with no
  demonstrated failing state is a proxy metric.
- `./bin/cinch check` clean; `make test` green.

### Critical files

- `cinch.yml` (the self-hosted entry)
- `AGENTS.md`, `README.md` (the convention)
- The chosen workflow under `internal/cinch/docs/templates/` — *only* to name
  its enforcing entry, not to restructure it

### Relationship to other plans

Pairs with `cinch-context.md`: context is the workflow runtime's entry point,
postconditions are its exit criterion. Independent of
`template-stack-agnosticism.md`, though if both land, the template touched
here should be one already genericized there, to avoid two passes over the
same file.
