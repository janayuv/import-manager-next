-- === V1__initial_schema.sql ===
-- V1__initial_schema.sql
-- Initial database schema migration

-- Suppliers table
CREATE TABLE IF NOT EXISTS suppliers (
    id TEXT PRIMARY KEY,
    supplier_name TEXT NOT NULL,
    short_name TEXT,
    country TEXT NOT NULL,
    email TEXT NOT NULL,
    phone TEXT,
    beneficiary_name TEXT,
    bank_name TEXT,
    branch TEXT,
    bank_address TEXT,
    account_no TEXT,
    iban TEXT,
    swift_code TEXT,
    is_active BOOLEAN NOT NULL
);

-- Shipments table
CREATE TABLE IF NOT EXISTS shipments (
    id TEXT PRIMARY KEY, 
    supplier_id TEXT NOT NULL, 
    invoice_number TEXT NOT NULL,
    invoice_date TEXT NOT NULL, 
    goods_category TEXT NOT NULL, 
    invoice_value REAL NOT NULL,
    invoice_currency TEXT NOT NULL, 
    incoterm TEXT NOT NULL, 
    shipment_mode TEXT,
    shipment_type TEXT, 
    bl_awb_number TEXT, 
    bl_awb_date TEXT, 
    vessel_name TEXT,
    container_number TEXT, 
    gross_weight_kg REAL, 
    etd TEXT, 
    eta TEXT,
    status TEXT, 
    date_of_delivery TEXT, 
    is_frozen BOOLEAN NOT NULL DEFAULT 0,
    FOREIGN KEY (supplier_id) REFERENCES suppliers (id)
);

-- Items table
CREATE TABLE IF NOT EXISTS items (
    id TEXT PRIMARY KEY,
    part_number TEXT NOT NULL UNIQUE,
    item_description TEXT NOT NULL,
    unit TEXT NOT NULL,
    currency TEXT NOT NULL,
    unit_price REAL NOT NULL,
    hsn_code TEXT NOT NULL,
    supplier_id TEXT,
    is_active BOOLEAN NOT NULL,
    country_of_origin TEXT,
    bcd TEXT,
    sws TEXT,
    igst TEXT,
    technical_write_up TEXT,
    category TEXT,
    end_use TEXT,
    net_weight_kg REAL,
    purchase_uom TEXT,
    gross_weight_per_uom_kg REAL,
    photo_path TEXT,
    FOREIGN KEY (supplier_id) REFERENCES suppliers (id)
);

-- Invoices table
CREATE TABLE IF NOT EXISTS invoices (
    id TEXT PRIMARY KEY NOT NULL,
    shipment_id TEXT NOT NULL,
    status TEXT NOT NULL,
    FOREIGN KEY (shipment_id) REFERENCES shipments(id) ON DELETE CASCADE
);

-- Invoice line items table
CREATE TABLE IF NOT EXISTS invoice_line_items (
    id TEXT PRIMARY KEY NOT NULL,
    invoice_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    quantity REAL NOT NULL,
    unit_price REAL NOT NULL,
    FOREIGN KEY (invoice_id) REFERENCES invoices(id) ON DELETE CASCADE,
    FOREIGN KEY (item_id) REFERENCES items(id)
);

-- BOE Details table
CREATE TABLE IF NOT EXISTS boe_details (
    id TEXT PRIMARY KEY NOT NULL,
    be_number TEXT NOT NULL,
    be_date TEXT NOT NULL,
    location TEXT NOT NULL,
    total_assessment_value REAL NOT NULL,
    duty_amount REAL NOT NULL,
    payment_date TEXT,
    duty_paid REAL,
    challan_number TEXT,
    ref_id TEXT,
    transaction_id TEXT,
    UNIQUE(be_number, be_date)
);

-- BOE Calculations table
CREATE TABLE IF NOT EXISTS boe_calculations (
    id TEXT PRIMARY KEY NOT NULL,
    shipment_id TEXT NOT NULL,
    boe_id TEXT,
    supplier_name TEXT NOT NULL,
    invoice_number TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'Awaiting BOE Data',
    form_values_json TEXT NOT NULL,
    item_inputs_json TEXT NOT NULL,
    calculation_result_json TEXT NOT NULL,
    attachments_json TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    FOREIGN KEY (shipment_id) REFERENCES shipments(id) ON DELETE CASCADE
);

-- Service Providers table
CREATE TABLE IF NOT EXISTS service_providers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    gstin TEXT UNIQUE,
    state TEXT,
    contact_person TEXT,
    contact_email TEXT,
    contact_phone TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Expense Types table
CREATE TABLE IF NOT EXISTS expense_types (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    default_cgst_rate DECIMAL(5, 2) DEFAULT 0.00,
    default_sgst_rate DECIMAL(5, 2) DEFAULT 0.00,
    default_igst_rate DECIMAL(5, 2) DEFAULT 0.00,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Expense Invoices table
CREATE TABLE IF NOT EXISTS expense_invoices (
    id TEXT PRIMARY KEY,
    shipment_id TEXT NOT NULL,
    service_provider_id TEXT NOT NULL,
    invoice_no TEXT NOT NULL,
    invoice_date DATE NOT NULL,
    total_amount DECIMAL(12, 2) NOT NULL,
    total_cgst_amount DECIMAL(12, 2) NOT NULL DEFAULT 0.00,
    total_sgst_amount DECIMAL(12, 2) NOT NULL DEFAULT 0.00,
    total_igst_amount DECIMAL(12, 2) NOT NULL DEFAULT 0.00,
    remarks TEXT,
    created_by TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (shipment_id) REFERENCES shipments(id),
    FOREIGN KEY (service_provider_id) REFERENCES service_providers(id),
    UNIQUE(service_provider_id, invoice_no)
);

-- Expenses table
CREATE TABLE IF NOT EXISTS expenses (
    id TEXT PRIMARY KEY,
    expense_invoice_id TEXT NOT NULL,
    shipment_id TEXT NOT NULL,
    service_provider_id TEXT NOT NULL,
    invoice_no TEXT NOT NULL,
    invoice_date DATE NOT NULL,
    expense_type_id TEXT NOT NULL,
    amount DECIMAL(12, 2) NOT NULL,
    cgst_rate DECIMAL(5, 2) DEFAULT 0.00,
    sgst_rate DECIMAL(5, 2) DEFAULT 0.00,
    igst_rate DECIMAL(5, 2) DEFAULT 0.00,
    tds_rate DECIMAL(5, 2) DEFAULT 0.00,
    cgst_amount DECIMAL(12, 2) GENERATED ALWAYS AS (amount * cgst_rate / 100) STORED,
    sgst_amount DECIMAL(12, 2) GENERATED ALWAYS AS (amount * sgst_rate / 100) STORED,
    igst_amount DECIMAL(12, 2) GENERATED ALWAYS AS (amount * igst_rate / 100) STORED,
    tds_amount DECIMAL(12, 2) GENERATED ALWAYS AS (amount * tds_rate / 100) STORED,
    total_amount DECIMAL(12, 2) GENERATED ALWAYS AS (amount + (amount * (cgst_rate + sgst_rate + igst_rate) / 100)) STORED,
    remarks TEXT,
    created_by TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (expense_invoice_id) REFERENCES expense_invoices(id),
    FOREIGN KEY (shipment_id) REFERENCES shipments(id),
    FOREIGN KEY (service_provider_id) REFERENCES service_providers(id),
    FOREIGN KEY (expense_type_id) REFERENCES expense_types(id),
    UNIQUE(expense_invoice_id, expense_type_id)
);

-- Expense Attachments table
CREATE TABLE IF NOT EXISTS expense_attachments (
    id TEXT PRIMARY KEY,
    expense_id TEXT NOT NULL,
    file_name TEXT NOT NULL,
    file_path TEXT NOT NULL,
    file_type TEXT,
    uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    uploaded_by TEXT,
    FOREIGN KEY (expense_id) REFERENCES expenses(id)
);

-- Create option tables for dropdowns
CREATE TABLE IF NOT EXISTS units (value TEXT PRIMARY KEY NOT NULL, label TEXT NOT NULL UNIQUE);
CREATE TABLE IF NOT EXISTS currencies (value TEXT PRIMARY KEY NOT NULL, label TEXT NOT NULL UNIQUE);
CREATE TABLE IF NOT EXISTS countries (value TEXT PRIMARY KEY NOT NULL, label TEXT NOT NULL UNIQUE);
CREATE TABLE IF NOT EXISTS bcd_rates (value TEXT PRIMARY KEY NOT NULL, label TEXT NOT NULL UNIQUE);
CREATE TABLE IF NOT EXISTS sws_rates (value TEXT PRIMARY KEY NOT NULL, label TEXT NOT NULL UNIQUE);
CREATE TABLE IF NOT EXISTS igst_rates (value TEXT PRIMARY KEY NOT NULL, label TEXT NOT NULL UNIQUE);
CREATE TABLE IF NOT EXISTS categories (value TEXT PRIMARY KEY NOT NULL, label TEXT NOT NULL UNIQUE);
CREATE TABLE IF NOT EXISTS end_uses (value TEXT PRIMARY KEY NOT NULL, label TEXT NOT NULL UNIQUE);
CREATE TABLE IF NOT EXISTS purchase_uoms (value TEXT PRIMARY KEY NOT NULL, label TEXT NOT NULL UNIQUE);
CREATE TABLE IF NOT EXISTS incoterms (value TEXT PRIMARY KEY NOT NULL, label TEXT NOT NULL UNIQUE);
CREATE TABLE IF NOT EXISTS shipment_modes (value TEXT PRIMARY KEY NOT NULL, label TEXT NOT NULL UNIQUE);
CREATE TABLE IF NOT EXISTS shipment_types (value TEXT PRIMARY KEY NOT NULL, label TEXT NOT NULL UNIQUE);
CREATE TABLE IF NOT EXISTS shipment_statuses (value TEXT PRIMARY KEY NOT NULL, label TEXT NOT NULL UNIQUE);

-- Create report view
CREATE VIEW IF NOT EXISTS report_view AS
WITH 
boe_items AS (
    SELECT
        bc.id AS boe_calc_id,
        bc.shipment_id,
        bc.supplier_name,
        bc.invoice_number,
        json_extract(item.value, '$.partNo') AS part_no,
        json_extract(item.value, '$.description') AS boe_description,
        CAST(json_extract(item.value, '$.assessableValue') AS REAL) AS boe_assessable_value,
        CAST(json_extract(item.value, '$.bcdValue') AS REAL) AS boe_bcd_amount,
        CAST(json_extract(item.value, '$.swsValue') AS REAL) AS boe_sws_amount,
        CAST(json_extract(item.value, '$.igstValue') AS REAL) AS boe_igst_amount
    FROM boe_calculations bc
    JOIN json_each(json_extract(bc.calculation_result_json, '$.calculatedItems')) AS item
),
shipment_expenses AS (
    SELECT ei.shipment_id, 
           SUM(e.amount) AS shipment_expenses_basic,
           SUM(e.total_amount) AS shipment_expenses_total
    FROM expense_invoices ei
    JOIN expenses e ON e.expense_invoice_id = ei.id
    GROUP BY ei.shipment_id
),
boe_assessable AS (
    SELECT shipment_id, SUM(boe_assessable_value) AS shipment_boe_assessable_total
    FROM boe_items
    GROUP BY shipment_id
)
SELECT 
    sup.supplier_name AS supplier,
    s.supplier_id AS supplier_id,
    s.invoice_number AS invoice_no,
    s.invoice_date AS invoice_date,
    bi.part_no AS part_no,
    COALESCE(i.item_description, bi.boe_description) AS description,
    i.unit AS unit,
    ili.quantity AS qty,
    ili.unit_price AS unit_price,
    bi.boe_assessable_value AS assessable_value,
    bi.boe_bcd_amount AS bcd_amount,
    bi.boe_sws_amount AS sws_amount,
    bi.boe_igst_amount AS igst_amount,
    COALESCE(se.shipment_expenses_basic, 0.0) * 
      (bi.boe_assessable_value / NULLIF(ba.shipment_boe_assessable_total, 0)) AS expenses_total,
    (
      (bi.boe_assessable_value + bi.boe_bcd_amount + bi.boe_sws_amount
       + (COALESCE(se.shipment_expenses_basic, 0.0) * (bi.boe_assessable_value / NULLIF(ba.shipment_boe_assessable_total, 0))))
    ) / NULLIF(ili.quantity, 0) AS ldc_per_qty
FROM boe_items bi
JOIN shipments s ON s.id = bi.shipment_id
JOIN suppliers sup ON sup.id = s.supplier_id
JOIN invoices inv ON inv.shipment_id = s.id
JOIN items i ON i.part_number = bi.part_no
JOIN invoice_line_items ili ON ili.invoice_id = inv.id AND ili.item_id = i.id
LEFT JOIN shipment_expenses se ON se.shipment_id = s.id
LEFT JOIN boe_assessable ba ON ba.shipment_id = s.id;

-- === V2__notifications.sql ===
-- V2__notifications.sql
-- Migration for notifications system
-- This migration adds the notifications table for the notification system

CREATE TABLE IF NOT EXISTS notifications (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    message TEXT NOT NULL,
    type TEXT NOT NULL,
    timestamp TEXT NOT NULL,
    read BOOLEAN NOT NULL DEFAULT FALSE,
    category TEXT,
    action_url TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- === V3__notifications.sql ===


-- === V4__db_management.sql ===
-- V4__db_management.sql
-- Migration for database management system
-- This migration adds audit logging, backup management, and scheduling tables

-- Audit logs table for tracking all data changes
CREATE TABLE IF NOT EXISTS audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    table_name TEXT NOT NULL,
    row_id TEXT,
    action TEXT NOT NULL, -- 'update'|'delete'|'insert'|'restore'|'backup'
    user_id TEXT,
    before_json TEXT,
    after_json TEXT,
    metadata TEXT, -- JSON string for additional context
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Backups table for tracking backup operations
CREATE TABLE IF NOT EXISTS backups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    filename TEXT NOT NULL,
    path TEXT NOT NULL,
    destination TEXT, -- 'local'|'s3'|'gdrive'
    size_bytes INTEGER,
    sha256 TEXT,
    created_by TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    retention_until DATETIME,
    notes TEXT,
    status TEXT DEFAULT 'completed', -- 'completed'|'failed'|'in_progress'
    error_message TEXT
);

-- Backup schedules table for managing scheduled backups
CREATE TABLE IF NOT EXISTS backup_schedules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    cron_expr TEXT, -- cron expression or humanized recurrence
    time_zone TEXT DEFAULT 'UTC',
    destination TEXT NOT NULL DEFAULT 'local',
    retention_count INTEGER DEFAULT 5, -- number of backups to keep
    retention_days INTEGER DEFAULT 30, -- days to keep backups
    enabled INTEGER DEFAULT 1,
    last_run DATETIME,
    next_run DATETIME,
    created_by TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    notes TEXT
);

-- User roles table for permission management
CREATE TABLE IF NOT EXISTS user_roles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL UNIQUE,
    role TEXT NOT NULL, -- 'admin'|'db_manager'|'user'|'viewer'
    permissions TEXT, -- JSON string of specific permissions
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Soft-delete columns and idx_*_deleted_at indexes are created in Rust after all refinery
-- migrations complete (see `post_refinery_migrations` in src/migrations.rs).

-- Create indexes for better performance
CREATE INDEX IF NOT EXISTS idx_audit_logs_table_name ON audit_logs(table_name);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);

CREATE INDEX IF NOT EXISTS idx_backups_created_at ON backups(created_at);
CREATE INDEX IF NOT EXISTS idx_backups_destination ON backups(destination);
CREATE INDEX IF NOT EXISTS idx_backups_status ON backups(status);

CREATE INDEX IF NOT EXISTS idx_backup_schedules_enabled ON backup_schedules(enabled);
CREATE INDEX IF NOT EXISTS idx_backup_schedules_next_run ON backup_schedules(next_run);

CREATE INDEX IF NOT EXISTS idx_user_roles_user_id ON user_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_role ON user_roles(role);

-- === V5__invoice_line_item_tax_rates.sql ===
-- Snapshot duty / SWS / IGST rates on each invoice line (see app usage).
-- Columns are added in Rust after all refinery migrations (`post_refinery_migrations` in src/migrations.rs).
SELECT 1;

-- === V6__audit_logs_tableName.sql ===
-- V6: IPC-aligned `tableName` on audit_logs (nullable); backfill from `table_name`.
-- Applied in Rust after all refinery migrations (`crate::db::ensure_audit_logs_table_name_column`).
SELECT 1;

-- === V7__app_metadata.sql ===
-- Application key/value metadata (e.g. logical DB schema version for migrations and tooling)
CREATE TABLE IF NOT EXISTS app_metadata (
  key TEXT PRIMARY KEY NOT NULL,
  value TEXT NOT NULL
);

INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('db_version', '1');

-- === V8__app_metadata_counts.sql ===
-- Cumulative backup/restore counters (observability; incremented by the app on success)
INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('backup_count', '0');
INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('restore_count', '0');

-- === V9__app_metadata_last_ops.sql ===
-- Last successful backup/restore instants (RFC 3339 UTC, set by the app on success)
INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('last_backup_time', '');
INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('last_restore_time', '');

-- === V10__add_missing_fk_indexes.sql ===
-- Covering indexes for foreign key columns (improves FK checks, scans, and deletes)
CREATE INDEX IF NOT EXISTS idx_boe_calculations_shipment_id
  ON boe_calculations(shipment_id);

CREATE INDEX IF NOT EXISTS idx_expense_attachments_expense_id
  ON expense_attachments(expense_id);

CREATE INDEX IF NOT EXISTS idx_expense_invoices_shipment_id
  ON expense_invoices(shipment_id);

CREATE INDEX IF NOT EXISTS idx_expenses_expense_type_id
  ON expenses(expense_type_id);

CREATE INDEX IF NOT EXISTS idx_expenses_service_provider_id
  ON expenses(service_provider_id);

CREATE INDEX IF NOT EXISTS idx_expenses_shipment_id
  ON expenses(shipment_id);

CREATE INDEX IF NOT EXISTS idx_invoice_line_items_item_id
  ON invoice_line_items(item_id);

CREATE INDEX IF NOT EXISTS idx_invoice_line_items_invoice_id
  ON invoice_line_items(invoice_id);

CREATE INDEX IF NOT EXISTS idx_invoices_shipment_id
  ON invoices(shipment_id);

CREATE INDEX IF NOT EXISTS idx_items_supplier_id
  ON items(supplier_id);

CREATE INDEX IF NOT EXISTS idx_shipments_supplier_id
  ON shipments(supplier_id);

-- === V11__app_metadata_recycle_retention.sql ===
-- Default retention for auto-purge of soft-deleted records (permanent delete after N days)
INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('recycle_retention_days', '30');

-- === V12__app_metadata_last_database_maintenance.sql ===
-- Last time ANALYZE/VACUUM maintenance ran (ISO-8601); empty until first run
INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('last_database_maintenance', '');

-- === V13__kpi_governance.sql ===
-- KPI governance: definitions registry and daily snapshot history for audit / trends.

CREATE TABLE IF NOT EXISTS kpi_metadata (
    kpi_name TEXT PRIMARY KEY NOT NULL,
    formula TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    unit TEXT NOT NULL DEFAULT 'count',
    last_updated TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

CREATE TABLE IF NOT EXISTS kpi_daily_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    snapshot_date TEXT NOT NULL,
    kpi_name TEXT NOT NULL,
    value REAL NOT NULL,
    dimensions_json TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    UNIQUE (snapshot_date, kpi_name)
);

CREATE INDEX IF NOT EXISTS idx_kpi_daily_snapshots_date
    ON kpi_daily_snapshots (snapshot_date DESC);

CREATE INDEX IF NOT EXISTS idx_kpi_daily_snapshots_name
    ON kpi_daily_snapshots (kpi_name);

-- Seed definitions (governance / tooltips). Adjust formulas in DB as finance approves.
INSERT OR IGNORE INTO kpi_metadata (kpi_name, formula, description, unit, last_updated) VALUES
('total_shipments', 'COUNT(*) FROM shipments WHERE filters apply', 'Count of shipment rows in scope', 'count', datetime('now')),
('total_suppliers', 'COUNT(*) FROM suppliers', 'Active supplier directory size', 'count', datetime('now')),
('total_items', 'COUNT(*) FROM items', 'Item master row count', 'count', datetime('now')),
('pending_shipments', 'COUNT(*) WHERE status = ''docu-received''', 'Shipments in documentation received state', 'count', datetime('now')),
('delivered_shipments', 'COUNT(*) WHERE status = ''delivered''', 'Completed deliveries', 'count', datetime('now')),
('reconciled_boes', 'COUNT(*) FROM boe_calculations WHERE status = ''Reconciled''', 'BOE calculations marked reconciled', 'count', datetime('now')),
('total_invoice_value', 'SUM(invoice_value) FROM shipments in scope', 'Sum of shipment invoice values (mixed currency if not filtered)', 'mixed_currency', datetime('now')),
('avg_transit_days', 'AVG(julianday(delivery) - julianday(etd)) where ISO yyyy-MM-dd dates', 'Average calendar days from ETD to delivery when parseable', 'days', datetime('now')),
('expense_total', 'SUM(expenses.total_amount) for shipments in scope', 'Total expense lines including generated tax columns', 'INR', datetime('now')),
('duty_total', 'SUM(boe_details.duty_amount)', 'Sum of duty amounts on BOE detail records', 'INR', datetime('now')),
('total_duty_savings_estimate', 'Derived vs simplified duty model (see app logic)', 'Estimated savings vs reference duty rate; finance must validate', 'INR', datetime('now')),
('landed_cost_total', 'invoice_value + duty_total + expense_total (naive)', 'Naive landed cost sum for overview; not substitute for formal allocation', 'INR', datetime('now'));

-- === V14__kpi_duty_metadata_update.sql ===
-- Align KPI registry with dashboard metrics implementation (duty from report_view in scope).
UPDATE kpi_metadata
SET
    formula = 'SUM(report_view.bcd_amount + sws_amount + igst_amount) JOIN shipments filter',
    description = 'Sum of BCD, SWS, and IGST from consolidated report_view rows, scoped like shipment filters',
    last_updated = datetime('now')
WHERE kpi_name = 'duty_total';

-- === V15__dashboard_erp_cache_alerts.sql ===
-- ERP-style dashboard: metrics cache, KPI alerts, exception trends, activity audit, permissions, compliance materialization.

CREATE TABLE IF NOT EXISTS dashboard_metrics_cache (
    cache_key TEXT PRIMARY KEY NOT NULL,
    metrics_payload TEXT NOT NULL,
    snapshot_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_dashboard_metrics_cache_expires
    ON dashboard_metrics_cache (expires_at);

CREATE TABLE IF NOT EXISTS kpi_alert_rules (
    id TEXT PRIMARY KEY NOT NULL,
    kpi_name TEXT NOT NULL,
    threshold_value REAL NOT NULL,
    comparison_operator TEXT NOT NULL,
    severity TEXT NOT NULL DEFAULT 'warning',
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

CREATE INDEX IF NOT EXISTS idx_kpi_alert_rules_kpi ON kpi_alert_rules (kpi_name);

CREATE TABLE IF NOT EXISTS kpi_alert_log (
    alert_id TEXT PRIMARY KEY NOT NULL,
    kpi_name TEXT NOT NULL,
    current_value REAL NOT NULL,
    threshold_value REAL NOT NULL,
    severity TEXT NOT NULL,
    triggered_at TEXT NOT NULL,
    resolved_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_kpi_alert_log_kpi ON kpi_alert_log (kpi_name);
CREATE INDEX IF NOT EXISTS idx_kpi_alert_log_resolved ON kpi_alert_log (resolved_at);

CREATE TABLE IF NOT EXISTS daily_exception_summary (
    snapshot_date TEXT PRIMARY KEY NOT NULL,
    overdue_count INTEGER NOT NULL DEFAULT 0,
    missing_boe_count INTEGER NOT NULL DEFAULT 0,
    missing_expense_count INTEGER NOT NULL DEFAULT 0,
    missing_document_count INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

CREATE TABLE IF NOT EXISTS role_dashboard_permissions (
    role TEXT NOT NULL,
    widget_key TEXT NOT NULL,
    visible INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (role, widget_key)
);

CREATE TABLE IF NOT EXISTS dashboard_activity_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    timestamp TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    action_type TEXT NOT NULL,
    details TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_dashboard_activity_log_ts ON dashboard_activity_log (timestamp DESC);

CREATE TABLE IF NOT EXISTS shipment_compliance_score (
    shipment_id TEXT PRIMARY KEY NOT NULL,
    score REAL NOT NULL,
    required_slots INTEGER NOT NULL,
    satisfied_slots INTEGER NOT NULL,
    computed_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    FOREIGN KEY (shipment_id) REFERENCES shipments(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_shipment_compliance_score ON shipment_compliance_score (score);

INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('kpi_snapshot_retention_days', '365');
INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('kpi_retention_last_run', '');

-- Seed KPI alert rules (absolute thresholds; admin can edit)
INSERT OR IGNORE INTO kpi_alert_rules (id, kpi_name, threshold_value, comparison_operator, severity, enabled) VALUES
('rule-pending', 'pending_shipments', 10, 'GT', 'warning', 1),
('rule-overdue', 'overdue_eta', 5, 'GT', 'warning', 1);

-- Role → widget visibility (widget_key matches frontend sections)
INSERT OR IGNORE INTO role_dashboard_permissions (role, widget_key, visible) VALUES
('admin', 'kpis', 1), ('admin', 'charts', 1), ('admin', 'exceptions', 1), ('admin', 'finance', 1), ('admin', 'compliance', 1), ('admin', 'history', 1), ('admin', 'forecast', 1),
('operations', 'kpis', 1), ('operations', 'charts', 1), ('operations', 'exceptions', 1), ('operations', 'finance', 0), ('operations', 'compliance', 1), ('operations', 'history', 1), ('operations', 'forecast', 1),
('finance', 'kpis', 1), ('finance', 'charts', 1), ('finance', 'exceptions', 1), ('finance', 'finance', 1), ('finance', 'compliance', 1), ('finance', 'history', 1), ('finance', 'forecast', 1),
('viewer', 'kpis', 1), ('viewer', 'charts', 1), ('viewer', 'exceptions', 0), ('viewer', 'finance', 0), ('viewer', 'compliance', 1), ('viewer', 'history', 0), ('viewer', 'forecast', 0);

-- === V16__dashboard_activity_erp_fields.sql ===
-- ERP activity traceability + retention default for dashboard_activity_log

ALTER TABLE dashboard_activity_log ADD COLUMN module_name TEXT NOT NULL DEFAULT '';
ALTER TABLE dashboard_activity_log ADD COLUMN record_reference TEXT NOT NULL DEFAULT '';
ALTER TABLE dashboard_activity_log ADD COLUMN navigation_target TEXT NOT NULL DEFAULT '';
ALTER TABLE dashboard_activity_log ADD COLUMN action_context TEXT NOT NULL DEFAULT '';

INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('dashboard_activity_retention_days', '90');

-- === V17__exception_workflow.sql ===
-- Entity-level exception cases, resolution audit, lifecycle, notes, retention.

CREATE TABLE IF NOT EXISTS exception_cases (
    id TEXT PRIMARY KEY NOT NULL,
    exception_type TEXT NOT NULL,
    entity_type TEXT NOT NULL DEFAULT 'shipment',
    entity_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'OPEN',
    priority TEXT NOT NULL DEFAULT 'MEDIUM',
    assigned_to TEXT,
    assigned_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    resolved_by TEXT,
    resolved_at TEXT,
    sla_deadline TEXT,
    sla_status TEXT NOT NULL DEFAULT 'ON_TIME',
    last_viewed_at TEXT,
    FOREIGN KEY (entity_id) REFERENCES shipments(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_exception_cases_entity
    ON exception_cases (exception_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_exception_cases_status_type
    ON exception_cases (status, exception_type);
CREATE INDEX IF NOT EXISTS idx_exception_cases_resolved_at
    ON exception_cases (resolved_at);

CREATE TABLE IF NOT EXISTS exception_resolution_log (
    resolution_id TEXT PRIMARY KEY NOT NULL,
    exception_case_id TEXT NOT NULL,
    exception_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    status TEXT NOT NULL,
    resolved_by TEXT,
    resolved_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    notes TEXT NOT NULL DEFAULT '',
    FOREIGN KEY (exception_case_id) REFERENCES exception_cases(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_exception_resolution_case
    ON exception_resolution_log (exception_case_id);

CREATE TABLE IF NOT EXISTS exception_lifecycle_events (
    id TEXT PRIMARY KEY NOT NULL,
    exception_case_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    user_id TEXT,
    details TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    FOREIGN KEY (exception_case_id) REFERENCES exception_cases(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_exception_lifecycle_case
    ON exception_lifecycle_events (exception_case_id, created_at);

CREATE TABLE IF NOT EXISTS exception_notes (
    note_id TEXT PRIMARY KEY NOT NULL,
    exception_case_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    note_text TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    FOREIGN KEY (exception_case_id) REFERENCES exception_cases(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_exception_notes_case
    ON exception_notes (exception_case_id, created_at);

INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('exception_retention_days', '365');

-- === V18__workflow_reliability.sql ===
-- Workflow reliability: integrity log, SLA escalation, metrics, backlog, recurrence, activity checksums.

CREATE TABLE IF NOT EXISTS exception_integrity_log (
    id TEXT PRIMARY KEY NOT NULL,
    exception_id TEXT NOT NULL DEFAULT '',
    issue_type TEXT NOT NULL,
    detected_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    details TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_exception_integrity_detected
    ON exception_integrity_log (detected_at DESC);
CREATE INDEX IF NOT EXISTS idx_exception_integrity_type
    ON exception_integrity_log (issue_type);

CREATE TABLE IF NOT EXISTS exception_sla_escalation_rules (
    exception_type TEXT NOT NULL,
    sla_hours INTEGER NOT NULL DEFAULT 0,
    escalation_level INTEGER NOT NULL DEFAULT 1,
    notify_role TEXT NOT NULL DEFAULT 'admin',
    PRIMARY KEY (exception_type, escalation_level)
);

INSERT OR IGNORE INTO exception_sla_escalation_rules (exception_type, sla_hours, escalation_level, notify_role) VALUES
    ('OVERDUE_ETA', 0, 1, 'admin'),
    ('MISSING_BOE', 0, 1, 'admin'),
    ('MISSING_EXPENSE', 0, 1, 'admin');

CREATE TABLE IF NOT EXISTS exception_escalation_log (
    id TEXT PRIMARY KEY NOT NULL,
    exception_case_id TEXT NOT NULL,
    exception_type TEXT NOT NULL,
    escalation_level INTEGER NOT NULL DEFAULT 1,
    notify_role TEXT NOT NULL DEFAULT '',
    details TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    FOREIGN KEY (exception_case_id) REFERENCES exception_cases(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_exception_escalation_case
    ON exception_escalation_log (exception_case_id, created_at);

CREATE TABLE IF NOT EXISTS exception_sla_metrics (
    snapshot_date TEXT NOT NULL PRIMARY KEY,
    avg_resolution_hours REAL,
    sla_compliance_rate REAL NOT NULL DEFAULT 0,
    sla_breach_count INTEGER NOT NULL DEFAULT 0,
    resolution_backlog INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

CREATE TABLE IF NOT EXISTS exception_backlog_snapshot (
    snapshot_date TEXT NOT NULL PRIMARY KEY,
    open_count INTEGER NOT NULL DEFAULT 0,
    in_progress_count INTEGER NOT NULL DEFAULT 0,
    critical_count INTEGER NOT NULL DEFAULT 0,
    resolved_today INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

CREATE TABLE IF NOT EXISTS exception_recurrence_counts (
    exception_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    total_opens INTEGER NOT NULL DEFAULT 0,
    last_opened_at TEXT,
    PRIMARY KEY (exception_type, entity_id)
);

CREATE TABLE IF NOT EXISTS exception_resolution_analytics (
    snapshot_date TEXT NOT NULL PRIMARY KEY,
    avg_resolution_hours REAL,
    p90_resolution_hours REAL,
    resolved_count INTEGER NOT NULL DEFAULT 0,
    resolutions_by_user_json TEXT NOT NULL DEFAULT '{}',
    common_delay_hours REAL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

ALTER TABLE exception_cases ADD COLUMN escalated_at TEXT;
ALTER TABLE exception_cases ADD COLUMN escalation_level INTEGER NOT NULL DEFAULT 0;
ALTER TABLE exception_cases ADD COLUMN workflow_timeout_flag INTEGER NOT NULL DEFAULT 0;
ALTER TABLE exception_cases ADD COLUMN recurrence_flag INTEGER NOT NULL DEFAULT 0;

ALTER TABLE dashboard_activity_log ADD COLUMN checksum TEXT NOT NULL DEFAULT '';

INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('workflow_in_progress_timeout_days', '7');
INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('exception_escalation_cooldown_hours', '24');

-- === V19__workflow_observability.sql ===
-- Observability: maintenance history, health scores, simulation reports, resolution efficiency.

CREATE TABLE IF NOT EXISTS workflow_maintenance_history (
    run_id TEXT PRIMARY KEY NOT NULL,
    job_name TEXT NOT NULL,
    started_at TEXT NOT NULL,
    completed_at TEXT NOT NULL,
    status TEXT NOT NULL,
    records_processed INTEGER NOT NULL DEFAULT 0,
    errors_detected INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_workflow_maint_completed
    ON workflow_maintenance_history (completed_at DESC);

CREATE TABLE IF NOT EXISTS daily_workflow_health_score (
    snapshot_date TEXT NOT NULL PRIMARY KEY,
    health_score REAL NOT NULL,
    factors_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

CREATE TABLE IF NOT EXISTS workflow_simulation_reports (
    id TEXT PRIMARY KEY NOT NULL,
    user_id TEXT NOT NULL DEFAULT '',
    started_at TEXT NOT NULL,
    completed_at TEXT NOT NULL,
    inserted INTEGER NOT NULL DEFAULT 0,
    cleaned INTEGER NOT NULL DEFAULT 0,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    peak_load INTEGER NOT NULL DEFAULT 0,
    failure_rate REAL NOT NULL DEFAULT 0,
    recovery_success INTEGER NOT NULL DEFAULT 1,
    details_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_workflow_sim_completed
    ON workflow_simulation_reports (completed_at DESC);

CREATE TABLE IF NOT EXISTS resolution_efficiency_metrics (
    snapshot_date TEXT NOT NULL,
    user_id TEXT NOT NULL,
    resolutions_count INTEGER NOT NULL DEFAULT 0,
    avg_resolution_hours REAL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    PRIMARY KEY (snapshot_date, user_id)
);

INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('last_workflow_maintenance_at', '');
INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('last_integrity_check_at', '');

-- === V20__workflow_automation.sql ===
-- Rule-driven workflow automation, adaptive SLA snapshots, efficiency scoring, audit trail.

CREATE TABLE IF NOT EXISTS workflow_decision_rules (
    rule_id TEXT PRIMARY KEY NOT NULL,
    rule_name TEXT NOT NULL,
    rule_type TEXT NOT NULL,
    condition_expression TEXT NOT NULL DEFAULT '{}',
    action_type TEXT NOT NULL DEFAULT '',
    priority INTEGER NOT NULL DEFAULT 0,
    enabled INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

CREATE INDEX IF NOT EXISTS idx_workflow_rules_type_enabled
    ON workflow_decision_rules (rule_type, enabled, priority DESC);

CREATE TABLE IF NOT EXISTS workflow_automation_log (
    automation_id TEXT PRIMARY KEY NOT NULL,
    rule_id TEXT NOT NULL DEFAULT '',
    action_taken TEXT NOT NULL,
    target_entity TEXT NOT NULL DEFAULT '',
    executed_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    execution_result TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_workflow_automation_executed
    ON workflow_automation_log (executed_at DESC);

CREATE INDEX IF NOT EXISTS idx_workflow_automation_action_hour
    ON workflow_automation_log (action_taken, executed_at DESC);

CREATE TABLE IF NOT EXISTS adaptive_sla_adjustments (
    id TEXT PRIMARY KEY NOT NULL,
    exception_type TEXT NOT NULL,
    snapshot_date TEXT NOT NULL,
    previous_hours REAL NOT NULL DEFAULT 24,
    adjusted_hours REAL NOT NULL DEFAULT 24,
    factors_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    UNIQUE (exception_type, snapshot_date)
);

CREATE INDEX IF NOT EXISTS idx_adaptive_sla_date
    ON adaptive_sla_adjustments (snapshot_date DESC);

CREATE TABLE IF NOT EXISTS daily_workflow_efficiency_score (
    snapshot_date TEXT NOT NULL PRIMARY KEY,
    efficiency_score REAL NOT NULL,
    factors_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

ALTER TABLE exception_cases ADD COLUMN assignment_method TEXT NOT NULL DEFAULT 'MANUAL';

INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('workflow_automation_master_enabled', '1');
INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('automation_max_auto_resolve_per_hour', '40');
INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('automation_max_auto_escalate_per_hour', '15');
INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('automation_max_priority_adjust_per_cycle', '80');
INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('automation_paused_until', '');
INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('automation_last_pause_reason', '');
INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('automation_adaptive_sla_apply', '0');

-- Safe default rules (condition JSON interpreted by workflow_automation engine).
INSERT OR IGNORE INTO workflow_decision_rules (rule_id, rule_name, rule_type, condition_expression, action_type, priority, enabled)
VALUES
    ('rule-auto-resolve-overdue-delivered', 'Resolve OVERDUE when shipment delivered', 'AUTO_RESOLVE',
     '{"exceptionTypes":["OVERDUE_ETA"],"predicate":"shipment_delivered"}', 'AUTO_RESOLVE', 100, 1),
    ('rule-auto-resolve-boe-present', 'Resolve MISSING_BOE when BOE exists', 'AUTO_RESOLVE',
     '{"exceptionTypes":["MISSING_BOE"],"predicate":"has_boe"}', 'AUTO_RESOLVE', 99, 1),
    ('rule-auto-resolve-expense-present', 'Resolve MISSING_EXPENSE when expenses exist', 'AUTO_RESOLVE',
     '{"exceptionTypes":["MISSING_EXPENSE"],"predicate":"has_expenses"}', 'AUTO_RESOLVE', 98, 1),
    ('rule-auto-assign-load-balance', 'Assign unassigned cases (load balance)', 'AUTO_ASSIGN',
     '{"strategy":"load_balance","roles":["admin","user"]}', 'AUTO_ASSIGN', 50, 1),
    ('rule-priority-recurrence', 'Raise priority on recurrence', 'PRIORITY_ADJUST',
     '{"when":"recurrence_flag","minPriority":"HIGH"}', 'PRIORITY_ADJUST', 40, 1),
    ('rule-auto-escalate-sla', 'Allow SLA escalation pass (engine)', 'AUTO_ESCALATE',
     '{"useExistingEngine":true}', 'AUTO_ESCALATE', 30, 1);

-- === V21__workflow_rule_change_log.sql ===
-- Audit trail for automation rule configuration changes.

CREATE TABLE IF NOT EXISTS workflow_rule_change_log (
    change_id TEXT PRIMARY KEY NOT NULL,
    rule_id TEXT NOT NULL,
    changed_by TEXT NOT NULL DEFAULT '',
    change_type TEXT NOT NULL,
    previous_value TEXT NOT NULL DEFAULT '',
    new_value TEXT NOT NULL DEFAULT '',
    changed_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

CREATE INDEX IF NOT EXISTS idx_workflow_rule_change_rule
    ON workflow_rule_change_log (rule_id, changed_at DESC);

INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('automation_pause_duration_minutes', '60');
INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('automation_last_cycle_at', '');

-- === V22__automation_optimization_intelligence.sql ===
-- Automation optimization: effectiveness aggregates, feedback, safety, rollback audit, stability, benchmarking, ROI.

ALTER TABLE workflow_automation_log ADD COLUMN cases_resolved INTEGER NOT NULL DEFAULT 0;
ALTER TABLE workflow_automation_log ADD COLUMN cases_assigned INTEGER NOT NULL DEFAULT 0;
ALTER TABLE workflow_automation_log ADD COLUMN priority_adjustments INTEGER NOT NULL DEFAULT 0;
ALTER TABLE workflow_automation_log ADD COLUMN sla_improvements INTEGER NOT NULL DEFAULT 0;
ALTER TABLE workflow_automation_log ADD COLUMN resolution_time_reduction_hours REAL NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS rule_effectiveness_metrics (
    rule_id TEXT NOT NULL,
    metric_date TEXT NOT NULL,
    actions_taken INTEGER NOT NULL DEFAULT 0,
    successful_actions INTEGER NOT NULL DEFAULT 0,
    failed_actions INTEGER NOT NULL DEFAULT 0,
    cases_resolved INTEGER NOT NULL DEFAULT 0,
    cases_assigned INTEGER NOT NULL DEFAULT 0,
    priority_adjustments INTEGER NOT NULL DEFAULT 0,
    sla_improvements INTEGER NOT NULL DEFAULT 0,
    avg_resolution_time_saved REAL NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    PRIMARY KEY (rule_id, metric_date)
);

CREATE INDEX IF NOT EXISTS idx_rule_effectiveness_date ON rule_effectiveness_metrics (metric_date DESC);

CREATE TABLE IF NOT EXISTS automation_decision_feedback (
    id TEXT PRIMARY KEY NOT NULL,
    source_automation_id TEXT NOT NULL DEFAULT '',
    rule_id TEXT NOT NULL,
    target_entity TEXT NOT NULL,
    decision_result TEXT NOT NULL DEFAULT '{}',
    resolution_success INTEGER NOT NULL DEFAULT 0,
    follow_up_required INTEGER NOT NULL DEFAULT 0,
    resolution_time_hours REAL,
    helpful INTEGER,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_automation_feedback_source
    ON automation_decision_feedback (source_automation_id)
    WHERE length(trim(source_automation_id)) > 0;

CREATE INDEX IF NOT EXISTS idx_automation_feedback_rule ON automation_decision_feedback (rule_id, datetime(created_at) DESC);
CREATE INDEX IF NOT EXISTS idx_automation_feedback_target ON automation_decision_feedback (target_entity);

CREATE TABLE IF NOT EXISTS rule_safety_index (
    rule_id TEXT NOT NULL,
    snapshot_date TEXT NOT NULL,
    safety_score REAL NOT NULL,
    factors_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    PRIMARY KEY (rule_id, snapshot_date)
);

CREATE INDEX IF NOT EXISTS idx_rule_safety_date ON rule_safety_index (snapshot_date DESC);

CREATE TABLE IF NOT EXISTS rollback_action_log (
    id TEXT PRIMARY KEY NOT NULL,
    original_automation_id TEXT NOT NULL,
    rollback_type TEXT NOT NULL,
    target_entity TEXT NOT NULL,
    performed_by TEXT NOT NULL,
    performed_at TEXT NOT NULL,
    details_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_rollback_target ON rollback_action_log (target_entity, datetime(performed_at) DESC);

CREATE TABLE IF NOT EXISTS automation_stability_alert (
    id TEXT PRIMARY KEY NOT NULL,
    alert_type TEXT NOT NULL,
    severity TEXT NOT NULL,
    details_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    acknowledged INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_stability_alert_open ON automation_stability_alert (acknowledged, datetime(created_at) DESC);

CREATE TABLE IF NOT EXISTS automation_benchmark_history (
    id TEXT PRIMARY KEY NOT NULL,
    period_type TEXT NOT NULL,
    period_start TEXT NOT NULL,
    period_end TEXT NOT NULL,
    metrics_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

CREATE INDEX IF NOT EXISTS idx_automation_benchmark_period ON automation_benchmark_history (period_type, datetime(period_start) DESC);

CREATE TABLE IF NOT EXISTS automation_roi_metrics (
    snapshot_date TEXT PRIMARY KEY NOT NULL,
    time_saved_hours_estimate REAL NOT NULL DEFAULT 0,
    manual_workload_reduction_pct REAL NOT NULL DEFAULT 0,
    sla_compliance_improvement_pct REAL NOT NULL DEFAULT 0,
    resolution_speed_increase_pct REAL NOT NULL DEFAULT 0,
    factors_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('automation_last_benchmark_week', '');
INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('automation_last_benchmark_month', '');

-- === V23__automation_cost_intelligence.sql ===
-- Automation cost intelligence, capacity snapshots, economics index, cost guardrails.

ALTER TABLE workflow_automation_log ADD COLUMN actual_execution_time_ms INTEGER NOT NULL DEFAULT 0;
ALTER TABLE workflow_automation_log ADD COLUMN records_processed INTEGER NOT NULL DEFAULT 0;
ALTER TABLE workflow_automation_log ADD COLUMN estimated_cost_units REAL NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS rule_execution_cost_estimate (
    rule_id TEXT NOT NULL DEFAULT '',
    action_type TEXT NOT NULL,
    estimated_cpu_cost REAL NOT NULL DEFAULT 1,
    estimated_io_cost REAL NOT NULL DEFAULT 1,
    estimated_time_ms REAL NOT NULL DEFAULT 50,
    estimated_memory_cost REAL NOT NULL DEFAULT 1,
    cost_weight REAL NOT NULL DEFAULT 1,
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    PRIMARY KEY (rule_id, action_type)
);

CREATE INDEX IF NOT EXISTS idx_rule_exec_cost_action ON rule_execution_cost_estimate (action_type);

CREATE TABLE IF NOT EXISTS rule_cost_efficiency_metrics (
    rule_id TEXT NOT NULL,
    metric_date TEXT NOT NULL,
    total_cost_units REAL NOT NULL DEFAULT 0,
    total_actions INTEGER NOT NULL DEFAULT 0,
    total_resolution_gain REAL NOT NULL DEFAULT 0,
    cost_per_resolution REAL NOT NULL DEFAULT 0,
    efficiency_score REAL NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    PRIMARY KEY (rule_id, metric_date)
);

CREATE INDEX IF NOT EXISTS idx_rule_cost_eff_date ON rule_cost_efficiency_metrics (metric_date DESC);

CREATE TABLE IF NOT EXISTS automation_capacity_load (
    id TEXT PRIMARY KEY NOT NULL,
    snapshot_at TEXT NOT NULL,
    current_active_rules INTEGER NOT NULL DEFAULT 0,
    actions_per_cycle INTEGER NOT NULL DEFAULT 0,
    records_processed_per_cycle INTEGER NOT NULL DEFAULT 0,
    peak_cycle_duration_ms INTEGER NOT NULL DEFAULT 0,
    queue_depth INTEGER NOT NULL DEFAULT 0,
    load_percentage REAL NOT NULL DEFAULT 0,
    load_state TEXT NOT NULL DEFAULT 'LOW',
    total_cost_units_cycle REAL NOT NULL DEFAULT 0,
    factors_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_capacity_load_snapshot ON automation_capacity_load (datetime(snapshot_at) DESC);

CREATE TABLE IF NOT EXISTS automation_cost_limits (
    id TEXT PRIMARY KEY NOT NULL DEFAULT 'default',
    max_cost_units_per_cycle REAL NOT NULL DEFAULT 500000,
    max_execution_time_per_cycle_ms INTEGER NOT NULL DEFAULT 120000,
    max_records_processed_per_cycle INTEGER NOT NULL DEFAULT 500000,
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

INSERT OR IGNORE INTO automation_cost_limits (id, max_cost_units_per_cycle, max_execution_time_per_cycle_ms, max_records_processed_per_cycle)
VALUES ('default', 500000, 120000, 500000);

CREATE TABLE IF NOT EXISTS daily_automation_economics_index (
    snapshot_date TEXT PRIMARY KEY NOT NULL,
    benefit_score REAL NOT NULL DEFAULT 0,
    cost_score REAL NOT NULL DEFAULT 0,
    efficiency_gain REAL NOT NULL DEFAULT 0,
    economics_index REAL NOT NULL DEFAULT 0,
    factors_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('automation_cycle_started_at', '');

-- Global default cost weights by engine action (rule_id '' = fallback).
INSERT OR IGNORE INTO rule_execution_cost_estimate
    (rule_id, action_type, estimated_cpu_cost, estimated_io_cost, estimated_time_ms, estimated_memory_cost, cost_weight, updated_at)
VALUES
    ('', 'AUTO_RESOLVE', 1, 1, 45, 1, 1.0, strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    ('', 'AUTO_ASSIGN', 1.2, 1.8, 120, 1.2, 2.0, strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    ('', 'PRIORITY_ADJUST', 1, 1, 35, 1, 1.0, strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    ('', 'AUTO_ESCALATE', 1.5, 2, 200, 1.5, 2.5, strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    ('', 'ADAPTIVE_SLA', 2, 2.5, 350, 2, 3.5, strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    ('', 'AUTO_REPAIR', 1.1, 1.4, 90, 1.1, 1.8, strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    ('', 'AUTOMATION_CYCLE_SUMMARY', 0.5, 0.5, 20, 0.5, 0.3, strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'));

-- Per-rule rows inherit rule_type as action_type proxy where applicable (AUTO_RESOLVE rules share weight).
INSERT OR IGNORE INTO rule_execution_cost_estimate
    (rule_id, action_type, estimated_cpu_cost, estimated_io_cost, estimated_time_ms, estimated_memory_cost, cost_weight, updated_at)
SELECT
    r.rule_id,
    r.rule_type,
    1,
    1,
    CASE r.rule_type
        WHEN 'AUTO_RESOLVE' THEN 45
        WHEN 'AUTO_ASSIGN' THEN 120
        WHEN 'PRIORITY_ADJUST' THEN 35
        WHEN 'AUTO_ESCALATE' THEN 200
        ELSE 60
    END,
    1,
    CASE r.rule_type
        WHEN 'AUTO_RESOLVE' THEN 1.0
        WHEN 'AUTO_ASSIGN' THEN 2.0
        WHEN 'PRIORITY_ADJUST' THEN 1.0
        WHEN 'AUTO_ESCALATE' THEN 2.5
        ELSE 1.5
    END,
    strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
FROM workflow_decision_rules r;

-- === V24__workflow_rule_deployment.sql ===
-- Version-controlled rule deployment, staging, approvals, deployment log, canary, impact metrics.

CREATE TABLE IF NOT EXISTS workflow_rule_versions (
    version_id TEXT PRIMARY KEY NOT NULL,
    rule_id TEXT NOT NULL,
    version_number INTEGER NOT NULL,
    rule_definition_json TEXT NOT NULL,
    created_by TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    is_active INTEGER NOT NULL DEFAULT 0,
    change_reason TEXT NOT NULL DEFAULT '',
    UNIQUE (rule_id, version_number)
);

CREATE INDEX IF NOT EXISTS idx_workflow_rule_versions_rule
    ON workflow_rule_versions (rule_id, version_number DESC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_workflow_rule_versions_one_active
    ON workflow_rule_versions (rule_id)
    WHERE is_active = 1;

CREATE TABLE IF NOT EXISTS workflow_rule_staging (
    staging_id TEXT PRIMARY KEY NOT NULL,
    rule_id TEXT NOT NULL,
    version_id TEXT NOT NULL,
    staging_environment TEXT NOT NULL DEFAULT 'default',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    status TEXT NOT NULL DEFAULT 'DRAFT'
);

CREATE INDEX IF NOT EXISTS idx_workflow_rule_staging_rule ON workflow_rule_staging (rule_id, datetime(created_at) DESC);

CREATE TABLE IF NOT EXISTS workflow_rule_deployment_log (
    deployment_id TEXT PRIMARY KEY NOT NULL,
    rule_id TEXT NOT NULL,
    version_id TEXT NOT NULL,
    deployed_by TEXT NOT NULL,
    deployment_status TEXT NOT NULL,
    deployment_time TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    rollback_flag INTEGER NOT NULL DEFAULT 0,
    details_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_workflow_rule_deploy_rule ON workflow_rule_deployment_log (rule_id, datetime(deployment_time) DESC);

CREATE TABLE IF NOT EXISTS workflow_rule_approvals (
    approval_id TEXT PRIMARY KEY NOT NULL,
    rule_id TEXT NOT NULL,
    version_id TEXT NOT NULL,
    approved_by TEXT NOT NULL DEFAULT '',
    approval_status TEXT NOT NULL DEFAULT 'PENDING',
    approval_time TEXT,
    requested_by TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

CREATE INDEX IF NOT EXISTS idx_workflow_rule_approvals_rule ON workflow_rule_approvals (rule_id, approval_status);

CREATE TABLE IF NOT EXISTS canary_rule_deployment (
    id TEXT PRIMARY KEY NOT NULL,
    rule_id TEXT NOT NULL,
    version_id TEXT NOT NULL,
    sample_size_percentage REAL NOT NULL DEFAULT 10,
    deployment_status TEXT NOT NULL DEFAULT 'ACTIVE',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    UNIQUE (rule_id)
);

CREATE TABLE IF NOT EXISTS rule_deployment_impact_metrics (
    id TEXT PRIMARY KEY NOT NULL,
    rule_id TEXT NOT NULL,
    version_id TEXT NOT NULL,
    snapshot_at TEXT NOT NULL,
    failure_rate REAL NOT NULL DEFAULT 0,
    cost_units_delta REAL NOT NULL DEFAULT 0,
    resolution_gain_delta REAL NOT NULL DEFAULT 0,
    execution_count INTEGER NOT NULL DEFAULT 0,
    factors_json TEXT NOT NULL DEFAULT '{}',
    UNIQUE (rule_id, snapshot_at)
);

CREATE INDEX IF NOT EXISTS idx_rule_deploy_impact_version ON rule_deployment_impact_metrics (version_id, snapshot_at DESC);

INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('workflow_rule_deployment_frozen', '0');
INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('workflow_rule_deployment_requires_approval', '0');

-- Seed version 1 from current rules (deterministic version_id for idempotency).
INSERT OR IGNORE INTO workflow_rule_versions (
    version_id, rule_id, version_number, rule_definition_json, created_by, created_at, is_active, change_reason
)
SELECT
    rule_id || ':v:1',
    rule_id,
    1,
    json_object(
        'ruleName', rule_name,
        'ruleType', rule_type,
        'conditionExpression', condition_expression,
        'actionType', action_type,
        'priority', priority,
        'enabled', enabled
    ),
    'system',
    strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'),
    1,
    'Initial snapshot from workflow_decision_rules'
FROM workflow_decision_rules;

-- === V25__multi_environment_tenant.sql ===
-- Multi-environment registry, tenant isolation, environment-scoped rule versions, env deployment log.

CREATE TABLE IF NOT EXISTS workflow_environments (
    environment_id TEXT PRIMARY KEY NOT NULL,
    environment_name TEXT NOT NULL,
    environment_type TEXT NOT NULL,
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

INSERT OR IGNORE INTO workflow_environments (environment_id, environment_name, environment_type, is_active)
VALUES
    ('env-dev', 'DEV', 'DEV', 1),
    ('env-test', 'TEST', 'TEST', 1),
    ('env-prod', 'PROD', 'PROD', 1);

CREATE TABLE IF NOT EXISTS workflow_tenants (
    tenant_id TEXT PRIMARY KEY NOT NULL,
    tenant_name TEXT NOT NULL,
    tenant_status TEXT NOT NULL DEFAULT 'ACTIVE',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

INSERT OR IGNORE INTO workflow_tenants (tenant_id, tenant_name, tenant_status)
VALUES ('tenant-default', 'Default organization', 'ACTIVE');

-- Live decision rules: tenant scope (PK rule_id unchanged — one row per rule id per tenant in practice).
ALTER TABLE workflow_decision_rules ADD COLUMN tenant_id TEXT NOT NULL DEFAULT 'tenant-default';

CREATE INDEX IF NOT EXISTS idx_workflow_decision_rules_tenant
    ON workflow_decision_rules (tenant_id, rule_type, enabled, priority DESC);

-- Rebuild workflow_rule_versions with tenant + environment scope (unique per tenant/env/rule/vn).
CREATE TABLE IF NOT EXISTS workflow_rule_versions_new (
    version_id TEXT PRIMARY KEY NOT NULL,
    rule_id TEXT NOT NULL,
    tenant_id TEXT NOT NULL DEFAULT 'tenant-default',
    environment_id TEXT NOT NULL DEFAULT 'env-prod',
    version_number INTEGER NOT NULL,
    rule_definition_json TEXT NOT NULL,
    created_by TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    is_active INTEGER NOT NULL DEFAULT 0,
    change_reason TEXT NOT NULL DEFAULT '',
    UNIQUE (tenant_id, environment_id, rule_id, version_number)
);

INSERT INTO workflow_rule_versions_new (
    version_id, rule_id, tenant_id, environment_id, version_number,
    rule_definition_json, created_by, created_at, is_active, change_reason
)
SELECT
    version_id,
    rule_id,
    'tenant-default',
    'env-prod',
    version_number,
    rule_definition_json,
    created_by,
    created_at,
    is_active,
    change_reason
FROM workflow_rule_versions;

DROP TABLE workflow_rule_versions;
ALTER TABLE workflow_rule_versions_new RENAME TO workflow_rule_versions;

CREATE INDEX IF NOT EXISTS idx_workflow_rule_versions_rule_env
    ON workflow_rule_versions (tenant_id, environment_id, rule_id, version_number DESC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_workflow_rule_versions_one_active
    ON workflow_rule_versions (tenant_id, environment_id, rule_id)
    WHERE is_active = 1;

-- Environment-level deployment traceability
CREATE TABLE IF NOT EXISTS workflow_environment_deployment_log (
    deployment_id TEXT PRIMARY KEY NOT NULL,
    tenant_id TEXT NOT NULL,
    environment_id TEXT NOT NULL,
    rule_id TEXT NOT NULL,
    version_id TEXT NOT NULL,
    status TEXT NOT NULL,
    timestamp TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    details_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_wf_env_deploy_env_time
    ON workflow_environment_deployment_log (environment_id, datetime(timestamp) DESC);

CREATE INDEX IF NOT EXISTS idx_wf_env_deploy_tenant
    ON workflow_environment_deployment_log (tenant_id, datetime(timestamp) DESC);

-- Canary: tenant scope (rebuild unique constraint)
CREATE TABLE IF NOT EXISTS canary_rule_deployment_new (
    id TEXT PRIMARY KEY NOT NULL,
    tenant_id TEXT NOT NULL DEFAULT 'tenant-default',
    rule_id TEXT NOT NULL,
    version_id TEXT NOT NULL,
    sample_size_percentage REAL NOT NULL DEFAULT 10,
    deployment_status TEXT NOT NULL DEFAULT 'ACTIVE',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    UNIQUE (tenant_id, rule_id)
);

INSERT INTO canary_rule_deployment_new (id, tenant_id, rule_id, version_id, sample_size_percentage, deployment_status, created_at)
SELECT id, 'tenant-default', rule_id, version_id, sample_size_percentage, deployment_status, created_at
FROM canary_rule_deployment;

DROP TABLE canary_rule_deployment;
ALTER TABLE canary_rule_deployment_new RENAME TO canary_rule_deployment;

-- Automation log: tenant attribution for tenant dashboards (optional filter).
ALTER TABLE workflow_automation_log ADD COLUMN tenant_id TEXT NOT NULL DEFAULT 'tenant-default';

CREATE INDEX IF NOT EXISTS idx_workflow_automation_log_tenant_time
    ON workflow_automation_log (tenant_id, datetime(executed_at) DESC);

INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('workflow_active_tenant_id', 'tenant-default');
INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('workflow_execution_environment_id', 'env-prod');
INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('workflow_rule_version_default_environment_id', 'env-prod');
INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('workflow_deploy_require_capacity_headroom', '1');
INSERT OR IGNORE INTO app_metadata (key, value) VALUES ('workflow_deploy_simulation_gate_enabled', '0');

-- === V26__deployment_safety.sql ===
-- Deployment safety: conflict audit, risk assessments, risk timeline.
-- Enforcement is opt-out: set app_metadata workflow_deploy_prod_safety_enforcement to '0' to disable production gates.

CREATE TABLE IF NOT EXISTS deployment_conflict_log (
    id TEXT PRIMARY KEY NOT NULL,
    detected_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    rule_id TEXT NOT NULL,
    tenant_id TEXT NOT NULL,
    environment_id TEXT NOT NULL,
    related_version_id TEXT,
    conflict_type TEXT NOT NULL,
    details_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_deployment_conflict_rule_time
    ON deployment_conflict_log (rule_id, datetime(detected_at) DESC);

CREATE TABLE IF NOT EXISTS deployment_risk_assessment (
    assessment_id TEXT PRIMARY KEY NOT NULL,
    assessed_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    rule_id TEXT NOT NULL,
    version_id TEXT NOT NULL,
    tenant_id TEXT NOT NULL,
    environment_id TEXT NOT NULL,
    risk_score REAL NOT NULL,
    risk_level TEXT NOT NULL,
    safe_to_deploy INTEGER NOT NULL,
    assessment_json TEXT NOT NULL DEFAULT '{}',
    UNIQUE (rule_id, version_id, tenant_id, environment_id)
);

CREATE INDEX IF NOT EXISTS idx_deployment_risk_assessment_rule
    ON deployment_risk_assessment (rule_id, datetime(assessed_at) DESC);

CREATE TABLE IF NOT EXISTS deployment_risk_timeline (
    id TEXT PRIMARY KEY NOT NULL,
    deployment_time TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    risk_score REAL NOT NULL,
    risk_level TEXT NOT NULL,
    environment_id TEXT NOT NULL,
    tenant_id TEXT NOT NULL,
    rule_id TEXT NOT NULL,
    version_id TEXT NOT NULL,
    result TEXT NOT NULL,
    details_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_deployment_risk_timeline_env_time
    ON deployment_risk_timeline (environment_id, datetime(deployment_time) DESC);

-- Default ON for new installs; set to '0' to opt out of automatic production safety blocking.
INSERT INTO app_metadata (key, value)
SELECT 'workflow_deploy_prod_safety_enforcement', '1'
WHERE NOT EXISTS (
    SELECT 1 FROM app_metadata WHERE key = 'workflow_deploy_prod_safety_enforcement'
);

-- === V27__workflow_background_jobs.sql ===
-- Background job registry, execution history, alerts, performance, dependencies, reliability.

CREATE TABLE IF NOT EXISTS workflow_background_jobs (
    job_id TEXT PRIMARY KEY NOT NULL,
    job_name TEXT NOT NULL,
    job_type TEXT NOT NULL,
    schedule_type TEXT NOT NULL,
    is_enabled INTEGER NOT NULL DEFAULT 1,
    expected_duration_ms INTEGER NOT NULL DEFAULT 120000,
    max_retries INTEGER NOT NULL DEFAULT 3,
    retry_delay_sec INTEGER NOT NULL DEFAULT 120,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

INSERT OR IGNORE INTO workflow_background_jobs (job_id, job_name, job_type, schedule_type, is_enabled, expected_duration_ms, max_retries, retry_delay_sec)
VALUES
    ('automation_cycle', 'Workflow automation cycle', 'AUTOMATION', 'DAILY', 1, 180000, 3, 120),
    ('deployment_safety_checks', 'Deployment safety checks', 'SAFETY', 'DAILY', 1, 60000, 2, 300),
    ('maintenance_cleanup', 'Retention and daily exception maintenance', 'MAINTENANCE', 'DAILY', 1, 300000, 2, 300),
    ('cost_metrics_aggregation', 'Daily automation economics index', 'METRICS', 'DAILY', 1, 90000, 3, 120),
    ('risk_evaluation', 'Deployment risk signals rollup', 'RISK', 'DAILY', 1, 60000, 2, 300),
    ('observability_update', 'Workflow observability completion', 'OBSERVABILITY', 'DAILY', 1, 120000, 2, 120);

CREATE TABLE IF NOT EXISTS workflow_job_execution_log (
    execution_id TEXT PRIMARY KEY NOT NULL,
    job_id TEXT NOT NULL,
    started_at TEXT NOT NULL,
    completed_at TEXT,
    status TEXT NOT NULL,
    records_processed INTEGER NOT NULL DEFAULT 0,
    error_message TEXT,
    execution_time_ms INTEGER,
    retry_count INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_wf_job_exec_job_time
    ON workflow_job_execution_log (job_id, datetime(started_at) DESC);

CREATE TABLE IF NOT EXISTS workflow_job_failure_alerts (
    alert_id TEXT PRIMARY KEY NOT NULL,
    job_id TEXT NOT NULL,
    detected_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    alert_type TEXT NOT NULL,
    details_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_wf_job_failure_job_time
    ON workflow_job_failure_alerts (job_id, datetime(detected_at) DESC);

CREATE TABLE IF NOT EXISTS workflow_job_performance_metrics (
    id TEXT PRIMARY KEY NOT NULL,
    job_id TEXT NOT NULL,
    window_label TEXT NOT NULL,
    avg_execution_ms REAL,
    max_execution_ms REAL,
    records_processed_total INTEGER,
    failure_rate REAL,
    retry_rate REAL,
    samples INTEGER,
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    UNIQUE (job_id, window_label)
);

CREATE TABLE IF NOT EXISTS workflow_job_dependencies (
    id TEXT PRIMARY KEY NOT NULL,
    parent_job_id TEXT NOT NULL,
    dependent_job_id TEXT NOT NULL,
    dependency_type TEXT NOT NULL,
    UNIQUE (parent_job_id, dependent_job_id)
);

INSERT OR IGNORE INTO workflow_job_dependencies (id, parent_job_id, dependent_job_id, dependency_type)
VALUES
    ('dep-1', 'maintenance_cleanup', 'observability_update', 'SEQUENTIAL'),
    ('dep-2', 'maintenance_cleanup', 'automation_cycle', 'SEQUENTIAL'),
    ('dep-3', 'automation_cycle', 'cost_metrics_aggregation', 'SEQUENTIAL');

CREATE TABLE IF NOT EXISTS workflow_job_alert_log (
    id TEXT PRIMARY KEY NOT NULL,
    job_id TEXT,
    execution_id TEXT,
    alert_level TEXT NOT NULL,
    message TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    details_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_wf_job_alert_time ON workflow_job_alert_log (datetime(created_at) DESC);

CREATE TABLE IF NOT EXISTS workflow_job_reliability_score (
    job_id TEXT PRIMARY KEY NOT NULL,
    score REAL NOT NULL,
    success_rate REAL,
    failure_rate REAL,
    retry_frequency REAL,
    sample_executions INTEGER,
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

-- === V28__workflow_job_schedule_recovery.sql ===
-- Missed schedule expectations, missed-run alerts, recovery audit, drift metrics, recovery scores.

CREATE TABLE IF NOT EXISTS workflow_job_schedule_expectations (
    job_id TEXT PRIMARY KEY NOT NULL,
    expected_interval_minutes INTEGER NOT NULL,
    grace_period_minutes INTEGER NOT NULL,
    last_expected_run_at TEXT,
    max_recovery_attempts INTEGER NOT NULL DEFAULT 5,
    recovery_delay_sec INTEGER NOT NULL DEFAULT 120,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

INSERT OR IGNORE INTO workflow_job_schedule_expectations (job_id, expected_interval_minutes, grace_period_minutes, last_expected_run_at, max_recovery_attempts, recovery_delay_sec)
VALUES
    ('automation_cycle', 1440, 120, strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'), 5, 120),
    ('maintenance_cleanup', 1440, 120, strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'), 5, 180),
    ('risk_evaluation', 1440, 120, strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'), 5, 120),
    ('observability_update', 1440, 120, strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'), 5, 120);

CREATE TABLE IF NOT EXISTS workflow_job_missed_alerts (
    alert_id TEXT PRIMARY KEY NOT NULL,
    job_id TEXT NOT NULL,
    expected_time TEXT NOT NULL,
    detected_time TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    recovery_triggered INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'PENDING'
);

CREATE INDEX IF NOT EXISTS idx_wf_job_missed_job_status
    ON workflow_job_missed_alerts (job_id, status, datetime(detected_time) DESC);

CREATE TABLE IF NOT EXISTS workflow_job_recovery_log (
    recovery_id TEXT PRIMARY KEY NOT NULL,
    job_id TEXT NOT NULL,
    alert_id TEXT,
    recovery_time TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    result TEXT NOT NULL,
    attempt_count INTEGER NOT NULL DEFAULT 1,
    error_message TEXT
);

CREATE INDEX IF NOT EXISTS idx_wf_job_recovery_job_time
    ON workflow_job_recovery_log (job_id, datetime(recovery_time) DESC);

CREATE TABLE IF NOT EXISTS daily_missed_job_metrics (
    metric_date TEXT PRIMARY KEY NOT NULL,
    missed_runs INTEGER NOT NULL DEFAULT 0,
    recovery_success INTEGER NOT NULL DEFAULT 0,
    recovery_failures INTEGER NOT NULL DEFAULT 0,
    drift_warnings INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

CREATE TABLE IF NOT EXISTS workflow_job_recovery_score (
    job_id TEXT PRIMARY KEY NOT NULL,
    score REAL NOT NULL DEFAULT 0,
    missed_jobs INTEGER NOT NULL DEFAULT 0,
    recovered_jobs INTEGER NOT NULL DEFAULT 0,
    window_days INTEGER NOT NULL DEFAULT 30,
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

-- === V29__workflow_job_manual_governance.sql ===
-- Manual operator governance: recovery guard overrides and audit trail.

CREATE TABLE IF NOT EXISTS workflow_job_manual_override_log (
    id TEXT PRIMARY KEY NOT NULL,
    job_id TEXT NOT NULL,
    action TEXT NOT NULL,
    reason TEXT,
    caller_role TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

CREATE INDEX IF NOT EXISTS idx_wf_job_manual_override_job_time
    ON workflow_job_manual_override_log (job_id, datetime(created_at) DESC);

-- === V30__workflow_production_observability.sql ===
-- Production observability: structured events, runtime counters, alert signals, timing samples, reliability score.

CREATE TABLE IF NOT EXISTS workflow_structured_event_log (
  id TEXT PRIMARY KEY NOT NULL,
  timestamp TEXT NOT NULL,
  module TEXT NOT NULL,
  event_type TEXT NOT NULL,
  entity_id TEXT,
  severity TEXT NOT NULL,
  details_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_workflow_structured_event_ts
  ON workflow_structured_event_log (datetime(timestamp) DESC);

CREATE INDEX IF NOT EXISTS idx_workflow_structured_event_module
  ON workflow_structured_event_log (module, datetime(timestamp) DESC);

CREATE TABLE IF NOT EXISTS workflow_runtime_metrics (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  jobs_executed INTEGER NOT NULL DEFAULT 0,
  jobs_failed INTEGER NOT NULL DEFAULT 0,
  jobs_recovered INTEGER NOT NULL DEFAULT 0,
  deployments_blocked INTEGER NOT NULL DEFAULT 0,
  deployments_succeeded INTEGER NOT NULL DEFAULT 0,
  recovery_attempts INTEGER NOT NULL DEFAULT 0,
  risk_evaluations INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL
);

INSERT OR IGNORE INTO workflow_runtime_metrics (id, jobs_executed, jobs_failed, jobs_recovered, deployments_blocked, deployments_succeeded, recovery_attempts, risk_evaluations, updated_at)
VALUES (1, 0, 0, 0, 0, 0, 0, 0, strftime('%Y-%m-%d %H:%M:%S', 'now'));

CREATE TABLE IF NOT EXISTS workflow_alert_signal_log (
  id TEXT PRIMARY KEY NOT NULL,
  created_at TEXT NOT NULL,
  signal_type TEXT NOT NULL,
  severity TEXT NOT NULL,
  entity_id TEXT,
  message TEXT NOT NULL,
  details_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_workflow_alert_signal_created
  ON workflow_alert_signal_log (datetime(created_at) DESC);

CREATE INDEX IF NOT EXISTS idx_workflow_alert_signal_severity
  ON workflow_alert_signal_log (severity, datetime(created_at) DESC);

CREATE TABLE IF NOT EXISTS performance_timing_metrics (
  id TEXT PRIMARY KEY NOT NULL,
  category TEXT NOT NULL,
  entity_id TEXT,
  duration_ms INTEGER NOT NULL,
  recorded_at TEXT NOT NULL,
  details_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_performance_timing_cat
  ON performance_timing_metrics (category, datetime(recorded_at) DESC);

CREATE TABLE IF NOT EXISTS system_reliability_score (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  score REAL NOT NULL,
  success_rate REAL NOT NULL,
  failure_rate REAL NOT NULL,
  recovery_success_rate REAL NOT NULL,
  sample_window_days INTEGER NOT NULL DEFAULT 30,
  updated_at TEXT NOT NULL,
  factors_json TEXT NOT NULL DEFAULT '{}'
);

INSERT OR IGNORE INTO system_reliability_score (id, score, success_rate, failure_rate, recovery_success_rate, sample_window_days, updated_at, factors_json)
VALUES (1, 1.0, 1.0, 0.0, 1.0, 30, strftime('%Y-%m-%d %H:%M:%S', 'now'), '{}');

-- === V31__workflow_incident_management.sql ===
-- Incident lifecycle: alert → incident → assignment → resolution → audit.

CREATE TABLE IF NOT EXISTS workflow_incidents (
  incident_id TEXT PRIMARY KEY NOT NULL,
  alert_id TEXT NOT NULL,
  incident_type TEXT NOT NULL,
  severity TEXT NOT NULL,
  status TEXT NOT NULL,
  assigned_to TEXT NOT NULL DEFAULT 'unassigned',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  resolved_at TEXT,
  resolution_notes TEXT,
  escalation_count INTEGER NOT NULL DEFAULT 0,
  UNIQUE(alert_id)
);

CREATE INDEX IF NOT EXISTS idx_workflow_incidents_status
  ON workflow_incidents (status, datetime(updated_at) DESC);
CREATE INDEX IF NOT EXISTS idx_workflow_incidents_severity
  ON workflow_incidents (severity, status);
CREATE INDEX IF NOT EXISTS idx_workflow_incidents_assigned
  ON workflow_incidents (assigned_to, status);

CREATE TABLE IF NOT EXISTS workflow_incident_notes (
  note_id TEXT PRIMARY KEY NOT NULL,
  incident_id TEXT NOT NULL,
  note_text TEXT NOT NULL,
  created_by TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_workflow_incident_notes_incident
  ON workflow_incident_notes (incident_id, datetime(created_at) ASC);

CREATE TABLE IF NOT EXISTS workflow_incident_metrics (
  metric_date TEXT PRIMARY KEY NOT NULL,
  incidents_opened INTEGER NOT NULL DEFAULT 0,
  incidents_resolved INTEGER NOT NULL DEFAULT 0,
  sum_resolution_minutes INTEGER NOT NULL DEFAULT 0,
  resolution_samples INTEGER NOT NULL DEFAULT 0,
  severity_fatal INTEGER NOT NULL DEFAULT 0,
  severity_critical INTEGER NOT NULL DEFAULT 0,
  severity_warning INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS workflow_incident_response_score (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  score REAL NOT NULL,
  avg_resolution_minutes REAL NOT NULL,
  backlog_open INTEGER NOT NULL,
  response_latency_minutes REAL NOT NULL,
  updated_at TEXT NOT NULL,
  factors_json TEXT NOT NULL DEFAULT '{}'
);

INSERT OR IGNORE INTO workflow_incident_response_score (id, score, avg_resolution_minutes, backlog_open, response_latency_minutes, updated_at, factors_json)
VALUES (1, 100.0, 0.0, 0, 0.0, strftime('%Y-%m-%d %H:%M:%S', 'now'), '{}');

-- === V32__workflow_incident_engine.sql ===
-- Single-operator incident engine: replaces V31 team-oriented tables.

PRAGMA foreign_keys = OFF;

DROP TABLE IF EXISTS workflow_incident_notes;
DROP TABLE IF EXISTS workflow_incident_history;
DROP TABLE IF EXISTS workflow_incidents;
DROP TABLE IF EXISTS workflow_incident_metrics;
DROP TABLE IF EXISTS workflow_incident_response_score;

PRAGMA foreign_keys = ON;

CREATE TABLE workflow_incidents (
  incident_id TEXT PRIMARY KEY NOT NULL,
  created_at TEXT NOT NULL,
  severity TEXT NOT NULL,
  status TEXT NOT NULL,
  error_context_json TEXT NOT NULL DEFAULT '{}',
  source_module TEXT NOT NULL,
  linked_alert_id TEXT NOT NULL,
  trigger_event_type TEXT NOT NULL,
  correlation_id TEXT NOT NULL,
  resolved_at TEXT,
  root_cause_summary TEXT
);

CREATE INDEX idx_workflow_incidents_status ON workflow_incidents (status);
CREATE INDEX idx_workflow_incidents_severity ON workflow_incidents (severity);
CREATE INDEX idx_workflow_incidents_created_at ON workflow_incidents (datetime(created_at) DESC);
CREATE INDEX idx_workflow_incidents_correlation ON workflow_incidents (correlation_id);

CREATE TABLE workflow_incident_history (
  history_id TEXT PRIMARY KEY NOT NULL,
  incident_id TEXT NOT NULL,
  event_type TEXT NOT NULL,
  event_timestamp TEXT NOT NULL,
  notes TEXT,
  details_json TEXT NOT NULL DEFAULT '{}',
  FOREIGN KEY (incident_id) REFERENCES workflow_incidents(incident_id) ON DELETE CASCADE
);

CREATE INDEX idx_workflow_incident_history_incident ON workflow_incident_history (incident_id, datetime(event_timestamp));

CREATE TABLE workflow_incident_metrics (
  metric_date TEXT PRIMARY KEY NOT NULL,
  incidents_created_today INTEGER NOT NULL DEFAULT 0,
  incidents_resolved_today INTEGER NOT NULL DEFAULT 0,
  avg_resolution_time REAL NOT NULL DEFAULT 0,
  critical_incident_count INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL
);

-- === V33__workflow_incident_correlation.sql ===
-- Incident correlation: stable key for grouping + daily correlation KPIs.

ALTER TABLE workflow_incidents ADD COLUMN correlation_key TEXT NOT NULL DEFAULT '';

UPDATE workflow_incidents
SET correlation_key = trim(source_module) || '|' || trim(trigger_event_type) || '|' ||
  CASE
    WHEN json_extract(error_context_json, '$.entityId') IS NOT NULL
      AND trim(json_extract(error_context_json, '$.entityId')) != ''
    THEN trim(json_extract(error_context_json, '$.entityId'))
    ELSE '__global__'
  END
WHERE correlation_key = '';

CREATE INDEX IF NOT EXISTS idx_workflow_incidents_correlation_key
  ON workflow_incidents (correlation_key, status);

CREATE TABLE workflow_incident_correlation_metrics (
  metric_date TEXT PRIMARY KEY NOT NULL,
  alerts_grouped INTEGER NOT NULL DEFAULT 0,
  incidents_created INTEGER NOT NULL DEFAULT 0,
  noise_reduction_ratio REAL NOT NULL DEFAULT 0,
  burst_signals_emitted INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL
);

-- === V34__workflow_failure_burst_log.sql ===
-- Systemic failure burst intelligence (rate vs baseline, cooling, incident promotion).

CREATE TABLE IF NOT EXISTS workflow_failure_burst_log (
  burst_id TEXT PRIMARY KEY NOT NULL,
  source_module TEXT NOT NULL,
  event_type TEXT NOT NULL,
  burst_start_time TEXT NOT NULL,
  burst_end_time TEXT NOT NULL,
  event_count INTEGER NOT NULL,
  baseline_rate REAL NOT NULL,
  current_rate REAL NOT NULL,
  severity TEXT NOT NULL,
  root_cause_hint TEXT NOT NULL DEFAULT '',
  details_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_workflow_failure_burst_module_time
  ON workflow_failure_burst_log (source_module, event_type, datetime(burst_start_time) DESC);

-- === V35__workflow_incident_unified_intelligence.sql ===
-- Unified incident intelligence: amplification counters, burst analytics, noise score.

ALTER TABLE workflow_incidents ADD COLUMN correlated_event_count INTEGER NOT NULL DEFAULT 1;
ALTER TABLE workflow_incidents ADD COLUMN last_correlated_at TEXT;

UPDATE workflow_incidents SET correlation_key = replace(correlation_key, '|', ':') WHERE correlation_key LIKE '%|%';

ALTER TABLE workflow_incident_correlation_metrics ADD COLUMN bursts_detected INTEGER NOT NULL DEFAULT 0;

ALTER TABLE workflow_failure_burst_log ADD COLUMN confidence_score REAL NOT NULL DEFAULT 0.72;

CREATE TABLE IF NOT EXISTS workflow_incident_noise_score (
  metric_date TEXT PRIMARY KEY NOT NULL,
  noise_score REAL NOT NULL DEFAULT 0,
  total_alerts INTEGER NOT NULL DEFAULT 0,
  alerts_grouped INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL
);

-- === V36__workflow_incident_suppression.sql ===
-- Intelligent incident suppression after bursts / manual ops (noise reduction).

CREATE TABLE IF NOT EXISTS workflow_incident_suppression (
  suppression_id TEXT PRIMARY KEY NOT NULL,
  correlation_id TEXT,
  incident_id TEXT,
  source_module TEXT NOT NULL,
  event_type TEXT NOT NULL,
  suppression_start TEXT NOT NULL,
  suppression_end TEXT NOT NULL,
  last_activity_at TEXT NOT NULL,
  suppressed_event_count INTEGER NOT NULL DEFAULT 0,
  confidence_score REAL NOT NULL DEFAULT 0.55,
  release_history_logged INTEGER NOT NULL DEFAULT 0,
  reason TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_wf_suppression_lookup
  ON workflow_incident_suppression (source_module, event_type, datetime(suppression_end));

CREATE TABLE IF NOT EXISTS workflow_incident_suppression_metrics (
  metric_date TEXT PRIMARY KEY NOT NULL,
  alerts_suppressed INTEGER NOT NULL DEFAULT 0,
  suppression_windows INTEGER NOT NULL DEFAULT 0,
  noise_reduction_gain REAL NOT NULL DEFAULT 0,
  confidence_score REAL NOT NULL DEFAULT 0.55,
  updated_at TEXT NOT NULL
);

-- === V37__workflow_incident_stabilization.sql ===
-- Stabilization phases: quiet → stabilizing → confirmed (recovery trust signals).

CREATE TABLE IF NOT EXISTS workflow_incident_stabilization (
  stabilization_id TEXT PRIMARY KEY NOT NULL,
  correlation_id TEXT,
  incident_id TEXT,
  source_module TEXT NOT NULL,
  event_type TEXT NOT NULL,
  stabilization_start TEXT NOT NULL,
  stabilization_confirmed TEXT,
  stability_duration_minutes INTEGER NOT NULL DEFAULT 0,
  confidence_score REAL NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_wf_stabilization_scope
  ON workflow_incident_stabilization (source_module, event_type, datetime(stabilization_start) DESC);

CREATE TABLE IF NOT EXISTS workflow_incident_stabilization_metrics (
  metric_date TEXT PRIMARY KEY NOT NULL,
  stabilizations_detected INTEGER NOT NULL DEFAULT 0,
  avg_stabilization_time REAL NOT NULL DEFAULT 0,
  false_recovery_rate REAL NOT NULL DEFAULT 0,
  stability_confidence_avg REAL NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS workflow_system_stability_score (
  id INTEGER PRIMARY KEY CHECK (id = 1) NOT NULL,
  stability_score REAL NOT NULL DEFAULT 0,
  successful_stabilizations INTEGER NOT NULL DEFAULT 0,
  total_incidents INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL
);

INSERT OR IGNORE INTO workflow_system_stability_score (id, stability_score, successful_stabilizations, total_incidents, updated_at)
VALUES (1, 0, 0, 0, strftime('%Y-%m-%d %H:%M:%S', 'now'));

-- === V38__workflow_incident_regression.sql ===
-- Post-stabilization recurrence (regression) tracking and fragility scoring.

CREATE TABLE IF NOT EXISTS workflow_incident_regression (
  regression_id TEXT PRIMARY KEY NOT NULL,
  correlation_id TEXT,
  incident_id TEXT NOT NULL,
  source_module TEXT NOT NULL,
  event_type TEXT NOT NULL,
  regression_detected_at TEXT NOT NULL,
  time_since_stabilization_minutes INTEGER NOT NULL,
  severity TEXT NOT NULL,
  details_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_wf_regression_detected
  ON workflow_incident_regression (datetime(regression_detected_at) DESC);

CREATE INDEX IF NOT EXISTS idx_wf_regression_scope
  ON workflow_incident_regression (source_module, event_type);

CREATE TABLE IF NOT EXISTS workflow_incident_regression_metrics (
  metric_date TEXT PRIMARY KEY NOT NULL,
  regressions_detected INTEGER NOT NULL DEFAULT 0,
  avg_regression_time_minutes REAL NOT NULL DEFAULT 0,
  regression_frequency REAL NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS workflow_regression_risk_score (
  id INTEGER PRIMARY KEY CHECK (id = 1) NOT NULL,
  regression_risk REAL NOT NULL DEFAULT 0,
  regressions_detected INTEGER NOT NULL DEFAULT 0,
  stabilizations_detected INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL
);

INSERT OR IGNORE INTO workflow_regression_risk_score (id, regression_risk, regressions_detected, stabilizations_detected, updated_at)
VALUES (1, 0, 0, 0, strftime('%Y-%m-%d %H:%M:%S', 'now'));

-- === V39__workflow_structured_regression_metrics.sql ===
-- Structured-triggered regression rollups (complement to alert-based regression).

CREATE TABLE IF NOT EXISTS workflow_structured_regression_metrics (
  metric_date TEXT PRIMARY KEY NOT NULL,
  structured_regressions_detected INTEGER NOT NULL DEFAULT 0,
  avg_structured_regression_time REAL NOT NULL DEFAULT 0,
  structured_regression_ratio REAL NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL
);

-- === V40__workflow_incident_persistence.sql ===
-- Persistent failure pressure: elevated rate after stabilization without sustained recovery.

CREATE TABLE IF NOT EXISTS workflow_incident_persistence (
  persistence_id TEXT PRIMARY KEY NOT NULL,
  correlation_id TEXT,
  incident_id TEXT NOT NULL,
  source_module TEXT NOT NULL,
  event_type TEXT NOT NULL,
  persistence_detected_at TEXT NOT NULL,
  failure_rate REAL NOT NULL,
  expected_rate REAL NOT NULL,
  severity TEXT NOT NULL,
  details_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_wf_persistence_detected
  ON workflow_incident_persistence (datetime(persistence_detected_at) DESC);

CREATE INDEX IF NOT EXISTS idx_wf_persistence_scope
  ON workflow_incident_persistence (source_module, event_type);

CREATE TABLE IF NOT EXISTS workflow_persistent_failure_metrics (
  metric_date TEXT PRIMARY KEY NOT NULL,
  persistent_failures_detected INTEGER NOT NULL DEFAULT 0,
  avg_persistence_duration REAL NOT NULL DEFAULT 0,
  persistence_frequency REAL NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS workflow_persistence_risk_score (
  id INTEGER PRIMARY KEY CHECK (id = 1) NOT NULL,
  persistence_risk REAL NOT NULL DEFAULT 0,
  persistent_failures_detected INTEGER NOT NULL DEFAULT 0,
  total_incidents INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL
);

INSERT OR IGNORE INTO workflow_persistence_risk_score (id, persistence_risk, persistent_failures_detected, total_incidents, updated_at)
VALUES (1, 0, 0, 0, strftime('%Y-%m-%d %H:%M:%S', 'now'));

-- === V41__workflow_failure_forecast.sql ===
-- Failure forecasting: trend + regression + persistence → probability, metrics, risk score.

CREATE TABLE IF NOT EXISTS workflow_failure_forecast (
    forecast_id TEXT PRIMARY KEY NOT NULL,
    source_module TEXT NOT NULL,
    event_type TEXT NOT NULL,
    forecast_time TEXT NOT NULL,
    predicted_failure_probability REAL NOT NULL,
    confidence_score REAL NOT NULL,
    forecast_horizon_minutes INTEGER NOT NULL DEFAULT 30,
    details_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_wf_failure_forecast_scope_time
    ON workflow_failure_forecast (source_module, event_type, datetime(forecast_time) DESC);

CREATE TABLE IF NOT EXISTS workflow_failure_forecast_metrics (
    metric_date TEXT PRIMARY KEY NOT NULL,
    forecasts_generated INTEGER NOT NULL DEFAULT 0,
    forecast_accuracy REAL NOT NULL DEFAULT 0,
    forecast_false_positive_rate REAL NOT NULL DEFAULT 0,
    prediction_accuracy_score REAL NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS workflow_forecast_risk_score (
    id INTEGER PRIMARY KEY CHECK (id = 1) NOT NULL,
    forecast_risk_score REAL NOT NULL DEFAULT 0,
    high_risk_forecasts INTEGER NOT NULL DEFAULT 0,
    total_forecasts INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL
);

INSERT OR IGNORE INTO workflow_forecast_risk_score (id, forecast_risk_score, high_risk_forecasts, total_forecasts, updated_at)
VALUES (1, 0, 0, 0, strftime('%Y-%m-%d %H:%M:%S', 'now'));

-- === V42__workflow_forecast_explainability.sql ===
-- Forecast explainability: triggers, summaries, explanation metrics, operator feedback.

ALTER TABLE workflow_failure_forecast ADD COLUMN primary_trigger TEXT NOT NULL DEFAULT '';
ALTER TABLE workflow_failure_forecast ADD COLUMN secondary_triggers_json TEXT NOT NULL DEFAULT '{}';
ALTER TABLE workflow_failure_forecast ADD COLUMN trend_summary TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS workflow_forecast_explanation_metrics (
    metric_date TEXT PRIMARY KEY NOT NULL,
    explanations_generated INTEGER NOT NULL DEFAULT 0,
    accurate_explanations INTEGER NOT NULL DEFAULT 0,
    misleading_explanations INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS workflow_forecast_explanation_score (
    id INTEGER PRIMARY KEY CHECK (id = 1) NOT NULL,
    explanation_accuracy_score REAL NOT NULL DEFAULT 0,
    accurate_explanations INTEGER NOT NULL DEFAULT 0,
    total_explanations INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL
);

INSERT OR IGNORE INTO workflow_forecast_explanation_score (id, explanation_accuracy_score, accurate_explanations, total_explanations, updated_at)
VALUES (1, 0, 0, 0, strftime('%Y-%m-%d %H:%M:%S', 'now'));

CREATE TABLE IF NOT EXISTS workflow_forecast_feedback (
    feedback_id TEXT PRIMARY KEY NOT NULL,
    forecast_id TEXT NOT NULL,
    feedback_kind TEXT NOT NULL,
    notes TEXT,
    created_at TEXT NOT NULL,
    caller_role TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_wf_forecast_feedback_forecast
    ON workflow_forecast_feedback (forecast_id);

CREATE INDEX IF NOT EXISTS idx_wf_forecast_feedback_created
    ON workflow_forecast_feedback (datetime(created_at) DESC);

-- === V43__workflow_forecast_preventive_actions.sql ===
-- Preventive action recommendations, acknowledgments, effectiveness, and reliability scoring.

ALTER TABLE workflow_failure_forecast ADD COLUMN recommended_actions_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE workflow_failure_forecast ADD COLUMN action_priority TEXT NOT NULL DEFAULT 'MEDIUM';
ALTER TABLE workflow_failure_forecast ADD COLUMN action_effectiveness_score REAL;

CREATE TABLE IF NOT EXISTS workflow_forecast_action_log (
    log_id TEXT PRIMARY KEY NOT NULL,
    forecast_id TEXT NOT NULL,
    action_taken TEXT NOT NULL,
    acknowledged_at TEXT NOT NULL,
    caller_role TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_wf_forecast_action_log_forecast
    ON workflow_forecast_action_log (forecast_id);

CREATE INDEX IF NOT EXISTS idx_wf_forecast_action_log_time
    ON workflow_forecast_action_log (datetime(acknowledged_at) DESC);

CREATE TABLE IF NOT EXISTS workflow_forecast_action_metrics (
    metric_date TEXT PRIMARY KEY NOT NULL,
    actions_generated INTEGER NOT NULL DEFAULT 0,
    actions_acknowledged INTEGER NOT NULL DEFAULT 0,
    actions_effective INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS workflow_preventive_reliability_score (
    id INTEGER PRIMARY KEY CHECK (id = 1) NOT NULL,
    preventive_reliability_score REAL NOT NULL DEFAULT 0,
    prevented_failures INTEGER NOT NULL DEFAULT 0,
    total_forecasts INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL
);

INSERT OR IGNORE INTO workflow_preventive_reliability_score (id, preventive_reliability_score, prevented_failures, total_forecasts, updated_at)
VALUES (1, 0, 0, 0, strftime('%Y-%m-%d %H:%M:%S', 'now'));

-- === V44__workflow_forecast_action_log_unique_forecast.sql ===
-- At most one acknowledgment row per forecast (operational safety + idempotent ack).
DELETE FROM workflow_forecast_action_log
WHERE rowid NOT IN (
    SELECT MIN(rowid) FROM workflow_forecast_action_log GROUP BY forecast_id
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_wf_forecast_action_log_forecast_id
    ON workflow_forecast_action_log (forecast_id);

-- === V45__system_integrity_snapshot.sql ===
-- Daily high-level counts for operational validation (no prediction logic).
CREATE TABLE IF NOT EXISTS system_integrity_snapshot (
    snapshot_date TEXT PRIMARY KEY NOT NULL,
    details_json TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_system_integrity_snapshot_updated
    ON system_integrity_snapshot (datetime(updated_at) DESC);

-- === V46__ai_extraction_log.sql ===
-- V0.2.2: AI invoice extraction — audit log of extraction attempts (no app logic in this migration).

CREATE TABLE IF NOT EXISTS ai_extraction_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    file_hash TEXT NOT NULL,
    file_name TEXT NOT NULL,
    supplier_hint TEXT,
    provider_used TEXT NOT NULL,
    prompt_version TEXT NOT NULL,
    raw_ai_response TEXT,
    extracted_json TEXT,
    confidence_score REAL,
    status TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by TEXT
);

-- === V47__app_settings.sql ===
-- V0.2.3: generic key–value app settings (AI provider, etc.).

CREATE TABLE IF NOT EXISTS app_settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_app_settings_updated_at
  ON app_settings (updated_at);

-- === V48__performance_indexes.sql ===
-- V0.2.3: read-heavy query paths (duplicate check, supplier/item match, analytics)
CREATE INDEX IF NOT EXISTS idx_shipments_supplier_invoice
ON shipments (
  supplier_id,
  invoice_number,
  invoice_date
);

CREATE INDEX IF NOT EXISTS idx_suppliers_name
ON suppliers (
  supplier_name
);

CREATE INDEX IF NOT EXISTS idx_items_part_number
ON items (
  part_number
);

CREATE INDEX IF NOT EXISTS idx_ai_extraction_log_created
ON ai_extraction_log (
  created_at
);

-- === V49__remove_ai_features.sql ===
-- Remove deprecated AI extraction artifacts.
DROP INDEX IF EXISTS idx_ai_extraction_log_created;
DROP TABLE IF EXISTS ai_extraction_log;

-- Remove AI provider settings keys if they were persisted.
DELETE FROM app_settings
WHERE key IN (
  'ai_provider',
  'deepseek_api_key',
  'ollama_endpoint',
  'ollama_model'
);

-- === V50__performance_index_review.sql ===
-- Performance index review for invoice and shipment query paths.
-- Adds only missing high-impact indexes; existing FK indexes are already covered in V10.

CREATE INDEX IF NOT EXISTS idx_invoices_status
ON invoices(status);

CREATE INDEX IF NOT EXISTS idx_invoices_status_id
ON invoices(status, id);

CREATE INDEX IF NOT EXISTS idx_shipments_status
ON shipments(status);

CREATE INDEX IF NOT EXISTS idx_shipments_status_id
ON shipments(status, id);

-- === V51__shipment_query_performance.sql ===
-- Optimize get_unfinalized_shipments() filtering and ordering path.
CREATE INDEX IF NOT EXISTS idx_shipments_frozen_date
ON shipments(is_frozen, invoice_date DESC);

-- === V53__invoice_calculation_precision_settings.sql ===
INSERT INTO app_settings (key, value, updated_at)
VALUES ('line_total_decimals', '2', datetime('now'))
ON CONFLICT(key) DO NOTHING;

INSERT INTO app_settings (key, value, updated_at)
VALUES ('invoice_total_decimals', '2', datetime('now'))
ON CONFLICT(key) DO NOTHING;

-- === V54__invoice_precision_per_invoice.sql ===
ALTER TABLE invoices ADD COLUMN line_total_decimals INTEGER NOT NULL DEFAULT 2;
ALTER TABLE invoices ADD COLUMN invoice_total_decimals INTEGER NOT NULL DEFAULT 2;

-- === V56__boe_performance_and_attachments.sql ===
CREATE INDEX IF NOT EXISTS idx_boe_created_at
ON boe_calculations(created_at);

CREATE INDEX IF NOT EXISTS idx_boe_status
ON boe_calculations(status);

CREATE INDEX IF NOT EXISTS idx_boe_shipment_status
ON boe_calculations(shipment_id, status);

CREATE INDEX IF NOT EXISTS idx_boe_number
ON boe_details(be_number);

CREATE TABLE IF NOT EXISTS boe_attachments (
    id TEXT PRIMARY KEY NOT NULL,
    boe_calculation_id TEXT NOT NULL,
    file_name TEXT NOT NULL,
    file_path TEXT NOT NULL,
    uploaded_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    FOREIGN KEY (boe_calculation_id) REFERENCES boe_calculations(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_boe_attachments_calc_id
ON boe_attachments(boe_calculation_id);

-- === V57__boe_items_and_recovery.sql ===
CREATE TABLE IF NOT EXISTS boe_items (
    id TEXT PRIMARY KEY NOT NULL,
    boe_calculation_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    assessable_value REAL NOT NULL,
    bcd_rate REAL NOT NULL,
    sws_rate REAL NOT NULL,
    igst_rate REAL NOT NULL,
    total REAL NOT NULL,
    FOREIGN KEY (boe_calculation_id) REFERENCES boe_calculations(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_boe_items_calc_id
ON boe_items(boe_calculation_id);

CREATE TABLE IF NOT EXISTS boe_write_recovery (
    id TEXT PRIMARY KEY NOT NULL,
    boe_calculation_id TEXT,
    operation TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

CREATE INDEX IF NOT EXISTS idx_boe_write_recovery_status
ON boe_write_recovery(status);

-- === V58__platform_reliability_journal.sql ===
CREATE TABLE IF NOT EXISTS write_recovery_journal (
    id TEXT PRIMARY KEY NOT NULL,
    module TEXT NOT NULL,
    operation TEXT NOT NULL,
    entity_id TEXT,
    status TEXT NOT NULL,
    error_message TEXT,
    started_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    finished_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_write_recovery_journal_status
ON write_recovery_journal(status);

CREATE TABLE IF NOT EXISTS index_recommendation_history (
    id TEXT PRIMARY KEY NOT NULL,
    recommendation_sql TEXT NOT NULL,
    source_query TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

-- === V59__shipment_reliability_indexes.sql ===
-- Shipment module reliability and query optimization indexes.
ALTER TABLE shipments
ADD COLUMN created_at TEXT;

UPDATE shipments
SET created_at = COALESCE(
    created_at,
    strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
);

CREATE INDEX IF NOT EXISTS idx_shipments_created_at
ON shipments(created_at);

CREATE INDEX IF NOT EXISTS idx_shipments_status
ON shipments(status);

CREATE INDEX IF NOT EXISTS idx_shipments_supplier
ON shipments(supplier_id);

-- === V60__shipments_invoice_date_index.sql ===
CREATE INDEX IF NOT EXISTS idx_shipments_invoice_date
ON shipments(invoice_date);

-- === V61__shipment_predicate_index_completion.sql ===
CREATE INDEX IF NOT EXISTS idx_shipments_eta
ON shipments(eta);

CREATE INDEX IF NOT EXISTS idx_shipments_status_date
ON shipments(status, invoice_date);

CREATE INDEX IF NOT EXISTS idx_boe_calculations_shipment_id
ON boe_calculations(shipment_id);

CREATE INDEX IF NOT EXISTS idx_expenses_shipment_id
ON expenses(shipment_id);

-- === V62__shipment_date_hygiene_audit.sql ===
CREATE TABLE IF NOT EXISTS shipment_date_normalization_audit (
    id TEXT PRIMARY KEY,
    batch_id TEXT NOT NULL,
    shipment_id TEXT NOT NULL,
    old_invoice_date TEXT NOT NULL,
    new_invoice_date TEXT NOT NULL,
    old_eta TEXT,
    new_eta TEXT,
    snapshot_json TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

CREATE INDEX IF NOT EXISTS idx_shipment_date_norm_audit_batch
ON shipment_date_normalization_audit(batch_id);

CREATE INDEX IF NOT EXISTS idx_shipment_date_norm_audit_shipment
ON shipment_date_normalization_audit(shipment_id);

CREATE TABLE IF NOT EXISTS shipment_query_plan_baseline (
    id TEXT PRIMARY KEY,
    summary TEXT NOT NULL,
    findings_json TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

-- === V63__dashboard_invoice_month_expr_index.sql ===
-- Helps dashboard monthly grouping by invoice month expression.
CREATE INDEX IF NOT EXISTS idx_shipments_invoice_month_expr
ON shipments (strftime('%Y-%m', invoice_date));

-- === V64__dashboard_kpi_preaggregation.sql ===
CREATE TABLE IF NOT EXISTS dashboard_kpi_snapshot (
    snapshot_date TEXT PRIMARY KEY NOT NULL,
    total_shipments INTEGER NOT NULL DEFAULT 0,
    pending_shipments INTEGER NOT NULL DEFAULT 0,
    delivered_shipments INTEGER NOT NULL DEFAULT 0,
    overdue_eta_count INTEGER NOT NULL DEFAULT 0,
    shipments_missing_boe INTEGER NOT NULL DEFAULT 0,
    shipments_missing_expense INTEGER NOT NULL DEFAULT 0,
    total_invoice_value REAL NOT NULL DEFAULT 0,
    total_expense_value REAL NOT NULL DEFAULT 0,
    total_duty_savings REAL NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

CREATE TABLE IF NOT EXISTS dashboard_monthly_snapshot (
    snapshot_date TEXT NOT NULL,
    period TEXT NOT NULL,
    shipments INTEGER NOT NULL DEFAULT 0,
    value REAL NOT NULL DEFAULT 0,
    duty_savings REAL NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    PRIMARY KEY (snapshot_date, period)
);

CREATE INDEX IF NOT EXISTS idx_dashboard_monthly_snapshot_period
ON dashboard_monthly_snapshot (period);

-- === V65__dashboard_exception_snapshot.sql ===
CREATE TABLE IF NOT EXISTS dashboard_exception_snapshot (
    snapshot_date TEXT PRIMARY KEY NOT NULL,
    missing_boe_count INTEGER NOT NULL DEFAULT 0,
    missing_expense_count INTEGER NOT NULL DEFAULT 0,
    overdue_eta_count INTEGER NOT NULL DEFAULT 0,
    open_exception_count INTEGER NOT NULL DEFAULT 0,
    sla_breach_count INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

CREATE TABLE IF NOT EXISTS dashboard_exception_sample_cache (
    snapshot_date TEXT NOT NULL,
    sample_kind TEXT NOT NULL,
    shipment_ids_json TEXT NOT NULL DEFAULT '[]',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    PRIMARY KEY (snapshot_date, sample_kind)
);

-- === V66__dashboard_workflow_snapshot.sql ===
CREATE TABLE IF NOT EXISTS dashboard_workflow_snapshot (
    snapshot_date TEXT PRIMARY KEY NOT NULL,
    open_workflow_count INTEGER NOT NULL DEFAULT 0,
    active_workflow_count INTEGER NOT NULL DEFAULT 0,
    sla_breach_count INTEGER NOT NULL DEFAULT 0,
    recent_incident_count INTEGER NOT NULL DEFAULT 0,
    resolved_today_count INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

CREATE TABLE IF NOT EXISTS dashboard_workflow_sample_cache (
    snapshot_date TEXT PRIMARY KEY NOT NULL,
    workflow_ids_json TEXT NOT NULL DEFAULT '[]',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
);

-- === V67__shipment_import_log.sql ===
CREATE TABLE IF NOT EXISTS shipment_import_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_name TEXT NOT NULL,
    total_rows INTEGER NOT NULL,
    inserted_rows INTEGER NOT NULL,
    skipped_rows INTEGER NOT NULL,
    error_rows INTEGER NOT NULL,
    status TEXT NOT NULL,
    created_at TEXT NOT NULL
);

-- === V68__boe_calculations_form_values_json_indexes.sql ===
-- Optimize json_extract() / JSON filter paths on boe_calculations.form_values_json.
-- Stored JSON keys match frontend / serde camelCase (e.g. supplierName, shipmentId).
-- invoiceNumber and status are not present in current FormValues JSON; columns remain
-- NULL until present in payloads, without changing the JSON schema.
--
-- Query plan validation (SQLite 3.46+, table analyzed, ~5k rows):
--   EXPLAIN QUERY PLAN SELECT id FROM boe_calculations
--     WHERE supplier_name_generated = ?;
--   --> SEARCH boe_calculations USING INDEX idx_boe_calc_form_supplier_name_gen (supplier_name_generated=?)
--   EXPLAIN QUERY PLAN SELECT id FROM boe_calculations
--     WHERE shipment_id_generated = ?;
--   --> SEARCH boe_calculations USING INDEX idx_boe_calc_form_shipment_id_gen (shipment_id_generated=?)

ALTER TABLE boe_calculations ADD COLUMN supplier_name_generated TEXT
    GENERATED ALWAYS AS (json_extract(form_values_json, '$.supplierName')) STORED;

ALTER TABLE boe_calculations ADD COLUMN invoice_number_generated TEXT
    GENERATED ALWAYS AS (json_extract(form_values_json, '$.invoiceNumber')) STORED;

ALTER TABLE boe_calculations ADD COLUMN status_generated TEXT
    GENERATED ALWAYS AS (json_extract(form_values_json, '$.status')) STORED;

ALTER TABLE boe_calculations ADD COLUMN shipment_id_generated TEXT
    GENERATED ALWAYS AS (json_extract(form_values_json, '$.shipmentId')) STORED;

CREATE INDEX IF NOT EXISTS idx_boe_calc_form_supplier_name_gen
    ON boe_calculations(supplier_name_generated);

CREATE INDEX IF NOT EXISTS idx_boe_calc_form_invoice_number_gen
    ON boe_calculations(invoice_number_generated);

CREATE INDEX IF NOT EXISTS idx_boe_calc_form_status_gen
    ON boe_calculations(status_generated);

CREATE INDEX IF NOT EXISTS idx_boe_calc_form_shipment_id_gen
    ON boe_calculations(shipment_id_generated);

-- === V69__dashboard_kpi_snapshot_extended_metrics.sql ===
-- Precomputed heavy dashboard aggregates (duty JOIN, avg transit, directory counts).
-- NULL extended columns mark rows written before this migration; readers force regeneration.

ALTER TABLE dashboard_kpi_snapshot ADD COLUMN reconciled_boes INTEGER;
ALTER TABLE dashboard_kpi_snapshot ADD COLUMN duty_total REAL;
ALTER TABLE dashboard_kpi_snapshot ADD COLUMN avg_transit_days REAL;
ALTER TABLE dashboard_kpi_snapshot ADD COLUMN shipments_missing_eta INTEGER;
ALTER TABLE dashboard_kpi_snapshot ADD COLUMN shipments_missing_etd INTEGER;
ALTER TABLE dashboard_kpi_snapshot ADD COLUMN total_suppliers INTEGER;
ALTER TABLE dashboard_kpi_snapshot ADD COLUMN total_items INTEGER;

-- === V70__query_path_index_audit.sql ===
-- Query-path index audit: composites for report/dashboard/BOE/expense hot paths.
-- See dashboard_metrics (report_view duty join, monthly JSON BOE), reports.rs (report_view),
-- shipment aggregates (supplier + date), invoice_line_items joins.
--
-- EXPLAIN validation (representative predicates, SQLite 3.46+):
--   boe_calculations: SEARCH USING COVERING INDEX idx_boe_calculations_shipment_id_status
--     (shipment_id=? AND status=?)
--   invoice_line_items: SEARCH USING COVERING INDEX idx_invoice_line_items_invoice_id_item_id
--     (invoice_id=? AND item_id=?)
--   shipments: SEARCH USING INDEX idx_shipments_supplier_id_invoice_date
--     (supplier_id=? AND invoice_date>? AND invoice_date<?)

-- Invoice line composite: matches JOIN invoice_line_items ON (invoice_id, item_id)
CREATE INDEX IF NOT EXISTS idx_invoice_line_items_invoice_id_item_id
ON invoice_line_items (invoice_id, item_id);

-- BOE: correlated subqueries / anti-joins on shipment_id + status (e.g. Reconciled JSON extract)
CREATE INDEX IF NOT EXISTS idx_boe_calculations_shipment_id_status
ON boe_calculations (shipment_id, status);

-- Shipments: supplier-scoped date-range filters (dashboard KPI snapshot, paginated lists)
CREATE INDEX IF NOT EXISTS idx_shipments_supplier_id_invoice_date
ON shipments (supplier_id, invoice_date);

-- Duplicate index cleanup: same column as idx_shipments_supplier_id (V10); keep FK-style name.
DROP INDEX IF EXISTS idx_shipments_supplier;

-- === V71__user_activity_audit_logs.sql ===
-- V71__user_activity_audit_logs.sql
CREATE TABLE IF NOT EXISTS user_activity_audit_logs (
    id TEXT PRIMARY KEY NOT NULL,
    user_id TEXT,
    action_name TEXT NOT NULL,
    entity_type TEXT,
    entity_id TEXT,
    details_json TEXT,
    status TEXT NOT NULL,
    timestamp TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_user_activity_timestamp ON user_activity_audit_logs(timestamp);
CREATE INDEX IF NOT EXISTS idx_user_activity_entity ON user_activity_audit_logs(entity_type, entity_id);

-- === V72__rbac_security_hardening.sql ===
-- V72__rbac_security_hardening.sql
-- Role/permission and authentication hardening.
--   * Reconciles `user_roles` schema drift (`created_by`, `updated_by` columns).
--   * Adds tables for failed login attempts and admin password history.
--   * Seeds default security policy values into `app_settings`.
--   * Role-name normalization (admin -> administrator etc.) is performed in Rust
--     post-refinery (see `migrations::normalize_user_roles_canonical`).

-- Idempotent column additions for `user_roles`. SQLite has no IF NOT EXISTS for
-- ALTER TABLE; Rust `post_refinery_migrations` re-adds these defensively if the
-- column ever drifts.
ALTER TABLE user_roles ADD COLUMN created_by TEXT;
ALTER TABLE user_roles ADD COLUMN updated_by TEXT;

CREATE TABLE IF NOT EXISTS auth_failed_attempts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    attempted_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    reason TEXT,
    source TEXT
);

CREATE INDEX IF NOT EXISTS idx_auth_failed_attempts_user_time
    ON auth_failed_attempts (user_id, attempted_at);

CREATE TABLE IF NOT EXISTS auth_password_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    algorithm TEXT NOT NULL,
    replaced_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_auth_password_history_user
    ON auth_password_history (user_id, replaced_at);

INSERT OR IGNORE INTO app_settings (key, value, updated_at) VALUES
    ('auth.idle_timeout_minutes',    '30', datetime('now')),
    ('auth.lockout_threshold',       '5',  datetime('now')),
    ('auth.lockout_window_minutes',  '15', datetime('now')),
    ('auth.lockout_duration_minutes','30', datetime('now'));

-- === V73__enterprise_security_depth.sql ===
-- V73__enterprise_security_depth.sql — v0.3.2 enterprise security depth
-- Persistent lockout, policy versioning, password history depth setting,
-- user activity severity classification.

CREATE TABLE IF NOT EXISTS auth_lockout_state (
    user_id TEXT PRIMARY KEY NOT NULL,
    failure_count INTEGER NOT NULL DEFAULT 0,
    window_started_at TEXT NOT NULL,
    locked_until TEXT,
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_auth_lockout_state_locked_until
    ON auth_lockout_state (locked_until);

CREATE TABLE IF NOT EXISTS security_policy_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    version INTEGER NOT NULL UNIQUE,
    policy_json TEXT NOT NULL,
    password_rules_json TEXT,
    changed_by TEXT,
    changed_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_security_policy_versions_changed_at
    ON security_policy_versions (changed_at DESC);

-- Classify security-relevant audit rows (INFO | WARNING | SECURITY | CRITICAL)
ALTER TABLE user_activity_audit_logs ADD COLUMN severity TEXT NOT NULL DEFAULT 'INFO';

INSERT OR IGNORE INTO app_settings (key, value, updated_at) VALUES
    ('auth.password_history_depth', '5', datetime('now')),
    ('security.policy_version', '0', datetime('now'));

-- === V74__backup_validation_columns.sql ===
-- Post-backup integrity validation metadata (local encrypted snapshots).
ALTER TABLE backups ADD COLUMN validation_status TEXT;
ALTER TABLE backups ADD COLUMN validation_checked_at TEXT;
ALTER TABLE backups ADD COLUMN validation_message TEXT;

-- === V75__backup_restore_simulation.sql ===
-- Restore simulation / periodic restorability test results (local backups).
ALTER TABLE backups ADD COLUMN restore_simulation_status TEXT;
ALTER TABLE backups ADD COLUMN restore_simulation_checked_at TEXT;
ALTER TABLE backups ADD COLUMN restore_simulation_message TEXT;

-- === V76__system_agent_audit_log.sql ===
-- System agent audit log and default settings.

CREATE TABLE IF NOT EXISTS system_agent_audit_log (
    audit_id TEXT PRIMARY KEY NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    session_id TEXT NOT NULL DEFAULT '',
    caller_user_id TEXT NOT NULL DEFAULT '',
    caller_role TEXT NOT NULL DEFAULT '',
    intent_route TEXT NOT NULL DEFAULT '',
    matched_rule_id TEXT NOT NULL DEFAULT '',
    llm_used INTEGER NOT NULL DEFAULT 0,
    policy_decision_json TEXT NOT NULL DEFAULT '{}',
    limiter_bucket TEXT NOT NULL DEFAULT '',
    was_blocked INTEGER NOT NULL DEFAULT 0,
    grounding_ok INTEGER,
    tools_attempted_json TEXT NOT NULL DEFAULT '[]',
    tools_executed_json TEXT NOT NULL DEFAULT '[]',
    confidence REAL,
    trace_checksum TEXT NOT NULL DEFAULT '',
    snapshot_checksum TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_system_agent_audit_created_at
    ON system_agent_audit_log (datetime(created_at) DESC);

CREATE INDEX IF NOT EXISTS idx_system_agent_audit_route
    ON system_agent_audit_log (intent_route, datetime(created_at) DESC);

INSERT OR IGNORE INTO app_settings (key, value, updated_at) VALUES ('system_agent_enabled', '0', datetime('now'));
INSERT OR IGNORE INTO app_settings (key, value, updated_at) VALUES ('system_agent_deepseek_model', 'deepseek-chat', datetime('now'));
INSERT OR IGNORE INTO app_settings (key, value, updated_at) VALUES ('system_agent_deepseek_base_url', 'https://api.deepseek.com/chat/completions', datetime('now'));
INSERT OR IGNORE INTO app_settings (key, value, updated_at) VALUES ('system_agent_confidence_threshold_mutate', '0.75', datetime('now'));
INSERT OR IGNORE INTO app_settings (key, value, updated_at) VALUES ('system_agent_max_llm_calls_per_day', '250', datetime('now'));
INSERT OR IGNORE INTO app_settings (key, value, updated_at) VALUES ('system_agent_max_llm_calls_per_session', '30', datetime('now'));

PRAGMA user_version = 76;

-- === V77__bug_notes.sql ===
-- In-app bug tracker (developer tool): notes + optional screenshot path + JSON context/meta.

CREATE TABLE IF NOT EXISTS bug_notes (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  description TEXT,
  status TEXT NOT NULL CHECK (status IN ('OPEN','SOLVED')) DEFAULT 'OPEN',
  screenshot_path TEXT,
  context TEXT,
  meta TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_bug_notes_updated_at_desc ON bug_notes(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_bug_notes_status ON bug_notes(status);

PRAGMA user_version = 77;

-- === V78__error_memory.sql ===
CREATE TABLE IF NOT EXISTS error_memory (
    id TEXT PRIMARY KEY,
    fingerprint TEXT NOT NULL UNIQUE,
    occurrence_count INTEGER NOT NULL DEFAULT 1,
    first_seen_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    app_version TEXT,
    build_version TEXT,
    environment TEXT,
    module_name TEXT,
    command_name TEXT,
    page_name TEXT,
    component_name TEXT,
    error_code TEXT,
    error_category TEXT,
    error_message TEXT NOT NULL,
    stack_trace TEXT,
    source_file TEXT,
    source_function TEXT,
    user_action TEXT,
    redacted_input_context TEXT,
    affected_entity_ids TEXT,
    severity TEXT NOT NULL DEFAULT 'error',
    recoverable INTEGER NOT NULL DEFAULT 0,
    retryable INTEGER NOT NULL DEFAULT 0,
    app_state_snapshot TEXT,
    status TEXT NOT NULL DEFAULT 'new',
    ai_summary TEXT
);

CREATE INDEX IF NOT EXISTS idx_error_memory_last_seen_desc
    ON error_memory (datetime(last_seen_at) DESC);
CREATE INDEX IF NOT EXISTS idx_error_memory_status
    ON error_memory (status);
CREATE INDEX IF NOT EXISTS idx_error_memory_severity
    ON error_memory (severity);
CREATE INDEX IF NOT EXISTS idx_error_memory_module
    ON error_memory (module_name);
CREATE INDEX IF NOT EXISTS idx_error_memory_command
    ON error_memory (command_name);

-- === V79__error_memory_cleanup.sql ===
CREATE TABLE IF NOT EXISTS error_memory_pruned_groups (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    occurrence_count INTEGER NOT NULL DEFAULT 1,
    first_seen_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    severity TEXT NOT NULL,
    status TEXT NOT NULL,
    prune_reason TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_error_memory_pruned_groups_run
    ON error_memory_pruned_groups (run_id);
CREATE INDEX IF NOT EXISTS idx_error_memory_pruned_groups_fingerprint
    ON error_memory_pruned_groups (fingerprint);
CREATE INDEX IF NOT EXISTS idx_error_memory_pruned_groups_created_at
    ON error_memory_pruned_groups (datetime(created_at) DESC);

CREATE TABLE IF NOT EXISTS error_memory_cleanup_runs (
    id TEXT PRIMARY KEY,
    executed_at TEXT NOT NULL,
    trigger_source TEXT NOT NULL,
    dry_run INTEGER NOT NULL DEFAULT 0,
    total_before INTEGER NOT NULL DEFAULT 0,
    total_after INTEGER NOT NULL DEFAULT 0,
    candidate_count INTEGER NOT NULL DEFAULT 0,
    deleted_count INTEGER NOT NULL DEFAULT 0,
    protected_count INTEGER NOT NULL DEFAULT 0,
    error_message TEXT
);

CREATE INDEX IF NOT EXISTS idx_error_memory_cleanup_runs_executed
    ON error_memory_cleanup_runs (datetime(executed_at) DESC);
