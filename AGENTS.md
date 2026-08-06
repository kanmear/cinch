# cinch

A referential integrity checker for agent documentation. **Status: shell —
the previous implementation was purged (D085); the rebuild has not landed.**

## What this repo is

- `docs/PRINCIPLES.md` — the spec the rebuild must satisfy. The six principles,
  ranked by evidence, each with its rebuild constraint. Read this before any
  rebuild work; it is the only memory of the old design that is allowed in the
  room — the old design itself is git history.
- `decisions.jsonl` — append-only decision log. Read it before proposing a
  change to a settled question; append a superseding entry rather than
  diverging silently.
- `README.md` — the orientation artifact: status, layout map, pointers.
- `main.go` — the shell stub (usage only), `tests/` — CLI tests against the
  built binary, `Makefile` — `build` / `test`.

## Rules

**The six principles govern everything that lands here** (docs/PRINCIPLES.md is
canonical): deterministic checkers with mutation fixtures over model audits;
the direction rule (doc-upstream, lateral, structural — never doc-downstream);
the derivability gate on docs and the holdability gate on the system; rule→test
markers in the test file; bind values in one place, shape by absence; no proxy
metrics — every check has a demonstrable failing state, every warn a reachable
green state, no accepted baselines.

**Handoffs go to plan files or decision entries, never `docs/`.** docs/ holds
spec only; session state is point-in-time and belongs in a plan file or a
decision/event entry — git is the archive.

**Historical artifacts are deleted, not kept alive.** If an old artifact
explains the new one better than the new one does, the new one is incomplete.
Recovering old material from git is normal; committing it back is not.

## Session start

Read `docs/PRINCIPLES.md`. Not the roadmap — there is no roadmap; the purge
deleted it and the rebuild is planned against the principles.
