import 'package:flutter/material.dart';
import '../../constants/app_colors.dart';
import '../../models/company.dart';
import '../../services/company_service.dart';
import 'create_company_dialog.dart';
import 'company_profile_dialog.dart';

class CompanySelectorSheet extends StatefulWidget {
  const CompanySelectorSheet({super.key});

  @override
  State<CompanySelectorSheet> createState() => _CompanySelectorSheetState();
}

class _CompanySelectorSheetState extends State<CompanySelectorSheet> {
  bool _switching = false;

  Future<void> _handleSwitch(Company company) async {
    if (CompanyService.instance.activeCompany.id == company.id) {
      Navigator.of(context).pop();
      return;
    }

    setState(() => _switching = true);
    try {
      await CompanyService.instance.switchCompany(company.id);
      if (mounted) {
        Navigator.of(context).pop();
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('✓ Switched to "${company.name}"'),
            backgroundColor: AppColors.delivered,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Switch error: ${e.toString().replaceAll('Exception: ', '')}'),
            backgroundColor: AppColors.delayed,
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _switching = false);
    }
  }

  Future<void> _handleDelete(Company company) async {
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: AppColors.darkCard,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(14)),
        title: const Row(
          children: [
            Icon(Icons.warning_amber_rounded, color: AppColors.delayed, size: 24),
            SizedBox(width: 8),
            Text('Delete Company?', style: TextStyle(fontWeight: FontWeight.bold, fontSize: 17)),
          ],
        ),
        content: Text(
          'Are you sure you want to permanently delete "${company.name}" and its dedicated SQLite database (${company.dbFileName})?\n\nThis action cannot be undone.',
          style: const TextStyle(fontSize: 13.5, color: Color(0xFFCBD5E1), height: 1.4),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: const Text('Cancel', style: TextStyle(color: Colors.white70)),
          ),
          ElevatedButton(
            style: ElevatedButton.styleFrom(
              backgroundColor: AppColors.delayed,
              foregroundColor: Colors.white,
            ),
            onPressed: () => Navigator.of(ctx).pop(true),
            child: const Text('Delete Permanently'),
          ),
        ],
      ),
    );

    if (confirm != true) return;

    try {
      await CompanyService.instance.deleteCompany(company.id);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('✓ Deleted company "${company.name}"'),
            backgroundColor: AppColors.delivered,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Delete error: ${e.toString().replaceAll('Exception: ', '')}'),
            backgroundColor: AppColors.delayed,
          ),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: CompanyService.instance,
      builder: (context, _) {
        final companies = CompanyService.instance.companies;
        final activeCompany = CompanyService.instance.activeCompany;

        return Container(
          decoration: const BoxDecoration(
            color: AppColors.darkSurface,
            borderRadius: BorderRadius.vertical(top: Radius.circular(22)),
            border: Border(top: BorderSide(color: AppColors.darkBorder)),
          ),
          padding: EdgeInsets.only(
            bottom: MediaQuery.of(context).viewInsets.bottom + 16,
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              // Pull bar
              Center(
                child: Container(
                  margin: const EdgeInsets.symmetric(vertical: 10),
                  width: 40,
                  height: 4,
                  decoration: BoxDecoration(
                    color: Colors.white30,
                    borderRadius: BorderRadius.circular(2),
                  ),
                ),
              ),

              // Title bar
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 6),
                child: Row(
                  children: [
                    const Icon(Icons.corporate_fare, color: AppColors.primaryLight, size: 22),
                    const SizedBox(width: 10),
                    const Text(
                      'Companies & Databases',
                      style: TextStyle(
                        fontSize: 18,
                        fontWeight: FontWeight.bold,
                        color: Colors.white,
                      ),
                    ),
                    const SizedBox(width: 8),
                    Container(
                      padding: const EdgeInsets.symmetric(horizontal: 7, vertical: 2),
                      decoration: BoxDecoration(
                        color: AppColors.primary.withAlpha(50),
                        borderRadius: BorderRadius.circular(10),
                      ),
                      child: Text(
                        '${companies.length}',
                        style: const TextStyle(
                          fontSize: 11,
                          fontWeight: FontWeight.bold,
                          color: AppColors.primaryLight,
                        ),
                      ),
                    ),
                    const Spacer(),
                    IconButton(
                      icon: const Icon(Icons.close, color: Colors.white70, size: 20),
                      onPressed: () => Navigator.of(context).pop(),
                    ),
                  ],
                ),
              ),
              const Divider(color: AppColors.darkBorder, height: 1),

              // List of companies
              Flexible(
                child: ListView.separated(
                  shrinkWrap: true,
                  padding: const EdgeInsets.all(16),
                  itemCount: companies.length,
                  separatorBuilder: (_, _) => const SizedBox(height: 10),
                  itemBuilder: (context, index) {
                    final comp = companies[index];
                    final isActive = comp.id == activeCompany.id;

                    return InkWell(
                      onTap: _switching ? null : () => _handleSwitch(comp),
                      borderRadius: BorderRadius.circular(14),
                      child: Container(
                        padding: const EdgeInsets.all(14),
                        decoration: BoxDecoration(
                          color: isActive ? AppColors.primary.withAlpha(25) : AppColors.darkCard,
                          borderRadius: BorderRadius.circular(14),
                          border: Border.all(
                            color: isActive ? AppColors.primary : AppColors.darkBorder,
                            width: isActive ? 1.5 : 1,
                          ),
                        ),
                        child: Row(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            // Short code badge
                            Container(
                              padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 6),
                              decoration: BoxDecoration(
                                color: isActive ? AppColors.primary : AppColors.darkSurface,
                                borderRadius: BorderRadius.circular(8),
                                border: Border.all(
                                  color: isActive ? AppColors.primaryLight : AppColors.darkBorder,
                                ),
                              ),
                              child: Text(
                                comp.shortCode,
                                style: const TextStyle(
                                  fontSize: 11,
                                  fontWeight: FontWeight.w800,
                                  color: Colors.white,
                                  letterSpacing: 0.5,
                                ),
                              ),
                            ),
                            const SizedBox(width: 12),

                            // Company info
                            Expanded(
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Row(
                                    children: [
                                      Expanded(
                                        child: Text(
                                          comp.name,
                                          style: TextStyle(
                                            fontSize: 14.5,
                                            fontWeight: FontWeight.w700,
                                            color: isActive ? Colors.white : const Color(0xFFE2E8F0),
                                          ),
                                        ),
                                      ),
                                      if (comp.isDefault)
                                        Container(
                                          margin: const EdgeInsets.only(left: 6),
                                          padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 1.5),
                                          decoration: BoxDecoration(
                                            color: Colors.blueGrey.withAlpha(60),
                                            borderRadius: BorderRadius.circular(4),
                                          ),
                                          child: const Text(
                                            'PRIMARY',
                                            style: TextStyle(fontSize: 9.5, fontWeight: FontWeight.bold, color: Colors.white70),
                                          ),
                                        ),
                                    ],
                                  ),
                                  const SizedBox(height: 4),

                                  // Statutory & Details
                                  Wrap(
                                    spacing: 8,
                                    runSpacing: 4,
                                    children: [
                                      if (comp.iecCode != null && comp.iecCode!.isNotEmpty)
                                        Text(
                                          'IEC: ${comp.iecCode}',
                                          style: const TextStyle(fontSize: 11, color: Color(0xFF94A3B8)),
                                        ),
                                      if (comp.gstin != null && comp.gstin!.isNotEmpty)
                                        Text(
                                          'GSTIN: ${comp.gstin}',
                                          style: const TextStyle(fontSize: 11, color: Color(0xFF94A3B8)),
                                        ),
                                      Text(
                                        'Currency: ${comp.baseCurrency}',
                                        style: const TextStyle(fontSize: 11, color: Color(0xFF94A3B8)),
                                      ),
                                    ],
                                  ),
                                ],
                              ),
                            ),

                            // Action buttons
                            Row(
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                // Edit profile
                                IconButton(
                                  icon: const Icon(Icons.edit_outlined, size: 18, color: Colors.white70),
                                  tooltip: 'Edit Profile',
                                  onPressed: () {
                                    showDialog(
                                      context: context,
                                      builder: (ctx) => CompanyProfileDialog(company: comp),
                                    );
                                  },
                                ),

                                // Delete (if not default and not active)
                                if (!comp.isDefault && !isActive)
                                  IconButton(
                                    icon: const Icon(Icons.delete_outline, size: 18, color: AppColors.delayed),
                                    tooltip: 'Delete Company',
                                    onPressed: () => _handleDelete(comp),
                                  ),

                                // Active checkmark
                                if (isActive)
                                  Container(
                                    margin: const EdgeInsets.only(left: 4),
                                    padding: const EdgeInsets.all(4),
                                    decoration: const BoxDecoration(
                                      color: AppColors.delivered,
                                      shape: BoxShape.circle,
                                    ),
                                    child: const Icon(Icons.check, size: 14, color: Colors.white),
                                  ),
                              ],
                            ),
                          ],
                        ),
                      ),
                    );
                  },
                ),
              ),

              // Bottom button: Create New Company
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                child: SizedBox(
                  width: double.infinity,
                  child: ElevatedButton.icon(
                    icon: const Icon(Icons.add, size: 18),
                    label: const Text('Create New Company (Fresh Database)'),
                    style: ElevatedButton.styleFrom(
                      backgroundColor: AppColors.primary,
                      foregroundColor: Colors.white,
                      padding: const EdgeInsets.symmetric(vertical: 14),
                      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
                    ),
                    onPressed: () {
                      showDialog(
                        context: context,
                        builder: (ctx) => const CreateCompanyDialog(),
                      );
                    },
                  ),
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}
