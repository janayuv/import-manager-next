import 'dart:async';
import 'package:flutter/material.dart';
import '../../constants/app_colors.dart';
import '../../database/db_helper.dart';
import '../../models/shipment.dart';
import '../widgets/shipment_card.dart';
import '../widgets/shipment_detail_sheet.dart';

class UpcomingShipmentsScreen extends StatefulWidget {
  const UpcomingShipmentsScreen({super.key});

  @override
  State<UpcomingShipmentsScreen> createState() => _UpcomingShipmentsScreenState();
}

class _UpcomingShipmentsScreenState extends State<UpcomingShipmentsScreen> {
  List<Shipment> _shipments = [];
  bool _loading = true;
  String _searchQuery = '';
  String _selectedStatus = 'all';
  late final TextEditingController _searchController;
  Timer? _debounceTimer;

  final List<Map<String, String>> _statusFilters = [
    {'label': 'All Active', 'value': 'all'},
    {'label': 'In-Transit', 'value': 'in-transit'},
    {'label': 'Customs', 'value': 'customs-clearance'},
    {'label': 'Ready Dly', 'value': 'ready-dly'},
    {'label': 'Docs Rcvd', 'value': 'docs-rcvd'},
  ];

  @override
  void initState() {
    super.initState();
    _searchController = TextEditingController();
    _loadShipments();
  }

  @override
  void dispose() {
    _debounceTimer?.cancel();
    _searchController.dispose();
    super.dispose();
  }

  void _onSearchChanged(String val) {
    if ((_searchQuery.isEmpty && val.isNotEmpty) || (_searchQuery.isNotEmpty && val.isEmpty)) {
      setState(() => _searchQuery = val);
    } else {
      _searchQuery = val;
    }
    _debounceTimer?.cancel();
    _debounceTimer = Timer(const Duration(milliseconds: 250), () {
      if (!mounted) return;
      _loadShipments();
    });
  }

  Future<void> _loadShipments() async {
    setState(() => _loading = true);
    try {
      final list = await DBHelper.instance.getUpcomingShipments(
        search: _searchQuery,
        statusFilter: _selectedStatus,
      );
      if (mounted) {
        setState(() {
          _shipments = list;
        });
      }
    } catch (e, st) {
      debugPrint('Error loading upcoming shipments: $e\n$st');
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final inTransitCount = _shipments.where((s) => s.status.toLowerCase() == 'in-transit').length;
    final customsCount = _shipments.where((s) => s.status.toLowerCase() == 'customs-clearance').length;

    return Scaffold(
      appBar: AppBar(
        title: const Text('Upcoming Shipments'),
        actions: [
          Container(
            margin: const EdgeInsets.only(right: 16),
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
            decoration: BoxDecoration(
              color: Colors.blue.withAlpha(30),
              borderRadius: BorderRadius.circular(6),
              border: Border.all(color: Colors.blue.withAlpha(80)),
            ),
            child: const Text(
              'VIEW ONLY',
              style: TextStyle(
                fontSize: 10.5,
                fontWeight: FontWeight.bold,
                color: Colors.blueAccent,
                letterSpacing: 0.5,
              ),
            ),
          ),
        ],
      ),
      body: RefreshIndicator(
        onRefresh: _loadShipments,
        child: Column(
          children: [
            // KPI Summary Counters
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 12, 16, 6),
              child: Row(
                children: [
                  _buildCounterCard('Total Active', '${_shipments.length}', AppColors.accent),
                  const SizedBox(width: 8),
                  _buildCounterCard('In Transit', '$inTransitCount', AppColors.inTransit),
                  const SizedBox(width: 8),
                  _buildCounterCard('Customs', '$customsCount', AppColors.customsClearance),
                ],
              ),
            ),

            // Search Bar
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
              child: TextField(
                controller: _searchController,
                onChanged: _onSearchChanged,
                decoration: InputDecoration(
                  hintText: 'Search by Invoice, Supplier, Vessel...',
                  prefixIcon: const Icon(Icons.search, size: 20, color: Color(0xFF94A3B8)),
                  suffixIcon: _searchQuery.isNotEmpty
                      ? IconButton(
                          icon: const Icon(Icons.clear, size: 18),
                          onPressed: () {
                            _debounceTimer?.cancel();
                            _searchController.clear();
                            setState(() => _searchQuery = '');
                            _loadShipments();
                          },
                        )
                      : null,
                ),
              ),
            ),

            // Filter Chips
            SizedBox(
              height: 38,
              child: ListView.builder(
                scrollDirection: Axis.horizontal,
                padding: const EdgeInsets.symmetric(horizontal: 16),
                itemCount: _statusFilters.length,
                itemBuilder: (context, index) {
                  final filter = _statusFilters[index];
                  final isSelected = _selectedStatus == filter['value'];
                  return Padding(
                    padding: const EdgeInsets.only(right: 8),
                    child: ChoiceChip(
                      label: Text(
                        filter['label']!,
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
                          setState(() => _selectedStatus = filter['value']!);
                          _loadShipments();
                        }
                      },
                    ),
                  );
                },
              ),
            ),
            const SizedBox(height: 8),

            // Shipments List
            Expanded(
              child: _loading
                  ? const Center(child: CircularProgressIndicator())
                  : _shipments.isEmpty
                      ? Center(
                          child: Column(
                            mainAxisAlignment: MainAxisAlignment.center,
                            children: [
                              Icon(Icons.inventory_2_outlined, size: 48, color: Colors.grey.withAlpha(120)),
                              const SizedBox(height: 12),
                              const Text(
                                'No upcoming shipments found',
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
                            return ShipmentCard(
                              shipment: item,
                              onTap: () => ShipmentDetailSheet.show(context, item),
                            );
                          },
                        ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildCounterCard(String title, String count, Color color) {
    return Expanded(
      child: Container(
        padding: const EdgeInsets.symmetric(vertical: 10, horizontal: 12),
        decoration: BoxDecoration(
          color: Theme.of(context).cardColor,
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: AppColors.darkBorder),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              title,
              style: const TextStyle(fontSize: 11, color: Color(0xFF94A3B8)),
            ),
            const SizedBox(height: 3),
            Text(
              count,
              style: TextStyle(
                fontSize: 18,
                fontWeight: FontWeight.bold,
                color: color,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
