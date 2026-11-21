import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';
import '../database/db_helper.dart';
import '../models/company.dart';

class CompanyService extends ChangeNotifier {
  static final CompanyService instance = CompanyService._init();

  static const String _prefCompaniesKey = 'companies_registry_v1';
  static const String _prefActiveCompanyKey = 'active_company_id';

  List<Company> _companies = [];
  Company? _activeCompany;
  bool _initialized = false;

  CompanyService._init();

  List<Company> get companies => List.unmodifiable(_companies);
  Company get activeCompany => _activeCompany ?? _defaultPrimaryCompany();
  bool get isInitialized => _initialized;

  static Company _defaultPrimaryCompany() {
    return const Company(
      id: 'comp_default',
      name: 'Primary Company',
      shortCode: 'PRIMARY',
      baseCurrency: 'INR',
      financialYear: '2026-2027',
      dbFileName: 'import_manager_mobile.db',
      isDefault: true,
      createdAt: '2026-01-01T00:00:00.000Z',
      updatedAt: '2026-01-01T00:00:00.000Z',
    );
  }

  Future<void> init() async {
    if (_initialized) return;

    try {
      final prefs = await SharedPreferences.getInstance();
      final registryJson = prefs.getString(_prefCompaniesKey);

      if (registryJson != null && registryJson.isNotEmpty) {
        final List<dynamic> list = json.decode(registryJson);
        _companies = list.map((item) => Company.fromMap(item as Map<String, dynamic>)).toList();
      }

      // Ensure default primary company is present
      if (_companies.isEmpty || !_companies.any((c) => c.isDefault || c.id == 'comp_default')) {
        _companies.insert(0, _defaultPrimaryCompany());
        await _saveRegistry(prefs);
      }

      // Determine active company
      final savedActiveId = prefs.getString(_prefActiveCompanyKey);
      if (savedActiveId != null) {
        final match = _companies.where((c) => c.id == savedActiveId);
        _activeCompany = match.isNotEmpty ? match.first : _companies.first;
      } else {
        _activeCompany = _companies.first;
      }

      // Ensure DBHelper is pointing to the active company's database
      if (DBHelper.instance.activeDbFileName != activeCompany.dbFileName) {
        await DBHelper.instance.switchDatabase(activeCompany.dbFileName);
      }

      // Synchronize company profile from SQLite if needed
      try {
        final dbProfile = await DBHelper.instance.getActiveCompanyProfile();
        if (dbProfile != null && dbProfile.id == activeCompany.id) {
          // Merge profile details
          _activeCompany = activeCompany.copyWith(
            name: dbProfile.name.isNotEmpty ? dbProfile.name : activeCompany.name,
            iecCode: dbProfile.iecCode ?? activeCompany.iecCode,
            gstin: dbProfile.gstin ?? activeCompany.gstin,
            pan: dbProfile.pan ?? activeCompany.pan,
            registeredAddress: dbProfile.registeredAddress ?? activeCompany.registeredAddress,
            bankName: dbProfile.bankName ?? activeCompany.bankName,
            bankAccountNo: dbProfile.bankAccountNo ?? activeCompany.bankAccountNo,
            bankBranch: dbProfile.bankBranch ?? activeCompany.bankBranch,
            bankSwift: dbProfile.bankSwift ?? activeCompany.bankSwift,
            baseCurrency: dbProfile.baseCurrency.isNotEmpty ? dbProfile.baseCurrency : activeCompany.baseCurrency,
            financialYear: dbProfile.financialYear ?? activeCompany.financialYear,
          );
        }
      } catch (e) {
        debugPrint('Sync SQLite profile error: $e');
      }

      _initialized = true;
      notifyListeners();
    } catch (e, st) {
      debugPrint('CompanyService.init error: $e\n$st');
      _companies = [_defaultPrimaryCompany()];
      _activeCompany = _companies.first;
      _initialized = true;
      notifyListeners();
    }
  }

  Future<void> _saveRegistry(SharedPreferences prefs) async {
    final list = _companies.map((c) => c.toMap()).toList();
    await prefs.setString(_prefCompaniesKey, json.encode(list));
  }

  /// Switches active company and rebinds database
  Future<void> switchCompany(String companyId) async {
    final target = _companies.firstWhere(
      (c) => c.id == companyId,
      orElse: () => throw Exception('Company with ID $companyId not found'),
    );

    if (_activeCompany?.id == target.id) return;

    // Switch SQLite database
    await DBHelper.instance.switchDatabase(target.dbFileName);

    _activeCompany = target;
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_prefActiveCompanyKey, target.id);

    notifyListeners();
  }

  /// Creates a new company with a dedicated fresh, clean database (0 records)
  Future<Company> createCompany(CreateCompanyInput input) async {
    final trimmedName = input.name.trim();
    final trimmedCode = input.shortCode.trim().toUpperCase();

    if (trimmedName.isEmpty) {
      throw Exception('Company Name is required');
    }
    if (trimmedCode.isEmpty) {
      throw Exception('Company Short Code is required');
    }

    final sanitizedCode = trimmedCode.replaceAll(RegExp(r'[^A-Za-z0-9_]'), '');
    if (sanitizedCode.isEmpty) {
      throw Exception('Short Code must contain valid letters or numbers');
    }

    // Check unique code
    if (_companies.any((c) => c.shortCode.toUpperCase() == sanitizedCode)) {
      throw Exception('Company with short code "$sanitizedCode" already exists');
    }

    final timestamp = DateTime.now().millisecondsSinceEpoch;
    final compId = 'comp_${sanitizedCode.toLowerCase()}_$timestamp';
    final dbFileName = 'company_${sanitizedCode.toLowerCase()}.db';
    final nowStr = DateTime.now().toIso8601String();

    final newCompany = Company(
      id: compId,
      name: trimmedName,
      shortCode: sanitizedCode,
      iecCode: input.iecCode?.trim().isNotEmpty == true ? input.iecCode!.trim() : null,
      gstin: input.gstin?.trim().isNotEmpty == true ? input.gstin!.trim() : null,
      pan: input.pan?.trim().isNotEmpty == true ? input.pan!.trim() : null,
      registeredAddress: input.registeredAddress?.trim().isNotEmpty == true ? input.registeredAddress!.trim() : null,
      bankName: input.bankName?.trim().isNotEmpty == true ? input.bankName!.trim() : null,
      bankAccountNo: input.bankAccountNo?.trim().isNotEmpty == true ? input.bankAccountNo!.trim() : null,
      bankBranch: input.bankBranch?.trim().isNotEmpty == true ? input.bankBranch!.trim() : null,
      bankSwift: input.bankSwift?.trim().isNotEmpty == true ? input.bankSwift!.trim() : null,
      baseCurrency: input.baseCurrency.trim().isNotEmpty ? input.baseCurrency.trim() : 'INR',
      financialYear: input.financialYear?.trim().isNotEmpty == true ? input.financialYear!.trim() : '2026-2027',
      dbFileName: dbFileName,
      isDefault: false,
      createdAt: nowStr,
      updatedAt: nowStr,
    );

    // Create fresh clean SQLite file (0 records)
    await DBHelper.instance.createFreshCompanyDatabase(dbFileName, newCompany);

    _companies.add(newCompany);
    final prefs = await SharedPreferences.getInstance();
    await _saveRegistry(prefs);

    // Auto-switch to newly created company
    await switchCompany(newCompany.id);

    return newCompany;
  }

  /// Updates profile and statutory info of a company
  Future<Company> updateCompanyProfile(UpdateCompanyInput input) async {
    final idx = _companies.indexWhere((c) => c.id == input.id);
    if (idx == -1) {
      throw Exception('Company not found');
    }

    final existing = _companies[idx];
    final updated = existing.copyWith(
      name: input.name.trim().isNotEmpty ? input.name.trim() : existing.name,
      iecCode: input.iecCode?.trim(),
      gstin: input.gstin?.trim(),
      pan: input.pan?.trim(),
      registeredAddress: input.registeredAddress?.trim(),
      bankName: input.bankName?.trim(),
      bankAccountNo: input.bankAccountNo?.trim(),
      bankBranch: input.bankBranch?.trim(),
      bankSwift: input.bankSwift?.trim(),
      baseCurrency: input.baseCurrency.trim().isNotEmpty ? input.baseCurrency.trim() : existing.baseCurrency,
      financialYear: input.financialYear?.trim(),
      updatedAt: DateTime.now().toIso8601String(),
    );

    _companies[idx] = updated;
    if (_activeCompany?.id == updated.id) {
      _activeCompany = updated;
      // Persist to active SQLite DB company_profile table
      await DBHelper.instance.saveCompanyProfile(updated);
    }

    final prefs = await SharedPreferences.getInstance();
    await _saveRegistry(prefs);

    notifyListeners();
    return updated;
  }

  /// Deletes a secondary company and its dedicated database
  Future<void> deleteCompany(String companyId) async {
    final idx = _companies.indexWhere((c) => c.id == companyId);
    if (idx == -1) {
      throw Exception('Company not found');
    }

    final company = _companies[idx];
    if (company.isDefault || company.id == 'comp_default') {
      throw Exception('Primary Company cannot be deleted');
    }

    if (_activeCompany?.id == company.id) {
      throw Exception('Cannot delete the currently active company. Please switch to another company first.');
    }

    // Delete database file
    await DBHelper.instance.deleteCompanyDatabase(company.dbFileName);

    _companies.removeAt(idx);
    final prefs = await SharedPreferences.getInstance();
    await _saveRegistry(prefs);

    notifyListeners();
  }
}
