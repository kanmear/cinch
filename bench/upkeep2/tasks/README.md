# Upkeep v2 tasks

There is one directory per candidate task (`U1`…`U8`). [DESIGN.md](../DESIGN.md) § Task selection
says how candidates are chosen and § Grading how they are graded. Everything here is part of the
pre-registration:

- It is committed at `prereg/upkeep2-pilot`.
- It doesn't change after that, except through a tagged amendment.
- A change to a prompt, hidden test or key after a pilot voids that task's pilots.

Two files sit at this level: `SELECTION.md` and `sequence.json`. No task exists yet.

## Files per task

| File | What it is |
|---|---|
| `prompt.md` | The exact text the agent receives. It is a plain feature request with a fixed public interface, and never mentions docs, rules, markers or cinch. It ends with the closing sentence (below). |
| `hidden/*.dart` | The hidden tests, copied to `test/_bench/<task>/` after the run and run alone. See § Hidden tests. |
| `reference.patch` | A correct solution: code, visible tests and corpus edits, as a `git diff` against the pin, without `test/_bench/`. It includes `lib/objectbox-model.json` changes, but not gitignored generated files. |
| `reference-naive.patch` | A competent code-only solution: the code an agent writes if it never meets the key rule. It has no `.docs/` hunks. It must pass F and fail K, or (change kind) leave the rule's text false. This is the breakability criterion, and `impact` is run on its file list to set the reach class. |
| `key.json` | The rules at stake, with the judge's truth statements and the far and reach metadata. Format below. |
| `judge-probes.json` | Calibration probes for the judge: rule texts with their expected verdicts (format below). Every probe must pass, for each judge, before the tag. |
| `NOTES.md` | Calibration evidence: the commands run and their results, the `cinch impact` output on the naive patch, the far-type check, the size of the code diff, and any interface change from the sketch and why. Pilot results and the cap are appended at `prereg/upkeep2`. |

The closing sentence of every `prompt.md` (v1's, unchanged):

> Follow the repository's conventions. Add or update tests. Commit your work on the current branch when done.

## Hidden tests

- **Imports.** Only `package:budgetron/...`, `package:flutter_test`/`package:test`, and helpers
  that exist under `test/support/` at the pin, imported as `../../support/...`.
- **Groups.** Every test name starts with `F:` or `K:`.
  - **F** (feature): the requested behavior works.
  - **K** (keep): the key rule's clause holds on the new path, and on any old path the change can
    reach.
  - A behavior pass is all F and all K passing. A build failure of the hidden package fails both.
- **Robustness.** They must compile on any tree where the task's interface exists. They must not
  depend on an internal another task might change, since part E applies tasks in sequence. Where
  two tasks touch the same type, the hidden tests use only what both prompts fix.
- **Not flaky.** Green 10 times in a row on `reference.patch` (`NOTES.md` records it).

## Key format

```json
{
  "task": "U1",
  "kind": "change | leave | add",
  "tier": 1,
  "far": {
    "types": ["ownership", "side-effect"],
    "ruleDoc": ".docs/domain/entries.md",
    "naiveDocs": [".docs/domain/settings.md"],
    "focusWords": ["default"]
  },
  "reach": {"impact": "R0 | R1 | R2", "orphan": "O0 | O1"},
  "rules": [
    {
      "id": "ENTRY-006",
      "kind": "change",
      "truth": "For a new install the entries sort is \"Recent\"; a stored choice is kept. Amount sorting (income first, then by absolute category total) is unchanged.",
      "required": ["the default for new installs is Recent", "an existing stored sort is kept"],
      "notRequired": ["how an unknown stored name falls back"],
      "retirable": false
    }
  ],
  "newRule": null,
  "collateralAllowed": []
}
```

The `ENTRY-006` text above shows the shape only. It is not a task.

### Fields

- **`kind`** (task level): `change` if any key rule must change, otherwise `leave` or `add`. Per
  rule, `kind` is `change` (the text must change, G2) or `leave` (it must keep its meaning).
- **`tier`**: 1 (in B's normative list and in census A's under-enforced or misplaced rows), 2 (B
  only) or 3 (A only).
- **`far.types`**: the types this task satisfies, each verified at calibration.
  - **ownership**: no `owns:` path of `ruleDoc` matches a non-test file of
    `reference-naive.patch`. `naiveDocs` lists the docs that do own those files.
  - **side-effect**: no word of `focusWords` occurs in `prompt.md` (case-insensitive, whole
    word), and the naive patch breaks the clause.
- **`reach`**: written by the calibration step from `cinch impact` on the naive patch's files.
  - R0: the advisory names neither the rule nor its doc.
  - R1: a marker of the rule is in a changed file.
  - R2: the rule's doc owns a changed file.
  - O1: the naive patch removes every test carrying the rule's markers.
- **`rules[].truth`**: what the code does after the change, on the rule's clause, in one or two
  sentences. It is written before any run, and the judge grades the final rule text against it.
- **`required`** and **`notRequired`**: the clauses the final text must state, and those that
  never count against it. They play the same part as the "Focus" notes in bench/recover.
- **`retirable`**: the rule may be absent, or present and marked retired (`(?i)\bretired\b`, or
  cinch's retire marker if the pinned version parses one). If so, it counts as CORRECT and is
  exempt from G4.
- **`newRule`**: for add kind, `{"truth": "...", "required": [...]}`. Some rule ID absent at the
  pin must exist with a marker in a file under `test/` (G4), and the judge must grade it CORRECT
  against this truth.
- **`collateralAllowed`**: rules outside the key whose text the task may legitimately change.
  Edits to any other rule are counted as collateral (secondary).

### What the grader evaluates

The grader runs on `cinch rules --json` of the final tree with its own cinch. The corpus passes
only if all of these hold:

1. **G1** every key rule exists, or is retired as allowed.
2. **G2** every `change` rule's text differs from the pin after whitespace normalization.
3. **G4** every key rule (and the new rule, for add kind) has at least one marker in a file
   under `test/`.
4. **G5** every rule present at the pin, other than a retirable one, still exists with at least
   one marker.
5. **The judge** gives CORRECT for every key rule, and for the new rule.

The judge isn't called in two cases, which are graded mechanically:

- an unchanged `change` rule is STALE;
- an unchanged `leave` rule is CORRECT.

G3 (a file with the rule's marker changed, or a new marker added) and collateral edits are
recorded but are not part of the pass.

## Judge probes

```json
[
  {"rule": "ENTRY-006", "text": "<rule text without number or bold ID>", "expected": "CORRECT", "why": "reference"},
  {"rule": "ENTRY-006", "text": "...", "expected": "STALE", "why": "pin text"}
]
```

Each task needs, per key rule, at least the following probes:

| Probe | Expected verdict |
|---|---|
| the reference's text | CORRECT |
| a paraphrase of it | CORRECT |
| the pin's text | STALE |
| a text that adds the new behavior but keeps the contradicting old clause | WRONG |
| a text missing one `required` clause | PARTIAL |
| a retired text (retirable rules only) | CORRECT |

## SELECTION.md (at this level)

`SELECTION.md` is the written outcome of DESIGN § Task selection:

- The eligible set: census B's normative rules (pasted from `normative-budgetron.json`, with its
  commit), census A's rows, and the exclusions.
- The tier of each eligible rule.
- Every sketch considered, and the criterion each rejected sketch failed.
- The ranking, and the eight drafted candidates.
- The owner's veto decisions.
- At `prereg/upkeep2`, the pilot table per engine (passes, commits, slowest commit, cap,
  kept/dropped and why) and the kept set per engine.

## sequence.json (at this level, part E)

```json
{
  "tasks": ["U1", "U2", "U3", "U4", "U5", "U7"],
  "overlapPairs": [["U2", "U3"], ["U3", "U4"]],
  "precedence": [],
  "orderSeed": 20261104,
  "orders": {
    "pi": [["U3", "U1", "U5", "U2", "U7", "U4"]],
    "claude": []
  }
}
```

The values above are placeholders, not a sequence.

- `orders` holds the R rows of the Williams design, drawn with `orderSeed` under the
  `precedence` constraints. Each row is shared by all four arms of that replicate.
- Reference-chain calibration (DESIGN § Calibration of E) is recorded per order in `NOTES-E.md`
  beside this file.
