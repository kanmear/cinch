import 'package:flutter_test/flutter_test.dart';

import 'package:budgetron/db/category_repository.dart';
import 'package:budgetron/logic/category/category_service.dart';
import 'package:budgetron/objectbox.g.dart';

import '../../support/fixtures.dart';
import '../../support/store_snapshot.dart';
import '../../support/test_services.dart';
import '../../support/test_store.dart';

void main() {
  late Store store;
  late TestServices services;
  late CategoryService service;
  late CategoryRepository categories;

  setUp(() {
    store = openTestStore();
    services = TestServices(store);
    service = services.categoryService;
    categories = services.categories;
  });
  tearDown(() => store.close());

  Matcher duplicateOf(String name) =>
      throwsA(isA<DuplicateCategoryName>().having((e) => e.name, 'name', name));

  group('adding', () {
    test('names that differ only by spaces collide', () {
      service.createCategory(makeCategory(name: 'Eating Out'));
      final before = snapshotAll(store);

      for (final name in [
        'EatingOut',
        'Eating  Out',
        ' Eating Out',
        'Eating Out ',
        'E a t i n g O u t',
      ]) {
        expect(
          () => service.createCategory(makeCategory(name: name)),
          duplicateOf(name),
        );
      }

      expect(snapshotAll(store), before);
      expect(categories.find(), hasLength(1));
    });

    test('the collision is symmetric', () {
      service.createCategory(makeCategory(name: 'EatingOut'));

      expect(
        () => service.createCategory(makeCategory(name: 'Eating Out')),
        duplicateOf('Eating Out'),
      );
      expect(categories.find().map((c) => c.name), ['EatingOut']);
    });

    test('a collision across expense and income still collides', () {
      service.createCategory(makeCategory(name: 'Side Gig'));

      expect(
        () => service.createCategory(
          makeCategory(name: 'SideGig', isExpense: false),
        ),
        duplicateOf('SideGig'),
      );
    });

    test('the repository refuses it too', () {
      categories.add(makeCategory(name: 'Eating Out'));

      expect(
        () => categories.add(makeCategory(name: 'EatingOut')),
        duplicateOf('EatingOut'),
      );
      expect(categories.find(), hasLength(1));
    });

    test('an exact repeat still collides', () {
      service.createCategory(makeCategory(name: 'Food'));

      expect(
        () => service.createCategory(makeCategory(name: 'Food')),
        duplicateOf('Food'),
      );
    });

    test('names that differ in case stay distinct, with or without spaces', () {
      service.createCategory(makeCategory(name: 'EatingOut'));
      service.createCategory(makeCategory(name: 'eatingout'));
      service.createCategory(makeCategory(name: 'Eating out'));

      expect(categories.find().map((c) => c.name).toSet(), {
        'EatingOut',
        'eatingout',
        'Eating out',
      });
    });

    test('names that differ in letters stay distinct', () {
      service.createCategory(makeCategory(name: 'Eating Out'));
      service.createCategory(makeCategory(name: 'Eating In'));
      service.createCategory(makeCategory(name: 'Eating'));

      expect(categories.find(), hasLength(3));
    });
  });

  group('renaming', () {
    test('to a spelling of another name is refused and changes nothing', () {
      service.createCategory(makeCategory(name: 'Eating Out'));
      final fun = makeCategory(name: 'Fun');
      service.createCategory(fun);
      final before = snapshotAll(store);

      for (final name in ['EatingOut', 'Eating  Out', ' EatingOut ']) {
        fun.name = name;
        expect(() => service.updateCategory(fun), duplicateOf(name));
      }

      expect(snapshotAll(store), before);
      expect(categories.get(fun.id)!.name, 'Fun');
    });

    test('a refused rename keeps the other fields stored as they were', () {
      service.createCategory(makeCategory(name: 'Eating Out'));
      final fun = makeCategory(name: 'Fun', color: 'ff112233');
      service.createCategory(fun);
      final before = snapshotAll(store);

      fun
        ..name = 'EatingOut'
        ..color = 'ff445566';

      expect(() => service.updateCategory(fun), duplicateOf('EatingOut'));
      expect(snapshotAll(store), before);
    });

    test('to a name differing in case is allowed', () {
      service.createCategory(makeCategory(name: 'Eating Out'));
      final fun = makeCategory(name: 'Fun');
      service.createCategory(fun);

      fun.name = 'eatingout';
      service.updateCategory(fun);

      expect(categories.get(fun.id)!.name, 'eatingout');
    });

    test('a category can take another spelling of its own name', () {
      final category = makeCategory(name: 'Eating Out');
      service.createCategory(category);

      category.name = 'EatingOut';
      service.updateCategory(category);
      expect(categories.get(category.id)!.name, 'EatingOut');

      category.name = 'Eating   Out';
      service.updateCategory(category);
      expect(categories.get(category.id)!.name, 'Eating   Out');
      expect(categories.find(), hasLength(1));
    });

    test('saving other fields under the same name works', () {
      final category = makeCategory(name: 'Eating Out');
      service.createCategory(category);

      category.color = 'ff0a0b0c';
      service.updateCategory(category);

      expect(categories.get(category.id)!.color, 'ff0a0b0c');
    });

    test('a freed spelling can be used after a rename', () {
      final category = makeCategory(name: 'Eating Out');
      service.createCategory(category);
      category.name = 'Dining';
      service.updateCategory(category);

      service.createCategory(makeCategory(name: 'EatingOut'));

      expect(categories.find().map((c) => c.name).toSet(), {
        'Dining',
        'EatingOut',
      });
    });

    test('after a delete the name is free again in any spelling', () {
      final category = makeCategory(name: 'Eating Out');
      service.createCategory(category);
      service.deleteCategory(category);

      service.createCategory(makeCategory(name: 'EatingOut'));

      expect(categories.find().map((c) => c.name), ['EatingOut']);
    });
  });
}
