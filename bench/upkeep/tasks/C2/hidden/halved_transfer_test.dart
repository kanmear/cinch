import 'dart:convert';

import 'package:clock/clock.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:budgetron/db/entry_repository.dart';
import 'package:budgetron/db/transfer/importer.dart';
import 'package:budgetron/db/transfer/transfer_format.dart';
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

/// Flags the stored entries with these absolute amounts as halved.
void _halve(TestServices services, Set<int> absMinors) {
  for (final entry in services.entries.find(const EntryQuery())) {
    if (absMinors.contains(entry.amountMinor.abs())) {
      entry.isHalved = true;
      services.entries.put(entry);
    }
  }
}

/// Amount, date and flag of every stored entry, independent of ids.
Set<String> _flags(TestServices services) => {
  for (final entry in services.entries.find(const EntryQuery()))
    '${entry.amountMinor}|${entry.dateTime.toUtc().toIso8601String()}|'
        '${entry.isHalved}',
};

Set<String> _keysOf(Object? node, [Set<String>? into]) {
  final keys = into ?? <String>{};
  if (node is Map<String, Object?>) {
    for (final e in node.entries) {
      keys.add(e.key);
      _keysOf(e.value, keys);
    }
  } else if (node is List) {
    for (final item in node) {
      _keysOf(item, keys);
    }
  }
  return keys;
}

void main() {
  late Store source;
  late Store plain;
  late Store target;
  late TestServices from;
  late TestServices unhalved;
  late TestServices to;

  setUp(() {
    source = openTestStore();
    plain = openTestStore();
    target = openTestStore();
    from = TestServices(source, clock: Clock.fixed(seedNow));
    unhalved = TestServices(plain, clock: Clock.fixed(seedNow));
    to = TestServices(target, clock: Clock.fixed(seedNow));
    seedAllKinds(from);
    seedAllKinds(unhalved);
  });

  tearDown(() {
    source.close();
    plain.close();
    target.close();
  });

  // Amounts in the seeded store: 1230, 10, 777, 50000, 250000, 399.
  final halvedAmounts = {1230, 777, 399, 250000};

  test(
    'halving changes what the app shows, so the round trip is a real test',
    () {
      _halve(from, halvedAmounts);

      expect(observe(from), isNot(observe(unhalved)));
    },
  );

  test('export then import keeps every flag and what the app shows', () {
    _halve(from, halvedAmounts);
    final first = exporterOf(from).exportJson();

    _importerOf(to).run(TransferDocument.parse(first));

    expect(_flags(to), _flags(from));
    expect(
      _flags(to).where((f) => f.endsWith('true')).length,
      halvedAmounts.length,
    );
    expect(observe(to), observe(from));
    expect(exporterOf(to).exportJson(), first);
  });

  test('the file carries the flag on each entry', () {
    _halve(from, halvedAmounts);

    final json =
        jsonDecode(exporterOf(from).exportJson()) as Map<String, Object?>;
    final entries = (json['entries']! as List).cast<Map<String, Object?>>();

    expect(entries.where((e) => e['isHalved'] == true).length, 4);
    for (final entry in entries) {
      expect(entry['isHalved'], isA<bool>());
    }
  });

  test('a file from before the flag imports every entry as not halved', () {
    final json =
        jsonDecode(exporterOf(from).exportJson()) as Map<String, Object?>;
    json['formatVersion'] = 4;
    for (final entry in json['entries']! as List) {
      (entry as Map<String, Object?>).remove('isHalved');
    }

    _importerOf(to).run(TransferDocument.parse(jsonEncode(json)));

    expect(to.entries.find(const EntryQuery()), isNotEmpty);
    expect(to.entries.find(const EntryQuery()).map((e) => e.isHalved).toSet(), {
      false,
    });
    expect(observe(to), observe(from));
  });

  test('importing an older file replaces halved entries with whole ones', () {
    _halve(to, halvedAmounts);
    final json =
        jsonDecode(exporterOf(unhalved).exportJson()) as Map<String, Object?>;
    json['formatVersion'] = 4;
    for (final entry in json['entries']! as List) {
      (entry as Map<String, Object?>).remove('isHalved');
    }

    _importerOf(to).run(TransferDocument.parse(jsonEncode(json)));

    expect(observe(to), observe(unhalved));
  });

  test('derived values are not exported: the file differs from an unhalved '
      'one only by the flag', () {
    _halve(from, halvedAmounts);
    final halvedJson =
        jsonDecode(exporterOf(from).exportJson()) as Map<String, Object?>;
    final plainJson =
        jsonDecode(exporterOf(unhalved).exportJson()) as Map<String, Object?>;

    expect(
      _keysOf(halvedJson).difference(_keysOf(plainJson)),
      anyOf(isEmpty, {'isHalved'}),
    );
    expect(halvedJson['accounts'], plainJson['accounts']);
    expect(halvedJson['budgets'], plainJson['budgets']);
    expect(halvedJson['transactions'], plainJson['transactions']);
    expect(halvedJson['transfers'], plainJson['transfers']);

    final halvedEntries = (halvedJson['entries']! as List)
        .cast<Map<String, Object?>>();
    final plainEntries = (plainJson['entries']! as List)
        .cast<Map<String, Object?>>();
    expect(
      halvedEntries.map((e) => e['amountMinor']).toList(),
      plainEntries.map((e) => e['amountMinor']).toList(),
      reason: 'amounts are exported whole',
    );
  });
}
