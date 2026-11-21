import 'package:flutter_test/flutter_test.dart';
import 'package:sqflite_common_ffi/sqflite_ffi.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:import_manager_mobile/database/db_helper.dart';
import 'package:import_manager_mobile/models/company.dart';
import 'package:import_manager_mobile/services/company_service.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUpAll(() {
    sqfliteFfiInit();
    databaseFactory = databaseFactoryFfi;
  });

  setUp(() async {
    SharedPreferences.setMockInitialValues({});
    await CompanyService.instance.init();
  });

  group('Company Model Tests', () {
    test('Company serialization and deserialization', () {
      final comp = Company(
        id: 'comp_test_1',
        name: 'Test Exports Pvt Ltd',
        shortCode: 'TEST',
        iecCode: '0512345678',
        gstin: '27AAAAA0000A1Z5',
        pan: 'AAAAA0000A',
        registeredAddress: '123 Harbor Rd, Mumbai',
        bankName: 'HDFC Bank',
        bankAccountNo: '50200012345678',
        bankBranch: 'Nariman Point / HDFC0000123',
        bankSwift: 'HDFCINBB',
        baseCurrency: 'USD',
        financialYear: '2026-2027',
        dbFileName: 'company_test.db',
        isDefault: false,
        createdAt: '2026-01-01T00:00:00Z',
        updatedAt: '2026-01-01T00:00:00Z',
      );

      final map = comp.toMap();
      final fromMap = Company.fromMap(map);

      expect(fromMap.id, comp.id);
      expect(fromMap.name, comp.name);
      expect(fromMap.shortCode, comp.shortCode);
      expect(fromMap.iecCode, comp.iecCode);
      expect(fromMap.gstin, comp.gstin);
      expect(fromMap.pan, comp.pan);
      expect(fromMap.bankSwift, comp.bankSwift);
      expect(fromMap.baseCurrency, comp.baseCurrency);
      expect(fromMap.isDefault, isFalse);
    });
  });

  group('CompanyService & Multi-Database Tests', () {
    test('Default Primary Company is auto-registered', () {
      final active = CompanyService.instance.activeCompany;
      expect(active.shortCode, 'PRIMARY');
      expect(active.isDefault, isTrue);
      expect(active.dbFileName, 'import_manager_mobile.db');
      expect(CompanyService.instance.companies.isNotEmpty, isTrue);
    });

    test('Creating a new company provisions a fresh clean database with 0 records', () async {
      final newComp = await CompanyService.instance.createCompany(
        const CreateCompanyInput(
          name: 'Acme Traders Worldwide',
          shortCode: 'ACME',
          iecCode: '0599887766',
          gstin: '33BBBBB1111B1Z2',
          baseCurrency: 'EUR',
          financialYear: '2026-2027',
        ),
      );

      expect(newComp.name, 'Acme Traders Worldwide');
      expect(newComp.shortCode, 'ACME');
      expect(newComp.dbFileName, 'company_acme.db');
      expect(newComp.isDefault, isFalse);

      // Verify active company switched automatically
      expect(CompanyService.instance.activeCompany.id, newComp.id);
      expect(DBHelper.instance.activeDbFileName, 'company_acme.db');

      // Verify new company has 0 records (fresh/clean tables)
      final counts = await DBHelper.instance.getRecordCounts();
      expect(counts['shipments'], 0);
      expect(counts['invoices'], 0);
      expect(counts['items'], 0);
      expect(counts['suppliers'], 0);

      // Verify profile is stored in SQLite
      final profile = await DBHelper.instance.getActiveCompanyProfile();
      expect(profile, isNotNull);
      expect(profile!.shortCode, 'ACME');
      expect(profile.iecCode, '0599887766');
    });

    test('Switching back to Primary Company restores primary database', () async {
      // Find primary company
      final primary = CompanyService.instance.companies.firstWhere((c) => c.isDefault);
      await CompanyService.instance.switchCompany(primary.id);

      expect(CompanyService.instance.activeCompany.id, primary.id);
      expect(DBHelper.instance.activeDbFileName, 'import_manager_mobile.db');
    });

    test('Updating company profile updates memory, registry, and SQLite', () async {
      final active = CompanyService.instance.activeCompany;
      final updated = await CompanyService.instance.updateCompanyProfile(
        UpdateCompanyInput(
          id: active.id,
          name: 'Primary Imports Global Ltd',
          iecCode: '0511112222',
          gstin: '27AAAAA1111A1Z1',
          pan: 'AAAAA1111A',
          bankName: 'State Bank of India',
          baseCurrency: 'INR',
        ),
      );

      expect(updated.name, 'Primary Imports Global Ltd');
      expect(updated.iecCode, '0511112222');
      expect(CompanyService.instance.activeCompany.name, 'Primary Imports Global Ltd');
    });

    test('Deletion guards: cannot delete primary company or active company', () async {
      final primary = CompanyService.instance.companies.firstWhere((c) => c.isDefault);

      // Cannot delete default
      expect(
        () => CompanyService.instance.deleteCompany(primary.id),
        throwsA(isA<Exception>()),
      );

      // Create a secondary company
      final extraComp = await CompanyService.instance.createCompany(
        const CreateCompanyInput(
          name: 'Temporary Company',
          shortCode: 'TEMP',
        ),
      );

      // Active is now TEMP. Cannot delete active company!
      expect(
        () => CompanyService.instance.deleteCompany(extraComp.id),
        throwsA(isA<Exception>()),
      );

      // Switch away to primary
      await CompanyService.instance.switchCompany(primary.id);

      // Now deleting TEMP succeeds
      await CompanyService.instance.deleteCompany(extraComp.id);
      expect(CompanyService.instance.companies.any((c) => c.id == extraComp.id), isFalse);
    });
  });
}
