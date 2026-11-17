import 'dart:ui';

import 'package:flutter_test/flutter_test.dart';

import 'package:budgetron/logic/category/category_color.dart';

void main() {
  const names = [
    'Groceries',
    'Rent',
    'Salary',
    'Eating Out',
    'eating out',
    'Кафе',
    '食品',
    '',
    ' ',
    'A very long category name that goes on and on, '
        'well past anything a person would type',
  ];

  test('the palette is a non-empty list of colors', () {
    expect(categoryPalette, isNotEmpty);
    expect(categoryPalette, everyElement(isA<Color>()));
  });

  test('every name maps to a color of the palette', () {
    for (final name in names) {
      expect(categoryPalette, contains(categoryColorFor(name)), reason: name);
    }
    for (var i = 0; i < 300; i++) {
      expect(categoryPalette, contains(categoryColorFor('Category $i')));
    }
  });

  test('the same name always gets the same color', () {
    for (final name in names) {
      final first = categoryColorFor(name);

      expect(categoryColorFor(name), first, reason: name);
      // A different String instance with the same characters.
      expect(
        categoryColorFor(String.fromCharCodes(name.codeUnits)),
        first,
        reason: name,
      );
      expect(categoryColorFor(['', name, ''].join()), first, reason: name);
    }
  });

  test('the color depends on the name, not on a constant', () {
    final used = {
      for (var i = 0; i < 300; i++) categoryColorFor('Category $i'),
    };

    expect(used.length, greaterThan(1));
  });
}
