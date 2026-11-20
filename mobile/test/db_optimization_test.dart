import 'package:flutter_test/flutter_test.dart';
import 'package:sqflite_common_ffi/sqflite_ffi.dart';
import 'package:import_manager_mobile/database/db_helper.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  setUpAll(() {
    sqfliteFfiInit();
    databaseFactory = databaseFactoryFfi;
  });

  test('Database indexes and performance PRAGMAs initialization', () async {
    final db = await DBHelper.instance.database;
    expect(db.isOpen, isTrue);

    // Verify performance indexes exist
    final indexRows = await db.rawQuery(
      "SELECT name FROM sqlite_master WHERE type='index' AND name LIKE 'idx_mob_%'",
    );
    final indexNames = indexRows.map((r) => r['name'] as String).toSet();

    expect(indexNames.contains('idx_mob_shipments_status'), isTrue);
    expect(indexNames.contains('idx_mob_shipments_inv'), isTrue);
    expect(indexNames.contains('idx_mob_items_part'), isTrue);
    expect(indexNames.contains('idx_mob_items_desc'), isTrue);
    expect(indexNames.contains('idx_mob_invoices_shipment'), isTrue);
    expect(indexNames.contains('idx_mob_inv_lines_inv'), isTrue);

    // Verify PRAGMA optimize runs cleanly
    await DBHelper.instance.optimizeDatabase();
  });
}
