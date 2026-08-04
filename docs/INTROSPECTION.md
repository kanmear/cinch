# Introspection contract — C6 / C7

The direction rule (D009) lets two checkers validate docs against code: C6
`models/*.md` ↔ real types, C7 `api/*.md` ↔ registered routes. Core never parses
project source (standing rule 5, D003): type and route introspection are
manifest-declared commands, and this document is the JSON contract both must
emit. The consumer implements the producers (D028); core checks the docs
against their output.

## Producers

Two keys under `development.commands` (commented in `scaffold/manifest.example.yml`):

- `introspect-types` — static parse (go/ast walk, TS declaration emit). Emits
  every type the docs are allowed to reference.
- `introspect-routes` — builds the router and prints its table (runtime; most
  routers enumerate — chi Walk, gin Routes, echo Routes).

A producer exits 0 and writes its JSON to stdout. A non-zero exit or
unparseable output is a C6/C7 error carrying the producer's stderr tail.

A repo that does not declare a producer key simply skips the corresponding
check — same absence-is-a-declaration semantics as C5's missing business layer
and C12's missing seams. A repo that declares it must have it green: the
declared command is the repo's binding to this contract.

## Output shapes

### introspect-types

```json
{"types": [{"name": "User", "fields": ["id", "name", "email", "PasswordHash"]}]}
```

- `name` — the type's declared name.
- `fields` — the type's canonical serialized names (json tag values for Go,
  property names for TypeScript), **including** fields excluded from
  serialization (`json:"-"`), which appear under their declared Go name — a
  doc that documents `PasswordHash` can prove it exists.

### introspect-routes

```json
{"routes": [{"method": "POST", "path": "/api/auth/signup"}]}
```

- `method` — uppercase.
- `path` — exact registered path.

## What the checkers match against

### C6 — `models/*.md`

A type is documented by a ```go fenced block containing a
`type <Name> struct` definition; the field names listed inside that block are
the doc's claim. Every documented type must exist in the introspection output
(error), and every documented field must be in that type's field set (error) —
the doc is the spec and must be true of the code. A real type with no
`models/*.md` documenting it warns: some types are legitimately internal, and
the warning is the question, not the verdict.

### C7 — `api/*.md`

A route is documented by an `## METHOD /path` heading. Every documented route
must be registered (error). A registered route with no `api/*.md` entry warns.
Headings inside fenced code blocks are illustrative examples, not
documentation — they are skipped (D046).
