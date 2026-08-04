# Findings: the cinch self-render audit

Status: **open worklist.** Written after P2 landed and the shared templates were rendered into
cinch's own `.agent/`. Records what that render exposed, what is decided, and what still needs a
decision. Supersedes nothing; feeds P1.

**Headline: P2 is complete. P1 is not.** The primitive renders and both planning workflows compose
it — P2's exit criterion, met. But rendering into a second real consumer showed the manifest binds
*values* and not *shape*, which is binding-layer work that Foundation §1 never anticipated.

**Revised after §7 below.** The audit originally concluded cinch has no domain layer. That was the
naming forcing a wrong answer: cinch has domain rules, several already enforced by tests. §1.3 and
§1.4 are narrowed accordingly, and §7 carries the correction.

---

## 1. What the render exposed

### 1.1 Bindings invented to satisfy templates

cinch's `manifest.yml` says it out loud:

- `paths.business: .agent/business` — "bind required by the shared templates; cinch has no business
  rules"
- `start-backend` / `start-frontend`, both `go run .` — "the bindings exist because the shared
  templates reference them"
- All four `test_tiers` rows resolve to `go test ./...`; all three `paths.tests` entries are `tests`
- `taxonomy.layers: [core, version]` — `version` exists to make the list plural

The rendered output shows the consequence of binding one value into slots that assumed different
ones:

- `workflows/fix-bug.md` §1.2 — `Dev servers (go run ., go run .)`
- `workflows/fix-bug.md` §2.3 — `Search tests/ and tests/`

A binding whose manifest comment explains why it is fake is a shape problem wearing a value
problem's clothes.

### 1.2 Structure assumed but never templated

The templating pass bound commands and the tests root, then stopped. Structure *below* those roots
is still projectX's, sitting in portable templates as literals.

| Location | Literal |
|---|---|
| `check-rules.md` §2, §3 | `tests/handlers/`, `tests/models/` |
| `execute-plan.md` §Commit Conventions | `.agent/conventions.md` |
| `fix-bug.md` §1.2, §1.3, §2.1 | `.agent/conventions.md`, `.agent/backend/troubleshooting.md`, `.agent/frontend/troubleshooting.md` |
| `fix-bug.md` §5 | `.agent/backend/testing.md` |
| `task-primitive.md` §Completion ritual | both troubleshooting docs |
| `task-primitive.md` §Context manifest | `.agent/models/<entity>.md`, `.agent/<service>/conventions.md` |
| `sync-docs.md` §4 | the entire routing table — `api/`, `models/`, `frontend/`, `backend/` |

**Two are live bugs in cinch right now, not just portability debt:**

- `execute-plan.md` requires an unconditional load of `.agent/conventions.md` before the first
  commit message of any session. That file does not exist in cinch. A required step points at
  nothing.
- `.agent/e2e.md` names `make fixtures` as a gate. Fixtures are being deleted (D040).

### 1.3 Workflows partly inapplicable in this consumer

**Narrowed by §7.** The original claim — both workflows are structurally void because cinch has no
rules — was wrong. cinch has domain rules; they were simply never written down. What survives is
narrower:

`rules.md` asserts "Business files mirror `api/` files 1:1" and its file template links
`../api/<resource>.md` and `../models/<resource>.md`. That mirroring is projectX's structure, not a
universal, and cinch has no `api/`. That part is genuine shape variance.

`check-rules.md`'s classification (API-enforced / model-enforced / structural-UI, keyed to
`tests/handlers/` and `tests/models/`) is likewise projectX's taxonomy. The *procedure* — enumerate
rules, match tests semantically, report gaps — applies to cinch unchanged.

So: the rule-maintenance and coverage-audit workflows are applicable here; their projectX-shaped
classification and mirroring conventions are not.

### 1.4 `business/overview.md` is a placeholder — two causes, both upstream

The file exists to satisfy a template, and its own text admits it: "The shared workflow templates
bind to this directory and to this file, so it exists with this single doc."

**Cause one — an optionality disagreement.** C5 was fixed to treat an absent business layer as
legitimate; the templates still reference the directory unconditionally. A checker that permits
absence plus a template that assumes presence produces a placeholder; the model resolved the
contradiction the only way it could. General lesson: a placeholder file signals that a checker and
a template disagree about optionality. Fix the disagreement, not the file.

**Cause two — the name asked the wrong question** (§7). "Does this repo have *business* rules?" gets
a no from a CLI. "Does this repo have *domain* rules?" gets a yes. The placeholder's content is a
correct answer to the question the naming posed.

---

## 2. Decided

- **Fixtures are cut** (D040). Portability is exercised by two real consumers — projectX
  (full-stack, domains, plans) and cinch (Go CLI, no domain layer). Portability is unproven until a
  third real consumer exists; that gap is stated, not papered over. This audit is the evidence: a
  toy repo would have passed everything above, because it would have been given a `business/` and an
  `api/` to make the render succeed.
- **D005 stands.** The substituter stays literal — no loops, no conditionals. Whatever fixes shape
  must not put branching into files the runtime model reads.

---

## 3. Open: how templates vary by repo shape

Three approaches. The first is rejected; the other two are complementary.

1. **Conditionals in the substituter.** Breaks D005. Cheapest now, worst later.
2. **File-level selection from key absence.** A manifest that declares no `paths.business` does not
   get `rules.md` or `check-rules.md` rendered at all. No branching — the file list is data.
3. **Fragment composition.** A template is a base plus optional fragments; the manifest names which
   fragments compose. Substitution stays literal because inclusion is a file list, not a branch.
   This is "scale by splitting" applied to templates.

**Recommended:** (2) now, since it costs almost nothing and handles whole-workflow applicability.
(3) incrementally, one template at a time as each one's variance actually bites — `sync-docs`'s
routing table, `task-primitive`'s example manifests, `fix-bug`'s reproduce table. Doing (3)
wholesale before knowing which fragments matter would be speculative.

**A third gap, independent of both: structural paths below the tests root.** `check-rules` needs to
know how tests are grouped (`handlers/`, `models/`) and `paths.tests.integration` does not carry it.
Either `paths.tests` gains sub-keys, or the API/model classification moves from prose into tier
data.

---

## 4. Checker improvement this earns

**Extend C11 to backticked paths.** It missed every finding in §1.2 because those are
backtick-quoted paths in prose, not markdown links. Any `` `.agent/...` `` token in a rendered file
should resolve. Would have caught `conventions.md`, both troubleshooting docs, and
`backend/testing.md` automatically. Same twenty-line shape as the C13 purity patterns.

---

## 5. Immediate cleanups

Do **not** do these before the §3 and §7 decisions — deleting or renaming `business/` while
templates still require it just breaks the render. Do them as part of the fix.

- **Replace** `.agent/business/` rather than delete it (§7): rename to the chosen term, and write
  cinch's real domain rules into it with IDs. Delete the placeholder `overview.md` text either way.
- Drop the invented `start-backend` / `start-frontend` bindings and the `version` layer.
- Correct `.agent/e2e.md` (drop the fixtures reference).
- Collapse the duplicate test-tier bindings, or accept them once tiers can be declared absent.

---

## 7. `business/` is the wrong name, and it produced a wrong answer

cinch **has** domain rules. Several are already enforced by tests — they were never written down as
rules, so nothing looked for them:

| Rule (currently prose or implicit) | Enforcing test |
|---|---|
| Checkers are doc-upstream or lateral only (D009) | none — prose in `AGENTS.md` |
| The auditor seam is never cheap | **C12** |
| Templates name no stack and no harness | **C13 patterns** |
| Rendered output is never hand-edited | **C2 tamper branch** |
| Rule IDs are permanent and never reused | none — testable, untested |

The closure already exists here, running backwards: tests enforcing invariants nobody wrote as
rules. `check-rules` would have found this if `business/` had not declared there was nothing to
look at.

**Consequence for P4.** C8 and the `// cinch:rule` markers become applicable to cinch. That
exercises the rule→test→marker closure in a second consumer of a genuinely different shape — the
portability evidence the toy fixtures could not have produced (D040).

### 7.1 Open: the name

`domain/` with `paths.domain` is the recommendation. It generalises cleanly — an app's domain is
Projects and Signatures; a CLI's domain is rendering, checking, and templating. `rules/` is more
literal but collides with the `rules.md` workflow. `invariants/` reads heavy in an app repo.

Renaming touches `paths.business` and every template referencing it; it is a mechanical change but
not a small one, and projectX renames with it.

### 7.2 Open: the log currently holds two kinds of entry

`decisions.jsonl` mixes *we chose X once* (D003, Go) with *X must always hold* (D009, the direction
rule). The second kind is a rule living in the wrong store.

Proposed split: the rule text lives in `domain/*.md` with an ID and a marker-tagged test; the log
entry keeps what only it can hold — the alternatives rejected and why — cross-referencing the rule
ID. Different questions, not duplication. Needs a pass over the existing entries to classify them.

### 7.3 The guardrail

**A rule earns a place in `domain/` only if a test can enforce it, or it constrains a future
decision.** Otherwise it is a log entry.

Without this, cinch accumulates rules as ceremony — the GROW failure (D021) in a different costume.
Worth writing into the domain overview as its first rule.

---

## 8. For the session picking this up

Read this file, `docs/ROADMAP.md` §P1, and `decisions.jsonl` entries D001, D005, D009, D010, D021,
D037, D038, D040. Not the whole roadmap and not the whole log.

Two decisions, in order:

1. **§7.1** — the name. It determines what §3 is deciding *about*, and whether §5's first item is a
   delete or a rename.
2. **§3** — file-level selection, fragments, or both. Everything else in §5 waits on it.

§7.2 (splitting the log) and §7.3 (the guardrail) can follow at any point; neither blocks.

Log each outcome as a decision entry with the alternatives, per `docs/decisions-log.md`.
