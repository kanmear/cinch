---
requires: paths.tests.handlers paths.tests.models
---
**API-enforced** — the rule is enforced at the handler level and causes a specific HTTP response
when violated (e.g. a mutation rejected with a specific status when the caller lacks the
permission the rule requires). Look in `{{paths.tests.handlers}}`.

**Model-enforced** — the rule is enforced by the DB schema or model layer (UNIQUE constraint,
CASCADE, model-level guard). Look in `{{paths.tests.models}}`. Examples: a uniqueness constraint,
a cascade delete, an immutable-history rule.
