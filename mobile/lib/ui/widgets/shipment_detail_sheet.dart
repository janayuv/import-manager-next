import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import '../../constants/app_colors.dart';
import '../../models/shipment.dart';
import 'status_badge.dart';

class ShipmentDetailSheet extends StatelessWidget {
  final Shipment shipment;

  const ShipmentDetailSheet({super.key, required this.shipment});

  static void show(BuildContext context, Shipment shipment) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (_) => ShipmentDetailSheet(shipment: shipment),
    );
  }

  @override
  Widget build(BuildContext context) {
    final currencyFormatter = NumberFormat.currency(
      symbol: shipment.invoiceCurrency == 'USD'
          ? '\$'
          : (shipment.invoiceCurrency == 'EUR' ? '€' : '₹ '),
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
      child: SingleChildScrollView(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            // Drag Handle
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

            // View-Only Banner
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
              decoration: BoxDecoration(
                color: Colors.blue.withAlpha(25),
                borderRadius: BorderRadius.circular(8),
                border: Border.all(color: Colors.blue.withAlpha(70)),
              ),
              child: const Row(
                children: [
                  Icon(Icons.visibility_outlined, size: 14, color: Colors.blueAccent),
                  SizedBox(width: 8),
                  Text(
                    'VIEW ONLY MODE • Mobile Viewer',
                    style: TextStyle(
                      fontSize: 11,
                      fontWeight: FontWeight.w600,
                      color: Colors.blueAccent,
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 16),

            // Title & Status
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        shipment.invoiceNumber,
                        style: const TextStyle(fontSize: 20, fontWeight: FontWeight.bold),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        shipment.supplierName,
                        style: const TextStyle(fontSize: 14, color: Color(0xFF94A3B8)),
                      ),
                    ],
                  ),
                ),
                StatusBadge(status: shipment.status),
              ],
            ),
            const SizedBox(height: 18),

            // Timeline Box
            Container(
              padding: const EdgeInsets.all(14),
              decoration: BoxDecoration(
                color: Theme.of(context).brightness == Brightness.dark
                    ? AppColors.darkSurface
                    : const Color(0xFFF1F5F9),
                borderRadius: BorderRadius.circular(12),
              ),
              child: Column(
                children: [
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      _buildInfoCol('ETD (Departure)', shipment.etd.isNotEmpty ? shipment.etd : 'Not set'),
                      const Icon(Icons.flight_takeoff, size: 20, color: AppColors.accent),
                      _buildInfoCol('ETA (Arrival)', shipment.eta.isNotEmpty ? shipment.eta : 'Not set', alignRight: true),
                    ],
                  ),
                  if (shipment.daysUntilEta != null) ...[
                    const SizedBox(height: 10),
                    const Divider(height: 1),
                    const SizedBox(height: 10),
                    Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        const Text(
                          'Voyage Transit Status:',
                          style: TextStyle(fontSize: 12, color: Color(0xFF94A3B8)),
                        ),
                        Text(
                          shipment.isDelayed
                              ? 'Delayed by ${shipment.daysUntilEta!.abs()} days'
                              : (shipment.daysUntilEta == 0 ? 'Arriving Today' : 'Arrives in ${shipment.daysUntilEta} days'),
                          style: TextStyle(
                            fontSize: 12.5,
                            fontWeight: FontWeight.bold,
                            color: shipment.isDelayed ? AppColors.delayed : AppColors.delivered,
                          ),
                        ),
                      ],
                    ),
                  ],
                ],
              ),
            ),

            const SizedBox(height: 16),
            const Text(
              'Logistics & Vessel Details',
              style: TextStyle(fontSize: 14, fontWeight: FontWeight.bold),
            ),
            const SizedBox(height: 8),

            _buildDetailRow('Shipment Mode', shipment.shipmentMode),
            _buildDetailRow('Shipment Type', shipment.shipmentType),
            _buildDetailRow('Vessel Name', shipment.vesselName.isNotEmpty ? shipment.vesselName : 'N/A'),
            _buildDetailRow('Container Number', shipment.containerNumber.isNotEmpty ? shipment.containerNumber : 'N/A'),
            _buildDetailRow('BL / AWB Number', shipment.blAwbNumber.isNotEmpty ? shipment.blAwbNumber : 'N/A'),
            _buildDetailRow('BL / AWB Date', shipment.blAwbDate.isNotEmpty ? shipment.blAwbDate : 'N/A'),
            _buildDetailRow('Incoterm', shipment.incoterm),
            _buildDetailRow('Gross Weight (KG)', '${shipment.grossWeightKg.toStringAsFixed(1)} kg'),
            _buildDetailRow('Commercial Value', currencyFormatter.format(shipment.invoiceValue)),
            _buildDetailRow('Delivery Date', shipment.dateOfDelivery.isNotEmpty ? shipment.dateOfDelivery : 'Pending'),

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

  Widget _buildInfoCol(String label, String value, {bool alignRight = false}) {
    return Column(
      crossAxisAlignment: alignRight ? CrossAxisAlignment.end : CrossAxisAlignment.start,
      children: [
        Text(label, style: const TextStyle(fontSize: 11, color: Color(0xFF94A3B8))),
        const SizedBox(height: 3),
        Text(value, style: const TextStyle(fontSize: 14, fontWeight: FontWeight.bold)),
      ],
    );
  }

  Widget _buildDetailRow(String label, String value) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(label, style: const TextStyle(fontSize: 13, color: Color(0xFF94A3B8))),
          Flexible(
            child: Text(
              value,
              textAlign: TextAlign.right,
              style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w600),
            ),
          ),
        ],
      ),
    );
  }
}
