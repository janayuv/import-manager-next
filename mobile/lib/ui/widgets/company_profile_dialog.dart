import 'package:flutter/material.dart';
import '../../constants/app_colors.dart';
import '../../models/company.dart';
import '../../services/company_service.dart';

class CompanyProfileDialog extends StatefulWidget {
  final Company company;

  const CompanyProfileDialog({
    super.key,
    required this.company,
  });

  @override
  State<CompanyProfileDialog> createState() => _CompanyProfileDialogState();
}

class _CompanyProfileDialogState extends State<CompanyProfileDialog> {
  final _formKey = GlobalKey<FormState>();

  late final TextEditingController _nameCtrl;
  late final TextEditingController _iecCtrl;
  late final TextEditingController _gstinCtrl;
  late final TextEditingController _panCtrl;
  late final TextEditingController _addressCtrl;
  late final TextEditingController _bankNameCtrl;
  late final TextEditingController _bankAccCtrl;
  late final TextEditingController _bankBranchCtrl;
  late final TextEditingController _bankSwiftCtrl;
  late final TextEditingController _fyCtrl;

  late String _currency;
  bool _submitting = false;

  final List<String> _currencies = ['INR', 'USD', 'EUR', 'GBP', 'AED', 'SGD', 'JPY'];

  @override
  void initState() {
    super.initState();
    _nameCtrl = TextEditingController(text: widget.company.name);
    _iecCtrl = TextEditingController(text: widget.company.iecCode ?? '');
    _gstinCtrl = TextEditingController(text: widget.company.gstin ?? '');
    _panCtrl = TextEditingController(text: widget.company.pan ?? '');
    _addressCtrl = TextEditingController(text: widget.company.registeredAddress ?? '');
    _bankNameCtrl = TextEditingController(text: widget.company.bankName ?? '');
    _bankAccCtrl = TextEditingController(text: widget.company.bankAccountNo ?? '');
    _bankBranchCtrl = TextEditingController(text: widget.company.bankBranch ?? '');
    _bankSwiftCtrl = TextEditingController(text: widget.company.bankSwift ?? '');
    _fyCtrl = TextEditingController(text: widget.company.financialYear ?? '2026-2027');
    _currency = widget.company.baseCurrency.isNotEmpty ? widget.company.baseCurrency : 'INR';
    if (!_currencies.contains(_currency)) {
      _currencies.add(_currency);
    }
  }

  @override
  void dispose() {
    _nameCtrl.dispose();
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

  Future<void> _handleSave() async {
    if (!_formKey.currentState!.validate()) return;

    setState(() => _submitting = true);
    try {
      final input = UpdateCompanyInput(
        id: widget.company.id,
        name: _nameCtrl.text.trim(),
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

      final updated = await CompanyService.instance.updateCompanyProfile(input);

      if (mounted) {
        Navigator.of(context).pop(updated);
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('✓ Profile updated for "${updated.name}"'),
            backgroundColor: AppColors.delivered,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Update error: ${e.toString().replaceAll('Exception: ', '')}'),
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
                    padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                    decoration: BoxDecoration(
                      color: AppColors.primary,
                      borderRadius: BorderRadius.circular(6),
                    ),
                    child: Text(
                      widget.company.shortCode,
                      style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 11, color: Colors.white),
                    ),
                  ),
                  const SizedBox(width: 12),
                  const Expanded(
                    child: Text(
                      'Company Identity & Profile',
                      style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold, color: Colors.white),
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
                      TextFormField(
                        controller: _nameCtrl,
                        decoration: const InputDecoration(
                          labelText: 'Company Legal Name *',
                          prefixIcon: Icon(Icons.business, size: 18),
                        ),
                        validator: (v) => v == null || v.trim().isEmpty ? 'Company Name is required' : null,
                      ),
                      const SizedBox(height: 12),
                      Row(
                        children: [
                          Expanded(
                            child: DropdownButtonFormField<String>(
                              initialValue: _currency,
                              decoration: const InputDecoration(
                                labelText: 'Base Currency',
                              ),
                              dropdownColor: AppColors.darkCard,
                              items: _currencies.map((c) => DropdownMenuItem(value: c, child: Text(c))).toList(),
                              onChanged: (val) {
                                if (val != null) setState(() => _currency = val);
                              },
                            ),
                          ),
                          const SizedBox(width: 12),
                          Expanded(
                            child: TextFormField(
                              controller: _fyCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Financial Year',
                                hintText: '2026-2027',
                              ),
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 18),

                      const Text(
                        'STATUTORY REGISTRATIONS',
                        style: TextStyle(fontSize: 11, fontWeight: FontWeight.bold, color: AppColors.primaryLight, letterSpacing: 0.8),
                      ),
                      const SizedBox(height: 10),
                      TextFormField(
                        controller: _iecCtrl,
                        decoration: const InputDecoration(
                          labelText: 'IEC Code (Import Export Code)',
                          hintText: '10-digit DGFT Code',
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
                      const SizedBox(height: 18),

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
                          hintText: 'Full physical registered address',
                          prefixIcon: Icon(Icons.location_on, size: 18),
                        ),
                      ),
                      const SizedBox(height: 18),

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
                        : const Icon(Icons.save, size: 18),
                    label: Text(_submitting ? 'Saving...' : 'Save Profile'),
                    style: ElevatedButton.styleFrom(
                      backgroundColor: AppColors.primary,
                      foregroundColor: Colors.white,
                      padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 12),
                      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
                    ),
                    onPressed: _submitting ? null : _handleSave,
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
