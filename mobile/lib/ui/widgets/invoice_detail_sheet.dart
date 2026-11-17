import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import '../../constants/app_colors.dart';
import '../../database/db_helper.dart';
import '../../models/invoice.dart';
import 'status_badge.dart';

class InvoiceDetailSheet extends StatefulWidget {
  final Invoice invoice;

  const InvoiceDetailSheet({super.key, required this.invoice});

  static void show(BuildContext context, Invoice invoice) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (_) => InvoiceDetailSheet(invoice: invoice),
    );
  }

  @override
  State<InvoiceDetailSheet> createState() => _InvoiceDetailSheetState();
}

class _InvoiceDetailSheetState extends State<InvoiceDetailSheet> {
  Invoice? _fullInvoice;
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _loadLines();
  }

  Future<void> _loadLines() async {
    final detail = await DBHelper.instance.getInvoiceDetail(widget.invoice.id);
    if (mounted) {
      setState(() {
        _fullInvoice = detail ?? widget.invoice;
        _loading = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final inv = _fullInvoice ?? widget.invoice;
    final currencyFormatter = NumberFormat.currency(
      symbol: inv.currency == 'USD'
          ? '\$'
          : (inv.currency == 'EUR' ? '€' : '₹ '),
      decimalDigits: 2,
    );

    return Container(
      decoration: BoxDecoration(
        color: Theme.of(context).cardColor,
        borderRadius: const BorderRadius.vertical(top: Radius.circular(24)),
      ),
      padding: EdgeInsets.only(
        top: 16,
        left: 20,
        right: 20,
        bottom: MediaQuery.of(context).viewPadding.bottom + 20,
      ),
      constraints: BoxConstraints(
        maxHeight: MediaQuery.of(context).size.height * 0.88,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          // Drag handle
          Center(
            child: Container(
              width: 44,
              height: 4,
              decoration: BoxDecoration(
                color: Colors.grey.withAlpha(90),
                borderRadius: BorderRadius.circular(2),
              ),
            ),
          ),
          const SizedBox(height: 16),

          // Header: Invoice No + Status
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      inv.invoiceNumber,
                      style: const TextStyle(fontSize: 20, fontWeight: FontWeight.bold),
                    ),
                    const SizedBox(height: 3),
                    Text(
                      inv.supplierName,
                      style: const TextStyle(fontSize: 13.5, color: Color(0xFF94A3B8)),
                    ),
                  ],
                ),
              ),
              StatusBadge(status: inv.status),
            ],
          ),
          const SizedBox(height: 14),

          // Summary Box
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
            decoration: BoxDecoration(
              color: Theme.of(context).brightness == Brightness.dark
                  ? AppColors.darkSurface
                  : const Color(0xFFF1F5F9),
              borderRadius: BorderRadius.circular(10),
            ),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const Text('Invoice Date', style: TextStyle(fontSize: 11, color: Color(0xFF94A3B8))),
                    const SizedBox(height: 2),
                    Text(inv.invoiceDate, style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w600)),
                  ],
                ),
                Column(
                  crossAxisAlignment: CrossAxisAlignment.end,
                  children: [
                    const Text('Commercial Total', style: TextStyle(fontSize: 11, color: Color(0xFF94A3B8))),
                    const SizedBox(height: 2),
                    Text(
                      currencyFormatter.format(inv.invoiceValue),
                      style: const TextStyle(fontSize: 15, fontWeight: FontWeight.bold, color: AppColors.primaryLight),
                    ),
                  ],
                ),
              ],
            ),
          ),
          const SizedBox(height: 16),

          // Line Items Header
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                'Line Items (${inv.lines.length})',
                style: const TextStyle(fontSize: 14, fontWeight: FontWeight.bold),
              ),
              const Text(
                'VIEW ONLY',
                style: TextStyle(fontSize: 10, fontWeight: FontWeight.bold, color: Colors.blueAccent),
              ),
            ],
          ),
          const SizedBox(height: 10),

          // Line Items List
          Expanded(
            child: _loading
                ? const Center(child: CircularProgressIndicator())
                : inv.lines.isEmpty
                    ? const Center(
                        child: Text(
                          'No line items found for this invoice.',
                          style: TextStyle(color: Color(0xFF94A3B8)),
                        ),
                      )
                    : ListView.separated(
                        itemCount: inv.lines.length,
                        separatorBuilder: (_, _) => const SizedBox(height: 8),
                        itemBuilder: (context, index) {
                          final line = inv.lines[index];
                          return Container(
                            padding: const EdgeInsets.all(12),
                            decoration: BoxDecoration(
                              color: Theme.of(context).brightness == Brightness.dark
                                  ? AppColors.darkSurface
                                  : Colors.white,
                              borderRadius: BorderRadius.circular(10),
                              border: Border.all(color: AppColors.darkBorder.withAlpha(60)),
                            ),
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Row(
                                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                                  children: [
                                    Expanded(
                                      child: Text(
                                        line.partNumber,
                                        style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 13.5),
                                      ),
                                    ),
                                    Text(
                                      currencyFormatter.format(line.lineTotal),
                                      style: const TextStyle(fontWeight: FontWeight.w700, fontSize: 13.5),
                                    ),
                                  ],
                                ),
                                const SizedBox(height: 3),
                                Text(
                                  line.itemDescription,
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  style: const TextStyle(fontSize: 12, color: Color(0xFF94A3B8)),
                                ),
                                const SizedBox(height: 6),
                                Row(
                                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                                  children: [
                                    Text(
                                      '${line.quantity.toStringAsFixed(0)} ${line.unit} × ${currencyFormatter.format(line.unitPrice)}',
                                      style: const TextStyle(fontSize: 11.5, color: Color(0xFF94A3B8)),
                                    ),
                                    Text(
                                      'HSN: ${line.hsnCode} • BCD: ${line.dutyPercent}%',
                                      style: const TextStyle(fontSize: 11, color: AppColors.accent),
                                    ),
                                  ],
                                ),
                              ],
                            ),
                          );
                        },
                      ),
          ),

          const SizedBox(height: 12),
          SizedBox(
            width: double.infinity,
            child: OutlinedButton(
              onPressed: () => Navigator.pop(context),
              style: OutlinedButton.styleFrom(
                padding: const EdgeInsets.symmetric(vertical: 14),
                shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
              ),
              child: const Text('Close'),
            ),
          ),
        ],
      ),
    );
  }
}
