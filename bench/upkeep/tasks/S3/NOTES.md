# S3 calibration notes

Pin: `bench-pin` (a46c926). New function only; nothing calls it. No rule changes.

## Calibration

a. **Pin + hidden tests: FAIL, by compile error** (`formatMoneyRoman` is not defined; the
   whole test file fails to load).
b. **Pin + reference.patch + hidden tests: PASS** (7 tests, tables of 32 values each
   direction, zero, minor units, 3999, 4000, 5000). `make check` PASS, `make test` PASS (883
   tests).
c. **Pin + reference: key PASSES** (`changed rule texts: none`). **Variant with MONEY-002
   reworded to drop its stored-values clause** ("Display formatting rounds via the selected
   currency.") **: key FAILS** (`MONEY-002: text fails match/forbid`); `cinch check` itself
   still passes on that variant (`rules: ok`).
d. **Pin + reference.patch: key PASSES** (same as c).

## Interface notes

- The brief's signature is kept exactly: `String formatMoneyRoman(Money money)` as a top-level
  function in `lib/models/money_format.dart`. DESIGN.md's table says `MoneyFormat.roman(Money)`;
  that is stale (the file has top-level functions, no `MoneyFormat` class), so the prompt and
  hidden tests use the top-level name.
- `Money` carries no currency, so the function cannot know the minor-unit digits. Every
  `Currency` at the pin has `minorUnits == 2`, and the prompt says so ("two minor-unit digits
  (hundredths)"), so the function divides by 100.
- Ambiguity settled in the prompt: an amount below one whole unit (e.g. `Money(-50)`) is
  zero and gives `N` with no sign.
- The reference adds no `cinch:rule` marker to its visible tests (no rule describes Roman
  output) and no new rule.

## Key

`MONEY-001` match `integer minor units` (the stable token) and `MONEY-002` match `never
changes stored values` (exact current wording: "Display formatting rounds via the selected
currency and never changes stored values."). Kind `leave`: the grader's generic checks (every
pin rule still exists and keeps a marker) apply on top.
