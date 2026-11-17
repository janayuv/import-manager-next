import 'dart:io';
import 'dart:typed_data';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/foundation.dart';
import 'package:intl/intl.dart';
import 'package:path_provider/path_provider.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../crypto/imbk_decryptor.dart';
import '../database/db_helper.dart';
import '../models/sync_status.dart';
import 'gdrive_service.dart';

class SyncService {
  static final SyncService instance = SyncService._init();
  final GDriveService gdrive = GDriveService();

  SyncService._init();

  static const String _prefLastSyncTime = 'last_sync_time';
  static const String _prefLastFileName = 'last_sync_file';
  static const String _prefSavedPassphrase = 'saved_backup_passphrase';

  Future<void> init() async {
    await gdrive.init();
  }

  Future<String?> getSavedPassphrase() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getString(_prefSavedPassphrase);
  }

  Future<void> savePassphrase(String passphrase) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_prefSavedPassphrase, passphrase);
  }

  Future<SyncStatus> getStatus() async {
    final prefs = await SharedPreferences.getInstance();
    final counts = await DBHelper.instance.getRecordCounts();
    final lastTime = prefs.getString(_prefLastSyncTime) ?? 'Never';
    final lastFile = prefs.getString(_prefLastFileName) ?? 'None';

    return SyncStatus(
      isConnectedToDrive: gdrive.isAuthenticated,
      lastSyncTime: lastTime,
      lastBackupFileName: lastFile,
      shipmentCount: counts['shipments'] ?? 0,
      invoiceCount: counts['invoices'] ?? 0,
      itemCount: counts['items'] ?? 0,
      ldcRowCount: counts['shipments'] ?? 0,
    );
  }

  /// Restores directly from a selected local file or Google Drive shared file
  Future<SyncStatus> restoreFromFilePicker({String? passphrase}) async {
    final picked = await FilePicker.pickFile(
      type: FileType.any,
    );

    if (picked == null) {
      throw Exception('No file selected');
    }

    final fileBytes = await picked.readAsBytes();
    return await restoreFromBytes(fileBytes, picked.name, passphrase: passphrase);
  }

  /// Syncs the latest backup automatically from Google Drive
  Future<SyncStatus> syncLatestFromDrive({String? passphrase}) async {
    final backups = await gdrive.listBackups();
    if (backups.isEmpty) {
      throw Exception('No backup files found in Google Drive');
    }

    final latest = backups.first;
    final bytes = await gdrive.downloadBackupFile(latest.id);

    return await restoreFromBytes(bytes, latest.name, passphrase: passphrase);
  }

  /// Restores from raw file bytes (supports .imbk2, .imbk1, and plaintext .db / .sqlite)
  Future<SyncStatus> restoreFromBytes(
    Uint8List rawBytes,
    String fileName, {
    String? passphrase,
  }) async {
    Uint8List sqliteBytes;

    // 1. Check if payload is IMBK2/IMBK1 encrypted
    if (ImbkDecryptor.isImbkFile(rawBytes)) {
      final pass = passphrase?.trim().isNotEmpty == true
          ? passphrase!
          : (await getSavedPassphrase() ?? '');

      if (pass.isEmpty) {
        throw Exception('Backup is encrypted with IMBK2. Passphrase is required.');
      }

      // Remember passphrase for future syncs
      await savePassphrase(pass);

      // Decrypt
      sqliteBytes = await ImbkDecryptor.decrypt(rawBytes, pass);
    } else if (ImbkDecryptor.isSqliteDatabase(rawBytes)) {
      // Direct SQLite database
      sqliteBytes = rawBytes;
    } else {
      throw Exception('Unrecognized backup format. Expected .imbk2 or .db SQLite file.');
    }

    // 2. Write decrypted bytes to temporary staging file
    final tempDir = await getTemporaryDirectory();
    final tempDbFile = File('${tempDir.path}/restored_staging_${DateTime.now().millisecondsSinceEpoch}.db');
    await tempDbFile.writeAsBytes(sqliteBytes);

    // 3. Atomically replace app database
    await DBHelper.instance.replaceDatabaseWithFile(tempDbFile.path);

    // Clean up temporary staging file
    try {
      await tempDbFile.delete();
    } catch (_) {}

    // 4. Update sync metadata
    final prefs = await SharedPreferences.getInstance();
    final nowFormatted = DateFormat('dd MMM yyyy, hh:mm a').format(DateTime.now());
    await prefs.setString(_prefLastSyncTime, nowFormatted);
    await prefs.setString(_prefLastFileName, fileName);

    return await getStatus();
  }
}
