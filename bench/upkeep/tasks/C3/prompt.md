Add sealed categories to Budgetron.

A category whose name starts with `#` (for example `#Private`) is sealed. Sealed categories are read-only for entries: nothing may be added to one, moved into one, or changed or removed from one.

- Creating an entry in a sealed category, editing an entry so that it ends up in one, editing an entry that is in one, and deleting an entry that is in one must all fail. `EntryService.createEntry`, `EntryService.update` and `EntryService.deleteEntry` throw `SealedCategory` and write nothing at all (the entry, the category usages and everything else stay exactly as they were).
- Confirming a pending SMS entry with a sealed category throws `SealedCategory` too and writes nothing: the pending entry stays pending, no entry is created, and no merchant rule is saved.
- Confirm all skips the pending SMS entries whose category is sealed, leaves them pending, and still confirms the others. It does not throw because of them.
- Sealing follows the category's name and is not stored anywhere. Renaming a category to a name that starts with `#` seals it (its existing entries become locked), and renaming it to a name that does not start with `#` unseals it. A `#` anywhere else in the name does not seal it. Categories that are not sealed behave exactly as before.

`SealedCategory` is a class `SealedCategory implements Exception` in `lib/logic/category/sealed_category.dart`. It must be constructible by the app code and catchable by type; tests only check that it is thrown, not its message.

Follow the repository's conventions. Add or update tests. Commit your work on the current branch when done.
