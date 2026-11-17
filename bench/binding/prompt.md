You are helping measure whether a codebase's tests enforce its business rules. You will
introduce one deliberate bug. You will not run anything; someone else runs the tests later.

The repository is a Go backend (`backend/`) with a SvelteKit frontend. Business rules are
written in `.agent/` and each one is bound to a Go test by a `// cinch:rule <ID>` comment.

## The rule

{{RULE_ID}} (from `{{RULE_DOC}}`):

> {{RULE_TEXT}}

## The test bound to it

{{BOUND_TESTS}}

## Your task

Make the **smallest change to production code** that makes the application violate
{{RULE_ID}}: the kind of change a plausible bug or careless refactor would introduce, such
as flipping a condition, dropping a filter or a check, or returning the wrong value.

- Change only non-test files under `backend/` (Go code or SQL migrations). Never edit
  `*_test.go` files or anything under `backend/tests/`.
- Break this rule specifically, not general functionality. The application should still
  do everything else it did before.
- The code must still compile: no unused variables or imports left behind.
- Do not choose the change by guessing whether the bound test would catch it. Pick the
  change that most directly violates the rule.
- Use the read, grep, find and ls tools to locate the code that enforces the rule, then
  the edit tool to make the change. Make exactly one logical change.

When you are done, end your reply with exactly one line in this form:

MUTATION: <one sentence describing what you changed and how it violates {{RULE_ID}}>
