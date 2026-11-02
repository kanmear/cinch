# Category names are the same ignoring spaces

Right now two categories can be told apart by spaces alone: `Eating Out` and `EatingOut` can both exist.

Change it so that category names are unique **ignoring whitespace**:

- `Eating Out`, `EatingOut`, `Eating  Out` (two spaces) and ` Eating Out ` (spaces at the ends) are all the same name.
- Adding a category whose name collides this way with an existing one, and renaming a category to a name that collides with a different existing one, is refused exactly like an exact repeat is today: `DuplicateCategoryName` is thrown and the stored rows stay unchanged.
- Case still matters: `eatingout` and `EatingOut` are different names and may both exist.
- A category does not collide with itself: renaming `Eating Out` to `EatingOut` when no other category has that name is fine.

The interface is the existing one: `CategoryService.createCategory` and `CategoryService.updateCategory` (and the repository's `add` and `update` underneath) throw `DuplicateCategoryName`; its `name` is the name that was being saved.

Follow the repository's conventions. Add or update tests. Commit your work on the current branch when done.
