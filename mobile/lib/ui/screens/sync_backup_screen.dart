import 'package:flutter/material.dart';
import '../../constants/app_colors.dart';
import '../../models/sync_status.dart';
import '../../services/sync_service.dart';

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
  final TextEditingController _tokenCtrl = TextEditingController();
  bool _obscurePassphrase = true;

  @override
  void initState() {
    super.initState();
    _loadState();
  }

  @override
  void dispose() {
    _passphraseCtrl.dispose();
    _tokenCtrl.dispose();
    super.dispose();
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

  Future<void> _handleSyncFromDrive() async {
    if (!SyncService.instance.gdrive.isAuthenticated) {
      _showDriveConfigDialog();
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
          const SnackBar(
            content: Text('✓ Live data synced from Google Drive!'),
            backgroundColor: AppColors.delivered,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Drive sync error: ${e.toString().replaceAll('Exception: ', '')}'),
            backgroundColor: AppColors.delayed,
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _syncing = false);
    }
  }

  void _showDriveConfigDialog() {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Google Drive Connection'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            ElevatedButton.icon(
              icon: const Icon(Icons.sync_outlined, size: 18),
              label: const Text('Auto-Connect Desktop Account'),
              style: ElevatedButton.styleFrom(
                backgroundColor: AppColors.primary,
                foregroundColor: Colors.white,
                padding: const EdgeInsets.symmetric(vertical: 12),
              ),
              onPressed: () async {
                try {
                  await SyncService.instance.gdrive.refreshAccessToken();
                  await _loadState();
                  if (ctx.mounted) Navigator.pop(ctx);
                  if (mounted) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      const SnackBar(
                        content: Text('✓ Connected to Google Drive with auto-refresh!'),
                        backgroundColor: AppColors.delivered,
                      ),
                    );
                  }
                } catch (e) {
                  if (mounted) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(content: Text('Connection failed: $e'), backgroundColor: AppColors.delayed),
                    );
                  }
                }
              },
            ),
            const SizedBox(height: 12),
            const Row(
              children: [
                Expanded(child: Divider(color: AppColors.darkBorder)),
                Padding(
                  padding: EdgeInsets.symmetric(horizontal: 8),
                  child: Text('OR PASTE TOKEN', style: TextStyle(fontSize: 10, color: Color(0xFF64748B))),
                ),
                Expanded(child: Divider(color: AppColors.darkBorder)),
              ],
            ),
            const SizedBox(height: 10),
            TextField(
              controller: _tokenCtrl,
              decoration: const InputDecoration(
                hintText: 'ya29.a0...',
                labelText: 'Drive Access Token',
              ),
            ),
            const SizedBox(height: 10),
            const Text(
              'Tip: You can also tap "Pick Backup File" to choose .imbk2 directly from your phone or Google Drive app with zero token required!',
              style: TextStyle(fontSize: 11, color: Colors.blueAccent),
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('Cancel'),
          ),
          ElevatedButton(
            onPressed: () async {
              if (_tokenCtrl.text.trim().isNotEmpty) {
                await SyncService.instance.gdrive.setAccessToken(_tokenCtrl.text.trim());
                await _loadState();
              }
              if (ctx.mounted) Navigator.pop(ctx);
            },
            child: const Text('Save Token'),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Google Drive Sync & Backup'),
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : SingleChildScrollView(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
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
                                  child: const Icon(Icons.cloud_sync_outlined, color: AppColors.primaryLight, size: 22),
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
                                color: _status.isConnectedToDrive
                                    ? AppColors.delivered.withAlpha(30)
                                    : Colors.orange.withAlpha(30),
                                borderRadius: BorderRadius.circular(6),
                              ),
                              child: Text(
                                _status.isConnectedToDrive ? 'CONNECTED' : 'STANDALONE',
                                style: TextStyle(
                                  fontSize: 10.5,
                                  fontWeight: FontWeight.bold,
                                  color: _status.isConnectedToDrive ? AppColors.delivered : Colors.orangeAccent,
                                ),
                              ),
                            ),
                          ],
                        ),
                        const SizedBox(height: 14),
                        const Divider(height: 1, color: Color(0xFF334155)),
                        const SizedBox(height: 12),

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

                  // Option 1: File Picker (Select from GDrive / Downloads / WhatsApp)
                  SizedBox(
                    width: double.infinity,
                    height: 50,
                    child: ElevatedButton.icon(
                      onPressed: _syncing ? null : _handleRestoreFromFile,
                      icon: _syncing
                          ? const SizedBox(width: 20, height: 20, child: CircularProgressIndicator(strokeWidth: 2))
                          : const Icon(Icons.file_open_outlined, size: 20),
                      label: Text(
                        _syncing ? 'Decrypting & Updating...' : 'Pick Backup File (.imbk2 or .db)',
                        style: const TextStyle(fontWeight: FontWeight.bold),
                      ),
                      style: ElevatedButton.styleFrom(
                        backgroundColor: AppColors.primary,
                        foregroundColor: Colors.white,
                        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
                      ),
                    ),
                  ),
                  const SizedBox(height: 10),

                  // Option 2: Direct Google Drive Cloud Sync
                  SizedBox(
                    width: double.infinity,
                    height: 50,
                    child: OutlinedButton.icon(
                      onPressed: _syncing ? null : _handleSyncFromDrive,
                      icon: const Icon(Icons.add_to_drive, size: 20, color: AppColors.accent),
                      label: Text(
                        _status.isConnectedToDrive
                            ? 'Sync Latest from Google Drive'
                            : 'Configure Google Drive Token',
                        style: const TextStyle(fontWeight: FontWeight.bold),
                      ),
                      style: OutlinedButton.styleFrom(
                        side: const BorderSide(color: AppColors.primaryLight, width: 1.2),
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
                        _buildStep('1', 'On Desktop, create a backup or configure automatic Google Drive sync to "ImportManagerBackups".'),
                        _buildStep('2', 'On your OnePlus 11, tap "Pick Backup File" or "Sync from Google Drive" to download the latest snapshot.'),
                        _buildStep('3', 'The app decrypts the AES-256 IMBK2 file and updates your offline database instantly.'),
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
