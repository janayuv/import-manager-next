import 'dart:async';
import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import '../../constants/app_colors.dart';
import '../../database/db_helper.dart';
import '../../models/landed_cost.dart';
import '../../services/company_service.dart';
import '../widgets/ldc_row_card.dart';
import '../widgets/company_switcher_badge.dart';

class LdcReportScreen extends StatefulWidget {
  const LdcReportScreen({super.key});

  @override
  State<LdcReportScreen> createState() => _LdcReportScreenState();
}

class _LdcReportScreenState extends State<LdcReportScreen> {
  List<LandedCostRow> _rows = [];
  LandedCostSummary? _summary;
  bool _loading = true;
  String _search = '';
  late final TextEditingController _searchController;
  Timer? _debounceTimer;

  @override
  void initState() {
    super.initState();
    _searchController = TextEditingController();
    CompanyService.instance.addListener(_onCompanyChanged);
    _loadReport();
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
      _loadReport();
    }
  }

  void _onSearchChanged(String val) {
    if ((_search.isEmpty && val.isNotEmpty) || (_search.isNotEmpty && val.isEmpty)) {
      setState(() => _search = val);
    } else {
      _search = val;
    }
    _debounceTimer?.cancel();
    _debounceTimer = Timer(const Duration(milliseconds: 250), () {
      if (!mounted) return;
      _loadReport();
    });
  }

  Future<void> _loadReport() async {
    setState(() => _loading = true);
    try {
      final data = await DBHelper.instance.getLandedCostRows(search: _search);
      final sum = LandedCostSummary.fromRows(data);
      if (mounted) {
        setState(() {
          _rows = data;
          _summary = sum;
        });
      }
    } catch (e, st) {
      debugPrint('Error loading LDC report: $e\n$st');
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final currencyFormatter = NumberFormat.currency(symbol: '₹ ', decimalDigits: 0);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Landed Duty Cost (LDC)'),
        actions: const [
          CompanySwitcherBadge(),
          SizedBox(width: 14),
        ],
      ),
      body: RefreshIndicator(
        onRefresh: _loadReport,
        child: Column(
          children: [
            // KPI Summary Dashboard Cards
            if (_summary != null) ...[
              Container(
                margin: const EdgeInsets.fromLTRB(16, 12, 16, 6),
                padding: const EdgeInsets.all(14),
                decoration: BoxDecoration(
                  gradient: LinearGradient(
                    colors: [
                      AppColors.primaryDark.withAlpha(80),
                      AppColors.darkSurface,
                    ],
                    begin: Alignment.topLeft,
                    end: Alignment.bottomRight,
                  ),
                  borderRadius: BorderRadius.circular(14),
                  border: Border.all(color: AppColors.primary.withAlpha(80)),
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    // Active company identity header
                    Row(
                      children: [
                        Container(
                          padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                          decoration: BoxDecoration(
                            color: AppColors.primary,
                            borderRadius: BorderRadius.circular(4),
                          ),
                          child: Text(
                            CompanyService.instance.activeCompany.shortCode,
                            style: const TextStyle(fontSize: 10, fontWeight: FontWeight.bold, color: Colors.white),
                          ),
                        ),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Text(
                            CompanyService.instance.activeCompany.name,
                            style: const TextStyle(fontSize: 12.5, fontWeight: FontWeight.bold, color: Colors.white),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                        if (CompanyService.instance.activeCompany.iecCode != null &&
                            CompanyService.instance.activeCompany.iecCode!.isNotEmpty)
                          Text(
                            'IEC: ${CompanyService.instance.activeCompany.iecCode}',
                            style: const TextStyle(fontSize: 11, color: Color(0xFF94A3B8)),
                          ),
                      ],
                    ),
                    const SizedBox(height: 10),
                    const Divider(height: 1, color: Color(0xFF334155)),
                    const SizedBox(height: 10),
                    const Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        Text(
                          'TOTAL LANDED COST (EXCL. IGST)',
                          style: TextStyle(
                            fontSize: 11,
                            fontWeight: FontWeight.bold,
                            color: Color(0xFF94A3B8),
                            letterSpacing: 0.5,
                          ),
                        ),
                        Icon(Icons.analytics_outlined, size: 16, color: AppColors.accent),
                      ],
                    ),
                    const SizedBox(height: 6),
                    Text(
                      currencyFormatter.format(_summary!.totalLandedCostExIgst),
                      style: const TextStyle(
                        fontSize: 22,
                        fontWeight: FontWeight.w800,
                        color: Colors.white,
                        letterSpacing: -0.5,
                      ),
                    ),
                    const SizedBox(height: 12),
                    const Divider(height: 1, color: Color(0xFF334155)),
                    const SizedBox(height: 10),
                    Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        _buildSubMetric('Assessable Base', currencyFormatter.format(_summary!.totalAssessableValue)),
                        _buildSubMetric('Customs Duty', currencyFormatter.format(_summary!.totalBasicDuties), color: AppColors.bcdColor),
                        _buildSubMetric('Expenses', currencyFormatter.format(_summary!.totalBasicExpenses)),
                        _buildSubMetric('IGST', currencyFormatter.format(_summary!.totalIgst), color: AppColors.igstColor),
                      ],
                    ),
                  ],
                ),
              ),
            ],

            // Search Bar
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
              child: TextField(
                controller: _searchController,
                onChanged: _onSearchChanged,
                decoration: InputDecoration(
                  hintText: 'Filter by Part No, Invoice No, Description...',
                  prefixIcon: const Icon(Icons.search, size: 20, color: Color(0xFF94A3B8)),
                  suffixIcon: _search.isNotEmpty
                      ? IconButton(
                          icon: const Icon(Icons.clear, size: 18),
                          onPressed: () {
                            _debounceTimer?.cancel();
                            _searchController.clear();
                            setState(() => _search = '');
                            _loadReport();
                          },
                        )
                      : null,
                ),
              ),
            ),
            const SizedBox(height: 4),

            // LDC Item Cards
            Expanded(
              child: _loading
                  ? const Center(child: CircularProgressIndicator())
                  : _rows.isEmpty
                      ? Center(
                          child: Column(
                            mainAxisAlignment: MainAxisAlignment.center,
                            children: [
                              Icon(Icons.assessment_outlined, size: 48, color: Colors.grey.withAlpha(120)),
                              const SizedBox(height: 12),
                              const Text(
                                'No Landed Duty Cost records found',
                                style: TextStyle(color: Color(0xFF94A3B8), fontSize: 14),
                              ),
                            ],
                          ),
                        )
                      : ListView.builder(
                          padding: const EdgeInsets.only(bottom: 24),
                          itemCount: _rows.length,
                          itemBuilder: (context, index) {
                            return LdcRowCard(row: _rows[index]);
                          },
                        ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildSubMetric(String label, String value, {Color? color}) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(label, style: const TextStyle(fontSize: 10, color: Color(0xFF94A3B8))),
        const SizedBox(height: 2),
        Text(
          value,
          style: TextStyle(
            fontSize: 12,
            fontWeight: FontWeight.bold,
            color: color ?? Colors.white,
          ),
        ),
      ],
    );
  }
}
