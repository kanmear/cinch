# Rule Coverage Audit

Use this periodically, or after a batch of rule changes, to check whether each rule's test really enforces it.

`cinch check` only proves that a marker exists. This audit asks whether the marked test means
anything.

## 1. Inventory

Run `cinch check` and get it green first. It reports markers that name no rule, which the
inventory below can't show. For each one, judge the test above it: a correct test with a mistyped
ID needs its marker fixed, not a new test.

Then run `cinch rules --json`. For every rule it gives the ID, the doc's `file` and `line`, the
markers, and any `cinch:ignore` reason, and for every doc its `owns:` paths. That is the complete
rule set, so don't re-enumerate the docs by hand. Quote a rule's text from its doc at `file:line`:
for the last rule in a list, the JSON `text` can run on into the lines that follow it.

## 2. Testable or N/A

A rule is N/A only if no automated test could observe it (display wording, a constraint on future
decisions). Declare it with `<!-- cinch:ignore: <reason> -->` directly under the rule item, not
just in the report. Hold existing ignore reasons to the same standard: a reason a test could
refute hides a missing test.

## 3. Derive coverage from the tests, then compare

This is a search with nonzero expected yield: find the rules whose tests don't enforce them. Expect
to find at least one, because rules rot as behavior changes. Don't frame the audit as confirming
each marker; that verifies the marker exists, not what the test means.

**First, derive the mapping without looking at markers.** Read the tests for the code each rule
doc `owns:` (where a doc has no `owns:`, the code its rules describe). For each test, write one sentence on what its assertions actually pin (the response
or state, not the test's name) and match that sentence against the rules. Write this test→rule
mapping down before you read any marker.

**Then compare it with the markers** in the inventory, reading every marked test next to its
rule's text:

- The assertions contradict the rule: **TENSION**. Quote the contradicting line.
- The marked test asserts too little to enforce the rule: under-enforced. Report it like TENSION.
- A test you derived as enforcing a rule carries no marker: the marker is missing or misplaced.
  Say where it belongs.
- A testable rule with no test that enforces it: **MISSING**.

## 4. Report

One table per rule doc, with verbatim evidence in every row:

```
## <doc path>

| Rule | Rule text (verbatim) | Testable | Covered | Test file | Test | Assertion (verbatim) |
|------|----------------------|----------|---------|-----------|------|----------------------|
| <ID> | `<exact rule text>` | Yes | ✅ | `<path>` | `<test name>` | `<exact assertion line>` |
| <ID> | `<exact rule text>` | Yes | ⚠️ TENSION | `<path>` | `<test name>` | `<contradicting line>` |
| <ID> | `<exact rule text>` | Yes | ⚠️ MISSING | — | — | — |
| <ID> | `<exact rule text>` | N/A | N/A | — | — | — |
```

Rule text and assertion cells are exact copies in backticks. A cell is one line, so for a
multi-line rule, quote a one-line fragment.

## 5. Suggest tests for gaps

For each MISSING or under-enforced rule, give a test name that follows the pattern of the
neighboring tests, the assertion that would prove the rule, the test file, and the
`cinch:rule <ID>` marker to put above it.

## Before finishing

- [ ] The rule set came from `cinch rules --json`, after a green `cinch check`
- [ ] The test→rule mapping was derived from assertions before any marker was read
- [ ] Every marked test was read against its rule's text
- [ ] Every row quotes its rule text and assertion verbatim
- [ ] Every N/A rule has a `cinch:ignore` reason in its doc
