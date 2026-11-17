class LandedCostRow {
  final String supplier;
  final String supplierId;
  final String invoiceNo;
  final String invoiceDate;
  final String partNo;
  final String description;
  final String unit;
  final double qty;
  final double unitPrice;
  final double assessableValue;
  final double bcdAmount;
  final double swsAmount;
  final double igstAmount;
  final double boeInterest;
  final double expensesTotal;
  final double ldcPerQty;

  LandedCostRow({
    required this.supplier,
    required this.supplierId,
    required this.invoiceNo,
    required this.invoiceDate,
    required this.partNo,
    required this.description,
    required this.unit,
    required this.qty,
    required this.unitPrice,
    required this.assessableValue,
    required this.bcdAmount,
    required this.swsAmount,
    required this.igstAmount,
    required this.boeInterest,
    required this.expensesTotal,
    required this.ldcPerQty,
  });

  factory LandedCostRow.fromMap(Map<String, dynamic> map) {
    return LandedCostRow(
      supplier: map['supplier']?.toString() ?? '',
      supplierId: map['supplier_id']?.toString() ?? '',
      invoiceNo: map['invoice_no']?.toString() ?? '',
      invoiceDate: map['invoice_date']?.toString() ?? '',
      partNo: map['part_no']?.toString() ?? '',
      description: map['description']?.toString() ?? '',
      unit: map['unit']?.toString() ?? 'PCS',
      qty: (map['qty'] as num?)?.toDouble() ?? 0.0,
      unitPrice: (map['unit_price'] as num?)?.toDouble() ?? 0.0,
      assessableValue: (map['assessable_value'] as num?)?.toDouble() ?? 0.0,
      bcdAmount: (map['bcd_amount'] as num?)?.toDouble() ?? 0.0,
      swsAmount: (map['sws_amount'] as num?)?.toDouble() ?? 0.0,
      igstAmount: (map['igst_amount'] as num?)?.toDouble() ?? 0.0,
      boeInterest: (map['boe_interest'] as num?)?.toDouble() ?? 0.0,
      expensesTotal: (map['expenses_total'] as num?)?.toDouble() ?? 0.0,
      ldcPerQty: (map['ldc_per_qty'] as num?)?.toDouble() ?? 0.0,
    );
  }

  // Basic customs duty sum (BCD + SWS)
  double get totalBasicDuties => bcdAmount + swsAmount;

  // Total Landed Cost (Excl. IGST) for this row
  double get totalLandedCostExIgst => assessableValue + bcdAmount + swsAmount + boeInterest + expensesTotal;
}

class LandedCostSummary {
  final int totalRows;
  final double totalQuantity;
  final double totalAssessableValue;
  final double totalBasicDuties; // BCD + SWS
  final double totalIgst;
  final double totalBoeInterest;
  final double totalBasicExpenses;
  final double totalLandedCostExIgst;

  LandedCostSummary({
    required this.totalRows,
    required this.totalQuantity,
    required this.totalAssessableValue,
    required this.totalBasicDuties,
    required this.totalIgst,
    required this.totalBoeInterest,
    required this.totalBasicExpenses,
    required this.totalLandedCostExIgst,
  });

  factory LandedCostSummary.fromRows(List<LandedCostRow> rows) {
    int count = rows.length;
    double qty = 0;
    double assessable = 0;
    double basicDuties = 0;
    double igst = 0;
    double interest = 0;
    double expenses = 0;
    double landedExIgst = 0;

    for (final r in rows) {
      qty += r.qty;
      assessable += r.assessableValue;
      basicDuties += (r.bcdAmount + r.swsAmount);
      igst += r.igstAmount;
      interest += r.boeInterest;
      expenses += r.expensesTotal;
      landedExIgst += r.totalLandedCostExIgst;
    }

    return LandedCostSummary(
      totalRows: count,
      totalQuantity: qty,
      totalAssessableValue: assessable,
      totalBasicDuties: basicDuties,
      totalIgst: igst,
      totalBoeInterest: interest,
      totalBasicExpenses: expenses,
      totalLandedCostExIgst: landedExIgst,
    );
  }
}
