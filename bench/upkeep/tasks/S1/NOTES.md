# S1 calibration notes

Pin: `bench-pin` (a46c926) in a fresh clone, `make gen` run. Commands are the ones in the
task brief; results below.

## Calibration

a. **Pin + hidden tests: FAIL on behavior, not on build.** `flutter test test/_bench/S1` at the
   pin: 8 tests, 4 fail ("is skipped one minute after the last scan", "is skipped at 5 and
   12 minutes 59 seconds", "the 13 minutes count from the end of the last scan", "a failed scan
   still throttles for 13 minutes"); the other 4 pass (first scan runs, runs again after 13
   minutes, never concurrent, permission and scan point still required).
b. **Pin + reference.patch + hidden tests: PASS** (8/8). `make check` PASS, `make test` PASS
   (882 tests: the 874 at the pin, 8 hidden copied under `test/_bench/S1`).
c. **Reference with `.docs/` reverted: hidden PASS, `make test` PASS, key FAILS**
   (`SMS-006: text fails match/forbid`).
d. **Pin + reference.patch: key PASSES** (`changed rule texts: SMS-006`). The untouched pin
   fails the key too (as it should).

## The reference

`SmsScanCubit.minInterval` 1 minute to 13 minutes, the two doc comments that said "a minute",
SMS-006's text ("at most once every 13 minutes"), and the visible test updated (12 min 59 s is
skipped, 13 min runs). `.docs/architecture.md` mentions SMS-006 only by ID and needs no edit.

## Interface and scope decisions

- No interface change: `SmsScanCubit(service, source, clock, digits:, currencyCode:)` and
  `scanInBackground()` exist at the pin; the hidden tests build the cubit the way the existing
  cubit test does (mocktail mocks of `SmsImportService` and `SmsSource`, `Clock(() => now)`).
- `minInterval` also gates `scanIfStale()` (opening the SMS page), which therefore also
  waits 13 minutes after the reference change. The prompt asks only for background scans and
  the hidden tests do not exercise `scanIfStale()` or a manual scan followed by a background
  one, so a solution that adds a separate background-only interval passes too.
- Boundary: exactly 13 minutes after the last scan finished runs (the pin's `< minInterval`
  does the same at 1 minute). The prompt says "less than 13 minutes ... is skipped", so this
  is stated.
- "A failed scan still counts" is existing behavior (an existing visible test), stated in the
  prompt because a hidden test checks it.

## Key

`SMS-006` match `13`, forbid `once a minute`, `once per minute`, `every minute`, `one minute`,
`per minute`, `1 minute`, `60 seconds`. Plain `13` (not `\b13\b`) is the looser choice. Risk:
a correct rewrite that mentions "a one minute interval no longer applies" would trip the
forbid list; unlikely.

## Amendment 1 (2026-10-28, after 1 graded run)

The key above failed a correct corpus. In pi run `S1-arm2-r2` the agent gave background scans
their own 13-minute interval and kept UI scans at one minute, a solution the notes above call
valid, and wrote SMS-006 as "… runs at most once every 13 minutes. UI scans run at most once a
minute." The `once a minute` forbid rejected it. The risk named above was real and not unlikely.

The amended key matches `13 minutes`. It forbids two things instead: the pin's sentence
(`(^|[.;]\s*)scans run at most once (a|per|every) minute`), and a one-minute interval in the
same sentence as "background" (`background[^.;]*(once (a|per) minute|every minute|one minute|1
minute|60 seconds)`). Checked texts:

| Text | Key |
|---|---|
| The pin | fail |
| The reference | pass |
| `S1-arm2-r2` | pass |
| 13 minutes added, pin sentence kept | fail |
| "A background scan runs at most once a minute; manual scans every 13 minutes" | fail |
| A background-only interval in other words, UI scans at once a minute | pass |

Arm 1 isn't graded on corpus, so the amendment applies the same way to arms 2 and 3. The
DESIGN section "Amendments" has the procedure.

## Amendment 2 (2026-11-01, after all 90 pi runs were graded)

The amendment-1 forbids still rejected four correct SMS-006 texts in the pi runs. Three give
background scans a 13-minute interval and name the interval for other scans in the same
sentence, so the second forbid (`background[^.;]*(… once a minute …)`) matched across the
comma. The fourth (`S1-arm3-r4`) keeps "Scans run at most once a minute" as a separate
statement and then says a background scan starting less than 13 minutes after the last scan
is skipped, so the first forbid (the pin's sentence) matched; the rule is true of the code
(background scans wait 13 minutes) and states the interval, so it is correct as well. Both
forbids guard against the same thing the hidden tests already grade, so they are dropped.

The key is now SMS-006 `match: ["13 minutes"]`, no forbid. The untouched rule still fails
(it has no `13 minutes`), and so does a rule that keeps the pin's text (calibration (c) and
(d) are unchanged: the reference with `.docs/` reverted fails, the full reference passes).

| Run | SMS-006 text (after the item number) | Amendment-1 key | Amendment-2 key |
|---|---|---|---|
| `S1-arm2-r1` | A background scan (app start or resume) runs only when the read permission is already granted and a scan point exists; it never prompts and never asks for a start point. A background scan runs at most once every 13 minutes after the last scan finished (a failed scan counts too), a foreground scan at most once a minute, and two scans never run at the same time. | fail (second forbid) | pass |
| `S1-arm3-r0` | … Scans never run concurrently; a background scan runs at most once every 13 minutes after the previous scan finished, whoever scanned last, and a page-opening scan (scanIfStale) at most once a minute. | fail (second forbid) | pass |
| `S1-arm3-r3` | … A background scan runs at most once every 13 minutes after the previous scan (of either kind) finished, other scans at most once a minute, and scans never run concurrently. | fail (second forbid) | pass |
| `S1-arm3-r4` | … Scans run at most once a minute and never concurrently; a background scan that would start less than 13 minutes after the last scan finished is skipped, whatever that scan's outcome. | fail (first forbid) | pass |
| The pin | … Scans run at most once a minute and never concurrently. | fail | fail (no `13 minutes`) |

Two texts that amendment 1 failed on purpose now pass: "A background scan runs at most once
every 13 minutes. Scans run at most once a minute …" and "A background scan runs at most once
a minute; manual scans every 13 minutes." Both contain `13 minutes`. The second is wrong
about background scans, and the key no longer catches it; the hidden tests do (behavior is
graded separately). `TestKeyAmendments` pins all of these cases.
