---
requires: paths.domain
---

# Rule Coverage Audit

Audit test coverage against documented domain rules in `{{paths.domain}}/`.

## Usage

Identify which domain rules lack test coverage, classified by testability.

## Instructions

### 1. Enumerate all rules

Read every file in `{{paths.domain}}/` except `overview.md`. Extract each numbered rule with its
source file. Build a flat list:

```
[<domain>.md #1] <rule summary>
[<domain>.md #2] <rule summary>
...
[<domain>.md #N] <rule summary>
```

### 2. Classify each rule by testability

Categories:
<!-- compose: check-rules-classification -->

**N/A** — the rule describes display or structural behavior that cannot be tested automatically
(e.g. "display names are derived from another field"). Mark as `N/A` and skip — not a test
coverage gap.

### 3. Find test coverage

For each testable rule from step 2, search the test locations named in the classification — or
`manifest.paths.tests` if no category named a directory — **semantically**: match by what the
test exercises, not just by name. A test called `TestUpdate_<Subject>Pending` covers the rule
"editing is always free while a request is pending" (asserts 200 OK and content updated while the
request is pending) even though the names don't literally match.

Read representative test files if you're unsure whether a test covers a rule.

### 4. Report

Produce a table per domain file:

```
## <domain>.md

| Rule | Testable | Covered | Test |
|------|----------|---------|------|
| #1 <rule summary> | Yes | ✅ | Test<Action>_<Condition> |
| #2 <rule summary> | Yes | ⚠️ MISSING | — |
| #3 <rule summary> | N/A | — | — |
```

Use ✅ for covered, ⚠️ MISSING for gap, N/A for rules skipped in step 2.

### 5. Suggest tests for gaps

For each ⚠️ MISSING rule, suggest:

- **Test name** — follow the pattern of existing tests in the same file (e.g. `Test<Action>_<Condition>`)
- **What to assert** — the specific response or state the test should verify
- **File** — which test file it belongs in

Example:

```
## Suggested tests

**[<domain>.md #N] <rule summary>**
- Test: `TestFoo_Bar` in the test file covering the rule's area
- Assert: <the response or state the rule requires>
```

## Summary Checklist

Before finishing, confirm:

- [ ] Every domain file (except overview.md) was read
- [ ] Every numbered rule was classified (API / Model / N/A)
- [ ] Test files were read, not just searched by name — semantic matching used
- [ ] MISSING entries have concrete test suggestions with names and assertions
