import 'dart:async';
import 'package:flutter/material.dart';
import '../../constants/app_colors.dart';
import '../../database/db_helper.dart';
import '../../models/shipment.dart';
import '../widgets/shipment_detail_sheet.dart';

class EtdEtaTrackerScreen extends StatefulWidget {
  const EtdEtaTrackerScreen({super.key});

  @override
  State<EtdEtaTrackerScreen> createState() => _EtdEtaTrackerScreenState();
}

class _EtdEtaTrackerScreenState extends State<EtdEtaTrackerScreen> {
  List<Shipment> _shipments = [];
  bool _loading = true;
  String _rangeFilter = 'all'; // all, week, 15days, delayed
  String _search = '';
  late final TextEditingController _searchController;
  Timer? _debounceTimer;

  @override
  void initState() {
    super.initState();
    _searchController = TextEditingController();
    _loadData();
  }

  @override
  void dispose() {
    _debounceTimer?.cancel();
    _searchController.dispose();
    super.dispose();
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
      _loadData();
    });
  }

  Future<void> _loadData() async {
    setState(() => _loading = true);
    try {
      final list = await DBHelper.instance.getShipmentsWithEtdEta(
        rangeFilter: _rangeFilter,
        search: _search,
      );
      if (mounted) {
        setState(() {
          _shipments = list;
        });
      }
    } catch (e, st) {
      debugPrint('Error loading ETD/ETA: $e\n$st');
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Shipments ETD - ETA'),
      ),
      body: RefreshIndicator(
        onRefresh: _loadData,
        child: Column(
          children: [
            // Search Input
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 12, 16, 6),
              child: TextField(
                controller: _searchController,
                onChanged: _onSearchChanged,
                decoration: InputDecoration(
                  hintText: 'Search by Invoice, Vessel, Supplier...',
                  prefixIcon: const Icon(Icons.search, size: 20, color: Color(0xFF94A3B8)),
                  suffixIcon: _search.isNotEmpty
                      ? IconButton(
                          icon: const Icon(Icons.clear, size: 18),
                          onPressed: () {
                            _debounceTimer?.cancel();
                            _searchController.clear();
                            setState(() => _search = '');
                            _loadData();
                          },
                        )
                      : null,
                ),
              ),
            ),

            // Time Range Filter Segment
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
              child: Row(
                children: [
                  _buildSegmentButton('All ETD/ETA', 'all'),
                  const SizedBox(width: 6),
                  _buildSegmentButton('Next 7 Days', 'week'),
                  const SizedBox(width: 6),
                  _buildSegmentButton('Next 15 Days', '15days'),
                  const SizedBox(width: 6),
                  _buildSegmentButton('Delayed', 'delayed', isAlert: true),
                ],
              ),
            ),
            const SizedBox(height: 6),

            // List of ETD-ETA Timelines
            Expanded(
              child: _loading
                  ? const Center(child: CircularProgressIndicator())
                  : _shipments.isEmpty
                      ? Center(
                          child: Column(
                            mainAxisAlignment: MainAxisAlignment.center,
                            children: [
                              Icon(Icons.schedule, size: 48, color: Colors.grey.withAlpha(120)),
                              const SizedBox(height: 12),
                              const Text(
                                'No matching ETD/ETA shipments found',
                                style: TextStyle(color: Color(0xFF94A3B8), fontSize: 14),
                              ),
                            ],
                          ),
                        )
                      : ListView.builder(
                          padding: const EdgeInsets.only(bottom: 24),
                          itemCount: _shipments.length,
                          itemBuilder: (context, index) {
                            final item = _shipments[index];
                            return _buildTimelineCard(item);
                          },
                        ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildSegmentButton(String label, String value, {bool isAlert = false}) {
    final isSelected = _rangeFilter == value;
    final activeColor = isAlert ? AppColors.delayed : AppColors.primary;

    return Expanded(
      child: GestureDetector(
        onTap: () {
          setState(() => _rangeFilter = value);
          _loadData();
        },
        child: Container(
          padding: const EdgeInsets.symmetric(vertical: 9),
          alignment: Alignment.center,
          decoration: BoxDecoration(
            color: isSelected ? activeColor : Theme.of(context).cardColor,
            borderRadius: BorderRadius.circular(8),
            border: Border.all(
              color: isSelected ? activeColor : AppColors.darkBorder,
            ),
          ),
          child: Text(
            label,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(
              fontSize: 11,
              fontWeight: isSelected ? FontWeight.bold : FontWeight.w500,
              color: isSelected ? Colors.white : const Color(0xFF94A3B8),
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildTimelineCard(Shipment s) {
    final days = s.daysUntilEta;
    final isAir = s.shipmentMode.toUpperCase().contains('AIR');

    // Calculate simulated transit percentage
    double transitProgress = 0.5;
    final etdDt = s.etdDate;
    final etaDt = s.etaDate;
    if (etdDt != null && etaDt != null && etaDt.isAfter(etdDt)) {
      final totalDuration = etaDt.difference(etdDt).inHours;
      final elapsed = DateTime.now().difference(etdDt).inHours;
      if (totalDuration > 0) {
        transitProgress = (elapsed / totalDuration).clamp(0.05, 0.95);
      }
    }

    return Card(
      child: InkWell(
        onTap: () => ShipmentDetailSheet.show(context, s),
        borderRadius: BorderRadius.circular(14),
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Header: Invoice No + Supplier
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Expanded(
                    child: Text(
                      s.invoiceNumber,
                      style: const TextStyle(fontSize: 16, fontWeight: FontWeight.bold),
                    ),
                  ),
                  if (days != null) ...[
                    Container(
                      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                      decoration: BoxDecoration(
                        color: s.isDelayed
                            ? AppColors.delayed.withAlpha(30)
                            : (days == 0 ? AppColors.readyDelivery.withAlpha(40) : AppColors.delivered.withAlpha(30)),
                        borderRadius: BorderRadius.circular(6),
                        border: Border.all(
                          color: s.isDelayed
                              ? AppColors.delayed
                              : (days == 0 ? AppColors.readyDelivery : AppColors.delivered),
                        ),
                      ),
                      child: Text(
                        s.isDelayed
                            ? 'Delayed ${days.abs()}d'
                            : (days == 0 ? 'Arriving Today' : '$days Days Left'),
                        style: TextStyle(
                          fontSize: 11,
                          fontWeight: FontWeight.bold,
                          color: s.isDelayed
                              ? AppColors.delayed
                              : (days == 0 ? AppColors.readyDelivery : AppColors.delivered),
                        ),
                      ),
                    ),
                  ],
                ],
              ),
              const SizedBox(height: 3),
              Text(
                s.supplierName,
                style: const TextStyle(fontSize: 13, color: Color(0xFF94A3B8)),
              ),
              const SizedBox(height: 16),

              // Visual Transit Progress Bar with Vehicle Icon
              Stack(
                alignment: Alignment.centerLeft,
                children: [
                  // Base track
                  Container(
                    height: 6,
                    decoration: BoxDecoration(
                      color: const Color(0xFF334155),
                      borderRadius: BorderRadius.circular(3),
                    ),
                  ),
                  // Progress filled track
                  FractionallySizedBox(
                    widthFactor: transitProgress,
                    child: Container(
                      height: 6,
                      decoration: BoxDecoration(
                        color: s.isDelayed ? AppColors.delayed : AppColors.accent,
                        borderRadius: BorderRadius.circular(3),
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 12),

              // Departure & Arrival Indicators
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const Row(
                        children: [
                          Icon(Icons.trip_origin, size: 12, color: AppColors.accent),
                          SizedBox(width: 4),
                          Text('ETD (Departure)', style: TextStyle(fontSize: 11, color: Color(0xFF94A3B8))),
                        ],
                      ),
                      const SizedBox(height: 3),
                      Text(
                        s.etd.isNotEmpty ? s.etd : 'Pending',
                        style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 13.5),
                      ),
                    ],
                  ),
                  Icon(
                    isAir ? Icons.airplanemode_active : Icons.directions_boat,
                    size: 20,
                    color: s.isDelayed ? AppColors.delayed : AppColors.accent,
                  ),
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.end,
                    children: [
                      const Row(
                        children: [
                          Text('ETA (Arrival)', style: TextStyle(fontSize: 11, color: Color(0xFF94A3B8))),
                          SizedBox(width: 4),
                          Icon(Icons.location_on, size: 12, color: AppColors.delivered),
                        ],
                      ),
                      const SizedBox(height: 3),
                      Text(
                        s.eta.isNotEmpty ? s.eta : 'Pending',
                        style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 13.5),
                      ),
                    ],
                  ),
                ],
              ),

              const SizedBox(height: 10),
              const Divider(height: 1, color: Color(0xFF334155)),
              const SizedBox(height: 10),

              // Vessel & Container details
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Expanded(
                    child: Text(
                      s.vesselName.isNotEmpty ? s.vesselName : 'Direct Shipment',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(fontSize: 12, color: Color(0xFF94A3B8)),
                    ),
                  ),
                  if (s.containerNumber.isNotEmpty)
                    Text(
                      'Container: ${s.containerNumber}',
                      style: const TextStyle(fontSize: 12, fontWeight: FontWeight.w600, fontFamily: 'monospace'),
                    ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}
