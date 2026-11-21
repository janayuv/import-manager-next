import 'dart:async';
import 'package:flutter/material.dart';
import '../../constants/app_colors.dart';
import '../../database/db_helper.dart';
import '../../models/item_part.dart';
import '../../services/company_service.dart';
import '../widgets/duty_breakdown_dialog.dart';
import '../widgets/part_duty_card.dart';
import '../widgets/company_switcher_badge.dart';

class PartsDutyScreen extends StatefulWidget {
  const PartsDutyScreen({super.key});

  @override
  State<PartsDutyScreen> createState() => _PartsDutyScreenState();
}

class _PartsDutyScreenState extends State<PartsDutyScreen> {
  List<ItemPart> _items = [];
  List<String> _categories = ['All'];
  String _selectedCategory = 'All';
  String _searchQuery = '';
  bool _loading = true;
  late final TextEditingController _searchController;
  Timer? _debounceTimer;

  @override
  void initState() {
    super.initState();
    _searchController = TextEditingController();
    CompanyService.instance.addListener(_onCompanyChanged);
    _initData();
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
      _initData();
    }
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
      _loadParts();
    });
  }

  Future<void> _initData() async {
    try {
      final cats = await DBHelper.instance.getPartCategories();
      if (mounted) {
        setState(() {
          _categories = ['All', ...cats];
        });
      }
    } catch (e) {
      debugPrint('Error loading categories: $e');
    }
    await _loadParts();
  }

  Future<void> _loadParts() async {
    setState(() => _loading = true);
    try {
      final list = await DBHelper.instance.getItemsParts(
        search: _searchQuery,
        category: _selectedCategory,
      );
      if (mounted) {
        setState(() {
          _items = list;
        });
      }
    } catch (e, st) {
      debugPrint('Error loading parts: $e\n$st');
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Parts & Duty Structure'),
        actions: const [
          CompanySwitcherBadge(),
          SizedBox(width: 14),
        ],
      ),
      body: RefreshIndicator(
        onRefresh: _loadParts,
        child: Column(
          children: [
            // Search Bar for HS Code & Part No
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 12, 16, 6),
              child: TextField(
                controller: _searchController,
                onChanged: _onSearchChanged,
                decoration: InputDecoration(
                  hintText: 'Search by Part No, HSN Code, Description...',
                  prefixIcon: const Icon(Icons.search, size: 20, color: Color(0xFF94A3B8)),
                  suffixIcon: _searchQuery.isNotEmpty
                      ? IconButton(
                          icon: const Icon(Icons.clear, size: 18),
                          onPressed: () {
                            _debounceTimer?.cancel();
                            _searchController.clear();
                            setState(() => _searchQuery = '');
                            _loadParts();
                          },
                        )
                      : null,
                ),
              ),
            ),

            // Category Filter Pills
            if (_categories.length > 1) ...[
              SizedBox(
                height: 38,
                child: ListView.builder(
                  scrollDirection: Axis.horizontal,
                  padding: const EdgeInsets.symmetric(horizontal: 16),
                  itemCount: _categories.length,
                  itemBuilder: (context, index) {
                    final cat = _categories[index];
                    final isSelected = _selectedCategory == cat;
                    return Padding(
                      padding: const EdgeInsets.only(right: 8),
                      child: ChoiceChip(
                        label: Text(
                          cat,
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
                            setState(() => _selectedCategory = cat);
                            _loadParts();
                          }
                        },
                      ),
                    );
                  },
                ),
              ),
              const SizedBox(height: 6),
            ],

            // Parts List
            Expanded(
              child: _loading
                  ? const Center(child: CircularProgressIndicator())
                  : _items.isEmpty
                      ? Center(
                          child: Column(
                            mainAxisAlignment: MainAxisAlignment.center,
                            children: [
                              Icon(Icons.category_outlined, size: 48, color: Colors.grey.withAlpha(120)),
                              const SizedBox(height: 12),
                              const Text(
                                'No matching parts or HS codes found',
                                style: TextStyle(color: Color(0xFF94A3B8), fontSize: 14),
                              ),
                            ],
                          ),
                        )
                      : ListView.builder(
                          padding: const EdgeInsets.only(bottom: 24),
                          itemCount: _items.length,
                          itemBuilder: (context, index) {
                            final item = _items[index];
                            return PartDutyCard(
                              item: item,
                              onTap: () => DutyBreakdownDialog.show(context, item),
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
