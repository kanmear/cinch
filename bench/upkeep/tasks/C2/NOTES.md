# C2 calibration notes

Pin: `bench-pin` (a46c926), 39 rules, 874 tests. Clone setup: `git clone`, `git checkout bench-pin`,
copy `.objectbox/lib`, `make gen` (needs `mkdir build` first, or `flutter gen-l10n` aborts on a
missing `build/`). Hidden tests are copied to `test/_bench/C2/`. Commands below were run in a
fresh clone with `reference.patch` applied by `git apply --index`, then `dart run build_runner
build` (the patch includes `lib/objectbox-model.json`; `objectbox.g.dart` is regenerated).

## The reference solution (what it changes)

- `Entry.isHalved` (new ObjectBox column, default false; `lib/objectbox-model.json` gains property 11) and
  `Entry.countedMinor` (`isHalved ? amountMinor ~/ 2 : amountMinor`). No migration: schema changes only add
  columns, and an existing row reads as false.
- Balance: `ObjectBoxAccountRepository.sums` sums the whole entries in the store and adds `countedMinor` of
  each halved one (loaded, since the truncation is per entry). `BalanceCalculator` is unchanged.
- Budget spent: `BudgetService` feeds `Money(entry.countedMinor)` to `BudgetCalculator.results`.
- Transfer: `TransferEntry.isHalved`, `formatVersion` 4 to 5, key optional on read (absent = false), always
  written. Exporter and importer carry it. Visible tests that pinned version 4 or the exact entry keys
  were updated.
- Corpus: ACCT-001 and BUDGET-002 changed; new rule ENTRY-008 (the flag, the `~/ 2` truncation, "nowhere
  else"); XFER-004 and XFER-005 extended (neither is keyed); `architecture.md` updated (format version,
  balance and spent derivation).
- Visible tests: 7 new (ACCT-001, BUDGET-002, ENTRY-008 x2, XFER-001, XFER-004, XFER-005). `make test`:
  881 tests (874 + 7).

## Calibration

a. Pin + hidden tests: FAIL. All three files fail to compile (`The setter 'isHalved' isn't defined for the
   type 'Entry'`). Command: `LD_LIBRARY_PATH=$PWD/.objectbox/lib flutter test test/_bench/C2`.

b. Pin + `reference.patch` + hidden tests: PASS (`+30: All tests passed!`). `make check` PASS (format,
   gen-l10n, analyze, strings, `cinch check`: 40 rules, 0 ignores). `make test` PASS with the hidden tests
   in place (`+911`, i.e. 881 + 30 hidden).

c. Pin + reference with `.docs/` reverted to the pin: hidden tests PASS (`+30`), key FAIL
   (`ACCT-001: text fails match/forbid`, `BUDGET-002: text fails match/forbid`). `cinch check` also fails
   there (the ENTRY-008 markers resolve to nothing), which is the arm-3 stray-marker block.

d. Key (`keycheck.py key.json pin.json final.json`):
   - full reference: PASS (changed texts: ACCT-001, BUDGET-002, XFER-004, XFER-005);
   - untouched pin as final: FAIL (ACCT-001, BUDGET-002);
   - only ACCT-001 updated: FAIL (`BUDGET-002: text fails match/forbid`);
   - only BUDGET-002 updated: FAIL (`ACCT-001: text fails match/forbid`);
   - both formulas updated and nothing else (no ENTRY-008, no XFER edits): PASS.

e. Mutation checks (the hidden tests catch each wrong implementation, run on the reference with one change):
   - balance ignores the flag: 10 balance tests fail;
   - budget ignores the flag: 6 budget tests fail;
   - importer drops the flag: the round-trip test fails;
   - exporter drops the flag: the round-trip and "carries the flag" tests fail;
   - reader requires the flag (breaks old files): the two older-file tests fail.

## Interface decisions and deviations from the spec

- The spec's interface is kept as is. The prompt fixes: `Entry.isHalved` (a mutable bool, default false),
  that it is set before `EntryService.createEntry`, the `~/ 2` rule per entry, the export key name
  `isHalved` on each entry object, and that an older file without it reads as not halved. The export key
  name has to be stated because the older-file test removes that key from a real export.
- The hidden tests simulate "an older file" by exporting, setting `formatVersion` to 4 and deleting the
  `isHalved` keys. That works whether or not a solution bumps the format version (4 is never newer than the
  app), so the version bump is left to the solution (the reference bumps to 5, as the repo did for each
  earlier added field). A solution that makes the key required in version 5 and optional below still passes.
- The hidden tests do not assert the format version number, the key set of any object other than "no key
  appears that an unhalved export lacks, except `isHalved`", or `entryCount` of a period (a halved entry
  that counts 0 is skipped by the calculator's `isNegative` filter, which would make `entryCount` ambiguous).
- No UI and no `EntryService.update` parameter for the flag; the spec asks for a stored field only, and the
  prompt says no screen needs to change. The reference keeps `update` from touching the flag, and a hidden
  test checks only that editing the amount keeps the stored amount whole.

## Key notes (key.json)

- ACCT-001 and BUDGET-002: `half|halv` as specified. The reference writes "a halved entry counting at half
  its amount" in both.
- XFER-001: `lossless|come back equal`; XFER-005: `derived`. Both appear in the current text and a correct
  solution has no reason to remove them. These two are a guard against an agent rewriting the rule away; they
  only fail when the rule loses its core wording.
- ENTRY-005 ("derived on read") also has to stay true per the spec but is not keyed (the task brief names four
  rules). Its marker is covered by the generic "no pin rule loses all markers" clause.

## Risks for the design

- The risk sits in the task, not the key: the halving has one obvious place per formula (the
  repository sum for balances, the `BudgetService` mapping for spent). An agent that finds both code paths
  will usually find both rules; the miss the design expects (one formula forgotten) mostly happens when the
  agent never reads `accounts.md`. Behavior tests catch a forgotten code path, so a forgotten doc only
  shows up if the code is right.
- Agents may write the new rule ENTRY-008-style text in a new file or in `entries.md` only and leave ACCT-001
  and BUDGET-002 untouched. That fails the key, which is intended (the formulas become false).
- `make gen` on a fresh clone fails on a missing `build/` directory (`l10n_untranslated.txt`); the harness
  setup should create it first.
