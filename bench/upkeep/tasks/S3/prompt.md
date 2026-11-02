# Show an amount in Roman numerals

We want to be able to render an amount of money in Roman numerals (for a joke theme to come later). Nothing in the app uses it yet, so this is only the formatting function.

Add a new top-level function in `lib/models/money_format.dart`:

```dart
String formatMoneyRoman(Money money)
```

- It writes the **whole units** of the amount as a Roman numeral in the standard subtractive form: 4 is `IV`, 9 is `IX`, 40 is `XL`, 1994 is `MCMXCIV`, 3999 is `MMMCMXCIX`.
- The minor units are dropped, not rounded: the amount is truncated toward zero. All currencies in the app have two minor-unit digits (hundredths), so `Money(199499)`, which is 1994.99, gives `MCMXCIV`.
- A negative amount is prefixed with `-`: `Money(-199400)` gives `-MCMXCIV`.
- Zero is written `N`. An amount whose whole units are zero, such as `Money(99)` or `Money(-50)`, is zero too and gives `N`, without a sign.
- Values above 3999 just repeat `M`: 4000 is `MMMM` and 5000 is `MMMMM`.

The function only produces text; the amount it is given is not changed.

Follow the repository's conventions. Add or update tests. Commit your work on the current branch when done.
