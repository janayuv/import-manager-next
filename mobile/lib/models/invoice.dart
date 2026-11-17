class Invoice {
  final String id;
  final String shipmentId;
  final String status;
  final String invoiceNumber;
  final String invoiceDate;
  final String supplierName;
  final String supplierId;
  final String currency;
  final double invoiceValue;
  final int lineCount;
  final List<InvoiceLineItem> lines;

  Invoice({
    required this.id,
    required this.shipmentId,
    required this.status,
    required this.invoiceNumber,
    required this.invoiceDate,
    required this.supplierName,
    required this.supplierId,
    required this.currency,
    required this.invoiceValue,
    required this.lineCount,
    this.lines = const [],
  });

  factory Invoice.fromMap(Map<String, dynamic> map, {List<InvoiceLineItem> lines = const []}) {
    return Invoice(
      id: map['id']?.toString() ?? '',
      shipmentId: map['shipment_id']?.toString() ?? '',
      status: map['status']?.toString() ?? 'Draft',
      invoiceNumber: map['invoice_number']?.toString() ?? '',
      invoiceDate: map['invoice_date']?.toString() ?? '',
      supplierName: map['supplier_name']?.toString() ?? '',
      supplierId: map['supplier_id']?.toString() ?? '',
      currency: map['currency']?.toString() ?? (map['invoice_currency']?.toString() ?? 'USD'),
      invoiceValue: (map['invoice_value'] as num?)?.toDouble() ?? (map['computed_total'] as num?)?.toDouble() ?? 0.0,
      lineCount: (map['line_count'] as num?)?.toInt() ?? lines.length,
      lines: lines,
    );
  }

  Invoice copyWithLines(List<InvoiceLineItem> newLines) {
    return Invoice(
      id: id,
      shipmentId: shipmentId,
      status: status,
      invoiceNumber: invoiceNumber,
      invoiceDate: invoiceDate,
      supplierName: supplierName,
      supplierId: supplierId,
      currency: currency,
      invoiceValue: invoiceValue,
      lineCount: newLines.length,
      lines: newLines,
    );
  }
}

class InvoiceLineItem {
  final String id;
  final String invoiceId;
  final String itemId;
  final String partNumber;
  final String itemDescription;
  final String unit;
  final String hsnCode;
  final double quantity;
  final double unitPrice;
  final double lineTotal;
  final double dutyPercent;
  final double swsPercent;
  final double igstPercent;

  InvoiceLineItem({
    required this.id,
    required this.invoiceId,
    required this.itemId,
    required this.partNumber,
    required this.itemDescription,
    required this.unit,
    required this.hsnCode,
    required this.quantity,
    required this.unitPrice,
    required this.lineTotal,
    required this.dutyPercent,
    required this.swsPercent,
    required this.igstPercent,
  });

  factory InvoiceLineItem.fromMap(Map<String, dynamic> map) {
    final qty = (map['quantity'] as num?)?.toDouble() ?? 0.0;
    final price = (map['unit_price'] as num?)?.toDouble() ?? 0.0;
    final total = (map['line_total'] as num?)?.toDouble() ?? (qty * price);

    return InvoiceLineItem(
      id: map['id']?.toString() ?? '',
      invoiceId: map['invoice_id']?.toString() ?? '',
      itemId: map['item_id']?.toString() ?? '',
      partNumber: map['part_number']?.toString() ?? '',
      itemDescription: map['item_description']?.toString() ?? '',
      unit: map['unit']?.toString() ?? 'PCS',
      hsnCode: map['hsn_code']?.toString() ?? '',
      quantity: qty,
      unitPrice: price,
      lineTotal: total,
      dutyPercent: (map['duty_percent'] as num?)?.toDouble() ?? 7.5,
      swsPercent: (map['sws_percent'] as num?)?.toDouble() ?? 10.0,
      igstPercent: (map['igst_percent'] as num?)?.toDouble() ?? 18.0,
    );
  }
}
