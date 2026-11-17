class Shipment {
  final String id;
  final String supplierId;
  final String supplierName;
  final String invoiceNumber;
  final String invoiceDate;
  final String goodsCategory;
  final double invoiceValue;
  final String invoiceCurrency;
  final String incoterm;
  final String shipmentMode;
  final String shipmentType;
  final String blAwbNumber;
  final String blAwbDate;
  final String vesselName;
  final String containerNumber;
  final double grossWeightKg;
  final String etd;
  final String eta;
  final String status;
  final String dateOfDelivery;
  final bool isFrozen;

  Shipment({
    required this.id,
    required this.supplierId,
    required this.supplierName,
    required this.invoiceNumber,
    required this.invoiceDate,
    required this.goodsCategory,
    required this.invoiceValue,
    required this.invoiceCurrency,
    required this.incoterm,
    required this.shipmentMode,
    required this.shipmentType,
    required this.blAwbNumber,
    required this.blAwbDate,
    required this.vesselName,
    required this.containerNumber,
    required this.grossWeightKg,
    required this.etd,
    required this.eta,
    required this.status,
    required this.dateOfDelivery,
    required this.isFrozen,
  });

  factory Shipment.fromMap(Map<String, dynamic> map) {
    return Shipment(
      id: map['id']?.toString() ?? '',
      supplierId: map['supplier_id']?.toString() ?? '',
      supplierName: map['supplier_name']?.toString() ?? '',
      invoiceNumber: map['invoice_number']?.toString() ?? '',
      invoiceDate: map['invoice_date']?.toString() ?? '',
      goodsCategory: map['goods_category']?.toString() ?? '',
      invoiceValue: (map['invoice_value'] as num?)?.toDouble() ?? 0.0,
      invoiceCurrency: map['invoice_currency']?.toString() ?? 'USD',
      incoterm: map['incoterm']?.toString() ?? '',
      shipmentMode: map['shipment_mode']?.toString() ?? '',
      shipmentType: map['shipment_type']?.toString() ?? '',
      blAwbNumber: map['bl_awb_number']?.toString() ?? '',
      blAwbDate: map['bl_awb_date']?.toString() ?? '',
      vesselName: map['vessel_name']?.toString() ?? '',
      containerNumber: map['container_number']?.toString() ?? '',
      grossWeightKg: (map['gross_weight_kg'] as num?)?.toDouble() ?? 0.0,
      etd: map['etd']?.toString() ?? '',
      eta: map['eta']?.toString() ?? '',
      status: map['status']?.toString() ?? 'in-transit',
      dateOfDelivery: map['date_of_delivery']?.toString() ?? '',
      isFrozen: (map['is_frozen'] == 1 || map['is_frozen'] == true),
    );
  }

  // ETD/ETA Helpers
  DateTime? get etdDate => _parseDate(etd);
  DateTime? get etaDate => _parseDate(eta);

  static DateTime? _parseDate(String raw) {
    if (raw.trim().isEmpty) return null;
    try {
      // Try ISO YYYY-MM-DD
      return DateTime.parse(raw.trim());
    } catch (_) {
      // Try DD/MM/YYYY or DD-MM-YYYY
      final parts = raw.split(RegExp(r'[-/]'));
      if (parts.length == 3) {
        final p0 = int.tryParse(parts[0]);
        final p1 = int.tryParse(parts[1]);
        final p2 = int.tryParse(parts[2]);
        if (p0 != null && p1 != null && p2 != null) {
          if (p2 > 1000) {
            // DD/MM/YYYY
            return DateTime(p2, p1, p0);
          } else if (p0 > 1000) {
            // YYYY/MM/DD
            return DateTime(p0, p1, p2);
          }
        }
      }
      return null;
    }
  }

  int? get daysUntilEta {
    final target = etaDate;
    if (target == null) return null;
    final now = DateTime.now();
    final today = DateTime(now.year, now.month, now.day);
    final targetDay = DateTime(target.year, target.month, target.day);
    return targetDay.difference(today).inDays;
  }

  bool get isUpcoming {
    final st = status.toLowerCase();
    if (st == 'delivered' || st == 'cancelled' || st == 'completed' || st == 'closed') {
      return false;
    }
    return true;
  }

  bool get isDelayed {
    final diff = daysUntilEta;
    if (diff == null) return false;
    final st = status.toLowerCase();
    return diff < 0 && st != 'delivered' && st != 'completed';
  }
}
