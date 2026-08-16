# cinch — assessment: claims, direction, and position

Status: **analysis** — point-in-time, 2026-07-22.

Not an implementation plan: a record of a critical assessment and a strategic
discussion, written so a future session can pick up the decisions without the
conversation. Claims marked *verified* were checked by building and running
the tool on this machine, not taken from the docs.

*Amended 2026-08-16, minimally:* citations to `.docs/plans/cinch-0.1.0.md`
repointed at git history (the file was deleted in `0ed1ebd`), §3.5 sharpened
after the `paths.docs` defect turned out to be larger than recorded, and §8's
second crack marked resolved. The analysis itself is unchanged — its value is
that it says what was believed when.

*Direction items closed since:* §6.1 `cinch context` (`276aedd`), §6.2
workflow postconditions (`24debab`), §8's first crack (`2d572e8`). Their plan
files are deleted, per this repo's rule that shipped artifacts are not kept
alive; git is the archive. §6.3 (small-repo adoption) remains open and
deliberately unplanned, and §6.4–6.6, §7 and §9 are standing direction rather
than work items.

## 1. What the project claims

- **Core claim:** operational docs (rules, workflows, conventions read by
  humans and agents) drift from what the code does, and agents increasingly
  act on stale docs. cinch makes "the docs are accurate" a script-verifiable
  property on every commit.
- **Mechanically:** five deterministic checks (`links`, `rules`, `coupling`,
  `generated`, `commit`) under `cinch check`; `init`/`render`/`hook`
  self-enforce via generated git hooks; manifest-driven extension seam.
- **Meta-claim:** every check ships a mutation fixture proving it can fail;
  no silent passes; no proxy metrics; the repo self-hosts as proof.

## 2. What it actually achieves

*Verified on 2026-07-22:*

- `make build` clean; `go test ./...` green; `./bin/cinch check` exits 0
  against its own tree. Self-hosting is real, not aspirational.
- The `generated` check genuinely makes rendered output tamper-evident
  (byte-compare against in-memory re-render, orphan detection, no-op-not-pass
  when unrendered).
- ~4.5k lines of Go including tests; the 0.1.0 plan records deviations
  honestly (wrong file lists, citation drift, a bug found-but-not-fixed).
  That plan was deleted in `0ed1ebd` under this repo's own rule that
  historical artifacts are deleted rather than kept alive; it is recoverable
  from git history, and the citations below name the commit rather than a
  path that no longer resolves.
- **The Stage 3 strain test had already run, contrary to the plan file.**
  The 0.1.0 plan (see `0ed1ebd`) said the `project_deltadocs` strain test
  "did not run"; it did. `~/code/project_deltadocs` is fully migrated: `cinch
  check` exits 0 there, all three hook events dispatch through cinch's seam,
  the hand-rolled `.githooks` triage is retired (per its own `cinch.yml`
  comments), and `scripts/docs-index.sh` is gone. The portability hypothesis
  has survived one real, structurally different consumer (`.agent` not
  `.docs`, custom manifest values, three hook events, path-scoped scripts).
  The plan file's stale claim was itself an instance of the failure mode this
  tool exists to catch — resolved by that plan's deletion (see §8).

## 3. Where the claims outrun the mechanism

1. **The rules check guarantees marker existence, not enforcement.** A
   `// cinch:rule ID-004` comment above a trivial or unrelated test
   satisfies the check. The link is structural (doc ↔ comment), not semantic
   (doc ↔ actual enforcement). The README's "so 'documented' and 'enforced'
   can't quietly diverge" is stronger than the check delivers; the divergence
   moves one layer down. PRINCIPLES.md is honest about this (principle 1);
   the README sells it harder than the principles allow.
2. **The coupling check catches sloppy workflows, not drift.** It is
   transition-scoped (working tree vs HEAD): it blocks a rule edit whose test
   edit is forgotten *in the same sitting*. A rule edit and its test edit in
   the *same commit* pass cleanly, and once committed, divergence is permanent
   and invisible. A guardrail against forgetfulness, not against drift —
   which is the stated failure mode.
3. **The central problem is unproven.** "Agents act on stale operational docs
   and break things" is a plausible prior, not a measured failure; no agent
   incident is cited. Forgivable at this tool's price, but the README's
   Overview reads as if the problem were established fact. (Counterweight:
   the author's own experience in project_deltadocs — the .md system "really
   helped" and produced real documentation for a real project — is evidence
   the *system* works, even if the specific agent-incident framing is
   unmeasured.)
4. **Self-hosting is a demonstration, not a validation** — until the
   deltadocs migration counted, which per §2 it now does. Remaining gap: the
   *small personal repo* use case is still untested (see §5).
5. **A live doc/code drift in the tool itself — and it is larger than a
   false README line.** The README claims `paths.docs` accepts "an absolute
   path." *Verified false:* with `paths.docs = /tmp/cinchtest/mydocs`, render
   writes to `<repo-root>/tmp/cinchtest/mydocs/...` — the absolute path nests
   under the root (`internal/cinch/render.go:212`, `filepath.Join` does not
   special-case an absolute second argument). The 0.1.0 plan (see `0ed1ebd`)
   flagged this exact bug as "not yet filed as follow-up work."

   *Re-examined 2026-08-16:* the write side nests, but the **read** side does
   not — `ResolveDocsRoot` (`internal/cinch/check.go:37-39`) honors an
   absolute value explicitly. So `links`/`rules`/`coupling` scan the absolute
   path (empty → clean) while `generated` verifies the nested copies (present
   → clean). All five checks go green while the rendered corpus is invisible
   to every check that reads it, and `{{paths.docs}}` in the rendered
   workflows points agents at the location render never writes to. The tool
   that exists to catch docs claiming what code doesn't do has a supported
   configuration in which drift is structurally undetectable. Fixed in
   `2d572e8`: both `paths.docs` and `paths.hooks` must now be repo-local, and
   the value is refused at manifest load.
6. **Version skew is a standing operational tax.** Every cinch upgrade
   reddens every consumer until re-render. Acknowledged, fix is exact, but it
   is a cost the README buries in a note. (See also
   `.docs/plans/cinch-version-pin.md` — the consumer-side pin is
   deliberately deferred.)

## 4. Verdict on claims

The mechanical claims (five checks, mutation fixtures, self-enforcement,
tamper-evidence, idempotent render) are substantially true and verifiable.
The *outcome* claim (prevents doc drift, protects agents from stale docs) is
weaker: cinch enforces **structural referential integrity** of a specific
doc convention, not **accuracy**. The semantic gap is acknowledged in
PRINCIPLES.md; the README's "closes that gap" overstates it.

## 5. Strategic context (from the author, 2026-07-22)

- Origin: the system lived as a collection of .md files inside
  project_deltadocs, where it demonstrably helped and produced real docs.
  cinch is the extraction of that system into a stack- and harness-agnostic
  tool.
- Semantic checks were dropped because the old cinch grew too fast (abused
  coding agents, author lost understanding of the project). PRINCIPLES.md is
  the memory of that failure.
- **Intent: a personal, highly opinionated helper — not a product.** Maybe a
  general-purpose tool later; for now the consumers are the author's own
  repos.
- **Stated worry: not wanting "a checker for the sake of green marks"** — the
  tool should do something useful besides being a portable center for the
  workflows.

## 6. What to pursue

Diagnosis behind the recommendations: the checks are only as useful as the
thing they protect, and right now the protected thing is *passive* —
`cinch workflow <name>` prints markdown and gets out of way. The value in
deltadocs was on the *working* side (docs that stay true, workflows that make
agents behave); cinch's center of gravity is currently on the verification
side. The useful direction is making cinch the **runtime** for the workflows,
not just their warehouse.

In order of value-per-line:

1. **`cinch context` — the session-start command.** The daily loop is:
   start session → re-orient → pick workflow → execute → commit. cinch can
   deterministically compress the first two steps, because every input is
   decidable: current branch, open plans under `plans/` and their status,
   last check result, the workflow trigger table, staged paths. Not a
   checker, no green mark — it does work. Harness-neutral in the same way
   the AGENTS.md lines are: any harness with a session-start hook calls it;
   any harness without one just has AGENTS.md say "run `cinch context`."
   It was deferred in 0.1.0 on holdability grounds ("0.1.0 is already at the
   holdability line"); 0.1.0 has since proven itself on two repos, so that
   argument has served its purpose. ~100 lines; reads state already computed
   (`index.go`, `title.go`, `check.go`); adds no new check to hold.
   → shipped, `276aedd`.
2. **Workflows as executable contracts, using the existing seam — no new
   code.** A workflow is more than prose if its postconditions live in the
   manifest: "fix-bug says tests must pass" becomes a `hooks.pre-commit`
   entry with a `when` scope; "plan marked shipped before merge" becomes a
   `commit-msg` script. The dispatch table (path scoping,
   accumulate-not-fail-fast) is exactly what workflow postconditions need.
   The work is *discipline, not features*: when a workflow is adopted or
   written, its checkable postconditions get written into `cinch.yml` in the
   same commit. The rule→test marker pattern, one level up.
   → shipped, `24debab`.
3. **Adopt on one or two small personal repos** (bevy-test, odin-landing,
   pistolhand, …). The only remaining experiment that answers the original
   question ("can I use this for other mostly personal projects?"). It will
   surface the one design doubt the deltadocs strain test can't: does a
   small repo want `cinch init`'s full scaffold (nine workflow templates +
   philosophy)? If adoption feels heavy, the evidence says the right form
   for small repos is `cinch check` + hand-written docs, with init's
   scaffold optional — a smaller default, not a new feature.
   *Deliberately unplanned as of 2026-08-16:* items 1 and 2 got plan files
   this pass and this one did not, by decision — it is an experiment to run,
   not work to specify, and specifying it in advance would prejudge the
   question it exists to answer.
4. **Freeze scope, deliberately.** The deferred list (seam guard, `cinch
   context` as a *checker*, `owns:` front-matter, auto version bumping,
   branch conventions) stays deferred *by decision*, not by drift. 0.1.0 is
   complete: entry point, self-enforcement, extension seam, two real
   consumers. The next version earns its existence only from what the
   small-repo adoptions break.
5. **Do not rebuild semantic/model checks.** They were dropped for the right
   reason, and PRINCIPLES.md already contains the correct replacement
   theory: judgment demoted behind decidable tripwires. The coupling check
   *is* the semantic layer in its honest form. If a specific semantic gap
   hurts in a real repo later, build the tripwire for that gap, with a
   mutation fixture, and nothing else.
6. **What to refuse:** plan lifecycle commands (`cinch plan new/done`) —
   a second mini-system with its own state to hold; revisit only if
   `cinch context` proves people live in plan status. Anything that makes a
   small repo adopt the full scaffold to get one check — opinionated is
   fine, all-or-nothing is how personal tools stop getting used.

## 7. Position relative to coding-agent harnesses

The overlap feels closer than it is. Harnesses and cinch attack the same
problem ("LLMs can't be trusted with a repo") on different axes.

What harnesses layer, in increasing hardness:

1. **Prompt-level** — system prompt, CLAUDE.md/AGENTS.md. Soft; the model
   can ignore it.
2. **Action gates** — permission lists, PreToolUse/PostToolUse hooks,
   plan-mode approval. Deterministic code around each tool call; hard, but
   per-action and per-session.
3. **Environment** — sandboxing, network isolation. Hardest.

All of it reigns in **the actor, in real time, for one session**, and it
lives in per-user, per-harness config that evaporates with the session and
binds only that harness's model. A harness is a speed limit.

cinch governs **the record, at the commit boundary, permanently**:

| | Harness | cinch |
|---|---|---|
| Constrains | the agent's actions | the repo's state |
| Granularity | per tool call | per commit |
| Lifetime | one session | until deleted from the repo |
| Binds | one harness's model | every committer, human or agent, any harness |
| Lives in | user/machine config | the repo itself, tamper-evidently |
| Contains an LLM | yes | no — fully deterministic |

The enemy differs too: harness mechanisms assume an **unreliable actor**
(the model might do something dumb right now); cinch assumes **drift**
(even a competent actor, after a refactor, leaves the record lying).
Containment vs. integrity. cinch's checks fire identically on a human
commit; a PreToolUse hook by definition only ever sees a model.

Two genuine overlaps, named precisely:

1. **Hooks as deterministic tripwires.** cinch's `hooks.*` + git hooks are
   the same *pattern* as PreToolUse/PostToolUse at a coarser grain (commit
   vs. tool call). This is why the "seam guard" deferral is correct:
   building it would re-implement, at commit granularity, what harnesses
   already do at action granularity.
2. **`cinch context` ≈ SessionStart.** Functionally what Claude Code's
   SessionStart hook and pi's system-prompt injection do — deterministic
   context computed at session start, handed to whatever model shows up.
   The difference is distribution: theirs is harness-internal config;
   cinch's is a command any harness (or human) can invoke, versioned in the
   repo. The deltadocs history is the proof of why that matters: hand-written
   per-harness SKILL.md shims and .githooks were rent paid to proprietary
   harness surfaces, until cinch's seam retired them.

Framing to keep: **harnesses are the runtime; cinch is the constitution.**
Runtimes get swapped; the constitution outlives every runtime, binds all of
them equally, and is the only layer a future agent with no memory of the
author's preferences will find still in force and still verifiable. The
vendor trend is to absorb more of the deterministic layer (better hooks,
context injection, repo-aware memory) — but the cross-harness, in-repo,
human-and-agent-binding layer is structurally outside what one vendor's
runtime can own. That is cinch's moat, and it only matters at personal
scale, which fits the stated intent.

This also answers the green-mark worry: a harness earns its keep visibly,
every session, by catching the model mid-mistake; cinch earns its keep
*invisibly*, by the gaps that never opened. The `cinch context` /
workflow-runtime direction (§6.1–2) matters because it gives the tool a face
in the daily loop, so its value isn't just the absence of rot.

## 8. Open cracks to fix (small)

- **Absolute `paths.docs` bug** — `internal/cinch/render.go:212`; README
  claims absolute paths work; they nest under the repo root. Fix or walk the
  README claim back. *Still live;* on re-examination it is a read/write split
  that renders drift undetectable rather than a mere false README line (see
  §3.5). Fixed in `2d572e8`.
- **Stale plan claim** — `.docs/plans/cinch-0.1.0.md` said the deltadocs
  strain test "did not run"; it did (see §2). **Resolved by deletion:** the
  plan file was removed in `0ed1ebd` along with ten other shipped plans, so
  there is no longer a stale claim to correct in place. The fact itself is
  preserved in §2.

## 9. The success metric

Not features shipped. In six months: the author's repos' docs still match
their code, `cinch check` still takes one person to verify end to end, and
the first thing run in a new session is `cinch context`. The tool's face
turns toward the work; verification stays quiet and load-bearing underneath.
