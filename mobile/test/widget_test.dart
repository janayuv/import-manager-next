import 'package:flutter_test/flutter_test.dart';
import 'package:sqflite_common_ffi/sqflite_ffi.dart';
import 'package:import_manager_mobile/main.dart';

void main() {
  setUpAll(() {
    sqfliteFfiInit();
    databaseFactory = databaseFactoryFfi;
  });

  testWidgets('App initialization test', (WidgetTester tester) async {
    await tester.pumpWidget(const ImportManagerMobileApp());
    await tester.pump(const Duration(milliseconds: 200));
    expect(find.text('Upcoming Shipments'), findsOneWidget);
  });
}
