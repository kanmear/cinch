# cinch

A referential integrity checker for agent documentation.

## Status: shell, rebuild pending

The previous implementation was purged on 2026-08-07 (D085): every checker,
template, self-harness, and operational artifact was deleted. Git history is
the archive of everything that was tried; `docs/PRINCIPLES.md` is the memory
of what survived contact with evidence — six principles ranked by what was
tested and passed, not by what seemed clever.

The shell compiles and tests green so the rebuild starts from a working build.
`make test` exercises the binary end-to-end.

## Layout

- `main.go` — the shell stub: usage only, no commands yet.
- `docs/PRINCIPLES.md` — the spec the rebuild must satisfy: the six principles
  (deterministic over semantic, the direction rule, the derivability and
  holdability gates, rule→test markers, bind values not shape, no proxy
  metrics), each with the evidence that earned it and its rebuild constraint.
- `decisions.jsonl` — append-only decision log; D085 records the purge.
- `tests/` — CLI-level tests exercising the built binary.
- `Makefile` — `build`, `test`.

Everything else was deleted and is recoverable from git.

## The rebuild

Not planned yet. When it is, it is planned against `docs/PRINCIPLES.md` and
nothing else about the old design — the first checker ships with a mutation
fixture that proves it catches its class.
