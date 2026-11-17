# C3 calibration notes

Fixture: pin `bench-pin` (a46c926), clones under the session scratchpad (`C3/work` for authoring, `C3/cal` for
calibration). Codegen with `make gen` (the first run needs `mkdir build` once, otherwise `flutter gen-l10n` fails
on `build/l10n_untranslated.txt`; unrelated to the task). Keys checked with `keycheck.py key.json pin.json final.json`,
`pin.json` from `cinch rules --json` on the clean pin.

## Interface: deviations from the first-draft spec

- The spec said `EntryService` "create, edit and delete". The real methods are `createEntry`, `update(int id, EntryUpdate)`
  and `deleteEntry(Entry)`; `prompt.md` names them. Confirm is `SmsImportService.confirm(pendingId, EntryUpdate, {ruleKeyword})`,
  confirm-all is `SmsImportService.confirmAll({currencyCode})`. The prompt describes both in product terms and does not
  need to name them (they are existing APIs the hidden tests call).
- "Confirming a pending SMS entry whose category is sealed" is ambiguous at the pin, because the user may confirm with a
  different category than the one the rules gave. `prompt.md` says "confirming ... with a sealed category" and the hidden
  tests only cover the category passed to `confirm` being sealed (both when it equals the pending entry's category and
  when the user picked it). Pending entry in a sealed category confirmed with a plain category is untested.
- Not covered by the task or the tests: data import (replaces the whole store), deleting a sealed category, and the UI
  (the entry form already maps any exception to its generic save-failed state).
- Reference rule: `ENTRY-008` in `.docs/domain/entries.md` (new), plus one sentence added to `SMS-004`
  ("A sealed category refuses the entry (ENTRY-008)."). ENTRY-005 untouched. The reference also makes the pending list's
  `confirmableCount` ignore sealed entries (it counts what confirm-all would create).
- Key: `newRule.match` is `["seal|#"]` (DESIGN has `seal`; looser, the control's favour). ENTRY-005 must contain `atomic`;
  SMS-004 must contain `confirm.all` and `transaction` (both in the current text; `confirm.all` also matches
  "Confirm all"/"confirm-all").

## Calibration

a. Pin + hidden tests: FAIL, compile failure. `Error reading 'lib/logic/category/sealed_category.dart': No such file`
   and `'SealedCategory' isn't a type` in both hidden files (`flutter test test/_bench/C3`, 0 tests run).

b. Pin + `reference.patch` (applies cleanly with `git apply`) + hidden tests: PASS (26 of 26).
   `make check`: PASS (format, l10n, analyze, strings; `cinch check`: rules ok, 40 rules, 8 rule docs, 0 ignores).
   `make test`: PASS, 909 tests (874 at the pin + 9 visible + 26 hidden, `test/_bench/C3` was in the tree).

c. Reference with `.docs/` reverted:
   - marker kept: hidden tests PASS (26); key FAIL (`newRule: no new marked rule matching ['seal|#']`); `cinch check`
     fails with 5 stray-marker errors ("ENTRY-008 does not resolve to any rule ID").
   - marker removed (all `cinch:rule ENTRY-008` comments deleted): hidden tests PASS (26); key FAIL (same reason);
     `cinch check` ok.

d. Reference: key PASS (changed rule texts: SMS-004 only; ENTRY-008 has markers in 4 test files).
   Reference with the docs kept but the ENTRY-008 markers removed: key FAIL (`newRule: no new marked rule`), and
   `cinch check` reports "ENTRY-008: no marker".

Extra, behaviour discrimination (mutants of the reference, hidden tests only):
- guard only on `createEntry` (no update or delete guard, no confirm-all skip): 8 of 26 fail;
- no check of the entry's current category on `update`: fails "moving an entry out of a sealed category throws";
- no confirm-all skip (confirm-all then throws on the first sealed entry): fails "with only sealed entries ... does not
  throw" and the mixed-case test;
- no guard on `deleteEntry`: fails the delete tests and "renaming ... seals".

## Hidden tests

`hidden/sealed_entry_writes_test.dart` (17 tests: create, edit, delete, name-derived sealing, ENTRY-005 rollback on a plain
category) and `hidden/sealed_sms_confirm_test.dart` (9 tests: confirm, confirm-all, rename round trip, plain confirm and
type-mismatch behaviour kept). They use only existing public API: `TestServices`, `snapshotAll`, `makeCategory`,
`makeEntry`, `CategoryService.updateCategory` (rename), `SmsRepository.putPending` (seeds pending entries directly, no SMS
parsing), plus `SealedCategory` from the stated file. "Writes nothing" is asserted with `snapshotAll(store)` equality, and
create also checks the in-memory category usages and entry id. Tests re-read entries from the repository where a rename is
involved, so a solution may read the category name from the entry or from the repository.

## Review change (main session, 2026-10-28)

The `SMS-004` key dropped `transaction`. A correct rewording of SMS-004 that leaves that word out would otherwise fail the key, which goes against the control. The key now needs only `confirm.all`, which matches "Confirm all" and "Confirm-all".
