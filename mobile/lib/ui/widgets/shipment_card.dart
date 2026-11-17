import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import '../../constants/app_colors.dart';
import '../../models/shipment.dart';
import 'status_badge.dart';

class ShipmentCard extends StatelessWidget {
  final Shipment shipment;
  final VoidCallback? onTap;

  const ShipmentCard({
    super.key,
    required this.shipment,
    this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final currencyFormatter = NumberFormat.currency(
      symbol: shipment.invoiceCurrency == 'USD'
          ? '\$'
          : (shipment.invoiceCurrency == 'EUR' ? '€' : '₹ '),
      decimalDigits: 2,
    );

    final days = shipment.daysUntilEta;
    Widget etaBadge;

    if (days != null) {
      if (days < 0 && !shipment.status.toLowerCase().contains('deliver')) {
        etaBadge = Container(
          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
          decoration: BoxDecoration(
            color: AppColors.delayed.withAlpha(35),
            borderRadius: BorderRadius.circular(6),
            border: Border.all(color: AppColors.delayed, width: 0.8),
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              const Icon(Icons.warning_amber_rounded, size: 12, color: AppColors.delayed),
              const SizedBox(width: 4),
              Text(
                'Delayed ${days.abs()}d',
                style: const TextStyle(
                  color: AppColors.delayed,
                  fontSize: 11,
                  fontWeight: FontWeight.bold,
                ),
              ),
            ],
          ),
        );
      } else if (days == 0) {
        etaBadge = Container(
          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
          decoration: BoxDecoration(
            color: AppColors.readyDelivery.withAlpha(40),
            borderRadius: BorderRadius.circular(6),
            border: Border.all(color: AppColors.readyDelivery, width: 0.8),
          ),
          child: const Text(
            'Arriving Today',
            style: TextStyle(
              color: AppColors.readyDelivery,
              fontSize: 11,
              fontWeight: FontWeight.bold,
            ),
          ),
        );
      } else {
        etaBadge = Container(
          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
          decoration: BoxDecoration(
            color: AppColors.delivered.withAlpha(30),
            borderRadius: BorderRadius.circular(6),
            border: Border.all(color: AppColors.delivered.withAlpha(90), width: 0.8),
          ),
          child: Text(
            'In $days days',
            style: const TextStyle(
              color: AppColors.delivered,
              fontSize: 11,
              fontWeight: FontWeight.bold,
            ),
          ),
        );
      }
    } else {
      etaBadge = const SizedBox.shrink();
    }

    return Card(
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(14),
        child: Padding(
          padding: const EdgeInsets.all(15),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Header: Invoice No + Status
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          shipment.invoiceNumber,
                          style: const TextStyle(
                            fontSize: 16,
                            fontWeight: FontWeight.bold,
                            letterSpacing: -0.2,
                          ),
                        ),
                        const SizedBox(height: 3),
                        Text(
                          shipment.supplierName.isNotEmpty
                              ? shipment.supplierName
                              : 'Supplier #${shipment.supplierId}',
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                            color: Color(0xFF94A3B8),
                            fontSize: 13,
                            fontWeight: FontWeight.w500,
                          ),
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(width: 8),
                  StatusBadge(status: shipment.status),
                ],
              ),
              const SizedBox(height: 12),

              // Mode, Vessel / Container
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
                decoration: BoxDecoration(
                  color: Theme.of(context).brightness == Brightness.dark
                      ? AppColors.darkSurface
                      : const Color(0xFFF1F5F9),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Row(
                  children: [
                    Icon(
                      shipment.shipmentMode.toUpperCase().contains('AIR')
                          ? Icons.airplanemode_active
                          : Icons.directions_boat,
                      size: 16,
                      color: AppColors.accent,
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        shipment.vesselName.isNotEmpty
                            ? shipment.vesselName
                            : (shipment.blAwbNumber.isNotEmpty ? 'BL: ${shipment.blAwbNumber}' : 'Direct Shipment'),
                        style: const TextStyle(
                          fontSize: 12.5,
                          fontWeight: FontWeight.w600,
                        ),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                    if (shipment.containerNumber.isNotEmpty) ...[
                      const SizedBox(width: 6),
                      Text(
                        '#${shipment.containerNumber}',
                        style: const TextStyle(
                          fontSize: 11.5,
                          color: Color(0xFF94A3B8),
                          fontFamily: 'monospace',
                        ),
                      ),
                    ],
                  ],
                ),
              ),

              const SizedBox(height: 12),

              // ETD -> ETA timeline row
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const Text(
                        'ETD (Depart)',
                        style: TextStyle(color: Color(0xFF94A3B8), fontSize: 11),
                      ),
                      const SizedBox(height: 2),
                      Text(
                        shipment.etd.isNotEmpty ? shipment.etd : 'TBD',
                        style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 13),
                      ),
                    ],
                  ),
                  const Icon(Icons.arrow_forward_rounded, size: 16, color: Color(0xFF64748B)),
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.end,
                    children: [
                      const Text(
                        'ETA (Arrival)',
                        style: TextStyle(color: Color(0xFF94A3B8), fontSize: 11),
                      ),
                      const SizedBox(height: 2),
                      Text(
                        shipment.eta.isNotEmpty ? shipment.eta : 'TBD',
                        style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 13),
                      ),
                    ],
                  ),
                ],
              ),

              const SizedBox(height: 10),
              const Divider(height: 1, thickness: 0.6, color: Color(0xFF334155)),
              const SizedBox(height: 10),

              // Bottom value and ETA badge
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text(
                    currencyFormatter.format(shipment.invoiceValue),
                    style: const TextStyle(
                      fontWeight: FontWeight.w700,
                      fontSize: 14.5,
                      color: AppColors.primaryLight,
                    ),
                  ),
                  etaBadge,
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}
