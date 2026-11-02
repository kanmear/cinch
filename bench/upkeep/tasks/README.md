# Upkeep tasks

One directory per task (`S1`–`S3`, `C1`–`C3`). [DESIGN.md](../DESIGN.md) says what each task
asks and why. Every file here is part of the pre-registration and doesn't change after the
`prereg/upkeep` commit.

| File | What it is |
|---|---|
| `prompt.md` | The exact text the agent receives. It's a plain feature request with a fixed interface, and never mentions docs, rules or cinch. It ends with the fixed closing sentence (below). |
| `hidden/*.dart` | Hidden tests, copied to `test/_bench/<task>/` after the run and run alone. They import only `package:budgetron/...`, `package:flutter_test`/`package:test`, and helpers that exist under `test/support/` at the pin (imported as `../../support/...`). |
| `reference.patch` | A reference solution: code, visible tests and corpus edits, as a `git diff` against the pin, without `test/_bench/`. `lib/objectbox-model.json` changes are included; gitignored generated files aren't. |
| `key.json` | The corpus key (format below). |
| `NOTES.md` | Calibration evidence: the commands run and their results, plus any interface change from the first draft and why. |

The closing sentence of every `prompt.md`:

> Follow the repository's conventions. Add or update tests. Commit your work on the current branch when done.

## Key format

```json
{
  "task": "S1",
  "kind": "change | add | leave",
  "rules": [
    {"id": "SMS-006", "match": ["13"], "forbid": ["once a minute"]}
  ],
  "retire": [
    {"id": "CAT-002", "ifPresent": {"match": ["name"], "forbid": ["round-trip"]}}
  ],
  "newRule": {"match": ["seal"]}
}
```

The key is evaluated against `cinch rules --json` on the final tree, using the grader's own
`cinch`. Patterns are Go/Python-compatible regexes matched case-insensitively against the
rule's `text`. The corpus passes only if all of these hold:

1. Every rule in `rules` exists, has at least one marker, matches every `match` pattern, and
   matches no `forbid` pattern. In a `leave` task, these are the rules that must keep their
   meaning.
2. Every rule in `retire` is either absent, or present and satisfying its `ifPresent` (an
   absent `ifPresent` means it must be absent).
3. If `newRule` is set, at least one rule ID absent at the pin exists, has at least one
   marker in a file under `test/`, and matches every `newRule.match` pattern.
4. Every rule present at the pin and not listed in `retire` still exists with at least one
   marker.

Omitted fields are empty. The grader also records, as a secondary measure, every rule whose
`text` changed from the pin.
