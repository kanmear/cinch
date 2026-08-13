# project_deltadocs — dead references left by the "prune trash" commit

Status: **proposed** — not started.

Target repo: `project_deltadocs` (planned here per the "all in cinch plans dir"
decision). This plan is a smoothing pass over dangling artifacts the consumer's
own `bc0b99f` ("docs: prune trash") left behind without finishing the removal.
Pure cleanup — no new behavior, no cinch change. Each item is a **decision** (the
artifacts are load-bearing *or* truly dead either way, so the call is
"restore vs. finish removing", and mostly it's the latter).

## Item 1 — `make drift-test*` targets are dead (D1)

**What's broken:** `scripts/drift-test.sh` reads `.agent/drift/{pinned-commit,
row-*.patch,expectations.json}` (script lines 62-75, header lines 9-16), but
`bc0b99f` deleted the entire `.agent/drift/` directory (per its diff: all ten
`row-*.patch` files, `expectations.json`, `pinned-commit`). The Makefile still
advertises four working targets (`drift-test`, `drift-test-semantic`,
`-semantic-pi`, `-pi-baseline`) whose script immediately fails with "pinned
commit … not reachable".

**Is it dead or load-bearing?** The score table this harness reproduces (P5.6)
was the consumer's adversarial strain-test of cinch-over-consumer — genuinely
useful as cinch's "Stage 3: a second real repo, letting it strain". But it
brittles quickly: `drift-test.sh` still greps for cinch's old C-numbered rows
(row-01-c8, row-03-c2, …) and old check language, and its `.agent/drift` files
were already committed/uncommitted in a shape that clashes with the new checks.

## Decision needed

- **A — finish removing (recommended).** The strain story has effectively
  migrated to cinch's own `make test` mutation fixtures; this consumer-side
  duplicate is unmaintained (references deleted C-numbers, resolved by
  `cinch-0.1.0.md`'s wait) and its fixtures are gone. Delete `scripts/drift-test.sh`
  and the four Makefile targets (and the `.PHONY` references), plus any remaining
  `drift` scrap. Clean, honest: if it can't be kept green, don't advertise it as
  a working target.
- **B — restore and port.** Rebuild `.agent/drift/` from git history, re-map the
  rows onto current check names (no C-numbers), keep the pinned-commit flow.
  Higher value if the consumer wants a *real* end-to-end adversarial corpus, but
  it's real work for an artifact cinch's own fixtures now cover.

## Item 2 — AGENTS.md references a pruned artifact: `.agent/events/events.jsonl`

**What's broken:** AGENTS.md:55-58 instructs agents to "append one JSONL event
to `.agent/events/events.jsonl` (id `D<n>`/`E<n>` …)". `bc0b99f` deleted
`.agent/events/events.jsonl` and there's no `.agent/events/` directory. Nothing
recreates it, so an agent following AGENTS.md appends to a nonexistent path.

**Is it load-bearing?** The episodic-memory convention was real prior to the
prune (E-numbers in the git log). Either the convention was retired (and
AGENTS.md is stale) or it was accidentally swept up.

## Decision needed

- **A — retire the convention (recommended if it's not being used).** Remove the
  "Episodic memory" paragraph from AGENTS.md. The corps is already the memory;
  git is the archive (matches the repo's own "handoffs go to plan files /
  historical artifacts are deleted" posture). Zero ongoing cost.
- **B — restore it properly.** Re-add `.agent/events/` with a real `.gitkeep` +
  a documented schema, decide whether it's tracked or gitignored (it *was*
  tracked pre-prune), and reword the AGENTS.md line so the path actually exists
  and the cost is owned deliberately. Only worth it if the event log is driving
  real decisions.

## Item 3 — Makefile `.PHONY` drift (D7)

The first `.PHONY:` (Makefile:1) lists only `drift-test-semantic-pi` /
`drift-test-pi-baseline` while the cinch block's `.PHONY` (Makefile:246) lists
all four plus `render`/`check-harness`/`ignores`. If the targets are deleted
(Item 1 A) the leftover `.PHONY` entries must go too; regardless, reconcile so a
file named e.g. `render` can't shadow the target.

---

## Steps

1. (Item 1, per decision) delete or restore `scripts/drift-test.sh` + Makefile
   targets + `.PHONY` lines + the Makefile comment blocks referencing the
   "P5.6 drift-test score table".
2. (Item 2, per decision) edit AGENTS.md — drop the episodic-memory paragraph,
   or re-add `.agent/events/` + schema.
3. (Item 3) reconcile `.PHONY`.

## Verification

- `make help` and `make -n <target>` list only live targets.
- `grep -rn 'drift\|events.jsonl' AGENTS.md Makefile scripts/` returns only
  intentional survivors.
- No behavioral path depends on the removed artifact (search for `events.jsonl`
  and `drift` across `.githooks/`, `.claude/`, `.pi/`, harness files).

### Critical files

- `project_deltadocs/Makefile`
- `project_deltadocs/scripts/drift-test.sh`
- `project_deltadocs/AGENTS.md`
- `project_deltadocs/.agent/` (if restoring Item 2)

### Relationship to other plans

Independent of cinch code. **Item 1's restored-variant (B) would depend on the
check renames** settled in this repo's own history; if the user picks A, this
plan is pure deletion and could be the first consumer plan executed. **Item 2
(A) is a one-line AGENTS.md edit** — safely standalone.
