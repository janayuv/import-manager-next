import 'dart:async';
import 'package:flutter/material.dart';
import '../../constants/app_colors.dart';
import '../../database/db_helper.dart';
import '../../models/invoice.dart';
import '../../services/company_service.dart';
import '../widgets/invoice_card.dart';
import '../widgets/invoice_detail_sheet.dart';
import '../widgets/company_switcher_badge.dart';

class InvoiceSearchScreen extends StatefulWidget {
  const InvoiceSearchScreen({super.key});

  @override
  State<InvoiceSearchScreen> createState() => _InvoiceSearchScreenState();
}

class _InvoiceSearchScreenState extends State<InvoiceSearchScreen> {
  List<Invoice> _invoices = [];
  bool _loading = true;
  String _query = '';
  String _selectedStatus = 'All';
  late final TextEditingController _searchController;
  Timer? _debounceTimer;

  final List<String> _statuses = ['All', 'Finalized', 'Draft'];

  @override
  void initState() {
    super.initState();
    _searchController = TextEditingController();
    CompanyService.instance.addListener(_onCompanyChanged);
    _loadInvoices();
  }

  @override
  void dispose() {
    CompanyService.instance.removeListener(_onCompanyChanged);
    _debounceTimer?.cancel();
    _searchController.dispose();
    super.dispose();
  }

  void _onCompanyChanged() {
    if (mounted) {
      _loadInvoices();
    }
  }

  void _onSearchChanged(String val) {
    if ((_query.isEmpty && val.isNotEmpty) || (_query.isNotEmpty && val.isEmpty)) {
      setState(() => _query = val);
    } else {
      _query = val;
    }
    _debounceTimer?.cancel();
    _debounceTimer = Timer(const Duration(milliseconds: 250), () {
      if (!mounted) return;
      _loadInvoices();
    });
  }

  Future<void> _loadInvoices() async {
    setState(() => _loading = true);
    try {
      final list = await DBHelper.instance.searchInvoices(
        query: _query,
        status: _selectedStatus,
      );
      if (mounted) {
        setState(() {
          _invoices = list;
        });
      }
    } catch (e, st) {
      debugPrint('Error loading invoices: $e\n$st');
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Invoice Search'),
        actions: const [
          CompanySwitcherBadge(),
          SizedBox(width: 14),
        ],
      ),
      body: RefreshIndicator(
        onRefresh: _loadInvoices,
        child: Column(
          children: [
            // Search Input
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 12, 16, 6),
              child: TextField(
                controller: _searchController,
                autofocus: false,
                onChanged: _onSearchChanged,
                decoration: InputDecoration(
                  hintText: 'Search by Invoice Number, Supplier...',
                  prefixIcon: const Icon(Icons.search, size: 20, color: Color(0xFF94A3B8)),
                  suffixIcon: _query.isNotEmpty
                      ? IconButton(
                          icon: const Icon(Icons.clear, size: 18),
                          onPressed: () {
                            _debounceTimer?.cancel();
                            _searchController.clear();
                            setState(() => _query = '');
                            _loadInvoices();
                          },
                        )
                      : null,
                ),
              ),
            ),

            // Status Filter Chips
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
              child: Row(
                children: _statuses.map((st) {
                  final isSelected = _selectedStatus == st;
                  return Padding(
                    padding: const EdgeInsets.only(right: 8),
                    child: ChoiceChip(
                      label: Text(
                        st,
                        style: TextStyle(
                          fontSize: 12,
                          fontWeight: isSelected ? FontWeight.bold : FontWeight.w500,
                          color: isSelected ? Colors.white : const Color(0xFF94A3B8),
                        ),
                      ),
                      selected: isSelected,
                      selectedColor: AppColors.primary,
                      backgroundColor: Theme.of(context).cardColor,
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(8),
                        side: BorderSide(
                          color: isSelected ? AppColors.primary : AppColors.darkBorder,
                        ),
                      ),
                      onSelected: (selected) {
                        if (selected) {
                          setState(() => _selectedStatus = st);
                          _loadInvoices();
                        }
                      },
                    ),
                  );
                }).toList(),
              ),
            ),
            const SizedBox(height: 6),

            // Invoices List
            Expanded(
              child: _loading
                  ? const Center(child: CircularProgressIndicator())
                  : _invoices.isEmpty
                      ? Center(
                          child: Column(
                            mainAxisAlignment: MainAxisAlignment.center,
                            children: [
                              Icon(Icons.search_off, size: 48, color: Colors.grey.withAlpha(120)),
                              const SizedBox(height: 12),
                              const Text(
                                'No invoices match your search',
                                style: TextStyle(color: Color(0xFF94A3B8), fontSize: 14),
                              ),
                            ],
                          ),
                        )
                      : ListView.builder(
                          padding: const EdgeInsets.only(bottom: 24),
                          itemCount: _invoices.length,
                          itemBuilder: (context, index) {
                            final inv = _invoices[index];
                            return InvoiceCard(
                              invoice: inv,
                              onTap: () => InvoiceDetailSheet.show(context, inv),
                            );
                          },
                        ),
            ),
          ],
        ),
      ),
    );
  }
}
