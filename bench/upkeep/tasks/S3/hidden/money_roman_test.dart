import 'package:flutter_test/flutter_test.dart';

import 'package:budgetron/models/money.dart';
import 'package:budgetron/models/money_format.dart';

void main() {
  group('formatMoneyRoman', () {
    // Whole units and the numeral they must give.
    const table = <int, String>{
      1: 'I',
      2: 'II',
      3: 'III',
      4: 'IV',
      5: 'V',
      6: 'VI',
      9: 'IX',
      10: 'X',
      14: 'XIV',
      19: 'XIX',
      40: 'XL',
      49: 'XLIX',
      50: 'L',
      90: 'XC',
      99: 'XCIX',
      100: 'C',
      400: 'CD',
      444: 'CDXLIV',
      500: 'D',
      900: 'CM',
      999: 'CMXCIX',
      1000: 'M',
      1994: 'MCMXCIV',
      2024: 'MMXXIV',
      2444: 'MMCDXLIV',
      3000: 'MMM',
      3888: 'MMMDCCCLXXXVIII',
      3999: 'MMMCMXCIX',
      4000: 'MMMM',
      4999: 'MMMMCMXCIX',
      5000: 'MMMMM',
      12000: 'MMMMMMMMMMMM',
    };

    test('whole amounts', () {
      table.forEach((units, numeral) {
        expect(
          formatMoneyRoman(Money(units * 100)),
          numeral,
          reason: '$units units',
        );
      });
    });

    test('negative amounts get a minus sign', () {
      table.forEach((units, numeral) {
        expect(
          formatMoneyRoman(Money(-units * 100)),
          '-$numeral',
          reason: '-$units units',
        );
      });
    });

    test('zero is N', () {
      expect(formatMoneyRoman(Money.zero), 'N');
      expect(formatMoneyRoman(const Money(0)), 'N');
    });

    test('minor units are dropped, not rounded', () {
      expect(formatMoneyRoman(const Money(199499)), 'MCMXCIV');
      expect(formatMoneyRoman(const Money(199400)), 'MCMXCIV');
      expect(formatMoneyRoman(const Money(199450)), 'MCMXCIV');
      expect(formatMoneyRoman(const Money(500)), 'V');
      expect(formatMoneyRoman(const Money(599)), 'V');
      expect(formatMoneyRoman(const Money(101)), 'I');
      expect(formatMoneyRoman(const Money(199)), 'I');
    });

    test('negative minor units truncate toward zero', () {
      expect(formatMoneyRoman(const Money(-199499)), '-MCMXCIV');
      expect(formatMoneyRoman(const Money(-599)), '-V');
      expect(formatMoneyRoman(const Money(-101)), '-I');
      expect(formatMoneyRoman(const Money(-199)), '-I');
    });

    test('an amount below one whole unit is N, with no sign', () {
      expect(formatMoneyRoman(const Money(1)), 'N');
      expect(formatMoneyRoman(const Money(99)), 'N');
      expect(formatMoneyRoman(const Money(-1)), 'N');
      expect(formatMoneyRoman(const Money(-50)), 'N');
      expect(formatMoneyRoman(const Money(-99)), 'N');
    });

    test('the argument is not changed', () {
      const money = Money(-199499);

      formatMoneyRoman(money);

      expect(money.minor, -199499);
    });
  });
}
