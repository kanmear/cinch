# `.agent/events/` — episodic record

Append-only JSONL. One object per line, one line per event. **Nothing here is ever edited or
deleted.** An entry records what was believed and decided at a point in time; it is not a claim
about the present, which is why it cannot go stale and why no checker validates it.

## Schema

| Field | Meaning |
|---|---|
| `id` | `D<n>` decision, `O<n>` open question, `E<n>` session event |
| `ts` | ISO date |
| `kind` | `decision` · `non-goal` · `deferred` · `open` · `event` |
| `title` | one line |
| `decision` | what was chosen |
| `rejected` | what was not chosen (`null` if not applicable) |
| `why` | the reasoning — the field that earns the file |
| `refs` | pointers into plans/docs |

`why` is the point of the log. An entry without it is a rule with no rationale, which the
why-framing north star says a weak model will discard.

## Superseding

Never edit. Append a new entry with `"supersedes": "D0NN"` and a `why` explaining what changed.
Resolving an open question works the same way: `O001` stays as written, and a new `D0NN` carries
`"supersedes": "O001"`.

## What belongs here

Decisions and their alternatives. Dead ends and why they were dead. Risks noticed and accepted.
Surprises.

Not: standing rules (→ `business/`), current state (→ Session Handoff), anything derivable from a
diff (→ git).

## Seeding

`decisions-seed.jsonl` carries the planning-phase decisions retroactively, back-dated to the day
they were made. Concatenate it as the log's first entries.
