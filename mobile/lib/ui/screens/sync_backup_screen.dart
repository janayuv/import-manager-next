import 'package:flutter/material.dart';
import '../../constants/app_colors.dart';
import '../../models/sync_status.dart';
import '../../services/sync_service.dart';
import '../../services/company_service.dart';
import '../widgets/company_switcher_badge.dart';

class SyncBackupScreen extends StatefulWidget {
  const SyncBackupScreen({super.key});

  @override
  State<SyncBackupScreen> createState() => _SyncBackupScreenState();
}

class _SyncBackupScreenState extends State<SyncBackupScreen> {
  SyncStatus _status = SyncStatus();
  bool _loading = true;
  bool _syncing = false;

  final TextEditingController _passphraseCtrl = TextEditingController();
  bool _obscurePassphrase = true;

  @override
  void initState() {
    super.initState();
    CompanyService.instance.addListener(_onCompanyChanged);
    _loadState();
  }

  @override
  void dispose() {
    CompanyService.instance.removeListener(_onCompanyChanged);
    _passphraseCtrl.dispose();
    super.dispose();
  }

  void _onCompanyChanged() {
    if (mounted) {
      _loadState();
    }
  }

  Future<void> _loadState() async {
    setState(() => _loading = true);
    try {
      await SyncService.instance.init();
      final status = await SyncService.instance.getStatus();
      final savedPass = await SyncService.instance.getSavedPassphrase();
      if (savedPass != null && savedPass.isNotEmpty) {
        _passphraseCtrl.text = savedPass;
      } else {
        _passphraseCtrl.text = 'Jana@1438';
      }
      if (mounted) {
        setState(() {
          _status = status;
        });
      }
    } catch (e, st) {
      debugPrint('Error loading sync state: $e\n$st');
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _handleRestoreFromFile() async {
    setState(() => _syncing = true);
    try {
      final updatedStatus = await SyncService.instance.restoreFromFilePicker(
        passphrase: _passphraseCtrl.text.trim(),
      );
      if (mounted) {
        setState(() => _status = updatedStatus);
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('✓ Backup successfully restored and database updated!'),
            backgroundColor: AppColors.delivered,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Restore error: ${e.toString().replaceAll('Exception: ', '')}'),
            backgroundColor: AppColors.delayed,
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _syncing = false);
    }
  }

  Future<void> _handleDirectSync({String? serverUrl, String? email, String? password}) async {
    setState(() => _syncing = true);
    try {
      final updatedStatus = await SyncService.instance.syncFromMobileServer(
        serverUrl: serverUrl,
        email: email,
        password: password,
        passphrase: _passphraseCtrl.text.trim(),
      );
      if (mounted) {
        setState(() => _status = updatedStatus);
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('✓ Live data synced via Email & Password (${updatedStatus.lastBackupFileName})!'),
            backgroundColor: AppColors.delivered,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        final errorMsg = e.toString().replaceAll('Exception: ', '');
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Direct Sync error: $errorMsg'),
            backgroundColor: AppColors.delayed,
            duration: const Duration(seconds: 6),
            action: SnackBarAction(
              label: 'Settings',
              textColor: Colors.white,
              onPressed: () => _showConnectionSettingsDialog(initialTabIndex: 0),
            ),
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _syncing = false);
    }
  }

  Future<void> _handleSyncFromDrive() async {
    if (!SyncService.instance.gdrive.isAuthenticated) {
      _showConnectionSettingsDialog(initialTabIndex: 1);
      return;
    }

    setState(() => _syncing = true);
    try {
      final updatedStatus = await SyncService.instance.syncLatestFromDrive(
        passphrase: _passphraseCtrl.text.trim(),
      );
      if (mounted) {
        setState(() => _status = updatedStatus);
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('✓ Live data synced from Google Drive (${updatedStatus.lastBackupFileName})!'),
            backgroundColor: AppColors.delivered,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        final errorMsg = e.toString().replaceAll('Exception: ', '');
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Drive sync error: $errorMsg'),
            backgroundColor: AppColors.delayed,
            duration: const Duration(seconds: 7),
            action: SnackBarAction(
              label: 'Settings',
              textColor: Colors.white,
              onPressed: () => _showConnectionSettingsDialog(initialTabIndex: 1),
            ),
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _syncing = false);
    }
  }

  void _showConnectionSettingsDialog({int initialTabIndex = 0}) {
    final gdrive = SyncService.instance.gdrive;
    final mobileSync = SyncService.instance.mobileSync;

    // Direct Sync Controllers
    final serverUrlCtrl = TextEditingController(text: mobileSync.effectiveServerUrl);
    final directEmailCtrl = TextEditingController(text: mobileSync.effectiveEmail);
    final directPassCtrl = TextEditingController(text: _passphraseCtrl.text);
    bool obscureDirectPass = true;

    // Google Drive Controllers
    final rTokenCtrl = TextEditingController(text: gdrive.effectiveRefreshToken);
    final cIdCtrl = TextEditingController(text: gdrive.effectiveClientId);
    final cSecCtrl = TextEditingController(text: gdrive.effectiveClientSecret);
    final gdriveEmailCtrl = TextEditingController(text: gdrive.effectiveAccountEmail);
    final manualTokenCtrl = TextEditingController();
    bool showAdvancedDrive = false;

    showDialog(
      context: context,
      builder: (ctx) => DefaultTabController(
        length: 2,
        initialIndex: initialTabIndex,
        child: StatefulBuilder(
          builder: (ctx, setDlgState) {
            return AlertDialog(
              backgroundColor: AppColors.darkCard,
              shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
              titlePadding: const EdgeInsets.fromLTRB(20, 16, 20, 0),
              contentPadding: const EdgeInsets.fromLTRB(20, 12, 20, 16),
              title: Column(
                children: [
                  const Row(
                    children: [
                      Icon(Icons.sync_alt, color: AppColors.accent, size: 22),
                      SizedBox(width: 8),
                      Text('Sync & Connection Options', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
                    ],
                  ),
                  const SizedBox(height: 12),
                  TabBar(
                    indicatorColor: AppColors.primary,
                    labelColor: Colors.white,
                    unselectedLabelColor: const Color(0xFF94A3B8),
                    labelStyle: const TextStyle(fontSize: 12.5, fontWeight: FontWeight.bold),
                    tabs: const [
                      Tab(icon: Icon(Icons.password, size: 18), text: 'Email & Password'),
                      Tab(icon: Icon(Icons.cloud_outlined, size: 18), text: 'Google Drive'),
                    ],
                  ),
                ],
              ),
              content: SizedBox(
                width: double.maxFinite,
                child: SingleChildScrollView(
                  child: SizedBox(
                    height: 380,
                    child: TabBarView(
                      children: [
                        // Tab 1: Email & Password Connect (Direct Local / LAN Sync)
                        SingleChildScrollView(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.stretch,
                            children: [
                              const SizedBox(height: 10),
                              Container(
                                padding: const EdgeInsets.all(10),
                                decoration: BoxDecoration(
                                  color: AppColors.primary.withAlpha(25),
                                  borderRadius: BorderRadius.circular(8),
                                  border: Border.all(color: AppColors.primary.withAlpha(70)),
                                ),
                                child: const Row(
                                  children: [
                                    Icon(Icons.bolt, color: AppColors.accent, size: 18),
                                    SizedBox(width: 8),
                                    Expanded(
                                      child: Text(
                                        'Sync directly with Desktop Import Manager over Wi-Fi without Google Drive errors.',
                                        style: TextStyle(fontSize: 11.5, color: Color(0xFFCBD5E1)),
                                      ),
                                    ),
                                  ],
                                ),
                              ),
                              const SizedBox(height: 12),
                              TextField(
                                controller: serverUrlCtrl,
                                decoration: const InputDecoration(
                                  labelText: 'Desktop Server Address / IP',
                                  hintText: 'http://192.168.1.5:8766',
                                  prefixIcon: Icon(Icons.lan_outlined, size: 18),
                                  isDense: true,
                                ),
                              ),
                              const SizedBox(height: 8),
                              TextField(
                                controller: directEmailCtrl,
                                decoration: const InputDecoration(
                                  labelText: 'Email or Username',
                                  hintText: 'admin or user@gmail.com',
                                  prefixIcon: Icon(Icons.person_outline, size: 18),
                                  isDense: true,
                                ),
                              ),
                              const SizedBox(height: 8),
                              TextField(
                                controller: directPassCtrl,
                                obscureText: obscureDirectPass,
                                decoration: InputDecoration(
                                  labelText: 'Password or Backup Passphrase',
                                  prefixIcon: const Icon(Icons.lock_outline, size: 18),
                                  isDense: true,
                                  suffixIcon: IconButton(
                                    icon: Icon(
                                      obscureDirectPass ? Icons.visibility_outlined : Icons.visibility_off_outlined,
                                      size: 18,
                                    ),
                                    onPressed: () => setDlgState(() => obscureDirectPass = !obscureDirectPass),
                                  ),
                                ),
                              ),
                              const SizedBox(height: 14),
                              ElevatedButton.icon(
                                icon: const Icon(Icons.login, size: 18),
                                label: const Text('Connect & Sync Live Data'),
                                style: ElevatedButton.styleFrom(
                                  backgroundColor: AppColors.primary,
                                  foregroundColor: Colors.white,
                                  padding: const EdgeInsets.symmetric(vertical: 12),
                                  shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
                                ),
                                onPressed: () async {
                                  Navigator.pop(ctx);
                                  await _handleDirectSync(
                                    serverUrl: serverUrlCtrl.text.trim(),
                                    email: directEmailCtrl.text.trim(),
                                    password: directPassCtrl.text.trim(),
                                  );
                                },
                              ),
                              const SizedBox(height: 8),
                              OutlinedButton.icon(
                                icon: const Icon(Icons.network_check, size: 18),
                                label: const Text('Test Server Connection'),
                                style: OutlinedButton.styleFrom(
                                  foregroundColor: const Color(0xFFCBD5E1),
                                  side: const BorderSide(color: AppColors.darkBorder),
                                  padding: const EdgeInsets.symmetric(vertical: 10),
                                  shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
                                ),
                                onPressed: () async {
                                  try {
                                    final status = await mobileSync.testConnection(serverUrlCtrl.text.trim());
                                    if (ctx.mounted && mounted) {
                                      ScaffoldMessenger.of(context).showSnackBar(
                                        SnackBar(
                                          content: Text('✓ Server Online: ${status['activeCompany'] ?? 'Desktop App'} (v${status['version']})'),
                                          backgroundColor: AppColors.delivered,
                                        ),
                                      );
                                    }
                                  } catch (e) {
                                    if (ctx.mounted && mounted) {
                                      ScaffoldMessenger.of(context).showSnackBar(
                                        SnackBar(
                                          content: Text('Connection failed: ${e.toString().replaceAll('Exception: ', '')}'),
                                          backgroundColor: AppColors.delayed,
                                        ),
                                      );
                                    }
                                  }
                                },
                              ),
                            ],
                          ),
                        ),

                        // Tab 2: Google Drive Cloud Sync
                        SingleChildScrollView(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.stretch,
                            children: [
                              const SizedBox(height: 10),
                              Container(
                                padding: const EdgeInsets.all(10),
                                decoration: BoxDecoration(
                                  color: AppColors.darkSurface,
                                  borderRadius: BorderRadius.circular(8),
                                  border: Border.all(color: AppColors.darkBorder),
                                ),
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Row(
                                      children: [
                                        Icon(
                                          gdrive.isAuthenticated ? Icons.check_circle : Icons.info_outline,
                                          color: gdrive.isAuthenticated ? AppColors.delivered : Colors.orangeAccent,
                                          size: 16,
                                        ),
                                        const SizedBox(width: 6),
                                        Text(
                                          gdrive.isAuthenticated ? 'Google OAuth Configured' : 'Google Cloud Drive',
                                          style: TextStyle(
                                            fontWeight: FontWeight.bold,
                                            fontSize: 12.5,
                                            color: gdrive.isAuthenticated ? AppColors.delivered : Colors.orangeAccent,
                                          ),
                                        ),
                                      ],
                                    ),
                                    const SizedBox(height: 4),
                                    Text(
                                      'Account: ${gdrive.effectiveAccountEmail}',
                                      style: const TextStyle(fontSize: 11.5, color: Color(0xFFCBD5E1)),
                                    ),
                                    const SizedBox(height: 4),
                                    const Text(
                                      'Notice: If Google returns Error 403 (Permission Denied), Google Drive API must be enabled in Google Cloud Console. Use "Email & Password" tab for hassle-free direct sync.',
                                      style: TextStyle(fontSize: 11, color: Color(0xFF94A3B8), height: 1.25),
                                    ),
                                  ],
                                ),
                              ),
                              const SizedBox(height: 12),
                              ElevatedButton.icon(
                                icon: const Icon(Icons.refresh, size: 18),
                                label: const Text('Test & Force Token Refresh'),
                                style: ElevatedButton.styleFrom(
                                  backgroundColor: AppColors.primary,
                                  foregroundColor: Colors.white,
                                  padding: const EdgeInsets.symmetric(vertical: 12),
                                  shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
                                ),
                                onPressed: () async {
                                  try {
                                    await gdrive.refreshAccessToken();
                                    await _loadState();
                                    if (ctx.mounted && mounted) {
                                      Navigator.pop(ctx);
                                      ScaffoldMessenger.of(context).showSnackBar(
                                        const SnackBar(
                                          content: Text('✓ Google Drive token refreshed successfully!'),
                                          backgroundColor: AppColors.delivered,
                                        ),
                                      );
                                    }
                                  } catch (e) {
                                    if (ctx.mounted && mounted) {
                                      ScaffoldMessenger.of(context).showSnackBar(
                                        SnackBar(
                                          content: Text('Refresh error: ${e.toString().replaceAll('Exception: ', '')}'),
                                          backgroundColor: AppColors.delayed,
                                        ),
                                      );
                                    }
                                  }
                                },
                              ),
                              const SizedBox(height: 8),
                              OutlinedButton.icon(
                                icon: const Icon(Icons.restore, size: 18),
                                label: const Text('Clear Saved Credentials'),
                                style: OutlinedButton.styleFrom(
                                  foregroundColor: const Color(0xFFCBD5E1),
                                  side: const BorderSide(color: AppColors.darkBorder),
                                  padding: const EdgeInsets.symmetric(vertical: 10),
                                  shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
                                ),
                                onPressed: () async {
                                  await gdrive.resetToDefaultCredentials();
                                  await _loadState();
                                  if (ctx.mounted && mounted) {
                                    Navigator.pop(ctx);
                                    ScaffoldMessenger.of(context).showSnackBar(
                                      const SnackBar(
                                        content: Text('✓ Cleared saved Google Drive credentials'),
                                        backgroundColor: AppColors.delivered,
                                      ),
                                    );
                                  }
                                },
                              ),
                              const SizedBox(height: 6),
                              TextButton.icon(
                                icon: Icon(showAdvancedDrive ? Icons.expand_less : Icons.expand_more, size: 18),
                                label: Text(
                                  showAdvancedDrive ? 'Hide Advanced Credentials' : 'Show Advanced / Custom OAuth',
                                  style: const TextStyle(fontSize: 12),
                                ),
                                onPressed: () => setDlgState(() => showAdvancedDrive = !showAdvancedDrive),
                              ),
                              if (showAdvancedDrive) ...[
                                const SizedBox(height: 6),
                                TextField(
                                  controller: gdriveEmailCtrl,
                                  decoration: const InputDecoration(labelText: 'Account Email', isDense: true),
                                ),
                                const SizedBox(height: 8),
                                TextField(
                                  controller: rTokenCtrl,
                                  decoration: const InputDecoration(labelText: 'OAuth Refresh Token', isDense: true),
                                ),
                                const SizedBox(height: 8),
                                TextField(
                                  controller: cIdCtrl,
                                  decoration: const InputDecoration(labelText: 'OAuth Client ID', isDense: true),
                                ),
                                const SizedBox(height: 8),
                                TextField(
                                  controller: cSecCtrl,
                                  decoration: const InputDecoration(labelText: 'OAuth Client Secret (Optional for PKCE)', isDense: true),
                                ),
                                const SizedBox(height: 8),
                                TextField(
                                  controller: manualTokenCtrl,
                                  decoration: const InputDecoration(
                                    labelText: 'Manual Access Token (Optional fallback)',
                                    hintText: 'ya29.a0...',
                                    isDense: true,
                                  ),
                                ),
                                const SizedBox(height: 10),
                                ElevatedButton(
                                  style: ElevatedButton.styleFrom(
                                    backgroundColor: AppColors.accent,
                                    foregroundColor: Colors.black,
                                  ),
                                  onPressed: () async {
                                    if (manualTokenCtrl.text.trim().isNotEmpty) {
                                      await gdrive.setAccessToken(manualTokenCtrl.text.trim());
                                    }
                                    if (rTokenCtrl.text.trim().isNotEmpty) {
                                      await gdrive.setRefreshToken(rTokenCtrl.text.trim());
                                    }
                                    await gdrive.setClientCredentials(
                                      clientId: cIdCtrl.text.trim(),
                                      clientSecret: cSecCtrl.text.trim(),
                                      email: gdriveEmailCtrl.text.trim(),
                                    );
                                    await _loadState();
                                    if (ctx.mounted && mounted) {
                                      Navigator.pop(ctx);
                                      ScaffoldMessenger.of(context).showSnackBar(
                                        const SnackBar(
                                          content: Text('✓ Custom OAuth credentials saved'),
                                          backgroundColor: AppColors.delivered,
                                        ),
                                      );
                                    }
                                  },
                                  child: const Text('Save Custom Credentials'),
                                ),
                              ],
                            ],
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
              actions: [
                TextButton(
                  onPressed: () => Navigator.pop(ctx),
                  child: const Text('Close'),
                ),
              ],
            );
          },
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final bool isDirect = _status.syncMode == 'direct';
    final bool isConnected = _status.isConnected;

    return Scaffold(
      appBar: AppBar(
        title: const Text('Sync & Backup'),
        actions: [
          const CompanySwitcherBadge(),
          IconButton(
            icon: const Icon(Icons.settings_outlined),
            tooltip: 'Connection Settings',
            onPressed: () => _showConnectionSettingsDialog(),
          ),
        ],
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : SingleChildScrollView(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // Active Company Target Banner
                  AnimatedBuilder(
                    animation: CompanyService.instance,
                    builder: (context, _) {
                      final comp = CompanyService.instance.activeCompany;
                      return Container(
                        margin: const EdgeInsets.only(bottom: 12),
                        padding: const EdgeInsets.all(12),
                        decoration: BoxDecoration(
                          color: AppColors.primary.withAlpha(25),
                          borderRadius: BorderRadius.circular(12),
                          border: Border.all(color: AppColors.primary.withAlpha(70)),
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
                                comp.shortCode,
                                style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 11, color: Colors.white),
                              ),
                            ),
                            const SizedBox(width: 10),
                            Expanded(
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Text(
                                    comp.name,
                                    style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 13, color: Colors.white),
                                    maxLines: 1,
                                    overflow: TextOverflow.ellipsis,
                                  ),
                                  Text(
                                    'Active Database: ${comp.dbFileName}',
                                    style: const TextStyle(fontSize: 11, color: Color(0xFF94A3B8)),
                                  ),
                                ],
                              ),
                            ),
                          ],
                        ),
                      );
                    },
                  ),

                  // Sync Status Card
                  Container(
                    padding: const EdgeInsets.all(16),
                    decoration: BoxDecoration(
                      color: Theme.of(context).cardColor,
                      borderRadius: BorderRadius.circular(16),
                      border: Border.all(color: AppColors.darkBorder),
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          mainAxisAlignment: MainAxisAlignment.spaceBetween,
                          children: [
                            Row(
                              children: [
                                Container(
                                  padding: const EdgeInsets.all(8),
                                  decoration: BoxDecoration(
                                    color: AppColors.primary.withAlpha(35),
                                    borderRadius: BorderRadius.circular(10),
                                  ),
                                  child: Icon(
                                    isDirect ? Icons.lan_outlined : Icons.cloud_sync_outlined,
                                    color: AppColors.primaryLight,
                                    size: 22,
                                  ),
                                ),
                                const SizedBox(width: 10),
                                const Text(
                                  'Live Data Sync Status',
                                  style: TextStyle(fontSize: 15, fontWeight: FontWeight.bold),
                                ),
                              ],
                            ),
                            Container(
                              padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                              decoration: BoxDecoration(
                                color: isConnected
                                    ? AppColors.delivered.withAlpha(30)
                                    : Colors.orange.withAlpha(30),
                                borderRadius: BorderRadius.circular(6),
                              ),
                              child: Text(
                                isConnected
                                    ? (isDirect ? 'CONNECTED (LAN)' : 'CONNECTED (DRIVE)')
                                    : 'STANDALONE',
                                style: TextStyle(
                                  fontSize: 10.5,
                                  fontWeight: FontWeight.bold,
                                  color: isConnected ? AppColors.delivered : Colors.orangeAccent,
                                ),
                              ),
                            ),
                          ],
                        ),
                        const SizedBox(height: 14),
                        const Divider(height: 1, color: Color(0xFF334155)),
                        const SizedBox(height: 12),

                        _buildInfoRow(
                          'Account',
                          _status.accountEmail.isNotEmpty ? _status.accountEmail : 'Not configured',
                        ),
                        _buildInfoRow(
                          'Sync Mode',
                          isDirect
                              ? 'Email & Password (Direct LAN Sync)'
                              : (_status.isConnectedToDrive ? 'Google Drive Cloud Sync' : 'Standalone / File Pick'),
                        ),
                        if (isDirect && _status.serverUrl.isNotEmpty)
                          _buildInfoRow('Server Address', _status.serverUrl),
                        _buildInfoRow('Last Updated From Desktop', _status.lastSyncTime),
                        _buildInfoRow('Last Backup File', _status.lastBackupFileName),

                        const SizedBox(height: 14),
                        // Database Stats Pills
                        Row(
                          children: [
                            _buildStatChip('Shipments', '${_status.shipmentCount}'),
                            const SizedBox(width: 6),
                            _buildStatChip('Invoices', '${_status.invoiceCount}'),
                            const SizedBox(width: 6),
                            _buildStatChip('Parts', '${_status.itemCount}'),
                          ],
                        ),
                      ],
                    ),
                  ),

                  const SizedBox(height: 20),
                  const Text(
                    'Backup Passphrase (.imbk2 Decryption)',
                    style: TextStyle(fontSize: 14, fontWeight: FontWeight.bold),
                  ),
                  const SizedBox(height: 6),
                  const Text(
                    'Desktop backups are encrypted with AES-256-GCM. Enter your desktop backup passphrase to decrypt automatically on your OnePlus 11.',
                    style: TextStyle(fontSize: 12, color: Color(0xFF94A3B8)),
                  ),
                  const SizedBox(height: 8),

                  TextField(
                    controller: _passphraseCtrl,
                    obscureText: _obscurePassphrase,
                    decoration: InputDecoration(
                      hintText: 'Enter backup passphrase...',
                      prefixIcon: const Icon(Icons.lock_outline, size: 20),
                      suffixIcon: IconButton(
                        icon: Icon(
                          _obscurePassphrase ? Icons.visibility_outlined : Icons.visibility_off_outlined,
                          size: 20,
                        ),
                        onPressed: () => setState(() => _obscurePassphrase = !_obscurePassphrase),
                      ),
                    ),
                    onChanged: (val) => SyncService.instance.savePassphrase(val.trim()),
                  ),

                  const SizedBox(height: 24),
                  const Text(
                    'Restore & Sync Actions',
                    style: TextStyle(fontSize: 14, fontWeight: FontWeight.bold),
                  ),
                  const SizedBox(height: 12),

                  // Option 1: Primary Action (Email & Password Connect / Sync)
                  SizedBox(
                    width: double.infinity,
                    height: 52,
                    child: ElevatedButton.icon(
                      onPressed: _syncing
                          ? null
                          : () {
                              if (isDirect && SyncService.instance.mobileSync.isAuthenticated) {
                                _handleDirectSync();
                              } else {
                                _showConnectionSettingsDialog(initialTabIndex: 0);
                              }
                            },
                      icon: _syncing
                          ? const SizedBox(width: 20, height: 20, child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white))
                          : const Icon(Icons.password_outlined, size: 20),
                      label: Text(
                        _syncing
                            ? 'Syncing Live Data...'
                            : (isDirect && SyncService.instance.mobileSync.isAuthenticated
                                ? 'Sync Live via Email & Password'
                                : 'Connect via Email & Password (Recommended)'),
                        style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 13.5),
                      ),
                      style: ElevatedButton.styleFrom(
                        backgroundColor: AppColors.primary,
                        foregroundColor: Colors.white,
                        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
                      ),
                    ),
                  ),
                  const SizedBox(height: 10),

                  // Option 2: Google Drive Cloud Sync
                  SizedBox(
                    width: double.infinity,
                    height: 50,
                    child: OutlinedButton.icon(
                      onPressed: _syncing ? null : _handleSyncFromDrive,
                      icon: _syncing
                          ? const SizedBox(width: 20, height: 20, child: CircularProgressIndicator(strokeWidth: 2, color: AppColors.accent))
                          : const Icon(Icons.add_to_drive, size: 20, color: AppColors.accent),
                      label: Text(
                        _syncing
                            ? 'Syncing from Google Drive...'
                            : (_status.isConnectedToDrive
                                ? 'Sync Latest from Google Drive'
                                : 'Connect Google Drive'),
                        style: const TextStyle(fontWeight: FontWeight.bold),
                      ),
                      style: OutlinedButton.styleFrom(
                        side: const BorderSide(color: AppColors.primaryLight, width: 1.2),
                        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
                      ),
                    ),
                  ),
                  const SizedBox(height: 10),

                  // Option 3: Pick Backup File
                  SizedBox(
                    width: double.infinity,
                    height: 48,
                    child: OutlinedButton.icon(
                      onPressed: _syncing ? null : _handleRestoreFromFile,
                      icon: const Icon(Icons.file_open_outlined, size: 20, color: Color(0xFF94A3B8)),
                      label: Text(
                        _syncing ? 'Processing File...' : 'Pick Backup File (.imbk2 or .db)',
                        style: const TextStyle(color: Color(0xFFCBD5E1)),
                      ),
                      style: OutlinedButton.styleFrom(
                        side: const BorderSide(color: AppColors.darkBorder),
                        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
                      ),
                    ),
                  ),

                  const SizedBox(height: 28),

                  // How It Works Guide
                  Container(
                    padding: const EdgeInsets.all(14),
                    decoration: BoxDecoration(
                      color: Theme.of(context).cardColor,
                      borderRadius: BorderRadius.circular(14),
                      border: Border.all(color: AppColors.darkBorder),
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        const Row(
                          children: [
                            Icon(Icons.info_outline, size: 16, color: Colors.blueAccent),
                            SizedBox(width: 8),
                            Text(
                              'How Desktop-to-Mobile Live Sync Works',
                              style: TextStyle(fontSize: 13, fontWeight: FontWeight.bold),
                            ),
                          ],
                        ),
                        const SizedBox(height: 8),
                        _buildStep('1', 'On Desktop, the live companion sync server runs automatically on port 8766 (find server IP under Desktop Backups).'),
                        _buildStep('2', 'On your OnePlus 11, tap "Connect via Email & Password", enter your Desktop username/email and password.'),
                        _buildStep('3', 'Your phone connects directly over Wi-Fi, downloads the encrypted backup, and updates SQLite in seconds.'),
                        _buildStep('4', 'All 5 views (Upcoming Shipments, ETD-ETA, Parts, Invoices, LDC) immediately reflect the fresh live data!'),
                      ],
                    ),
                  ),
                ],
              ),
            ),
    );
  }

  Widget _buildInfoRow(String label, String value) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(label, style: const TextStyle(fontSize: 12.5, color: Color(0xFF94A3B8))),
          Flexible(
            child: Text(
              value,
              textAlign: TextAlign.right,
              style: const TextStyle(fontSize: 12.5, fontWeight: FontWeight.bold),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildStatChip(String label, String count) {
    return Expanded(
      child: Container(
        padding: const EdgeInsets.symmetric(vertical: 6),
        decoration: BoxDecoration(
          color: AppColors.darkSurface,
          borderRadius: BorderRadius.circular(8),
          border: Border.all(color: AppColors.darkBorder),
        ),
        alignment: Alignment.center,
        child: Text(
          '$count $label',
          style: const TextStyle(
            fontSize: 11.5,
            fontWeight: FontWeight.w600,
            color: AppColors.accent,
          ),
        ),
      ),
    );
  }

  Widget _buildStep(String number, String text) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 18,
            height: 18,
            alignment: Alignment.center,
            decoration: BoxDecoration(
              color: Colors.blueAccent.withAlpha(40),
              shape: BoxShape.circle,
            ),
            child: Text(
              number,
              style: const TextStyle(fontSize: 10, fontWeight: FontWeight.bold, color: Colors.blueAccent),
            ),
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              text,
              style: const TextStyle(fontSize: 12, color: Color(0xFF94A3B8), height: 1.3),
            ),
          ),
        ],
      ),
    );
  }
}
