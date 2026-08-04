# Rule Coverage Audit

Audit test coverage against documented business rules in `{{paths.business}}/`.

## Usage

Identify which business rules lack test coverage, classified by testability.

## Instructions

### 1. Enumerate all rules

Read every file in `{{paths.business}}/` except `overview.md`. Extract each numbered rule with its
source file. Build a flat list:

```
[<domain>.md #1] <rule summary>
[<domain>.md #2] <rule summary>
...
[<domain>.md #N] <rule summary>
```

### 2. Classify each rule by testability

Three categories:

**API-enforced** — the rule is enforced at the handler level and causes a specific HTTP response
when violated (e.g. a mutation rejected with a specific status when the caller lacks the
permission the rule requires). Look in `{{paths.tests.integration}}/handlers/`.

**Model-enforced** — the rule is enforced by the DB schema or model layer (UNIQUE constraint,
CASCADE, model-level guard). Look in `{{paths.tests.integration}}/models/`. Examples: a
uniqueness constraint, a cascade delete, an immutable-history rule.

**Structural/UI** — the rule describes display or structural behavior that cannot be tested at the
handler or model level (e.g. "display names are derived from another field").
Mark as `N/A` and skip — not a test coverage gap.

### 3. Find test coverage

Test file locations live under `manifest.paths.tests.integration` (`{{paths.tests.integration}}`):

- Handler tests: `{{paths.tests.integration}}/handlers/`
- Model tests: `{{paths.tests.integration}}/models/`

For each API-enforced or model-enforced rule, search the appropriate test directory **semantically**
— match by what the test exercises, not just by name. A test called `TestUpdate_<Subject>Pending`
covers the rule "editing is always free while a request is pending" (asserts 200 OK and content
updated while the request is pending) even though the names don't literally match.

Read representative test files if you're unsure whether a test covers a rule.

### 4. Report

Produce a table per business file:

```
## <domain>.md

| Rule | Testable | Covered | Test |
|------|----------|---------|------|
| #1 <rule summary> | API | ✅ | Test<Action>_<Condition> |
| #2 <rule summary> | Model | ⚠️ MISSING | — |
| #3 <rule summary> | N/A | — | — |
```

Use ✅ for covered, ⚠️ MISSING for gap, N/A for structural/UI rules.

### 5. Suggest tests for gaps

For each ⚠️ MISSING rule, suggest:

- **Test name** — follow the pattern of existing tests in the same file (e.g. `Test<Action>_<Condition>`)
- **What to assert** — the specific HTTP status code or model state the test should verify
- **File** — which test file it belongs in

Example:

```
## Suggested tests

**[<domain>.md #N] <rule summary>**
- Test: `TestFoo_Bar` in `{{paths.tests.integration}}/handlers/<domain>_test.go`
- Assert: <HTTP status or model state the rule requires>
```

## Summary Checklist

Before finishing, confirm:

- [ ] Every business file (except overview.md) was read
- [ ] Every numbered rule was classified (API / Model / N/A)
- [ ] Test files were read, not just searched by name — semantic matching used
- [ ] MISSING entries have concrete test suggestions with names and assertions
