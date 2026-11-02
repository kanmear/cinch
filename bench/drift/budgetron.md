# Budgetron — Rule Coverage Audit (plan 29, item A: drift census)

Workflow: `cinch workflow docs-audit-coverage`, followed step by step. Repo audited read-only:
`/home/romius/code/budgetron`, branch `dev`, commit `0fe3070f39721c57e91e99c250d51fd4ed75e4af`.
The pre-marker mapping is in [`budgetron-mapping.md`](budgetron-mapping.md); the per-rule verdicts are in
[`budgetron.csv`](budgetron.csv).

## 1. Inventory

`cinch check` (exit 0):

```
links: ok (35 docs, 150 links checked)
rules: ok (39 rules, 8 rule docs, 0 ignores)
index: ok (20 docs checked)
generated: ok
core: ok
hooks: ok
```

No marker names an unknown rule. The rule set is the 39 rules from `cinch rules --json` across 8 rule
docs; `cinch ignores` reports `0 cinch:ignore declarations found`.

## 2. Testable or N/A

All 39 rules are testable: each describes stored state, a computed value or a call sequence that a
unit or widget test can observe. None is N/A, so no `cinch:ignore` is needed. The repo has no
existing ignores to hold to that standard.

## 3. Derive coverage from the tests, then compare

**Mapping first.** I read every test file under `test/` (88 files) with `cinch:rule` comments
stripped, and the rule texts with the `markers` field dropped. I wrote one line per test (plumbing
groups collapsed) saying what the assertions pin and which rules they match. That is
`budgetron-mapping.md`, finished at 2026-11-01T02:11:47+03:00, before any marker was read.

**Then the markers.** I compared every marker from `cinch rules --json` (179 marker sites) with that
mapping and read each marked test against its rule's full text. Verdicts:

- **covered**: the marked tests assert every clause of the rule.
- **tension**: an assertion in a marked test contradicts the rule's text.
- **under_enforced**: the marked tests assert strictly less than the rule, and no other test covers the rest.
- **marker_misplaced**: some clause is enforced only by a test that has no marker for this rule.
- **missing**: no test enforces the rule. None found.

| Verdict | Count |
|---|---|
| covered | 28 |
| tension | 2 (SMS-002, SMS-008) |
| under_enforced | 5 (MONEY-001, SET-002, SMS-001, SMS-003, SMS-007) |
| marker_misplaced | 4 (ENTRY-005, ENTRY-006, SMS-004, SMS-006) |
| missing | 0 |
| na | 0 |

## 4. Report

## .docs/domain/accounts.md

| Rule | Rule text (verbatim) | Testable | Covered | Test file | Test | Assertion (verbatim) |
|------|----------------------|----------|---------|-----------|------|----------------------|
| ACCT-001 | `Balance equals opening balance plus the sum of the account's entries plus the sum of its transactions plus transfers in minus transfers out.` | Yes | ✅ | `test/logic/account/balance_calculator_test.dart:93` | `over a store / the repository sums feed the calculator exactly` | `expect(balanceOf(a), const Money(10000 + 3750 + 300 + 50 - 700));` |
| ACCT-002 | `` At most one account is default, and `Settings.defaultAccountId` points at it (or is `-1` when none). `` | Yes | ✅ | `test/logic/account/account_service_test.dart:58` | `at most one default account / unchecking the default account resets the setting to -1` | `expect(settings().defaultAccountId, -1);` |
| ACCT-003 | `A transfer changes the two balances by equal and opposite amounts.` | Yes | ✅ | `test/logic/account/balance_calculator_test.dart:115` | `over a store / a transfer moves equal and opposite amounts, the total is unchanged` | `expect(balanceOf(a) + balanceOf(b), beforeA + beforeB);` |
| ACCT-004 | `` Deleting an account, in one transaction, removes its transactions and every transfer from or to it, clears it from its entries, SMS templates and pending entries (they stay), and resets `Settings.defaultAccountId` to `-1` if it was the default. `` | Yes | ✅ | `test/logic/account/account_delete_test.dart:195` | `deleting an account / a failure rolls everything back` | `expect(store.box<Transfer>().count(), 3);` |

## .docs/domain/budgets.md

| Rule | Rule text (verbatim) | Testable | Covered | Test file | Test | Assertion (verbatim) |
|------|----------------------|----------|---------|-----------|------|----------------------|
| BUDGET-001 | `Period starts are: week on Monday, month on day 1, year on January 1.` | Yes | ✅ | `test/logic/budget/budget_calculator_test.dart:35` | `periodStart / week of $date starts on Monday $start` | `expect(calc.periodStart(BudgetPeriod.week, date).weekday, 1);` |
| BUDGET-002 | `` Spent for a period is the sum of absolute values of the category's expense entries dated in `[periodStart, periodEnd)`. `` | Yes | ✅ | `test/logic/budget/budget_service_test.dart:193` | `spent is computed from the entries / entries are counted by category and date, linked or not, with boundaries in the later period` | `expect(summary.periods.map((p) => p.spent.minor), [300, 400]);` |
| BUDGET-003 | `Any number of elapsed periods roll over, not just one.` | Yes | ✅ | `test/logic/budget/budget_calculator_test.dart:169` | `spans / every missed period yields a span, not just one` | `expect(spans, hasLength(15));` |
| BUDGET-004 | `A past period is evaluated against the target in effect during that period.` | Yes | ✅ | `test/logic/budget/budget_service_test.dart:303` | `periods / changing the target keeps the past target and replaces a revision of the same period` | `expect(summary.periods.map((p) => p.span.target.minor), [100, 100, 200]);` |
| BUDGET-005 | `Creating a budget marks its category budget-tracked; deleting the budget clears the flag; deleting a tracked category deletes its budget.` | Yes | ✅ | `test/logic/budget/budget_service_test.dart:164` | `lifecycle / deleting a tracked category deletes its budget in the same transaction` | `expect(store.box<Budget>().count(), 0);` |
| BUDGET-006 | `Archived budgets do not accrue new periods.` | Yes | ✅ | `test/logic/budget/budget_service_test.dart:404` | `archiving / an archived budget accrues no new periods` | `expect(summary.periods, hasLength(3));` |
| BUDGET-007 | `Changing a budget's period type takes effect at once. The new period starts at the start (per BUDGET-001) of the new-type period containing the change date. The previous period is truncated to end there, kept in history as a *partial* period, and evaluated against its full old target (not prorated). Periods never overlap, and every entry belongs to exactly one period. Changing only the target applies to the whole current period, and past periods keep theirs (BUDGET-004).` | Yes | ✅ | `test/logic/budget/budget_calculator_test.dart:292` | `spans / month to week on Wednesday the 20th: partial month, then weeks` | `expect(spans[2].target, const Money(100));` |
| BUDGET-008 | `Reading budgets never writes: computing summaries, however often and however long after the last visit, leaves every budget and revision row unchanged.` | Yes | ✅ | `test/logic/budget/budget_service_test.dart:433` | `archiving / reading summaries, however often and however late, writes nothing` | `expect(budgetRepository.writes, 0);` |

## .docs/domain/categories.md

| Rule | Rule text (verbatim) | Testable | Covered | Test file | Test | Assertion (verbatim) |
|------|----------------------|----------|---------|-----------|------|----------------------|
| CAT-001 | `` Category names are unique. Exact match, case-sensitive; adding or renaming to a taken name throws `DuplicateCategoryName` and leaves the stored rows unchanged. `` | Yes | ✅ | `test/db/category_repository_test.dart:46` | `category names are unique / renaming a category to a taken name is rejected` | `expect(repository.get(fun.id)!.name, 'Fun');` |
| CAT-002 | `` A category color string and its `Color` round-trip losslessly. `` | Yes | ✅ | `test/logic/category/category_service_test.dart:254` | `Convert Color to String and back to Color / Keeps the alpha channel, including values below 0x10` | `CategoryService.colorToString(color),` |

## .docs/domain/data-transfer.md

| Rule | Rule text (verbatim) | Testable | Covered | Test file | Test | Assertion (verbatim) |
|------|----------------------|----------|---------|-----------|------|----------------------|
| XFER-001 | `Export followed by import into an empty store is lossless: entries, categories, groups, accounts, transactions, transfers, budgets with their revisions, SMS templates, SMS category rules, and settings come back equal, with relations intact. Ids may change.` | Yes | ✅ | `test/db/transfer/round_trip_test.dart:58` | `export, import into an empty store, export again: the files are equal, and balances and budget summaries match the source` | `expect(second, first);` |
| XFER-002 | `Import is all-or-nothing. The whole file is validated before any write, and the replace runs in one transaction. An invalid file, or a failure part-way through, leaves the existing data exactly as it was.` | Yes | ✅ | `test/db/transfer/import_atomic_test.dart:239` | `a failure part-way through the replace leaves every box as it was` | `expect(snapshotAll(store), before);` |
| XFER-003 | `` Import replaces existing data only after the user explicitly confirms, and only after the current database has been copied to `data.mdb.pre-import-<timestamp>`. `` | Yes | ✅ | `test/db/transfer/data_transfer_service_test.dart:77` | `the backup is taken before any row changes, then the import runs` | `expect(backup.snapshotAtBackup, before);` |
| XFER-004 | `` Every file carries a format version. A file newer than the app supports is rejected; older formats are upgraded on import (a version-1 file has no SMS templates or category rules; a rule without a `mode`, as in versions 1 and 2, is a `contains` rule; a template without a `currencyRegex`, as in versions 1 to 3, reads no currency). Import switches the app's currency to the file's currency, since the amounts are in that currency's minor units. `` | Yes | ✅ | `test/db/transfer/format_version_test.dart:388` | `importing the version 1 fixture / switches the currency, theme and default periods to the file's` | `expect(settings.currencyCode, 'UAH');` |
| XFER-005 | `The file holds stored facts of the current model only: amounts as integer minor units (MONEY-001), opening balances not balances (ACCT-001), budget revisions not spent values (BUDGET-002). Derived values and retired legacy fields are never written.` | Yes | ✅ | `test/db/transfer/exporter_test.dart:218` | `what the exporter writes has only allow-listed keys` | `checkSchema(json);` |
| XFER-006 | `Import removes every pending SMS entry and every unrecognized SMS (SMS-008, neither is exported), since they point at the replaced categories and accounts, and keeps the SMS scan point.` | Yes | ✅ | `test/db/transfer/import_atomic_test.dart:322` | `import removes every pending entry whatever its status and keeps the SMS scan point` | `expect(store.box<UnrecognizedSms>().count(), 0);` |

## .docs/domain/entries.md

| Rule | Rule text (verbatim) | Testable | Covered | Test file | Test | Assertion (verbatim) |
|------|----------------------|----------|---------|-----------|------|----------------------|
| ENTRY-001 | `` Expense entries are stored with a negative value and income entries with a positive value; the sign comes from `category.isExpense`. `` | Yes | ✅ | `test/logic/entry/entry_service_update_test.dart:85` | `sign convention / an expense is stored negative, income positive, whichever sign the caller passes` | `expect(stored(negativeIncome.id).amountMinor, 700);` |
| ENTRY-002 | `Date-range filters are inclusive on both ends.` | Yes | ✅ | `test/db/entry_repository_test.dart:140` | `find date range is inclusive on both ends / entries one millisecond outside are excluded` | `expect(query(from: day1.add(ms), to: day3.subtract(ms)), [e2.id]);` |
| ENTRY-003 | `Combining filters (period, ids, category, account) is a logical AND.` | Yes | ✅ | `test/db/entry_repository_test.dart:84` | `find combines filters with AND / period and ids with no overlap` | `expect(query(from: day2Start, to: day2End, ids: {e1.id, e3.id}), isEmpty);` |
| ENTRY-004 | `Editing an entry updates it in place and keeps its id.` | Yes | ✅ | `test/logic/entry/entry_service_update_test.dart:122` | `update in place / keeps the id and changes value, date, category and account` | `expect(store.box<Entry>().count(), 1);` |
| ENTRY-005 | `` Creating, editing or deleting an entry and all dependent writes (category usages) are atomic. `EntryService` runs each of these in one `UnitOfWork`; deleting also decrements the category usages (never below 0), and an edit that changes the category moves one usage from the old category to the new one. Budget spent and account balances are derived on read, so no write maintains them. `` | Yes | ⚠️ MARKER MISPLACED | `test/logic/entry/entry_service_update_test.dart:140` | `update in place / the category selected in the UI is honored and usages move with it` | `expect((usages(food), usages(rent)), (0, 1));` |
| ENTRY-006 | `Inside a period group, sorting by amount lists income categories before expense categories, each side ordered by the absolute value of the category total: largest first for "amount, descending" (the default), smallest first for "amount, ascending". "Recent" keeps categories in the order of their newest entry.` | Yes | ⚠️ MARKER MISPLACED | `test/routes/pages/entry/cubit/entries_cubit_test.dart:158` | `sorting / the sort of the settings is applied, income first by default` | `expect((cubit.state as EntriesLoaded).sort, EntrySort.amountDesc);` |
| ENTRY-007 | `A period group and each of its categories expose their income sum and their expense sum separately; income plus expense equals the signed total (ENTRY-001).` | Yes | ✅ | `test/logic/entry/entry_grouping_test.dart:239` | `boundaries / income and expense are split by sign and add up to the total` | `expect(group.income + group.expense, group.total);` |

- **ENTRY-005** (⚠️ MARKER MISPLACED, confidence high): The ENTRY-005-marked tests (entry_service_atomic_test 'entry writes are atomic' group; entry_service_update_test 'a failing write rolls back the usages as well') pin create/delete atomicity, the decrement and the floor at 0, and edit rollback. Two clauses are pinned only by tests without an ENTRY-005 marker: the usage move on a category-changing edit (this test, marked ENTRY-004 only: `expect((usages(food), usages(rent)), (0, 1));`) and 'no write maintains' balances (account_balance_test 'no account write path touches the stored counter', unmarked: `expect(store.box<Account>().getAll().map((x) => x.balanceMinor), [0, 0]);`; plus 'balances and budget periods follow the edit, nothing else to maintain', marked ENTRY-004). Add `// cinch:rule ENTRY-005` above both.
- **ENTRY-006** (⚠️ MARKER MISPLACED, confidence medium): The marked test (entry_grouping_test 'amount sorts list income before expenses, ...') pins the order for each sort but not which sort is the default; the rule says amount descending is the default. That is pinned only by this unmarked test and by settings_cubit_test 'the entries sort defaults to largest first, survives a restart and falls back on an unknown name' (unmarked: `expect(first.state.entriesSort, EntrySort.amountDesc);`). Add `// cinch:rule ENTRY-006` above both.

## .docs/domain/money.md

| Rule | Rule text (verbatim) | Testable | Covered | Test file | Test | Assertion (verbatim) |
|------|----------------------|----------|---------|-----------|------|----------------------|
| MONEY-001 | `` Amounts are stored as integer minor units (`*Minor` fields), so sums are exact. The legacy `double` columns stay in the schema until the column-retirement release (plan 5 T11) and are `@Deprecated`; only `lib/db/migrations` reads them. `` | Yes | ⚠️ UNDER-ENFORCED | `test/models/legacy_fields_test.dart:12` | `no code outside the migration reads a legacy double or index field` | `const _fields = 'currencyId\|themeModeIndex\|targetValue\|currentValue\|endValue';` |
| MONEY-002 | `Display formatting rounds via the selected currency and never changes stored values.` | Yes | ✅ | `test/models/money_format_test.dart:22` | `formatMoney rounds only the text, for digits 0, 2 and 3` | `expect(formatMoney(money, currency: Currency.usd), '123.45');` |

- **MONEY-001** (⚠️ UNDER-ENFORCED, confidence low): The rule says the legacy double columns are @Deprecated and only lib/db/migrations reads them. The marked scan covers only currencyId/themeModeIndex/targetValue/currentValue/endValue/currencyIndex and exempts lib/models/; the doubles Entry.value, Transaction.value, Transfer.value and Account.balance are left to the analyzer (``// (Entry/Transaction/Transfer `value` is covered by the analyzer, through``), which only fires while the @Deprecated annotation is present, and nothing asserts the annotation. Missing: a test that every legacy double field carries @Deprecated (or a scan for `.value`/`.balance` reads on those entities outside lib/db/migrations).
- **MONEY-002** (note): Note: the 'never changes stored values' half is asserted by `expect(money, const Money(12345));`, which cannot fail on a const value; the clause holds by construction (Money is immutable, formatting is pure), so not flagged.

## .docs/domain/settings.md

| Rule | Rule text (verbatim) | Testable | Covered | Test file | Test | Assertion (verbatim) |
|------|----------------------|----------|---------|-----------|------|----------------------|
| SET-001 | `` Exactly one `Settings` row exists after startup. `` | Yes | ✅ | `test/db/settings_repository_test.dart:40` | `ensure leaves exactly one settings row / three rows are reduced to the lowest id` | `expect(rowCount(), 1);` |
| SET-002 | `` Schema migrations are versioned (`Settings.schemaVersion`), run in one transaction before the UI starts, and are idempotent. `` | Yes | ⚠️ UNDER-ENFORCED | `test/db/migrations/migrator_test.dart:108` | `versioning / all steps and the version bump share one transaction` | `expect(storedVersion(), 0);` |

- **SET-001** (note): Note: that main() calls ensure() at startup is untested (lib/main.dart:69); import paths keeping one row are pinned by unmarked tests, one of them named '... one settings row (SET-001)' without a marker.
- **SET-002** (⚠️ UNDER-ENFORCED, confidence medium): Versioned, one transaction and idempotent are pinned (14 marked tests). 'before the UI starts' is asserted nowhere: the ordering lives only in lib/main.dart (`// Order matters: backup (inside openAppStore) -> open -> ensure -> migrate`, then runApp) with no test. Missing: a startup test (e.g. extract bootstrap() and assert migrate runs and completes before the app widget is built, and a migration failure yields FatalErrorApp, not the UI).

## .docs/domain/sms-import.md

| Rule | Rule text (verbatim) | Testable | Covered | Test file | Test | Assertion (verbatim) |
|------|----------------------|----------|---------|-----------|------|----------------------|
| SMS-001 | `An SMS becomes at most one pending entry, ever: messages are recognized by a key (provider id plus receive time) whatever the pending entry's status (pending, confirmed, dismissed); a scan never recreates a dismissed one, but the user can undo a dismissal, which sets that entry back to pending (re-resolving its category by the current rules, SMS-003). A scan reads only messages received after its start point; the stored scan point only moves forward.` | Yes | ⚠️ UNDER-ENFORCED | `test/logic/sms/sms_import_service_test.dart:281` | `SMS-001 one pending entry per message / a dismissed or confirmed message is not recreated` | `expect(pendingFor('SHOP A').status, PendingEntryStatus.dismissed);` |
| SMS-002 | `` A message is parsed by the first enabled template (by id) whose sender filter and keywords match (template regexes and keywords are case-insensitive); the type is the keyword that occurs first; the amount is parsed into `Money` with the app currency's minor units and must be above zero; without a date match the receive time is used. A template error (bad regex, unparseable date) skips the message and never aborts the scan. `` | Yes | ⚠️ TENSION | `test/logic/sms/sms_import_service_test.dart:374` | `SMS-002 template errors never abort a scan / a template with an unparseable date skips the message` | `expect(await scan(), 1);` |
| SMS-003 | `` A category rule matches the merchant case-insensitively and, when no rule matches it (or there is no merchant), the whole SMS text, as its match mode says: `contains` (the default, and what rules stored without a mode mean) finds the keyword anywhere, `word` only as a whole word (bounded by the text's ends or a character that is neither a letter nor a digit), `prefix` only at the start of the text. Only categories of the pending entry's type count; within each pass the longest keyword wins and the earlier rule wins a tie. A pending entry's category only ever comes from the rules (the user sets one only by confirming), so rules are re-applied to all pending entries whenever one is saved, edited or removed, in the same transaction as the rule write: each category becomes whatever the current rules resolve, possibly none. Rules are editable (keyword, category, mode). Keywords are unique case-insensitively: saving a new rule with an existing keyword updates that rule, an edit that would give a rule another rule's keyword is refused and writes nothing. `` | Yes | ⚠️ UNDER-ENFORCED | `test/logic/sms/sms_import_service_test.dart:615` | `SMS-003 category rules / removeRule rolls back when re-applying fails` | `expect(services.sms.allRules(), hasLength(1));` |
| SMS-004 | `Confirming a pending entry creates the entry (ENTRY-001, ENTRY-005), marks the pending entry confirmed and, when the user gave a rule keyword (not blank after trimming; the form offers the whole merchant), saves the rule "keyword means the entry's category" (SMS-003), in one transaction. Confirm-all confirms only pending entries whose category has the pending entry's type.` | Yes | ⚠️ MARKER MISPLACED | `test/routes/pages/entry/cubit/entry_form_cubit_test.dart:477` | `from a pending entry / is pre-filled from the pending entry, not from the clock` | `expect(form.ruleKeyword, 'SHOP 1');` |
| SMS-005 | `Deleting a category deletes its rules and clears it from pending entries in the same transaction.` | Yes | ✅ | `test/logic/category/category_service_test.dart:103` | `deleting a category cascades to the SMS data / removes the rules and clears the pending entries` | `expect(keywords(), ['cinema']);` |
| SMS-006 | `A background scan (app start or resume) runs only when the read permission is already granted and a scan point exists; it never prompts and never asks for a start point. Scans run at most once a minute and never concurrently.` | Yes | ⚠️ MARKER MISPLACED | `test/cubit/sms_scan_cubit_test.dart:193` | `manual scan / a second scan while one runs is ignored` | `await cubit.scanInBackground();` |
| SMS-007 | `A template may read the currency code; a pending entry whose code differs from the app currency is never confirmed by confirm-all and is marked on the list.` | Yes | ⚠️ UNDER-ENFORCED | `test/routes/pages/entry/cubit/entry_form_cubit_test.dart:509` | `from a pending entry / an amount in another currency is marked on the form` | `).state.form.foreignCurrencyCode,` |
| SMS-008 | `A message no enabled template reads is kept as unrecognized when it looks like a transaction (an amount followed by a three-letter code in capitals), at most once per message; saving or enabling a template re-reads the kept ones, and those it reads become pending entries. A dismissed unrecognized message is never shown again.` | Yes | ⚠️ TENSION | `test/logic/sms/sms_parser_test.dart:633` | `looksLikeTransaction / rejects codes, plain text and amounts without a currency` | `expect(looksLikeTransaction('Pay 5 BYN'), isFalse);` |

- **SMS-001** (⚠️ UNDER-ENFORCED, confidence low): Every behavioural clause is pinned (dedupe across statuses, no recreate, undo-dismiss re-resolves, start point, monotone scan point). The key definition '(provider id plus receive time)' is not: tests compare against the same getter (`expect(shop.smsKey, _card(1, '').key);`), so changing the key to id-only (or to body) passes. Missing: two messages with the same provider id and different receive times both become pending, or `expect(sms.key, '${id}:${date.millisecondsSinceEpoch}')`.
- **SMS-002** (⚠️ TENSION, confidence medium): Rule: 'parsed by the first enabled template ... whose sender filter and keywords match' and 'A template error (bad regex, unparseable date) skips the message'. In this test the bad-date template (lower id) matches sender and keyword, fails on the date, and the message is still imported by the next template (the preset): scan() returns 1. Same in 'a broken template is skipped and the others still import' (`expect(await scan(), 2);`). Behaviour: the erroring template is passed over and the next enabled template is tried; the message is not skipped. The rule's literal reading contradicts the assertion; reword to 'a template error makes that template pass over the message (the next enabled template is tried) and never aborts the scan'.
- **SMS-003** (⚠️ UNDER-ENFORCED, confidence medium): Re-applying on save, edit and remove is pinned, and so is case-insensitive uniqueness, refusal of a clashing edit, modes and passes. 'in the same transaction as the rule write' is pinned only for remove (this test). No test fails the re-apply during saveRule or an edit and checks the rule write is rolled back. Missing: with failPutPending=true, `saveRule('sosedi', food)` throws and leaves `allRules()` empty and pending categories unchanged; same for an edit via `id:`.
- **SMS-004** (⚠️ MARKER MISPLACED, confidence medium): The marked tests (service 'SMS-004 confirming' group; entry form 'submit passes the keyword the user typed', 'a blank keyword saves no rule') pin create/confirm/rule/one-transaction/confirm-all. 'the form offers the whole merchant' is pinned only here, without a marker. Add `// cinch:rule SMS-004` above this test.
- **SMS-006** (⚠️ MARKER MISPLACED, confidence medium): The six marked background tests pin permission/scan-point gating, no prompt, no start-point request and the one-minute throttle. 'never concurrently' is pinned only by this unmarked test (a manual scan in flight, then scan() and scanInBackground(), service.scan called once). Add `// cinch:rule SMS-006` above it. The '(app start or resume)' triggers are wired in lib/main.dart (addPostFrameCallback, AppLifecycleState.resumed) and untested; not flagged separately.
- **SMS-007** (⚠️ UNDER-ENFORCED, confidence low): The code is read and stored, and confirm-all skips foreign entries (marked service tests). The rule says the entry 'is marked on the list'; the marked test asserts a mark on the confirm form, not the list. The list mark is rendered in lib/routes/pages/sms/widgets/pending_entry_list_tile.dart (`isForeign ? entry.currencyCode : currency.code`) with no test. Missing: a widget test on PendingEntryListTile (or a list-state flag) showing the entry's code when it differs from the app currency; or reword the rule to name the form.
- **SMS-008** (⚠️ TENSION, confidence medium): Rule: kept 'when it looks like a transaction (an amount followed by a three-letter code in capitals)'. 'Pay 5 BYN' is an amount followed by a capitalised code, and the marked test asserts it is not kept. The code (lib/logic/sms/sms_parser.dart:68, `\d[\d ]*[.,]\d{2}\s*[A-Z]{3}\b`) and its doc comment require 'an amount with two decimals'; the rule dropped that qualifier. Everything else in SMS-008 is pinned.

## 5. Suggest tests for gaps

The workflow asks for this for MISSING and under-enforced rules. The two TENSION rows are included
too, since each one needs either the rule or the test to change. For marker_misplaced rows the fix
is only to move a marker; where it belongs is listed at the end.

**SMS-002 (TENSION).** The behaviour looks deliberate: an erroring template is passed over and the
next enabled template is tried. So the fix is probably the rule text, not the code. Proposed text:
"A template error (bad regex, unparseable date) makes that template pass over the message, the
next enabled template is tried, and the scan is never aborted." Keep the existing test
`a template with an unparseable date skips the message` in
`test/logic/sms/sms_import_service_test.dart`, renamed to
`a template with an unparseable date passes the message to the next template`, under the existing
group marker `// cinch:rule SMS-002`.
To pin the case where every template errors, add
`a message every matching template fails on is not imported and the scan goes on` in the same
group: two messages, the only enabled template has a bad date regex for the first one, then
`expect(await scan(), 1);` and the second message becomes pending.

**SMS-008 (TENSION).** Change the rule to "an amount with two decimals followed by a three-letter
code in capitals", which is what `_amountWithCode` and its doc comment say. Alternatively, if
integer amounts should be kept, change the regex and flip
`expect(looksLikeTransaction('Pay 5 BYN'), isFalse);`. Either way it stays in
`test/logic/sms/sms_parser_test.dart` under `// cinch:rule SMS-008`.

**MONEY-001 (under-enforced).**
- Test file: `test/models/legacy_fields_test.dart`.
- New test, next to the existing scans: `every legacy double column is @Deprecated`.
- Assertion: for each of `lib/models/entry.dart`, `lib/models/account/transaction.dart`,
  `lib/models/account/transfer.dart` (`double value;`) and `lib/models/account/account.dart`
  (`double balance;`), the line before the field declaration matches `@Deprecated(`:
  `expect(missing, isEmpty);`.
- Marker: `// cinch:rule MONEY-001`.

**SET-002 (under-enforced).**
- Test file: new `test/startup_test.dart`, after extracting `main()`'s try block into a testable
  `Future<StartupResult> bootstrap(...)`.
- New test: `migrations run and finish before the app is built`.
- Assertion: given a v1-shaped store, `bootstrap` returns after `Settings.schemaVersion ==
  Migrator.currentSchemaVersion`, and a throwing step yields the fatal-error result, never the app.
  For example `expect(result, isA<StartupFailed>());`.
- Marker: `// cinch:rule SET-002`. The same test can also pin SET-001's "after startup":
  `expect(store.box<Settings>().count(), 1);`.

**SMS-001 (under-enforced).**
- Test file: `test/logic/sms/sms_import_service_test.dart`, group `SMS-001 one pending entry per message`.
- New test: `messages with the same provider id but different receive times are distinct`.
- Assertion: two `SmsMessage`s with `id: 1` and different `date`, then `expect(await scan(), 2);`,
  then rescan `expect(await scan(), 0);`.
- Marker: under the existing group marker `// cinch:rule SMS-001`.

**SMS-003 (under-enforced).**
- Test file: `test/logic/sms/sms_import_service_test.dart`, group `SMS-003 category rules`.
- New tests: `saveRule rolls back when re-applying fails` and `an edit rolls back when re-applying fails`.
- Assertion for the first: scan a matching message, set `sms.failPutPending = true`, then
  `expect(() => service.saveRule('sosedi', food), throwsStateError);` and, after resetting,
  `expect(services.sms.allRules(), isEmpty);` and
  `expect(pendingFor('SHOP SOSEDI').category.target, isNull);`.
- Assertion for the edit: the rule keeps its old keyword and category.
- Marker: under the existing group marker `// cinch:rule SMS-003`.

**SMS-007 (under-enforced).**
- Test file: new `test/routes/pages/sms/widgets/pending_entry_list_tile_test.dart`.
- New test: `an entry in another currency shows its own code on the list`.
- Assertion: pump `PendingEntryListTile` with `currencyCode: 'USD'` under an app currency of BYN,
  then `expect(find.textContaining('USD'), findsOneWidget);`. With `'BYN'` the BYN code shows.
- Marker: `// cinch:rule SMS-007`. Alternatively reword the rule to "is marked on the confirm form",
  which the existing marked test already pins.

**Markers to move (marker_misplaced):**
- ENTRY-005: add `// cinch:rule ENTRY-005` above
  `test/logic/entry/entry_service_update_test.dart` `the category selected in the UI is honored and
  usages move with it` (it may keep ENTRY-004), and above
  `test/logic/account/account_balance_test.dart` `no account write path touches the stored counter`.
- ENTRY-006: add `// cinch:rule ENTRY-006` above
  `test/routes/pages/entry/cubit/entries_cubit_test.dart` `the sort of the settings is applied,
  income first by default`, and above `test/cubit/settings_cubit_test.dart` `the entries sort
  defaults to largest first, survives a restart and falls back on an unknown name`.
- SMS-004: add `// cinch:rule SMS-004` above
  `test/routes/pages/entry/cubit/entry_form_cubit_test.dart` `is pre-filled from the pending entry,
  not from the clock`.
- SMS-006: add `// cinch:rule SMS-006` above `test/cubit/sms_scan_cubit_test.dart` `a second scan
  while one runs is ignored`.

## Before finishing

- [x] The rule set came from `cinch rules --json`, after a green `cinch check`
- [x] The test→rule mapping was derived from assertions before any marker was read. Rule text was
  printed without `markers` and tests were read with marker comments stripped. Two small leaks are
  described in Audit notes.
- [x] Every marked test was read against its rule's text. All 179 marker sites were resolved to
  their test; for group markers, every test in the group was read.
- [x] Every row quotes its rule text and assertion verbatim. Each assertion cell was checked
  byte-for-byte against `file:line` by the generator script.
- [x] Every N/A rule has a `cinch:ignore` reason in its doc. There are no N/A rules.

## Audit notes

- **Commit audited:** `0fe3070f39721c57e91e99c250d51fd4ed75e4af` (dev, committed 2026-10-28). The
  repo was not modified: no edits, stages, renders or test runs.
- **cinch version:** 0.13.2.
- **Tests read:** all 88 files under `test/` (16,291 lines, about 813 `test(`/`blocTest(` call
  sites, more at runtime from generated cases). The 8 support helpers under `test/support/` were
  read where a rule's evidence depends on them (`transfer_seed.dart`, `store_snapshot.dart`).
  Production code was read only to settle a verdict: `sms_parser.dart`, `main.dart`,
  `store_backup.dart`, the `@Deprecated` model fields, and the analyzer config and Makefile.
- **Pre-marker hygiene, two leaks.** (1) Stripping the marker comments left blank lines, so I could
  see *that* a test was marked, though not for which rule. (2) Some test names and group names
  contain rule IDs as prose, for example `SMS-001 one pending entry per message`,
  `a taken name is reported as a duplicate (CAT-001)`, `... one settings row (SET-001)`,
  `the range is inclusive on both ends (ENTRY-002)`, and `BUDGET-008` in a doc comment. They are
  test text, not markers, and I read them as written. Neither leak changes a verdict: every verdict
  rests on assertions read against the full rule text.

**Confidence per flagged row:**

| Rule | Verdict | Confidence | Why not higher |
|---|---|---|---|
| SMS-002 | tension | medium | "skips the message" could have meant "the template skips it". The literal text and "parsed by the first ... template whose sender filter and keywords match" both contradict `expect(await scan(), 1);`. |
| SMS-008 | tension | medium | A reader could take "an amount" to mean a decimal amount, but the code's own doc comment says "with two decimals" and the rule dropped it. |
| SET-002 | under_enforced | medium | The clause is certainly unasserted. It is in startup wiring that is awkward to test, and the order is commented in `main.dart`. |
| SMS-003 | under_enforced | medium | Re-applying on save and edit runs inside the same `UnitOfWork` in code. It is just never proven by a rollback test the way remove is. |
| SMS-001 | under_enforced | low | The key composition is a mechanism in parentheses. Behaviour only diverges when a provider id is reused. |
| SMS-007 | under_enforced | low | The `isForeign` predicate is tested and the list tile uses it. Only the rendering is untested, and the rule may mean the form, which is tested. |
| MONEY-001 | under_enforced | low | `make check` runs `flutter analyze` with `deprecated_member_use_from_same_package`, a real guard as long as the annotations stay. Only the annotations themselves are unasserted. |
| ENTRY-005 | marker_misplaced | high | The behaviour is fully tested. Only the traceability is wrong. |
| ENTRY-006 | marker_misplaced | medium | The only unmarked clause is the parenthetical "(the default)". |
| SMS-004 | marker_misplaced | medium | The only unmarked clause is the parenthetical "the form offers the whole merchant". |
| SMS-006 | marker_misplaced | medium | "never concurrently" is pinned, but in the manual-scan group without the marker. |

**Ambiguous or noted, not flagged:**

- **Dead write path against ACCT-002.** `SettingsCubit.setDefaultAccountId` writes
  `Settings.defaultAccountId` without touching `Account.isDefault`. `test/cubit/settings_cubit_test.dart`
  pins it with no accounts present: `expect(row.defaultAccountId, 7)`, and
  `expect(restarted.state.defaultAccountId, 3)`. `lib/` has no caller today. Wired into the UI, it
  would break ACCT-002 without any marked test failing.
- **Helper drift in `test/support/store_snapshot.dart`.** Its doc says "Every stored field of every
  row in every box", but it omits `CategoryRule.modeName`, `SmsTemplate.currencyRegex` and
  `PendingEntry.currencyCode`, all added in format versions 3–4. XFER-002's "exactly as it was" and
  XFER-003's "the copy opens with exactly the rows the store had" both lean on this snapshot. The
  importer replaces rows rather than updating them in place, so the practical gap is small. Adding
  the three fields is a one-line fix.
- **Untested glue for XFER-003.** `lib/db/transfer/store_backup.dart`, the production
  `DatabaseBackup`, joins the two tested pieces (`preImportSuffix`, `backupOpenStore`) and has no
  test. The service test uses a fake backup.
- **SMS-003 tie order.** "the earlier rule wins a tie" means earlier in `allRules()` order, which
  is by keyword (`b.order(CategoryRule_.keyword)`), not by creation. The parser test pins list order
  only. A reader would probably assume creation order. The rule could say so.
- **XFER-005 and `usages`.** The file carries the category `usages` counter (allow-listed in
  `export_schema_test.dart`). Per ENTRY-005 it is a maintained "dependent write", not a value
  derived on read, so it doesn't contradict "derived values are never written" under the docs' own
  terms. Flag it if the owner considers it derived.
- **MONEY-002.** `expect(money, const Money(12345));` cannot fail. "Never changes stored values"
  holds by construction, so the row is covered.
- **Markers on behaviour no rule states.** Three XFER-001 markers sit on exporter tests for orphan
  handling: `a template whose account was deleted is exported without one`, and `a rule whose
  category no longer exists is left out, not an error` (that one drops data, so it is not
  lossless). XFER-002 marks `a store with no settings row gets exactly one`, which is SET-001
  content.
- **Behaviour the code pins that no rule states** (candidates for item B, the density census):
  - pending-entry "possible duplicate" flag: same absolute amount within ±24 h
  - a used category cannot be deleted
  - an entry's type cannot be switched on edit
  - the preset template is auto-added when there are none
  - pending account resolution: the template's account, else the default account
  - the pre-migration `data.mdb.pre-vN` backup
  - accounts sorted by name ignoring case

## Verification

An independent verifier checked every flagged row adversarially against the code and tests at
`0fe3070` (read-only, no test runs). It also spot-checked 8 covered rows, one per rule doc: ACCT-004, BUDGET-007,
CAT-001, XFER-003, ENTRY-002, SET-001, SMS-005, MONEY-002. Per-row evidence is in
[`budgetron-verified.csv`](budgetron-verified.csv). The owner confirmed SMS-002 and SMS-008 (tension) beforehand, so they were not
re-verified.

**Audit rows.** All 9 audit-flagged rows that went to verification were confirmed and none was
refuted. The 5 under_enforced rows hold: SET-002, SMS-003, SMS-001, SMS-007 and MONEY-001. The 4
marker_misplaced rows hold too: ENTRY-005, ENTRY-006, SMS-004 and SMS-006.

**Verifier findings beyond the audit** (not owner-confirmed):

- **SMS-006: tension, medium confidence.** The rule says "Scans run at most once a minute", and
  `lib/cubit/sms_scan_cubit.dart:14` says "Scans (manual or background) start at most this often". But
  `scan()` (`:42`, `:79`) never checks the one-minute window, and the scan button calls it
  (`lib/routes/pages/sms/pending_entry_list_page.dart:125`). The marker finding still holds.
- **SET-001: covered becomes under_enforced, low severity.** "after startup" rests only on
  `lib/main.dart:69` `settingsRepository.ensure();`, which no test runs. It's the same gap as SET-002, judged
  consistently.
- The other 7 spot checks hold as covered. BUDGET-007 has a note: the week-to-month change absorbs
  completed weeks, which is consistent with the literal text.

| Verified count | Audit-only (confirmed) | Including verifier findings |
|---|---|---|
| tension | 2 (SMS-002, SMS-008) | 3 (+SMS-006) |
| under_enforced | 5 (SET-002, SMS-003, SMS-001, SMS-007, MONEY-001) | 6 (+SET-001) |
| marker_misplaced | 4 (ENTRY-005, ENTRY-006, SMS-004, SMS-006) | 3 (SMS-006 moves to tension) |
| missing | 0 | 0 |
| refuted | 0 of 9 flagged | 1 of 8 covered spot checks (SET-001) |

**Rates out of 39 rules:**

| Rate | Audit-only | Including verifier findings |
|---|---|---|
| (a) stale rule text (tension) | 2/39 = 5.1% | 3/39 = 7.7% |
| (b) stale or under-enforced (plan 29 definition) | 7/39 = 17.9% | 9/39 = 23.1% |

Rate (b) straddles the pre-registered 20% line, depending on two verifier-found rows that the owner
hasn't confirmed. Four of the under-enforced rows are low severity and leave a parenthetical
mechanism or UI clause unasserted: MONEY-001's annotation, SMS-001's key composition, SMS-007's
list mark and SET-001's startup call. Without them, (b) is 4/39 (audit-only) or 5/39 (including SMS-006). Every under-enforced clause
except SMS-003's is one that only startup wiring, a widget or an annotation would pin, which the
test suite doesn't reach.
