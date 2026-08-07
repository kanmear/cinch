# Principles — what the rebuild must be

Status: **spec.** This file is the memory of what survived contact with
evidence. Everything else the previous cinch built or believed was deleted on
2026-08-07; git history is the archive. A rebuild session reads this
file and nothing else about the old design — the old design is the history.

## The failure mode

A system can pass every check it defines and still be past the point where you
can verify it. The old cinch proved this: its own harness checked itself green
(exit 0, "known warns") while the semantic audit layer was confabulating —
nothing checked the auditor's output until C15 was built for exactly that
purpose, and C15 itself cannot judge a well-quoted wrong link. That is the
derivability gate pointed at the harness instead of the docs: **build only what
you can hold.** Verification ends where judgment begins, and the system must
state that boundary rather than pretending a check closes it.

## Principles, ranked by what survived contact with evidence

### 1. Deterministic checkers over model audits, where the property is decidable

The only thing that was tested and passed — 9/9, no overreach. A script cannot
return a false ✅; a model demonstrably will, fluently, under exactly the
conditions where it matters most: claude confabulated a ✅ 3/3 times pre-
blinding, pi went 1/2 then 0/2 under it, and a C15-style evidence check caught
the 27B's improvised report mechanically.

**Rebuild constraint:** a property is checkable only if it is decidable; the
property's decision procedure is the checker. Semantic residue is not
abolished — it is demoted: judgment stays, but it must sit behind a decidable
tripwire (the diff-coupling pattern: "this text changed and its enforcing file
did not"; the evidence-quote pattern: "this report's quotes appear verbatim in
their sources"). **The first checker ships with a mutation fixture that proves
it catches its class** — a mutated corpus the checker must flag. That is the
only thing that ever told this project the truth rather than reporting a
status; a checker without its mutation proof is not done.

### 2. The direction rule

Check things your docs are the source of; delete things they're a copy of.
Checks are doc-upstream (the doc is the spec, code conforms), lateral (two
authored artifacts agree), or structural (a corpus or contract referent
exists) — never doc-downstream. A check that validates a doc's copy of a
machine-readable fact means the duplication should be deleted instead.

**Rebuild constraint:** this is what stops a checker suite from becoming a
duplication patrol, and it is what kept the old suite few enough to be
trustworthy. Every proposed checker must name its direction before it is
built; a doc-downstream proposal is closed by deletion, not by validation.

### 3. The derivability gate on docs; the holdability gate on the system

Write only what code can't answer. Solves staleness at write time rather than
through maintenance. Corollary: build only what you can hold — the same
principle pointed at the system instead of the docs.

**Rebuild constraint:** the suite crosses the holdability line when a new
checker would require a new checker to verify it. C15 was exactly that moment
— the audit report was the only artifact nothing validated until a checker was
built for it. The suite must remain enumerable and independently verifiable by
one person; growth past that is the failure mode, not a milestone.

### 4. Rule → test markers, in the test file

A `// cinch:rule <ID>` comment above a rule's enforcing test. It works because
deletion is self-correcting: remove the test and the marker goes with it, and
the gap surfaces immediately. Any version where the link lives in a doc keeps
claiming coverage it lost.

**Rebuild constraint:** the link lives where the deletion happens, never in a
registry. It depends on scripted enumeration — the marker closure is computed,
never hand-listed; a script cannot produce a false checkmark.

### 5. Bind project specifics in one place

The manifest idea, minus everything built on top of it. Without it you can't
tell portable from local, and that distinction is what makes anything
reusable.

**Rebuild constraint:** the manifest binds values, not shape — and shape is
where the real variance lives. Shape variance is answered by absence-based
selection (a file-level `requires:` declaration, not conditionals), never by
growing the schema. When a key would serve exactly one template, the thing is
project content, not a manifest key.

### 6. Proxy metrics are worthless

"AGENTS.MD initialized" was the first thing cut, and the failure recurred:
the phantom C13, the accepted C11 baseline, the circular self-harness. All
green lights guaranteed to be green. A check that cannot fail tells you
nothing, and sits next to checks that can, making those less legible.

**Rebuild constraint:** every check must have a demonstrable failing state —
principle 1's mutation fixture is the proof of life, which makes this
enforceable rather than taste. Every warning must have a reachable green
state; a warn with no closing action is a proxy metric wearing a costume.
No accepted baselines: a permanent "known warn" is a failed check, not a
fixture of the exit code.
