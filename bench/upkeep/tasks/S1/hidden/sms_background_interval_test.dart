import 'dart:async';

import 'package:clock/clock.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

import 'package:budgetron/cubit/sms_scan_cubit.dart';
import 'package:budgetron/logic/sms/sms_import_service.dart';
import 'package:budgetron/logic/sms/sms_source.dart';

class _MockSmsImportService extends Mock implements SmsImportService {}

class _MockSmsSource extends Mock implements SmsSource {}

void main() {
  late _MockSmsImportService service;
  late _MockSmsSource source;
  late DateTime now;
  final since = DateTime(2024, 5, 1, 8);

  SmsScanCubit newCubit() {
    final cubit = SmsScanCubit(
      service,
      source,
      Clock(() => now),
      digits: () => 2,
      currencyCode: () => 'BYN',
    );
    addTearDown(cubit.close);
    return cubit;
  }

  void verifyScans(int times) {
    verify(
      () => service.scan(
        any(),
        digits: any(named: 'digits'),
        currencyCode: any(named: 'currencyCode'),
      ),
    ).called(times);
  }

  void verifyNoScan() {
    verifyNever(
      () => service.scan(
        any(),
        digits: any(named: 'digits'),
        currencyCode: any(named: 'currencyCode'),
      ),
    );
  }

  setUp(() {
    now = DateTime(2024, 5, 2, 12);
    service = _MockSmsImportService();
    source = _MockSmsSource();
    when(() => source.isSupported).thenReturn(true);
    when(() => source.hasPermission()).thenAnswer((_) async => true);
    when(() => source.requestPermission()).thenAnswer((_) async => true);
    when(() => service.lastScanAt).thenReturn(since);
    when(
      () => service.scan(
        any(),
        digits: any(named: 'digits'),
        currencyCode: any(named: 'currencyCode'),
      ),
    ).thenAnswer((_) async => 3);
  });

  group('background scan interval', () {
    test('the first background scan runs', () async {
      final cubit = newCubit();

      await cubit.scanInBackground();

      verifyScans(1);
      expect(cubit.state.count, 3);
    });

    test('is skipped one minute after the last scan', () async {
      final cubit = newCubit();

      await cubit.scanInBackground();
      verifyScans(1);

      now = now.add(const Duration(minutes: 1));
      await cubit.scanInBackground();

      verifyNoScan();
    });

    test('is skipped at 5 and 12 minutes 59 seconds', () async {
      final cubit = newCubit();

      await cubit.scanInBackground();
      verifyScans(1);

      now = now.add(const Duration(minutes: 5));
      await cubit.scanInBackground();
      now = now.add(const Duration(minutes: 7, seconds: 59));
      await cubit.scanInBackground();

      verifyNoScan();
    });

    test('runs again once 13 minutes have passed', () async {
      final cubit = newCubit();

      await cubit.scanInBackground();
      verifyScans(1);

      now = now.add(const Duration(minutes: 13));
      await cubit.scanInBackground();

      verifyScans(1);
    });

    test('the 13 minutes count from the end of the last scan', () async {
      final cubit = newCubit();

      await cubit.scanInBackground();
      verifyScans(1);

      now = now.add(const Duration(minutes: 13));
      await cubit.scanInBackground();
      verifyScans(1);

      now = now.add(const Duration(minutes: 12));
      await cubit.scanInBackground();
      verifyNoScan();

      now = now.add(const Duration(minutes: 1));
      await cubit.scanInBackground();
      verifyScans(1);
    });

    test('a failed scan still throttles for 13 minutes', () async {
      when(
        () => service.scan(
          any(),
          digits: any(named: 'digits'),
          currencyCode: any(named: 'currencyCode'),
        ),
      ).thenThrow(StateError('boom'));
      final cubit = newCubit();

      await cubit.scanInBackground();
      verifyScans(1);

      now = now.add(const Duration(minutes: 12));
      await cubit.scanInBackground();
      verifyNoScan();

      now = now.add(const Duration(minutes: 1));
      await cubit.scanInBackground();
      verifyScans(1);
    });

    test('never runs two scans at once', () async {
      final read = Completer<int>();
      when(
        () => service.scan(
          any(),
          digits: any(named: 'digits'),
          currencyCode: any(named: 'currencyCode'),
        ),
      ).thenAnswer((_) => read.future);
      final cubit = newCubit();

      final first = cubit.scanInBackground();
      await pumpEventQueue();
      expect(cubit.state.isScanning, isTrue);

      now = now.add(const Duration(minutes: 20));
      await cubit.scanInBackground();
      read.complete(1);
      await first;

      verifyScans(1);
      expect(cubit.state.isScanning, isFalse);
    });

    test('still needs the permission and a scan point', () async {
      when(() => source.hasPermission()).thenAnswer((_) async => false);
      final cubit = newCubit();

      await cubit.scanInBackground();

      verifyNoScan();
      verifyNever(() => source.requestPermission());

      when(() => source.hasPermission()).thenAnswer((_) async => true);
      when(() => service.lastScanAt).thenReturn(null);
      await cubit.scanInBackground();

      verifyNoScan();

      when(() => service.lastScanAt).thenReturn(since);
      await cubit.scanInBackground();

      verifyScans(1);
    });
  });
}
