# Legacy Command Inventory – Import Manager (Tauri IPC -> Wails 3 Go Services)

> **Source of Truth:** `src-tauri/src/main.rs::generate_handler![]` + 41 Rust modules
> **Totals:** **384** explicit `#[tauri::command]` definitions (`381` registered in `main.rs`, `3` unregistered in `generate_handler![]`: `delete_shipment`, `get_shipment_by_id`, `generate_id`) + **26** macro-generated option commands in `options.rs` = **407 registered IPC handlers (410 total functions)**.

---

## Summary by Module

| Module File | Domain | Commands | Target Wails 3 Go Service | Notes / Security Gate |
| :--- | :--- | :---: | :--- | :--- |
| `src-tauri\src\desktop_session.rs` | Desktop Authentication & Session Lifecycle | 6 | `SecurityService` | Argon2id/bcrypt, 12h/30d sessions, admin bootstrap |
| `src-tauri\src\expense.rs` | Expense Paise/BP Invoice Engine | 4 | `ExpenseService` | expense.* / integer paise & basis points, idempotency |
| `src-tauri\src\commands\app_info.rs` | Application Version & System Paths | 2 | `AppLifecycleService` | Version, AppData paths |
| `src-tauri\src\commands\app_metadata.rs` | App Metadata KV Accessors | 2 | `AppLifecycleService` | app_metadata reads/writes |
| `src-tauri\src\commands\app_settings.rs` | Application Settings Store | 2 | `AppLifecycleService` | app_settings persistence |
| `src-tauri\src\commands\backup_key.rs` | Backup Key Keyring (.imkey) | 4 | `BackupService` | Windows Credential Manager backup_encryption_password |
| `src-tauri\src\commands\boe.rs` | Bill of Entry (BOE) & Duty Entry | 26 | `BOEService / BOEEntryService` | boe.* / server-side calculation & attachment storage |
| `src-tauri\src\commands\bug_notes.rs` | Internal Bug Notes Tracker | 5 | `DiagnosticsService` | bug_notes CRUD |
| `src-tauri\src\commands\dashboard_metrics.rs` | Dashboard & KPI Snapshots | 10 | `DashboardService` | dashboard.* / FY filter, cache, daily snapshots |
| `src-tauri\src\commands\db_management.rs` | Database Management & Encrypted Backups | 38 | `DatabaseManagementService / BackupService` | db.* / IMBK2 AES-256-GCM, bulk delete/undo, restore lock |
| `src-tauri\src\commands\diagnostics.rs` | Diagnostics ZIP Bundle Export | 1 | `DiagnosticsService` | audit.view / exports redacted diagnostic zip |
| `src-tauri\src\commands\error_memory.rs` | Runtime Error Memory & Cleanup | 6 | `DiagnosticsService` | error_memory capture, search, pruning |
| `src-tauri\src\commands\exception_reliability.rs` | Exception Integrity, Escalation & Load Sim | 5 | `WorkflowService` | SLA escalation, priority recalc, integrity checks |
| `src-tauri\src\commands\exception_workflow.rs` | Exception Workflow Cases & SLA | 7 | `WorkflowService` | OPEN -> IN_PROGRESS -> RESOLVED/IGNORED |
| `src-tauri\src\commands\expenses.rs` | Legacy Expenses & Admin Diagnostics | 29 | `ExpenseService` | expense.* / bulk import & admin repair tools |
| `src-tauri\src\commands\google_drive.rs` | Google Drive OAuth & Cloud Backup | 9 | `GoogleDriveService` | OAuth2 127.0.0.1:8765, Drive API v3 ImportManagerBackups |
| `src-tauri\src\commands\invoices.rs` | Commercial Invoices & Precision | 6 | `InvoiceService` | invoice.* / freeze guarded |
| `src-tauri\src\commands\items.rs` | Item Master | 4 | `ItemMasterService` | item.* / soft-delete supported |
| `src-tauri\src\commands\logs.rs` | Application Log Reader | 1 | `DiagnosticsService` | Reads runtime log files |
| `src-tauri\src\commands\ops_admin.rs` | Dashboard Snapshot Rebuild | 1 | `DashboardService` | admin / rebuilds daily KPI/exception/workflow snapshots |
| `src-tauri\src\commands\options.rs` | Lookup Options | 1 | `OptionService` | App dropdowns & option tables (+26 macro get_*/add_* commands) |
| `src-tauri\src\commands\recovery.rs` | Emergency Recovery Mode | 5 | `SecurityService` | Active only with --recovery or IMPORT_MANAGER_RECOVERY=1 |
| `src-tauri\src\commands\recycle_bin.rs` | Recycle Bin & Soft-Delete Restore | 5 | `RecycleBinService` | Parent-existence validation, FK dependency scan |
| `src-tauri\src\commands\reference_scan.rs` | Hard-Delete Dependency Reference Scanner | 3 | `DatabaseManagementService` | Pre-delete FK/reference blocker check across tables |
| `src-tauri\src\commands\reliability.rs` | Write Recovery Journal & System Integrity | 4 | `DatabaseManagementService` | write_recovery_journal & boe_write_recovery |
| `src-tauri\src\commands\reports.rs` | Landed Cost & Expense Reports | 8 | `ReportService / ExpenseReportService` | report.* / queries report_view & expenses |
| `src-tauri\src\commands\security.rs` | RBAC, Security Policy, Lockout & Audit | 12 | `SecurityService / AuditService` | 4 roles, 25 perms, password policy, lockout management |
| `src-tauri\src\commands\shipments.rs` | Shipment Management | 31 | `ShipmentService` | shipment.* / delete/freeze guarded |
| `src-tauri\src\commands\suppliers.rs` | Supplier Management | 10 | `SupplierService` | supplier.* / role-checked via session |
| `src-tauri\src\commands\system_agent.rs` | DeepSeek System Agent | 4 | `DiagnosticsService` | automation.system_agent / DeepSeek keyring |
| `src-tauri\src\commands\system_health.rs` | System Health Metrics | 1 | `DiagnosticsService` | DB/runtime health snapshot |
| `src-tauri\src\commands\test_reset.rs` | Test Reset Utility | 1 | `Test/Dev Only` | Disabled in production builds; resets test state |
| `src-tauri\src\commands\user_activity_audit.rs` | User Activity Audit Stream | 1 | `AuditService` | admin.user_activity |
| `src-tauri\src\commands\utils.rs` | Shared Utility Commands | 3 | `AppLifecycleService` | Helper IPC utilities |
| `src-tauri\src\commands\workflow_automation.rs` | Workflow Decision Rules, Cost & ROI | 37 | `WorkflowService` | Requires admin/automationmanager (Next Gen uses server session) |
| `src-tauri\src\commands\workflow_incident_management.rs` | Incident Correlation, Suppression & Forecasting | 13 | `WorkflowService` | Root cause >=50 chars, burst detection, probability forecast |
| `src-tauri\src\commands\workflow_job_monitoring.rs` | Background Job Monitoring & Recovery Guard | 23 | `Scheduler / WorkflowService` | 6 jobs, timeout 2x/2h, recovery guard disable |
| `src-tauri\src\commands\workflow_multienv.rs` | Multi-Environment & Tenant Context | 9 | `WorkflowService` | DEV -> TEST -> PROD one-step promotion |
| `src-tauri\src\commands\workflow_observability.rs` | Workflow Health, AI Consistency & Self-Heal | 10 | `WorkflowService` | Runtime anomaly score, self-healing recovery |
| `src-tauri\src\commands\workflow_production_observability.rs` | Production Alerts, Metrics & Health | 6 | `WorkflowService` | Alert suppression, reliability score |
| `src-tauri\src\commands\workflow_rule_deployment.rs` | Rule Versioning, Canary & Deployment Safety | 29 | `WorkflowService` | Approval gate, freeze, canary FNV-1a, risk score |
| `src-tauri\src\commands\options.rs` (macro) | Lookup Options | 26 | `OptionService` | 13 `get_*` + 13 `add_*` macro-generated commands |

---

## Desktop Authentication & Session Lifecycle – `src-tauri\src\desktop_session.rs` (6 commands)

- **Target Go Service:** `SecurityService`
- **Security / Policy Note:** Argon2id/bcrypt, 12h/30d sessions, admin bootstrap

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L259 | `get_desktop_auth_setup_status` | `(db_state: State<'_, DbState>,) -> Result<DesktopAuthSetupStatus, String>` | Yes | `SecurityService.GetDesktopAuthSetupStatus` |
| L277 | `complete_desktop_admin_setup` | `(input: CompleteDesktopAdminSetupInput, db_state: State<'_, DbState>,) -> Result<(), IpcError>` | Yes | `SecurityService.CompleteDesktopAdminSetup` |
| L342 | `authenticate_desktop` | `(username: String, password: String, remember_me: bool, state: State<DesktopSessionState>, db_state: State<DbState>, recovery: State<RecoveryModeState>,) -> Result<DesktopSessionInfo, IpcError>` | Yes | `SecurityService.AuthenticateDesktop` |
| L551 | `get_desktop_session` | `(session: State<DesktopSessionState>, db: State<DbState>,) -> Result<Option<DesktopSessionInfo>, String>` | Yes | `SecurityService.GetDesktopSession` |
| L635 | `terminate_desktop_session_for_admin` | `(caller_user_id: String, target_session_id: Option<String>, session: State<DesktopSessionState>, db_state: State<DbState>,) -> Result<(), String>` | Yes | `SecurityService.TerminateDesktopSessionForAdmin` |
| L696 | `clear_desktop_session` | `(session: State<DesktopSessionState>, db_state: State<DbState>,) -> Result<(), String>` | Yes | `SecurityService.ClearDesktopSession` |

## Expense Paise/BP Invoice Engine – `src-tauri\src\expense.rs` (4 commands)

- **Target Go Service:** `ExpenseService`
- **Security / Policy Note:** expense.* / integer paise & basis points, idempotency

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L1687 | `create_expense_invoice` | `(payload: ExpenseInvoicePayload, state: State<'_, DbState>, session: State<'_, crate::desktop_session::DesktopSessionState>,) -> Result<ExpenseInvoiceResponse, String>` | Yes | `ExpenseService.CreateExpenseInvoice` |
| L1699 | `preview_expense_invoice` | `(payload: ExpenseInvoicePayload, state: State<'_, DbState>,) -> Result<ExpenseInvoicePreview, String>` | Yes | `ExpenseService.PreviewExpenseInvoice` |
| L1708 | `combine_expense_duplicates` | `(invoice_id: String, request: CombineDuplicatesRequest, state: State<'_, DbState>,) -> Result<ExpenseInvoiceResponse, String>` | Yes | `ExpenseService.CombineExpenseDuplicates` |
| L1719 | `get_expense_invoice` | `(invoice_id: String, state: State<'_, DbState>,) -> Result<ExpenseInvoiceResponse, String>` | Yes | `ExpenseService.GetExpenseInvoice` |

## Application Version & System Paths – `src-tauri\src\commands\app_info.rs` (2 commands)

- **Target Go Service:** `AppLifecycleService`
- **Security / Policy Note:** Version, AppData paths

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L4 | `get_shell_version` | `() -> String` | Yes | `AppLifecycleService.GetShellVersion` |
| L12 | `log_client_event` | `(level: String, message: String) -> ()` | Yes | `AppLifecycleService.LogClientEvent` |

## App Metadata KV Accessors – `src-tauri\src\commands\app_metadata.rs` (2 commands)

- **Target Go Service:** `AppLifecycleService`
- **Security / Policy Note:** app_metadata reads/writes

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L9 | `get_app_metadata_value` | `(db_state: State<'_, DbState>, key: String,) -> Result<Option<String>, String>` | Yes | `AppLifecycleService.GetAppMetadataValue` |
| L28 | `set_app_metadata_value` | `(db_state: State<'_, DbState>, key: String, value: String,) -> Result<(), String>` | Yes | `AppLifecycleService.SetAppMetadataValue` |

## Application Settings Store – `src-tauri\src\commands\app_settings.rs` (2 commands)

- **Target Go Service:** `AppLifecycleService`
- **Security / Policy Note:** app_settings persistence

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L85 | `get_invoice_calculation_settings` | `(state: State<DbState>,) -> Result<InvoiceCalculationSettings, String>` | Yes | `AppLifecycleService.GetInvoiceCalculationSettings` |
| L93 | `set_invoice_calculation_settings` | `(line_total_decimals: u8, invoice_total_decimals: u8, user_id: Option<String>, state: State<DbState>,) -> Result<InvoiceCalculationSettings, IpcError>` | Yes | `AppLifecycleService.SetInvoiceCalculationSettings` |

## Backup Key Keyring (.imkey) – `src-tauri\src\commands\backup_key.rs` (4 commands)

- **Target Go Service:** `BackupService`
- **Security / Policy Note:** Windows Credential Manager backup_encryption_password

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L16 | `has_backup_key_in_keyring` | `() -> Result<bool, String>` | Yes | `BackupService.HasBackupKeyInKeyring` |
| L24 | `export_backup_key_to_path` | `(path: String, user_id: Option<String>, db_state: State<'_, DbState>,) -> Result<(), IpcError>` | Yes | `BackupService.ExportBackupKeyToPath` |
| L46 | `export_backup_key` | `(window: WebviewWindow, user_id: Option<String>, db_state: State<'_, DbState>,) -> Result<(), IpcError>` | Yes | `BackupService.ExportBackupKey` |
| L81 | `import_backup_key_from_path` | `(path: String, replace_confirmed: bool, user_id: Option<String>, db_state: State<'_, DbState>,) -> Result<(), IpcError>` | Yes | `BackupService.ImportBackupKeyFromPath` |

## Bill of Entry (BOE) & Duty Entry – `src-tauri\src\commands\boe.rs` (26 commands)

- **Target Go Service:** `BOEService / BOEEntryService`
- **Security / Policy Note:** boe.* / server-side calculation & attachment storage

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L71 | `get_boes` | `(state: State<DbState>) -> Result<Vec<BoeDetails>, String>` | Yes | `BOEService.GetBoes` |
| L105 | `get_boes_paginated` | `(page: i64, page_size: i64, _state: State<DbState>, connection_manager: State<ConnectionManager>,) -> Result<PaginatedResult<BoeDetails>, String>` | Yes | `BOEService.GetBoesPaginated` |
| L165 | `add_boe` | `(payload: NewBoePayload, state: State<DbState>) -> Result<String, String>` | Yes | `BOEService.AddBoe` |
| L239 | `update_boe` | `(boe: BoeDetails, state: State<DbState>) -> Result<(), String>` | Yes | `BOEService.UpdateBoe` |
| L318 | `delete_boe` | `(id: String, state: State<DbState>) -> Result<(), String>` | Yes | `BOEService.DeleteBoe` |
| L400 | `get_boe_calculations` | `(state: State<DbState>) -> Result<Vec<SavedBoe>, String>` | Yes | `BOEService.GetBoeCalculations` |
| L423 | `get_boe_calculations_paginated` | `(page: i64, page_size: i64, _state: State<DbState>, connection_manager: State<ConnectionManager>,) -> Result<PaginatedResult<SavedBoe>, String>` | Yes | `BOEService.GetBoeCalculationsPaginated` |
| L469 | `add_boe_calculation` | `(payload: SavedBoe, state: State<DbState>) -> Result<String, String>` | Yes | `BOEService.AddBoeCalculation` |
| L599 | `update_boe_calculation` | `(payload: SavedBoe, state: State<DbState>) -> Result<(), String>` | Yes | `BOEService.UpdateBoeCalculation` |
| L710 | `delete_boe_calculation` | `(id: String, state: State<DbState>) -> Result<(), String>` | Yes | `BOEService.DeleteBoeCalculation` |
| L720 | `get_shipments_for_boe_entry` | `(state: State<DbState>) -> Result<Vec<BoeShipment>, String>` | Yes | `BOEService.GetShipmentsForBoeEntry` |
| L826 | `get_shipments_for_boe_entry_paginated` | `(page: i64, page_size: i64, _state: State<DbState>, connection_manager: State<ConnectionManager>,) -> Result<PaginatedResult<BoeShipment>, String>` | Yes | `BOEService.GetShipmentsForBoeEntryPaginated` |
| L962 | `get_shipments_for_boe_summary` | `(state: State<DbState>) -> Result<Vec<BoeShipment>, String>` | Yes | `BOEService.GetShipmentsForBoeSummary` |
| L1062 | `update_boe_status` | `(id: String, status: String, state: State<DbState>) -> Result<(), String>` | Yes | `BOEService.UpdateBoeStatus` |
| L1076 | `add_boe_attachment` | `(id: String, attachment: Attachment, state: State<DbState>,) -> Result<(), String>` | Yes | `BOEService.AddBoeAttachment` |
| L1121 | `get_boe_reconciliation` | `(saved_boe_id: String, state: State<DbState>,) -> Result<BoeReconciliationReport, String>` | Yes | `BOEService.GetBoeReconciliation` |
| L1273 | `save_boe_attachment_file` | `(app: tauri::AppHandle, id: String, src_path: String,) -> Result<String, String>` | Yes | `BOEService.SaveBoeAttachmentFile` |
| L1322 | `save_item_photo_file` | `(app: tauri::AppHandle, src_path: String) -> Result<String, String>` | Yes | `BOEService.SaveItemPhotoFile` |
| L1363 | `get_shipment_ids_with_boe_calculations` | `(state: State<DbState>,) -> Result<Vec<String>, String>` | Yes | `BOEService.GetShipmentIdsWithBoeCalculations` |
| L1381 | `reconcile_boe_attachments_command` | `(state: State<DbState>) -> Result<i64, String>` | Yes | `BOEService.ReconcileBoeAttachments` |
| L1388 | `validate_boe_integrity_command` | `(state: State<DbState>) -> Result<serde_json::Value, String>` | Yes | `BOEService.ValidateBoeIntegrity` |
| L1395 | `get_boe_largest_json_rows` | `(limit: Option<i64>, state: State<DbState>,) -> Result<serde_json::Value, String>` | Yes | `BOEService.GetBoeLargestJsonRows` |
| L1405 | `analyze_boe_query_plans_command` | `(state: State<DbState>) -> Result<serde_json::Value, String>` | Yes | `BOEService.AnalyzeBoeQueryPlans` |
| L1427 | `get_boe_health_summary_command` | `(state: State<DbState>, connection_manager: State<ConnectionManager>,) -> Result<serde_json::Value, String>` | Yes | `BOEService.GetBoeHealthSummary` |
| L1443 | `recover_interrupted_boe_writes_command` | `(state: State<DbState>) -> Result<i64, String>` | Yes | `BOEService.RecoverInterruptedBoeWrites` |
| L1449 | `validate_global_integrity_command` | `(state: State<DbState>,) -> Result<serde_json::Value, String>` | Yes | `BOEService.ValidateGlobalIntegrity` |

## Internal Bug Notes Tracker – `src-tauri\src\commands\bug_notes.rs` (5 commands)

- **Target Go Service:** `DiagnosticsService`
- **Security / Policy Note:** bug_notes CRUD

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L214 | `create_bug_note` | `(state: State<DbState>, payload: CreateBugNotePayload,) -> Result<BugNote, String>` | Yes | `DiagnosticsService.CreateBugNote` |
| L256 | `get_bug_notes` | `(state: State<DbState>, status: Option<String>, limit: Option<i64>,) -> Result<Vec<BugNote>, String>` | Yes | `DiagnosticsService.GetBugNotes` |
| L304 | `update_bug_note` | `(state: State<DbState>, payload: UpdateBugNotePayload,) -> Result<BugNote, String>` | Yes | `DiagnosticsService.UpdateBugNote` |
| L364 | `delete_bug_note` | `(app: tauri::AppHandle, state: State<DbState>, id: String,) -> Result<(), String>` | Yes | `DiagnosticsService.DeleteBugNote` |
| L382 | `save_bug_screenshot` | `(app: tauri::AppHandle, payload: SaveBugScreenshotPayload,) -> Result<String, String>` | Yes | `DiagnosticsService.SaveBugScreenshot` |

## Dashboard & KPI Snapshots – `src-tauri\src\commands\dashboard_metrics.rs` (10 commands)

- **Target Go Service:** `DashboardService`
- **Security / Policy Note:** dashboard.* / FY filter, cache, daily snapshots

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L1251 | `get_dashboard_metrics` | `(filters: Option<DashboardMetricsFilters>, state: State<DbState>, connection_manager: State<ConnectionManager>,) -> Result<DashboardMetricsResponse, IpcError>` | Yes | `DashboardService.GetDashboardMetrics` |
| L1582 | `get_kpi_metadata` | `(state: State<DbState>) -> Result<Vec<KpiMetadataRow>, String>` | Yes | `DashboardService.GetKpiMetadata` |
| L1615 | `get_kpi_snapshot_history` | `(query: Option<KpiSnapshotQuery>, state: State<DbState>,) -> Result<Vec<KpiSnapshotHistoryRow>, String>` | Yes | `DashboardService.GetKpiSnapshotHistory` |
| L1759 | `get_kpi_alert_rules` | `(state: State<DbState>) -> Result<Vec<KpiAlertRuleRow>, String>` | Yes | `DashboardService.GetKpiAlertRules` |
| L1783 | `save_kpi_alert_rule` | `(rule: KpiAlertRuleInput, state: State<DbState>) -> Result<(), String>` | Yes | `DashboardService.SaveKpiAlertRule` |
| L1814 | `log_dashboard_activity` | `(input: DashboardActivityInput, state: State<DbState>, session: State<crate::desktop_session::DesktopSessionState>,) -> Result<(), String>` | Yes | `DashboardService.LogDashboardActivity` |
| L1857 | `get_dashboard_activity_log` | `(limit: Option<i32>, caller_user_id: String, state: State<DbState>, session: State<crate::desktop_session::DesktopSessionState>,) -> Result<Vec<DashboardActivityRow>, String>` | Yes | `DashboardService.GetDashboardActivityLog` |
| L1983 | `query_dashboard_activity_log` | `(query: Option<ActivityLogQueryInput>, caller_user_id: String, state: State<DbState>, session: State<crate::desktop_session::DesktopSessionState>,) -> Result<Vec<DashboardActivityRow>, String>` | Yes | `DashboardService.QueryDashboardActivityLog` |
| L1996 | `get_exception_trend_history` | `(limit_days: Option<i64>, state: State<DbState>,) -> Result<Vec<DailyExceptionSummaryRow>, String>` | Yes | `DashboardService.GetExceptionTrendHistory` |
| L2027 | `set_kpi_snapshot_retention_days` | `(days: i64, state: State<DbState>) -> Result<(), String>` | Yes | `DashboardService.SetKpiSnapshotRetentionDays` |

## Database Management & Encrypted Backups – `src-tauri\src\commands\db_management.rs` (38 commands)

- **Target Go Service:** `DatabaseManagementService / BackupService`
- **Security / Policy Note:** db.* / IMBK2 AES-256-GCM, bulk delete/undo, restore lock

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L826 | `get_hard_delete_pin_settings` | `(db_state: State<'_, DbState>,) -> Result<HardDeletePinSettings, String>` | Yes | `DatabaseManagementService.GetHardDeletePinSettings` |
| L841 | `set_hard_delete_pin_enabled` | `(db_state: State<'_, DbState>, enabled: bool,) -> Result<(), String>` | Yes | `DatabaseManagementService.SetHardDeletePinEnabled` |
| L856 | `set_hard_delete_pin_threshold` | `(db_state: State<'_, DbState>, threshold: u32,) -> Result<(), String>` | Yes | `DatabaseManagementService.SetHardDeletePinThreshold` |
| L869 | `set_hard_delete_pin` | `(db_state: State<'_, DbState>, pin: String) -> Result<(), String>` | Yes | `DatabaseManagementService.SetHardDeletePin` |
| L892 | `change_hard_delete_pin` | `(db_state: State<'_, DbState>, current_pin: String, new_pin: String,) -> Result<(), String>` | Yes | `DatabaseManagementService.ChangeHardDeletePin` |
| L915 | `verify_hard_delete_pin` | `(db_state: State<'_, DbState>, pin: String,) -> Result<HardDeletePinVerifyResult, String>` | Yes | `DatabaseManagementService.VerifyHardDeletePin` |
| L1952 | `create_audit_log` | `(db_state: State<'_, DbState>, tableName: String, row_id: Option<String>, action: String, userId: Option<String>, before_json: Option<String>, after_json: Option<String>, metadata: Option<String>,) -> Result<i64, String>` | Yes | `DatabaseManagementService.CreateAuditLog` |
| L1988 | `get_audit_logs` | `(db_state: State<'_, DbState>, tableName: Option<String>, action: Option<String>, userId: Option<String>, limit: Option<i64>, offset: Option<i64>,) -> Result<Vec<AuditLog>, String>` | Yes | `DatabaseManagementService.GetAuditLogs` |
| L2102 | `get_database_stats` | `(db_state: State<'_, DbState>) -> Result<DatabaseStats, String>` | Yes | `DatabaseManagementService.GetDatabaseStats` |
| L2210 | `get_backup_redundancy_settings` | `(db_state: State<'_, DbState>,) -> Result<BackupRedundancySettings, String>` | Yes | `DatabaseManagementService.GetBackupRedundancySettings` |
| L2229 | `set_backup_redundancy_settings` | `(db_state: State<'_, DbState>, input: SetBackupRedundancyInput, userId: Option<String>,) -> Result<(), String>` | Yes | `DatabaseManagementService.SetBackupRedundancySettings` |
| L2264 | `get_backup_health_metrics` | `(db_state: State<'_, DbState>,) -> Result<BackupHealthMetrics, String>` | Yes | `DatabaseManagementService.GetBackupHealthMetrics` |
| L2422 | `create_backup` | `(window: WebviewWindow, db_state: State<'_, DbState>, request: BackupRequest, userId: Option<String>,) -> Result<BackupInfo, String>` | Yes | `DatabaseManagementService.CreateBackup` |
| L2887 | `get_backup_history` | `(db_state: State<'_, DbState>, limit: Option<i64>,) -> Result<Vec<BackupInfo>, String>` | Yes | `DatabaseManagementService.GetBackupHistory` |
| L3001 | `soft_delete_record` | `(db_state: State<'_, DbState>, tableName: String, record_id: String, userId: Option<String>,) -> Result<(), String>` | Yes | `DatabaseManagementService.SoftDeleteRecord` |
| L3046 | `hard_delete_record` | `(db_state: State<'_, DbState>, tableName: String, record_id: String, userId: Option<String>, confirmation: String,) -> Result<(), String>` | Yes | `DatabaseManagementService.HardDeleteRecord` |
| L3138 | `preview_restore` | `(window: WebviewWindow, db_state: State<'_, DbState>, backupPath: String,) -> Result<RestorePreview, String>` | Yes | `DatabaseManagementService.PreviewRestore` |
| L3670 | `restore_database` | `(window: WebviewWindow, db_state: State<'_, DbState>, backupPath: String, userId: Option<String>,) -> Result<RestoreResult, String>` | Yes | `DatabaseManagementService.RestoreDatabase` |
| L4359 | `bulk_search_records` | `(db_state: State<'_, DbState>, tableName: String, filters: HashMap<String, serde_json::Value>, pageSize: Option<i64>, includeDeleted: Option<bool>,) -> Result<TableData, String>` | Yes | `DatabaseManagementService.BulkSearchRecords` |
| L4556 | `get_bulk_manageable_tables` | `(db_state: State<'_, DbState>,) -> Result<Vec<BulkManageableTable>, String>` | Yes | `DatabaseManagementService.GetBulkManageableTables` |
| L5067 | `bulk_get_matching_record_ids` | `(db_state: State<'_, DbState>, tableName: String, filters: HashMap<String, serde_json::Value>, includeDeleted: Option<bool>,) -> Result<Vec<String>, String>` | Yes | `DatabaseManagementService.BulkGetMatchingRecordIds` |
| L5107 | `bulk_delete_records` | `(app: AppHandle, db_state: State<'_, DbState>, tableName: String, record_ids: Vec<String>, userId: Option<String>, delete_type: String, // "soft" or "hard") -> Result<BulkDeleteResult, String>` | Yes | `DatabaseManagementService.BulkDeleteRecords` |
| L5217 | `bulk_delete_records_by_filter` | `(app: AppHandle, db_state: State<'_, DbState>, tableName: String, filters: HashMap<String, serde_json::Value>, includeDeleted: Option<bool>, excludedRecordIds: Vec<String>, expectedSelectedCount: usize, operationTimeoutMs: Option<u64>, userId: Option<String>, deleteType: String,) -> Result<BulkDeleteResult, String>` | Yes | `DatabaseManagementService.BulkDeleteRecordsByFilter` |
| L5384 | `restore_deleted_records_using_token` | `(db_state: State<'_, DbState>, undoToken: String,) -> Result<BulkDeleteResult, String>` | Yes | `DatabaseManagementService.RestoreDeletedRecordsUsingToken` |
| L5477 | `create_backup_schedule` | `(db_state: State<'_, DbState>, name: String, cron_expr: String, destination: String, retention_count: Option<i32>, retention_days: Option<i32>, enabled: Option<bool>, time_zone: Option<String>, notes: Option<String>, userId: Option<String>,) -> Result<i64, String>` | Yes | `DatabaseManagementService.CreateBackupSchedule` |
| L5540 | `get_backup_schedules` | `(db_state: State<'_, DbState>,) -> Result<Vec<BackupSchedule>, String>` | Yes | `DatabaseManagementService.GetBackupSchedules` |
| L5578 | `update_backup_schedule` | `(db_state: State<'_, DbState>, schedule_id: i64, name: Option<String>, cron_expr: Option<String>, destination: Option<String>, retention_count: Option<i32>, retention_days: Option<i32>, enabled: Option<bool>, time_zone: Option<String>, notes: Option<String>, userId: Option<String>,) -> Result<(), String>` | Yes | `DatabaseManagementService.UpdateBackupSchedule` |
| L5693 | `delete_backup_schedule` | `(db_state: State<'_, DbState>, schedule_id: i64, userId: Option<String>,) -> Result<(), String>` | Yes | `DatabaseManagementService.DeleteBackupSchedule` |
| L5744 | `run_scheduled_backup` | `(app: AppHandle, db_state: State<'_, DbState>, schedule_id: i64, userId: Option<String>,) -> Result<BackupInfo, String>` | Yes | `DatabaseManagementService.RunScheduledBackup` |
| L5862 | `create_user_role` | `(db_state: State<'_, DbState>, userId: String, role: String, permissions: Option<String>, created_by: Option<String>,) -> Result<i64, String>` | Yes | `DatabaseManagementService.CreateUserRole` |
| L5920 | `get_user_roles` | `(db_state: State<'_, DbState>, caller_user_id: Option<String>,) -> Result<Vec<UserRole>, String>` | Yes | `DatabaseManagementService.GetUserRoles` |
| L5954 | `update_user_role` | `(db_state: State<'_, DbState>, role_id: i64, role: Option<String>, permissions: Option<String>, updated_by: Option<String>,) -> Result<(), String>` | Yes | `DatabaseManagementService.UpdateUserRole` |
| L6053 | `delete_user_role` | `(db_state: State<'_, DbState>, role_id: i64, deleted_by: Option<String>,) -> Result<(), String>` | Yes | `DatabaseManagementService.DeleteUserRole` |
| L6119 | `bootstrap_first_admin_if_eligible` | `(db_state: State<'_, DbState>, userId: String, caller_user_id: Option<String>,) -> Result<(), String>` | Yes | `DatabaseManagementService.BootstrapFirstAdminIfEligible` |
| L6133 | `check_user_permission` | `(db_state: State<'_, DbState>, userId: String, permission: String,) -> Result<bool, String>` | Yes | `DatabaseManagementService.CheckUserPermission` |
| L6144 | `get_user_permissions` | `(_db_state: State<'_, DbState>, userId: String,) -> Result<Vec<String>, String>` | Yes | `DatabaseManagementService.GetUserPermissions` |
| L6244 | `browse_table_data` | `(db_state: State<'_, DbState>, tableName: String, page: Option<i64>, pageSize: Option<i64>, includeDeleted: Option<bool>,) -> Result<TableData, String>` | Yes | `DatabaseManagementService.BrowseTableData` |
| L6360 | `update_record` | `(db_state: State<'_, DbState>, request: RecordUpdate,) -> Result<UpdateResult, String>` | Yes | `DatabaseManagementService.UpdateRecord` |

## Diagnostics ZIP Bundle Export – `src-tauri\src\commands\diagnostics.rs` (1 commands)

- **Target Go Service:** `DiagnosticsService`
- **Security / Policy Note:** audit.view / exports redacted diagnostic zip

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L454 | `export_diagnostics_bundle` | `(caller_user_id: String, output_path: String, client_reported_app_version: Option<String>, session: tauri::State<'_, DesktopSessionState>, app: tauri::AppHandle,) -> Result<(), String>` | Yes | `DiagnosticsService.ExportDiagnosticsBundle` |

## Runtime Error Memory & Cleanup – `src-tauri\src\commands\error_memory.rs` (6 commands)

- **Target Go Service:** `DiagnosticsService`
- **Security / Policy Note:** error_memory capture, search, pruning

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L709 | `capture_error_event` | `(state: State<'_, DbState>, payload: ErrorEventPayload,) -> Result<String, String>` | Yes | `DiagnosticsService.CaptureErrorEvent` |
| L721 | `list_error_events` | `(state: State<'_, DbState>, filter: Option<ErrorMemoryFilter>,) -> Result<Vec<ErrorMemoryRow>, String>` | Yes | `DiagnosticsService.ListErrorEvents` |
| L815 | `update_error_event_status` | `(state: State<'_, DbState>, id: String, status: String,) -> Result<(), String>` | Yes | `DiagnosticsService.UpdateErrorEventStatus` |
| L831 | `get_error_memory_maintenance_stats` | `(state: State<'_, DbState>,) -> Result<ErrorMemoryMaintenanceStats, String>` | Yes | `DiagnosticsService.GetErrorMemoryMaintenanceStats` |
| L886 | `run_error_memory_cleanup` | `(state: State<'_, DbState>, request: Option<ErrorMemoryCleanupRequest>,) -> Result<ErrorMemoryCleanupResult, String>` | Yes | `DiagnosticsService.RunErrorMemoryCleanup` |
| L917 | `export_error_events_cursor_report` | `(state: State<'_, DbState>, ids: Vec<String>,) -> Result<String, String>` | Yes | `DiagnosticsService.ExportErrorEventsCursorReport` |

## Exception Integrity, Escalation & Load Sim – `src-tauri\src\commands\exception_reliability.rs` (5 commands)

- **Target Go Service:** `WorkflowService`
- **Security / Policy Note:** SLA escalation, priority recalc, integrity checks

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L744 | `validate_exception_integrity_command` | `(state: State<DbState>) -> Result<i64, String>` | Yes | `WorkflowService.ValidateExceptionIntegrity` |
| L750 | `revalidate_open_exceptions_command` | `(state: State<DbState>) -> Result<(), String>` | Yes | `WorkflowService.RevalidateOpenExceptions` |
| L829 | `simulate_rule_execution_command` | `(caller_role: String, state: State<DbState>,) -> Result<serde_json::Value, String>` | Yes | `WorkflowService.SimulateRuleExecution` |
| L839 | `simulate_exception_load_command` | `(count: Option<i32>, user_id: String, include_rule_simulation: Option<bool>, state: State<DbState>,) -> Result<SimulateLoadReport, String>` | Yes | `WorkflowService.SimulateExceptionLoad` |
| L855 | `get_exception_reliability_report` | `(state: State<DbState>,) -> Result<serde_json::Value, String>` | Yes | `WorkflowService.GetExceptionReliabilityReport` |

## Exception Workflow Cases & SLA – `src-tauri\src\commands\exception_workflow.rs` (7 commands)

- **Target Go Service:** `WorkflowService`
- **Security / Policy Note:** OPEN -> IN_PROGRESS -> RESOLVED/IGNORED

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L563 | `list_exception_cases` | `(query: Option<ExceptionCaseQuery>, state: State<DbState>,) -> Result<Vec<EntityExceptionDto>, String>` | Yes | `WorkflowService.ListExceptionCases` |
| L615 | `update_exception_case` | `(input: UpdateExceptionCaseInput, state: State<DbState>,) -> Result<(), String>` | Yes | `WorkflowService.UpdateExceptionCase` |
| L706 | `add_exception_note` | `(input: AddExceptionNoteInput, state: State<DbState>,) -> Result<(), String>` | Yes | `WorkflowService.AddExceptionNote` |
| L729 | `get_exception_lifecycle_events` | `(exception_case_id: String, state: State<DbState>,) -> Result<Vec<LifecycleEventRow>, String>` | Yes | `WorkflowService.GetExceptionLifecycleEvents` |
| L764 | `list_exception_notes` | `(exception_case_id: String, state: State<DbState>,) -> Result<Vec<ExceptionNoteRow>, String>` | Yes | `WorkflowService.ListExceptionNotes` |
| L791 | `record_exception_viewed` | `(exception_case_id: String, user_id: String, state: State<DbState>,) -> Result<(), String>` | Yes | `WorkflowService.RecordExceptionViewed` |
| L808 | `bulk_resolve_exception_cases` | `(input: BulkResolveInput, state: State<DbState>,) -> Result<i32, String>` | Yes | `WorkflowService.BulkResolveExceptionCases` |

## Legacy Expenses & Admin Diagnostics – `src-tauri\src\commands\expenses.rs` (29 commands)

- **Target Go Service:** `ExpenseService`
- **Security / Policy Note:** expense.* / bulk import & admin repair tools

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L16 | `get_service_providers` | `(state: State<DbState>) -> Result<Vec<ServiceProvider>, String>` | Yes | `ExpenseService.GetServiceProviders` |
| L42 | `add_service_provider` | `(name: String, state: State<DbState>,) -> Result<ServiceProvider, String>` | Yes | `ExpenseService.AddServiceProvider` |
| L69 | `get_expense_types` | `(state: State<DbState>) -> Result<Vec<ExpenseType>, String>` | Yes | `ExpenseService.GetExpenseTypes` |
| L94 | `debug_expense_types` | `(state: State<DbState>) -> Result<String, String>` | Yes | `ExpenseService.DebugExpenseTypes` |
| L161 | `fix_expense_types` | `(state: State<DbState>) -> Result<String, String>` | Yes | `ExpenseService.FixExpenseTypes` |
| L249 | `debug_expense_data` | `(state: State<DbState>) -> Result<String, String>` | Yes | `ExpenseService.DebugExpenseData` |
| L308 | `clear_expense_data` | `(state: State<DbState>) -> Result<String, String>` | Yes | `ExpenseService.ClearExpenseData` |
| L326 | `cleanup_orphaned_expenses` | `(state: State<DbState>) -> Result<String, String>` | Yes | `ExpenseService.CleanupOrphanedExpenses` |
| L462 | `fix_existing_expenses` | `(state: State<DbState>) -> Result<String, String>` | Yes | `ExpenseService.FixExistingExpenses` |
| L571 | `fix_lcl_charges_rate` | `(state: State<DbState>) -> Result<String, String>` | Yes | `ExpenseService.FixLclChargesRate` |
| L642 | `add_expense_type` | `(name: String, state: State<DbState>) -> Result<ExpenseType, String>` | Yes | `ExpenseService.AddExpenseType` |
| L664 | `add_expense_type_with_rates` | `(name: String, cgst_rate: i32, sgst_rate: i32, igst_rate: i32, state: State<DbState>,) -> Result<ExpenseType, String>` | Yes | `ExpenseService.AddExpenseTypeWithRates` |
| L693 | `get_expense_invoices_for_shipment` | `(shipment_id: String, state: State<DbState>,) -> Result<Vec<ExpenseInvoice>, String>` | Yes | `ExpenseService.GetExpenseInvoicesForShipment` |
| L729 | `check_expense_invoice_exists` | `(service_provider_id: String, invoice_no: String, state: State<DbState>,) -> Result<bool, String>` | Yes | `ExpenseService.CheckExpenseInvoiceExists` |
| L749 | `get_expenses_for_invoice` | `(expense_invoice_id: String, state: State<DbState>,) -> Result<Vec<Expense>, String>` | Yes | `ExpenseService.GetExpensesForInvoice` |
| L789 | `get_expenses_for_shipment` | `(shipment_id: String, state: State<DbState>,) -> Result<Vec<ExpenseWithInvoice>, String>` | Yes | `ExpenseService.GetExpensesForShipment` |
| L832 | `attach_invoice_to_expense` | `(app: tauri::AppHandle, expense_id: String, src_path: String, file_type: Option<String>, user_id: Option<String>, state: State<DbState>,) -> Result<ExpenseAttachment, String>` | Yes | `ExpenseService.AttachInvoiceToExpense` |
| L891 | `generate_shipment_expense_report` | `(shipment_id: String, state: State<DbState>,) -> Result<Vec<Expense>, String>` | Yes | `ExpenseService.GenerateShipmentExpenseReport` |
| L954 | `generate_monthly_gst_summary` | `(month: u32, year: i32, state: State<DbState>,) -> Result<Vec<Expense>, String>` | Yes | `ExpenseService.GenerateMonthlyGstSummary` |
| L1113 | `add_expense_invoice_with_expenses` | `(payload: ExpenseInvoiceWithExpensesPayload, state: State<'_, DbState>, session: State<'_, DesktopSessionState>,) -> Result<ExpenseInvoice, String>` | Yes | `ExpenseService.AddExpenseInvoiceWithExpenses` |
| L1245 | `add_expense` | `(payload: ExpensePayload, state: State<'_, DbState>, session: State<'_, DesktopSessionState>,) -> Result<Expense, String>` | Yes | `ExpenseService.AddExpense` |
| L1329 | `update_expense` | `(id: String, payload: ExpensePayload, state: State<'_, DbState>,) -> Result<Expense, String>` | Yes | `ExpenseService.UpdateExpense` |
| L1399 | `delete_expense` | `(id: String, state: State<DbState>) -> Result<(), String>` | Yes | `ExpenseService.DeleteExpense` |
| L1437 | `add_expenses_bulk` | `(payload: BulkExpensePayload, state: State<DbState>,) -> Result<String, String>` | Yes | `ExpenseService.AddExpensesBulk` |
| L1619 | `delete_expense_invoice` | `(invoice_id: String, state: State<DbState>) -> Result<(), String>` | Yes | `ExpenseService.DeleteExpenseInvoice` |
| L1663 | `debug_expense_data_counts` | `(state: State<DbState>) -> Result<String, String>` | Yes | `ExpenseService.DebugExpenseDataCounts` |
| L1719 | `create_test_expense_data` | `(state: State<DbState>) -> Result<String, String>` | Yes | `ExpenseService.CreateTestExpenseData` |
| L1893 | `cleanup_orphaned_expense_invoices` | `(state: State<DbState>) -> Result<String, String>` | Yes | `ExpenseService.CleanupOrphanedExpenseInvoices` |
| L1949 | `get_shipment_ids_with_expense_lines` | `(state: State<DbState>) -> Result<Vec<String>, String>` | Yes | `ExpenseService.GetShipmentIdsWithExpenseLines` |

## Google Drive OAuth & Cloud Backup – `src-tauri\src\commands\google_drive.rs` (9 commands)

- **Target Go Service:** `GoogleDriveService`
- **Security / Policy Note:** OAuth2 127.0.0.1:8765, Drive API v3 ImportManagerBackups

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L194 | `google_drive_reset_cancel` | `() -> ()` | Yes | `GoogleDriveService.GoogleDriveResetCancel` |
| L201 | `google_drive_cancel_operation` | `() -> ()` | Yes | `GoogleDriveService.GoogleDriveCancelOperation` |
| L309 | `google_drive_status` | `(db_state: State<'_, DbState>, user_id: Option<String>,) -> Result<GoogleDriveStatus, IpcError>` | Yes | `GoogleDriveService.GoogleDriveStatus` |
| L367 | `google_drive_refresh_profile` | `(db_state: State<'_, DbState>, user_id: Option<String>,) -> Result<Option<String>, IpcError>` | Yes | `GoogleDriveService.GoogleDriveRefreshProfile` |
| L399 | `google_drive_disconnect` | `(db_state: State<'_, DbState>, user_id: Option<String>,) -> Result<(), IpcError>` | Yes | `GoogleDriveService.GoogleDriveDisconnect` |
| L427 | `google_drive_connect` | `(db_state: State<'_, DbState>, user_id: Option<String>,) -> Result<(), IpcError>` | Yes | `GoogleDriveService.GoogleDriveConnect` |
| L1449 | `get_google_oauth_credentials` | `(db_state: State<'_, DbState>, user_id: Option<String>,) -> Result<GoogleOAuthCredentialsInfo, IpcError>` | Yes | `GoogleDriveService.GetGoogleOauthCredentials` |
| L1477 | `set_google_oauth_credentials` | `(db_state: State<'_, DbState>, client_id: String, client_secret: String, user_id: Option<String>,) -> Result<(), IpcError>` | Yes | `GoogleDriveService.SetGoogleOauthCredentials` |
| L1514 | `clear_google_oauth_credentials` | `(db_state: State<'_, DbState>, user_id: Option<String>,) -> Result<(), IpcError>` | Yes | `GoogleDriveService.ClearGoogleOauthCredentials` |

## Commercial Invoices & Precision – `src-tauri\src\commands\invoices.rs` (6 commands)

- **Target Go Service:** `InvoiceService`
- **Security / Policy Note:** invoice.* / freeze guarded

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L195 | `get_invoices` | `(state: State<DbState>, connection_manager: State<ConnectionManager>,) -> Result<Vec<Invoice>, IpcError>` | Yes | `InvoiceService.GetInvoices` |
| L340 | `bulk_finalize_invoices` | `(input: BulkFinalizeInvoicesInput, state: State<DbState>,) -> Result<BulkFinalizeInvoicesResult, String>` | Yes | `InvoiceService.BulkFinalizeInvoices` |
| L410 | `add_invoice` | `(payload: NewInvoicePayload, state: State<DbState>) -> Result<String, String>` | Yes | `InvoiceService.AddInvoice` |
| L459 | `add_invoices_bulk` | `(payloads: Vec<NewInvoicePayload>, state: State<DbState>,) -> Result<Vec<String>, String>` | Yes | `InvoiceService.AddInvoicesBulk` |
| L566 | `update_invoice` | `(id: String, payload: NewInvoicePayload, state: State<DbState>,) -> Result<(), String>` | Yes | `InvoiceService.UpdateInvoice` |
| L636 | `delete_invoice` | `(id: String, state: State<DbState>) -> Result<(), String>` | Yes | `InvoiceService.DeleteInvoice` |

## Item Master – `src-tauri\src\commands\items.rs` (4 commands)

- **Target Go Service:** `ItemMasterService`
- **Security / Policy Note:** item.* / soft-delete supported

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L6 | `get_items` | `(state: State<DbState>) -> Result<Vec<Item>, String>` | Yes | `ItemMasterService.GetItems` |
| L42 | `add_items_bulk` | `(state: State<DbState>, items: Vec<Item>) -> Result<(), String>` | Yes | `ItemMasterService.AddItemsBulk` |
| L63 | `add_item` | `(item: Item, state: State<DbState>) -> Result<(), String>` | Yes | `ItemMasterService.AddItem` |
| L79 | `update_item` | `(item: Item, state: State<DbState>) -> Result<(), String>` | Yes | `ItemMasterService.UpdateItem` |

## Application Log Reader – `src-tauri\src\commands\logs.rs` (1 commands)

- **Target Go Service:** `DiagnosticsService`
- **Security / Policy Note:** Reads runtime log files

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L8 | `get_application_logs` | `(app: tauri::AppHandle) -> Result<Vec<String>, String>` | Yes | `DiagnosticsService.GetApplicationLogs` |

## Dashboard Snapshot Rebuild – `src-tauri\src\commands\ops_admin.rs` (1 commands)

- **Target Go Service:** `DashboardService`
- **Security / Policy Note:** admin / rebuilds daily KPI/exception/workflow snapshots

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L32 | `rebuild_dashboard_snapshots` | `(caller_user_id: String, state: State<DbState>, session: State<DesktopSessionState>,) -> Result<RebuildDashboardResult, String>` | Yes | `DashboardService.RebuildDashboardSnapshots` |

## Lookup Options – `src-tauri\src\commands\options.rs` (1 commands)

- **Target Go Service:** `OptionService`
- **Security / Policy Note:** App dropdowns & option tables (+26 macro get_*/add_* commands)

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L67 | `add_option` | `(option_type: String, option: SelectOption, state: State<DbState>,) -> Result<(), String>` | Yes | `OptionService.AddOption` |

## Emergency Recovery Mode – `src-tauri\src\commands\recovery.rs` (5 commands)

- **Target Go Service:** `SecurityService`
- **Security / Policy Note:** Active only with --recovery or IMPORT_MANAGER_RECOVERY=1

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L36 | `is_recovery_mode_active` | `(recovery: State<'_, RecoveryModeState>) -> bool` | Yes | `SecurityService.IsRecoveryModeActive` |
| L41 | `get_recovery_mode_status` | `(recovery: State<'_, RecoveryModeState>) -> RecoveryModeStatus` | Yes | `SecurityService.GetRecoveryModeStatus` |
| L50 | `recovery_clear_lockout` | `(recovery: State<'_, RecoveryModeState>, session: State<'_, DesktopSessionState>, db: State<'_, DbState>,) -> Result<(), String>` | Yes | `SecurityService.RecoveryClearLockout` |
| L82 | `recovery_reset_security_policy` | `(recovery: State<'_, RecoveryModeState>, db: State<'_, DbState>,) -> Result<SecurityPolicy, String>` | Yes | `SecurityService.RecoveryResetSecurityPolicy` |
| L112 | `recovery_set_admin_password` | `(recovery: State<'_, RecoveryModeState>, db: State<'_, DbState>, new_password: String,) -> Result<(), IpcError>` | Yes | `SecurityService.RecoverySetAdminPassword` |

## Recycle Bin & Soft-Delete Restore – `src-tauri\src\commands\recycle_bin.rs` (5 commands)

- **Target Go Service:** `RecycleBinService`
- **Security / Policy Note:** Parent-existence validation, FK dependency scan

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L76 | `get_soft_delete_tables` | `(db_state: tauri::State<'_, DbState>, userId: Option<String>,) -> Result<Vec<String>, IpcError>` | Yes | `RecycleBinService.GetSoftDeleteTables` |
| L89 | `get_recycle_bin_deleted_count` | `(db_state: tauri::State<'_, DbState>, userId: Option<String>,) -> Result<i64, IpcError>` | Yes | `RecycleBinService.GetRecycleBinDeletedCount` |
| L758 | `get_deleted_records` | `(db_state: tauri::State<'_, DbState>, userId: Option<String>, tableName: Option<String>, search: Option<String>, page: Option<u32>, pageSize: Option<u32>,) -> Result<GetDeletedRecordsResponse, IpcError>` | Yes | `RecycleBinService.GetDeletedRecords` |
| L1098 | `restore_deleted_records` | `(db_state: tauri::State<'_, DbState>, tableName: String, recordIds: Vec<String>, userId: Option<String>,) -> Result<String, IpcError>` | Yes | `RecycleBinService.RestoreDeletedRecords` |
| L1197 | `permanently_delete_records` | `(db_state: tauri::State<'_, DbState>, tableName: String, recordIds: Vec<String>, userId: Option<String>,) -> Result<String, IpcError>` | Yes | `RecycleBinService.PermanentlyDeleteRecords` |

## Hard-Delete Dependency Reference Scanner – `src-tauri\src\commands\reference_scan.rs` (3 commands)

- **Target Go Service:** `DatabaseManagementService`
- **Security / Policy Note:** Pre-delete FK/reference blocker check across tables

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L727 | `preview_hard_delete_impact` | `(db_state: State<'_, DbState>, tableName: String, recordIds: Vec<String>,) -> Result<HardDeleteImpactPreview, String>` | Yes | `DatabaseManagementService.PreviewHardDeleteImpact` |
| L1346 | `get_reference_counts` | `(db_state: State<'_, DbState>, tableName: String, recordIds: Vec<String>,) -> Result<Vec<RecordReferenceCounts>, String>` | Yes | `DatabaseManagementService.GetReferenceCounts` |
| L1362 | `preview_delete_dependencies` | `(db_state: State<'_, DbState>, tableName: String, recordIds: Vec<String>,) -> Result<PreviewDeleteDependenciesResult, String>` | Yes | `DatabaseManagementService.PreviewDeleteDependencies` |

## Write Recovery Journal & System Integrity – `src-tauri\src\commands\reliability.rs` (4 commands)

- **Target Go Service:** `DatabaseManagementService`
- **Security / Policy Note:** write_recovery_journal & boe_write_recovery

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L9 | `recover_interrupted_writes_command` | `(state: State<DbState>) -> Result<i64, String>` | Yes | `DatabaseManagementService.RecoverInterruptedWrites` |
| L15 | `validate_system_integrity_command` | `(state: State<DbState>,) -> Result<serde_json::Value, String>` | Yes | `DatabaseManagementService.ValidateSystemIntegrity` |
| L24 | `get_platform_health_summary_command` | `(state: State<DbState>, connection_manager: State<ConnectionManager>,) -> Result<serde_json::Value, String>` | Yes | `DatabaseManagementService.GetPlatformHealthSummary` |
| L34 | `run_memory_guard_command` | `() -> Result<(), String>` | Yes | `DatabaseManagementService.RunMemoryGuard` |

## Landed Cost & Expense Reports – `src-tauri\src\commands\reports.rs` (8 commands)

- **Target Go Service:** `ReportService / ExpenseReportService`
- **Security / Policy Note:** report.* / queries report_view & expenses

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L120 | `get_report` | `(filters: ReportFilters, state: State<DbState>, connection_manager: State<ConnectionManager>,) -> Result<ReportResponse, String>` | Yes | `ReportService.GetReport` |
| L313 | `generate_detailed_expense_report` | `(filters: ExpenseReportFilters, state: State<DbState>,) -> Result<ExpenseReportResponse, String>` | Yes | `ReportService.GenerateDetailedExpenseReport` |
| L323 | `generate_expense_summary_by_type` | `(filters: ExpenseReportFilters, state: State<DbState>,) -> Result<Vec<ExpenseSummaryByType>, String>` | Yes | `ReportService.GenerateExpenseSummaryByType` |
| L333 | `generate_expense_summary_by_provider` | `(filters: ExpenseReportFilters, state: State<DbState>,) -> Result<Vec<ExpenseSummaryByProvider>, String>` | Yes | `ReportService.GenerateExpenseSummaryByProvider` |
| L343 | `generate_expense_summary_by_shipment` | `(filters: ExpenseReportFilters, state: State<DbState>,) -> Result<Vec<ExpenseSummaryByShipment>, String>` | Yes | `ReportService.GenerateExpenseSummaryByShipment` |
| L353 | `generate_expense_summary_by_month` | `(filters: ExpenseReportFilters, state: State<DbState>,) -> Result<Vec<ExpenseSummaryByMonth>, String>` | Yes | `ReportService.GenerateExpenseSummaryByMonth` |
| L363 | `debug_expense_report_filters` | `(filters: ExpenseReportFilters, state: State<DbState>,) -> Result<String, String>` | Yes | `ReportService.DebugExpenseReportFilters` |
| L389 | `debug_expense_dates` | `(state: State<DbState>) -> Result<String, String>` | Yes | `ReportService.DebugExpenseDates` |

## RBAC, Security Policy, Lockout & Audit – `src-tauri\src\commands\security.rs` (12 commands)

- **Target Go Service:** `SecurityService / AuditService`
- **Security / Policy Note:** 4 roles, 25 perms, password policy, lockout management

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L53 | `get_my_permissions` | `(caller_user_id: String, session: State<'_, DesktopSessionState>, db: State<'_, DbState>,) -> Result<Vec<String>, String>` | Yes | `SecurityService.GetMyPermissions` |
| L73 | `get_my_session` | `(session: State<'_, DesktopSessionState>,) -> Result<Option<DesktopSessionInfo>, String>` | Yes | `SecurityService.GetMySession` |
| L80 | `get_security_settings` | `(caller_user_id: String, session: State<'_, DesktopSessionState>, db: State<'_, DbState>,) -> Result<SecuritySettings, String>` | Yes | `SecurityService.GetSecuritySettings` |
| L98 | `update_security_settings` | `(caller_user_id: String, input: SecuritySettingsInput, session: State<'_, DesktopSessionState>, db: State<'_, DbState>,) -> Result<SecuritySettings, String>` | Yes | `SecurityService.UpdateSecuritySettings` |
| L185 | `get_lockout_status` | `(caller_user_id: String, target_user_id: Option<String>, session: State<'_, DesktopSessionState>, db: State<'_, DbState>,) -> Result<LockoutStatus, String>` | Yes | `SecurityService.GetLockoutStatus` |
| L203 | `reset_account_lockout` | `(caller_user_id: String, target_user_id: Option<String>, session: State<'_, DesktopSessionState>, db: State<'_, DbState>,) -> Result<(), String>` | Yes | `SecurityService.ResetAccountLockout` |
| L232 | `change_admin_password` | `(caller_user_id: String, current_password: String, new_password: String, session: State<'_, DesktopSessionState>, db: State<'_, DbState>,) -> Result<(), IpcError>` | Yes | `SecurityService.ChangeAdminPassword` |
| L336 | `get_recent_auth_events` | `(caller_user_id: String, limit: Option<i64>, // Supports server-side pagination; callers pass offset = page_index * page_size. offset: Option<i64>, session: State<'_, DesktopSessionState>, db: State<'_, DbState>,) -> Result<Vec<UserActivityAuditLog>, String>` | Yes | `SecurityService.GetRecentAuthEvents` |
| L382 | `get_role_permissions_matrix` | `() -> Vec<RolePermissionRow>` | Yes | `SecurityService.GetRolePermissionsMatrix` |
| L473 | `get_security_dashboard_metrics` | `(caller_user_id: String, session: State<'_, DesktopSessionState>, db: State<'_, DbState>,) -> Result<SecurityDashboardMetrics, String>` | Yes | `SecurityService.GetSecurityDashboardMetrics` |
| L603 | `get_active_desktop_sessions` | `(caller_user_id: String, session: State<'_, DesktopSessionState>, db: State<'_, DbState>,) -> Result<Vec<DesktopSessionInfo>, String>` | Yes | `SecurityService.GetActiveDesktopSessions` |
| L613 | `get_security_trend_series` | `(caller_user_id: String, days: Option<i64>, session: State<'_, DesktopSessionState>, db: State<'_, DbState>,) -> Result<Vec<SecurityTrendDay>, String>` | Yes | `SecurityService.GetSecurityTrendSeries` |

## Shipment Management – `src-tauri\src\commands\shipments.rs` (31 commands)

- **Target Go Service:** `ShipmentService`
- **Security / Policy Note:** shipment.* / delete/freeze guarded

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L28 | `get_shipments` | `(state: State<DbState>, connection_manager: State<ConnectionManager>,) -> Result<Vec<Shipment>, IpcError>` | Yes | `ShipmentService.GetShipments` |
| L43 | `get_shipments_paginated` | `(page: i64, page_size: i64, status: Option<String>, supplier_id: Option<String>, date_from: Option<String>, date_to: Option<String>, overdue_only: Option<bool>, boe_missing_only: Option<bool>, expense_missing_only: Option<bool>, state: State<DbState>, connection_manager: State<ConnectionManager>,) -> Result<PaginatedResult<Shipment>, IpcError>` | Yes | `ShipmentService.GetShipmentsPaginated` |
| L79 | `check_shipment_duplicate` | `(shipment_id: Option<String>, invoice_number: String, exclude_id: Option<String>, state: State<DbState>, connection_manager: State<ConnectionManager>,) -> Result<bool, String>` | Yes | `ShipmentService.CheckShipmentDuplicate` |
| L100 | `get_shipment_items_by_shipment_id` | `(shipment_id: String, state: State<DbState>, connection_manager: State<ConnectionManager>,) -> Result<Vec<ShipmentItemLite>, String>` | Yes | `ShipmentService.GetShipmentItemsByShipmentId` |
| L114 | `get_shipment_exception_summary` | `(state: State<DbState>, connection_manager: State<ConnectionManager>,) -> Result<ShipmentExceptionSummary, String>` | Yes | `ShipmentService.GetShipmentExceptionSummary` |
| L127 | `get_shipment_totals_by_status` | `(state: State<DbState>, connection_manager: State<ConnectionManager>,) -> Result<Vec<ShipmentStatusTotal>, String>` | Yes | `ShipmentService.GetShipmentTotalsByStatus` |
| L140 | `get_recent_shipments` | `(limit: Option<i64>, status: Option<String>, state: State<DbState>, connection_manager: State<ConnectionManager>,) -> Result<Vec<Shipment>, String>` | Yes | `ShipmentService.GetRecentShipments` |
| L155 | `analyze_shipment_query_plans_command` | `(state: State<DbState>, connection_manager: State<ConnectionManager>,) -> Result<Vec<ShipmentQueryPlanFinding>, String>` | Yes | `ShipmentService.AnalyzeShipmentQueryPlans` |
| L176 | `simulate_shipment_items_dual_write_command` | `(shipment_id: String, state: State<DbState>,) -> Result<String, String>` | Yes | `ShipmentService.SimulateShipmentItemsDualWrite` |
| L185 | `detect_invalid_shipment_date_rows_command` | `(state: State<DbState>,) -> Result<Vec<InvalidShipmentDateRow>, String>` | Yes | `ShipmentService.DetectInvalidShipmentDateRows` |
| L193 | `check_shipment_timezone_consistency_command` | `(state: State<DbState>,) -> Result<TimezoneConsistencyReport, String>` | Yes | `ShipmentService.CheckShipmentTimezoneConsistency` |
| L201 | `export_invalid_shipment_dates_csv_command` | `(state: State<DbState>) -> Result<String, String>` | Yes | `ShipmentService.ExportInvalidShipmentDatesCsv` |
| L207 | `normalize_shipment_dates_dry_run_command` | `(state: State<DbState>,) -> Result<ShipmentDateNormalizationDryRunReport, String>` | Yes | `ShipmentService.NormalizeShipmentDatesDryRun` |
| L215 | `apply_shipment_date_normalization_command` | `(state: State<DbState>, connection_manager: State<ConnectionManager>,) -> Result<ShipmentDateNormalizationApplyReport, String>` | Yes | `ShipmentService.ApplyShipmentDateNormalization` |
| L233 | `shipment_timezone_validation_report_command` | `(state: State<DbState>,) -> Result<ShipmentTimezoneValidationReport, String>` | Yes | `ShipmentService.ShipmentTimezoneValidationReport` |
| L241 | `snapshot_shipment_query_plan_baseline_command` | `(state: State<DbState>,) -> Result<ShipmentQueryPlanBaseline, String>` | Yes | `ShipmentService.SnapshotShipmentQueryPlanBaseline` |
| L249 | `get_shipment_by_id` | `(state: State<DbState>, id: String, connection_manager: State<ConnectionManager>,) -> Result<Option<Shipment>, String>` | No (Unregistered) | `ShipmentService.GetShipmentById` |
| L304 | `add_shipment` | `(state: State<DbState>, shipment: Shipment, connection_manager: State<ConnectionManager>,) -> Result<(), String>` | Yes | `ShipmentService.AddShipment` |
| L321 | `update_shipment` | `(state: State<DbState>, shipment: Shipment, connection_manager: State<ConnectionManager>,) -> Result<(), String>` | Yes | `ShipmentService.UpdateShipment` |
| L338 | `delete_shipment` | `(state: State<DbState>, id: String) -> Result<(), String>` | No (Unregistered) | `ShipmentService.DeleteShipment` |
| L383 | `update_shipment_status_on_invoice_add` | `(state: State<DbState>, shipment_id: String,) -> Result<(), String>` | Yes | `ShipmentService.UpdateShipmentStatusOnInvoiceAdd` |
| L402 | `update_shipment_status_on_boe_add` | `(state: State<DbState>, shipment_id: String,) -> Result<(), String>` | Yes | `ShipmentService.UpdateShipmentStatusOnBoeAdd` |
| L421 | `check_and_update_ready_for_delivery` | `(state: State<DbState>) -> Result<(), String>` | Yes | `ShipmentService.CheckAndUpdateReadyForDelivery` |
| L453 | `migrate_shipment_statuses` | `(state: State<DbState>) -> Result<(), String>` | Yes | `ShipmentService.MigrateShipmentStatuses` |
| L469 | `get_active_shipments` | `(state: State<DbState>, connection_manager: State<ConnectionManager>,) -> Result<Vec<Shipment>, String>` | Yes | `ShipmentService.GetActiveShipments` |
| L689 | `add_shipments_bulk` | `(state: State<DbState>, shipments: Vec<Shipment>, file_name: String, total_rows: usize, skipped_rows: usize, error_rows: usize,) -> Result<(), String>` | Yes | `ShipmentService.AddShipmentsBulk` |
| L752 | `log_shipment_import_result` | `(state: State<DbState>, file_name: String, total_rows: usize, inserted_rows: usize, skipped_rows: usize, error_rows: usize, status: String,) -> Result<(), String>` | Yes | `ShipmentService.LogShipmentImportResult` |
| L774 | `get_unfinalized_shipments` | `(state: State<DbState>, connection_manager: State<ConnectionManager>,) -> Result<Vec<Shipment>, String>` | Yes | `ShipmentService.GetUnfinalizedShipments` |
| L864 | `freeze_shipment` | `(state: State<DbState>, id: String) -> Result<(), String>` | Yes | `ShipmentService.FreezeShipment` |
| L878 | `update_shipment_status` | `(state: State<DbState>, id: String, status: String,) -> Result<(), String>` | Yes | `ShipmentService.UpdateShipmentStatus` |
| L896 | `validate_shipment_import` | `(shipments: Vec<Shipment>, state: State<DbState>,) -> Result<Vec<String>, String>` | Yes | `ShipmentService.ValidateShipmentImport` |

## Supplier Management – `src-tauri\src\commands\suppliers.rs` (10 commands)

- **Target Go Service:** `SupplierService`
- **Security / Policy Note:** supplier.* / role-checked via session

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L6 | `get_suppliers` | `(state: State<DbState>, limit: Option<i64>, offset: Option<i64>, search_text: Option<String>,) -> Result<Vec<Supplier>, String>` | Yes | `SupplierService.GetSuppliers` |
| L129 | `get_suppliers_count` | `(state: State<DbState>, search_text: Option<String>,) -> Result<i64, String>` | Yes | `SupplierService.GetSuppliersCount` |
| L161 | `get_deleted_suppliers` | `(state: State<DbState>) -> Result<Vec<Supplier>, String>` | Yes | `SupplierService.GetDeletedSuppliers` |
| L216 | `add_supplier` | `(state: State<DbState>, supplier: Supplier) -> Result<(), String>` | Yes | `SupplierService.AddSupplier` |
| L241 | `generate_supplier_id` | `(state: State<DbState>) -> Result<String, String>` | Yes | `SupplierService.GenerateSupplierId` |
| L271 | `update_supplier` | `(state: State<DbState>, supplier: Supplier) -> Result<(), String>` | Yes | `SupplierService.UpdateSupplier` |
| L296 | `add_suppliers_bulk` | `(state: State<DbState>, suppliers: Vec<Supplier>,) -> Result<usize, String>` | Yes | `SupplierService.AddSuppliersBulk` |
| L358 | `delete_supplier` | `(state: State<DbState>, supplier_id: String) -> Result<(), String>` | Yes | `SupplierService.DeleteSupplier` |
| L387 | `restore_supplier` | `(state: State<DbState>, supplier_id: String) -> Result<(), String>` | Yes | `SupplierService.RestoreSupplier` |
| L408 | `check_supplier_exists` | `(state: State<DbState>, supplier_id: String) -> Result<bool, String>` | Yes | `SupplierService.CheckSupplierExists` |

## DeepSeek System Agent – `src-tauri\src\commands\system_agent.rs` (4 commands)

- **Target Go Service:** `DiagnosticsService`
- **Security / Policy Note:** automation.system_agent / DeepSeek keyring

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L483 | `get_system_agent_settings` | `(caller_role: String, state: State<'_, DbState>,) -> Result<SystemAgentSettings, String>` | Yes | `DiagnosticsService.GetSystemAgentSettings` |
| L511 | `set_system_agent_settings` | `(caller_role: String, input: UpdateSystemAgentSettingsInput, state: State<'_, DbState>,) -> Result<SystemAgentSettings, String>` | Yes | `DiagnosticsService.SetSystemAgentSettings` |
| L844 | `system_agent_turn` | `(input: SystemAgentTurnInput, state: State<'_, DbState>,) -> Result<SystemAgentTurnOutput, String>` | Yes | `DiagnosticsService.SystemAgentTurn` |
| L862 | `get_system_agent_observability_summary` | `(caller_role: String, state: State<'_, DbState>,) -> Result<SystemAgentObservabilitySummary, String>` | Yes | `DiagnosticsService.GetSystemAgentObservabilitySummary` |

## System Health Metrics – `src-tauri\src\commands\system_health.rs` (1 commands)

- **Target Go Service:** `DiagnosticsService`
- **Security / Policy Note:** DB/runtime health snapshot

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L138 | `get_system_health_metrics` | `(db_state: tauri::State<'_, DbState>,) -> Result<SystemHealthMetrics, String>` | Yes | `DiagnosticsService.GetSystemHealthMetrics` |

## Test Reset Utility – `src-tauri\src\commands\test_reset.rs` (1 commands)

- **Target Go Service:** `Test/Dev Only`
- **Security / Policy Note:** Disabled in production builds; resets test state

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L30 | `reset_test_database` | `(app: AppHandle, state: State<'_, DbState>) -> Result<(), String>` | Yes | `Test.ResetTestDatabase` |

## User Activity Audit Stream – `src-tauri\src\commands\user_activity_audit.rs` (1 commands)

- **Target Go Service:** `AuditService`
- **Security / Policy Note:** admin.user_activity

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L7 | `query_user_activity_logs` | `(limit: Option<i64>, offset: Option<i64>, caller_user_id: String, session: State<'_, DesktopSessionState>, db: State<'_, DbState>, manager: State<'_, ConnectionManager>,) -> Result<Vec<UserActivityAuditLog>, String>` | Yes | `AuditService.QueryUserActivityLogs` |

## Shared Utility Commands – `src-tauri\src\commands\utils.rs` (3 commands)

- **Target Go Service:** `AppLifecycleService`
- **Security / Policy Note:** Helper IPC utilities

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L20 | `generate_id` | `(prefix: Option<String>) -> String` | No (Unregistered) | `AppLifecycleService.GenerateId` |
| L30 | `get_current_user_info` | `() -> Result<serde_json::Value, String>` | Yes | `AppLifecycleService.GetCurrentUserInfo` |
| L46 | `get_user_context` | `() -> Result<serde_json::Value, String>` | Yes | `AppLifecycleService.GetUserContext` |

## Workflow Decision Rules, Cost & ROI – `src-tauri\src\commands\workflow_automation.rs` (37 commands)

- **Target Go Service:** `WorkflowService`
- **Security / Policy Note:** Requires admin/automationmanager (Next Gen uses server session)

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L2565 | `list_workflow_decision_rules` | `(caller_role: String, state: State<DbState>,) -> Result<Vec<WorkflowDecisionRuleRow>, String>` | Yes | `WorkflowService.ListWorkflowDecisionRules` |
| L2599 | `set_workflow_decision_rule_enabled` | `(rule_id: String, enabled: bool, caller_role: String, changed_by: String, state: State<DbState>,) -> Result<(), String>` | Yes | `WorkflowService.SetWorkflowDecisionRuleEnabled` |
| L2656 | `set_workflow_automation_master_enabled` | `(enabled: bool, caller_role: String, changed_by: String, state: State<DbState>,) -> Result<(), String>` | Yes | `WorkflowService.SetWorkflowAutomationMasterEnabled` |
| L2688 | `set_automation_guardrails` | `(input: AutomationGuardrailsInput, caller_role: String, state: State<DbState>,) -> Result<(), String>` | Yes | `WorkflowService.SetAutomationGuardrails` |
| L2726 | `set_adaptive_sla_apply_enabled` | `(enabled: bool, caller_role: String, changed_by: String, state: State<DbState>,) -> Result<(), String>` | Yes | `WorkflowService.SetAdaptiveSlaApplyEnabled` |
| L2749 | `apply_adaptive_sla_decision` | `(accept: bool, caller_role: String, changed_by: String, state: State<DbState>,) -> Result<i32, String>` | Yes | `WorkflowService.ApplyAdaptiveSlaDecision` |
| L2783 | `query_workflow_automation_log` | `(query: AutomationLogQuery, caller_role: String, state: State<DbState>,) -> Result<Vec<AutomationLogRow>, String>` | Yes | `WorkflowService.QueryWorkflowAutomationLog` |
| L2880 | `get_automation_health` | `(caller_role: String, state: State<DbState>,) -> Result<AutomationHealthSnapshot, String>` | Yes | `WorkflowService.GetAutomationHealth` |
| L2927 | `get_automation_impact_summary` | `(caller_role: String, state: State<DbState>,) -> Result<AutomationImpactSummary, String>` | Yes | `WorkflowService.GetAutomationImpactSummary` |
| L2960 | `list_adaptive_sla_adjustments` | `(limit: Option<i64>, caller_role: String, state: State<DbState>,) -> Result<Vec<AdaptiveSlaAdjustmentRow>, String>` | Yes | `WorkflowService.ListAdaptiveSlaAdjustments` |
| L2992 | `list_workflow_rule_change_log` | `(limit: Option<i64>, caller_role: String, state: State<DbState>,) -> Result<Vec<WorkflowRuleChangeRow>, String>` | Yes | `WorkflowService.ListWorkflowRuleChangeLog` |
| L3024 | `run_workflow_automation_cycle_command` | `(caller_role: String, state: State<DbState>,) -> Result<AutomationCycleReport, String>` | Yes | `WorkflowService.RunWorkflowAutomationCycle` |
| L3034 | `analyze_workflow_efficiency_command` | `(caller_role: String, state: State<DbState>,) -> Result<serde_json::Value, String>` | Yes | `WorkflowService.AnalyzeWorkflowEfficiency` |
| L3044 | `suggest_resolution_actions_command` | `(case_id: String, caller_role: String, state: State<DbState>,) -> Result<Vec<String>, String>` | Yes | `WorkflowService.SuggestResolutionActions` |
| L3128 | `rollback_automation_action` | `(original_automation_id: String, rollback_type: String, performed_by: String, caller_role: String, state: State<DbState>,) -> Result<String, String>` | Yes | `WorkflowService.RollbackAutomationAction` |
| L3256 | `list_rule_effectiveness_metrics` | `(days: Option<i64>, caller_role: String, state: State<DbState>,) -> Result<Vec<RuleEffectivenessMetricRow>, String>` | Yes | `WorkflowService.ListRuleEffectivenessMetrics` |
| L3294 | `get_rule_performance_dashboard` | `(caller_role: String, state: State<DbState>,) -> Result<serde_json::Value, String>` | Yes | `WorkflowService.GetRulePerformanceDashboard` |
| L3388 | `list_automation_decision_feedback` | `(limit: Option<i64>, caller_role: String, state: State<DbState>,) -> Result<Vec<AutomationDecisionFeedbackRow>, String>` | Yes | `WorkflowService.ListAutomationDecisionFeedback` |
| L3424 | `list_rule_safety_index` | `(days: Option<i64>, caller_role: String, state: State<DbState>,) -> Result<Vec<RuleSafetyIndexRow>, String>` | Yes | `WorkflowService.ListRuleSafetyIndex` |
| L3455 | `list_automation_stability_alerts` | `(include_acknowledged: Option<bool>, limit: Option<i64>, caller_role: String, state: State<DbState>,) -> Result<Vec<AutomationStabilityAlertRow>, String>` | Yes | `WorkflowService.ListAutomationStabilityAlerts` |
| L3490 | `acknowledge_automation_stability_alert` | `(alert_id: String, caller_role: String, state: State<DbState>,) -> Result<(), String>` | Yes | `WorkflowService.AcknowledgeAutomationStabilityAlert` |
| L3506 | `list_automation_benchmark_history` | `(limit: Option<i64>, caller_role: String, state: State<DbState>,) -> Result<Vec<AutomationBenchmarkRow>, String>` | Yes | `WorkflowService.ListAutomationBenchmarkHistory` |
| L3537 | `list_automation_roi_metrics` | `(limit: Option<i64>, caller_role: String, state: State<DbState>,) -> Result<Vec<AutomationRoiMetricRow>, String>` | Yes | `WorkflowService.ListAutomationRoiMetrics` |
| L3569 | `generate_rule_optimization_recommendations_command` | `(caller_role: String, state: State<DbState>,) -> Result<serde_json::Value, String>` | Yes | `WorkflowService.GenerateRuleOptimizationRecommendations` |
| L3579 | `generate_automation_learning_suggestions_command` | `(caller_role: String, state: State<DbState>,) -> Result<serde_json::Value, String>` | Yes | `WorkflowService.GenerateAutomationLearningSuggestions` |
| L3589 | `simulate_multiple_rule_sets_command` | `(caller_role: String, staged_version_id: Option<String>, tenant_id: Option<String>, environment_id: Option<String>, state: State<DbState>,) -> Result<serde_json::Value, String>` | Yes | `WorkflowService.SimulateMultipleRuleSets` |
| L3669 | `list_rule_execution_cost_estimates` | `(caller_role: String, state: State<DbState>,) -> Result<Vec<RuleExecutionCostEstimateRow>, String>` | Yes | `WorkflowService.ListRuleExecutionCostEstimates` |
| L3713 | `upsert_rule_execution_cost_estimate` | `(input: RuleExecutionCostEstimateInput, caller_role: String, state: State<DbState>,) -> Result<(), String>` | Yes | `WorkflowService.UpsertRuleExecutionCostEstimate` |
| L3750 | `get_automation_cost_limits` | `(caller_role: String, state: State<DbState>,) -> Result<AutomationCostLimitsRow, String>` | Yes | `WorkflowService.GetAutomationCostLimits` |
| L3782 | `set_automation_cost_limits` | `(input: AutomationCostLimitsInput, caller_role: String, state: State<DbState>,) -> Result<(), String>` | Yes | `WorkflowService.SetAutomationCostLimits` |
| L3827 | `list_rule_cost_efficiency_metrics` | `(days: Option<i64>, caller_role: String, state: State<DbState>,) -> Result<Vec<RuleCostEfficiencyMetricRow>, String>` | Yes | `WorkflowService.ListRuleCostEfficiencyMetrics` |
| L3862 | `list_automation_capacity_load` | `(limit: Option<i64>, caller_role: String, state: State<DbState>,) -> Result<Vec<AutomationCapacityLoadRow>, String>` | Yes | `WorkflowService.ListAutomationCapacityLoad` |
| L3899 | `list_daily_automation_economics_index` | `(limit: Option<i64>, caller_role: String, state: State<DbState>,) -> Result<Vec<DailyAutomationEconomicsRow>, String>` | Yes | `WorkflowService.ListDailyAutomationEconomicsIndex` |
| L3930 | `detect_inefficient_rules_command` | `(caller_role: String, state: State<DbState>,) -> Result<serde_json::Value, String>` | Yes | `WorkflowService.DetectInefficientRules` |
| L3940 | `generate_cost_optimization_suggestions_command` | `(caller_role: String, state: State<DbState>,) -> Result<serde_json::Value, String>` | Yes | `WorkflowService.GenerateCostOptimizationSuggestions` |
| L3950 | `predictive_capacity_forecast_command` | `(caller_role: String, state: State<DbState>,) -> Result<serde_json::Value, String>` | Yes | `WorkflowService.PredictiveCapacityForecast` |
| L3960 | `get_automation_cost_vs_benefit_dashboard` | `(caller_role: String, state: State<DbState>,) -> Result<serde_json::Value, String>` | Yes | `WorkflowService.GetAutomationCostVsBenefitDashboard` |

## Incident Correlation, Suppression & Forecasting – `src-tauri\src\commands\workflow_incident_management.rs` (13 commands)

- **Target Go Service:** `WorkflowService`
- **Security / Policy Note:** Root cause >=50 chars, burst detection, probability forecast

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L6234 | `submit_workflow_forecast_feedback_command` | `(forecast_id: String, feedback_kind: String, notes: Option<String>, caller_role: String, state: State<DbState>,) -> Result<(), String>` | Yes | `WorkflowService.SubmitWorkflowForecastFeedback` |
| L6252 | `acknowledge_workflow_forecast_actions_command` | `(forecast_id: String, action_taken: String, caller_role: String, state: State<DbState>,) -> Result<(), String>` | Yes | `WorkflowService.AcknowledgeWorkflowForecastActions` |
| L6268 | `get_operations_center_dashboard_command` | `(caller_role: String, state: State<DbState>,) -> Result<Value, String>` | Yes | `WorkflowService.GetOperationsCenterDashboard` |
| L6278 | `get_workflow_incident_detail_command` | `(incident_id: String, caller_role: String, state: State<DbState>,) -> Result<Value, String>` | Yes | `WorkflowService.GetWorkflowIncidentDetail` |
| L6289 | `get_correlated_incident_timeline_command` | `(incident_id: String, caller_role: String, state: State<DbState>,) -> Result<Value, String>` | Yes | `WorkflowService.GetCorrelatedIncidentTimeline` |
| L6300 | `scan_systemic_failure_bursts_command` | `(caller_role: String, state: State<DbState>,) -> Result<i64, String>` | Yes | `WorkflowService.ScanSystemicFailureBursts` |
| L6315 | `detect_stabilization_phase_command` | `(caller_role: String, state: State<DbState>,) -> Result<i64, String>` | Yes | `WorkflowService.DetectStabilizationPhase` |
| L6329 | `start_manual_incident_suppression_command` | `(source_module: String, event_type: String, window_minutes: i64, reason: String, incident_id: Option<String>, caller_role: String, state: State<DbState>,) -> Result<String, String>` | Yes | `WorkflowService.StartManualIncidentSuppression` |
| L6354 | `append_workflow_incident_resolution_note_command` | `(incident_id: String, notes: String, caller_role: String, state: State<DbState>,) -> Result<(), String>` | Yes | `WorkflowService.AppendWorkflowIncidentResolutionNote` |
| L6366 | `resolve_workflow_incident_command` | `(incident_id: String, root_cause_summary: String, caller_role: String, state: State<DbState>,) -> Result<(), String>` | Yes | `WorkflowService.ResolveWorkflowIncident` |
| L6378 | `export_workflow_incidents_report_csv_command` | `(caller_role: String, state: State<DbState>,) -> Result<String, String>` | Yes | `WorkflowService.ExportWorkflowIncidentsReportCsv` |
| L6388 | `refresh_workflow_incident_metrics_command` | `(caller_role: String, state: State<DbState>,) -> Result<(), String>` | Yes | `WorkflowService.RefreshWorkflowIncidentMetrics` |
| L6398 | `debug_trigger_failure_command` | `(mode: String, caller_role: String, state: State<DbState>,) -> Result<Value, String>` | Yes | `WorkflowService.DebugTriggerFailure` |

## Background Job Monitoring & Recovery Guard – `src-tauri\src\commands\workflow_job_monitoring.rs` (23 commands)

- **Target Go Service:** `Scheduler / WorkflowService`
- **Security / Policy Note:** 6 jobs, timeout 2x/2h, recovery guard disable

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L1650 | `list_workflow_background_jobs_command` | `(caller_role: String, state: State<DbState>,) -> Result<Vec<Value>, String>` | Yes | `Scheduler.ListWorkflowBackgroundJobs` |
| L1681 | `list_workflow_job_execution_log_command` | `(job_id: Option<String>, limit: Option<i64>, caller_role: String, state: State<DbState>,) -> Result<Vec<Value>, String>` | Yes | `Scheduler.ListWorkflowJobExecutionLog` |
| L1741 | `get_background_job_health_dashboard_command` | `(caller_role: String, state: State<DbState>,) -> Result<Value, String>` | Yes | `Scheduler.GetBackgroundJobHealthDashboard` |
| L1827 | `retry_failed_job_command` | `(execution_id: String, caller_role: String, state: State<DbState>,) -> Result<String, String>` | Yes | `Scheduler.RetryFailedJob` |
| L1838 | `simulate_background_jobs_command` | `(caller_role: String, state: State<DbState>,) -> Result<Value, String>` | Yes | `Scheduler.SimulateBackgroundJobs` |
| L1848 | `detect_job_failures_command` | `(caller_role: String, state: State<DbState>,) -> Result<i64, String>` | Yes | `Scheduler.DetectJobFailures` |
| L1858 | `list_workflow_job_alert_log_command` | `(limit: Option<i64>, caller_role: String, state: State<DbState>,) -> Result<Vec<Value>, String>` | Yes | `Scheduler.ListWorkflowJobAlertLog` |
| L1891 | `list_workflow_job_failure_alerts_command` | `(limit: Option<i64>, caller_role: String, state: State<DbState>,) -> Result<Vec<Value>, String>` | Yes | `Scheduler.ListWorkflowJobFailureAlerts` |
| L1922 | `detect_missed_job_runs_command` | `(caller_role: String, state: State<DbState>,) -> Result<i64, String>` | Yes | `Scheduler.DetectMissedJobRuns` |
| L1932 | `recover_missed_job_command` | `(job_id: String, alert_id: Option<String>, caller_role: String, state: State<DbState>,) -> Result<String, String>` | Yes | `Scheduler.RecoverMissedJob` |
| L1949 | `get_missed_schedule_dashboard_command` | `(caller_role: String, state: State<DbState>,) -> Result<Value, String>` | Yes | `Scheduler.GetMissedScheduleDashboard` |
| L2068 | `list_workflow_job_missed_alerts_command` | `(limit: Option<i64>, caller_role: String, state: State<DbState>,) -> Result<Vec<Value>, String>` | Yes | `Scheduler.ListWorkflowJobMissedAlerts` |
| L2100 | `set_workflow_background_job_enabled_command` | `(job_id: String, enabled: bool, caller_role: String, state: State<DbState>,) -> Result<(), String>` | Yes | `Scheduler.SetWorkflowBackgroundJobEnabled` |
| L2112 | `reset_job_schedule_anchor_command` | `(job_id: String, caller_role: String, state: State<DbState>,) -> Result<(), String>` | Yes | `Scheduler.ResetJobScheduleAnchor` |
| L2123 | `recovery_guard_override_reenable_command` | `(job_id: String, reason: String, caller_role: String, state: State<DbState>,) -> Result<(), String>` | Yes | `Scheduler.RecoveryGuardOverrideReenable` |
| L2135 | `update_job_schedule_expectations_command` | `(job_id: String, grace_period_minutes: Option<i64>, recovery_delay_sec: Option<i64>, max_recovery_attempts: Option<i64>, caller_role: String, state: State<DbState>,) -> Result<(), String>` | Yes | `Scheduler.UpdateJobScheduleExpectations` |
| L2156 | `retry_latest_failed_job_command` | `(job_id: String, caller_role: String, state: State<DbState>,) -> Result<String, String>` | Yes | `Scheduler.RetryLatestFailedJob` |
| L2167 | `get_job_failure_insights_command` | `(caller_role: String, state: State<DbState>,) -> Result<Value, String>` | Yes | `Scheduler.GetJobFailureInsights` |
| L2177 | `export_workflow_job_recovery_log_csv_command` | `(caller_role: String, state: State<DbState>,) -> Result<String, String>` | Yes | `Scheduler.ExportWorkflowJobRecoveryLogCsv` |
| L2187 | `get_workflow_job_dependencies_tree_command` | `(caller_role: String, state: State<DbState>,) -> Result<Value, String>` | Yes | `Scheduler.GetWorkflowJobDependenciesTree` |
| L2197 | `get_job_execution_timeline_command` | `(job_id: String, hours: Option<i64>, caller_role: String, state: State<DbState>,) -> Result<Value, String>` | Yes | `Scheduler.GetJobExecutionTimeline` |
| L2209 | `list_workflow_job_schedule_expectations_command` | `(caller_role: String, state: State<DbState>,) -> Result<Vec<Value>, String>` | Yes | `Scheduler.ListWorkflowJobScheduleExpectations` |
| L2241 | `list_workflow_job_manual_override_log_command` | `(limit: Option<i64>, caller_role: String, state: State<DbState>,) -> Result<Vec<Value>, String>` | Yes | `Scheduler.ListWorkflowJobManualOverrideLog` |

## Multi-Environment & Tenant Context – `src-tauri\src\commands\workflow_multienv.rs` (9 commands)

- **Target Go Service:** `WorkflowService`
- **Security / Policy Note:** DEV -> TEST -> PROD one-step promotion

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L168 | `list_workflow_environments` | `(caller_role: String, state: State<DbState>,) -> Result<Vec<WorkflowEnvironmentRow>, String>` | Yes | `WorkflowService.ListWorkflowEnvironments` |
| L196 | `list_workflow_tenants` | `(caller_role: String, state: State<DbState>,) -> Result<Vec<WorkflowTenantRow>, String>` | Yes | `WorkflowService.ListWorkflowTenants` |
| L222 | `get_workflow_execution_context` | `(caller_role: String, state: State<DbState>,) -> Result<Value, String>` | Yes | `WorkflowService.GetWorkflowExecutionContext` |
| L236 | `set_workflow_active_tenant` | `(tenant_id: String, caller_role: String, state: State<DbState>,) -> Result<(), String>` | Yes | `WorkflowService.SetWorkflowActiveTenant` |
| L259 | `set_workflow_execution_environment` | `(environment_id: String, caller_role: String, state: State<DbState>,) -> Result<(), String>` | Yes | `WorkflowService.SetWorkflowExecutionEnvironment` |
| L282 | `list_workflow_environment_deployment_log` | `(environment_id: Option<String>, tenant_id: Option<String>, limit: Option<i64>, caller_role: String, state: State<DbState>,) -> Result<Vec<WorkflowEnvironmentDeploymentLogRow>, String>` | Yes | `WorkflowService.ListWorkflowEnvironmentDeploymentLog` |
| L406 | `get_environment_health_dashboard` | `(caller_role: String, state: State<DbState>,) -> Result<Value, String>` | Yes | `WorkflowService.GetEnvironmentHealthDashboard` |
| L480 | `get_tenant_performance_dashboard` | `(tenant_id: Option<String>, caller_role: String, state: State<DbState>,) -> Result<Value, String>` | Yes | `WorkflowService.GetTenantPerformanceDashboard` |
| L610 | `promote_rule_version_command` | `(source_version_id: String, target_environment_id: String, promoted_by: String, caller_role: String, state: State<DbState>,) -> Result<String, String>` | Yes | `WorkflowService.PromoteRuleVersion` |

## Workflow Health, AI Consistency & Self-Heal – `src-tauri\src\commands\workflow_observability.rs` (10 commands)

- **Target Go Service:** `WorkflowService`
- **Security / Policy Note:** Runtime anomaly score, self-healing recovery

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L930 | `get_workflow_health_summary` | `(state: State<DbState>) -> Result<WorkflowHealthSummary, String>` | Yes | `WorkflowService.GetWorkflowHealthSummary` |
| L936 | `get_workflow_maintenance_history` | `(limit: Option<i64>, state: State<DbState>,) -> Result<Vec<WorkflowMaintenanceHistoryRow>, String>` | Yes | `WorkflowService.GetWorkflowMaintenanceHistory` |
| L945 | `run_recovery_readiness_check` | `(state: State<DbState>,) -> Result<RecoveryReadinessReport, String>` | Yes | `WorkflowService.RunRecoveryReadinessCheck` |
| L953 | `reconstruct_exception_lifecycle` | `(state: State<DbState>) -> Result<i64, String>` | Yes | `WorkflowService.ReconstructExceptionLifecycle` |
| L959 | `get_reliability_diagnostics` | `(state: State<DbState>,) -> Result<ReliabilityDiagnostics, String>` | Yes | `WorkflowService.GetReliabilityDiagnostics` |
| L967 | `get_predictive_workflow_risk` | `(state: State<DbState>) -> Result<PredictiveRisk, String>` | Yes | `WorkflowService.GetPredictiveWorkflowRisk` |
| L973 | `get_audit_verification_summary` | `(state: State<DbState>,) -> Result<AuditVerificationSummary, String>` | Yes | `WorkflowService.GetAuditVerificationSummary` |
| L981 | `run_ai_consistency_auditor` | `(state: State<DbState>,) -> Result<AiConsistencyAuditReport, String>` | Yes | `WorkflowService.RunAiConsistencyAuditor` |
| L989 | `get_runtime_anomaly_report` | `(state: State<DbState>) -> Result<RuntimeAnomalyReport, String>` | Yes | `WorkflowService.GetRuntimeAnomalyReport` |
| L995 | `run_self_healing_recovery_flow` | `(state: State<DbState>,) -> Result<SelfHealingRecoveryResult, String>` | Yes | `WorkflowService.RunSelfHealingRecoveryFlow` |

## Production Alerts, Metrics & Health – `src-tauri\src\commands\workflow_production_observability.rs` (6 commands)

- **Target Go Service:** `WorkflowService`
- **Security / Policy Note:** Alert suppression, reliability score

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L803 | `get_system_metrics_command` | `(caller_role: String, state: State<DbState>,) -> Result<Value, String>` | Yes | `WorkflowService.GetSystemMetrics` |
| L813 | `get_system_health_command` | `(caller_role: String, state: State<DbState>,) -> Result<Value, String>` | Yes | `WorkflowService.GetSystemHealth` |
| L823 | `list_workflow_alert_signal_log_command` | `(limit: Option<i64>, caller_role: String, state: State<DbState>,) -> Result<Vec<Value>, String>` | Yes | `WorkflowService.ListWorkflowAlertSignalLog` |
| L856 | `get_workflow_alert_signal_dashboard_command` | `(caller_role: String, state: State<DbState>,) -> Result<Value, String>` | Yes | `WorkflowService.GetWorkflowAlertSignalDashboard` |
| L949 | `simulate_alert_event_command` | `(scenario: String, caller_role: String, state: State<DbState>,) -> Result<Value, String>` | Yes | `WorkflowService.SimulateAlertEvent` |
| L959 | `export_metrics_snapshot_csv_command` | `(caller_role: String, state: State<DbState>,) -> Result<String, String>` | Yes | `WorkflowService.ExportMetricsSnapshotCsv` |

## Rule Versioning, Canary & Deployment Safety – `src-tauri\src\commands\workflow_rule_deployment.rs` (29 commands)

- **Target Go Service:** `WorkflowService`
- **Security / Policy Note:** Approval gate, freeze, canary FNV-1a, risk score

| Line | Legacy Command | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| L850 | `list_workflow_rule_versions` | `(rule_id: String, tenant_id: Option<String>, environment_id: Option<String>, caller_role: String, state: State<DbState>,) -> Result<Vec<WorkflowRuleVersionRow>, String>` | Yes | `WorkflowService.ListWorkflowRuleVersions` |
| L933 | `create_workflow_rule_version` | `(input: CreateRuleVersionInput, caller_role: String, state: State<DbState>,) -> Result<String, String>` | Yes | `WorkflowService.CreateWorkflowRuleVersion` |
| L986 | `compare_rule_versions_command` | `(version_id_a: String, version_id_b: String, caller_role: String, state: State<DbState>,) -> Result<Value, String>` | Yes | `WorkflowService.CompareRuleVersions` |
| L998 | `create_workflow_rule_staging` | `(rule_id: String, version_id: String, staging_environment: Option<String>, caller_role: String, state: State<DbState>,) -> Result<String, String>` | Yes | `WorkflowService.CreateWorkflowRuleStaging` |
| L1023 | `update_workflow_rule_staging_status` | `(staging_id: String, status: String, caller_role: String, state: State<DbState>,) -> Result<(), String>` | Yes | `WorkflowService.UpdateWorkflowRuleStagingStatus` |
| L1047 | `list_workflow_rule_staging` | `(rule_id: Option<String>, caller_role: String, state: State<DbState>,) -> Result<Vec<WorkflowRuleStagingRow>, String>` | Yes | `WorkflowService.ListWorkflowRuleStaging` |
| L1105 | `submit_rule_version_approval` | `(rule_id: String, version_id: String, requested_by: String, caller_role: String, state: State<DbState>,) -> Result<String, String>` | Yes | `WorkflowService.SubmitRuleVersionApproval` |
| L1126 | `record_rule_approval_decision` | `(approval_id: String, approve: bool, approved_by: String, caller_role: String, state: State<DbState>,) -> Result<(), String>` | Yes | `WorkflowService.RecordRuleApprovalDecision` |
| L1146 | `list_workflow_rule_approvals` | `(rule_id: Option<String>, caller_role: String, state: State<DbState>,) -> Result<Vec<WorkflowRuleApprovalRow>, String>` | Yes | `WorkflowService.ListWorkflowRuleApprovals` |
| L1208 | `deploy_rule_version_command` | `(rule_id: String, version_id: String, deployed_by: String, caller_role: String, safety_override_acknowledged: Option<bool>, state: State<DbState>,) -> Result<(), String>` | Yes | `WorkflowService.DeployRuleVersion` |
| L1232 | `rollback_rule_version_command` | `(rule_id: String, target_version_id: String, performed_by: String, environment_id: Option<String>, caller_role: String, state: State<DbState>,) -> Result<(), String>` | Yes | `WorkflowService.RollbackRuleVersion` |
| L1253 | `validate_rule_deployment_command` | `(rule_id: String, version_id: String, caller_role: String, state: State<DbState>,) -> Result<Value, String>` | Yes | `WorkflowService.ValidateRuleDeployment` |
| L1273 | `list_workflow_rule_deployment_log` | `(rule_id: Option<String>, limit: Option<i64>, caller_role: String, state: State<DbState>,) -> Result<Vec<WorkflowRuleDeploymentLogRow>, String>` | Yes | `WorkflowService.ListWorkflowRuleDeploymentLog` |
| L1337 | `set_canary_rule_deployment` | `(rule_id: String, version_id: String, sample_size_percentage: f64, caller_role: String, state: State<DbState>,) -> Result<(), String>` | Yes | `WorkflowService.SetCanaryRuleDeployment` |
| L1365 | `clear_canary_rule_deployment` | `(rule_id: String, caller_role: String, state: State<DbState>,) -> Result<(), String>` | Yes | `WorkflowService.ClearCanaryRuleDeployment` |
| L1382 | `list_canary_rule_deployments` | `(caller_role: String, state: State<DbState>,) -> Result<Vec<CanaryRuleDeploymentRow>, String>` | Yes | `WorkflowService.ListCanaryRuleDeployments` |
| L1412 | `set_deployment_freeze` | `(frozen: bool, caller_role: String, state: State<DbState>,) -> Result<(), String>` | Yes | `WorkflowService.SetDeploymentFreeze` |
| L1428 | `get_deployment_freeze_status` | `(caller_role: String, state: State<DbState>,) -> Result<Value, String>` | Yes | `WorkflowService.GetDeploymentFreezeStatus` |
| L1443 | `set_deployment_requires_approval` | `(required: bool, caller_role: String, state: State<DbState>,) -> Result<(), String>` | Yes | `WorkflowService.SetDeploymentRequiresApproval` |
| L1459 | `list_rule_deployment_impact_metrics` | `(rule_id: Option<String>, limit: Option<i64>, caller_role: String, state: State<DbState>,) -> Result<Vec<RuleDeploymentImpactRow>, String>` | Yes | `WorkflowService.ListRuleDeploymentImpactMetrics` |
| L1525 | `refresh_rule_deployment_impact_metrics` | `(rule_id: String, caller_role: String, state: State<DbState>,) -> Result<(), String>` | Yes | `WorkflowService.RefreshRuleDeploymentImpactMetrics` |
| L1607 | `validate_deployment_safety_command` | `(rule_id: String, version_id: String, caller_role: String, state: State<DbState>,) -> Result<Value, String>` | Yes | `WorkflowService.ValidateDeploymentSafety` |
| L1659 | `run_deployment_dry_run_command` | `(rule_id: String, version_id: String, caller_role: String, state: State<DbState>,) -> Result<Value, String>` | Yes | `WorkflowService.RunDeploymentDryRun` |
| L1698 | `get_deployment_safety_dashboard_command` | `(caller_role: String, state: State<DbState>,) -> Result<Value, String>` | Yes | `WorkflowService.GetDeploymentSafetyDashboard` |
| L1746 | `get_smart_deployment_recommendations_command` | `(caller_role: String, state: State<DbState>,) -> Result<Value, String>` | Yes | `WorkflowService.GetSmartDeploymentRecommendations` |
| L1789 | `generate_deployment_safety_audit_report_command` | `(rule_id: String, version_id: String, caller_role: String, state: State<DbState>,) -> Result<Value, String>` | Yes | `WorkflowService.GenerateDeploymentSafetyAuditReport` |
| L1867 | `list_deployment_conflict_log_command` | `(limit: Option<i64>, caller_role: String, state: State<DbState>,) -> Result<Vec<Value>, String>` | Yes | `WorkflowService.ListDeploymentConflictLog` |
| L1901 | `list_deployment_risk_timeline_command` | `(limit: Option<i64>, caller_role: String, state: State<DbState>,) -> Result<Vec<Value>, String>` | Yes | `WorkflowService.ListDeploymentRiskTimeline` |
| L1937 | `set_deployment_prod_safety_enforcement_command` | `(enabled: bool, caller_role: String, state: State<DbState>,) -> Result<(), String>` | Yes | `WorkflowService.SetDeploymentProdSafetyEnforcement` |

## Lookup Options Macro Commands – `src-tauri\src\commands\options.rs` (26 commands)

| Command | Origin | Signature | Registered | Next-Gen Method Target |
| :--- | :--- | :--- | :---: | :--- |
| `get_units` | `options.rs (macro)` | `() -> Result<Vec<SelectOption>, String>` | Yes | `OptionService.ListUnits()` |
| `add_unit` | `options.rs (macro)` | `(option: SelectOption) -> Result<(), String>` | Yes | `OptionService.AddUnit()` |
| `get_currencies` | `options.rs (macro)` | `() -> Result<Vec<SelectOption>, String>` | Yes | `OptionService.ListCurrencies()` |
| `add_currency` | `options.rs (macro)` | `(option: SelectOption) -> Result<(), String>` | Yes | `OptionService.AddCurrency()` |
| `get_countries` | `options.rs (macro)` | `() -> Result<Vec<SelectOption>, String>` | Yes | `OptionService.ListCountries()` |
| `add_country` | `options.rs (macro)` | `(option: SelectOption) -> Result<(), String>` | Yes | `OptionService.AddCountry()` |
| `get_bcd_rates` | `options.rs (macro)` | `() -> Result<Vec<SelectOption>, String>` | Yes | `OptionService.ListBCDRates()` |
| `add_bcd_rate` | `options.rs (macro)` | `(option: SelectOption) -> Result<(), String>` | Yes | `OptionService.AddBCDRate()` |
| `get_sws_rates` | `options.rs (macro)` | `() -> Result<Vec<SelectOption>, String>` | Yes | `OptionService.ListSWSRates()` |
| `add_sws_rate` | `options.rs (macro)` | `(option: SelectOption) -> Result<(), String>` | Yes | `OptionService.AddSWSRate()` |
| `get_igst_rates` | `options.rs (macro)` | `() -> Result<Vec<SelectOption>, String>` | Yes | `OptionService.ListIGSTRates()` |
| `add_igst_rate` | `options.rs (macro)` | `(option: SelectOption) -> Result<(), String>` | Yes | `OptionService.AddIGSTRate()` |
| `get_categories` | `options.rs (macro)` | `() -> Result<Vec<SelectOption>, String>` | Yes | `OptionService.ListCategories()` |
| `add_category` | `options.rs (macro)` | `(option: SelectOption) -> Result<(), String>` | Yes | `OptionService.AddCategory()` |
| `get_end_uses` | `options.rs (macro)` | `() -> Result<Vec<SelectOption>, String>` | Yes | `OptionService.ListEndUses()` |
| `add_end_use` | `options.rs (macro)` | `(option: SelectOption) -> Result<(), String>` | Yes | `OptionService.AddEndUse()` |
| `get_purchase_uoms` | `options.rs (macro)` | `() -> Result<Vec<SelectOption>, String>` | Yes | `OptionService.ListPurchaseUOMs()` |
| `add_purchase_uom` | `options.rs (macro)` | `(option: SelectOption) -> Result<(), String>` | Yes | `OptionService.AddPurchaseUOM()` |
| `get_incoterms` | `options.rs (macro)` | `() -> Result<Vec<SelectOption>, String>` | Yes | `OptionService.ListIncoterms()` |
| `add_incoterm` | `options.rs (macro)` | `(option: SelectOption) -> Result<(), String>` | Yes | `OptionService.AddIncoterm()` |
| `get_shipment_modes` | `options.rs (macro)` | `() -> Result<Vec<SelectOption>, String>` | Yes | `OptionService.ListShipmentModes()` |
| `add_shipment_mode` | `options.rs (macro)` | `(option: SelectOption) -> Result<(), String>` | Yes | `OptionService.AddShipmentMode()` |
| `get_shipment_types` | `options.rs (macro)` | `() -> Result<Vec<SelectOption>, String>` | Yes | `OptionService.ListShipmentTypes()` |
| `add_shipment_type` | `options.rs (macro)` | `(option: SelectOption) -> Result<(), String>` | Yes | `OptionService.AddShipmentType()` |
| `get_shipment_statuses` | `options.rs (macro)` | `() -> Result<Vec<SelectOption>, String>` | Yes | `OptionService.ListShipmentStatuses()` |
| `add_shipment_status` | `options.rs (macro)` | `(option: SelectOption) -> Result<(), String>` | Yes | `OptionService.AddShipmentStatus()` |
