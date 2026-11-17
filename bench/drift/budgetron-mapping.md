# Budgetron — pre-marker test→rule mapping (plan 29 item A, drift census)

Repo: /home/romius/code/budgetron, branch dev, commit 0fe3070f39721c57e91e99c250d51fd4ed75e4af.
Workflow: `cinch workflow docs-audit-coverage` (cinch 0.13.2), step 3 "derive the mapping without looking at markers".

**Method.** Rule texts came from `cinch rules --json` with the `markers` field dropped before
printing. Every test file was read from a copy with `cinch:rule` comments stripped
(`sed -E 's#(//|/\*|\*)\s*cinch:rule.*$##'`), so marker IDs were never seen. Two leaks, noted
honestly: (1) stripping left a blank line where a marker had been, which shows *that* a test is
marked but not *which* rule; (2) a few test names and helper comments embed rule IDs in prose
(e.g. "SMS-001 one pending entry per message", "(CAT-001)", "(SET-001)", "(ENTRY-002)",
"BUDGET-008" in a doc comment); these are test text, not markers, and are kept as read.
Each line says what the assertions pin (state/response), not what the name claims, and which
rules it matches. Groups of plumbing tests with no rule are collapsed into one line per group.

**Finished writing this mapping: 2026-11-01T02:11:47+03:00** (before any marker was read).

## Accounts (lib/logic/account, account repo, models/account, routes/pages/account)

| File | Test | What the assertions pin | Rules matched |
|---|---|---|---|
| test/logic/account/balance_calculator_test.dart | balance is opening plus entries, transactions and transfers in, minus transfers out | `BalanceCalculator.balance` returns opening+entries+transactions+in-out on a concrete vector and zero on all-zero inputs | ACCT-001 |
| test/logic/account/balance_calculator_test.dart | the repository sums feed the calculator exactly | `accounts.sums(id)` returns per-account entry/transaction/transfer-in/out sums (ignoring other account's entries) and `fromSums` balance equals the formula for both accounts | ACCT-001 |
| test/logic/account/balance_calculator_test.dart | a transfer moves equal and opposite amounts, the total is unchanged | after inserting one transfer A->B, A drops by 10, B rises by 10, A+B unchanged | ACCT-003 |
| test/logic/account/balance_calculator_test.dart | an account without operations shows its opening balance | balance with no operations == opening | ACCT-001 (edge) |
| test/logic/account/account_balance_test.dart | a new account shows the balance it was created with | balanceOf == created balance and stored openingBalanceMinor == it | ACCT-001 |
| test/logic/account/account_balance_test.dart | create 100, expense -30, transfer 10 out shows 60; deleting the expense shows 90 | via services: expense stored as -3000, balance 7000, after transfer 6000/1000, after deleting entry 9000 | ACCT-001, ACCT-003, ENTRY-001 (sign), ENTRY-005 (balance derived on read) |
| test/logic/account/account_balance_test.dart | transactions move the balance by their signed amount | two transactions -250, +1010 on 500 opening -> 1260 | ACCT-001 |
| test/logic/account/account_balance_test.dart | ten transfers of 0.10 leave the total of both balances unchanged | 10 x 10 minor transfers: A -100, B +100, sum unchanged (exact minor) | ACCT-003, MONEY-001 (exact sums) |
| test/logic/account/account_balance_test.dart | editing the balance sets opening = typed - sum of operations | updateAccount(balance: 5000) => balance 5000, opening 4700, name saved | ACCT-001 (opening derived from typed balance) |
| test/logic/account/account_balance_test.dart | no account write path touches the stored counter | after entry/transfer/transaction/update, legacy balanceMinor stays 0 on all accounts | ENTRY-005 ("no write maintains" balances), XFER-005-adjacent (balance not stored) |
| test/logic/account/account_balance_test.dart | watchBalance and watchBalances follow every operation | balance streams emit 1000, 700, 1000 as entry added and deleted | ACCT-001 (reactive) |
| test/logic/account/account_balance_test.dart | watchAccountsWithBalances lists the accounts with their balances | stream lists accounts with correct balances after a new account+transaction | ACCT-001 |
| test/logic/account/account_balance_test.dart | watchAccountsWithBalances sorts by name ignoring case, ties by id | ordering of accounts list | none (undocumented ordering) |
| test/logic/account/account_service_test.dart | at most one default account / setDefaultAccount clears every other default | after setDefault(b), only b isDefault and settings.defaultAccountId==b | ACCT-002 |
| test/logic/account/account_service_test.dart | unchecking the default account resets the setting to -1 | updateAccount with isDefault false => setting -1, stored isDefault false | ACCT-002 |
| test/logic/account/account_service_test.dart | creating a default account demotes the previous default | create default D => only D default, setting == D | ACCT-002 |
| test/logic/account/account_delete_test.dart | deleting an account / keeps its entries and clears their account | entry of deleted account survives with account cleared and amount intact; other entry untouched | ACCT-004 |
| test/logic/account/account_delete_test.dart | removes its transactions and every transfer from or to it | only other account's transaction and the other->third transfer remain | ACCT-004 |
| test/logic/account/account_delete_test.dart | the other balances lose the removed transfers | balance of other/third recomputed without removed transfers | ACCT-004, ACCT-001 |
| test/logic/account/account_delete_test.dart | clears it from SMS templates and pending entries, which stay | template's account cleared, other template keeps it; pending entry stays with account cleared | ACCT-004 |
| test/logic/account/account_delete_test.dart | the default account resets to -1 | deleting default => settings.defaultAccountId == -1 | ACCT-004, ACCT-002 |
| test/logic/account/account_delete_test.dart | deleting another account leaves the default alone | deleting non-default leaves setting and isDefault | ACCT-004 (only-if clause) |
| test/logic/account/account_delete_test.dart | a failure rolls everything back | failing the final account remove throws and leaves account, entry link, 2 transactions, 3 transfers, templates, pending link, default all as before | ACCT-004 (one transaction) |
| test/routes/pages/account/cubit/account_form_cubit_test.dart | create / starts empty with a zero balance | initial create form state is empty, balance 0, not submittable | none |
| test/routes/pages/account/cubit/account_form_cubit_test.dart | create / needs a name, a color and a parseable balance | canSubmit toggles on name+color+parseable balance; '10.5' parses to 1050 minor | none (form validation; MONEY-001 parse only incidentally) |
| test/routes/pages/account/cubit/account_form_cubit_test.dart | create / submit creates the account with the typed balance | mock createAccount receives name, isDefault, color string and Money(9999) | none |
| test/routes/pages/account/cubit/account_form_cubit_test.dart | create / submit with an incomplete form fails | emits invalidForm failure | none |
| test/routes/pages/account/cubit/account_form_cubit_test.dart | create / a failing create (e.g. a taken name) emits Failure | service throw -> saveFailed | none |
| test/routes/pages/account/cubit/account_form_cubit_test.dart | edit / starts from the account and its current balance | edit form prefilled from account and service balance '120.50' | none |
| test/routes/pages/account/cubit/account_form_cubit_test.dart | edit / can only be saved once something changed | canSubmit only when a field differs | none |
| test/routes/pages/account/cubit/account_form_cubit_test.dart | edit / an empty name cannot be saved | canSubmit false with empty name | none |
| test/routes/pages/account/cubit/account_form_cubit_test.dart | edit / submit updates the account in place with the typed balance | mock updateAccount receives id 3, new name, Money(5000) | none |
| test/routes/pages/account/cubit/account_form_cubit_test.dart | edit / a failing update leaves the account as it was | failure state; the passed Account object not mutated | none |
| test/routes/pages/account/cubit/account_form_cubit_test.dart | edit / delete removes the account and emits Done as deleted | calls deleteAccount(3) once, Done(wasDeleted) | none (ACCT-004 is in the service) |
| test/routes/pages/account/cubit/account_form_cubit_test.dart | edit / a failing delete emits a failure key | deleteFailed | none |
| test/routes/pages/account/cubit/account_form_cubit_test.dart | create (delete) / nothing can be deleted while creating | no emit, no deleteAccount | none |
| test/routes/pages/account/cubit/account_operations_cubit_test.dart | 10 tests (starts on the account period...; emits the operations...; groups the entries by category...; setPeriod cancels...; shift back...; cannot step before the first operation...; an account without operations starts at now; cannot step after the current period; a failing stream emits Failure; closing cancels the subscription) | period navigation of an account's operation list over a mocked stream; day range [00:00, 23:59:59.999]; group total 750 = -300+1000+50 | none (ENTRY-002 inclusive-range shape incidental, but through a mock, not a filter) |
| test/routes/pages/account/cubit/accounts_cubit_test.dart | 4 tests (emits the accounts with their balances; follows every change; failing stream; closing cancels) | cubit relays mocked AccountWithBalance stream | none |
| test/routes/pages/account/cubit/transaction_form_cubit_test.dart | 6 tests (starts empty; keyboard builds amount; type/account/date/time change; submit without valid amount; a debit is stored positive; a credit is stored negative; failing save) | form state; debit -> +500, credit -> -500 transaction sign | none (transaction sign undocumented) |
| test/routes/pages/account/cubit/transfer_cubit_test.dart | 7 tests (starts from given account; needs destination and amount above zero; destination must differ; changing source clears destination; submit creates transfer; invalid transfer fails; failing transfer) | transfer form validation: amount > 0 at minor precision (0.004 rejected), from != to; submitted transfer 1234 minor | none (ACCT-003 is the balance effect, not tested here) |

## Budgets (lib/logic/budget, budget repo, models/budget, routes/pages/budget)

| File | Test | What the assertions pin | Rules matched |
|---|---|---|---|
| test/logic/budget/budget_calculator_test.dart | periodStart / week of $date starts on Monday $start (12 generated cases) | periodStart(week) returns the Monday at 00:00 on or before the date, incl. year boundary, leap day, DST days; weekday==1 | BUDGET-001 |
| test/logic/budget/budget_calculator_test.dart | periodStart / month of $date starts on $start (6 cases) | periodStart(month) returns day 1 of that month | BUDGET-001 |
| test/logic/budget/budget_calculator_test.dart | periodStart / year starts on January 1 | periodStart(year) returns Jan 1 | BUDGET-001 |
| test/logic/budget/budget_calculator_test.dart | nextStart / week adds seven calendar days, also over DST changes | next week start is +7 calendar days at hour 0 | BUDGET-001 (period boundaries), BUDGET-003 support |
| test/logic/budget/budget_calculator_test.dart | nextStart / month and year | next month/year boundaries incl. Dec->Jan | BUDGET-001 |
| test/logic/budget/budget_calculator_test.dart | spans / no revisions, no spans | empty input -> no spans | none |
| test/logic/budget/budget_calculator_test.dart | spans / a revision starting in the current period gives one open span | mid-month revision -> one span [May 1, Jun 1) | BUDGET-001, BUDGET-007 (target-change-whole-period flavour) |
| test/logic/budget/budget_calculator_test.dart | spans / a revision in the future produces nothing | future revision -> no span | none |
| test/logic/budget/budget_calculator_test.dart | spans / a span starting exactly at now exists | span starting at now included, 3 spans | BUDGET-003 (edge) |
| test/logic/budget/budget_calculator_test.dart | spans / every missed period yields a span, not just one | 15 contiguous monthly spans from Jan 2023 to Mar 2024, none partial | BUDGET-003 |
| test/logic/budget/budget_calculator_test.dart | spans / weeks and years roll over any number of times | 11 contiguous week spans, 6 contiguous year spans | BUDGET-003 |
| test/logic/budget/budget_calculator_test.dart | spans / a past period keeps the target in effect during it | Jan/Feb 100, Mar/Apr 150 | BUDGET-004 |
| test/logic/budget/budget_calculator_test.dart | spans / a mid-period target change applies to the whole current period | revision on Mar 20 -> March span has target 150, starts Mar 1 | BUDGET-007 (target-only clause), BUDGET-004 |
| test/logic/budget/budget_calculator_test.dart | spans / the order of the input does not matter, the later effectiveFrom wins on equal starts | shuffled revisions give [100,100,150] | BUDGET-007/BUDGET-004 (tie-break; undocumented detail) |
| test/logic/budget/budget_calculator_test.dart | spans / an archived budget accrues no periods after the archive date | archivedAt Mar 10 -> 3 spans, last is March | BUDGET-006 |
| test/logic/budget/budget_calculator_test.dart | spans / archivedAt in the future or before any revision changes nothing or yields nothing | future archive = no effect; archive before revision = no spans | BUDGET-006 |
| test/logic/budget/budget_calculator_test.dart | spans / month to week on Wednesday the 20th: partial month, then weeks | exact span list: March truncated at Mon Mar 18, partial, target 100 (not prorated), then weeks from Mar 18 | BUDGET-007 |
| test/logic/budget/budget_calculator_test.dart | spans / week to month on the 20th: earlier weeks... absorbed, straddling week truncated | week Feb 26 truncated to Mar 1 partial, then month spans from Mar 1 | BUDGET-007 |
| test/logic/budget/budget_calculator_test.dart | spans / periods never overlap or leave a gap across several changes | contiguous spans across year->month->week->month, 3 partials | BUDGET-007 (never overlap) |
| test/logic/budget/budget_calculator_test.dart | spans / a change that starts before the old revision started drops it | new month revision whose start precedes week revision replaces it | BUDGET-007 (edge, undocumented) |
| test/logic/budget/budget_calculator_test.dart | spans / week spans keep calendar days over DST changes | all week spans start Monday 00:00, contiguous | BUDGET-001 |
| test/logic/budget/budget_calculator_test.dart | results / spent is the sum of absolute expense amounts in [start, end) | March spent 1000 (2 entries), April 1 | BUDGET-002 |
| test/logic/budget/budget_calculator_test.dart | results / an entry exactly at start counts, exactly at end belongs to the next span | start inclusive, end exclusive | BUDGET-002 |
| test/logic/budget/budget_calculator_test.dart | results / income, zero and entries outside every span are not spent | positive/zero/out-of-range amounts give spent 0 | BUDGET-002 |
| test/logic/budget/budget_calculator_test.dart | results / no spans or no entries | empty cases | BUDGET-002 (edge) |
| test/logic/budget/budget_calculator_test.dart | results / the input order of spans and entries does not matter | results sorted by span start | none |
| test/logic/budget/budget_calculator_test.dart | results / property: the sum over spans equals the total in range and no entry is counted twice (200 seeds) | contiguity, total spent == in-range abs negatives, each entry in exactly one span | BUDGET-007 ("every entry belongs to exactly one period", never overlap), BUDGET-002 |
| test/logic/budget/budget_service_test.dart | lifecycle / creating a budget marks the category tracked and stores the first revision at the start of the current period | after create: category.isBudgetTracked true, one revision effectiveFrom = Mar 1, target 50000, period 'month' | BUDGET-005 (create clause), BUDGET-007 (revision at period start) |
| test/logic/budget/budget_service_test.dart | lifecycle / deleting a budget removes its revisions and clears the flag | deleted budget gone, its revisions gone, flag false; other budget/flag untouched | BUDGET-005 (delete-budget clause) |
| test/logic/budget/budget_service_test.dart | lifecycle / deleting a tracked category deletes its budget in the same transaction | failing category remove rolls back budget/revision/flag; successful delete leaves 0 budgets, 0 revisions, category gone | BUDGET-005 (delete-category clause) |
| test/logic/budget/budget_service_test.dart | spent is computed from the entries / entries are counted by category and date, linked or not, with boundaries in the later period | spent [300, 400] for Mar/Apr with start-inclusive, end-exclusive; Feb entry ignored | BUDGET-002 |
| test/logic/budget/budget_service_test.dart | spent is computed from the entries / deleting an entry lowers its period, never raises it | deleting a past entry lowers that period's spent from 350 to 100, other period stays 40 | BUDGET-002, ENTRY-005 (derived on read) |
| test/logic/budget/budget_service_test.dart | spent is computed from the entries / other categories and income do not count | spent == only the budget category's expense (100), not rent or salary | BUDGET-002 |
| test/logic/budget/budget_service_test.dart | periods / a budget not opened for 14 months shows every missed period | 15 periods from Jan 2023, current is Mar 2024 | BUDGET-003 |
| test/logic/budget/budget_service_test.dart | periods / changing the target keeps the past target and replaces a revision of the same period | 2 revisions; second same-period change replaces; targets [100,100,200]; onMainPage updated | BUDGET-004, BUDGET-007 (target-only clause) |
| test/logic/budget/budget_service_test.dart | periods / month to week on Wednesday the 20th: partial month, then weeks | partial span Mar1-Mar18 target 100 (full old), spent 1010, last week from Mar 18 spent 1, total 1011 | BUDGET-007 |
| test/logic/budget/budget_service_test.dart | periods / week to month on the 20th: earlier weeks... absorbed and the old revision rows that began inside it are replaced | revisions collapse to [Feb 5 week, Mar 1 month]; one partial ending Mar 1; March spent 8; total 15 | BUDGET-007 |
| test/logic/budget/budget_service_test.dart | archiving / an archived budget accrues no new periods | archived Mar 10: isArchived, 3 periods, last March, even at Dec | BUDGET-006 |
| test/logic/budget/budget_service_test.dart | archiving / reading summaries, however often and however late, writes nothing | repo write counter 0 and budget/revision snapshot identical after repeated summaries/summaryOf 2+ years later | BUDGET-008 |
| test/logic/budget/budget_service_test.dart | watchSummaries emits again when an entry or the budget changes | summary stream spent 0 then 70 | BUDGET-002 (reactive) |
| test/db/budget_repository_test.dart | findByCategory returns the budget tracking the category | lookup returns budget | none |
| test/db/budget_repository_test.dart | findByCategory returns null when nothing tracks the category | null lookup | none |
| test/db/budget_repository_test.dart | revisions are returned for their budget, oldest first, and keep their period by name | findRevisions filtered by budget, sorted by effectiveFrom, period round-trips; removeRevision | none (BUDGET-004 storage support) |
| test/logic/budget/remaining_time_test.dart | 12 tests (no time left...; under an hour...; one hour; hours inside the last day; one day; a day and some hours...; 29 days...; 30 days is one month...; 61 days...; 90 days...; an ended period gives negative numbers...; the injected clock is not needed...) | remainingUntil(end, now) breakdown into months(30d)/days/hours | none (display arithmetic, undocumented) |
| test/routes/pages/budget/cubit/budget_form_cubit_test.dart | 9 tests (offers the untracked expense categories...; needs a category and a parseable target; submit creates...; incomplete form fails...; failing create; edit starts from current period; can only be saved once something changed; unparseable target; submit changes the details; delete removes...; nothing can be deleted while creating) | form state and the mocked BudgetService calls (create/changeDetails/delete args); create offers only untracked expense categories | none (BUDGET-005 lives in the service; "untracked categories only" is an undocumented UI consequence) |
| test/routes/pages/budget/cubit/budget_overview_cubit_test.dart | 7 tests (shows the newest period with its entries; selectPeriod...; keeps the selected period...; a selection that no longer exists...; missing budget...; failing stream; closing) | overview queries entries of the budget category from span start to end-1ms (Jun 1..Jun 30 23:59:59.999) | BUDGET-002 (UI query mirrors [start,end) via inclusive filter) — weak, through mocks |
| test/routes/pages/budget/cubit/budgets_cubit_test.dart | 7 tests (groups by period type; sorts a group by progress...; archived budgets leave the period groups...; a budget without periods counts as monthly; no budgets; failing stream; closing) | list grouping/sorting of mocked summaries; archived listed separately by name | none (display ordering) |

## Categories (lib/logic/category, category repo, models/category)

| File | Test | What the assertions pin | Rules matched |
|---|---|---|---|
| test/logic/category/category_service_test.dart | deleting a category cascades to the SMS data / removes the rules and clears the pending entries | after deleteCategory(food): only 'cinema' rule left; pending of food -> category 0, others untouched; category gone | SMS-005 |
| test/logic/category/category_service_test.dart | deleting a category cascades... / a failure clearing the pending entries rolls everything back | clearPendingCategory throw -> category, all 3 rules and pending links unchanged | SMS-005 (same transaction) |
| test/logic/category/category_service_test.dart | deleting a category cascades... / a failure deleting the category rolls the cascade back | category remove throw -> rules and pending links unchanged | SMS-005 (same transaction) |
| test/logic/category/category_service_test.dart | Convert Color to String and back to Color / Properly converts static Color values back and forth | 9 Material colors round-trip ARGB through colorToString/stringToColor | CAT-002 |
| test/logic/category/category_service_test.dart | ... / Properly converts randomly generated Colors back and forth | 100 random opaque colors round-trip | CAT-002 |
| test/logic/category/category_service_test.dart | ... / Opaque colors keep the stored ffrrggbb form | colorToString yields 8-hex 'ffrrggbb' | CAT-002 (format detail) |
| test/logic/category/category_service_test.dart | ... / Keeps the alpha channel, including values below 0x10 | colors with alpha 0x00..0xff round-trip | CAT-002 |
| test/db/category_repository_test.dart | category names are unique / adding a second category with the same name is rejected | add duplicate (other type) throws DuplicateCategoryName('Food'); still 1 row | CAT-001 |
| test/db/category_repository_test.dart | category names are unique / renaming a category to a taken name is rejected | update to taken name throws; stored name stays 'Fun' | CAT-001 |
| test/db/category_repository_test.dart | category names are unique / names differing only by case are distinct (exact match) | 'Food' and 'food' both stored | CAT-001 (case-sensitive clause) |
| test/db/category_repository_test.dart | find / filters by name substring and type, newest first | find ordering newest-first; type filter; case-insensitive substring | none |
| test/db/category_repository_test.dart | find / untracked expense categories exclude tracked and income | findUntrackedExpense returns only untracked expense | none (supports BUDGET-005 indirectly) |
| test/routes/pages/category/cubit/categories_cubit_test.dart | 7 tests (starts loading...; lists the categories, the most used first; switching the type...; selecting the type already shown...; toggle...; initial selection copied...; failing stream; closing) | category picker list state over mocked stream | none |
| test/routes/pages/category/cubit/category_form_cubit_test.dart | create: starts empty; needs a name and a color...; submit creates a trimmed category...; a taken name is reported as a duplicate (CAT-001); incomplete form fails | form state; mocked DuplicateCategoryName -> duplicateName failure; name trimmed | CAT-001 (UI reporting only, via mock; does not enforce) |
| test/routes/pages/category/cubit/category_form_cubit_test.dart | edit: starts from the category; its type cannot change; can only be saved once name or color changed; knows whether the category is used; submit updates in place; a duplicate name leaves the category as it was; delete removes an unused category; a used category cannot be deleted | edit form; the in-memory category object keeps old name on mocked duplicate; used category delete is a no-op | none enforced (CAT-001 reporting via mock; "type cannot change", "used category cannot be deleted" undocumented) |
| test/routes/pages/category/cubit/category_search_bloc_test.dart | 9 tests (initial query; debounced query; re-subscribe; loading; ...; failing; type switch; closing) | search bloc query/type plumbing over mocks | none |

## Data transfer (lib/db/transfer)

| File | Test | What the assertions pin | Rules matched |
|---|---|---|---|
| test/db/transfer/round_trip_test.dart | export, import into an empty store, export again: the files are equal, and balances and budget summaries match the source | re-export of the imported seed (every kind: entries, categories, groups, accounts, transactions, transfer, budgets w/ revisions, templates, rules, non-default settings) is byte-equal; observe() (balances, periods, flags, templates, rules, default account) equal | XFER-001 |
| test/db/transfer/round_trip_test.dart | the numbers the app shows survive: ... | Cash balance, a partial period, archivedAt, tracked flags survive | XFER-001, XFER-005 (derived values recomputed from stored facts) |
| test/db/transfer/round_trip_test.dart | relations are intact: entries keep their category and account, transfers their two sides, groups their categories | names via remapped ids for entry category/account, transfer from/to, group categories | XFER-001 (relations intact) |
| test/db/transfer/round_trip_test.dart | SMS templates (with their account, in order) and category rules (with their category) survive | template names/order/account/keywords/dateFormat/currencyRegex/enabled; rule keyword->category; modes prefix/word | XFER-001 |
| test/db/transfer/round_trip_test.dart | a file whose templates have no currency regex imports them empty | v3 file w/o currencyRegex -> all templates currencyRegex '' | XFER-004 (v1-3 template upgrade) |
| test/db/transfer/round_trip_test.dart | a file whose rules have no mode imports them as contains | v2 file w/o mode -> all rules 'contains' | XFER-004 (v1-2 rule upgrade), SMS-003 (default mode) |
| test/db/transfer/round_trip_test.dart | import replaces what was there and remaps ids | importing into a populated store: observe equal to source, target-only category gone, re-export equal | XFER-001 ("ids may change"), XFER-002-adjacent (replace) |
| test/db/transfer/round_trip_test.dart | a larger generated dataset round-trips with equal balances | 3000 entries, 5 budgets: balances map equal, counts equal | XFER-001 |
| test/db/transfer/round_trip_test.dart | exactly one settings row, at the current schema version | after import: 1 Settings row, schemaVersion == current | SET-001 (after import) |
| test/db/transfer/data_transfer_service_test.dart | the backup is taken before any row changes, then the import runs | backup called once, store snapshot at backup time == before; store changed after; returns backup path | XFER-003 (backup-before-replace clause) |
| test/db/transfer/data_transfer_service_test.dart | if the backup fails nothing is imported | backup throws -> BackupFailedException, snapshot unchanged | XFER-003 |
| test/db/transfer/data_transfer_service_test.dart | exportJson is the exporter output and parse reads it back | export parses back to 6 entries; export takes no backup | none |
| test/db/transfer/import_atomic_test.dart | $key is rejected before anything is written (27 generated invalid files: bad JSON, array, other format, missing field/list, wrong types, bad/local dates, dup category name, dup account name, dup id, dangling ids, two budgets on one category, unknown period/currency/theme, version newer than supported (5), v2 without templates/rules, dangling template account/rule category, rule keywords differing only in case, dup template id) | importer.run throws TransferFormatException and snapshotAll unchanged | XFER-002 (validated before any write), XFER-004 (newer version rejected), CAT-001 / SMS-003 (uniqueness validated in file) |
| test/db/transfer/import_atomic_test.dart | a failure part-way through the replace leaves every box as it was | injected throw at 2nd revision write -> StateError, snapshot unchanged, 1 Settings row | XFER-002 (one transaction) |
| test/db/transfer/import_atomic_test.dart | a failure while writing SMS rows leaves every box as it was, pending entries and scan point included | throw at 2nd rule write after templates -> snapshot unchanged, pending still there | XFER-002 |
| test/db/transfer/import_atomic_test.dart | import removes every pending entry whatever its status and keeps the SMS scan point | 3 pending (all statuses) and 2 unrecognized -> 0 and 0; isKnown false; lastSmsScanAt kept | XFER-006 |
| test/db/transfer/import_atomic_test.dart | a version 1 file also clears pending entries and keeps the scan point | v1 import: pending 0, scan point kept | XFER-006 |
| test/db/transfer/import_atomic_test.dart | the file replaces templates and rules, mapping account and category ids | v2 import: templates/rules replaced, account/category ids remapped | XFER-001 (relations), XFER-004 |
| test/db/transfer/import_atomic_test.dart | the importer needs a parsed document: validation cannot be skipped | hand-built document with dangling category -> TransferFormatException, snapshot unchanged | XFER-002 |
| test/db/transfer/import_atomic_test.dart | a valid file replaces everything, nothing of the old data stays | after v1 import: only file categories/accounts/entries/budget/revisions/group; templates and rules empty | XFER-002 (replace), XFER-004 (v1 has no templates/rules) |
| test/db/transfer/import_atomic_test.dart | an empty-data file yields an empty store with one settings row (SET-001) | empty file -> empty boxes, 1 settings row, current schema version, defaultAccountId -1, currency UAH | SET-001, ACCT-002 (-1 when none), XFER-004 (currency switched to file's) |
| test/db/transfer/import_atomic_test.dart | a store with no settings row gets exactly one | import into store without Settings -> exactly 1 | SET-001 |
| test/db/transfer/import_atomic_test.dart | BudgetRepository is replaced as a whole, revisions included | removeAll clears budgets and revisions | none |
| test/db/transfer/format_version_test.dart | format version 4 is the current one | formatVersion const 4; v1 file serializes as 4 | XFER-004 |
| test/db/transfer/format_version_test.dart | a file newer than the app supports is rejected, naming the version | versions 5 and 104 -> TransferFormatException naming the version | XFER-004 |
| test/db/transfer/format_version_test.dart | a missing, non-integer or non-positive version is rejected | null/'1'/1.5/0/-1/absent -> TransferFormatException | XFER-004 ("every file carries a format version") |
| test/db/transfer/format_version_test.dart | the checked-in version 1 fixture parses, field by field | v1 fixture fields parse (currency, settings, categories, accounts opening, entries minor, transfers, revisions) | XFER-004 (older format read), XFER-005 (opening balances, minor units) |
| test/db/transfer/format_version_test.dart | a version 1 file reads as having no SMS templates or rules | v1 -> empty templates and rules | XFER-004 |
| test/db/transfer/format_version_test.dart | parse then toJson reproduces the fixture, upgraded to version 4 | codec round-trip of v1 equals fixture + version 4 + empty lists | XFER-004 |
| test/db/transfer/format_version_test.dart | version 2 / templates and rules parse field by field | v2 template/rule fields | XFER-004 |
| test/db/transfer/format_version_test.dart | version 2 / a rule without a mode reads as contains, an unknown mode is rejected | modes ['word','contains']; 'regex' rejected | XFER-004, SMS-003 (default contains) |
| test/db/transfer/format_version_test.dart | version 2 / parse then toJson reproduces the document, upgraded | v2 -> v4 with currencyRegex '' and mode contains added | XFER-004 |
| test/db/transfer/format_version_test.dart | version 2 / a template without a currency regex reads as empty | v2 currencyRegex ''; v4 keeps given regex | XFER-004 |
| test/db/transfer/format_version_test.dart | version 2 / regular expressions are stored as written, valid or not | invalid regexes kept verbatim | none (SMS-002 tolerance downstream) |
| test/db/transfer/format_version_test.dart | version 2 / both new lists are required | v2 without either list rejected naming key | XFER-004/XFER-002 (validation) |
| test/db/transfer/format_version_test.dart | version 2 / a version 5 file is rejected as too new | tooNew flag true | XFER-004 |
| test/db/transfer/format_version_test.dart | version 2 / a malformed template or rule names its path | error reason names the JSON path | XFER-002 (validation) |
| test/db/transfer/format_version_test.dart | version 2 / a dangling template account or rule category is rejected | dangling ids rejected at parse | XFER-002 (validation) |
| test/db/transfer/format_version_test.dart | version 2 / keywords that differ only in case are rejected | 'sosedi' + 'SOSEDI' rejected | SMS-003 (case-insensitive unique keywords, in file) |
| test/db/transfer/format_version_test.dart | importing the version 1 fixture / switches the currency, theme and default periods to the file's | after import currencyCode 'UAH' (was not), theme, periods, defaultAccountId = Card, exactly one default account | XFER-004 (currency switch), ACCT-002 |
| test/db/transfer/format_version_test.dart | importing the version 1 fixture / leaves no SMS templates or rules | v1 import -> no templates/rules | XFER-004 |
| test/db/transfer/format_version_test.dart | importing the version 1 fixture / stores what the file says and derives the rest | balances derived from opening+ops (7270, 499000), tracked flag derived from budget, usages 2 | XFER-005, ACCT-001 |
| test/db/transfer/exporter_test.dart | an empty store exports one settings block and no rows | empty lists, defaultAccountId null, first currency | none |
| test/db/transfer/exporter_test.dart | every kind is written with file-local ids and resolvable relations | exporter document fields per kind incl. openingBalanceMinor, revisions periods, archivedAt; round-trips through parser | XFER-001, XFER-005 (opening balances) |
| test/db/transfer/exporter_test.dart | templates and rules are numbered in store order, with file-local account and category ids | template/rule ids and relations | XFER-001 |
| test/db/transfer/exporter_test.dart | a template whose account was deleted is exported without one | dangling template account -> null | none (undocumented) |
| test/db/transfer/exporter_test.dart | a rule whose category no longer exists is left out, not an error | orphan rule dropped | none (undocumented) |
| test/db/transfer/exporter_test.dart | pending entries and the scan point are not in the file | export text has no pending/lastSmsScanAt/smsKey; schema ok | XFER-006 (neither exported), XFER-005 |
| test/db/transfer/exporter_test.dart | unrecognized SMS are not in the file | text has no unrecognized body | XFER-006 |
| test/db/transfer/exporter_test.dart | ids are normalized: gaps in the store ids do not show | ids renumbered 1..5 | XFER-001 ("ids may change") |
| test/db/transfer/exporter_test.dart | what the exporter writes has only allow-listed keys | checkSchema: allow-listed keys, forbidden derived/legacy keys absent, *Minor ints, no doubles; entry shape | XFER-005, MONEY-001 |
| test/db/transfer/exporter_test.dart | a store row that points at a missing category is not exported | exporter throws TransferFormatException | none |
| test/db/transfer/export_schema_test.dart | a document written by the codec has only allow-listed keys | codec output passes checkSchema | XFER-005 |
| test/db/transfer/export_schema_test.dart | the checker itself rejects a derived key and a stray key | checkSchema fails on balanceMinor | XFER-005 (meta) |
| test/routes/pages/settings/cubit/data_transfer_cubit_test.dart | export / writes the export under budgetron-<local date>.json | saveJson called with dated filename | none |
| test/routes/pages/settings/cubit/data_transfer_cubit_test.dart | export / a cancelled save dialog is not an error | idle | none |
| test/routes/pages/settings/cubit/data_transfer_cubit_test.dart | export / a failing export or save is a failure key | exportFailed | none |
| test/routes/pages/settings/cubit/data_transfer_cubit_test.dart | import / picking a file only reviews it: the importer is not called until the user confirms | Review state; importDocument never called | XFER-003 (explicit confirm) |
| test/routes/pages/settings/cubit/data_transfer_cubit_test.dart | import / declining the review never imports | cancel -> Idle, importDocument never | XFER-003 |
| test/routes/pages/settings/cubit/data_transfer_cubit_test.dart | import / confirming without a reviewed file does nothing | no emit, no import | XFER-003 |
| test/routes/pages/settings/cubit/data_transfer_cubit_test.dart | import / confirming imports the reviewed document once | importDocument called exactly once despite double confirm | XFER-003 |
| test/routes/pages/settings/cubit/data_transfer_cubit_test.dart | import / a cancelled file picker returns to idle; an unreadable file...; an invalid file... a newer one says so; a malformed file...; a failed backup and a failed replace are told apart | failure keys per error | XFER-004 (newer reported) weakly; none |

## Entries (lib/logic/entry, entry repo, models/entry.dart)

| File | Test | What the assertions pin | Rules matched |
|---|---|---|---|
| test/logic/entry/entry_service_test.dart | calculateTotalValue / is zero for no entries | empty -> 0 | none |
| test/logic/entry/entry_service_test.dart | calculateTotalValue / sums exactly where doubles drift | 10010+20020 == 30030 exactly | MONEY-001 (exact sums) |
| test/logic/entry/entry_service_test.dart | calculateTotalValue / is the absolute value of the signed sum | -500 + -250 -> 750 | none |
| test/logic/entry/entry_service_atomic_test.dart | entry writes are atomic / a failing entry write rolls back the usages of createEntry | put throws -> 0 entries, stored usages still 3 | ENTRY-005 (create atomic) |
| test/logic/entry/entry_service_atomic_test.dart | ... / a failing createEntry restores the in-memory state | in-memory usages, amount and id restored | ENTRY-005 |
| test/logic/entry/entry_service_atomic_test.dart | ... / createEntry stores the entry and the usages together | 1 entry, usages 4 | ENTRY-005 |
| test/logic/entry/entry_service_atomic_test.dart | ... / a failing removal rolls back deleteEntry | remove throws -> entry kept, usages 4 stored and in memory | ENTRY-005 (delete atomic) |
| test/logic/entry/entry_service_atomic_test.dart | ... / deleteEntry removes the entry and reverts the usages | entry gone, usages back to 3 | ENTRY-005 (delete decrements) |
| test/logic/entry/entry_service_atomic_test.dart | ... / deleteEntry never takes usages below zero | usages 0 stays 0 | ENTRY-005 (never below 0) |
| test/logic/entry/entry_service_update_test.dart | sign convention / an expense is stored negative, income positive, whichever sign the caller passes | food +1250 -> -1250, -300 -> -300, salary 5000 -> 5000, -700 -> 700 | ENTRY-001 |
| test/logic/entry/entry_service_update_test.dart | sign convention / an update stores the sign of the selected category | update to salary -> +900, to rent -> -450 | ENTRY-001 |
| test/logic/entry/entry_service_update_test.dart | update in place / keeps the id and changes value, date, category and account | 1 row, same id, new amount/date/category/account | ENTRY-004 |
| test/logic/entry/entry_service_update_test.dart | update in place / the category selected in the UI is honored and usages move with it | usages (1,0)->(0,1); same category again keeps (0,1) | ENTRY-005 (edit moves one usage) |
| test/logic/entry/entry_service_update_test.dart | update in place / an account can be removed | update without account clears it | ENTRY-004 |
| test/logic/entry/entry_service_update_test.dart | update in place / balances and budget periods follow the edit, nothing else to maintain | balances 9000->10000/-400 and budget spent 1000->400 after edit | ENTRY-005 (derived on read), ACCT-001, BUDGET-002 |
| test/logic/entry/entry_service_update_test.dart | update in place / a failing write rolls back the usages as well | put throws on update -> usages (1,0), amount and category unchanged | ENTRY-005 (edit atomic) |
| test/logic/entry/entry_service_update_test.dart | update in place / an unknown id is an error and writes nothing | StateError, usages 0 | ENTRY-004 (edge) |
| test/logic/entry/entry_grouping_test.dart | on the fixed fixture / entries by day/month/year equal the former output; a week groups by day; operations by day/month/year equal the former output (7 tests) | digest of grouping equals recorded legacy digests | none (refactor parity) |
| test/logic/entry/entry_grouping_test.dart | on the fixed fixture / every entry lands in exactly one group and the totals add up | each entry in one group; group/category total == signed sum; groups newest first | ENTRY-007 (signed total) weakly |
| test/logic/entry/entry_grouping_test.dart | boundaries / midnight starts a new day, the last millisecond ends one | day boundaries; group total -300 | none (grouping boundaries, undocumented) |
| test/logic/entry/entry_grouping_test.dart | boundaries / categories keep the order of their first entry | categories in order of newest entry (input is newest-first) | ENTRY-006 ("recent" clause, at grouping level) |
| test/logic/entry/entry_grouping_test.dart | boundaries / months and years | month/year grouping and totals | none |
| test/logic/entry/entry_grouping_test.dart | boundaries / income and expense are split by sign and add up to the total | group income 1050, expense -570, sum == total; per-category income/expense and sum == total | ENTRY-007 |
| test/logic/entry/entry_grouping_test.dart | boundaries / no entries give no groups | empty | none |
| test/logic/entry/entry_grouping_test.dart | boundaries / an entry of an unknown category throws a StateError | throws | none |
| test/logic/entry/entry_grouping_test.dart | boundaries / operations: other items are newest first, and a period of only those still gets a group | transactions grouped; income/expense of others | ENTRY-007 (operations variant) |
| test/logic/entry/entry_grouping_test.dart | sortGroups / amount sorts list income before expenses, ties keep input order, recent keeps the input order | recent = input order; amountDesc = income by |total| desc then expense desc; amountAsc = income asc then expense asc; totals preserved | ENTRY-006 |
| test/db/entry_repository_test.dart | find combines filters with AND / period and ids; period spanning two days and ids...; period and ids with no overlap; period and category; period and account; ids, category and account; expense flag and account; categories and a range...; an empty category set matches nothing (9 tests) | result is intersection of filters | ENTRY-003 |
| test/db/entry_repository_test.dart | find date range is inclusive on both ends / entries exactly on either bound are returned | from==day1, to==day3 includes both | ENTRY-002 |
| test/db/entry_repository_test.dart | find date range is inclusive on both ends / entries one millisecond outside are excluded | +-1ms excludes | ENTRY-002 |
| test/db/entry_repository_test.dart | existsWithAmountBetween / matches the absolute amount of an expense and of an income; the range is inclusive on both ends | abs amount match; inclusive range | ENTRY-002 (different query), SMS-related dedupe (undocumented) |
| test/db/entry_repository_test.dart | earliestDateTime / is the oldest entry overall; with categories only counts their entries | earliest dates | none |
| test/db/entry_repository_test.dart | find / returns newest first and honours the limit; watch emits the current rows and honours the limit | ordering/limit | none |
| test/routes/pages/entry/cubit/entries_cubit_test.dart | starts loading the default period of the settings, for today | query from 00:00 to 23:59:59.999 of today; year default honored | ENTRY-002 (UI builds inclusive range) weakly |
| test/routes/pages/entry/cubit/entries_cubit_test.dart | emits the entries...; groups the entries of the day by category with their totals | grouped state; salary before food | ENTRY-006 (default sort) weakly |
| test/routes/pages/entry/cubit/entries_cubit_test.dart | sorting / the sort of the settings is applied, income first by default | default sort amountDesc; salary before food | ENTRY-006 ("the default" clause) |
| test/routes/pages/entry/cubit/entries_cubit_test.dart | sorting / starts with the sort of the settings | recent -> food, salary | ENTRY-006 (recent) |
| test/routes/pages/entry/cubit/entries_cubit_test.dart | sorting / setSort re-emits without querying, recent restores the order | sort changes re-emit, one query | ENTRY-006 (recent) weakly |
| test/routes/pages/entry/cubit/entries_cubit_test.dart | 10 other tests (unknown category failure; new emission replaces; setPeriod; shift back; cancelled subscription ignored; cannot step after; before earliest; without entries; failing stream; only current subscription active; closing) | navigation/subscription plumbing | none |
| test/routes/pages/entry/cubit/entry_form_cubit_test.dart | create group (15 tests: starts as an expense of 0...; uses the default account...; default account no longer exists...; category picked first...; income category picked first...; amount keeps the digits...; keyboard builds amount; other type clears category; same type keeps; type follows category; date/time/account; cannot submit before...; submit creates...; nothing to save fails; failing save; after a failure editable) | form state; createEntry called with entry 150 minor + category (sign applied by service) | none (ENTRY-001 sign is applied in service, not here) |
| test/routes/pages/entry/cubit/entry_form_cubit_test.dart | edit group (11 tests: starts from the entry; income starts as income; nothing submittable until changed; changed fields dirty; seconds don't count; zero/mid-expression not submittable; type of existing entry cannot be switched; submit updates in place and keeps the id; delete; failing delete; nothing deleted while creating) | service.update called with id 7, never createEntry | ENTRY-004 (UI side: update not create) |
| test/routes/pages/entry/cubit/entry_form_cubit_test.dart | from a pending entry / is pre-filled from the pending entry, not from the clock | form from pending: ruleKeyword defaults to the whole merchant 'SHOP 1', remember off | SMS-004 ("the form offers the whole merchant") |
| test/routes/pages/entry/cubit/entry_form_cubit_test.dart | from a pending entry / an income pending entry starts as income; a category of the other type is not pre-filled; without a category or account...; type can be switched...; only a merchant shows the remember option; toggleRememberCategory flips | form prefill logic | none (SMS-004-adjacent UI) |
| test/routes/pages/entry/cubit/entry_form_cubit_test.dart | from a pending entry / an amount in another currency is marked on the form | foreignCurrencyCode 'EUR' when pending code differs from app USD, null when equal or empty | SMS-007 ("marked") — on the confirm form, not the list |
| test/routes/pages/entry/cubit/entry_form_cubit_test.dart | from a pending entry / submit confirms the pending entry with the form values; submit passes the remember flag when it is on; submit passes the keyword the user typed; a blank keyword saves no rule; the remember flag is not passed without a merchant; failing confirm | sms.confirm(id, update, ruleKeyword) args: null when off/blank('  ')/no merchant; typed keyword passed | SMS-004 (blank-after-trim -> no rule, at form level) |

## Money (models/enums/currency.dart, models/money.dart, models/money_format.dart)

| File | Test | What the assertions pin | Rules matched |
|---|---|---|---|
| test/models/money_test.dart | arithmetic / adds and subtracts exactly where doubles drift | Money(10010)+Money(20020) == 30030 and back | MONEY-001 |
| test/models/money_test.dart | arithmetic / negation, abs and sign; equality, hashCode and ordering | value semantics | none |
| test/models/money_test.dart | parse (5 tests: parses whole/fractional/negative; rejects malformed & too many digits; roundExcess half away from zero; tryParseInput; beyond int range) | text -> minor units parsing | MONEY-001 (minor units) weakly |
| test/models/money_test.dart | times (3 tests) | decimal multiply without doubles, half-away rounding | MONEY-001 (no doubles) weakly |
| test/models/money_format_test.dart | Currency / minor units, byIndex and byCode | all currencies 2 minor units; lookups | none |
| test/models/money_format_test.dart | formatMoney rounds only the text, for digits 0, 2 and 3 | formatted strings per digits/wholeUnits; source Money unchanged (minor 12345) | MONEY-002 |
| test/models/money_format_test.dart | moneyRatio and formatPercent | ratio/percent text | none |
| test/models/money_format_test.dart | formatMoney groups the whole part only for a given locale | locale grouping | none (display) |
| test/models/legacy_fields_test.dart | no code outside the migration touches a retired counter | no lib file outside lib/db/migrations, lib/models, objectbox.g reads currentMinor/balanceMinor/startDate/resetDate/budgetPeriodIndex/BudgetHistory | MONEY-001 (legacy only read by migrations) partially; XFER-005 (retired) |
| test/models/legacy_fields_test.dart | no code outside the migration reads a legacy double or index field | same scan for currencyId/themeModeIndex/targetValue/currentValue/endValue/currencyIndex (Entry/Transaction/Transfer `value` and Account `balance` left to the analyzer) | MONEY-001 ("only lib/db/migrations reads them") |

## Settings (cubit/settings_cubit.dart, settings repo, models/settings.dart, db/migrations)

| File | Test | What the assertions pin | Rules matched |
|---|---|---|---|
| test/db/settings_repository_test.dart | ensure leaves exactly one settings row / an empty store gets one row | ensure on empty store -> 1 row, find returns it | SET-001 |
| test/db/settings_repository_test.dart | ensure leaves exactly one settings row / three rows are reduced to the lowest id | 3 rows -> 1, lowest id kept with its values | SET-001 |
| test/db/settings_repository_test.dart | ensure leaves exactly one settings row / calling it twice leaves one row with the same id | idempotent ensure | SET-001 |
| test/db/settings_repository_test.dart | ensure leaves exactly one settings row / an existing row is kept untouched | values kept | SET-001 |
| test/db/settings_repository_test.dart | find is null before ensure | no row -> null | none |
| test/db/migrations/migrator_test.dart | versioning / the version is stamped on a legacy row and on a fresh install | schemaVersion 0 -> currentSchemaVersion after run | SET-002 (versioned) |
| test/db/migrations/migrator_test.dart | versioning / running twice changes nothing the second time | step runs once; second run from 2 to 2; values equal | SET-002 (idempotent) |
| test/db/migrations/migrator_test.dart | versioning / a step that throws leaves the store and the version untouched | throw -> 1 settings row, version 0, currency UAH (extra row put rolled back) | SET-002 (one transaction) |
| test/db/migrations/migrator_test.dart | versioning / all steps and the version bump share one transaction | step 2 throws -> version stays 0 | SET-002 (one transaction) |
| test/db/migrations/migrator_test.dart | versioning / an unversioned row and a version 1 row both start at step 1 | step mapping | SET-002 (versioned) |
| test/db/migrations/migrator_test.dart | versioning / a missing step or a newer store version is a fatal error | StateError; version 99 untouched | SET-002 |
| test/db/migrations/migrator_test.dart | versioning / without a settings row the migrator refuses to run | StateError | SET-001-adjacent |
| test/db/unit_of_work_test.dart | run commits all writes...; an exception rolls back every write...; a nested run joins the outer transaction | UnitOfWork commit/rollback/nesting | none directly (infrastructure behind ENTRY-005, XFER-002, ACCT-004, SET-002) |
| test/db/test_store_smoke_test.dart | $name test sees only its own store (2) | test isolation | none |
| test/db/migrations/backup_test.dart | the backup path sits next to data.mdb and names the schema version this build writes | backupPathFor -> data.mdb.pre-v3 | none (migration backup undocumented in rules) |
| test/db/migrations/backup_test.dart | a v2 backup from the previous build is left alone...; an explicit suffix keeps the old pre-v2 behavior; copies data.mdb once; does nothing when there is no database yet; never overwrites an existing backup; a leftover partial copy is replaced | pre-migration backup file behaviour | none |
| test/db/migrations/backup_test.dart | pre-import backup of an open store / the suffix carries the UTC time to the millisecond | preImportSuffix -> '.pre-import-20261006-070509-042', zone-independent | XFER-003 (`data.mdb.pre-import-<timestamp>` name) |
| test/db/migrations/backup_test.dart | pre-import backup of an open store / copies the live data.mdb to data.mdb.pre-import-<timestamp> once, and the copy opens with exactly the rows the store had | backupOpenStore makes the named copy once; copy restores to identical snapshot | XFER-003 |
| test/db/migrations/backup_test.dart | pre-import backup of an open store / a failure to copy throws instead of reporting a backup | FileSystemException | XFER-003 |
| test/db/migrations/v1_to_v2_test.dart | 14 tests (converts every legacy amount and the settings; legacy fields left exactly; relations survive; ten 0.10 sum to exactly 1.00; a second run leaves every field unchanged; the step alone is idempotent per row; a step failing after converting rows rolls everything back; a store already at v2 is untouched; 4 defensive settings mapping; fresh install same path; unconvertible values counted and left at 0) | v1->v2 conversion of doubles to *Minor ints, idempotent, atomic | SET-002 (idempotent, one transaction, versioned), MONEY-001 (minor units exact; legacy left in schema) |
| test/db/migrations/v2_to_v3_test.dart | 20 tests (step registered and build at v3; accounts: opening = stored balance - ops..., no operations, entries without account; budgets: coalesce, week->month revisions, week starts normalized, gap, no histories, counters disagree, archivedAt = min(resetDate, now), fallback, unknown period index; safety: second run no-op, step twice, legacy/counter fields never changed, failure after step 1, failure after step 2, store at v3 untouched, v1 store carried through both steps) | v2->v3 derives openings and revisions; idempotent; atomic | SET-002, ACCT-001 (opening derived so computed balance == stored), BUDGET-004/007 (revisions), MONEY-001 (legacy kept) |
| test/cubit/settings_cubit_test.dart | the stored default period ints are the enum indexes | enum index stability | none |
| test/cubit/settings_cubit_test.dart | over mocked repositories (11 tests: loads every stored field; unknown stored value falls back; setThemeMode/setCurrency/setDefaultAccountId/default periods/shortcut switches/entries sort persist; entries sort defaults to largest first...; follows a write by someone else; a no-op write emits nothing; closing) | cubit persists fields; `setDefaultAccountId(7)` writes 7 with no account involved; entries sort default amountDesc | ENTRY-006 ("amount, descending (the default)"); note: setDefaultAccountId bypasses ACCT-002 (no isDefault sync) but lib has no caller |
| test/cubit/settings_cubit_test.dart | over an in-memory store (3 tests: currency, theme and default account survive a restart; shortcut switches survive; follows an external write) | persistence across cubit instances; defaultAccountId 3 stored with no accounts | none (see ACCT-002 note) |

## SMS import (lib/logic/sms, sms repo, models/sms)

| File | Test | What the assertions pin | Rules matched |
|---|---|---|---|
| test/logic/sms/sms_parser_test.dart | Preset card template / parses $merchant $amount (5 generated) | preset reads amount (minor), date, merchant, expense from real card SMS | SMS-002 |
| test/logic/sms/sms_parser_test.dart | Preset card template / detects an income keyword | 'Zachislenie' -> income, '1 250,00' -> 125000 | SMS-002 |
| test/logic/sms/sms_parser_test.dart | Preset card template / reads the short credit format | no date match -> receive time used; merchant '' | SMS-002 (receive-time fallback) |
| test/logic/sms/sms_parser_test.dart | Preset card template / reads the ATM cash format | ATM format parsed | SMS-002 |
| test/logic/sms/sms_parser_test.dart | Preset card template / regexes and keywords ignore case | uppercase regex/keyword match lowercase text | SMS-002 (case-insensitive) |
| test/logic/sms/sms_parser_test.dart | Preset card template / ignores a non-banking SMS | noKeyword error | SMS-002 |
| test/logic/sms/sms_parser_test.dart | Preset card template / the keyword that occurs first decides the type | first-occurring keyword wins | SMS-002 |
| test/logic/sms/sms_parser_test.dart | Template configuration / the sender filter is applied | substring, case-insensitive sender filter; senderMismatch | SMS-002 |
| test/logic/sms/sms_parser_test.dart | Template configuration / falls back to the receive time without a date regex | receive time | SMS-002 |
| test/logic/sms/sms_parser_test.dart | Template configuration / an amount regex without a group uses the whole match | whole match | none (undocumented) |
| test/logic/sms/sms_parser_test.dart | Template configuration / an invalid regex is reported instead of thrown | invalid amount/date/merchant regex -> error codes, no throw | SMS-002 (template error) |
| test/logic/sms/sms_parser_test.dart | Template configuration / a date that does not fit the format is reported | invalidDate | SMS-002 |
| test/logic/sms/sms_parser_test.dart | Template configuration / a missing amount is reported | amountNotFound | SMS-002 |
| test/logic/sms/sms_parser_test.dart | Template configuration / a zero amount is rejected | 0.00 / 0,00 -> invalidAmount | SMS-002 (above zero) |
| test/logic/sms/sms_parser_test.dart | Template configuration / an amount that rounds to zero is rejected | 0.004 at 2 digits rejected, at 3 digits 4 | SMS-002 (above zero, app minor units) |
| test/logic/sms/sms_parser_test.dart | Template configuration / the first matching template wins | parseWithTemplates picks first template that matches sender+keywords | SMS-002 (first template) |
| test/logic/sms/sms_parser_test.dart | Template configuration / no template matching gives null | null | SMS-002 |
| test/logic/sms/sms_parser_test.dart | Amount normalization (4 tests: separators; minor units of the currency; rounds excess; rejects non-positive) | calculateAmount to minor units | SMS-002 (minor units, above zero) |
| test/logic/sms/sms_parser_test.dart | Category rules / the longest matching keyword wins | longest keyword wins | SMS-003 |
| test/logic/sms/sms_parser_test.dart | Category rules / the category type must match the entry type | type filter | SMS-003 |
| test/logic/sms/sms_parser_test.dart | Category rules / matches the text when there is no merchant | no merchant -> text pass | SMS-003 |
| test/logic/sms/sms_parser_test.dart | Category rules / falls back to the whole text when no rule matches the merchant | text pass after merchant miss | SMS-003 |
| test/logic/sms/sms_parser_test.dart | Category rules / a merchant match wins over a longer keyword in the text | merchant pass precedes text pass | SMS-003 |
| test/logic/sms/sms_parser_test.dart | Category rules / the text pass keeps the type filter and the earlier-rule tie | tie -> earlier rule; type filter in text pass | SMS-003 |
| test/logic/sms/sms_parser_test.dart | Category rules / contains matches before, after and inside words | contains semantics | SMS-003 |
| test/logic/sms/sms_parser_test.dart | Category rules / word needs non-alphanumeric or text bounds on both sides | word bounds incl. Cyrillic letters | SMS-003 |
| test/logic/sms/sms_parser_test.dart | Category rules / word escapes regex characters of the keyword | literal keyword | SMS-003 (detail) |
| test/logic/sms/sms_parser_test.dart | Category rules / prefix matches only at the start of the trimmed text | prefix at start (leading spaces trimmed) | SMS-003 |
| test/logic/sms/sms_parser_test.dart | Category rules / modes compete by keyword length, the earlier rule wins a tie | cross-mode longest/tie | SMS-003 |
| test/logic/sms/sms_parser_test.dart | Category rules / the keyword is case-insensitive and blank keywords are skipped | case-insensitive match; blank rule skipped | SMS-003 |
| test/logic/sms/sms_parser_test.dart | currency code (4 tests: preset reads code uppercased; without regex unknown; non-matching regex unknown; invalid currency regex is a failure) | currency code extraction | SMS-007 (template may read code), XFER-004 (no regex reads no currency) |
| test/logic/sms/sms_parser_test.dart | looksLikeTransaction / accepts an amount followed by a three-letter code | '5.29 BYN', '1 250,00 BYN', '5.29BYN', '5,29 USD' true | SMS-008 |
| test/logic/sms/sms_parser_test.dart | looksLikeTransaction / rejects codes, plain text and amounts without a currency | 'Pay 5 BYN' false (integer amount), 'paid 5.50 for it' false, 'BYNN' false | SMS-008 — NOTE `expect(looksLikeTransaction('Pay 5 BYN'), isFalse);` requires two decimals, which the rule text does not say |
| test/logic/sms/sms_parser_test.dart | splitKeywords trims and drops empty items | keyword list parsing | none |
| test/logic/sms/sms_import_service_test.dart | scan / creates a pending entry per message the preset reads | 2 of 3 messages become pending with amount/type/status/date/key | SMS-002, SMS-001 |
| test/logic/sms/sms_import_service_test.dart | scan / stores the currency code the SMS states | currencyCode BYN/USD stored; isForeign | SMS-007 |
| test/logic/sms/sms_import_service_test.dart | scan / a template without a currency regex stores no code | '' and not foreign | SMS-007 |
| test/logic/sms/sms_import_service_test.dart | scan / adds the preset template when there is none | preset added once | none (undocumented) |
| test/logic/sms/sms_import_service_test.dart | scan / fills the category from the rules | category from rule | SMS-003 |
| test/logic/sms/sms_import_service_test.dart | scan / uses the template account, else the default account; no default account leaves the account empty; a default account that no longer exists is ignored | pending account resolution | none (undocumented) |
| test/logic/sms/sms_import_service_test.dart | scan / a failing write leaves nothing and does not move the point | throw -> no pending, lastScanAt null | SMS-001 (scan point) |
| test/logic/sms/sms_import_service_test.dart | SMS-001 one pending entry per message / rescanning the same inbox creates nothing | 2 then 0 | SMS-001 |
| test/logic/sms/sms_import_service_test.dart | SMS-001 ... / a dismissed or confirmed message is not recreated | dismissed/confirmed not recreated, statuses kept, 1 entry | SMS-001 |
| test/logic/sms/sms_import_service_test.dart | SMS-001 ... / a scan only reads messages after its start point | only NEW | SMS-001 |
| test/logic/sms/sms_import_service_test.dart | SMS-001 ... / the scan point is never moved back | lastScanAt monotone | SMS-001 |
| test/logic/sms/sms_import_service_test.dart | SMS-001 ... / a first scan without messages stores its start point; a message the templates cannot read still moves the point | scan point semantics | SMS-001 |
| test/logic/sms/sms_import_service_test.dart | SMS-002 template errors never abort a scan / a broken template is skipped and the others still import | broken template first -> both messages still imported by preset | SMS-002 — the message is NOT skipped: next template reads it |
| test/logic/sms/sms_import_service_test.dart | SMS-002 ... / a template with an unparseable date skips the message | `expect(await scan(), 1);` — the message is imported via the preset after the bad-date template fails | SMS-002 — assertion shows the template, not the message, is skipped |
| test/logic/sms/sms_import_service_test.dart | SMS-002 ... / the first enabled template wins and disabled ones are skipped | disabled 'Off' skipped, 'Plain' (lower id than preset) wins | SMS-002 (first enabled by id) |
| test/logic/sms/sms_import_service_test.dart | SMS-002 ... / a zero amount creates nothing; the amount is read in the currency's minor units | 0.00 -> 0; digits 3 -> 5290 | SMS-002 |
| test/logic/sms/sms_import_service_test.dart | SMS-003 category rules / a keyword in another case updates the existing rule | 1 rule, original keyword, new category | SMS-003 (unique case-insensitive; save updates) |
| test/logic/sms/sms_import_service_test.dart | SMS-003 ... / a rule is saved with its match mode, contains by default; a stored rule without a mode name reads as contains | default contains | SMS-003 |
| test/logic/sms/sms_import_service_test.dart | SMS-003 ... / saving with an id edits that rule; editing a rule to change only its case or keep its keyword is allowed | edit keyword/category/mode in place | SMS-003 (editable) |
| test/logic/sms/sms_import_service_test.dart | SMS-003 ... / an edit onto another rule's keyword is refused, writing nothing | DuplicateRuleKeywordError; stored category/mode unchanged; 2 rules | SMS-003 |
| test/logic/sms/sms_import_service_test.dart | SMS-003 ... / editing an unknown rule is a StateError; a keyword is trimmed and an empty one is ignored | edge cases | SMS-003 (edge) |
| test/logic/sms/sms_import_service_test.dart | SMS-003 ... / a rule matches the SMS text when the merchant has no match | text fallback at scan | SMS-003 |
| test/logic/sms/sms_import_service_test.dart | SMS-003 ... / saving a rule re-resolves every pending entry | manual category overwritten by rule result; wrong-type rule -> none | SMS-003 (re-apply on save; only from rules) |
| test/logic/sms/sms_import_service_test.dart | SMS-003 ... / editing a rule recategorizes the pending entries it matched | edit re-applies; possibly none | SMS-003 (re-apply on edit) |
| test/logic/sms/sms_import_service_test.dart | SMS-003 ... / removeRule stops matching later scans; removeRule clears the category of the entries only it matched | remove re-applies | SMS-003 (re-apply on remove) |
| test/logic/sms/sms_import_service_test.dart | SMS-003 ... / removeRule rolls back when re-applying fails | rule and category restored | SMS-003 (same transaction, remove only) |
| test/logic/sms/sms_import_service_test.dart | SMS-003 ... / dismissed and confirmed entries are not re-resolved | dismissed untouched | SMS-003 ("pending entries" only) |
| test/logic/sms/sms_import_service_test.dart | SMS-004 confirming / creates a signed entry and marks the pending one confirmed | entry -529, date, category, account, usages 1, status confirmed, no rule | SMS-004, ENTRY-001, ENTRY-005 |
| test/logic/sms/sms_import_service_test.dart | SMS-004 ... / an income category gives a positive entry; uses the values the user edited | sign, edited values | SMS-004, ENTRY-001 |
| test/logic/sms/sms_import_service_test.dart | SMS-004 ... / saves the merchant rule and applies it to the other entries | rule saved; other pending re-resolved | SMS-004, SMS-003 |
| test/logic/sms/sms_import_service_test.dart | SMS-004 ... / a blank or missing rule keyword saves no rule; the rule keyword is trimmed and its mode kept | '  ' -> no rule; trimmed | SMS-004 (not blank after trimming) |
| test/logic/sms/sms_import_service_test.dart | SMS-004 ... / an unknown or already confirmed entry is a StateError; a dismissed entry cannot be confirmed | guards | SMS-004 (edge) |
| test/logic/sms/sms_import_service_test.dart | SMS-004 ... / a failure after the entry write rolls everything back; a failing rule write rolls back the entry and the status | one transaction | SMS-004 |
| test/logic/sms/sms_import_service_test.dart | SMS-004 ... / confirm all confirms the categorized entries of their own type | 3 confirmed, no-category and wrong-type stay pending | SMS-004 (confirm-all type filter) |
| test/logic/sms/sms_import_service_test.dart | SMS-004 ... / confirm all is all or nothing | throw -> 0 entries | SMS-004 |
| test/logic/sms/sms_import_service_test.dart | SMS-007 foreign currency / confirm all skips entries in another currency | USD stays pending; manual confirm works | SMS-007 |
| test/logic/sms/sms_import_service_test.dart | SMS-007 ... / confirm all takes an entry whose code is the app currency | USD confirmed when app currency USD | SMS-007 |
| test/logic/sms/sms_import_service_test.dart | SMS-008 unrecognized messages / a transaction-looking message no template reads is kept once | kept with key/address/body/date; noise dropped; rescan keeps 1 | SMS-008 |
| test/logic/sms/sms_import_service_test.dart | SMS-008 ... / a message a template reads is pending, not unrecognized | split | SMS-008 |
| test/logic/sms/sms_import_service_test.dart | SMS-008 ... / a dismissed unrecognized message is not stored again | dismissed stays, not listed | SMS-008 |
| test/logic/sms/sms_import_service_test.dart | SMS-008 ... / a failing write keeps nothing and does not move the point | atomic scan | SMS-008, SMS-001 |
| test/logic/sms/sms_import_service_test.dart | SMS-008 ... / retry turns the kept messages a saved template reads into pending entries | retryUnrecognized after saveTemplate -> 2 pending with rule category; scan creates nothing new | SMS-008 |
| test/logic/sms/sms_import_service_test.dart | SMS-008 ... / retry leaves messages no enabled template reads and uses only enabled templates | disabled template doesn't read | SMS-008 |
| test/logic/sms/sms_import_service_test.dart | SMS-008 ... / retry never touches a dismissed message | dismissed untouched | SMS-008 |
| test/logic/sms/sms_import_service_test.dart | SMS-008 ... / a scan that a new template makes read a kept message makes it pending only | scan converts kept message | SMS-008 |
| test/logic/sms/sms_import_service_test.dart | SMS-008 ... / retry is all or nothing; dismissUnrecognized hides the message and ignores unknown ids | atomic retry; hide | SMS-008 |
| test/logic/sms/sms_import_service_test.dart | dismiss and templates / dismiss hides the entry from the pending list | dismissed not in pending() | SMS-001 |
| test/logic/sms/sms_import_service_test.dart | dismiss and templates / restore sets a dismissed entry back to pending | restore -> pending; scan creates nothing | SMS-001 (undo dismissal) |
| test/logic/sms/sms_import_service_test.dart | dismiss and templates / restore resolves the category by the current rules | restored entry gets rule category | SMS-001 (re-resolve on undo) |
| test/logic/sms/sms_import_service_test.dart | dismiss and templates / restore ignores unknown, pending and confirmed entries; templates can be saved and deleted | guards | SMS-001 (edge) |
| test/db/sms_repository_test.dart | templates / are read by id, enabledTemplates skips the disabled | allTemplates by id; enabledTemplates filters | SMS-002 (by id, enabled) |
| test/db/sms_repository_test.dart | templates / keep their account; remove and removeAll; watch emits... by id | storage | none |
| test/db/sms_repository_test.dart | category rules / are read ordered by keyword | allRules sorted by keyword | none (NB: "earlier rule wins a tie" then means earlier keyword order at scan time? see notes) |
| test/db/sms_repository_test.dart | category rules / ruleByKeyword ignores case and needs the whole keyword | case-insensitive exact lookup | SMS-003 (case-insensitive uniqueness support) |
| test/db/sms_repository_test.dart | category rules / removeRulesOfCategory removes only that category's rules; remove and removeAll; watch | storage | SMS-005 (support) |
| test/db/sms_repository_test.dart | pending entries / status stored by name; pending() and getPending; isKnown is true for every status; watchPending newest first; watchPendingCount; clearPendingCategory clears only that category; removeAllPending removes every status | isKnown across statuses; clearPendingCategory | SMS-001 (isKnown any status), SMS-005 (support), XFER-006 (support) |
| test/db/sms_repository_test.dart | unrecognized messages / watchUnrecognized lists the non-dismissed, newest first; watchUnrecognizedCount; isUnrecognizedKnown covers dismissed ones and not pending ones; remove... | dismissed hidden from list and count; known incl. dismissed | SMS-008 |
| test/db/sms_repository_test.dart | Settings.lastSmsScanAt is null until set and round-trips | field storage | none |
| test/cubit/sms_scan_cubit_test.dart | manual scan (9 tests: scans from the scan point...; scanFrom ignores stored point; no inbox; denied permission; throwing request counts as denied; without a scan point it asks for a start point; read failure; zero count; a second scan while one runs is ignored) | manual flow; concurrent scan (incl. scanInBackground) ignored | SMS-006 (never concurrently) |
| test/cubit/sms_scan_cubit_test.dart | background scan / is skipped without the permission and never prompts | no requestPermission, no scan | SMS-006 |
| test/cubit/sms_scan_cubit_test.dart | background scan / is skipped on a platform without an inbox | no permission check | SMS-006 (edge) |
| test/cubit/sms_scan_cubit_test.dart | background scan / is skipped without a scan point and never asks for one | state unchanged (no startNeeded), no scan | SMS-006 |
| test/cubit/sms_scan_cubit_test.dart | background scan / runs with the permission and a scan point | scan from lastScanAt; no prompt | SMS-006 |
| test/cubit/sms_scan_cubit_test.dart | background scan / runs at most once a minute, whoever scanned last | manual scan then background at +59s skipped, at +60s runs | SMS-006 |
| test/cubit/sms_scan_cubit_test.dart | background scan / a failure is recorded quietly and still throttles | failure throttles | SMS-006 |
| test/cubit/sms_scan_cubit_test.dart | scanIfStale / scans when nothing ran yet and after a minute; a scan that needed a start point does not throttle | throttle | SMS-006 |
| test/routes/pages/sms/cubit/pending_entries_cubit_test.dart | list / starts loading and then follows the pending entries; a failing duplicate lookup is no flag; failing stream; closing | list items with category | none |
| test/routes/pages/sms/cubit/pending_entries_cubit_test.dart | list / counts only the entries confirm all would create | confirmableCount excludes uncategorized, foreign, wrong type | SMS-004, SMS-007 (count side) |
| test/routes/pages/sms/cubit/pending_entries_cubit_test.dart | possible duplicate (4 tests) | isPossibleDuplicate when same abs amount within 24h either side | none (undocumented behaviour) |
| test/routes/pages/sms/cubit/pending_entries_cubit_test.dart | notice / confirm all / dismiss / undo dismiss (10 tests) | notices; undo calls service.restore | SMS-001 (undo, UI side) |
| test/routes/pages/sms/cubit/sms_template_form_cubit_test.dart | opening/initial sample/save validity/submit/delete/preview/errors (~30 tests) | form state; preview errors | none, except below |
| test/routes/pages/sms/cubit/sms_template_form_cubit_test.dart | submit / re-reads the unrecognized SMS after saving, with the app currency digits | saveTemplate then retryUnrecognized(digits: 2) in order | SMS-008 (saving re-reads; enabling is only via this form) |
| test/routes/pages/sms/cubit/sms_template_form_cubit_test.dart | submit / a failing retry does not undo the save; a failing save does not retry | retry ordering | SMS-008 (edge) |
| test/routes/pages/sms/cubit/sms_template_form_cubit_test.dart | preview / without a date match the time of the preview is now | preview uses now | SMS-002-adjacent (preview) |
| test/routes/pages/sms/cubit/category_rule_form_cubit_test.dart | 9 tests (starts empty; needs keyword and category; switching type clears; type follows category; submit saves through the service; invalid does nothing; failing save; edit mode prefill; passes id and mode; clash emits duplicateKeyword) | form -> service.saveRule args; DuplicateRuleKeywordError surfaced | SMS-003 (UI side, via mocks) |
| test/routes/pages/sms/cubit/category_rules_cubit_test.dart | 5 tests | list/remove via service | none |
| test/routes/pages/sms/cubit/unrecognized_sms_cubit_test.dart | 5 tests | list/dismiss via service | SMS-008 (UI side) weakly |
| test/routes/pages/sms/cubit/sms_templates_cubit_test.dart | 4 tests (adds the built-in template once...; follows; failing; closing) | list | none |
| test/cubit/pending_count_cubit_test.dart | follows the pending count and cancels on close | count relay | none |
| test/cubit/unrecognized_count_cubit_test.dart | follows the unrecognized count and cancels on close | count relay | none |

## Tests outside the owned paths that touch rules

| File | Test | What the assertions pin | Rules matched |
|---|---|---|---|
| test/logic/stats/stats_service_test.dart | equals the former per-entry sums ... (6 generated); a range without entries gives nothing | totals by category equal legacy computation | none |
| test/logic/stats/stats_service_test.dart | the range is inclusive on both ends (ENTRY-002) | from==to==last ms includes the entry | ENTRY-002 (stats path) |
| test/logic/stats/stats_service_test.dart | watchChanges emits now and the earliest date is the index minimum | earliest | none |
| test/db/earliest_dates_test.dart | account: is null without operations; is the earliest of its entries, transactions and transfers; moves forward when the oldest is deleted; is not stored: creating operations writes no account row; entries: earliest entry date follows deletes | derived earliest dates, no account writes on entry/transaction create | ENTRY-005 ("no write maintains" account state) weakly |
| test/routes/pages/home/cubit/micro_overview_cubit_test.dart | sums income and expenses separately, as positive amounts; what is still to come this month is not counted; ... | income/expense split on home | ENTRY-007-adjacent (positive presentation; not the period-group API) |
| test/routes/pages/stats, group, home (others), l10n, ui, utils, perf, amount_expression, period_range, data_generator tests | (~120 tests) | UI plumbing, formatting, keypad arithmetic, period navigation, perf timings | none |
