import 'package:flutter/material.dart';
import '../../constants/app_colors.dart';
import '../../models/company.dart';
import '../../services/company_service.dart';

class CreateCompanyDialog extends StatefulWidget {
  const CreateCompanyDialog({super.key});

  @override
  State<CreateCompanyDialog> createState() => _CreateCompanyDialogState();
}

class _CreateCompanyDialogState extends State<CreateCompanyDialog> {
  final _formKey = GlobalKey<FormState>();

  final _nameCtrl = TextEditingController();
  final _codeCtrl = TextEditingController();
  final _iecCtrl = TextEditingController();
  final _gstinCtrl = TextEditingController();
  final _panCtrl = TextEditingController();
  final _addressCtrl = TextEditingController();
  final _bankNameCtrl = TextEditingController();
  final _bankAccCtrl = TextEditingController();
  final _bankBranchCtrl = TextEditingController();
  final _bankSwiftCtrl = TextEditingController();
  final _fyCtrl = TextEditingController(text: '2026-2027');

  String _currency = 'INR';
  bool _submitting = false;

  final List<String> _currencies = ['INR', 'USD', 'EUR', 'GBP', 'AED', 'SGD', 'JPY'];

  @override
  void dispose() {
    _nameCtrl.dispose();
    _codeCtrl.dispose();
    _iecCtrl.dispose();
    _gstinCtrl.dispose();
    _panCtrl.dispose();
    _addressCtrl.dispose();
    _bankNameCtrl.dispose();
    _bankAccCtrl.dispose();
    _bankBranchCtrl.dispose();
    _bankSwiftCtrl.dispose();
    _fyCtrl.dispose();
    super.dispose();
  }

  Future<void> _handleCreate() async {
    if (!_formKey.currentState!.validate()) return;

    setState(() => _submitting = true);
    try {
      final input = CreateCompanyInput(
        name: _nameCtrl.text.trim(),
        shortCode: _codeCtrl.text.trim().toUpperCase(),
        iecCode: _iecCtrl.text.trim(),
        gstin: _gstinCtrl.text.trim(),
        pan: _panCtrl.text.trim(),
        registeredAddress: _addressCtrl.text.trim(),
        bankName: _bankNameCtrl.text.trim(),
        bankAccountNo: _bankAccCtrl.text.trim(),
        bankBranch: _bankBranchCtrl.text.trim(),
        bankSwift: _bankSwiftCtrl.text.trim(),
        baseCurrency: _currency,
        financialYear: _fyCtrl.text.trim(),
      );

      final created = await CompanyService.instance.createCompany(input);

      if (mounted) {
        Navigator.of(context).pop(created);
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('✓ Company "${created.name}" created with fresh database!'),
            backgroundColor: AppColors.delivered,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Error: ${e.toString().replaceAll('Exception: ', '')}'),
            backgroundColor: AppColors.delayed,
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Dialog(
      backgroundColor: AppColors.darkCard,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      insetPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 24),
      child: Container(
        constraints: const BoxConstraints(maxWidth: 480, maxHeight: 680),
        child: Column(
          children: [
            // Header
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 16),
              decoration: const BoxDecoration(
                border: Border(bottom: BorderSide(color: AppColors.darkBorder)),
              ),
              child: Row(
                children: [
                  Container(
                    padding: const EdgeInsets.all(8),
                    decoration: BoxDecoration(
                      color: AppColors.primary.withAlpha(40),
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: const Icon(Icons.add_business, color: AppColors.primaryLight, size: 20),
                  ),
                  const SizedBox(width: 12),
                  const Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          'Create New Company',
                          style: TextStyle(fontSize: 17, fontWeight: FontWeight.bold, color: Colors.white),
                        ),
                        Text(
                          'Provision dedicated clean database',
                          style: TextStyle(fontSize: 12, color: Color(0xFF94A3B8)),
                        ),
                      ],
                    ),
                  ),
                  IconButton(
                    icon: const Icon(Icons.close, color: Colors.white70, size: 20),
                    onPressed: () => Navigator.of(context).pop(),
                  ),
                ],
              ),
            ),

            // Form
            Expanded(
              child: SingleChildScrollView(
                padding: const EdgeInsets.all(20),
                child: Form(
                  key: _formKey,
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      // Fresh DB notice
                      Container(
                        padding: const EdgeInsets.all(12),
                        decoration: BoxDecoration(
                          color: AppColors.accent.withAlpha(25),
                          borderRadius: BorderRadius.circular(10),
                          border: Border.all(color: AppColors.accent.withAlpha(60)),
                        ),
                        child: const Row(
                          children: [
                            Icon(Icons.info_outline, color: AppColors.accent, size: 18),
                            SizedBox(width: 10),
                            Expanded(
                              child: Text(
                                'A separate, dedicated SQLite database will be initialized with 0 records for complete data isolation.',
                                style: TextStyle(fontSize: 11.5, color: Color(0xFFE2E8F0), height: 1.3),
                              ),
                            ),
                          ],
                        ),
                      ),
                      const SizedBox(height: 18),

                      // Section 1: General Details
                      const Text(
                        'COMPANY IDENTITY',
                        style: TextStyle(fontSize: 11, fontWeight: FontWeight.bold, color: AppColors.primaryLight, letterSpacing: 0.8),
                      ),
                      const SizedBox(height: 10),
                      TextFormField(
                        controller: _nameCtrl,
                        decoration: const InputDecoration(
                          labelText: 'Company Name *',
                          hintText: 'e.g. Apex Global Logistics Pvt Ltd',
                          prefixIcon: Icon(Icons.business, size: 18),
                        ),
                        validator: (v) => v == null || v.trim().isEmpty ? 'Company Name is required' : null,
                      ),
                      const SizedBox(height: 12),
                      Row(
                        children: [
                          Expanded(
                            flex: 3,
                            child: TextFormField(
                              controller: _codeCtrl,
                              textCapitalization: TextCapitalization.characters,
                              decoration: const InputDecoration(
                                labelText: 'Short Code *',
                                hintText: 'e.g. APEX',
                                prefixIcon: Icon(Icons.badge, size: 18),
                              ),
                              validator: (v) {
                                if (v == null || v.trim().isEmpty) return 'Code required';
                                if (v.trim().length > 10) return 'Max 10 chars';
                                return null;
                              },
                            ),
                          ),
                          const SizedBox(width: 12),
                          Expanded(
                            flex: 2,
                            child: DropdownButtonFormField<String>(
                              initialValue: _currency,
                              decoration: const InputDecoration(
                                labelText: 'Currency',
                              ),
                              dropdownColor: AppColors.darkCard,
                              items: _currencies.map((c) => DropdownMenuItem(value: c, child: Text(c))).toList(),
                              onChanged: (val) {
                                if (val != null) setState(() => _currency = val);
                              },
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 12),
                      TextFormField(
                        controller: _fyCtrl,
                        decoration: const InputDecoration(
                          labelText: 'Financial Year',
                          hintText: 'e.g. 2026-2027',
                          prefixIcon: Icon(Icons.date_range, size: 18),
                        ),
                      ),
                      const SizedBox(height: 20),

                      // Section 2: Statutory
                      const Text(
                        'STATUTORY REGISTRATIONS',
                        style: TextStyle(fontSize: 11, fontWeight: FontWeight.bold, color: AppColors.primaryLight, letterSpacing: 0.8),
                      ),
                      const SizedBox(height: 10),
                      TextFormField(
                        controller: _iecCtrl,
                        decoration: const InputDecoration(
                          labelText: 'IEC Code (Import Export Code)',
                          hintText: '10-digit DGFT IEC Code',
                          prefixIcon: Icon(Icons.verified, size: 18),
                        ),
                      ),
                      const SizedBox(height: 12),
                      Row(
                        children: [
                          Expanded(
                            child: TextFormField(
                              controller: _gstinCtrl,
                              textCapitalization: TextCapitalization.characters,
                              decoration: const InputDecoration(
                                labelText: 'GSTIN',
                                hintText: '15-digit GSTIN',
                              ),
                            ),
                          ),
                          const SizedBox(width: 12),
                          Expanded(
                            child: TextFormField(
                              controller: _panCtrl,
                              textCapitalization: TextCapitalization.characters,
                              decoration: const InputDecoration(
                                labelText: 'PAN',
                                hintText: '10-digit PAN',
                              ),
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 20),

                      // Section 3: Registered Address
                      const Text(
                        'REGISTERED ADDRESS',
                        style: TextStyle(fontSize: 11, fontWeight: FontWeight.bold, color: AppColors.primaryLight, letterSpacing: 0.8),
                      ),
                      const SizedBox(height: 10),
                      TextFormField(
                        controller: _addressCtrl,
                        maxLines: 2,
                        decoration: const InputDecoration(
                          labelText: 'Registered Corporate Address',
                          hintText: 'Street, City, State, PIN, Country',
                          prefixIcon: Icon(Icons.location_on, size: 18),
                        ),
                      ),
                      const SizedBox(height: 20),

                      // Section 4: Bank Details
                      const Text(
                        'BANKING & SWIFT DETAILS',
                        style: TextStyle(fontSize: 11, fontWeight: FontWeight.bold, color: AppColors.primaryLight, letterSpacing: 0.8),
                      ),
                      const SizedBox(height: 10),
                      TextFormField(
                        controller: _bankNameCtrl,
                        decoration: const InputDecoration(
                          labelText: 'Bank Name',
                          hintText: 'e.g. State Bank of India / HDFC Bank',
                          prefixIcon: Icon(Icons.account_balance, size: 18),
                        ),
                      ),
                      const SizedBox(height: 12),
                      TextFormField(
                        controller: _bankAccCtrl,
                        decoration: const InputDecoration(
                          labelText: 'Account Number',
                          hintText: 'Corporate Current Account Number',
                          prefixIcon: Icon(Icons.numbers, size: 18),
                        ),
                      ),
                      const SizedBox(height: 12),
                      Row(
                        children: [
                          Expanded(
                            child: TextFormField(
                              controller: _bankBranchCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Branch / IFSC',
                                hintText: 'e.g. HDFC0001234',
                              ),
                            ),
                          ),
                          const SizedBox(width: 12),
                          Expanded(
                            child: TextFormField(
                              controller: _bankSwiftCtrl,
                              textCapitalization: TextCapitalization.characters,
                              decoration: const InputDecoration(
                                labelText: 'SWIFT / BIC',
                                hintText: 'e.g. HDFCINBBXXX',
                              ),
                            ),
                          ),
                        ],
                      ),
                    ],
                  ),
                ),
              ),
            ),

            // Footer actions
            Container(
              padding: const EdgeInsets.all(16),
              decoration: const BoxDecoration(
                border: Border(top: BorderSide(color: AppColors.darkBorder)),
              ),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.end,
                children: [
                  TextButton(
                    onPressed: _submitting ? null : () => Navigator.of(context).pop(),
                    child: const Text('Cancel', style: TextStyle(color: Colors.white70)),
                  ),
                  const SizedBox(width: 12),
                  ElevatedButton.icon(
                    icon: _submitting
                        ? const SizedBox(
                            width: 16,
                            height: 16,
                            child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                          )
                        : const Icon(Icons.check, size: 18),
                    label: Text(_submitting ? 'Creating Database...' : 'Create Company'),
                    style: ElevatedButton.styleFrom(
                      backgroundColor: AppColors.primary,
                      foregroundColor: Colors.white,
                      padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 12),
                      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
                    ),
                    onPressed: _submitting ? null : _handleCreate,
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
