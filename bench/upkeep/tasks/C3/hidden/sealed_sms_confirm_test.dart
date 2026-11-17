import 'package:flutter_test/flutter_test.dart';

import 'package:budgetron/db/entry_repository.dart';
import 'package:budgetron/logic/category/sealed_category.dart';
import 'package:budgetron/logic/entry/entry_service.dart';
import 'package:budgetron/models/category/category.dart';
import 'package:budgetron/models/enums/pending_entry_status.dart';
import 'package:budgetron/models/sms/pending_entry.dart';
import 'package:budgetron/objectbox.g.dart';

import '../../support/fixtures.dart';
import '../../support/store_snapshot.dart';
import '../../support/test_services.dart';
import '../../support/test_store.dart';

void main() {
  late Store store;
  late TestServices services;
  late EntryCategory food, salary, vault;
  var next = 0;

  PendingEntry pendingIn(
    EntryCategory? category, {
    bool isExpense = true,
    int minor = 529,
  }) {
    next++;
    final pending = PendingEntry(
      amountMinor: minor,
      dateTime: DateTime(2026, 10, 6, 12, next),
      isExpense: isExpense,
      merchant: 'SHOP $next',
      rawText: 'Oplata $minor BYN. SHOP $next',
      smsKey: '$next:1',
      currencyCode: 'BYN',
    );
    pending.category.target = category;
    services.sms.putPending(pending);
    return pending;
  }

  EntryUpdate valuesFor(PendingEntry pending, EntryCategory category) =>
      EntryUpdate(
        amount: pending.amount,
        dateTime: pending.dateTime,
        category: category,
      );

  PendingEntryStatus statusOf(PendingEntry pending) =>
      services.sms.getPending(pending.id)!.status;

  int usages(EntryCategory c) => store.box<EntryCategory>().get(c.id)!.usages;
  int entryCount() => services.entries.find(const EntryQuery()).length;

  void rename(EntryCategory category, String name) {
    category.name = name;
    services.categoryService.updateCategory(category);
  }

  setUp(() {
    store = openTestStore();
    services = TestServices(store);
    next = 0;
    food = makeCategory(name: 'Food');
    salary = makeCategory(name: 'Salary', isExpense: false);
    vault = makeCategory(name: '#Vault');
    for (final c in [food, salary, vault]) {
      services.categories.add(c);
    }
  });
  tearDown(() => store.close());

  group('confirm', () {
    test('with a sealed category throws and the entry stays pending', () {
      final pending = pendingIn(vault);
      final before = snapshotAll(store);

      expect(
        () => services.smsImportService.confirm(
          pending.id,
          valuesFor(pending, vault),
        ),
        throwsA(isA<SealedCategory>()),
      );

      expect(snapshotAll(store), before);
      expect(statusOf(pending), PendingEntryStatus.pending);
      expect(entryCount(), 0);
      expect(usages(vault), 0);
    });

    test('with a sealed category saves no rule either', () {
      final pending = pendingIn(vault);

      expect(
        () => services.smsImportService.confirm(
          pending.id,
          valuesFor(pending, vault),
          ruleKeyword: 'SHOP',
        ),
        throwsA(isA<SealedCategory>()),
      );

      expect(services.sms.allRules(), isEmpty);
      expect(statusOf(pending), PendingEntryStatus.pending);
    });

    test('a category the user picked and that is sealed is refused', () {
      final pending = pendingIn(food);
      final before = snapshotAll(store);

      expect(
        () => services.smsImportService.confirm(
          pending.id,
          valuesFor(pending, vault),
        ),
        throwsA(isA<SealedCategory>()),
      );

      expect(snapshotAll(store), before);
      expect(statusOf(pending), PendingEntryStatus.pending);
    });

    test('a category sealed by a rename is refused until it is unsealed', () {
      final pending = pendingIn(food);
      rename(food, '#Food');

      expect(
        () => services.smsImportService.confirm(
          pending.id,
          valuesFor(pending, food),
        ),
        throwsA(isA<SealedCategory>()),
      );
      expect(statusOf(pending), PendingEntryStatus.pending);

      rename(food, 'Food');
      services.smsImportService.confirm(pending.id, valuesFor(pending, food));

      expect(statusOf(pending), PendingEntryStatus.confirmed);
      expect(entryCount(), 1);
    });

    test('with a plain category still creates the entry and the rule', () {
      final pending = pendingIn(food);

      services.smsImportService.confirm(
        pending.id,
        valuesFor(pending, food),
        ruleKeyword: 'SHOP',
      );

      final entry = services.entries.find(const EntryQuery()).single;
      expect(entry.amountMinor, -529);
      expect(entry.category.targetId, food.id);
      expect(usages(food), 1);
      expect(statusOf(pending), PendingEntryStatus.confirmed);
      expect(services.sms.allRules().single.keyword, 'SHOP');
    });
  });

  group('confirm all', () {
    test('skips the entries in a sealed category and confirms the rest', () {
      final plain = pendingIn(food);
      final sealed = pendingIn(vault);
      final income = pendingIn(salary, isExpense: false, minor: 2000);
      final uncategorized = pendingIn(null);

      final confirmed = services.smsImportService.confirmAll(
        currencyCode: 'BYN',
      );

      expect(confirmed, 2);
      expect(statusOf(plain), PendingEntryStatus.confirmed);
      expect(statusOf(income), PendingEntryStatus.confirmed);
      expect(statusOf(sealed), PendingEntryStatus.pending);
      expect(statusOf(uncategorized), PendingEntryStatus.pending);
      expect(entryCount(), 2);
      expect(usages(food), 1);
      expect(usages(salary), 1);
      expect(usages(vault), 0);
    });

    test('with only sealed entries confirms nothing and does not throw', () {
      pendingIn(vault);
      pendingIn(vault);
      final before = snapshotAll(store);

      expect(services.smsImportService.confirmAll(currencyCode: 'BYN'), 0);

      expect(snapshotAll(store), before);
    });

    test('follows the name: a renamed category is skipped, then confirmed', () {
      final first = pendingIn(food);
      final second = pendingIn(food);
      rename(food, '#Food');

      expect(services.smsImportService.confirmAll(currencyCode: 'BYN'), 0);
      expect(statusOf(first), PendingEntryStatus.pending);
      expect(statusOf(second), PendingEntryStatus.pending);

      rename(food, 'Food');
      expect(services.smsImportService.confirmAll(currencyCode: 'BYN'), 2);
      expect(statusOf(first), PendingEntryStatus.confirmed);
      expect(statusOf(second), PendingEntryStatus.confirmed);
      expect(usages(food), 2);
    });

    test('still leaves the wrong-type entries pending', () {
      final wrong = pendingIn(salary);
      final plain = pendingIn(food);

      expect(services.smsImportService.confirmAll(currencyCode: 'BYN'), 1);

      expect(statusOf(wrong), PendingEntryStatus.pending);
      expect(statusOf(plain), PendingEntryStatus.confirmed);
    });
  });
}
