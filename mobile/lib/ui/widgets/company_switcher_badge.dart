import 'package:flutter/material.dart';
import '../../constants/app_colors.dart';
import '../../services/company_service.dart';
import 'company_selector_sheet.dart';

class CompanySwitcherBadge extends StatelessWidget {
  final VoidCallback? onCompanyChanged;

  const CompanySwitcherBadge({
    super.key,
    this.onCompanyChanged,
  });

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: CompanyService.instance,
      builder: (context, _) {
        final active = CompanyService.instance.activeCompany;

        return InkWell(
          onTap: () async {
            await showModalBottomSheet(
              context: context,
              isScrollControlled: true,
              backgroundColor: Colors.transparent,
              builder: (ctx) => const CompanySelectorSheet(),
            );
            onCompanyChanged?.call();
          },
          borderRadius: BorderRadius.circular(20),
          child: Container(
            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
            decoration: BoxDecoration(
              color: AppColors.primary.withAlpha(35),
              borderRadius: BorderRadius.circular(20),
              border: Border.all(
                color: AppColors.primaryLight.withAlpha(90),
                width: 1,
              ),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                  decoration: BoxDecoration(
                    color: AppColors.primary,
                    borderRadius: BorderRadius.circular(6),
                  ),
                  child: Text(
                    active.shortCode,
                    style: const TextStyle(
                      fontSize: 10,
                      fontWeight: FontWeight.w800,
                      color: Colors.white,
                      letterSpacing: 0.5,
                    ),
                  ),
                ),
                const SizedBox(width: 7),
                ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 110),
                  child: Text(
                    active.name,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                      fontSize: 12,
                      fontWeight: FontWeight.w600,
                      color: Colors.white,
                    ),
                  ),
                ),
                const SizedBox(width: 4),
                const Icon(
                  Icons.arrow_drop_down,
                  size: 16,
                  color: AppColors.primaryLight,
                ),
              ],
            ),
          ),
        );
      },
    );
  }
}
