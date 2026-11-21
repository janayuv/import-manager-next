class Company {
  final String id;
  final String name;
  final String shortCode;
  final String? iecCode;
  final String? gstin;
  final String? pan;
  final String? registeredAddress;
  final String? bankName;
  final String? bankAccountNo;
  final String? bankBranch;
  final String? bankSwift;
  final String baseCurrency;
  final String? financialYear;
  final String dbFileName;
  final bool isDefault;
  final String createdAt;
  final String updatedAt;

  const Company({
    required this.id,
    required this.name,
    required this.shortCode,
    this.iecCode,
    this.gstin,
    this.pan,
    this.registeredAddress,
    this.bankName,
    this.bankAccountNo,
    this.bankBranch,
    this.bankSwift,
    this.baseCurrency = 'INR',
    this.financialYear,
    required this.dbFileName,
    this.isDefault = false,
    required this.createdAt,
    required this.updatedAt,
  });

  Company copyWith({
    String? id,
    String? name,
    String? shortCode,
    String? iecCode,
    String? gstin,
    String? pan,
    String? registeredAddress,
    String? bankName,
    String? bankAccountNo,
    String? bankBranch,
    String? bankSwift,
    String? baseCurrency,
    String? financialYear,
    String? dbFileName,
    bool? isDefault,
    String? createdAt,
    String? updatedAt,
  }) {
    return Company(
      id: id ?? this.id,
      name: name ?? this.name,
      shortCode: shortCode ?? this.shortCode,
      iecCode: iecCode ?? this.iecCode,
      gstin: gstin ?? this.gstin,
      pan: pan ?? this.pan,
      registeredAddress: registeredAddress ?? this.registeredAddress,
      bankName: bankName ?? this.bankName,
      bankAccountNo: bankAccountNo ?? this.bankAccountNo,
      bankBranch: bankBranch ?? this.bankBranch,
      bankSwift: bankSwift ?? this.bankSwift,
      baseCurrency: baseCurrency ?? this.baseCurrency,
      financialYear: financialYear ?? this.financialYear,
      dbFileName: dbFileName ?? this.dbFileName,
      isDefault: isDefault ?? this.isDefault,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
    );
  }

  Map<String, dynamic> toMap() {
    return {
      'id': id,
      'company_name': name,
      'short_code': shortCode,
      'iec_code': iecCode,
      'gstin': gstin,
      'pan': pan,
      'registered_address': registeredAddress,
      'bank_name': bankName,
      'bank_account_no': bankAccountNo,
      'bank_branch': bankBranch,
      'bank_swift': bankSwift,
      'base_currency': baseCurrency,
      'financial_year': financialYear,
      'db_file_name': dbFileName,
      'is_default': isDefault ? 1 : 0,
      'created_at': createdAt,
      'updated_at': updatedAt,
    };
  }

  factory Company.fromMap(Map<String, dynamic> map) {
    return Company(
      id: map['id']?.toString() ?? '',
      name: map['company_name']?.toString() ?? map['name']?.toString() ?? '',
      shortCode: map['short_code']?.toString() ?? map['shortCode']?.toString() ?? '',
      iecCode: map['iec_code']?.toString() ?? map['iecCode']?.toString(),
      gstin: map['gstin']?.toString(),
      pan: map['pan']?.toString(),
      registeredAddress: map['registered_address']?.toString() ?? map['registeredAddress']?.toString(),
      bankName: map['bank_name']?.toString() ?? map['bankName']?.toString(),
      bankAccountNo: map['bank_account_no']?.toString() ?? map['bankAccountNo']?.toString(),
      bankBranch: map['bank_branch']?.toString() ?? map['bankBranch']?.toString(),
      bankSwift: map['bank_swift']?.toString() ?? map['bankSwift']?.toString(),
      baseCurrency: map['base_currency']?.toString() ?? map['baseCurrency']?.toString() ?? 'INR',
      financialYear: map['financial_year']?.toString() ?? map['financialYear']?.toString(),
      dbFileName: map['db_file_name']?.toString() ?? map['dbFileName']?.toString() ?? 'import_manager_mobile.db',
      isDefault: map['is_default'] == 1 || map['is_default'] == true || map['isDefault'] == true,
      createdAt: map['created_at']?.toString() ?? map['createdAt']?.toString() ?? DateTime.now().toIso8601String(),
      updatedAt: map['updated_at']?.toString() ?? map['updatedAt']?.toString() ?? DateTime.now().toIso8601String(),
    );
  }

  Map<String, dynamic> toJson() => toMap();

  factory Company.fromJson(Map<String, dynamic> json) => Company.fromMap(json);
}

class CreateCompanyInput {
  final String name;
  final String shortCode;
  final String? iecCode;
  final String? gstin;
  final String? pan;
  final String? registeredAddress;
  final String? bankName;
  final String? bankAccountNo;
  final String? bankBranch;
  final String? bankSwift;
  final String baseCurrency;
  final String? financialYear;

  const CreateCompanyInput({
    required this.name,
    required this.shortCode,
    this.iecCode,
    this.gstin,
    this.pan,
    this.registeredAddress,
    this.bankName,
    this.bankAccountNo,
    this.bankBranch,
    this.bankSwift,
    this.baseCurrency = 'INR',
    this.financialYear,
  });
}

class UpdateCompanyInput {
  final String id;
  final String name;
  final String? iecCode;
  final String? gstin;
  final String? pan;
  final String? registeredAddress;
  final String? bankName;
  final String? bankAccountNo;
  final String? bankBranch;
  final String? bankSwift;
  final String baseCurrency;
  final String? financialYear;

  const UpdateCompanyInput({
    required this.id,
    required this.name,
    this.iecCode,
    this.gstin,
    this.pan,
    this.registeredAddress,
    this.bankName,
    this.bankAccountNo,
    this.bankBranch,
    this.bankSwift,
    this.baseCurrency = 'INR',
    this.financialYear,
  });
}
