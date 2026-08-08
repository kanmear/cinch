# cinch — rebuild plan

Deliberately short. If this document grows past two pages, that is the first symptom of the failure
it exists to avoid.

---

## Standing rules

1. **Pure function of the tree plus HEAD.** No stored state, no cache, no database. Run it twice
   anywhere, get the same answer.
2. **Every check ships with the mutation that proves it catches something.** Not later. In the same
   commit.
3. **No warning baselines.** A warning is either fixed or the check is deleted. No ignore lists, no
   "known warns", no accepted-red. This does not forbid `cinch:ignore` — a suppression says "this
   fired and I've decided to live with it, it'll fire again tomorrow"; a declaration says "this rule
   is outside the check's domain, nothing is pending." `cinch:ignore` on a rule is the second kind:
   some rules assert a fact a test can't point at (a UI-derivation rule, the absence of a flow), and
   that's a property of the rule, not a coverage gap. It stays a declaration and not a baseline by
   three conditions: it lives next to the rule, never in a central list; it carries a reason, so a
   genuine N/A can't be told apart from someone dodging a red check; and it is inventoriable
   (`cinch ignores` lists every one with its reason) without gating anything, so a periodic read stays
   cheap. The line: ignore means no test *can* enforce this, never no test *does yet* — the second is
   a coverage gap and belongs as red. Same principle as the `rule-reword:` escape — force the decision
   at the point of the change, leave no queue behind.
4. **Five-minute test.** At any point you must be able to explain the whole tool to someone in five
   minutes. When that stops being true, stop adding.
5. **Give-up condition, written now:** if a repo requires a config key that describes doc
   *structure* rather than naming a *value*, single-source templating has failed — fall back to
   copies. (`layers.business.present` was the line the first attempt crossed.)

---

## Stage 0 — Bare shell

Strip cinch to: the Go module, a README stub, and the philosophy doc. Delete the decision log, the
roadmap, the seams, the event log, the fragments, the index generator, and every checker.

Keep as reference material *outside* the repo: the mutation-test score sheet, and the audit findings.
They are evidence, not code.

**Done when:** the repo is explainable in one paragraph.

---

## Stage 1 — Three checks

One binary, no config. Each lands with its mutation test.

| Check | Fires on | Level |
|---|---|---|
| **links** | a relative markdown link under the docs dir that doesn't resolve | error |
| **rules** | a rule ID with no `// cinch:rule` marker; a marker resolving to no rule ID | error |
| **coupling** | a rule's item text changed and its marked test file didn't | **blocks; escape via `rule-reword: <ID>` in the commit message** |

Notes that already cost a round to learn:

- Rule text is **item-scoped** — ID line through the last line before the next item. Not the opening
  line.
- Normalize whitespace before comparing. Frequent use of the escape hatch means the check is too
  sensitive; fix normalization, never add tolerance.
- Coupling is a **transition check** — its window is the working tree against HEAD. It is
  invisible post-commit and must say so when it no-ops, or silence reads as a pass.
- No doc index. `ls -1R` is always correct.

**Done when:** three checks, three mutations, `cinch check` runs on any repo with no configuration.

---

## Stage 2 — Philosophy and workflows in cinch

Philosophy is copied verbatim; it has no variables and needs no renderer.

Workflows become templates with **value substitution only** — commands and roots. No conditionals,
no fragments, no shape config. Write your own substituter; an undefined variable is an error.

Rendered output is committed to the consumer, carries a `generated — do not edit` header, and the
render is idempotent so a re-render diff proves tampering.

**Done when:** cinch renders its workflow set into one real repo and that repo uses them.

---

## Stage 3 — Second repo, and let it strain

Render into a second real project. **Do not fix what breaks.** Record it.

Expect the first strain to be a workflow that is *wholly inapplicable*, not a path that's wrong.
Collect three or four failures before designing anything — fragments were invented last time to
solve a problem observed once.

**Done when:** you have a written list of what substitution could not express.

---

## Stage 4 — One mechanism, or fall back

From the Stage 3 list, decide:

- **Whole-file selection from key absence** — a repo that declares no rules layer doesn't get the
  rules workflow rendered. This is data, not a language, and is the expected answer.
- **Anything requiring conditional sections inside a file** — the give-up condition. Fall back to
  copies per repo and keep the checks.

**Done when:** the decision is made and written in the README, including which way it went.

---

## Stage 5 — Third repo

Ideally one neither of us shaped, and not the same stack. This is the only real portability
evidence; two repos built around the tool prove nothing.

---

## Release readiness

Small items, none of them urgent before Stage 3.

**Distribution** — `go install` path that works, tagged versions, a `--version` flag, and a single
static binary with no runtime dependencies.

**First-run experience** — `cinch check` on a repo with no rules and no docs directory should exit 0
quietly, not error. Zero configuration is the default; a manifest is only needed once templates are
in play.

**Error messages** — every failure names the file, the line, and the fix. `C2 error` is not a
message. This is the single biggest difference between a tool you'd hand someone and one you
wouldn't.

**Exit codes** — 0 clean, 1 findings, 2 usage error. Stable, documented, greppable output shape.

**README** — what it checks, what it deliberately doesn't, the give-up condition, and an honest
"unproven" section. The last one is what keeps you from believing your own green lights.

**CI example** — one workflow file people can copy. Note which checks are transition-scoped and
therefore hook-only.

**License, and a stance on contributions.** "Personal tool, issues welcome, PRs unlikely" is a
complete and honest answer, and it protects rule 4.

---

## What is deliberately not here

Seams and model tiering. Permission gates. The event log. Compaction anchors. A decision log.
Semantic auditing of any kind. Fixtures. A doc index.

Each was a real answer to a real problem — none of which you had yet. Add any of them only when its
absence hurts, and only one at a time.
