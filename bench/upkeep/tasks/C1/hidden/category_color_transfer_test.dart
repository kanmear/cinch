import 'dart:convert';
import 'dart:io';

import 'package:clock/clock.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:budgetron/db/transfer/importer.dart';
import 'package:budgetron/db/transfer/transfer_format.dart';
import 'package:budgetron/logic/category/category_color.dart';
import 'package:budgetron/models/category/category.dart';
import 'package:budgetron/objectbox.g.dart';

import '../../support/test_services.dart';
import '../../support/test_store.dart';
import '../../support/transfer_seed.dart';

DataImporter _importerOf(TestServices services) => DataImporter(
  categories: services.categories,
  groups: services.groups,
  accounts: services.accounts,
  entries: services.entries,
  budgets: services.budgets,
  settings: services.settings,
  sms: services.sms,
  unitOfWork: services.unitOfWork,
);

Map<String, Object?> _exportOf(TestServices services) =>
    jsonDecode(exporterOf(services).exportJson()) as Map<String, Object?>;

List<Map<String, Object?>> _categoriesOf(Map<String, Object?> json) =>
    (json['categories']! as List).cast<Map<String, Object?>>();

void main() {
  late Store source;
  late Store target;
  late TestServices from;
  late TestServices to;

  setUp(() {
    source = openTestStore();
    target = openTestStore();
    from = TestServices(source, clock: Clock.fixed(seedNow));
    to = TestServices(target, clock: Clock.fixed(seedNow));
  });

  tearDown(() {
    source.close();
    target.close();
  });

  test('a category is created from a name and a type alone', () {
    final category = EntryCategory(name: 'Pets', isExpense: true);
    from.categoryService.createCategory(category);

    final stored = from.categories.find().single;
    expect(stored.name, 'Pets');
    expect(stored.isExpense, isTrue);
    expect(categoryPalette, contains(categoryColorFor(stored.name)));
  });

  test('the export carries no color for any category', () {
    seedAllKinds(from);

    final categories = _categoriesOf(_exportOf(from));

    expect(categories, isNotEmpty);
    for (final category in categories) {
      expect(category.keys, isNot(contains('color')), reason: '$category');
      expect(category['name'], isA<String>());
      expect(category['isExpense'], isA<bool>());
    }
  });

  test('accounts still export their color', () {
    seedAllKinds(from);

    final accounts = (_exportOf(from)['accounts']! as List)
        .cast<Map<String, Object?>>();

    expect(accounts, isNotEmpty);
    expect(accounts, everyElement(containsPair('color', isA<String>())));
  });

  test('export then import into an empty store is lossless', () {
    seedAllKinds(from);

    _importerOf(to).run(TransferDocument.parse(exporterOf(from).exportJson()));

    expect(observe(to), observe(from));
    expect(exporterOf(to).exportJson(), exporterOf(from).exportJson());
    expect(
      {for (final a in to.accounts.find()) a.name: a.color},
      {for (final a in from.accounts.find()) a.name: a.color},
      reason: 'account colors are still stored',
    );
    expect(
      [for (final c in to.categories.find()) (c.name, c.isExpense, c.usages)],
      [for (final c in from.categories.find()) (c.name, c.isExpense, c.usages)],
    );
  });

  test(
    'a file from before the change, with category colors, still imports',
    () {
      seedAllKinds(from);
      final json = _exportOf(from);
      json['formatVersion'] = TransferDocument.formatVersion - 1;
      final colors = [
        'ff8bc34a',
        'ffff5252',
        '805bcd14',
        'ff795548',
        'ff000000',
      ];
      for (final (i, category) in _categoriesOf(json).indexed) {
        category['color'] = colors[i % colors.length];
      }

      _importerOf(to).run(TransferDocument.parse(jsonEncode(json)));

      expect(observe(to), observe(from));
      expect(
        to.categories.find().map((c) => c.name).toSet(),
        from.categories.find().map((c) => c.name).toSet(),
      );
      expect(
        _categoriesOf(_exportOf(to)),
        everyElement(isNot(contains('color'))),
        reason: 'the old colors are dropped, not carried over',
      );
      expect(exporterOf(to).exportJson(), exporterOf(from).exportJson());
    },
  );

  test(
    'the checked-in version 1 file, with category colors, still imports',
    () {
      final text = File('test/db/transfer/fixture_v1.json').readAsStringSync();
      final names = _categoriesOf(jsonDecode(text) as Map<String, Object?>)
          .map((c) => c['name']! as String)
          .toSet();
      expect(names, isNotEmpty);

      _importerOf(to).run(TransferDocument.parse(text));

      expect(to.categories.find().map((c) => c.name).toSet(), names);
      expect(
        _categoriesOf(_exportOf(to)),
        everyElement(isNot(contains('color'))),
      );
    },
  );
}
