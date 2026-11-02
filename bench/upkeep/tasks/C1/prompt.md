# Derived category colors

Right now every category stores a color that the user picks in the category form. We want to stop storing it: a category's color should be worked out from its name instead.

## What to build

- Add `lib/logic/category/category_color.dart` with two top-level members:
  - `const List<Color> categoryPalette`: a fixed, non-empty list of colors that categories can have.
  - `Color categoryColorFor(String name)`: maps a category name to one entry of `categoryPalette`. The mapping must be deterministic and stable: the same name gives the same color on every run and on every platform, so don't use `String.hashCode` (Dart doesn't promise it is stable). Use your own stable hash of the name. Different names should spread across the palette rather than all getting one color.
- Everywhere the app shows a category's color (lists, pickers, charts, budgets, SMS screens, the category form), use the derived color for that category's current name. Renaming a category therefore changes its color. The category form no longer offers a color picker; it can show the derived color as a plain swatch next to the name field.
- A category no longer holds a color. `EntryCategory` is created from its name and type alone, for example `EntryCategory(name: 'Pets', isExpense: true)`, with no `color` argument.
- Data export no longer writes a color for categories. Accounts keep their stored color, and everything else in the export stays as it is.
- Importing an export file made before this change, which still has a `color` on each category, must keep working: the import succeeds, every category comes in with its name, type and usages, and the old color is ignored. Export followed by import into an empty store must still give back everything else unchanged.

Follow the repository's conventions. Add or update tests. Commit your work on the current branch when done.
