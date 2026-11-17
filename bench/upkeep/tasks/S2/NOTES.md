# S2 calibration notes

Pin: `bench-pin` (a46c926). The interface is all existing API: `CategoryService.createCategory`
/ `updateCategory`, `CategoryRepository.add` / `update`, `DuplicateCategoryName`.

## Calibration

a. **Pin + hidden tests: FAIL on behavior, not on build.** 14 tests, 6 fail (the add
   collisions across service and repository, the symmetric collision, expense vs income, and
   the rename refusals). The case-distinct, own-name rename, delete-then-reuse and
   exact-repeat tests pass at the pin by design (they are "must stay true").
b. **Pin + reference.patch + hidden tests: PASS** (14/14). `make check` PASS (`rules: ok`),
   `make test` PASS (893 tests: 874 at the pin + 14 hidden + 4 new repository tests + 1 new import case).
c. **Reference with `.docs/` reverted: hidden PASS, `make check` PASS, `make test` PASS, key
   FAILS** (`CAT-001: text fails match/forbid`, no rule text changed).
d. **Pin + reference.patch: key PASSES** (`changed rule texts: CAT-001`).

## The reference

`@Unique()` is dropped from `EntryCategory.name`, so `lib/objectbox-model.json` changes (the
`name` index is retired: `retiredIndexUids` gets the old index, the property loses `indexId`
and `flags`). `ObjectBoxCategoryRepository._put` compares `categoryNameKey(name)`
(`lib/models/category/category_name.dart`, whitespace stripped with `\s`, case kept) with every
other row (different id) and throws `DuplicateCategoryName` before writing. `TransferDocument.validate`
uses the same key for categories, so an import file with `Gro ceries` and `Groceries` is a
`TransferFormatException` and leaves the store unchanged (XFER-002, new case in the visible
`import_atomic_test`). CAT-001's text and the visible repository tests are updated.

A solution that keeps the `@Unique()` and stores a second normalised column would need a
migration for existing rows; that was not tried. The import-file check is not required by the
prompt or the hidden tests.

## Interface notes

- The prompt says "whitespace" and lists only space characters; the hidden tests use spaces
  only (leading, trailing, repeated, between every letter), never tabs or non-breaking spaces.
- Hidden tests compare `snapshotAll(store)` before and after a refused write, so any partial
  write (including an id consumed by a row that was then removed) would fail; a solution that
  checks before writing, as the reference does, passes.

## Key

`CAT-001` match `space|whitespace`, forbid `exact match`. The pin text begins "Exact match,
case-sensitive", so the forbid catches an untouched rule. A rewrite that says "case-sensitive
but ignoring spaces" still passes (`case-sensitive` is not forbidden).

## Amendment 1 (2026-10-28, with S1's)

The `exact match` forbid is dropped. A correct rewrite can keep the phrase, as in "an exact
match once whitespace is removed", and the forbid would reject it. That's S1's false-negative
failure mode, caught here before any S2 run hit it. The forbid isn't needed: the pin text has
no `space` or `whitespace`, so `match` already fails an untouched rule. Checked: the pin
fails, while the reference and the "exact match once whitespace is removed" wording pass.
