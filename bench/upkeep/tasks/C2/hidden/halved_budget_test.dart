import 'package:clock/clock.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:budgetron/db/objectbox/object_box_category_repository.dart';
import 'package:budgetron/models/category/category.dart';
import 'package:budgetron/models/entry.dart';
import 'package:budgetron/models/enums/budget_period.dart';
import 'package:budgetron/models/money.dart';
import 'package:budgetron/objectbox.g.dart';

import '../../support/fixtures.dart';
import '../../support/test_services.dart';
import '../../support/test_store.dart';

void main() {
  late Store store;
  late ObjectBoxCategoryRepository categories;
  late EntryCategory food;

  setUp(() {
    store = openTestStore();
    categories = ObjectBoxCategoryRepository(store);
    food = makeCategory(name: 'Food');
    categories.add(food);
  });
  tearDown(() => store.close());

  TestServices at(DateTime now) => TestServices(
    store,
    clock: Clock.fixed(now),
    categoryRepository: categories,
  );

  void spend(
    TestServices services,
    int minor,
    DateTime when, {
    bool halved = false,
    EntryCategory? category,
  }) {
    final entry = Entry(amountMinor: minor, dateTime: when);
    if (halved) entry.isHalved = true;
    services.entryService.createEntry(entry, category ?? food);
  }

  List<int> spentPerPeriod(TestServices services) => services.budgetService
      .summaries()
      .single
      .periods
      .map((p) => p.spent.minor)
      .toList();

  TestServices monthlyBudget() {
    final services = at(DateTime(2024, 3, 10));
    services.budgetService.create(
      category: food,
      target: const Money(100000),
      period: BudgetPeriod.month,
    );
    return services;
  }

  test('a halved expense counts at half its amount in spent', () {
    final services = monthlyBudget();
    spend(services, 1000, DateTime(2024, 3, 5), halved: true);

    expect(spentPerPeriod(services), [500]);
  });

  test('an odd halved expense truncates toward zero: 1001 counts 500', () {
    final services = monthlyBudget();
    spend(services, 1001, DateTime(2024, 3, 5), halved: true);

    expect(spentPerPeriod(services), [500]);
  });

  test('tiny halved expenses: 1 counts 0, 3 counts 1', () {
    final services = monthlyBudget();
    spend(services, 1, DateTime(2024, 3, 5), halved: true);
    spend(services, 3, DateTime(2024, 3, 6), halved: true);

    expect(spentPerPeriod(services), [1]);
  });

  test('each halved entry is rounded on its own, not their sum', () {
    final services = monthlyBudget();
    spend(services, 3, DateTime(2024, 3, 5), halved: true);
    spend(services, 3, DateTime(2024, 3, 6), halved: true);

    expect(spentPerPeriod(services), [2]);
  });

  test('halved and whole entries mix across periods, boundaries included', () {
    final services = monthlyBudget();
    spend(services, 1001, DateTime(2024, 3, 5), halved: true); // 500
    spend(services, 3, DateTime(2024, 3, 6), halved: true); // 1
    spend(services, 2000, DateTime(2024, 3, 7)); // 2000
    spend(
      services,
      5,
      DateTime(2024, 3, 31, 23, 59, 59, 999),
      halved: true,
    ); // 2
    spend(services, 100, DateTime(2024, 4), halved: true); // 50, April
    spend(services, 7, DateTime(2024, 4, 9)); // 7

    final later = at(DateTime(2024, 4, 15));

    expect(spentPerPeriod(later), [2503, 57]);
  });

  test('a halved income does not count as spent', () {
    final services = monthlyBudget();
    final salary = makeCategory(name: 'Salary', isExpense: false);
    categories.add(salary);
    spend(services, 100, DateTime(2024, 3, 5));
    spend(services, 900, DateTime(2024, 3, 6), halved: true, category: salary);

    expect(spentPerPeriod(services), [100]);
  });

  test('a halved entry of another category does not count', () {
    final services = monthlyBudget();
    final rent = makeCategory(name: 'Rent');
    categories.add(rent);
    spend(services, 100, DateTime(2024, 3, 5));
    spend(services, 900, DateTime(2024, 3, 6), halved: true, category: rent);

    expect(spentPerPeriod(services), [100]);
  });

  test('the stored amount stays whole', () {
    final services = monthlyBudget();
    spend(services, 1001, DateTime(2024, 3, 5), halved: true);
    spentPerPeriod(services);

    final stored = store.box<Entry>().getAll().single;
    expect(stored.amountMinor, -1001);
    expect(stored.isHalved, isTrue);
  });

  test('a watched summary follows halved entries', () async {
    final services = monthlyBudget();
    spend(services, 1001, DateTime(2024, 3, 5), halved: true);

    final summaries = await services.budgetService.watchSummaries().first;

    expect(summaries.single.current!.spent, const Money(500));
  });
}
