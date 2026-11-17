import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:path/path.dart' as p;
import 'package:sqflite/sqflite.dart';

import '../models/shipment.dart';
import '../models/item_part.dart';
import '../models/invoice.dart';
import '../models/landed_cost.dart';

class DBHelper {
  static final DBHelper instance = DBHelper._init();
  static Database? _database;
  static Completer<Database>? _initCompleter;

  DBHelper._init();

  Future<Database> get database async {
    if (_database != null && _database!.isOpen) return _database!;
    if (_initCompleter != null) {
      return _initCompleter!.future;
    }
    final completer = Completer<Database>();
    _initCompleter = completer;
    try {
      _database = await _initDB('import_manager_mobile.db');
      completer.complete(_database);
      return _database!;
    } catch (e, st) {
      completer.completeError(e, st);
      rethrow;
    } finally {
      _initCompleter = null;
    }
  }

  Future<Database> _initDB(String filePath) async {
    final dbPath = await getDatabasesPath();
    final path = p.join(dbPath, filePath);

    final file = File(path);
    final exists = await file.exists();
    // Bundled desktop database is 2.99MB. If file is < 50KB or empty, it must be seeded.
    bool needSeed = !exists || (await file.length()) < 50000;

    if (exists && !needSeed) {
      // Test if shipments table exists and has active records
      try {
        final testDb = await openDatabase(path, readOnly: true);
        final tables = await testDb.rawQuery(
          "SELECT name FROM sqlite_master WHERE type='table' AND name='shipments'",
        );
        if (tables.isEmpty) {
          needSeed = true;
        } else {
          final countRes = await testDb.rawQuery("SELECT count(*) as cnt FROM shipments WHERE deleted_at IS NULL");
          final cnt = Sqflite.firstIntValue(countRes) ?? 0;
          if (cnt == 0) {
            needSeed = true;
          }
        }
        await testDb.close();
      } catch (e) {
        debugPrint('DBHelper existing DB test: $e, will re-seed');
        needSeed = true;
      }
    }

    if (needSeed) {
      debugPrint('DBHelper: Seeding database from bundled asset...');
      try {
        await Directory(dbPath).create(recursive: true);
        try {
          if (await file.exists()) await file.delete();
          final walFile = File('$path-wal');
          if (await walFile.exists()) await walFile.delete();
          final shmFile = File('$path-shm');
          if (await shmFile.exists()) await shmFile.delete();
        } catch (_) {}

        final data = await rootBundle.load('assets/default_import_manager.db');
        final bytes = data.buffer.asUint8List(data.offsetInBytes, data.lengthInBytes);
        await file.writeAsBytes(bytes, flush: true);
        debugPrint('DBHelper: Successfully pre-seeded database from assets (${bytes.length} bytes)');
      } catch (e) {
        debugPrint('DBHelper: Asset DB copy skipped: $e');
      }
    }

    final db = await openDatabase(path);
    await _ensureSchema(db);
    return db;
  }

  Future<void> _ensureSchema(Database db) async {
    try {
      final tables = await db.rawQuery(
        "SELECT name FROM sqlite_master WHERE type='table' AND name='shipments'",
      );
      if (tables.isEmpty) {
        await _createDB(db);
      }
    } catch (e) {
      debugPrint('DBHelper schema check error: $e');
    }
  }

  Future<void> _createDB(Database db) async {
    // 1. Suppliers
    await db.execute('''
      CREATE TABLE IF NOT EXISTS suppliers (
        id TEXT PRIMARY KEY,
        supplier_name TEXT NOT NULL,
        short_name TEXT,
        country TEXT NOT NULL,
        email TEXT,
        phone TEXT,
        beneficiary_name TEXT,
        bank_name TEXT,
        branch TEXT,
        bank_address TEXT,
        account_no TEXT,
        iban TEXT,
        swift_code TEXT,
        is_active BOOLEAN NOT NULL DEFAULT 1,
        deleted_at TEXT
      )
    ''');

    // 2. Shipments
    await db.execute('''
      CREATE TABLE IF NOT EXISTS shipments (
        id TEXT PRIMARY KEY,
        supplier_id TEXT NOT NULL,
        invoice_number TEXT NOT NULL,
        invoice_date TEXT NOT NULL,
        goods_category TEXT NOT NULL,
        invoice_value REAL NOT NULL DEFAULT 0.0,
        invoice_currency TEXT NOT NULL DEFAULT 'USD',
        incoterm TEXT NOT NULL DEFAULT '',
        shipment_mode TEXT,
        shipment_type TEXT,
        bl_awb_number TEXT,
        bl_awb_date TEXT,
        vessel_name TEXT,
        container_number TEXT,
        gross_weight_kg REAL DEFAULT 0.0,
        etd TEXT,
        eta TEXT,
        status TEXT DEFAULT 'in-transit',
        date_of_delivery TEXT,
        is_frozen BOOLEAN NOT NULL DEFAULT 0,
        deleted_at TEXT
      )
    ''');

    // 3. Items (Parts Master & Customs Duty Structure)
    await db.execute('''
      CREATE TABLE IF NOT EXISTS items (
        id TEXT PRIMARY KEY,
        part_number TEXT NOT NULL UNIQUE,
        item_description TEXT NOT NULL,
        unit TEXT NOT NULL DEFAULT 'PCS',
        currency TEXT NOT NULL DEFAULT 'USD',
        unit_price REAL NOT NULL DEFAULT 0.0,
        hsn_code TEXT NOT NULL,
        supplier_id TEXT,
        is_active BOOLEAN NOT NULL DEFAULT 1,
        country_of_origin TEXT,
        bcd TEXT DEFAULT '7.5',
        sws TEXT DEFAULT '10',
        igst TEXT DEFAULT '18',
        technical_write_up TEXT,
        category TEXT,
        end_use TEXT,
        net_weight_kg REAL DEFAULT 0.0,
        purchase_uom TEXT,
        gross_weight_per_uom_kg REAL DEFAULT 0.0,
        photo_path TEXT,
        deleted_at TEXT
      )
    ''');

    // 4. Invoices
    await db.execute('''
      CREATE TABLE IF NOT EXISTS invoices (
        id TEXT PRIMARY KEY,
        shipment_id TEXT NOT NULL,
        status TEXT NOT NULL DEFAULT 'Finalized',
        deleted_at TEXT
      )
    ''');

    // 5. Invoice Line Items
    await db.execute('''
      CREATE TABLE IF NOT EXISTS invoice_line_items (
        id TEXT PRIMARY KEY,
        invoice_id TEXT NOT NULL,
        item_id TEXT NOT NULL,
        quantity REAL NOT NULL DEFAULT 0.0,
        unit_price REAL NOT NULL DEFAULT 0.0,
        deleted_at TEXT
      )
    ''');

    // 6. Landed Cost Report Table or View
    await db.execute('''
      CREATE TABLE IF NOT EXISTS ldc_cached_report (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        supplier TEXT,
        supplier_id TEXT,
        invoice_no TEXT,
        invoice_date TEXT,
        part_no TEXT,
        description TEXT,
        unit TEXT,
        qty REAL,
        unit_price REAL,
        assessable_value REAL,
        bcd_amount REAL,
        sws_amount REAL,
        igst_amount REAL,
        boe_interest REAL,
        expenses_total REAL,
        ldc_per_qty REAL
      )
    ''');

    // Seed sample data for immediate view-only experience
    await _seedInitialData(db);
  }

  Future<void> _seedInitialData(Database db) async {
    try {
      final countRes = await db.rawQuery("SELECT count(*) as cnt FROM shipments");
      final count = Sqflite.firstIntValue(countRes) ?? 0;
      if (count > 0) return; // DB already has data, do not seed!
    } catch (_) {}

    try {
      await db.rawInsert('''
        INSERT INTO suppliers (id, supplier_name, short_name, country, email, is_active)
        VALUES 
        ('sup-1', 'Shenzhen Precision Tech Co. Ltd', 'Shenzhen Precision', 'China', 'export@szprecision.cn', 1),
        ('sup-2', 'Kyoto Microelectronics Corp', 'Kyoto Micro', 'Japan', 'sales@kyoto-micro.jp', 1),
        ('sup-3', 'Frankfurt Automation GmbH', 'Frankfurt Auto', 'Germany', 'trade@ffm-auto.de', 1),
        ('sup-4', 'Taiwan Semiconductor Allied', 'TSA', 'Taiwan', 'intl@tsallied.tw', 1)
      ''');
    } catch (e) {
      debugPrint('Seed suppliers error: $e');
    }

    try {
      await db.rawInsert('''
        INSERT INTO items (id, part_number, item_description, unit, currency, unit_price, hsn_code, supplier_id, is_active, country_of_origin, bcd, sws, igst, category, end_use, technical_write_up)
        VALUES 
        ('it-1', 'PN-MICRO-8840', 'High Performance ARM Cortex-M7 Core Controller', 'PCS', 'USD', 24.50, '8542.31.00', 'sup-1', 1, 'China', '7.5', '10', '18', 'Electronic ICs', 'Industrial Automation Board', 'High-density QFP package with onboard flash memory'),
        ('it-2', 'PN-OPTIC-420', 'Multi-mode Fiber Optic Laser Transceiver 10Gbps', 'PCS', 'USD', 68.00, '8517.62.90', 'sup-2', 1, 'Japan', '10.0', '10', '18', 'Optoelectronics', 'Telecom Routing Units', 'SFP+ pluggable module with low power dissipation'),
        ('it-3', 'PN-SERVOMOTOR-75', 'Brushless DC Servo Motor 750W with Encoder', 'SET', 'EUR', 185.00, '8501.31.19', 'sup-3', 1, 'Germany', '7.5', '10', '18', 'Electrical Motors', 'Pick & Place Robots', 'Precision positioning servo motor with resolver feedback'),
        ('it-4', 'PN-CAPACITOR-ALU', 'Ultra Low ESR Aluminum Polymer Capacitors 470uF', 'NOS', 'USD', 0.42, '8532.22.00', 'sup-4', 1, 'Taiwan', '10.0', '10', '18', 'Passive Components', 'Power Supply Units', 'High ripple current handling up to 105C rating'),
        ('it-5', 'PN-SENSOR-PRESSURE', 'Differential Pressure Transducer 0-10 Bar', 'PCS', 'EUR', 42.00, '9026.20.00', 'sup-3', 1, 'Germany', '7.5', '10', '18', 'Sensors', 'Hydraulic Monitoring', 'Stainless steel diaphragm, 4-20mA analog output'),
        ('it-6', 'PN-RELAY-SOLID', 'Industrial Solid State Relay 40A 240VAC', 'PCS', 'USD', 12.80, '8536.49.00', 'sup-1', 1, 'China', '7.5', '10', '18', 'Switchgear', 'Control Panels', 'Opto-isolated switching with zero-cross turn-on')
      ''');
    } catch (e) {
      debugPrint('Seed items error: $e');
    }

    try {
      await db.rawInsert('''
        INSERT INTO shipments (id, supplier_id, invoice_number, invoice_date, goods_category, invoice_value, invoice_currency, incoterm, shipment_mode, shipment_type, bl_awb_number, bl_awb_date, vessel_name, container_number, gross_weight_kg, etd, eta, status)
        VALUES 
        ('sh-1', 'sup-1', 'INV-2026-0914', '2026-11-04', 'Electronic Components', 34200.00, 'USD', 'FOB', 'SEA', 'FCL', 'MEDUS482910', '2026-11-06', 'MSC VALENTINA / V.840', 'MSCU9482710', 4280.0, '2026-11-08', '2026-11-20', 'in-transit'),
        ('sh-2', 'sup-2', 'INV-KYO-8831', '2026-11-09', 'Optoelectronic Transceivers', 20400.00, 'USD', 'CIF', 'AIR', 'Air Cargo', 'AWB-618-3829104', '2026-11-10', 'Lufthansa Cargo LH822', 'AKE-48291LH', 320.0, '2026-11-12', '2026-11-18', 'customs-clearance'),
        ('sh-3', 'sup-3', 'INV-FFM-4402', '2026-11-12', 'Servo Motors & Sensors', 55500.00, 'EUR', 'FOB', 'SEA', 'LCL', 'HLCUHAM938210', '2026-11-14', 'HAPAG AL JASRAH', 'HLXU8392019', 2850.0, '2026-11-16', '2026-11-28', 'in-transit'),
        ('sh-4', 'sup-4', 'INV-TSA-1092', '2026-11-14', 'Passive Polymer Capacitors', 8400.00, 'USD', 'FOB', 'AIR', 'Air Cargo', 'AWB-297-8849201', '2026-11-15', 'Emirates SkyCargo EK924', 'PMC-99381EK', 145.0, '2026-11-17', '2026-11-21', 'docs-rcvd'),
        ('sh-5', 'sup-1', 'INV-2026-0881', '2026-10-18', 'Control Board ICs', 48000.00, 'USD', 'CIF', 'SEA', 'FCL', 'COSU62819302', '2026-10-20', 'COSCO SHIPPING PLANET', 'COSU8391024', 6200.0, '2026-10-22', '2026-11-10', 'ready-dly'),
        ('sh-6', 'sup-3', 'INV-FFM-3912', '2026-10-02', 'Industrial Sensors', 16800.00, 'EUR', 'FOB', 'AIR', 'Air Cargo', 'AWB-020-7749102', '2026-10-04', 'Lufthansa Cargo LH820', 'AKE-29182LH', 210.0, '2026-10-05', '2026-10-14', 'delivered')
      ''');
    } catch (e) {
      debugPrint('Seed shipments error: $e');
    }

    try {
      await db.rawInsert('''
        INSERT INTO invoices (id, shipment_id, status)
        VALUES 
        ('inv-1', 'sh-1', 'Finalized'),
        ('inv-2', 'sh-2', 'Finalized'),
        ('inv-3', 'sh-3', 'Finalized'),
        ('inv-4', 'sh-4', 'Draft'),
        ('inv-5', 'sh-5', 'Finalized')
      ''');
    } catch (e) {
      debugPrint('Seed invoices error: $e');
    }

    try {
      await db.rawInsert('''
        INSERT INTO invoice_line_items (id, invoice_id, item_id, quantity, unit_price)
        VALUES 
        ('li-1', 'inv-1', 'it-1', 1000.0, 24.50),
        ('li-2', 'inv-1', 'it-6', 750.0, 12.80),
        ('li-3', 'inv-2', 'it-2', 300.0, 68.00),
        ('li-4', 'inv-3', 'it-3', 250.0, 185.00),
        ('li-5', 'inv-3', 'it-5', 200.0, 42.00),
        ('li-6', 'inv-4', 'it-4', 20000.0, 0.42),
        ('li-7', 'inv-5', 'it-1', 1500.0, 24.50)
      ''');
    } catch (e) {
      debugPrint('Seed line items error: $e');
    }

    try {
      final ldcTableCheck = await db.rawQuery("SELECT name FROM sqlite_master WHERE type='table' AND name='ldc_cached_report'");
      if (ldcTableCheck.isNotEmpty) {
        await db.rawInsert('''
          INSERT INTO ldc_cached_report (supplier, supplier_id, invoice_no, invoice_date, part_no, description, unit, qty, unit_price, assessable_value, bcd_amount, sws_amount, igst_amount, boe_interest, expenses_total, ldc_per_qty)
          VALUES 
          ('Shenzhen Precision Tech Co. Ltd', 'sup-1', 'INV-2026-0881', '2026-10-18', 'PN-MICRO-8840', 'ARM Cortex-M7 Core Controller', 'PCS', 1500, 24.50, 3123750.0, 234281.25, 23428.13, 608662.69, 12450.0, 85600.0, 2319.67),
          ('Kyoto Microelectronics Corp', 'sup-2', 'INV-KYO-8831', '2026-11-09', 'PN-OPTIC-420', 'Fiber Optic Laser Transceiver 10Gbps', 'PCS', 300, 68.00, 1734000.0, 173400.00, 17340.00, 346453.20, 0.0, 42100.0, 6556.13),
          ('Frankfurt Automation GmbH', 'sup-3', 'INV-FFM-3912', '2026-10-02', 'PN-SERVOMOTOR-75', 'Brushless DC Servo Motor 750W', 'SET', 80, 185.00, 1332000.0, 99900.00, 9990.00, 259540.20, 0.0, 68400.0, 18878.63),
          ('Frankfurt Automation GmbH', 'sup-3', 'INV-FFM-3912', '2026-10-02', 'PN-SENSOR-PRESSURE', 'Pressure Transducer 0-10 Bar', 'PCS', 100, 42.00, 378000.0, 28350.00, 2835.00, 73653.30, 0.0, 19400.0, 4285.85)
        ''');
      }
    } catch (e) {
      debugPrint('Seed ldc error: $e');
    }
  }

  // ==========================================
  // 1. UPCOMING SHIPMENTS
  // ==========================================
  Future<List<Shipment>> getUpcomingShipments({
    String? search,
    String? statusFilter,
    String? sortBy,
  }) async {
    final db = await database;
    var query = '''
      SELECT s.*, sup.supplier_name
      FROM shipments s
      LEFT JOIN suppliers sup ON sup.id = s.supplier_id
      WHERE s.deleted_at IS NULL
    ''';

    final params = <dynamic>[];

    if (statusFilter != null && statusFilter.isNotEmpty && statusFilter != 'all') {
      query += ' AND LOWER(s.status) = ?';
      params.add(statusFilter.toLowerCase());
    } else {
      // Upcoming shipments exclude delivered and cancelled
      query += " AND LOWER(s.status) NOT IN ('delivered', 'cancelled', 'completed', 'closed')";
    }

    if (search != null && search.trim().isNotEmpty) {
      final term = '%${search.trim()}%';
      query += ''' AND (
        s.invoice_number LIKE ? OR 
        sup.supplier_name LIKE ? OR 
        s.vessel_name LIKE ? OR 
        s.container_number LIKE ? OR
        s.bl_awb_number LIKE ?
      )''';
      params.addAll([term, term, term, term, term]);
    }

    if (sortBy == 'etd') {
      query += ' ORDER BY s.etd ASC';
    } else {
      // Default order by ETA nearest first
      query += ' ORDER BY s.eta ASC';
    }

    final rows = await db.rawQuery(query, params);
    return rows.map((e) => Shipment.fromMap(e)).toList();
  }

  // ==========================================
  // 2. SHIPMENTS ETD - ETA TIMELINE
  // ==========================================
  Future<List<Shipment>> getShipmentsWithEtdEta({
    String? rangeFilter, // all, week, 15days, delayed
    String? search,
  }) async {
    final db = await database;
    var query = '''
      SELECT s.*, sup.supplier_name
      FROM shipments s
      LEFT JOIN suppliers sup ON sup.id = s.supplier_id
      WHERE s.deleted_at IS NULL AND s.eta IS NOT NULL AND s.eta != ''
    ''';

    final params = <dynamic>[];

    if (search != null && search.trim().isNotEmpty) {
      final term = '%${search.trim()}%';
      query += ' AND (s.invoice_number LIKE ? OR sup.supplier_name LIKE ? OR s.vessel_name LIKE ?)';
      params.addAll([term, term, term]);
    }

    query += ' ORDER BY s.eta ASC';
    final rows = await db.rawQuery(query, params);
    final all = rows.map((e) => Shipment.fromMap(e)).toList();

    if (rangeFilter == 'week') {
      return all.where((s) {
        final d = s.daysUntilEta;
        return d != null && d >= 0 && d <= 7;
      }).toList();
    } else if (rangeFilter == '15days') {
      return all.where((s) {
        final d = s.daysUntilEta;
        return d != null && d >= 0 && d <= 15;
      }).toList();
    } else if (rangeFilter == 'delayed') {
      return all.where((s) => s.isDelayed).toList();
    }

    return all;
  }

  // ==========================================
  // 3. PARTS & CUSTOMS DUTY STRUCTURE
  // ==========================================
  Future<List<ItemPart>> getItemsParts({
    String? search,
    String? category,
  }) async {
    final db = await database;
    var query = '''
      SELECT i.*, sup.supplier_name
      FROM items i
      LEFT JOIN suppliers sup ON sup.id = i.supplier_id
      WHERE i.deleted_at IS NULL
    ''';
    final params = <dynamic>[];

    if (category != null && category.isNotEmpty && category != 'All') {
      query += ' AND i.category = ?';
      params.add(category);
    }

    if (search != null && search.trim().isNotEmpty) {
      final term = '%${search.trim()}%';
      query += ''' AND (
        i.part_number LIKE ? OR 
        i.hsn_code LIKE ? OR 
        i.item_description LIKE ? OR 
        sup.supplier_name LIKE ?
      )''';
      params.addAll([term, term, term, term]);
    }

    query += ' ORDER BY i.part_number ASC';
    final rows = await db.rawQuery(query, params);
    return rows.map((e) => ItemPart.fromMap(e)).toList();
  }

  Future<List<String>> getPartCategories() async {
    final db = await database;
    final rows = await db.rawQuery('''
      SELECT DISTINCT category FROM items 
      WHERE category IS NOT NULL AND category != '' AND deleted_at IS NULL
      ORDER BY category ASC
    ''');
    return rows.map((r) => r['category'].toString()).toList();
  }

  // ==========================================
  // 4. INVOICE SEARCH & LINE ITEMS
  // ==========================================
  Future<List<Invoice>> searchInvoices({
    String? query,
    String? status,
  }) async {
    final db = await database;
    var sql = '''
      SELECT 
        inv.id,
        inv.shipment_id,
        inv.status,
        s.invoice_number,
        s.invoice_date,
        s.invoice_currency AS currency,
        s.invoice_value,
        sup.supplier_name,
        sup.id AS supplier_id,
        (SELECT COUNT(*) FROM invoice_line_items ili WHERE ili.invoice_id = inv.id AND ili.deleted_at IS NULL) AS line_count
      FROM invoices inv
      JOIN shipments s ON s.id = inv.shipment_id AND s.deleted_at IS NULL
      LEFT JOIN suppliers sup ON sup.id = s.supplier_id
      WHERE inv.deleted_at IS NULL
    ''';

    final params = <dynamic>[];

    if (status != null && status.isNotEmpty && status != 'All') {
      sql += ' AND inv.status = ?';
      params.add(status);
    }

    if (query != null && query.trim().isNotEmpty) {
      final term = '%${query.trim()}%';
      sql += ''' AND (
        s.invoice_number LIKE ? OR 
        sup.supplier_name LIKE ? OR 
        s.invoice_date LIKE ?
      )''';
      params.addAll([term, term, term]);
    }

    sql += ' ORDER BY s.invoice_date DESC';
    final rows = await db.rawQuery(sql, params);
    return rows.map((e) => Invoice.fromMap(e)).toList();
  }

  Future<Invoice?> getInvoiceDetail(String invoiceId) async {
    final db = await database;
    final invRows = await db.rawQuery('''
      SELECT 
        inv.id,
        inv.shipment_id,
        inv.status,
        s.invoice_number,
        s.invoice_date,
        s.invoice_currency AS currency,
        s.invoice_value,
        sup.supplier_name,
        sup.id AS supplier_id
      FROM invoices inv
      JOIN shipments s ON s.id = inv.shipment_id
      LEFT JOIN suppliers sup ON sup.id = s.supplier_id
      WHERE inv.id = ?
    ''', [invoiceId]);

    if (invRows.isEmpty) return null;

    final lineRows = await db.rawQuery('''
      SELECT 
        ili.id,
        ili.invoice_id,
        ili.item_id,
        ili.quantity,
        ili.unit_price,
        (ili.quantity * ili.unit_price) AS line_total,
        i.part_number,
        i.item_description,
        i.unit,
        i.hsn_code,
        CAST(COALESCE(i.bcd, '7.5') AS REAL) AS duty_percent,
        CAST(COALESCE(i.sws, '10.0') AS REAL) AS sws_percent,
        CAST(COALESCE(i.igst, '18.0') AS REAL) AS igst_percent
      FROM invoice_line_items ili
      JOIN items i ON i.id = ili.item_id
      WHERE ili.invoice_id = ? AND ili.deleted_at IS NULL
      ORDER BY i.part_number ASC
    ''', [invoiceId]);

    final lines = lineRows.map((e) => InvoiceLineItem.fromMap(e)).toList();
    return Invoice.fromMap(invRows.first, lines: lines);
  }

  // ==========================================
  // 5. LDC (LANDED DUTY COST) REPORT
  // ==========================================
  Future<List<LandedCostRow>> getLandedCostRows({
    String? search,
    String? supplierId,
  }) async {
    final db = await database;

    try {
      final rows = await db.rawQuery('''
        SELECT 
          supplier,
          supplier_id,
          invoice_no,
          invoice_date,
          part_no,
          description,
          unit,
          qty,
          unit_price,
          assessable_value,
          bcd_amount,
          sws_amount,
          igst_amount,
          boe_interest,
          expenses_total,
          ldc_per_qty
        FROM report_view
      ''');
      if (rows.isNotEmpty) {
        return _filterLdcRows(
          rows.map((e) => LandedCostRow.fromMap(e)).toList(),
          search: search,
          supplierId: supplierId,
        );
      }
    } catch (e) {
      debugPrint('report_view query skipped ($e), calculating from boe_calculations...');
    }

    // Fallback: Compute from boe_calculations table
    try {
      return await _calculateLdcRowsFromBoe(db, search: search, supplierId: supplierId);
    } catch (e) {
      debugPrint('Error calculating LDC rows: $e');
      return [];
    }
  }

  List<LandedCostRow> _filterLdcRows(
    List<LandedCostRow> list, {
    String? search,
    String? supplierId,
  }) {
    var result = list;
    if (supplierId != null && supplierId.isNotEmpty && supplierId != 'All') {
      result = result.where((r) => r.supplierId == supplierId).toList();
    }
    if (search != null && search.trim().isNotEmpty) {
      final q = search.trim().toLowerCase();
      result = result.where((r) =>
        r.invoiceNo.toLowerCase().contains(q) ||
        r.partNo.toLowerCase().contains(q) ||
        r.description.toLowerCase().contains(q) ||
        r.supplier.toLowerCase().contains(q)
      ).toList();
    }
    return result;
  }

  Future<List<LandedCostRow>> _calculateLdcRowsFromBoe(
    Database db, {
    String? search,
    String? supplierId,
  }) async {
    final boeRows = await db.rawQuery('''
      SELECT 
        bc.id,
        bc.shipment_id,
        bc.supplier_name,
        bc.invoice_number,
        s.invoice_date,
        s.supplier_id,
        bc.calculation_result_json,
        bc.form_values_json
      FROM boe_calculations bc
      LEFT JOIN shipments s ON s.id = bc.shipment_id
      WHERE bc.deleted_at IS NULL
      ORDER BY s.invoice_date DESC
    ''');

    final result = <LandedCostRow>[];

    for (final row in boeRows) {
      final resJsonStr = row['calculation_result_json']?.toString();
      if (resJsonStr == null || resJsonStr.isEmpty) continue;

      try {
        final resData = json.decode(resJsonStr);
        final items = resData['items'] ?? resData['calculatedItems'] as List? ?? [];
        final totalAssessable = (resData['totalAssessableValue'] as num?)?.toDouble() ?? 1.0;
        final interest = (resData['interest'] as num?)?.toDouble() ?? 0.0;
        final logistics = (resData['allocatedExpenses'] as num?)?.toDouble() ?? 0.0;

        for (final item in items) {
          final partNo = (item['partNumber'] ?? item['partNo'] ?? '').toString();
          final desc = (item['description'] ?? '').toString();
          final assessable = (item['assessableValue'] as num?)?.toDouble() ?? 0.0;
          final bcd = (item['bcdValue'] as num?)?.toDouble() ?? 0.0;
          final sws = (item['swsValue'] as num?)?.toDouble() ?? 0.0;
          final igst = (item['igstValue'] as num?)?.toDouble() ?? 0.0;
          final qty = (item['quantity'] as num?)?.toDouble() ?? 1.0;
          final unitPrice = (item['unitPrice'] as num?)?.toDouble() ?? 0.0;

          final ratio = totalAssessable > 0 ? (assessable / totalAssessable) : 0.0;
          final boeInt = interest * ratio;
          final expTotal = logistics * ratio;
          final ldcPerQty = qty > 0 ? ((assessable + bcd + sws + boeInt + expTotal) / qty) : 0.0;

          result.add(
            LandedCostRow(
              supplier: row['supplier_name']?.toString() ?? '',
              supplierId: row['supplier_id']?.toString() ?? '',
              invoiceNo: row['invoice_number']?.toString() ?? '',
              invoiceDate: row['invoice_date']?.toString() ?? '',
              partNo: partNo,
              description: desc,
              unit: (item['unit'] ?? 'PCS').toString(),
              qty: qty,
              unitPrice: unitPrice,
              assessableValue: assessable,
              bcdAmount: bcd,
              swsAmount: sws,
              igstAmount: igst,
              boeInterest: boeInt,
              expensesTotal: expTotal,
              ldcPerQty: ldcPerQty,
            ),
          );
        }
      } catch (e) {
        debugPrint('Error parsing boe row: $e');
      }
    }

    return _filterLdcRows(result, search: search, supplierId: supplierId);
  }

  // ==========================================
  // 6. DASHBOARD & DATABASE REPLACEMENT
  // ==========================================
  Future<Map<String, int>> getRecordCounts() async {
    final db = await database;
    try {
      final sh = Sqflite.firstIntValue(await db.rawQuery('SELECT count(*) FROM shipments WHERE deleted_at IS NULL')) ?? 0;
      final inv = Sqflite.firstIntValue(await db.rawQuery('SELECT count(*) FROM invoices WHERE deleted_at IS NULL')) ?? 0;
      final itm = Sqflite.firstIntValue(await db.rawQuery('SELECT count(*) FROM items WHERE deleted_at IS NULL')) ?? 0;
      final sup = Sqflite.firstIntValue(await db.rawQuery('SELECT count(*) FROM suppliers WHERE deleted_at IS NULL')) ?? 0;
      return {
        'shipments': sh,
        'invoices': inv,
        'items': itm,
        'suppliers': sup,
      };
    } catch (_) {
      return {'shipments': 0, 'invoices': 0, 'items': 0, 'suppliers': 0};
    }
  }

  /// Hot-swaps the active SQLite database with a restored snapshot file
  Future<void> replaceDatabaseWithFile(String newDbFilePath) async {
    if (_database != null) {
      try {
        await _database!.close();
      } catch (_) {}
      _database = null;
    }

    final dbPath = await getDatabasesPath();
    final targetPath = p.join(dbPath, 'import_manager_mobile.db');

    final sourceFile = File(newDbFilePath);
    if (!await sourceFile.exists()) {
      throw Exception('Restored database file not found on disk');
    }

    await Directory(dbPath).create(recursive: true);

    try {
      final oldFile = File(targetPath);
      if (await oldFile.exists()) await oldFile.delete();
      final walFile = File('$targetPath-wal');
      if (await walFile.exists()) await walFile.delete();
      final shmFile = File('$targetPath-shm');
      if (await shmFile.exists()) await shmFile.delete();
    } catch (_) {}

    // Replace the database file
    await sourceFile.copy(targetPath);

    // Reopen database cleanly
    _database = await openDatabase(targetPath);
    await _ensureSchema(_database!);
  }
}
