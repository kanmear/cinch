import 'package:flutter_test/flutter_test.dart';

import 'package:budgetron/db/objectbox/object_box_entry_repository.dart';
import 'package:budgetron/logic/category/sealed_category.dart';
import 'package:budgetron/logic/entry/entry_service.dart';
import 'package:budgetron/models/category/category.dart';
import 'package:budgetron/models/entry.dart';
import 'package:budgetron/models/money.dart';
import 'package:budgetron/objectbox.g.dart';

import '../../support/fixtures.dart';
import '../../support/store_snapshot.dart';
import '../../support/test_services.dart';
import '../../support/test_store.dart';

class _FailingEntryRepository extends ObjectBoxEntryRepository {
  bool failPut = false;

  _FailingEntryRepository(super.store);

  @override
  int put(Entry entry) {
    if (failPut) throw StateError('entry write failed');
    return super.put(entry);
  }
}

void main() {
  late Store store;
  late _FailingEntryRepository failing;
  late TestServices services;
  late EntryService service;
  late EntryCategory food, rent, vault;

  final day = DateTime(2024, 3, 10, 12);

  EntryUpdate updateTo(EntryCategory category, {int minor = 900}) =>
      EntryUpdate(amount: Money(minor), dateTime: day, category: category);

  Entry newEntry([int minor = 1250]) =>
      makeEntry(amount: Money(minor), dateTime: day);

  Entry create(EntryCategory category) {
    final entry = newEntry();
    service.createEntry(entry, category);
    return entry;
  }

  /// Renames the stored category through the category service, as the app does.
  void rename(EntryCategory category, String name) {
    category.name = name;
    services.categoryService.updateCategory(category);
  }

  /// An entry created while its category was plain, which is then sealed by
  /// renaming the category.
  Entry entryInSealed() {
    final entry = create(food);
    rename(food, '#Food');
    return entry;
  }

  Entry stored(int id) => services.entries.get(id)!;
  int usages(EntryCategory c) => store.box<EntryCategory>().get(c.id)!.usages;

  setUp(() {
    store = openTestStore();
    failing = _FailingEntryRepository(store);
    services = TestServices(store, entryRepository: failing);
    service = services.entryService;
    food = makeCategory(name: 'Food');
    rent = makeCategory(name: 'Rent');
    vault = makeCategory(name: '#Vault');
    for (final c in [food, rent, vault]) {
      services.categories.add(c);
    }
  });
  tearDown(() => store.close());

  group('create', () {
    test('into a sealed category throws SealedCategory and writes nothing', () {
      final before = snapshotAll(store);
      final entry = newEntry();

      expect(
        () => service.createEntry(entry, vault),
        throwsA(isA<SealedCategory>()),
      );

      expect(snapshotAll(store), before);
      expect(services.entries.count(), 0);
      expect(usages(vault), 0);
      expect(vault.usages, 0);
      expect(entry.id, 0);
    });

    test('into a category renamed to a # name throws', () {
      rename(food, '#Food');
      final before = snapshotAll(store);

      expect(() => create(food), throwsA(isA<SealedCategory>()));

      expect(snapshotAll(store), before);
    });

    test('a # that is not the first character does not seal', () {
      rename(food, 'Food #1');

      final entry = create(food);

      expect(stored(entry.id).category.targetId, food.id);
      expect(usages(food), 1);
    });

    test('into a plain category still stores the entry and the usage', () {
      final entry = create(food);

      expect(stored(entry.id).amountMinor, -1250);
      expect(usages(food), 1);
    });

    test('a plain create is still atomic', () {
      failing.failPut = true;
      final entry = newEntry();

      expect(() => service.createEntry(entry, food), throwsStateError);

      expect(services.entries.count(), 0);
      expect(usages(food), 0);
      expect(food.usages, 0);
      expect(entry.id, 0);
    });
  });

  group('edit', () {
    test(
      'moving an entry into a sealed category throws and writes nothing',
      () {
        final entry = create(food);
        final before = snapshotAll(store);

        expect(
          () => service.update(entry.id, updateTo(vault)),
          throwsA(isA<SealedCategory>()),
        );

        expect(snapshotAll(store), before);
        expect(stored(entry.id).category.targetId, food.id);
        expect(usages(food), 1);
        expect(usages(vault), 0);
      },
    );

    test('moving an entry into a category renamed to # throws', () {
      final entry = create(food);
      rename(rent, '#Rent');
      final before = snapshotAll(store);

      expect(
        () => service.update(entry.id, updateTo(rent)),
        throwsA(isA<SealedCategory>()),
      );

      expect(snapshotAll(store), before);
    });

    test('editing an entry that is in a sealed category throws', () {
      final entry = entryInSealed();
      final before = snapshotAll(store);

      expect(
        () => service.update(entry.id, updateTo(food, minor: 333)),
        throwsA(isA<SealedCategory>()),
      );

      expect(snapshotAll(store), before);
      expect(stored(entry.id).amountMinor, -1250);
    });

    test('moving an entry out of a sealed category throws', () {
      final entry = entryInSealed();
      final before = snapshotAll(store);

      expect(
        () => service.update(entry.id, updateTo(rent)),
        throwsA(isA<SealedCategory>()),
      );

      expect(snapshotAll(store), before);
      expect(stored(entry.id).category.targetId, food.id);
    });

    test(
      'an entry stored in a sealed category by a lower layer is locked too',
      () {
        // Written below the service, as a row that was stored before sealing.
        final entry = newEntry()..category.target = vault;
        services.entries.put(entry);
        final before = snapshotAll(store);

        expect(
          () => service.update(entry.id, updateTo(rent)),
          throwsA(isA<SealedCategory>()),
        );

        expect(snapshotAll(store), before);
      },
    );

    test('between plain categories still moves the usage', () {
      final entry = create(food);

      service.update(entry.id, updateTo(rent, minor: 700));

      expect(stored(entry.id).amountMinor, -700);
      expect(stored(entry.id).category.targetId, rent.id);
      expect(usages(food), 0);
      expect(usages(rent), 1);
    });
  });

  group('delete', () {
    test('an entry in a sealed category throws and nothing is removed', () {
      final entry = entryInSealed();
      final before = snapshotAll(store);

      expect(
        () => service.deleteEntry(stored(entry.id)),
        throwsA(isA<SealedCategory>()),
      );

      expect(snapshotAll(store), before);
      expect(services.entries.count(), 1);
      expect(usages(food), 1);
    });

    test('the entry object that was created is also refused', () {
      final entry = entryInSealed();
      final before = snapshotAll(store);

      expect(() => service.deleteEntry(entry), throwsA(isA<SealedCategory>()));

      expect(snapshotAll(store), before);
    });

    test('an entry in a plain category is still removed', () {
      final entry = create(food);

      service.deleteEntry(stored(entry.id));

      expect(services.entries.count(), 0);
      expect(usages(food), 0);
    });
  });

  group('sealing follows the name', () {
    test('renaming to a # name seals, renaming back unseals', () {
      final entry = create(food);

      rename(food, '#Food');
      expect(() => create(food), throwsA(isA<SealedCategory>()));
      expect(
        () => service.update(entry.id, updateTo(food)),
        throwsA(isA<SealedCategory>()),
      );
      expect(
        () => service.deleteEntry(stored(entry.id)),
        throwsA(isA<SealedCategory>()),
      );

      rename(food, 'Food');
      final second = create(food);
      service.update(entry.id, updateTo(food, minor: 400));
      expect(stored(entry.id).amountMinor, -400);
      service.deleteEntry(stored(entry.id));
      service.deleteEntry(stored(second.id));

      expect(services.entries.count(), 0);
      expect(usages(food), 0);
    });

    test(
      'renaming a sealed category to a plain name makes its entries editable',
      () {
        final entry = newEntry()..category.target = vault;
        services.entries.put(entry);

        rename(vault, 'Vault');

        service.update(entry.id, updateTo(rent));
        expect(stored(entry.id).category.targetId, rent.id);
      },
    );

    test('a category with entries can still be renamed to a # name', () {
      create(food);

      rename(food, '#Food');

      expect(services.categories.get(food.id)!.name, '#Food');
      expect(services.entries.count(), 1);
    });
  });
}
