package models

// BackupRecord represents an encrypted backup artifact registered in backups.
type BackupRecord struct {
	ID            int64  `json:"id"`
	Filename      string `json:"filename"`
	Path          string `json:"path"`
	Destination   string `json:"destination"`
	SizeBytes     int64  `json:"sizeBytes"`
	SHA256        string `json:"sha256"`
	CreatedBy     string `json:"createdBy"`
	CreatedAt     string `json:"createdAt"`
	RetentionDays int64  `json:"retentionDays,omitempty"`
	Notes         string `json:"notes"`
	Status        string `json:"status"`
	ErrorMessage  string `json:"errorMessage,omitempty"`
	SidecarExists bool   `json:"sidecarExists"`
}

// CreateBackupRequest configures encrypted IMBK2 snapshot creation.
type CreateBackupRequest struct {
	Passphrase    string `json:"passphrase,omitempty"`
	SavePassToKey bool   `json:"savePassToKey,omitempty"`
	Destination   string `json:"destination,omitempty"` // local | gdrive
	OutputDir     string `json:"outputDir,omitempty"`
	Notes         string `json:"notes,omitempty"`
}

// RestoreBackupRequest restores or dry-run validates an IMBK2 / legacy IMBK1 backup file.
type RestoreBackupRequest struct {
	BackupPath   string `json:"backupPath"`
	Passphrase   string `json:"passphrase,omitempty"`
	DryRun       bool   `json:"dryRun"`
	VerifySidecar bool  `json:"verifySidecar"`
}

// RestoreVerificationSummary describes parity and structural checks from backup dry-run or execution.
type RestoreVerificationSummary struct {
	DryRun           bool             `json:"dryRun"`
	SourcePath       string           `json:"sourcePath"`
	BackupFormat     string           `json:"backupFormat"` // IMBK2 | IMBK1
	PayloadEncrypted bool             `json:"payloadEncrypted"`
	SidecarVerified  bool             `json:"sidecarVerified"`
	IntegrityCheck   string           `json:"integrityCheck"`
	ForeignKeysOK    bool             `json:"foreignKeysOk"`
	SafetyBackupPath string           `json:"safetyBackupPath,omitempty"`
	TableCounts      map[string]int64 `json:"tableCounts"`
	TotalRows        int64            `json:"totalRows"`
	RestoredAt       string           `json:"restoredAt,omitempty"`
}

// BackupSchedule represents a scheduled backup rule in backup_schedules.
type BackupSchedule struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	CronExpr       string `json:"cronExpr"`
	TimeZone       string `json:"timeZone"`
	Destination    string `json:"destination"`
	RetentionCount int64  `json:"retentionCount"`
	RetentionDays  int64  `json:"retentionDays"`
	Enabled        bool   `json:"enabled"`
	LastRun        string `json:"lastRun,omitempty"`
	NextRun        string `json:"nextRun,omitempty"`
	CreatedBy      string `json:"createdBy"`
	CreatedAt      string `json:"createdAt"`
	Notes          string `json:"notes"`
}

// SaveBackupScheduleInput creates or updates a scheduled backup configuration.
type SaveBackupScheduleInput struct {
	ID             int64  `json:"id,omitempty"`
	Name           string `json:"name"`
	CronExpr       string `json:"cronExpr"`
	TimeZone       string `json:"timeZone"`
	Destination    string `json:"destination"`
	RetentionCount int64  `json:"retentionCount"`
	RetentionDays  int64  `json:"retentionDays"`
	Enabled        bool   `json:"enabled"`
	Notes          string `json:"notes"`
}

// IMKeyOperationResult summarizes .imkey key export/import.
type IMKeyOperationResult struct {
	FilePath         string `json:"filePath"`
	HasBackupPass    bool   `json:"hasBackupPass"`
	StoredInKeyring  bool   `json:"storedInKeyring"`
	CompletedAt      string `json:"completedAt"`
}

// GoogleDriveStatus describes stored Google Drive OAuth credentials and connection metadata.
type GoogleDriveStatus struct {
	Connected       bool   `json:"connected"`
	HasClientConfig bool   `json:"hasClientConfig"`
	AccountEmail    string `json:"accountEmail,omitempty"`
	FolderID        string `json:"folderId,omitempty"`
	FolderName      string `json:"folderName"`
	TokenExpiry     string `json:"tokenExpiry,omitempty"`
	RetentionLimit  int    `json:"retentionLimit"`
}

// GoogleDriveConfigInput registers OAuth2 desktop client credentials in Windows Credential Manager.
type GoogleDriveConfigInput struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

// GoogleDriveRemoteFile represents a cloud backup stored in ImportManagerBackups.
type GoogleDriveRemoteFile struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	SizeBytes    int64  `json:"sizeBytes"`
	CreatedTime  string `json:"createdTime"`
	ModifiedTime string `json:"modifiedTime"`
	Locator      string `json:"locator"` // gdrive:<fileId>
}

// RecycleBinRecord is one soft-deleted record shown in the centralized Recycle Bin explorer.
type RecycleBinRecord struct {
	TableName  string         `json:"tableName"`
	RecordID   string         `json:"recordId"`
	Label      string         `json:"label"`
	Summary    string         `json:"summary"`
	DeletedAt  string         `json:"deletedAt"`
	DeletedBy  string         `json:"deletedBy"`
	RawFields  map[string]any `json:"rawFields,omitempty"`
}

// RecycleBinListResponse returns paginated soft-deleted records and aggregate reconciliation counts.
type RecycleBinListResponse struct {
	TableName         string             `json:"tableName"`
	Search            string             `json:"search"`
	Page              int                `json:"page"`
	PageSize          int                `json:"pageSize"`
	Total             int64              `json:"total"`
	AggregateDeleted  int64              `json:"aggregateDeleted"`
	TableSumDeleted   int64              `json:"tableSumDeleted"`
	CountMismatch     bool               `json:"countMismatch"`
	PerTableCounts    map[string]int64   `json:"perTableCounts"`
	Rows              []RecycleBinRecord `json:"rows"`
}

// MissingParentGroup groups missing or soft-deleted parent IDs blocking restore.
type MissingParentGroup struct {
	TableName string   `json:"tableName"`
	ParentIDs []string `json:"parentIds"`
}

// RecycleBinActionResult returns structured counts or dependency/missing_parent blockers.
type RecycleBinActionResult struct {
	Action           string               `json:"action"` // restore | purge | purge_expired
	TableName        string               `json:"tableName"`
	ProcessedCount   int                  `json:"processedCount"`
	SkippedCount     int                  `json:"skippedCount"`
	Blocked          bool                 `json:"blocked"`
	ErrorCode        string               `json:"errorCode,omitempty"` // missing_parent | DEPENDENCY
	Message          string               `json:"message"`
	MissingParents   []MissingParentGroup `json:"missingParents,omitempty"`
	Dependencies     []DependencyCount    `json:"dependencies,omitempty"`
}

// DependencyCount identifies child table references pointing to a candidate record or set.
type DependencyCount struct {
	ChildTable  string `json:"childTable"`
	ForeignKey  string `json:"foreignKey"`
	ActiveCount int64  `json:"activeCount"`
	TotalCount  int64  `json:"totalCount"`
}

// DatabaseOverviewStats summarizes live encrypted database health, size, and table row counts.
type DatabaseOverviewStats struct {
	DatabasePath     string           `json:"databasePath"`
	CipherVersion    string           `json:"cipherVersion"`
	JournalMode      string           `json:"journalMode"`
	ForeignKeysOn    bool             `json:"foreignKeysOn"`
	IntegrityCheck   string           `json:"integrityCheck"`
	PageCount        int64            `json:"pageCount"`
	PageSize         int64            `json:"pageSize"`
	FileSizeBytes    int64            `json:"fileSizeBytes"`
	SchemaVersion    int              `json:"schemaVersion"`
	RestoreLocked    bool             `json:"restoreLocked"`
	TableCounts      map[string]int64 `json:"tableCounts"`
	DeletedCounts    map[string]int64 `json:"deletedCounts"`
}

// BrowseTableRequest requests paginated rows from an allowlisted table.
type BrowseTableRequest struct {
	TableName      string `json:"tableName"`
	Search         string `json:"search"`
	IncludeDeleted bool   `json:"includeDeleted"`
	Page           int    `json:"page"`
	PageSize       int    `json:"pageSize"`
}

// BrowseTableResponse returns allowlisted table schema columns and rows.
type BrowseTableResponse struct {
	TableName   string           `json:"tableName"`
	PrimaryKey  string           `json:"primaryKey"`
	Editable    bool             `json:"editable"`
	Columns     []string         `json:"columns"`
	Page        int              `json:"page"`
	PageSize    int              `json:"pageSize"`
	Total       int64            `json:"total"`
	Rows        []map[string]any `json:"rows"`
}

// UpdateTableCellRequest updates a single allowlisted cell in the Database Console inline editor.
type UpdateTableCellRequest struct {
	TableName string `json:"tableName"`
	RowID     string `json:"rowId"`
	Column    string `json:"column"`
	Value     string `json:"value"`
	IsNull    bool   `json:"isNull"`
}

// BulkDeleteRequest soft-deletes or PIN-gated hard-deletes selected rows in chunks of 500.
type BulkDeleteRequest struct {
	TableName          string   `json:"tableName"`
	RecordIDs          []string `json:"recordIds"`
	HardDelete         bool     `json:"hardDelete"`
	Pin                string   `json:"pin,omitempty"`
	HighImpactConfirm  string   `json:"highImpactConfirm,omitempty"` // Must be "DELETE" when total dependencies > 5000
}

// BulkDeleteResult returns counts, chunks processed, optional undo token (5-min window), and dependency info.
type BulkDeleteResult struct {
	TableName         string            `json:"tableName"`
	Mode              string            `json:"mode"` // soft | hard
	RequestedCount    int               `json:"requestedCount"`
	ProcessedCount    int               `json:"processedCount"`
	ChunkCount        int               `json:"chunkCount"`
	TotalDependencies int64             `json:"totalDependencies"`
	Dependencies      []DependencyCount `json:"dependencies,omitempty"`
	UndoToken         string            `json:"undoToken,omitempty"`
	UndoExpiresAt     string            `json:"undoExpiresAt,omitempty"`
}

// LegacyInspectRequest inspects a legacy Import Manager .db file non-destructively.
type LegacyInspectRequest struct {
	SourcePath string `json:"sourcePath"`
	SourceKey  string `json:"sourceKey,omitempty"`
}

// LegacyFinancialSummary totals financial columns for migration parity checks.
type LegacyFinancialSummary struct {
	InvoiceValueSum       float64 `json:"invoiceValueSum"`
	BOETotalDutyPaise     int64   `json:"boeTotalDutyPaise"`
	ExpenseAmountPaise    int64   `json:"expenseAmountPaise"`
	ExpenseTotalPaise     int64   `json:"expenseTotalPaise"`
}

// LegacyTableInspect reports row counts in source vs target for one mapped table.
type LegacyTableInspect struct {
	TableName     string   `json:"tableName"`
	SourceExists  bool     `json:"sourceExists"`
	SourceRows    int64    `json:"sourceRows"`
	TargetRows    int64    `json:"targetRows"`
	MatchedCols   []string `json:"matchedCols"`
	MissingInSrc  []string `json:"missingInSrc,omitempty"`
}

// LegacyInspectResult reports detected format, encryption, schema history, row counts, and financial totals.
type LegacyInspectResult struct {
	SourcePath        string                 `json:"sourcePath"`
	FileSizeBytes     int64                  `json:"fileSizeBytes"`
	IsPlaintextSQLite bool                   `json:"isPlaintextSqlite"`
	IsEncrypted       bool                   `json:"isEncrypted"`
	KeyValid          bool                   `json:"keyValid"`
	MigrationVersion  string                 `json:"migrationVersion"`
	Tables            []LegacyTableInspect   `json:"tables"`
	SourceFinancials  LegacyFinancialSummary `json:"sourceFinancials"`
	Warnings          []string               `json:"warnings,omitempty"`
}

// LegacyMigrationRequest runs dry-run or live commit migration from a legacy database.
type LegacyMigrationRequest struct {
	SourcePath string `json:"sourcePath"`
	SourceKey  string `json:"sourceKey,omitempty"`
	DryRun     bool   `json:"dryRun"`
}

// LegacyMigrationReport is the verifiable parity report produced after dry-run or live import.
type LegacyMigrationReport struct {
	DryRun            bool                   `json:"dryRun"`
	SourcePath        string                 `json:"sourcePath"`
	SafetyBackupPath  string                 `json:"safetyBackupPath,omitempty"`
	IntegrityCheck    string                 `json:"integrityCheck"`
	ForeignKeysOK     bool                   `json:"foreignKeysOk"`
	ForeignKeyErrors  int                    `json:"foreignKeyErrors"`
	RowsCopiedByTable map[string]int64       `json:"rowsCopiedByTable"`
	RowParityOK       bool                   `json:"rowParityOk"`
	SourceFinancials  LegacyFinancialSummary `json:"sourceFinancials"`
	TargetFinancials  LegacyFinancialSummary `json:"targetFinancials"`
	FinancialParityOK bool                   `json:"financialParityOk"`
	Warnings          []string               `json:"warnings,omitempty"`
	CompletedAt       string                 `json:"completedAt"`
}
