import 'package:flutter/material.dart';
import '../../constants/app_colors.dart';

class StatusBadge extends StatelessWidget {
  final String status;

  const StatusBadge({super.key, required this.status});

  @override
  Widget build(BuildContext context) {
    final st = status.toLowerCase().trim();
    Color bg;
    Color fg;
    String label = status;

    switch (st) {
      case 'in-transit':
        bg = AppColors.inTransit.withAlpha(40);
        fg = AppColors.inTransit;
        label = 'IN TRANSIT';
        break;
      case 'customs-clearance':
        bg = AppColors.customsClearance.withAlpha(40);
        fg = AppColors.customsClearance;
        label = 'CUSTOMS CLEARANCE';
        break;
      case 'ready-dly':
      case 'ready-delivery':
        bg = AppColors.readyDelivery.withAlpha(40);
        fg = AppColors.readyDelivery;
        label = 'READY DELIVERY';
        break;
      case 'docs-rcvd':
      case 'docu-received':
        bg = AppColors.docsReceived.withAlpha(40);
        fg = const Color(0xFF94A3B8);
        label = 'DOCS RECEIVED';
        break;
      case 'in-warehouse':
        bg = Colors.teal.withAlpha(40);
        fg = Colors.tealAccent;
        label = 'IN WAREHOUSE';
        break;
      case 'delivered':
      case 'completed':
        bg = AppColors.delivered.withAlpha(40);
        fg = AppColors.delivered;
        label = 'DELIVERED';
        break;
      case 'cancelled':
        bg = AppColors.delayed.withAlpha(40);
        fg = AppColors.delayed;
        label = 'CANCELLED';
        break;
      case 'finalized':
        bg = AppColors.primary.withAlpha(40);
        fg = AppColors.primaryLight;
        label = 'FINALIZED';
        break;
      case 'draft':
        bg = Colors.orange.withAlpha(40);
        fg = Colors.orangeAccent;
        label = 'DRAFT';
        break;
      default:
        bg = Colors.grey.withAlpha(40);
        fg = Colors.grey;
        label = status.toUpperCase();
    }

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 4),
      decoration: BoxDecoration(
        color: bg,
        borderRadius: BorderRadius.circular(6),
        border: Border.all(color: fg.withAlpha(80), width: 0.8),
      ),
      child: Text(
        label,
        style: TextStyle(
          color: fg,
          fontSize: 10.5,
          fontWeight: FontWeight.w700,
          letterSpacing: 0.4,
        ),
      ),
    );
  }
}
