import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import '../../constants/app_colors.dart';
import '../../database/db_helper.dart';
import '../../models/landed_cost.dart';
import '../widgets/ldc_row_card.dart';

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

  @override
  void initState() {
    super.initState();
    _loadReport();
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
        actions: [
          Container(
            margin: const EdgeInsets.only(right: 16),
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
            decoration: BoxDecoration(
              color: AppColors.ldcBadge.withAlpha(25),
              borderRadius: BorderRadius.circular(6),
              border: Border.all(color: AppColors.ldcBadge.withAlpha(80)),
            ),
            child: const Text(
              'LDC REPORT',
              style: TextStyle(
                fontSize: 10.5,
                fontWeight: FontWeight.bold,
                color: AppColors.ldcBadge,
              ),
            ),
          ),
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
                onChanged: (val) {
                  setState(() => _search = val);
                  _loadReport();
                },
                decoration: InputDecoration(
                  hintText: 'Filter by Part No, Invoice No, Description...',
                  prefixIcon: const Icon(Icons.search, size: 20, color: Color(0xFF94A3B8)),
                  suffixIcon: _search.isNotEmpty
                      ? IconButton(
                          icon: const Icon(Icons.clear, size: 18),
                          onPressed: () {
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
