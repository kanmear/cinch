# Rule Coverage Audit

Audit test coverage against documented domain rules in `{{paths.docs}}/`.

## Usage

Identify which domain rules lack test coverage, classified by testability.

## Instructions

### 1. Get the closure from the harness checkers

The rule set is **script output, not something to re-read by hand**. Run `cinch check` —
its rule check reads every rule item from the markdown under `{{paths.docs}}/`, then scans for
`// cinch:rule <ID>` markers in every tracked and untracked-but-not-ignored file (per the repo's
git ignore rules) outside `{{paths.docs}}/`. It reports, per rule:

- **unmarked** — no `// cinch:rule <ID>` marker anywhere in the tree: the rule is either untested
  (a coverage gap) or declared N/A (step 2)
- **unresolved marker** — a marker that names no real rule: dead syntax, fix it

`cinch ignores` lists every rule declared N/A. The checker's findings plus that inventory **are**
the enumerated list — `cinch check` reports one line per unmarked rule (`<ID>: no // cinch:rule
marker (or cinch:ignore declaration)`, no rule summary included) and one per unresolved marker
(`// cinch:rule <ID> does not resolve to any rule ID`); `cinch ignores` separately lists every
`cinch:ignore`'d ID with its reason. Read the rule's own text from its doc, not from the finding
line — the finding only carries the ID and location.

Your job starts where the script stops: the semantic half in steps 2–3. Do not re-enumerate the
rule set by hand — the checkers never miss a rule; a hand extraction can.

### 2. Classify each rule by testability

Categories:
**API-enforced** — the rule is enforced at the handler level and causes a specific HTTP response
when violated (e.g. a mutation rejected with a specific status when the caller lacks the
permission the rule requires). Look in the test directory covering your request-handling layer
(e.g. `<service>/tests/handlers`).

**Model-enforced** — the rule is enforced by the DB schema or model layer (UNIQUE constraint,
CASCADE, model-level guard). Look in the test directory covering your persistence layer (e.g.
`<service>/tests/models`). Examples: a uniqueness constraint, a cascade delete, an
immutable-history rule.

**N/A** — the rule describes display or structural behavior that cannot be tested automatically
(e.g. "display names are derived from another field"). Declare it in the doc by placing
`<!-- cinch:ignore: <reason> -->` directly under the rule item, so `cinch check` stops warning
about it as uncovered and `cinch ignores` lists the inventory. Not a test coverage gap.

### 3. Find test coverage

This is a **search with nonzero expected yield**: find the rules whose tests do not enforce
them. Expect to find at least one — rules rot as behaviour changes. Do not frame it as
"confirm each rule is enforced"; confirmation of an existing link is the leading question, and
it verifies the marker's existence instead of the test's meaning.

**Derive the mapping from the tests, not from the markers.** Read the tests in the locations
named in the classification — or `manifest.paths.tests` if no category named a directory — and
for each test write down, in one sentence, what its assertions actually enforce (the response
or state it pins, not its name). Then match each statement against the rules: which rule does
that assertion enforce? A test called `TestUpdate_<Subject>Pending` enforces "editing is
always free while a request is pending" (asserts 200 OK and content updated while the request
is pending) even though the names don't literally match — the assertions are the evidence, not
the name. This derived test→rule mapping is your answer; write it down before you consult the
markers.

**Then diff your mapping against the recorded one.** The markers — found by searching the tree
for `// cinch:rule <ID>` — plus the closure's report are the recorded answer. Read every
marked test and restate its rule's text and the test's assertions side by side, and flag any
tension *before* deciding. A disagreement is a conflict, not a doubt:

- the marked test's assertions contradict the rule's text → **TENSION** — report it; the content
  question — does the marker's test enforce the rule's *new* meaning? — is yours
- a rule you derived as enforced by a test that carries no marker above it → the marker is
  missing or misplaced — the rules check warns on every unmarked rule, so search the tree for
  `// cinch:rule <ID>` first and judge the marker's placement and its test's quality
- a rule with no derived enforcer → MISSING (cross-check against the unmarked list)
- an unresolved marker (a `// cinch:rule <ID>` naming no real rule) → judge the test the
  marker sits above: a correct test with a mistyped marker is covered — report the marker typo,
  not a missing test

A rule can be marked yet under-enforced (marker above a weak test); it cannot be unmarked under
a zero-unmarked closure. Read representative test files if you're unsure whether a test covers
a rule.

### 4. Report

Produce one table per domain file under a `## <domain>.md` heading, with **verbatim
evidence in every row**:

```
## <domain>.md

| Rule | Rule text (verbatim) | Testable | Covered | Test file | Test | Assertion (verbatim) |
|------|----------------------|----------|---------|-----------|------|----------------------|
| RULE-001 | `<the rule's text, copied exactly from the doc>` | Yes | ✅ | `<path/to/the_test_file>` | `TestTheCoveringTest` | `the exact assertion line from the test, verbatim` |
| RULE-002 | `<a rule whose test asserts something that contradicts it>` | Yes | ⚠️ TENSION | `<path/to/the_test_file>` | `TestTheContradictingTest` | `the contradicting assertion line, verbatim` |
| RULE-003 | `<a testable rule with no test anywhere>` | Yes | ⚠️ MISSING | — | — | — |
| RULE-004 | `<a rule describing display or structural behavior>` | N/A | N/A | — | — | — |
```

The **Rule text** and **Assertion** cells are verbatim copies: copy the exact text from
the rule's doc item and the exact assertion line from the test source, in backticks,
nothing more and nothing less. A cell is one line, so a multi-line rule quotes a single-line
fragment of its text.

Use ✅ for covered, ⚠️ TENSION for a marked test whose assertions contradict its rule's
text (the Assertion cell carries the contradicting line verbatim), ⚠️ MISSING for gap,
N/A for rules skipped in step 2. MISSING and N/A rows carry `—` in the Test file, Test,
and Assertion columns. When a test covers a rule, the `// cinch:rule <ID>` marker goes
above that test function.

### 5. Suggest tests for gaps

For each ⚠️ MISSING rule, suggest:

- **Test name** — follow the pattern of existing tests in the same file (e.g. `Test<Action>_<Condition>`)
- **What to assert** — the specific response or state the test should verify
- **File** — which test file it belongs in

Example:

```
## Suggested tests

**[SIG-0NN] <rule summary>**
- Test: `TestFoo_Bar` in the test file covering the rule's area
- Assert: <the response or state the rule requires>
- Marker: `// cinch:rule SIG-0NN` above the test function
```

## Summary Checklist

Before finishing, confirm:

- [ ] The rule closure came from the harness checkers — no hand re-enumeration of `{{paths.docs}}/`
- [ ] Every rule was classified by its ID (API / Model / N/A)
- [ ] N/A rules are declared `<!-- cinch:ignore: <reason> -->` in their doc — not just skipped in the report
- [ ] The test→rule mapping was derived from test assertions **before** markers were consulted
- [ ] Every marked test was restated against its rule's text, and tensions flagged before deciding
- [ ] Every row quotes the rule text — and, for ✅/⚠️ TENSION rows, the assertion line —
      verbatim from its source
- [ ] Test files were read, not just searched by name — semantic matching used
- [ ] TENSION rows restate the contradiction; unresolved-marker rows judge the test, not the typo
- [ ] Covered rules carry a `// cinch:rule <ID>` marker above the enforcing test
- [ ] MISSING entries have concrete test suggestions with names, assertions, and marker
