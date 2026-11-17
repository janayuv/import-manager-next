import 'package:flutter/material.dart';
import '../../constants/app_colors.dart';
import '../../models/item_part.dart';

class PartDutyCard extends StatelessWidget {
  final ItemPart item;
  final VoidCallback? onTap;

  const PartDutyCard({
    super.key,
    required this.item,
    this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final effectiveDuty = item.effectiveDutyRatePercent;

    return Card(
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(14),
        child: Padding(
          padding: const EdgeInsets.all(15),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Part Number + HSN Code
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Expanded(
                    child: Text(
                      item.partNumber,
                      style: const TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.bold,
                        letterSpacing: -0.2,
                      ),
                    ),
                  ),
                  Container(
                    padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                    decoration: BoxDecoration(
                      color: AppColors.accent.withAlpha(30),
                      borderRadius: BorderRadius.circular(6),
                      border: Border.all(color: AppColors.accent.withAlpha(90), width: 0.8),
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        const Icon(Icons.tag, size: 12, color: AppColors.accent),
                        const SizedBox(width: 4),
                        Text(
                          item.hsnCode.isNotEmpty ? item.hsnCode : 'No HS Code',
                          style: const TextStyle(
                            color: AppColors.accent,
                            fontSize: 12,
                            fontWeight: FontWeight.w700,
                            fontFamily: 'monospace',
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 6),

              // Description
              Text(
                item.itemDescription,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(
                  fontSize: 13.5,
                  color: Color(0xFF94A3B8),
                  height: 1.3,
                ),
              ),

              const SizedBox(height: 12),

              // Duty Structure Grid
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
                      mainAxisAlignment: MainAxisAlignment.spaceAround,
                      children: [
                        _buildDutyPill('BCD', '${item.bcd}%', AppColors.bcdColor),
                        _buildDutyPill('SWS', '${item.sws}%', AppColors.swsColor),
                        _buildDutyPill('IGST', '${item.igst}%', AppColors.igstColor),
                        _buildDutyPill('Effective', '${effectiveDuty.toStringAsFixed(2)}%', AppColors.totalDuty, isBold: true),
                      ],
                    ),
                  ],
                ),
              ),

              const SizedBox(height: 12),

              // Footer: Supplier & Unit Price
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Row(
                    children: [
                      const Icon(Icons.business_outlined, size: 14, color: Color(0xFF64748B)),
                      const SizedBox(width: 5),
                      Text(
                        item.supplierName.isNotEmpty ? item.supplierName : 'Origin: ${item.countryOfOrigin}',
                        style: const TextStyle(fontSize: 12, color: Color(0xFF94A3B8)),
                      ),
                    ],
                  ),
                  Text(
                    '${item.currency} ${item.unitPrice.toStringAsFixed(2)} / ${item.unit}',
                    style: const TextStyle(
                      fontSize: 13.5,
                      fontWeight: FontWeight.w700,
                      color: AppColors.primaryLight,
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildDutyPill(String title, String value, Color color, {bool isBold = false}) {
    return Column(
      children: [
        Text(
          title,
          style: TextStyle(
            fontSize: 10.5,
            fontWeight: FontWeight.w600,
            color: color,
          ),
        ),
        const SizedBox(height: 2),
        Text(
          value,
          style: TextStyle(
            fontSize: 12.5,
            fontWeight: isBold ? FontWeight.w800 : FontWeight.w600,
            color: isBold ? color : null,
          ),
        ),
      ],
    );
  }
}
