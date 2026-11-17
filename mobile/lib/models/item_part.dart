class ItemPart {
  final String id;
  final String partNumber;
  final String itemDescription;
  final String unit;
  final String currency;
  final double unitPrice;
  final String hsnCode;
  final String supplierId;
  final String supplierName;
  final String countryOfOrigin;
  final String bcd;
  final String sws;
  final String igst;
  final String technicalWriteUp;
  final String category;
  final String endUse;
  final double netWeightKg;
  final double grossWeightPerUomKg;

  ItemPart({
    required this.id,
    required this.partNumber,
    required this.itemDescription,
    required this.unit,
    required this.currency,
    required this.unitPrice,
    required this.hsnCode,
    required this.supplierId,
    required this.supplierName,
    required this.countryOfOrigin,
    required this.bcd,
    required this.sws,
    required this.igst,
    required this.technicalWriteUp,
    required this.category,
    required this.endUse,
    required this.netWeightKg,
    required this.grossWeightPerUomKg,
  });

  factory ItemPart.fromMap(Map<String, dynamic> map) {
    return ItemPart(
      id: map['id']?.toString() ?? '',
      partNumber: map['part_number']?.toString() ?? '',
      itemDescription: map['item_description']?.toString() ?? '',
      unit: map['unit']?.toString() ?? 'PCS',
      currency: map['currency']?.toString() ?? 'USD',
      unitPrice: (map['unit_price'] as num?)?.toDouble() ?? 0.0,
      hsnCode: map['hsn_code']?.toString() ?? '',
      supplierId: map['supplier_id']?.toString() ?? '',
      supplierName: map['supplier_name']?.toString() ?? '',
      countryOfOrigin: map['country_of_origin']?.toString() ?? '',
      bcd: map['bcd']?.toString() ?? '7.5',
      sws: map['sws']?.toString() ?? '10',
      igst: map['igst']?.toString() ?? '18',
      technicalWriteUp: map['technical_write_up']?.toString() ?? '',
      category: map['category']?.toString() ?? '',
      endUse: map['end_use']?.toString() ?? '',
      netWeightKg: (map['net_weight_kg'] as num?)?.toDouble() ?? 0.0,
      grossWeightPerUomKg: (map['gross_weight_per_uom_kg'] as num?)?.toDouble() ?? 0.0,
    );
  }

  // Duty parse helpers
  double get bcdPercent => _parsePercent(bcd);
  double get swsPercent => _parsePercent(sws);
  double get igstPercent => _parsePercent(igst);

  static double _parsePercent(String raw) {
    final cleaned = raw.replaceAll('%', '').trim();
    return double.tryParse(cleaned) ?? 0.0;
  }

  // Customs Duty Structure calculation on 100 base units:
  // Assessable Value = 100
  // BCD Amount = 100 * (BCD% / 100)
  // SWS Amount = BCD Amount * (SWS% / 100) -> note SWS is applied on BCD, e.g. 10% of 7.5% = 0.75%
  // Subtotal for IGST = 100 + BCD Amount + SWS Amount
  // IGST Amount = Subtotal * (IGST% / 100)
  // Total Effective Duty = BCD Amount + SWS Amount + IGST Amount
  // Effective Duty Rate % = Total Effective Duty / 100 * 100%
  double get effectiveDutyRatePercent {
    final bcdRate = bcdPercent / 100.0;
    final swsRate = (swsPercent / 100.0) * bcdRate;
    final subtotal = 1.0 + bcdRate + swsRate;
    final igstRate = (igstPercent / 100.0) * subtotal;
    final totalDutyRate = bcdRate + swsRate + igstRate;
    return totalDutyRate * 100.0;
  }

  // Basic Duty (BCD + SWS) without IGST
  double get basicDutyRatePercent {
    final bcdRate = bcdPercent / 100.0;
    final swsRate = (swsPercent / 100.0) * bcdRate;
    return (bcdRate + swsRate) * 100.0;
  }
}
