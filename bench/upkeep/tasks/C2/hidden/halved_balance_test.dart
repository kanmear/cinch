import 'package:flutter_test/flutter_test.dart';

import 'package:budgetron/db/entry_repository.dart';
import 'package:budgetron/logic/account/account_service.dart';
import 'package:budgetron/logic/entry/category_total.dart';
import 'package:budgetron/logic/entry/entry_service.dart';
import 'package:budgetron/models/account/account.dart';
import 'package:budgetron/models/category/category.dart';
import 'package:budgetron/models/entry.dart';
import 'package:budgetron/models/money.dart';
import 'package:budgetron/objectbox.g.dart';

import '../../support/fixtures.dart';
import '../../support/test_services.dart';
import '../../support/test_store.dart';

void main() {
  late Store store;
  late TestServices services;
  late AccountService accounts;
  late EntryService entries;
  late Account cash;
  late EntryCategory food;
  late EntryCategory salary;

  setUp(() {
    store = openTestStore();
    services = TestServices(store);
    accounts = services.accountService;
    entries = services.entryService;

    cash = makeAccount(name: 'Cash');
    accounts.createAccount(cash, balance: const Money(1000));
    food = makeCategory(name: 'Food');
    salary = makeCategory(name: 'Salary', isExpense: false);
    services.categories.add(food);
    services.categories.add(salary);
  });
  tearDown(() => store.close());

  /// Creates an entry of [minor] (magnitude) in [category] on the Cash account.
  Entry add(
    int minor,
    EntryCategory category, {
    bool halved = false,
    DateTime? at,
  }) {
    final entry = Entry(
      amountMinor: minor,
      dateTime: at ?? DateTime(2024, 3, 5),
    )..account.target = cash;
    if (halved) entry.isHalved = true;
    entries.createEntry(entry, category);
    return entry;
  }

  test('an entry is not halved unless it says so', () {
    expect(Entry(dateTime: DateTime(2024, 3, 5)).isHalved, isFalse);

    final entry = add(300, food);

    expect(store.box<Entry>().get(entry.id)!.isHalved, isFalse);
    expect(accounts.balanceOf(cash.id), const Money(700));
  });

  test('a halved expense counts at half its amount in the balance', () {
    add(1000, food, halved: true);

    expect(accounts.balanceOf(cash.id), const Money(500));
  });

  test('a halved income counts at half its amount in the balance', () {
    add(600, salary, halved: true);

    expect(accounts.balanceOf(cash.id), const Money(1300));
  });

  test('an odd halved expense truncates toward zero: -1001 counts -500', () {
    add(1001, food, halved: true);

    expect(accounts.balanceOf(cash.id), const Money(500));
  });

  test('an odd halved income truncates toward zero: 999 counts 499', () {
    add(999, salary, halved: true);

    expect(accounts.balanceOf(cash.id), const Money(1499));
  });

  test('tiny halved amounts: 1 counts 0, 3 counts 1, for both signs', () {
    add(1, food, halved: true);
    expect(accounts.balanceOf(cash.id), const Money(1000));

    add(3, food, halved: true);
    expect(accounts.balanceOf(cash.id), const Money(999));

    add(1, salary, halved: true);
    expect(accounts.balanceOf(cash.id), const Money(999));

    add(3, salary, halved: true);
    expect(accounts.balanceOf(cash.id), const Money(1000));
  });

  test('each halved entry is rounded on its own, not their sum', () {
    add(3, food, halved: true);
    add(3, food, halved: true);
    // two times -3 ~/ 2 = -1: sum -2 (halving the sum -6 would give -3)
    expect(accounts.balanceOf(cash.id), const Money(998));

    add(3, salary, halved: true);
    add(3, salary, halved: true);
    expect(accounts.balanceOf(cash.id), const Money(1000));
  });

  test('halved and whole entries mix in one balance', () {
    add(1001, food, halved: true); // -500
    add(999, salary, halved: true); // +499
    add(7, food); // -7
    add(250, salary); // +250

    expect(
      accounts.balanceOf(cash.id),
      const Money(1000 - 500 + 499 - 7 + 250),
    );
  });

  test('a halved entry of another account does not touch this balance', () {
    final card = makeAccount(name: 'Card');
    accounts.createAccount(card, balance: Money.zero);
    entries.createEntry(
      Entry(amountMinor: 100, dateTime: DateTime(2024, 3, 5))
        ..account.target = card
        ..isHalved = true,
      food,
    );

    expect(accounts.balanceOf(cash.id), const Money(1000));
    expect(accounts.balanceOf(card.id), const Money(-50));
  });

  test('deleting a halved entry takes back only its half', () {
    final entry = add(1001, food, halved: true);
    add(40, food);
    expect(accounts.balanceOf(cash.id), const Money(460));

    entries.deleteEntry(entry);

    expect(accounts.balanceOf(cash.id), const Money(960));
  });

  test('typing a balance shows exactly that balance with halved entries', () {
    add(1001, food, halved: true);
    add(999, salary, halved: true);

    accounts.updateAccount(cash, balance: const Money(5000));

    expect(accounts.balanceOf(cash.id), const Money(5000));
  });

  test('watchBalance emits the halved balance', () async {
    add(1001, food, halved: true);

    expect(await accounts.watchBalance(cash.id).first, const Money(500));
  });

  test('the stored amount stays whole', () {
    final expense = add(1001, food, halved: true);
    final income = add(999, salary, halved: true);

    final storedExpense = store.box<Entry>().get(expense.id)!;
    final storedIncome = store.box<Entry>().get(income.id)!;

    expect(storedExpense.amountMinor, -1001);
    expect(storedExpense.isHalved, isTrue);
    expect(storedIncome.amountMinor, 999);
    expect(storedIncome.isHalved, isTrue);
  });

  test('lists and totals keep showing the whole amount', () {
    add(1001, food, halved: true);
    add(999, salary, halved: true);

    final listed = services.entries.find(const EntryQuery());
    expect(listed.map((e) => e.amountMinor).toSet(), {-1001, 999});

    final totals = totalsOf(listed.where((e) => e.amountMinor < 0), {
      food.id: food,
      salary.id: salary,
    });
    expect(totals.single.amount, const Money(1001));
  });

  test('editing a halved entry elsewhere keeps its amount whole', () {
    final entry = add(1001, food, halved: true);

    entries.update(
      entry.id,
      EntryUpdate(
        amount: const Money(2001),
        dateTime: DateTime(2024, 3, 6),
        category: food,
        account: cash,
      ),
    );

    expect(store.box<Entry>().get(entry.id)!.amountMinor, -2001);
  });
}
