# C1 calibration notes

Pin: `bench-pin` (a46c926) of `pin.git`. Clones under `scratchpad/C1/{work,pin,ref,refnodocs}` (setup: clone, `git checkout bench-pin`, `mkdir -p .objectbox build`, copy `.objectbox/lib`, `make gen`; note `make gen` fails on a fresh clone unless `build/` exists, because `flutter gen-l10n` writes `build/l10n_untranslated.txt` - the harness should `mkdir -p build` first).

## Reference solution (summary)

- New `lib/logic/category/category_color.dart`: `categoryPalette` (18 colors, the first-column colors of the existing picker), `categoryColorFor(name)` = djb2 (`h*33+byte`, masked to 32 bits) over the UTF-8 bytes, `% palette.length`.
- `EntryCategory.color` kept as a retired `@Deprecated` column with default `''` (repo convention: "schema changes only add columns", so `objectbox-model.json` is untouched, no migration). `Selectable` now exposes `Color resolveColor()` instead of a `color` field; callers use `categoryColorFor(category.name)`. `CategoryTotal.color` removed. Category form cubit/dialog lose the color; `ColorAndNameField.onColorChanged` is optional (account form still uses the picker). l10n key `categoryFormColorAndName` renamed `categoryFormName`.
- Transfer: `TransferCategory` has no `color`; `formatVersion` 4 -> 5; the reader does not read `color`, so older files import and the color is dropped.
- Docs: CAT-002 rewritten (derived from name, never stored; no "round-trip"); XFER-004 mentions the ignored `color`; XFER-005 names the derived category color; architecture.md updated. New tests `test/logic/category/category_color_test.dart` carry `cinch:rule CAT-002`; XFER-004 test added for an old file with colors. The old Color<->String tests in `category_service_test.dart` stay (accounts still store a color string) but lose the CAT-002 marker.
- Visible tests: 874 -> 878.

## Calibration

a. Pin + hidden tests: FAIL (compile failure: `category_color.dart` missing, `EntryCategory` requires `color`, `categoryPalette`/`categoryColorFor` undefined).
   `cd pin && cp hidden/*.dart test/_bench/C1/ && LD_LIBRARY_PATH=$PWD/.objectbox/lib flutter test test/_bench/C1`
b. Pin + reference.patch (`git apply`): hidden tests 10/10 PASS; `make check` exit 0 (format, l10n, analyze, strings, `cinch check`: 39 rules, 0 ignores); `make test` 878/878 PASS (~22 s while other builds ran).
c. Pin + reference.patch minus `.docs/` (`git apply --exclude='.docs/*'`): hidden tests 10/10 PASS; keycheck FAIL (`CAT-002: present and not satisfying ifPresent`, changed rule texts: none). Note `cinch check` still passes in this state (CAT-002 keeps its markers on the new tests): the false rule survives silently, which is the arm-2 failure mode.
d. Pin + full reference.patch: keycheck PASS (changed rule texts: CAT-002, XFER-004, XFER-005).
   `cinch rules --json > pin.json` (clean pin), `... > final.json` (patched), `python3 keycheck.py key.json pin.json final.json`.

## Key

`XFER-001` must contain `lossless`; `XFER-004` `older`; `XFER-005` `derived`; CAT-002 may be absent, or present with `name` and without `round[- ]?trip` / `lossless`. Tokens are words the current texts contain and a rewording is unlikely to drop. CAT-002 is the only retirable rule (exempt from the marker requirement), as in the README format.

## Interface notes / deviations from the spec

- `categoryPalette` has 18 entries; hidden tests only require it be non-empty and that results are members of it, and that 300 names reach more than one color. No hash value is pinned in the hidden tests (algorithm is free).
- The prompt also fixes `EntryCategory(name:, isExpense:)` (no `color`), since the hidden tests construct categories that way. A solution that keeps an optional `color` parameter also compiles.
- Hidden tests use pin helpers `seedAllKinds`, `observe`, `exporterOf` (test/support/transfer_seed.dart), `TestServices`, `openTestStore`, and the checked-in `test/db/transfer/fixture_v1.json` (a version-1 file with category colors). They build `DataImporter` themselves (the pin's `importerOf` lives in a test file, not support). Risk: an agent that changes those helper signatures breaks compilation; unlikely for this task.
- The "older file" test sets `formatVersion = TransferDocument.formatVersion - 1`, so it works whether or not the solution bumps the format version.

## Risks seen in the design

- A solution that keeps `EntryCategory.color` as a required constructor argument fails to compile the hidden tests (intended; the prompt says no `color` argument).
- The orphan mechanism fires only if the agent deletes CAT-002's tests; here the old CAT-002 tests are Color<->String conversions that remain valid for accounts, so an agent may keep them (still marked) and leave CAT-002 untouched. Then arm 3's `cinch check` stays green and CAT-002 stays false: both arms fail the corpus key, so C1 may separate the arms less than DESIGN expects. The impact advisory is the only cinch pull in that path.
- `make gen` needs `build/` to exist on a fresh clone (see above).

## Amendment 2 (2026-11-01, after all 90 pi runs were graded)

cinch's own rule-maintenance workflow retires a rule by keeping its item and stating that it
is retired, not by deleting it. In pi run `C1-arm3-r3` the agent did that, and the grader
failed the corpus because CAT-002 was present and its text still contained `round-trip`:

> 2. **CAT-002** *(retired)* A category color string and its `Color` round-trip losslessly.
> Categories no longer store a color; it is derived from the name (CAT-003). The string/`Color`
> helpers remain for account colors, which are still stored.

The grader now treats a
rule listed in a key's `retire` whose final text matches `(?i)\bretired\b` as retired,
whether or not `ifPresent` holds. `key.json` is unchanged; the check is in `evalKey`
(`key.go`); `tasks/README.md` § Key format is not edited and describes the rule as it stood at
`prereg/upkeep`.

| CAT-002 text | Amendment-1 grader | Amendment-2 grader |
|---|---|---|
| The pin | fail | fail |
| `C1-arm3-r3`, begins `*(retired)*` | fail | pass |
| The pin text with "unretired helpers" appended (no word match) | fail | fail |
| Rewritten: derived from the name, never stored | pass | pass |

`TestKeyAmendments` pins these cases.
