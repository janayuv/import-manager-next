class SyncStatus {
  final bool isConnectedToDrive;
  final String accountEmail;
  final String lastSyncTime;
  final String lastBackupFileName;
  final int shipmentCount;
  final int invoiceCount;
  final int itemCount;
  final int ldcRowCount;
  final bool isSyncing;
  final String syncMessage;

  SyncStatus({
    this.isConnectedToDrive = false,
    this.accountEmail = '',
    this.lastSyncTime = '',
    this.lastBackupFileName = '',
    this.shipmentCount = 0,
    this.invoiceCount = 0,
    this.itemCount = 0,
    this.ldcRowCount = 0,
    this.isSyncing = false,
    this.syncMessage = 'Idle',
  });

  SyncStatus copyWith({
    bool? isConnectedToDrive,
    String? accountEmail,
    String? lastSyncTime,
    String? lastBackupFileName,
    int? shipmentCount,
    int? invoiceCount,
    int? itemCount,
    int? ldcRowCount,
    bool? isSyncing,
    String? syncMessage,
  }) {
    return SyncStatus(
      isConnectedToDrive: isConnectedToDrive ?? this.isConnectedToDrive,
      accountEmail: accountEmail ?? this.accountEmail,
      lastSyncTime: lastSyncTime ?? this.lastSyncTime,
      lastBackupFileName: lastBackupFileName ?? this.lastBackupFileName,
      shipmentCount: shipmentCount ?? this.shipmentCount,
      invoiceCount: invoiceCount ?? this.invoiceCount,
      itemCount: itemCount ?? this.itemCount,
      ldcRowCount: ldcRowCount ?? this.ldcRowCount,
      isSyncing: isSyncing ?? this.isSyncing,
      syncMessage: syncMessage ?? this.syncMessage,
    );
  }
}

class DriveBackupFile {
  final String id;
  final String name;
  final int sizeBytes;
  final String modifiedTime;

  DriveBackupFile({
    required this.id,
    required this.name,
    required this.sizeBytes,
    required this.modifiedTime,
  });

  factory DriveBackupFile.fromMap(Map<String, dynamic> map) {
    return DriveBackupFile(
      id: map['id']?.toString() ?? '',
      name: map['name']?.toString() ?? '',
      sizeBytes: int.tryParse(map['size']?.toString() ?? '0') ?? 0,
      modifiedTime: map['modifiedTime']?.toString() ?? '',
    );
  }
}
