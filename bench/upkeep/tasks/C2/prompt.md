# Halved entries

Add support for "halved" entries to Budgetron. Some expenses are shared, and the user only wants half of such an entry to count against their money.

## What to build

- An entry gets a stored boolean `isHalved` on the `Entry` model, `false` by default. Code sets it like any other field before saving the entry (for example `entry.isHalved = true` before `EntryService.createEntry`). No screen needs to change.
- A halved entry counts at **half its amount** in two places:
  - the **balance** of its account (`AccountService.balanceOf` and the balance streams);
  - the **spent** sum of a budget period (`BudgetService.summaries` and `watchSummaries`).
- Half means integer minor units, truncated toward zero, each entry on its own: `amountMinor ~/ 2`, whichever the sign. An expense stored as `-1001` counts as `-500`, an income of `999` counts as `499`, and an amount of `1` or `-1` counts as `0`. Entries are rounded one by one, never as a sum.
- Everywhere else nothing changes. The **stored amount stays whole** (`amountMinor` is still `-1001`), and entry lists and per-category totals keep showing the whole amount.
- **Export and import** carry the flag. Each entry object in the export file has an `isHalved` boolean, and importing that file gives back the same flags, so an export followed by an import into an empty store loses nothing. The file still holds only stored facts: the halved sums, the balances and the spent values are derived, and are not written to the file.
- A file exported **before** this change has no `isHalved` on its entries. Importing it must still work, and every entry in it reads as not halved.

Follow the repository's conventions. Add or update tests. Commit your work on the current branch when done.
