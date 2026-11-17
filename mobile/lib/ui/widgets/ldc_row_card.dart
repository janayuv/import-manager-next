import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import '../../constants/app_colors.dart';
import '../../models/landed_cost.dart';

class LdcRowCard extends StatelessWidget {
  final LandedCostRow row;

  const LdcRowCard({super.key, required this.row});

  @override
  Widget build(BuildContext context) {
    final currencyFormatter = NumberFormat.currency(symbol: '₹ ', decimalDigits: 2);

    return Card(
      child: Padding(
        padding: const EdgeInsets.all(15),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Part Number + LDC / Qty Badge
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Expanded(
                  child: Text(
                    row.partNo,
                    style: const TextStyle(
                      fontSize: 16,
                      fontWeight: FontWeight.bold,
                      letterSpacing: -0.2,
                    ),
                  ),
                ),
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
                  decoration: BoxDecoration(
                    color: AppColors.ldcBadge.withAlpha(35),
                    borderRadius: BorderRadius.circular(8),
                    border: Border.all(color: AppColors.ldcBadge),
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.end,
                    children: [
                      const Text(
                        'LDC / QTY',
                        style: TextStyle(
                          fontSize: 9.5,
                          fontWeight: FontWeight.bold,
                          color: AppColors.ldcBadge,
                        ),
                      ),
                      Text(
                        '₹ ${row.ldcPerQty.toStringAsFixed(2)}',
                        style: const TextStyle(
                          fontSize: 14,
                          fontWeight: FontWeight.w800,
                          color: AppColors.ldcBadge,
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
            const SizedBox(height: 3),

            // Description
            Text(
              row.description,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(fontSize: 13, color: Color(0xFF94A3B8)),
            ),
            const SizedBox(height: 10),

            // Invoice & Supplier Info
            Row(
              children: [
                const Icon(Icons.receipt_outlined, size: 14, color: AppColors.accent),
                const SizedBox(width: 5),
                Text(
                  '${row.invoiceNo} (${row.invoiceDate})',
                  style: const TextStyle(fontSize: 12, fontWeight: FontWeight.w600),
                ),
                const Spacer(),
                Text(
                  '${row.qty.toStringAsFixed(0)} ${row.unit}',
                  style: const TextStyle(fontSize: 12, fontWeight: FontWeight.bold, color: Color(0xFF94A3B8)),
                ),
              ],
            ),

            const SizedBox(height: 12),

            // Financial Breakup Grid
            Container(
              padding: const EdgeInsets.all(10),
              decoration: BoxDecoration(
                color: Theme.of(context).brightness == Brightness.dark
                    ? AppColors.darkSurface
                    : const Color(0xFFF1F5F9),
                borderRadius: BorderRadius.circular(10),
              ),
              child: Column(
                children: [
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      _buildMetric('Assessable Value', currencyFormatter.format(row.assessableValue)),
                      _buildMetric('Basic Duty (BCD+SWS)', currencyFormatter.format(row.totalBasicDuties), color: AppColors.bcdColor),
                    ],
                  ),
                  const SizedBox(height: 6),
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      _buildMetric('Expenses & Logistics', currencyFormatter.format(row.expensesTotal)),
                      _buildMetric('IGST Amount', currencyFormatter.format(row.igstAmount), color: AppColors.igstColor),
                    ],
                  ),
                  if (row.boeInterest > 0) ...[
                    const SizedBox(height: 6),
                    Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        _buildMetric('BOE Interest', currencyFormatter.format(row.boeInterest), color: Colors.orange),
                        const SizedBox.shrink(),
                      ],
                    ),
                  ],
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildMetric(String label, String value, {Color? color}) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          label,
          style: const TextStyle(fontSize: 10.5, color: Color(0xFF94A3B8)),
        ),
        const SizedBox(height: 1),
        Text(
          value,
          style: TextStyle(
            fontSize: 12.5,
            fontWeight: FontWeight.w600,
            color: color,
          ),
        ),
      ],
    );
  }
}
