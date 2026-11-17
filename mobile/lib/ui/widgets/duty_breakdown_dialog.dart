import 'package:flutter/material.dart';
import '../../constants/app_colors.dart';
import '../../models/item_part.dart';

class DutyBreakdownDialog extends StatelessWidget {
  final ItemPart item;

  const DutyBreakdownDialog({super.key, required this.item});

  static void show(BuildContext context, ItemPart item) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (_) => DutyBreakdownDialog(item: item),
    );
  }

  @override
  Widget build(BuildContext context) {
    final bcdPct = item.bcdPercent;
    final swsPct = item.swsPercent;
    final igstPct = item.igstPercent;

    // Simulation on Base 10,000 INR
    const baseValue = 10000.0;
    final bcdVal = baseValue * (bcdPct / 100.0);
    final swsVal = bcdVal * (swsPct / 100.0);
    final subtotal = baseValue + bcdVal + swsVal;
    final igstVal = subtotal * (igstPct / 100.0);
    final totalDuties = bcdVal + swsVal + igstVal;
    final landedFactor = baseValue + totalDuties;

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
      child: SingleChildScrollView(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
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

            // Header
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        item.partNumber,
                        style: const TextStyle(fontSize: 19, fontWeight: FontWeight.bold),
                      ),
                      const SizedBox(height: 3),
                      Text(
                        'HSN / HS Code: ${item.hsnCode}',
                        style: const TextStyle(
                          color: AppColors.accent,
                          fontWeight: FontWeight.bold,
                          fontFamily: 'monospace',
                        ),
                      ),
                    ],
                  ),
                ),
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
                  decoration: BoxDecoration(
                    color: AppColors.totalDuty.withAlpha(30),
                    borderRadius: BorderRadius.circular(8),
                    border: Border.all(color: AppColors.totalDuty),
                  ),
                  child: Column(
                    children: [
                      const Text(
                        'EFFECTIVE',
                        style: TextStyle(fontSize: 9, fontWeight: FontWeight.bold, color: AppColors.totalDuty),
                      ),
                      Text(
                        '${item.effectiveDutyRatePercent.toStringAsFixed(2)}%',
                        style: const TextStyle(fontSize: 14, fontWeight: FontWeight.bold, color: AppColors.totalDuty),
                      ),
                    ],
                  ),
                ),
              ],
            ),
            const SizedBox(height: 14),

            Text(
              item.itemDescription,
              style: const TextStyle(fontSize: 13.5, color: Color(0xFF94A3B8)),
            ),

            const SizedBox(height: 18),
            const Text(
              'Customs Duty Calculation Simulation (Base ₹10,000)',
              style: TextStyle(fontSize: 13.5, fontWeight: FontWeight.bold),
            ),
            const SizedBox(height: 10),

            // Duty Calculation Breakdown Table
            Container(
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: Theme.of(context).brightness == Brightness.dark
                    ? AppColors.darkSurface
                    : const Color(0xFFF1F5F9),
                borderRadius: BorderRadius.circular(12),
              ),
              child: Column(
                children: [
                  _buildDutyRow('Assessable Value (Base)', '₹ 10,000.00', null),
                  _buildDutyRow('Basic Customs Duty (BCD ${item.bcd}%)', '+ ₹ ${bcdVal.toStringAsFixed(2)}', AppColors.bcdColor),
                  _buildDutyRow('Social Welfare Surcharge (SWS ${item.sws}% on BCD)', '+ ₹ ${swsVal.toStringAsFixed(2)}', AppColors.swsColor),
                  _buildDutyRow('Subtotal (Assessable + BCD + SWS)', '₹ ${subtotal.toStringAsFixed(2)}', null, isSubtotal: true),
                  _buildDutyRow('Integrated GST (IGST ${item.igst}% on Subtotal)', '+ ₹ ${igstVal.toStringAsFixed(2)}', AppColors.igstColor),
                  const Divider(height: 14),
                  _buildDutyRow('Total Customs Duty & Taxes', '₹ ${totalDuties.toStringAsFixed(2)}', AppColors.totalDuty, isBold: true),
                  _buildDutyRow('Total Landed Cost Factor', '₹ ${landedFactor.toStringAsFixed(2)}', AppColors.ldcBadge, isBold: true),
                ],
              ),
            ),

            const SizedBox(height: 16),
            const Text(
              'Part Specifications & Master Data',
              style: TextStyle(fontSize: 14, fontWeight: FontWeight.bold),
            ),
            const SizedBox(height: 8),

            _buildDetailRow('Category', item.category.isNotEmpty ? item.category : 'N/A'),
            _buildDetailRow('End Use', item.endUse.isNotEmpty ? item.endUse : 'N/A'),
            _buildDetailRow('Country of Origin', item.countryOfOrigin.isNotEmpty ? item.countryOfOrigin : 'N/A'),
            _buildDetailRow('Master Unit Price', '${item.currency} ${item.unitPrice.toStringAsFixed(2)} / ${item.unit}'),
            if (item.technicalWriteUp.isNotEmpty) ...[
              const SizedBox(height: 10),
              const Text('Technical Write-Up:', style: TextStyle(fontSize: 12, color: Color(0xFF94A3B8))),
              const SizedBox(height: 4),
              Text(item.technicalWriteUp, style: const TextStyle(fontSize: 12.5)),
            ],

            const SizedBox(height: 20),
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
      ),
    );
  }

  Widget _buildDutyRow(String title, String amount, Color? color, {bool isSubtotal = false, bool isBold = false}) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Expanded(
            child: Text(
              title,
              style: TextStyle(
                fontSize: 12,
                fontStyle: isSubtotal ? FontStyle.italic : null,
                color: isSubtotal ? const Color(0xFF94A3B8) : null,
              ),
            ),
          ),
          Text(
            amount,
            style: TextStyle(
              fontSize: 12.5,
              fontWeight: isBold ? FontWeight.bold : FontWeight.w600,
              color: color,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildDetailRow(String label, String value) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 5),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(label, style: const TextStyle(fontSize: 13, color: Color(0xFF94A3B8))),
          Text(value, style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w600)),
        ],
      ),
    );
  }
}
