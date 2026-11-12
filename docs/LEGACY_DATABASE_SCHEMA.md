# Legacy Database Schema Reference – Import Manager

> **Source of Truth:** `src-tauri/migrations/V1..V80`, `src-tauri/src/migrations.rs::post_refinery_migrations`, `src-tauri/src/db.rs::init_schema`
> **Introspected Object Count:** **155 Tables + 1 View (`report_view`) = 156 Objects** (+ SQLite internal `refinery_schema_history`)

---

## 1. Three-Layer Schema Initialization Architecture (Legacy)

1. **Layer 1 – Versioned Migrations (`src-tauri/migrations/V1..V80`):** Executed by Rust `refinery` tracking versions in `refinery_schema_history`. Note that **77 of 80** apply cleanly on blank SQLite; **3 migrations** (`V52`, `V55`, `V80`) fail on a fresh SQLite DB without pre-running Layer 2/3 runtime DDL:
   - `V52__supplier_pagination_indexes.sql`: expects `deleted_at` column added by Layer 2.
   - `V55__db_management_performance_indexes.sql`: expects `deleted_at` column added by Layer 2.
   - `V80__backfill_expense_paise_from_decimal.sql`: expects `amount_paise` column added by Layer 3.
2. **Layer 2 – Post-Refinery Runtime DDL (`migrations.rs::post_refinery_migrations`):** Dynamically executes `ALTER TABLE ADD COLUMN` idempotently:
   - Adds `deleted_at TEXT` and `deleted_by TEXT` soft-delete columns across **11 core tables** (`suppliers`, `items`, `shipments`, `invoices`, `invoice_line_items`, `boe_details`, `boe_calculations`, `expenses`, `expense_invoices`, `service_providers`, `expense_types`).
   - Adds `duty_percent`, `sws_percent`, `igst_percent` on `invoice_line_items`.
   - Adds compatibility column `"tableName"` on `audit_logs` and `{created_by, updated_by}` on `user_roles` + role canonicalization (`admin` -> `administrator`, etc.).
   - Adds `severity` column on `user_activity_audit_logs`.
3. **Layer 3 – Application Startup Schema & Seeds (`db.rs::init_schema`):**
   - Adds integer paise/basis-point & concurrency columns on `expenses` and `expense_invoices` (`amount_paise`, `cgst_rate_bp`, `sgst_rate_bp`, `igst_rate_bp`, `tds_rate_bp`, `cgst_amount_paise`, `sgst_amount_paise`, `igst_amount_paise`, `tds_amount_paise`, `total_amount_paise`, `net_amount_paise`, `invoice_number`, `total_cgst_paise`, `total_sgst_paise`, `total_igst_paise`, `total_tds_paise`, `net_payable_paise`, `currency`, `idempotency_key`, `version`).
   - Recreates `report_view` for Landed Cost calculation (`SUM(e.amount)` basic expenses allocated proportionally by `boe_assessable_value / shipment_boe_assessable_total`, `ldc_per_qty = (assessable_value + bcd_amount + sws_amount + allocated_basic_expenses) / qty`).
   - Seeds baseline master suppliers (`Sup-001` .. `Sup-011`) and lookup defaults.

---

## 2. Migration History (`V1` – `V80`)

| Ver | File |
| :--- | :--- |
| `V1` | `V1__initial_schema.sql` |
| `V2` | `V2__notifications.sql` |
| `V3` | `V3__notifications.sql` |
| `V4` | `V4__db_management.sql` |
| `V5` | `V5__invoice_line_item_tax_rates.sql` |
| `V6` | `V6__audit_logs_tableName.sql` |
| `V7` | `V7__app_metadata.sql` |
| `V8` | `V8__app_metadata_counts.sql` |
| `V9` | `V9__app_metadata_last_ops.sql` |
| `V10` | `V10__add_missing_fk_indexes.sql` |
| `V11` | `V11__app_metadata_recycle_retention.sql` |
| `V12` | `V12__app_metadata_last_database_maintenance.sql` |
| `V13` | `V13__kpi_governance.sql` |
| `V14` | `V14__kpi_duty_metadata_update.sql` |
| `V15` | `V15__dashboard_erp_cache_alerts.sql` |
| `V16` | `V16__dashboard_activity_erp_fields.sql` |
| `V17` | `V17__exception_workflow.sql` |
| `V18` | `V18__workflow_reliability.sql` |
| `V19` | `V19__workflow_observability.sql` |
| `V20` | `V20__workflow_automation.sql` |
| `V21` | `V21__workflow_rule_change_log.sql` |
| `V22` | `V22__automation_optimization_intelligence.sql` |
| `V23` | `V23__automation_cost_intelligence.sql` |
| `V24` | `V24__workflow_rule_deployment.sql` |
| `V25` | `V25__multi_environment_tenant.sql` |
| `V26` | `V26__deployment_safety.sql` |
| `V27` | `V27__workflow_background_jobs.sql` |
| `V28` | `V28__workflow_job_schedule_recovery.sql` |
| `V29` | `V29__workflow_job_manual_governance.sql` |
| `V30` | `V30__workflow_production_observability.sql` |
| `V31` | `V31__workflow_incident_management.sql` |
| `V32` | `V32__workflow_incident_engine.sql` |
| `V33` | `V33__workflow_incident_correlation.sql` |
| `V34` | `V34__workflow_failure_burst_log.sql` |
| `V35` | `V35__workflow_incident_unified_intelligence.sql` |
| `V36` | `V36__workflow_incident_suppression.sql` |
| `V37` | `V37__workflow_incident_stabilization.sql` |
| `V38` | `V38__workflow_incident_regression.sql` |
| `V39` | `V39__workflow_structured_regression_metrics.sql` |
| `V40` | `V40__workflow_incident_persistence.sql` |
| `V41` | `V41__workflow_failure_forecast.sql` |
| `V42` | `V42__workflow_forecast_explainability.sql` |
| `V43` | `V43__workflow_forecast_preventive_actions.sql` |
| `V44` | `V44__workflow_forecast_action_log_unique_forecast.sql` |
| `V45` | `V45__system_integrity_snapshot.sql` |
| `V46` | `V46__ai_extraction_log.sql` |
| `V47` | `V47__app_settings.sql` |
| `V48` | `V48__performance_indexes.sql` |
| `V49` | `V49__remove_ai_features.sql` |
| `V50` | `V50__performance_index_review.sql` |
| `V51` | `V51__shipment_query_performance.sql` |
| `V52` | `V52__supplier_pagination_indexes.sql` |
| `V53` | `V53__invoice_calculation_precision_settings.sql` |
| `V54` | `V54__invoice_precision_per_invoice.sql` |
| `V55` | `V55__db_management_performance_indexes.sql` |
| `V56` | `V56__boe_performance_and_attachments.sql` |
| `V57` | `V57__boe_items_and_recovery.sql` |
| `V58` | `V58__platform_reliability_journal.sql` |
| `V59` | `V59__shipment_reliability_indexes.sql` |
| `V60` | `V60__shipments_invoice_date_index.sql` |
| `V61` | `V61__shipment_predicate_index_completion.sql` |
| `V62` | `V62__shipment_date_hygiene_audit.sql` |
| `V63` | `V63__dashboard_invoice_month_expr_index.sql` |
| `V64` | `V64__dashboard_kpi_preaggregation.sql` |
| `V65` | `V65__dashboard_exception_snapshot.sql` |
| `V66` | `V66__dashboard_workflow_snapshot.sql` |
| `V67` | `V67__shipment_import_log.sql` |
| `V68` | `V68__boe_calculations_form_values_json_indexes.sql` |
| `V69` | `V69__dashboard_kpi_snapshot_extended_metrics.sql` |
| `V70` | `V70__query_path_index_audit.sql` |
| `V71` | `V71__user_activity_audit_logs.sql` |
| `V72` | `V72__rbac_security_hardening.sql` |
| `V73` | `V73__enterprise_security_depth.sql` |
| `V74` | `V74__backup_validation_columns.sql` |
| `V75` | `V75__backup_restore_simulation.sql` |
| `V76` | `V76__system_agent_audit_log.sql` |
| `V77` | `V77__bug_notes.sql` |
| `V78` | `V78__error_memory.sql` |
| `V79` | `V79__error_memory_cleanup.sql` |
| `V80` | `V80__backfill_expense_paise_from_decimal.sql` |

---

## 3. Domain-Grouped Schema Catalog (156 Introspected Objects)

> Exact SQLite column names, types, primary keys, foreign keys, and indexes introspected across `V1..V80`.

> **Note on Runtime Columns:** Columns appended outside `V1..V80` (Section 1 Layers 2 & 3, e.g., `deleted_at`/`deleted_by` on 11 tables and `*_paise`/`*_bp` columns on `expenses`/`expense_invoices`) exist dynamically in live legacy DBs and are verified via `PRAGMA table_info` during legacy import.

## Domain 1. Master Data (Suppliers & Items) (2 objects)

### TABLE: `items` (20 cols, 1 FKs, 4 indexes)
  - `id` `TEXT` PK
  - `part_number` `TEXT` NOT NULL
  - `item_description` `TEXT` NOT NULL
  - `unit` `TEXT` NOT NULL
  - `currency` `TEXT` NOT NULL
  - `unit_price` `REAL` NOT NULL
  - `hsn_code` `TEXT` NOT NULL
  - `supplier_id` `TEXT` 
  - `is_active` `BOOLEAN` NOT NULL
  - `country_of_origin` `TEXT` 
  - `bcd` `TEXT` 
  - `sws` `TEXT` 
  - `igst` `TEXT` 
  - `technical_write_up` `TEXT` 
  - `category` `TEXT` 
  - `end_use` `TEXT` 
  - `net_weight_kg` `REAL` 
  - `purchase_uom` `TEXT` 
  - `gross_weight_per_uom_kg` `REAL` 
  - `photo_path` `TEXT` 
  - FK: `supplier_id` -> `suppliers(id)` (ON UPDATE NO ACTION, ON DELETE NO ACTION)
  - INDEX `idx_items_part_number` (cols=['part_number'])
  - INDEX `idx_items_supplier_id` (cols=['supplier_id'])
  - INDEX `sqlite_autoindex_items_2` (UNIQUE cols=['part_number'])

### TABLE: `suppliers` (14 cols, 0 FKs, 2 indexes)
  - `id` `TEXT` PK
  - `supplier_name` `TEXT` NOT NULL
  - `short_name` `TEXT` 
  - `country` `TEXT` NOT NULL
  - `email` `TEXT` NOT NULL
  - `phone` `TEXT` 
  - `beneficiary_name` `TEXT` 
  - `bank_name` `TEXT` 
  - `branch` `TEXT` 
  - `bank_address` `TEXT` 
  - `account_no` `TEXT` 
  - `iban` `TEXT` 
  - `swift_code` `TEXT` 
  - `is_active` `BOOLEAN` NOT NULL
  - INDEX `idx_suppliers_name` (cols=['supplier_name'])

## Domain 2. Lookup / Option Tables (13 objects)

### TABLE: `bcd_rates` (2 cols, 0 FKs, 2 indexes)
  - `value` `TEXT` PK NOT NULL
  - `label` `TEXT` NOT NULL
  - INDEX `sqlite_autoindex_bcd_rates_2` (UNIQUE cols=['label'])

### TABLE: `categories` (2 cols, 0 FKs, 2 indexes)
  - `value` `TEXT` PK NOT NULL
  - `label` `TEXT` NOT NULL
  - INDEX `sqlite_autoindex_categories_2` (UNIQUE cols=['label'])

### TABLE: `countries` (2 cols, 0 FKs, 2 indexes)
  - `value` `TEXT` PK NOT NULL
  - `label` `TEXT` NOT NULL
  - INDEX `sqlite_autoindex_countries_2` (UNIQUE cols=['label'])

### TABLE: `currencies` (2 cols, 0 FKs, 2 indexes)
  - `value` `TEXT` PK NOT NULL
  - `label` `TEXT` NOT NULL
  - INDEX `sqlite_autoindex_currencies_2` (UNIQUE cols=['label'])

### TABLE: `end_uses` (2 cols, 0 FKs, 2 indexes)
  - `value` `TEXT` PK NOT NULL
  - `label` `TEXT` NOT NULL
  - INDEX `sqlite_autoindex_end_uses_2` (UNIQUE cols=['label'])

### TABLE: `igst_rates` (2 cols, 0 FKs, 2 indexes)
  - `value` `TEXT` PK NOT NULL
  - `label` `TEXT` NOT NULL
  - INDEX `sqlite_autoindex_igst_rates_2` (UNIQUE cols=['label'])

### TABLE: `incoterms` (2 cols, 0 FKs, 2 indexes)
  - `value` `TEXT` PK NOT NULL
  - `label` `TEXT` NOT NULL
  - INDEX `sqlite_autoindex_incoterms_2` (UNIQUE cols=['label'])

### TABLE: `purchase_uoms` (2 cols, 0 FKs, 2 indexes)
  - `value` `TEXT` PK NOT NULL
  - `label` `TEXT` NOT NULL
  - INDEX `sqlite_autoindex_purchase_uoms_2` (UNIQUE cols=['label'])

### TABLE: `shipment_modes` (2 cols, 0 FKs, 2 indexes)
  - `value` `TEXT` PK NOT NULL
  - `label` `TEXT` NOT NULL
  - INDEX `sqlite_autoindex_shipment_modes_2` (UNIQUE cols=['label'])

### TABLE: `shipment_statuses` (2 cols, 0 FKs, 2 indexes)
  - `value` `TEXT` PK NOT NULL
  - `label` `TEXT` NOT NULL
  - INDEX `sqlite_autoindex_shipment_statuses_2` (UNIQUE cols=['label'])

### TABLE: `shipment_types` (2 cols, 0 FKs, 2 indexes)
  - `value` `TEXT` PK NOT NULL
  - `label` `TEXT` NOT NULL
  - INDEX `sqlite_autoindex_shipment_types_2` (UNIQUE cols=['label'])

### TABLE: `sws_rates` (2 cols, 0 FKs, 2 indexes)
  - `value` `TEXT` PK NOT NULL
  - `label` `TEXT` NOT NULL
  - INDEX `sqlite_autoindex_sws_rates_2` (UNIQUE cols=['label'])

### TABLE: `units` (2 cols, 0 FKs, 2 indexes)
  - `value` `TEXT` PK NOT NULL
  - `label` `TEXT` NOT NULL
  - INDEX `sqlite_autoindex_units_2` (UNIQUE cols=['label'])

## Domain 3. Shipments & Invoices (6 objects)

### TABLE: `invoice_line_items` (5 cols, 2 FKs, 4 indexes)
  - `id` `TEXT` PK NOT NULL
  - `invoice_id` `TEXT` NOT NULL
  - `item_id` `TEXT` NOT NULL
  - `quantity` `REAL` NOT NULL
  - `unit_price` `REAL` NOT NULL
  - FK: `item_id` -> `items(id)` (ON UPDATE NO ACTION, ON DELETE NO ACTION)
  - FK: `invoice_id` -> `invoices(id)` (ON UPDATE NO ACTION, ON DELETE CASCADE)
  - INDEX `idx_invoice_line_items_invoice_id_item_id` (cols=['invoice_id', 'item_id'])
  - INDEX `idx_invoice_line_items_invoice_id` (cols=['invoice_id'])
  - INDEX `idx_invoice_line_items_item_id` (cols=['item_id'])

### TABLE: `invoices` (5 cols, 1 FKs, 4 indexes)
  - `id` `TEXT` PK NOT NULL
  - `shipment_id` `TEXT` NOT NULL
  - `status` `TEXT` NOT NULL
  - `line_total_decimals` `INTEGER` NOT NULL DEFAULT 2
  - `invoice_total_decimals` `INTEGER` NOT NULL DEFAULT 2
  - FK: `shipment_id` -> `shipments(id)` (ON UPDATE NO ACTION, ON DELETE CASCADE)
  - INDEX `idx_invoices_status_id` (cols=['status', 'id'])
  - INDEX `idx_invoices_status` (cols=['status'])
  - INDEX `idx_invoices_shipment_id` (cols=['shipment_id'])

### TABLE: `shipment_date_normalization_audit` (9 cols, 0 FKs, 3 indexes)
  - `id` `TEXT` PK
  - `batch_id` `TEXT` NOT NULL
  - `shipment_id` `TEXT` NOT NULL
  - `old_invoice_date` `TEXT` NOT NULL
  - `new_invoice_date` `TEXT` NOT NULL
  - `old_eta` `TEXT` 
  - `new_eta` `TEXT` 
  - `snapshot_json` `TEXT` NOT NULL
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - INDEX `idx_shipment_date_norm_audit_shipment` (cols=['shipment_id'])
  - INDEX `idx_shipment_date_norm_audit_batch` (cols=['batch_id'])

### TABLE: `shipment_import_log` (8 cols, 0 FKs, 0 indexes)
  - `id` `INTEGER` PK
  - `file_name` `TEXT` NOT NULL
  - `total_rows` `INTEGER` NOT NULL
  - `inserted_rows` `INTEGER` NOT NULL
  - `skipped_rows` `INTEGER` NOT NULL
  - `error_rows` `INTEGER` NOT NULL
  - `status` `TEXT` NOT NULL
  - `created_at` `TEXT` NOT NULL

### TABLE: `shipment_query_plan_baseline` (4 cols, 0 FKs, 1 indexes)
  - `id` `TEXT` PK
  - `summary` `TEXT` NOT NULL
  - `findings_json` `TEXT` NOT NULL
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')

### TABLE: `shipments` (21 cols, 1 FKs, 12 indexes)
  - `id` `TEXT` PK
  - `supplier_id` `TEXT` NOT NULL
  - `invoice_number` `TEXT` NOT NULL
  - `invoice_date` `TEXT` NOT NULL
  - `goods_category` `TEXT` NOT NULL
  - `invoice_value` `REAL` NOT NULL
  - `invoice_currency` `TEXT` NOT NULL
  - `incoterm` `TEXT` NOT NULL
  - `shipment_mode` `TEXT` 
  - `shipment_type` `TEXT` 
  - `bl_awb_number` `TEXT` 
  - `bl_awb_date` `TEXT` 
  - `vessel_name` `TEXT` 
  - `container_number` `TEXT` 
  - `gross_weight_kg` `REAL` 
  - `etd` `TEXT` 
  - `eta` `TEXT` 
  - `status` `TEXT` 
  - `date_of_delivery` `TEXT` 
  - `is_frozen` `BOOLEAN` NOT NULL DEFAULT 0
  - `created_at` `TEXT` 
  - FK: `supplier_id` -> `suppliers(id)` (ON UPDATE NO ACTION, ON DELETE NO ACTION)
  - INDEX `idx_shipments_supplier_id_invoice_date` (cols=['supplier_id', 'invoice_date'])
  - INDEX `idx_shipments_invoice_month_expr` (cols=[None])
  - INDEX `idx_shipments_status_date` (cols=['status', 'invoice_date'])
  - INDEX `idx_shipments_eta` (cols=['eta'])
  - INDEX `idx_shipments_invoice_date` (cols=['invoice_date'])
  - INDEX `idx_shipments_created_at` (cols=['created_at'])
  - INDEX `idx_shipments_frozen_date` (cols=['is_frozen', 'invoice_date'])
  - INDEX `idx_shipments_status_id` (cols=['status', 'id'])
  - INDEX `idx_shipments_status` (cols=['status'])
  - INDEX `idx_shipments_supplier_invoice` (cols=['supplier_id', 'invoice_number', 'invoice_date'])
  - INDEX `idx_shipments_supplier_id` (cols=['supplier_id'])

## Domain 4. Bill of Entry (BOE) & Duty Calculations (5 objects)

### TABLE: `boe_attachments` (5 cols, 1 FKs, 2 indexes)
  - `id` `TEXT` PK NOT NULL
  - `boe_calculation_id` `TEXT` NOT NULL
  - `file_name` `TEXT` NOT NULL
  - `file_path` `TEXT` NOT NULL
  - `uploaded_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - FK: `boe_calculation_id` -> `boe_calculations(id)` (ON UPDATE NO ACTION, ON DELETE CASCADE)
  - INDEX `idx_boe_attachments_calc_id` (cols=['boe_calculation_id'])

### TABLE: `boe_calculations` (11 cols, 1 FKs, 10 indexes)
  - `id` `TEXT` PK NOT NULL
  - `shipment_id` `TEXT` NOT NULL
  - `boe_id` `TEXT` 
  - `supplier_name` `TEXT` NOT NULL
  - `invoice_number` `TEXT` NOT NULL
  - `status` `TEXT` NOT NULL DEFAULT 'Awaiting BOE Data'
  - `form_values_json` `TEXT` NOT NULL
  - `item_inputs_json` `TEXT` NOT NULL
  - `calculation_result_json` `TEXT` NOT NULL
  - `attachments_json` `TEXT` 
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - FK: `shipment_id` -> `shipments(id)` (ON UPDATE NO ACTION, ON DELETE CASCADE)
  - INDEX `idx_boe_calculations_shipment_id_status` (cols=['shipment_id', 'status'])
  - INDEX `idx_boe_calc_form_shipment_id_gen` (cols=['shipment_id_generated'])
  - INDEX `idx_boe_calc_form_status_gen` (cols=['status_generated'])
  - INDEX `idx_boe_calc_form_invoice_number_gen` (cols=['invoice_number_generated'])
  - INDEX `idx_boe_calc_form_supplier_name_gen` (cols=['supplier_name_generated'])
  - INDEX `idx_boe_shipment_status` (cols=['shipment_id', 'status'])
  - INDEX `idx_boe_status` (cols=['status'])
  - INDEX `idx_boe_created_at` (cols=['created_at'])
  - INDEX `idx_boe_calculations_shipment_id` (cols=['shipment_id'])

### TABLE: `boe_details` (11 cols, 0 FKs, 3 indexes)
  - `id` `TEXT` PK NOT NULL
  - `be_number` `TEXT` NOT NULL
  - `be_date` `TEXT` NOT NULL
  - `location` `TEXT` NOT NULL
  - `total_assessment_value` `REAL` NOT NULL
  - `duty_amount` `REAL` NOT NULL
  - `payment_date` `TEXT` 
  - `duty_paid` `REAL` 
  - `challan_number` `TEXT` 
  - `ref_id` `TEXT` 
  - `transaction_id` `TEXT` 
  - INDEX `idx_boe_number` (cols=['be_number'])
  - INDEX `sqlite_autoindex_boe_details_2` (UNIQUE cols=['be_number', 'be_date'])

### TABLE: `boe_items` (8 cols, 1 FKs, 2 indexes)
  - `id` `TEXT` PK NOT NULL
  - `boe_calculation_id` `TEXT` NOT NULL
  - `item_id` `TEXT` NOT NULL
  - `assessable_value` `REAL` NOT NULL
  - `bcd_rate` `REAL` NOT NULL
  - `sws_rate` `REAL` NOT NULL
  - `igst_rate` `REAL` NOT NULL
  - `total` `REAL` NOT NULL
  - FK: `boe_calculation_id` -> `boe_calculations(id)` (ON UPDATE NO ACTION, ON DELETE CASCADE)
  - INDEX `idx_boe_items_calc_id` (cols=['boe_calculation_id'])

### TABLE: `boe_write_recovery` (5 cols, 0 FKs, 2 indexes)
  - `id` `TEXT` PK NOT NULL
  - `boe_calculation_id` `TEXT` 
  - `operation` `TEXT` NOT NULL
  - `status` `TEXT` NOT NULL
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - INDEX `idx_boe_write_recovery_status` (cols=['status'])

## Domain 5. Expenses & Service Providers (5 objects)

### TABLE: `expense_attachments` (7 cols, 1 FKs, 2 indexes)
  - `id` `TEXT` PK
  - `expense_id` `TEXT` NOT NULL
  - `file_name` `TEXT` NOT NULL
  - `file_path` `TEXT` NOT NULL
  - `file_type` `TEXT` 
  - `uploaded_at` `TIMESTAMP` DEFAULT CURRENT_TIMESTAMP
  - `uploaded_by` `TEXT` 
  - FK: `expense_id` -> `expenses(id)` (ON UPDATE NO ACTION, ON DELETE NO ACTION)
  - INDEX `idx_expense_attachments_expense_id` (cols=['expense_id'])

### TABLE: `expense_invoices` (13 cols, 2 FKs, 3 indexes)
  - `id` `TEXT` PK
  - `shipment_id` `TEXT` NOT NULL
  - `service_provider_id` `TEXT` NOT NULL
  - `invoice_no` `TEXT` NOT NULL
  - `invoice_date` `DATE` NOT NULL
  - `total_amount` `DECIMAL(12, 2)` NOT NULL
  - `total_cgst_amount` `DECIMAL(12, 2)` NOT NULL DEFAULT 0.00
  - `total_sgst_amount` `DECIMAL(12, 2)` NOT NULL DEFAULT 0.00
  - `total_igst_amount` `DECIMAL(12, 2)` NOT NULL DEFAULT 0.00
  - `remarks` `TEXT` 
  - `created_by` `TEXT` 
  - `created_at` `TIMESTAMP` DEFAULT CURRENT_TIMESTAMP
  - `updated_at` `TIMESTAMP` DEFAULT CURRENT_TIMESTAMP
  - FK: `service_provider_id` -> `service_providers(id)` (ON UPDATE NO ACTION, ON DELETE NO ACTION)
  - FK: `shipment_id` -> `shipments(id)` (ON UPDATE NO ACTION, ON DELETE NO ACTION)
  - INDEX `idx_expense_invoices_shipment_id` (cols=['shipment_id'])
  - INDEX `sqlite_autoindex_expense_invoices_2` (UNIQUE cols=['service_provider_id', 'invoice_no'])

### TABLE: `expense_types` (7 cols, 0 FKs, 2 indexes)
  - `id` `TEXT` PK
  - `name` `TEXT` NOT NULL
  - `default_cgst_rate` `DECIMAL(5, 2)` DEFAULT 0.00
  - `default_sgst_rate` `DECIMAL(5, 2)` DEFAULT 0.00
  - `default_igst_rate` `DECIMAL(5, 2)` DEFAULT 0.00
  - `is_active` `BOOLEAN` DEFAULT TRUE
  - `created_at` `TIMESTAMP` DEFAULT CURRENT_TIMESTAMP
  - INDEX `sqlite_autoindex_expense_types_2` (UNIQUE cols=['name'])

### TABLE: `expenses` (16 cols, 4 FKs, 5 indexes)
  - `id` `TEXT` PK
  - `expense_invoice_id` `TEXT` NOT NULL
  - `shipment_id` `TEXT` NOT NULL
  - `service_provider_id` `TEXT` NOT NULL
  - `invoice_no` `TEXT` NOT NULL
  - `invoice_date` `DATE` NOT NULL
  - `expense_type_id` `TEXT` NOT NULL
  - `amount` `DECIMAL(12, 2)` NOT NULL
  - `cgst_rate` `DECIMAL(5, 2)` DEFAULT 0.00
  - `sgst_rate` `DECIMAL(5, 2)` DEFAULT 0.00
  - `igst_rate` `DECIMAL(5, 2)` DEFAULT 0.00
  - `tds_rate` `DECIMAL(5, 2)` DEFAULT 0.00
  - `remarks` `TEXT` 
  - `created_by` `TEXT` 
  - `created_at` `TIMESTAMP` DEFAULT CURRENT_TIMESTAMP
  - `updated_at` `TIMESTAMP` DEFAULT CURRENT_TIMESTAMP
  - FK: `expense_type_id` -> `expense_types(id)` (ON UPDATE NO ACTION, ON DELETE NO ACTION)
  - FK: `service_provider_id` -> `service_providers(id)` (ON UPDATE NO ACTION, ON DELETE NO ACTION)
  - FK: `shipment_id` -> `shipments(id)` (ON UPDATE NO ACTION, ON DELETE NO ACTION)
  - FK: `expense_invoice_id` -> `expense_invoices(id)` (ON UPDATE NO ACTION, ON DELETE NO ACTION)
  - INDEX `idx_expenses_shipment_id` (cols=['shipment_id'])
  - INDEX `idx_expenses_service_provider_id` (cols=['service_provider_id'])
  - INDEX `idx_expenses_expense_type_id` (cols=['expense_type_id'])
  - INDEX `sqlite_autoindex_expenses_2` (UNIQUE cols=['expense_invoice_id', 'expense_type_id'])

### TABLE: `service_providers` (8 cols, 0 FKs, 2 indexes)
  - `id` `TEXT` PK
  - `name` `TEXT` NOT NULL
  - `gstin` `TEXT` 
  - `state` `TEXT` 
  - `contact_person` `TEXT` 
  - `contact_email` `TEXT` 
  - `contact_phone` `TEXT` 
  - `created_at` `TIMESTAMP` DEFAULT CURRENT_TIMESTAMP
  - INDEX `sqlite_autoindex_service_providers_2` (UNIQUE cols=['gstin'])

## Domain 6. Reporting Views & Compliance (2 objects)

### TABLE: `shipment_compliance_score` (5 cols, 1 FKs, 2 indexes)
  - `shipment_id` `TEXT` PK NOT NULL
  - `score` `REAL` NOT NULL
  - `required_slots` `INTEGER` NOT NULL
  - `satisfied_slots` `INTEGER` NOT NULL
  - `computed_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - FK: `shipment_id` -> `shipments(id)` (ON UPDATE NO ACTION, ON DELETE CASCADE)
  - INDEX `idx_shipment_compliance_score` (cols=['score'])

### VIEW: `report_view` (15 cols, 0 FKs, 0 indexes)
  - `supplier` `TEXT` 
  - `supplier_id` `TEXT` 
  - `invoice_no` `TEXT` 
  - `invoice_date` `TEXT` 
  - `part_no` `ANY` 
  - `description` `ANY` 
  - `unit` `TEXT` 
  - `qty` `REAL` 
  - `unit_price` `REAL` 
  - `assessable_value` `REAL` 
  - `bcd_amount` `REAL` 
  - `sws_amount` `REAL` 
  - `igst_amount` `REAL` 
  - `expenses_total` `ANY` 
  - `ldc_per_qty` `ANY`

## Domain 7. Dashboard Snapshots, Cache & KPI Governance (12 objects)

### TABLE: `dashboard_activity_log` (10 cols, 0 FKs, 1 indexes)
  - `id` `INTEGER` PK
  - `user_id` `TEXT` NOT NULL
  - `timestamp` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - `action_type` `TEXT` NOT NULL
  - `details` `TEXT` NOT NULL DEFAULT ''
  - `module_name` `TEXT` NOT NULL DEFAULT ''
  - `record_reference` `TEXT` NOT NULL DEFAULT ''
  - `navigation_target` `TEXT` NOT NULL DEFAULT ''
  - `action_context` `TEXT` NOT NULL DEFAULT ''
  - `checksum` `TEXT` NOT NULL DEFAULT ''
  - INDEX `idx_dashboard_activity_log_ts` (cols=['timestamp'])

### TABLE: `dashboard_exception_sample_cache` (4 cols, 0 FKs, 1 indexes)
  - `snapshot_date` `TEXT` PK NOT NULL
  - `sample_kind` `TEXT` PK NOT NULL
  - `shipment_ids_json` `TEXT` NOT NULL DEFAULT '[]'
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')

### TABLE: `dashboard_exception_snapshot` (7 cols, 0 FKs, 1 indexes)
  - `snapshot_date` `TEXT` PK NOT NULL
  - `missing_boe_count` `INTEGER` NOT NULL DEFAULT 0
  - `missing_expense_count` `INTEGER` NOT NULL DEFAULT 0
  - `overdue_eta_count` `INTEGER` NOT NULL DEFAULT 0
  - `open_exception_count` `INTEGER` NOT NULL DEFAULT 0
  - `sla_breach_count` `INTEGER` NOT NULL DEFAULT 0
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')

### TABLE: `dashboard_kpi_snapshot` (18 cols, 0 FKs, 1 indexes)
  - `snapshot_date` `TEXT` PK NOT NULL
  - `total_shipments` `INTEGER` NOT NULL DEFAULT 0
  - `pending_shipments` `INTEGER` NOT NULL DEFAULT 0
  - `delivered_shipments` `INTEGER` NOT NULL DEFAULT 0
  - `overdue_eta_count` `INTEGER` NOT NULL DEFAULT 0
  - `shipments_missing_boe` `INTEGER` NOT NULL DEFAULT 0
  - `shipments_missing_expense` `INTEGER` NOT NULL DEFAULT 0
  - `total_invoice_value` `REAL` NOT NULL DEFAULT 0
  - `total_expense_value` `REAL` NOT NULL DEFAULT 0
  - `total_duty_savings` `REAL` NOT NULL DEFAULT 0
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - `reconciled_boes` `INTEGER` 
  - `duty_total` `REAL` 
  - `avg_transit_days` `REAL` 
  - `shipments_missing_eta` `INTEGER` 
  - `shipments_missing_etd` `INTEGER` 
  - `total_suppliers` `INTEGER` 
  - `total_items` `INTEGER`

### TABLE: `dashboard_metrics_cache` (4 cols, 0 FKs, 2 indexes)
  - `cache_key` `TEXT` PK NOT NULL
  - `metrics_payload` `TEXT` NOT NULL
  - `snapshot_at` `TEXT` NOT NULL
  - `expires_at` `TEXT` NOT NULL
  - INDEX `idx_dashboard_metrics_cache_expires` (cols=['expires_at'])

### TABLE: `dashboard_monthly_snapshot` (6 cols, 0 FKs, 2 indexes)
  - `snapshot_date` `TEXT` PK NOT NULL
  - `period` `TEXT` PK NOT NULL
  - `shipments` `INTEGER` NOT NULL DEFAULT 0
  - `value` `REAL` NOT NULL DEFAULT 0
  - `duty_savings` `REAL` NOT NULL DEFAULT 0
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - INDEX `idx_dashboard_monthly_snapshot_period` (cols=['period'])

### TABLE: `dashboard_workflow_sample_cache` (3 cols, 0 FKs, 1 indexes)
  - `snapshot_date` `TEXT` PK NOT NULL
  - `workflow_ids_json` `TEXT` NOT NULL DEFAULT '[]'
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')

### TABLE: `dashboard_workflow_snapshot` (7 cols, 0 FKs, 1 indexes)
  - `snapshot_date` `TEXT` PK NOT NULL
  - `open_workflow_count` `INTEGER` NOT NULL DEFAULT 0
  - `active_workflow_count` `INTEGER` NOT NULL DEFAULT 0
  - `sla_breach_count` `INTEGER` NOT NULL DEFAULT 0
  - `recent_incident_count` `INTEGER` NOT NULL DEFAULT 0
  - `resolved_today_count` `INTEGER` NOT NULL DEFAULT 0
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')

### TABLE: `kpi_alert_log` (7 cols, 0 FKs, 3 indexes)
  - `alert_id` `TEXT` PK NOT NULL
  - `kpi_name` `TEXT` NOT NULL
  - `current_value` `REAL` NOT NULL
  - `threshold_value` `REAL` NOT NULL
  - `severity` `TEXT` NOT NULL
  - `triggered_at` `TEXT` NOT NULL
  - `resolved_at` `TEXT` 
  - INDEX `idx_kpi_alert_log_resolved` (cols=['resolved_at'])
  - INDEX `idx_kpi_alert_log_kpi` (cols=['kpi_name'])

### TABLE: `kpi_alert_rules` (8 cols, 0 FKs, 2 indexes)
  - `id` `TEXT` PK NOT NULL
  - `kpi_name` `TEXT` NOT NULL
  - `threshold_value` `REAL` NOT NULL
  - `comparison_operator` `TEXT` NOT NULL
  - `severity` `TEXT` NOT NULL DEFAULT 'warning'
  - `enabled` `INTEGER` NOT NULL DEFAULT 1
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - `updated_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - INDEX `idx_kpi_alert_rules_kpi` (cols=['kpi_name'])

### TABLE: `kpi_daily_snapshots` (6 cols, 0 FKs, 3 indexes)
  - `id` `INTEGER` PK
  - `snapshot_date` `TEXT` NOT NULL
  - `kpi_name` `TEXT` NOT NULL
  - `value` `REAL` NOT NULL
  - `dimensions_json` `TEXT` 
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - INDEX `idx_kpi_daily_snapshots_name` (cols=['kpi_name'])
  - INDEX `idx_kpi_daily_snapshots_date` (cols=['snapshot_date'])
  - INDEX `sqlite_autoindex_kpi_daily_snapshots_1` (UNIQUE cols=['snapshot_date', 'kpi_name'])

### TABLE: `kpi_metadata` (5 cols, 0 FKs, 1 indexes)
  - `kpi_name` `TEXT` PK NOT NULL
  - `formula` `TEXT` NOT NULL DEFAULT ''
  - `description` `TEXT` NOT NULL DEFAULT ''
  - `unit` `TEXT` NOT NULL DEFAULT 'count'
  - `last_updated` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')

## Domain 8. Security, RBAC, Auth & Audit Logging (8 objects)

### TABLE: `audit_logs` (9 cols, 0 FKs, 5 indexes)
  - `id` `INTEGER` PK
  - `table_name` `TEXT` NOT NULL
  - `row_id` `TEXT` 
  - `action` `TEXT` NOT NULL
  - `user_id` `TEXT` 
  - `before_json` `TEXT` 
  - `after_json` `TEXT` 
  - `metadata` `TEXT` 
  - `created_at` `DATETIME` DEFAULT CURRENT_TIMESTAMP
  - INDEX `idx_audit_logs_user_created_at` (cols=['user_id', 'created_at'])
  - INDEX `idx_audit_logs_table_action_created_at` (cols=[None, 'action', 'created_at'])
  - INDEX `idx_audit_logs_action` (cols=['action'])
  - INDEX `idx_audit_logs_created_at` (cols=['created_at'])
  - INDEX `idx_audit_logs_table_name` (cols=['table_name'])

### TABLE: `auth_failed_attempts` (5 cols, 0 FKs, 1 indexes)
  - `id` `INTEGER` PK
  - `user_id` `TEXT` NOT NULL
  - `attempted_at` `DATETIME` DEFAULT CURRENT_TIMESTAMP
  - `reason` `TEXT` 
  - `source` `TEXT` 
  - INDEX `idx_auth_failed_attempts_user_time` (cols=['user_id', 'attempted_at'])

### TABLE: `auth_lockout_state` (5 cols, 0 FKs, 2 indexes)
  - `user_id` `TEXT` PK NOT NULL
  - `failure_count` `INTEGER` NOT NULL DEFAULT 0
  - `window_started_at` `TEXT` NOT NULL
  - `locked_until` `TEXT` 
  - `updated_at` `TEXT` NOT NULL DEFAULT datetime('now')
  - INDEX `idx_auth_lockout_state_locked_until` (cols=['locked_until'])

### TABLE: `auth_password_history` (5 cols, 0 FKs, 1 indexes)
  - `id` `INTEGER` PK
  - `user_id` `TEXT` NOT NULL
  - `password_hash` `TEXT` NOT NULL
  - `algorithm` `TEXT` NOT NULL
  - `replaced_at` `DATETIME` DEFAULT CURRENT_TIMESTAMP
  - INDEX `idx_auth_password_history_user` (cols=['user_id', 'replaced_at'])

### TABLE: `role_dashboard_permissions` (3 cols, 0 FKs, 1 indexes)
  - `role` `TEXT` PK NOT NULL
  - `widget_key` `TEXT` PK NOT NULL
  - `visible` `INTEGER` NOT NULL DEFAULT 1

### TABLE: `security_policy_versions` (6 cols, 0 FKs, 2 indexes)
  - `id` `INTEGER` PK
  - `version` `INTEGER` NOT NULL
  - `policy_json` `TEXT` NOT NULL
  - `password_rules_json` `TEXT` 
  - `changed_by` `TEXT` 
  - `changed_at` `TEXT` NOT NULL DEFAULT datetime('now')
  - INDEX `idx_security_policy_versions_changed_at` (cols=['changed_at'])
  - INDEX `sqlite_autoindex_security_policy_versions_1` (UNIQUE cols=['version'])

### TABLE: `user_activity_audit_logs` (9 cols, 0 FKs, 3 indexes)
  - `id` `TEXT` PK NOT NULL
  - `user_id` `TEXT` 
  - `action_name` `TEXT` NOT NULL
  - `entity_type` `TEXT` 
  - `entity_id` `TEXT` 
  - `details_json` `TEXT` 
  - `status` `TEXT` NOT NULL
  - `timestamp` `TEXT` NOT NULL
  - `severity` `TEXT` NOT NULL DEFAULT 'INFO'
  - INDEX `idx_user_activity_entity` (cols=['entity_type', 'entity_id'])
  - INDEX `idx_user_activity_timestamp` (cols=['timestamp'])

### TABLE: `user_roles` (8 cols, 0 FKs, 3 indexes)
  - `id` `INTEGER` PK
  - `user_id` `TEXT` NOT NULL
  - `role` `TEXT` NOT NULL
  - `permissions` `TEXT` 
  - `created_at` `DATETIME` DEFAULT CURRENT_TIMESTAMP
  - `updated_at` `DATETIME` DEFAULT CURRENT_TIMESTAMP
  - `created_by` `TEXT` 
  - `updated_by` `TEXT` 
  - INDEX `idx_user_roles_role` (cols=['role'])
  - INDEX `idx_user_roles_user_id` (cols=['user_id'])
  - INDEX `sqlite_autoindex_user_roles_1` (UNIQUE cols=['user_id'])

## Domain 9. Database Management, Backups, Settings, Diagnostics & Recovery (12 objects)

### TABLE: `app_metadata` (2 cols, 0 FKs, 1 indexes)
  - `key` `TEXT` PK NOT NULL
  - `value` `TEXT` NOT NULL

### TABLE: `app_settings` (3 cols, 0 FKs, 2 indexes)
  - `key` `TEXT` PK
  - `value` `TEXT` NOT NULL
  - `updated_at` `TEXT` NOT NULL DEFAULT datetime('now')
  - INDEX `idx_app_settings_updated_at` (cols=['updated_at'])

### TABLE: `backup_schedules` (13 cols, 0 FKs, 2 indexes)
  - `id` `INTEGER` PK
  - `name` `TEXT` NOT NULL
  - `cron_expr` `TEXT` 
  - `time_zone` `TEXT` DEFAULT 'UTC'
  - `destination` `TEXT` NOT NULL DEFAULT 'local'
  - `retention_count` `INTEGER` DEFAULT 5
  - `retention_days` `INTEGER` DEFAULT 30
  - `enabled` `INTEGER` DEFAULT 1
  - `last_run` `DATETIME` 
  - `next_run` `DATETIME` 
  - `created_by` `TEXT` 
  - `created_at` `DATETIME` DEFAULT CURRENT_TIMESTAMP
  - `notes` `TEXT` 
  - INDEX `idx_backup_schedules_next_run` (cols=['next_run'])
  - INDEX `idx_backup_schedules_enabled` (cols=['enabled'])

### TABLE: `backups` (18 cols, 0 FKs, 3 indexes)
  - `id` `INTEGER` PK
  - `filename` `TEXT` NOT NULL
  - `path` `TEXT` NOT NULL
  - `destination` `TEXT` 
  - `size_bytes` `INTEGER` 
  - `sha256` `TEXT` 
  - `created_by` `TEXT` 
  - `created_at` `DATETIME` DEFAULT CURRENT_TIMESTAMP
  - `retention_until` `DATETIME` 
  - `notes` `TEXT` 
  - `status` `TEXT` DEFAULT 'completed'
  - `error_message` `TEXT` 
  - `validation_status` `TEXT` 
  - `validation_checked_at` `TEXT` 
  - `validation_message` `TEXT` 
  - `restore_simulation_status` `TEXT` 
  - `restore_simulation_checked_at` `TEXT` 
  - `restore_simulation_message` `TEXT` 
  - INDEX `idx_backups_status` (cols=['status'])
  - INDEX `idx_backups_destination` (cols=['destination'])
  - INDEX `idx_backups_created_at` (cols=['created_at'])

### TABLE: `bug_notes` (9 cols, 0 FKs, 3 indexes)
  - `id` `TEXT` PK
  - `title` `TEXT` NOT NULL
  - `description` `TEXT` 
  - `status` `TEXT` NOT NULL DEFAULT 'OPEN'
  - `screenshot_path` `TEXT` 
  - `context` `TEXT` 
  - `meta` `TEXT` 
  - `created_at` `INTEGER` NOT NULL
  - `updated_at` `INTEGER` NOT NULL
  - INDEX `idx_bug_notes_status` (cols=['status'])
  - INDEX `idx_bug_notes_updated_at_desc` (cols=['updated_at'])

### TABLE: `error_memory` (27 cols, 0 FKs, 7 indexes)
  - `id` `TEXT` PK
  - `fingerprint` `TEXT` NOT NULL
  - `occurrence_count` `INTEGER` NOT NULL DEFAULT 1
  - `first_seen_at` `TEXT` NOT NULL
  - `last_seen_at` `TEXT` NOT NULL
  - `app_version` `TEXT` 
  - `build_version` `TEXT` 
  - `environment` `TEXT` 
  - `module_name` `TEXT` 
  - `command_name` `TEXT` 
  - `page_name` `TEXT` 
  - `component_name` `TEXT` 
  - `error_code` `TEXT` 
  - `error_category` `TEXT` 
  - `error_message` `TEXT` NOT NULL
  - `stack_trace` `TEXT` 
  - `source_file` `TEXT` 
  - `source_function` `TEXT` 
  - `user_action` `TEXT` 
  - `redacted_input_context` `TEXT` 
  - `affected_entity_ids` `TEXT` 
  - `severity` `TEXT` NOT NULL DEFAULT 'error'
  - `recoverable` `INTEGER` NOT NULL DEFAULT 0
  - `retryable` `INTEGER` NOT NULL DEFAULT 0
  - `app_state_snapshot` `TEXT` 
  - `status` `TEXT` NOT NULL DEFAULT 'new'
  - `ai_summary` `TEXT` 
  - INDEX `idx_error_memory_command` (cols=['command_name'])
  - INDEX `idx_error_memory_module` (cols=['module_name'])
  - INDEX `idx_error_memory_severity` (cols=['severity'])
  - INDEX `idx_error_memory_status` (cols=['status'])
  - INDEX `idx_error_memory_last_seen_desc` (cols=[None])
  - INDEX `sqlite_autoindex_error_memory_2` (UNIQUE cols=['fingerprint'])

### TABLE: `error_memory_cleanup_runs` (10 cols, 0 FKs, 2 indexes)
  - `id` `TEXT` PK
  - `executed_at` `TEXT` NOT NULL
  - `trigger_source` `TEXT` NOT NULL
  - `dry_run` `INTEGER` NOT NULL DEFAULT 0
  - `total_before` `INTEGER` NOT NULL DEFAULT 0
  - `total_after` `INTEGER` NOT NULL DEFAULT 0
  - `candidate_count` `INTEGER` NOT NULL DEFAULT 0
  - `deleted_count` `INTEGER` NOT NULL DEFAULT 0
  - `protected_count` `INTEGER` NOT NULL DEFAULT 0
  - `error_message` `TEXT` 
  - INDEX `idx_error_memory_cleanup_runs_executed` (cols=[None])

### TABLE: `error_memory_pruned_groups` (10 cols, 0 FKs, 4 indexes)
  - `id` `TEXT` PK
  - `run_id` `TEXT` NOT NULL
  - `fingerprint` `TEXT` NOT NULL
  - `occurrence_count` `INTEGER` NOT NULL DEFAULT 1
  - `first_seen_at` `TEXT` NOT NULL
  - `last_seen_at` `TEXT` NOT NULL
  - `severity` `TEXT` NOT NULL
  - `status` `TEXT` NOT NULL
  - `prune_reason` `TEXT` NOT NULL
  - `created_at` `TEXT` NOT NULL
  - INDEX `idx_error_memory_pruned_groups_created_at` (cols=[None])
  - INDEX `idx_error_memory_pruned_groups_fingerprint` (cols=['fingerprint'])
  - INDEX `idx_error_memory_pruned_groups_run` (cols=['run_id'])

### TABLE: `index_recommendation_history` (4 cols, 0 FKs, 1 indexes)
  - `id` `TEXT` PK NOT NULL
  - `recommendation_sql` `TEXT` NOT NULL
  - `source_query` `TEXT` 
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')

### TABLE: `notifications` (9 cols, 0 FKs, 1 indexes)
  - `id` `TEXT` PK
  - `title` `TEXT` NOT NULL
  - `message` `TEXT` NOT NULL
  - `type` `TEXT` NOT NULL
  - `timestamp` `TEXT` NOT NULL
  - `read` `BOOLEAN` NOT NULL DEFAULT FALSE
  - `category` `TEXT` 
  - `action_url` `TEXT` 
  - `created_at` `TIMESTAMP` DEFAULT CURRENT_TIMESTAMP

### TABLE: `system_agent_audit_log` (17 cols, 0 FKs, 3 indexes)
  - `audit_id` `TEXT` PK NOT NULL
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - `session_id` `TEXT` NOT NULL DEFAULT ''
  - `caller_user_id` `TEXT` NOT NULL DEFAULT ''
  - `caller_role` `TEXT` NOT NULL DEFAULT ''
  - `intent_route` `TEXT` NOT NULL DEFAULT ''
  - `matched_rule_id` `TEXT` NOT NULL DEFAULT ''
  - `llm_used` `INTEGER` NOT NULL DEFAULT 0
  - `policy_decision_json` `TEXT` NOT NULL DEFAULT '{}'
  - `limiter_bucket` `TEXT` NOT NULL DEFAULT ''
  - `was_blocked` `INTEGER` NOT NULL DEFAULT 0
  - `grounding_ok` `INTEGER` 
  - `tools_attempted_json` `TEXT` NOT NULL DEFAULT '[]'
  - `tools_executed_json` `TEXT` NOT NULL DEFAULT '[]'
  - `confidence` `REAL` 
  - `trace_checksum` `TEXT` NOT NULL DEFAULT ''
  - `snapshot_checksum` `TEXT` NOT NULL DEFAULT ''
  - INDEX `idx_system_agent_audit_route` (cols=['intent_route', None])
  - INDEX `idx_system_agent_audit_created_at` (cols=[None])

### TABLE: `write_recovery_journal` (8 cols, 0 FKs, 2 indexes)
  - `id` `TEXT` PK NOT NULL
  - `module` `TEXT` NOT NULL
  - `operation` `TEXT` NOT NULL
  - `entity_id` `TEXT` 
  - `status` `TEXT` NOT NULL
  - `error_message` `TEXT` 
  - `started_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - `finished_at` `TEXT` 
  - INDEX `idx_write_recovery_journal_status` (cols=['status'])

## Domain 10. Exception Workflow & SLA Reliability (14 objects)

### TABLE: `adaptive_sla_adjustments` (7 cols, 0 FKs, 3 indexes)
  - `id` `TEXT` PK NOT NULL
  - `exception_type` `TEXT` NOT NULL
  - `snapshot_date` `TEXT` NOT NULL
  - `previous_hours` `REAL` NOT NULL DEFAULT 24
  - `adjusted_hours` `REAL` NOT NULL DEFAULT 24
  - `factors_json` `TEXT` NOT NULL DEFAULT '{}'
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - INDEX `idx_adaptive_sla_date` (cols=['snapshot_date'])
  - INDEX `sqlite_autoindex_adaptive_sla_adjustments_2` (UNIQUE cols=['exception_type', 'snapshot_date'])

### TABLE: `daily_exception_summary` (6 cols, 0 FKs, 1 indexes)
  - `snapshot_date` `TEXT` PK NOT NULL
  - `overdue_count` `INTEGER` NOT NULL DEFAULT 0
  - `missing_boe_count` `INTEGER` NOT NULL DEFAULT 0
  - `missing_expense_count` `INTEGER` NOT NULL DEFAULT 0
  - `missing_document_count` `INTEGER` NOT NULL DEFAULT 0
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')

### TABLE: `exception_backlog_snapshot` (6 cols, 0 FKs, 1 indexes)
  - `snapshot_date` `TEXT` PK NOT NULL
  - `open_count` `INTEGER` NOT NULL DEFAULT 0
  - `in_progress_count` `INTEGER` NOT NULL DEFAULT 0
  - `critical_count` `INTEGER` NOT NULL DEFAULT 0
  - `resolved_today` `INTEGER` NOT NULL DEFAULT 0
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')

### TABLE: `exception_cases` (20 cols, 1 FKs, 4 indexes)
  - `id` `TEXT` PK NOT NULL
  - `exception_type` `TEXT` NOT NULL
  - `entity_type` `TEXT` NOT NULL DEFAULT 'shipment'
  - `entity_id` `TEXT` NOT NULL
  - `status` `TEXT` NOT NULL DEFAULT 'OPEN'
  - `priority` `TEXT` NOT NULL DEFAULT 'MEDIUM'
  - `assigned_to` `TEXT` 
  - `assigned_at` `TEXT` 
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - `updated_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - `resolved_by` `TEXT` 
  - `resolved_at` `TEXT` 
  - `sla_deadline` `TEXT` 
  - `sla_status` `TEXT` NOT NULL DEFAULT 'ON_TIME'
  - `last_viewed_at` `TEXT` 
  - `escalated_at` `TEXT` 
  - `escalation_level` `INTEGER` NOT NULL DEFAULT 0
  - `workflow_timeout_flag` `INTEGER` NOT NULL DEFAULT 0
  - `recurrence_flag` `INTEGER` NOT NULL DEFAULT 0
  - `assignment_method` `TEXT` NOT NULL DEFAULT 'MANUAL'
  - FK: `entity_id` -> `shipments(id)` (ON UPDATE NO ACTION, ON DELETE CASCADE)
  - INDEX `idx_exception_cases_resolved_at` (cols=['resolved_at'])
  - INDEX `idx_exception_cases_status_type` (cols=['status', 'exception_type'])
  - INDEX `idx_exception_cases_entity` (cols=['exception_type', 'entity_id'])

### TABLE: `exception_escalation_log` (7 cols, 1 FKs, 2 indexes)
  - `id` `TEXT` PK NOT NULL
  - `exception_case_id` `TEXT` NOT NULL
  - `exception_type` `TEXT` NOT NULL
  - `escalation_level` `INTEGER` NOT NULL DEFAULT 1
  - `notify_role` `TEXT` NOT NULL DEFAULT ''
  - `details` `TEXT` NOT NULL DEFAULT ''
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - FK: `exception_case_id` -> `exception_cases(id)` (ON UPDATE NO ACTION, ON DELETE CASCADE)
  - INDEX `idx_exception_escalation_case` (cols=['exception_case_id', 'created_at'])

### TABLE: `exception_integrity_log` (5 cols, 0 FKs, 3 indexes)
  - `id` `TEXT` PK NOT NULL
  - `exception_id` `TEXT` NOT NULL DEFAULT ''
  - `issue_type` `TEXT` NOT NULL
  - `detected_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - `details` `TEXT` NOT NULL DEFAULT ''
  - INDEX `idx_exception_integrity_type` (cols=['issue_type'])
  - INDEX `idx_exception_integrity_detected` (cols=['detected_at'])

### TABLE: `exception_lifecycle_events` (6 cols, 1 FKs, 2 indexes)
  - `id` `TEXT` PK NOT NULL
  - `exception_case_id` `TEXT` NOT NULL
  - `event_type` `TEXT` NOT NULL
  - `user_id` `TEXT` 
  - `details` `TEXT` NOT NULL DEFAULT ''
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - FK: `exception_case_id` -> `exception_cases(id)` (ON UPDATE NO ACTION, ON DELETE CASCADE)
  - INDEX `idx_exception_lifecycle_case` (cols=['exception_case_id', 'created_at'])

### TABLE: `exception_notes` (5 cols, 1 FKs, 2 indexes)
  - `note_id` `TEXT` PK NOT NULL
  - `exception_case_id` `TEXT` NOT NULL
  - `user_id` `TEXT` NOT NULL
  - `note_text` `TEXT` NOT NULL
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - FK: `exception_case_id` -> `exception_cases(id)` (ON UPDATE NO ACTION, ON DELETE CASCADE)
  - INDEX `idx_exception_notes_case` (cols=['exception_case_id', 'created_at'])

### TABLE: `exception_recurrence_counts` (4 cols, 0 FKs, 1 indexes)
  - `exception_type` `TEXT` PK NOT NULL
  - `entity_id` `TEXT` PK NOT NULL
  - `total_opens` `INTEGER` NOT NULL DEFAULT 0
  - `last_opened_at` `TEXT`

### TABLE: `exception_resolution_analytics` (7 cols, 0 FKs, 1 indexes)
  - `snapshot_date` `TEXT` PK NOT NULL
  - `avg_resolution_hours` `REAL` 
  - `p90_resolution_hours` `REAL` 
  - `resolved_count` `INTEGER` NOT NULL DEFAULT 0
  - `resolutions_by_user_json` `TEXT` NOT NULL DEFAULT '{}'
  - `common_delay_hours` `REAL` 
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')

### TABLE: `exception_resolution_log` (8 cols, 1 FKs, 2 indexes)
  - `resolution_id` `TEXT` PK NOT NULL
  - `exception_case_id` `TEXT` NOT NULL
  - `exception_type` `TEXT` NOT NULL
  - `entity_id` `TEXT` NOT NULL
  - `status` `TEXT` NOT NULL
  - `resolved_by` `TEXT` 
  - `resolved_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - `notes` `TEXT` NOT NULL DEFAULT ''
  - FK: `exception_case_id` -> `exception_cases(id)` (ON UPDATE NO ACTION, ON DELETE CASCADE)
  - INDEX `idx_exception_resolution_case` (cols=['exception_case_id'])

### TABLE: `exception_sla_escalation_rules` (4 cols, 0 FKs, 1 indexes)
  - `exception_type` `TEXT` PK NOT NULL
  - `sla_hours` `INTEGER` NOT NULL DEFAULT 0
  - `escalation_level` `INTEGER` PK NOT NULL DEFAULT 1
  - `notify_role` `TEXT` NOT NULL DEFAULT 'admin'

### TABLE: `exception_sla_metrics` (6 cols, 0 FKs, 1 indexes)
  - `snapshot_date` `TEXT` PK NOT NULL
  - `avg_resolution_hours` `REAL` 
  - `sla_compliance_rate` `REAL` NOT NULL DEFAULT 0
  - `sla_breach_count` `INTEGER` NOT NULL DEFAULT 0
  - `resolution_backlog` `INTEGER` NOT NULL DEFAULT 0
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')

### TABLE: `resolution_efficiency_metrics` (5 cols, 0 FKs, 1 indexes)
  - `snapshot_date` `TEXT` PK NOT NULL
  - `user_id` `TEXT` PK NOT NULL
  - `resolutions_count` `INTEGER` NOT NULL DEFAULT 0
  - `avg_resolution_hours` `REAL` 
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')

## Domain 11. Workflow Automation, Rules, Costs & Multi-Env Deployment (29 objects)

### TABLE: `automation_benchmark_history` (6 cols, 0 FKs, 2 indexes)
  - `id` `TEXT` PK NOT NULL
  - `period_type` `TEXT` NOT NULL
  - `period_start` `TEXT` NOT NULL
  - `period_end` `TEXT` NOT NULL
  - `metrics_json` `TEXT` NOT NULL DEFAULT '{}'
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - INDEX `idx_automation_benchmark_period` (cols=['period_type', None])

### TABLE: `automation_capacity_load` (11 cols, 0 FKs, 2 indexes)
  - `id` `TEXT` PK NOT NULL
  - `snapshot_at` `TEXT` NOT NULL
  - `current_active_rules` `INTEGER` NOT NULL DEFAULT 0
  - `actions_per_cycle` `INTEGER` NOT NULL DEFAULT 0
  - `records_processed_per_cycle` `INTEGER` NOT NULL DEFAULT 0
  - `peak_cycle_duration_ms` `INTEGER` NOT NULL DEFAULT 0
  - `queue_depth` `INTEGER` NOT NULL DEFAULT 0
  - `load_percentage` `REAL` NOT NULL DEFAULT 0
  - `load_state` `TEXT` NOT NULL DEFAULT 'LOW'
  - `total_cost_units_cycle` `REAL` NOT NULL DEFAULT 0
  - `factors_json` `TEXT` NOT NULL DEFAULT '{}'
  - INDEX `idx_capacity_load_snapshot` (cols=[None])

### TABLE: `automation_cost_limits` (5 cols, 0 FKs, 1 indexes)
  - `id` `TEXT` PK NOT NULL DEFAULT 'default'
  - `max_cost_units_per_cycle` `REAL` NOT NULL DEFAULT 500000
  - `max_execution_time_per_cycle_ms` `INTEGER` NOT NULL DEFAULT 120000
  - `max_records_processed_per_cycle` `INTEGER` NOT NULL DEFAULT 500000
  - `updated_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')

### TABLE: `automation_decision_feedback` (10 cols, 0 FKs, 4 indexes)
  - `id` `TEXT` PK NOT NULL
  - `source_automation_id` `TEXT` NOT NULL DEFAULT ''
  - `rule_id` `TEXT` NOT NULL
  - `target_entity` `TEXT` NOT NULL
  - `decision_result` `TEXT` NOT NULL DEFAULT '{}'
  - `resolution_success` `INTEGER` NOT NULL DEFAULT 0
  - `follow_up_required` `INTEGER` NOT NULL DEFAULT 0
  - `resolution_time_hours` `REAL` 
  - `helpful` `INTEGER` 
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - INDEX `idx_automation_feedback_target` (cols=['target_entity'])
  - INDEX `idx_automation_feedback_rule` (cols=['rule_id', None])
  - INDEX `idx_automation_feedback_source` (UNIQUE cols=['source_automation_id'])

### TABLE: `automation_roi_metrics` (7 cols, 0 FKs, 1 indexes)
  - `snapshot_date` `TEXT` PK NOT NULL
  - `time_saved_hours_estimate` `REAL` NOT NULL DEFAULT 0
  - `manual_workload_reduction_pct` `REAL` NOT NULL DEFAULT 0
  - `sla_compliance_improvement_pct` `REAL` NOT NULL DEFAULT 0
  - `resolution_speed_increase_pct` `REAL` NOT NULL DEFAULT 0
  - `factors_json` `TEXT` NOT NULL DEFAULT '{}'
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')

### TABLE: `automation_stability_alert` (6 cols, 0 FKs, 2 indexes)
  - `id` `TEXT` PK NOT NULL
  - `alert_type` `TEXT` NOT NULL
  - `severity` `TEXT` NOT NULL
  - `details_json` `TEXT` NOT NULL DEFAULT '{}'
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - `acknowledged` `INTEGER` NOT NULL DEFAULT 0
  - INDEX `idx_stability_alert_open` (cols=['acknowledged', None])

### TABLE: `canary_rule_deployment` (7 cols, 0 FKs, 2 indexes)
  - `id` `TEXT` PK NOT NULL
  - `tenant_id` `TEXT` NOT NULL DEFAULT 'tenant-default'
  - `rule_id` `TEXT` NOT NULL
  - `version_id` `TEXT` NOT NULL
  - `sample_size_percentage` `REAL` NOT NULL DEFAULT 10
  - `deployment_status` `TEXT` NOT NULL DEFAULT 'ACTIVE'
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - INDEX `sqlite_autoindex_canary_rule_deployment_2` (UNIQUE cols=['tenant_id', 'rule_id'])

### TABLE: `daily_automation_economics_index` (7 cols, 0 FKs, 1 indexes)
  - `snapshot_date` `TEXT` PK NOT NULL
  - `benefit_score` `REAL` NOT NULL DEFAULT 0
  - `cost_score` `REAL` NOT NULL DEFAULT 0
  - `efficiency_gain` `REAL` NOT NULL DEFAULT 0
  - `economics_index` `REAL` NOT NULL DEFAULT 0
  - `factors_json` `TEXT` NOT NULL DEFAULT '{}'
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')

### TABLE: `daily_workflow_efficiency_score` (4 cols, 0 FKs, 1 indexes)
  - `snapshot_date` `TEXT` PK NOT NULL
  - `efficiency_score` `REAL` NOT NULL
  - `factors_json` `TEXT` NOT NULL DEFAULT '{}'
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')

### TABLE: `deployment_conflict_log` (8 cols, 0 FKs, 2 indexes)
  - `id` `TEXT` PK NOT NULL
  - `detected_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - `rule_id` `TEXT` NOT NULL
  - `tenant_id` `TEXT` NOT NULL
  - `environment_id` `TEXT` NOT NULL
  - `related_version_id` `TEXT` 
  - `conflict_type` `TEXT` NOT NULL
  - `details_json` `TEXT` NOT NULL DEFAULT '{}'
  - INDEX `idx_deployment_conflict_rule_time` (cols=['rule_id', None])

### TABLE: `deployment_risk_assessment` (10 cols, 0 FKs, 3 indexes)
  - `assessment_id` `TEXT` PK NOT NULL
  - `assessed_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - `rule_id` `TEXT` NOT NULL
  - `version_id` `TEXT` NOT NULL
  - `tenant_id` `TEXT` NOT NULL
  - `environment_id` `TEXT` NOT NULL
  - `risk_score` `REAL` NOT NULL
  - `risk_level` `TEXT` NOT NULL
  - `safe_to_deploy` `INTEGER` NOT NULL
  - `assessment_json` `TEXT` NOT NULL DEFAULT '{}'
  - INDEX `idx_deployment_risk_assessment_rule` (cols=['rule_id', None])
  - INDEX `sqlite_autoindex_deployment_risk_assessment_2` (UNIQUE cols=['rule_id', 'version_id', 'tenant_id', 'environment_id'])

### TABLE: `deployment_risk_timeline` (10 cols, 0 FKs, 2 indexes)
  - `id` `TEXT` PK NOT NULL
  - `deployment_time` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - `risk_score` `REAL` NOT NULL
  - `risk_level` `TEXT` NOT NULL
  - `environment_id` `TEXT` NOT NULL
  - `tenant_id` `TEXT` NOT NULL
  - `rule_id` `TEXT` NOT NULL
  - `version_id` `TEXT` NOT NULL
  - `result` `TEXT` NOT NULL
  - `details_json` `TEXT` NOT NULL DEFAULT '{}'
  - INDEX `idx_deployment_risk_timeline_env_time` (cols=['environment_id', None])

### TABLE: `rollback_action_log` (7 cols, 0 FKs, 2 indexes)
  - `id` `TEXT` PK NOT NULL
  - `original_automation_id` `TEXT` NOT NULL
  - `rollback_type` `TEXT` NOT NULL
  - `target_entity` `TEXT` NOT NULL
  - `performed_by` `TEXT` NOT NULL
  - `performed_at` `TEXT` NOT NULL
  - `details_json` `TEXT` NOT NULL DEFAULT '{}'
  - INDEX `idx_rollback_target` (cols=['target_entity', None])

### TABLE: `rule_cost_efficiency_metrics` (8 cols, 0 FKs, 2 indexes)
  - `rule_id` `TEXT` PK NOT NULL
  - `metric_date` `TEXT` PK NOT NULL
  - `total_cost_units` `REAL` NOT NULL DEFAULT 0
  - `total_actions` `INTEGER` NOT NULL DEFAULT 0
  - `total_resolution_gain` `REAL` NOT NULL DEFAULT 0
  - `cost_per_resolution` `REAL` NOT NULL DEFAULT 0
  - `efficiency_score` `REAL` NOT NULL DEFAULT 0
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - INDEX `idx_rule_cost_eff_date` (cols=['metric_date'])

### TABLE: `rule_deployment_impact_metrics` (9 cols, 0 FKs, 3 indexes)
  - `id` `TEXT` PK NOT NULL
  - `rule_id` `TEXT` NOT NULL
  - `version_id` `TEXT` NOT NULL
  - `snapshot_at` `TEXT` NOT NULL
  - `failure_rate` `REAL` NOT NULL DEFAULT 0
  - `cost_units_delta` `REAL` NOT NULL DEFAULT 0
  - `resolution_gain_delta` `REAL` NOT NULL DEFAULT 0
  - `execution_count` `INTEGER` NOT NULL DEFAULT 0
  - `factors_json` `TEXT` NOT NULL DEFAULT '{}'
  - INDEX `idx_rule_deploy_impact_version` (cols=['version_id', 'snapshot_at'])
  - INDEX `sqlite_autoindex_rule_deployment_impact_metrics_2` (UNIQUE cols=['rule_id', 'snapshot_at'])

### TABLE: `rule_effectiveness_metrics` (11 cols, 0 FKs, 2 indexes)
  - `rule_id` `TEXT` PK NOT NULL
  - `metric_date` `TEXT` PK NOT NULL
  - `actions_taken` `INTEGER` NOT NULL DEFAULT 0
  - `successful_actions` `INTEGER` NOT NULL DEFAULT 0
  - `failed_actions` `INTEGER` NOT NULL DEFAULT 0
  - `cases_resolved` `INTEGER` NOT NULL DEFAULT 0
  - `cases_assigned` `INTEGER` NOT NULL DEFAULT 0
  - `priority_adjustments` `INTEGER` NOT NULL DEFAULT 0
  - `sla_improvements` `INTEGER` NOT NULL DEFAULT 0
  - `avg_resolution_time_saved` `REAL` NOT NULL DEFAULT 0
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - INDEX `idx_rule_effectiveness_date` (cols=['metric_date'])

### TABLE: `rule_execution_cost_estimate` (8 cols, 0 FKs, 2 indexes)
  - `rule_id` `TEXT` PK NOT NULL DEFAULT ''
  - `action_type` `TEXT` PK NOT NULL
  - `estimated_cpu_cost` `REAL` NOT NULL DEFAULT 1
  - `estimated_io_cost` `REAL` NOT NULL DEFAULT 1
  - `estimated_time_ms` `REAL` NOT NULL DEFAULT 50
  - `estimated_memory_cost` `REAL` NOT NULL DEFAULT 1
  - `cost_weight` `REAL` NOT NULL DEFAULT 1
  - `updated_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - INDEX `idx_rule_exec_cost_action` (cols=['action_type'])

### TABLE: `rule_safety_index` (5 cols, 0 FKs, 2 indexes)
  - `rule_id` `TEXT` PK NOT NULL
  - `snapshot_date` `TEXT` PK NOT NULL
  - `safety_score` `REAL` NOT NULL
  - `factors_json` `TEXT` NOT NULL DEFAULT '{}'
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - INDEX `idx_rule_safety_date` (cols=['snapshot_date'])

### TABLE: `workflow_automation_log` (15 cols, 0 FKs, 4 indexes)
  - `automation_id` `TEXT` PK NOT NULL
  - `rule_id` `TEXT` NOT NULL DEFAULT ''
  - `action_taken` `TEXT` NOT NULL
  - `target_entity` `TEXT` NOT NULL DEFAULT ''
  - `executed_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - `execution_result` `TEXT` NOT NULL DEFAULT '{}'
  - `cases_resolved` `INTEGER` NOT NULL DEFAULT 0
  - `cases_assigned` `INTEGER` NOT NULL DEFAULT 0
  - `priority_adjustments` `INTEGER` NOT NULL DEFAULT 0
  - `sla_improvements` `INTEGER` NOT NULL DEFAULT 0
  - `resolution_time_reduction_hours` `REAL` NOT NULL DEFAULT 0
  - `actual_execution_time_ms` `INTEGER` NOT NULL DEFAULT 0
  - `records_processed` `INTEGER` NOT NULL DEFAULT 0
  - `estimated_cost_units` `REAL` NOT NULL DEFAULT 0
  - `tenant_id` `TEXT` NOT NULL DEFAULT 'tenant-default'
  - INDEX `idx_workflow_automation_log_tenant_time` (cols=['tenant_id', None])
  - INDEX `idx_workflow_automation_action_hour` (cols=['action_taken', 'executed_at'])
  - INDEX `idx_workflow_automation_executed` (cols=['executed_at'])

### TABLE: `workflow_decision_rules` (10 cols, 0 FKs, 3 indexes)
  - `rule_id` `TEXT` PK NOT NULL
  - `rule_name` `TEXT` NOT NULL
  - `rule_type` `TEXT` NOT NULL
  - `condition_expression` `TEXT` NOT NULL DEFAULT '{}'
  - `action_type` `TEXT` NOT NULL DEFAULT ''
  - `priority` `INTEGER` NOT NULL DEFAULT 0
  - `enabled` `INTEGER` NOT NULL DEFAULT 0
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - `updated_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - `tenant_id` `TEXT` NOT NULL DEFAULT 'tenant-default'
  - INDEX `idx_workflow_decision_rules_tenant` (cols=['tenant_id', 'rule_type', 'enabled', 'priority'])
  - INDEX `idx_workflow_rules_type_enabled` (cols=['rule_type', 'enabled', 'priority'])

### TABLE: `workflow_environment_deployment_log` (8 cols, 0 FKs, 3 indexes)
  - `deployment_id` `TEXT` PK NOT NULL
  - `tenant_id` `TEXT` NOT NULL
  - `environment_id` `TEXT` NOT NULL
  - `rule_id` `TEXT` NOT NULL
  - `version_id` `TEXT` NOT NULL
  - `status` `TEXT` NOT NULL
  - `timestamp` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - `details_json` `TEXT` NOT NULL DEFAULT '{}'
  - INDEX `idx_wf_env_deploy_tenant` (cols=['tenant_id', None])
  - INDEX `idx_wf_env_deploy_env_time` (cols=['environment_id', None])

### TABLE: `workflow_environments` (5 cols, 0 FKs, 1 indexes)
  - `environment_id` `TEXT` PK NOT NULL
  - `environment_name` `TEXT` NOT NULL
  - `environment_type` `TEXT` NOT NULL
  - `is_active` `INTEGER` NOT NULL DEFAULT 1
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')

### TABLE: `workflow_rule_approvals` (8 cols, 0 FKs, 2 indexes)
  - `approval_id` `TEXT` PK NOT NULL
  - `rule_id` `TEXT` NOT NULL
  - `version_id` `TEXT` NOT NULL
  - `approved_by` `TEXT` NOT NULL DEFAULT ''
  - `approval_status` `TEXT` NOT NULL DEFAULT 'PENDING'
  - `approval_time` `TEXT` 
  - `requested_by` `TEXT` NOT NULL DEFAULT ''
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - INDEX `idx_workflow_rule_approvals_rule` (cols=['rule_id', 'approval_status'])

### TABLE: `workflow_rule_change_log` (7 cols, 0 FKs, 2 indexes)
  - `change_id` `TEXT` PK NOT NULL
  - `rule_id` `TEXT` NOT NULL
  - `changed_by` `TEXT` NOT NULL DEFAULT ''
  - `change_type` `TEXT` NOT NULL
  - `previous_value` `TEXT` NOT NULL DEFAULT ''
  - `new_value` `TEXT` NOT NULL DEFAULT ''
  - `changed_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - INDEX `idx_workflow_rule_change_rule` (cols=['rule_id', 'changed_at'])

### TABLE: `workflow_rule_deployment_log` (8 cols, 0 FKs, 2 indexes)
  - `deployment_id` `TEXT` PK NOT NULL
  - `rule_id` `TEXT` NOT NULL
  - `version_id` `TEXT` NOT NULL
  - `deployed_by` `TEXT` NOT NULL
  - `deployment_status` `TEXT` NOT NULL
  - `deployment_time` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - `rollback_flag` `INTEGER` NOT NULL DEFAULT 0
  - `details_json` `TEXT` NOT NULL DEFAULT '{}'
  - INDEX `idx_workflow_rule_deploy_rule` (cols=['rule_id', None])

### TABLE: `workflow_rule_staging` (6 cols, 0 FKs, 2 indexes)
  - `staging_id` `TEXT` PK NOT NULL
  - `rule_id` `TEXT` NOT NULL
  - `version_id` `TEXT` NOT NULL
  - `staging_environment` `TEXT` NOT NULL DEFAULT 'default'
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - `status` `TEXT` NOT NULL DEFAULT 'DRAFT'
  - INDEX `idx_workflow_rule_staging_rule` (cols=['rule_id', None])

### TABLE: `workflow_rule_versions` (10 cols, 0 FKs, 4 indexes)
  - `version_id` `TEXT` PK NOT NULL
  - `rule_id` `TEXT` NOT NULL
  - `tenant_id` `TEXT` NOT NULL DEFAULT 'tenant-default'
  - `environment_id` `TEXT` NOT NULL DEFAULT 'env-prod'
  - `version_number` `INTEGER` NOT NULL
  - `rule_definition_json` `TEXT` NOT NULL
  - `created_by` `TEXT` NOT NULL DEFAULT ''
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - `is_active` `INTEGER` NOT NULL DEFAULT 0
  - `change_reason` `TEXT` NOT NULL DEFAULT ''
  - INDEX `idx_workflow_rule_versions_one_active` (UNIQUE cols=['tenant_id', 'environment_id', 'rule_id'])
  - INDEX `idx_workflow_rule_versions_rule_env` (cols=['tenant_id', 'environment_id', 'rule_id', 'version_number'])
  - INDEX `sqlite_autoindex_workflow_rule_versions_2` (UNIQUE cols=['tenant_id', 'environment_id', 'rule_id', 'version_number'])

### TABLE: `workflow_simulation_reports` (11 cols, 0 FKs, 2 indexes)
  - `id` `TEXT` PK NOT NULL
  - `user_id` `TEXT` NOT NULL DEFAULT ''
  - `started_at` `TEXT` NOT NULL
  - `completed_at` `TEXT` NOT NULL
  - `inserted` `INTEGER` NOT NULL DEFAULT 0
  - `cleaned` `INTEGER` NOT NULL DEFAULT 0
  - `duration_ms` `INTEGER` NOT NULL DEFAULT 0
  - `peak_load` `INTEGER` NOT NULL DEFAULT 0
  - `failure_rate` `REAL` NOT NULL DEFAULT 0
  - `recovery_success` `INTEGER` NOT NULL DEFAULT 1
  - `details_json` `TEXT` NOT NULL DEFAULT '{}'
  - INDEX `idx_workflow_sim_completed` (cols=['completed_at'])

### TABLE: `workflow_tenants` (4 cols, 0 FKs, 1 indexes)
  - `tenant_id` `TEXT` PK NOT NULL
  - `tenant_name` `TEXT` NOT NULL
  - `tenant_status` `TEXT` NOT NULL DEFAULT 'ACTIVE'
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')

## Domain 12. Background Job Monitoring & Production Observability (19 objects)

### TABLE: `daily_missed_job_metrics` (6 cols, 0 FKs, 1 indexes)
  - `metric_date` `TEXT` PK NOT NULL
  - `missed_runs` `INTEGER` NOT NULL DEFAULT 0
  - `recovery_success` `INTEGER` NOT NULL DEFAULT 0
  - `recovery_failures` `INTEGER` NOT NULL DEFAULT 0
  - `drift_warnings` `INTEGER` NOT NULL DEFAULT 0
  - `updated_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')

### TABLE: `performance_timing_metrics` (6 cols, 0 FKs, 2 indexes)
  - `id` `TEXT` PK NOT NULL
  - `category` `TEXT` NOT NULL
  - `entity_id` `TEXT` 
  - `duration_ms` `INTEGER` NOT NULL
  - `recorded_at` `TEXT` NOT NULL
  - `details_json` `TEXT` NOT NULL DEFAULT '{}'
  - INDEX `idx_performance_timing_cat` (cols=['category', None])

### TABLE: `system_reliability_score` (8 cols, 0 FKs, 0 indexes)
  - `id` `INTEGER` PK
  - `score` `REAL` NOT NULL
  - `success_rate` `REAL` NOT NULL
  - `failure_rate` `REAL` NOT NULL
  - `recovery_success_rate` `REAL` NOT NULL
  - `sample_window_days` `INTEGER` NOT NULL DEFAULT 30
  - `updated_at` `TEXT` NOT NULL
  - `factors_json` `TEXT` NOT NULL DEFAULT '{}'

### TABLE: `workflow_alert_signal_log` (7 cols, 0 FKs, 3 indexes)
  - `id` `TEXT` PK NOT NULL
  - `created_at` `TEXT` NOT NULL
  - `signal_type` `TEXT` NOT NULL
  - `severity` `TEXT` NOT NULL
  - `entity_id` `TEXT` 
  - `message` `TEXT` NOT NULL
  - `details_json` `TEXT` NOT NULL DEFAULT '{}'
  - INDEX `idx_workflow_alert_signal_severity` (cols=['severity', None])
  - INDEX `idx_workflow_alert_signal_created` (cols=[None])

### TABLE: `workflow_background_jobs` (10 cols, 0 FKs, 1 indexes)
  - `job_id` `TEXT` PK NOT NULL
  - `job_name` `TEXT` NOT NULL
  - `job_type` `TEXT` NOT NULL
  - `schedule_type` `TEXT` NOT NULL
  - `is_enabled` `INTEGER` NOT NULL DEFAULT 1
  - `expected_duration_ms` `INTEGER` NOT NULL DEFAULT 120000
  - `max_retries` `INTEGER` NOT NULL DEFAULT 3
  - `retry_delay_sec` `INTEGER` NOT NULL DEFAULT 120
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - `updated_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')

### TABLE: `workflow_job_alert_log` (7 cols, 0 FKs, 2 indexes)
  - `id` `TEXT` PK NOT NULL
  - `job_id` `TEXT` 
  - `execution_id` `TEXT` 
  - `alert_level` `TEXT` NOT NULL
  - `message` `TEXT` NOT NULL
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - `details_json` `TEXT` NOT NULL DEFAULT '{}'
  - INDEX `idx_wf_job_alert_time` (cols=[None])

### TABLE: `workflow_job_dependencies` (4 cols, 0 FKs, 2 indexes)
  - `id` `TEXT` PK NOT NULL
  - `parent_job_id` `TEXT` NOT NULL
  - `dependent_job_id` `TEXT` NOT NULL
  - `dependency_type` `TEXT` NOT NULL
  - INDEX `sqlite_autoindex_workflow_job_dependencies_2` (UNIQUE cols=['parent_job_id', 'dependent_job_id'])

### TABLE: `workflow_job_execution_log` (9 cols, 0 FKs, 2 indexes)
  - `execution_id` `TEXT` PK NOT NULL
  - `job_id` `TEXT` NOT NULL
  - `started_at` `TEXT` NOT NULL
  - `completed_at` `TEXT` 
  - `status` `TEXT` NOT NULL
  - `records_processed` `INTEGER` NOT NULL DEFAULT 0
  - `error_message` `TEXT` 
  - `execution_time_ms` `INTEGER` 
  - `retry_count` `INTEGER` NOT NULL DEFAULT 0
  - INDEX `idx_wf_job_exec_job_time` (cols=['job_id', None])

### TABLE: `workflow_job_failure_alerts` (5 cols, 0 FKs, 2 indexes)
  - `alert_id` `TEXT` PK NOT NULL
  - `job_id` `TEXT` NOT NULL
  - `detected_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - `alert_type` `TEXT` NOT NULL
  - `details_json` `TEXT` NOT NULL DEFAULT '{}'
  - INDEX `idx_wf_job_failure_job_time` (cols=['job_id', None])

### TABLE: `workflow_job_manual_override_log` (6 cols, 0 FKs, 2 indexes)
  - `id` `TEXT` PK NOT NULL
  - `job_id` `TEXT` NOT NULL
  - `action` `TEXT` NOT NULL
  - `reason` `TEXT` 
  - `caller_role` `TEXT` NOT NULL
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - INDEX `idx_wf_job_manual_override_job_time` (cols=['job_id', None])

### TABLE: `workflow_job_missed_alerts` (6 cols, 0 FKs, 2 indexes)
  - `alert_id` `TEXT` PK NOT NULL
  - `job_id` `TEXT` NOT NULL
  - `expected_time` `TEXT` NOT NULL
  - `detected_time` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - `recovery_triggered` `INTEGER` NOT NULL DEFAULT 0
  - `status` `TEXT` NOT NULL DEFAULT 'PENDING'
  - INDEX `idx_wf_job_missed_job_status` (cols=['job_id', 'status', None])

### TABLE: `workflow_job_performance_metrics` (10 cols, 0 FKs, 2 indexes)
  - `id` `TEXT` PK NOT NULL
  - `job_id` `TEXT` NOT NULL
  - `window_label` `TEXT` NOT NULL
  - `avg_execution_ms` `REAL` 
  - `max_execution_ms` `REAL` 
  - `records_processed_total` `INTEGER` 
  - `failure_rate` `REAL` 
  - `retry_rate` `REAL` 
  - `samples` `INTEGER` 
  - `updated_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - INDEX `sqlite_autoindex_workflow_job_performance_metrics_2` (UNIQUE cols=['job_id', 'window_label'])

### TABLE: `workflow_job_recovery_log` (7 cols, 0 FKs, 2 indexes)
  - `recovery_id` `TEXT` PK NOT NULL
  - `job_id` `TEXT` NOT NULL
  - `alert_id` `TEXT` 
  - `recovery_time` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - `result` `TEXT` NOT NULL
  - `attempt_count` `INTEGER` NOT NULL DEFAULT 1
  - `error_message` `TEXT` 
  - INDEX `idx_wf_job_recovery_job_time` (cols=['job_id', None])

### TABLE: `workflow_job_recovery_score` (6 cols, 0 FKs, 1 indexes)
  - `job_id` `TEXT` PK NOT NULL
  - `score` `REAL` NOT NULL DEFAULT 0
  - `missed_jobs` `INTEGER` NOT NULL DEFAULT 0
  - `recovered_jobs` `INTEGER` NOT NULL DEFAULT 0
  - `window_days` `INTEGER` NOT NULL DEFAULT 30
  - `updated_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')

### TABLE: `workflow_job_reliability_score` (7 cols, 0 FKs, 1 indexes)
  - `job_id` `TEXT` PK NOT NULL
  - `score` `REAL` NOT NULL
  - `success_rate` `REAL` 
  - `failure_rate` `REAL` 
  - `retry_frequency` `REAL` 
  - `sample_executions` `INTEGER` 
  - `updated_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')

### TABLE: `workflow_job_schedule_expectations` (8 cols, 0 FKs, 1 indexes)
  - `job_id` `TEXT` PK NOT NULL
  - `expected_interval_minutes` `INTEGER` NOT NULL
  - `grace_period_minutes` `INTEGER` NOT NULL
  - `last_expected_run_at` `TEXT` 
  - `max_recovery_attempts` `INTEGER` NOT NULL DEFAULT 5
  - `recovery_delay_sec` `INTEGER` NOT NULL DEFAULT 120
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
  - `updated_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')

### TABLE: `workflow_maintenance_history` (7 cols, 0 FKs, 2 indexes)
  - `run_id` `TEXT` PK NOT NULL
  - `job_name` `TEXT` NOT NULL
  - `started_at` `TEXT` NOT NULL
  - `completed_at` `TEXT` NOT NULL
  - `status` `TEXT` NOT NULL
  - `records_processed` `INTEGER` NOT NULL DEFAULT 0
  - `errors_detected` `INTEGER` NOT NULL DEFAULT 0
  - INDEX `idx_workflow_maint_completed` (cols=['completed_at'])

### TABLE: `workflow_runtime_metrics` (9 cols, 0 FKs, 0 indexes)
  - `id` `INTEGER` PK
  - `jobs_executed` `INTEGER` NOT NULL DEFAULT 0
  - `jobs_failed` `INTEGER` NOT NULL DEFAULT 0
  - `jobs_recovered` `INTEGER` NOT NULL DEFAULT 0
  - `deployments_blocked` `INTEGER` NOT NULL DEFAULT 0
  - `deployments_succeeded` `INTEGER` NOT NULL DEFAULT 0
  - `recovery_attempts` `INTEGER` NOT NULL DEFAULT 0
  - `risk_evaluations` `INTEGER` NOT NULL DEFAULT 0
  - `updated_at` `TEXT` NOT NULL

### TABLE: `workflow_structured_event_log` (7 cols, 0 FKs, 3 indexes)
  - `id` `TEXT` PK NOT NULL
  - `timestamp` `TEXT` NOT NULL
  - `module` `TEXT` NOT NULL
  - `event_type` `TEXT` NOT NULL
  - `entity_id` `TEXT` 
  - `severity` `TEXT` NOT NULL
  - `details_json` `TEXT` NOT NULL DEFAULT '{}'
  - INDEX `idx_workflow_structured_event_module` (cols=['module', None])
  - INDEX `idx_workflow_structured_event_ts` (cols=[None])

## Domain 13. Incident Management, Correlation, Stabilization & Forecasting (29 objects)

### TABLE: `daily_workflow_health_score` (4 cols, 0 FKs, 1 indexes)
  - `snapshot_date` `TEXT` PK NOT NULL
  - `health_score` `REAL` NOT NULL
  - `factors_json` `TEXT` NOT NULL DEFAULT '{}'
  - `created_at` `TEXT` NOT NULL DEFAULT strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')

### TABLE: `system_integrity_snapshot` (3 cols, 0 FKs, 2 indexes)
  - `snapshot_date` `TEXT` PK NOT NULL
  - `details_json` `TEXT` NOT NULL
  - `updated_at` `TEXT` NOT NULL
  - INDEX `idx_system_integrity_snapshot_updated` (cols=[None])

### TABLE: `workflow_failure_burst_log` (12 cols, 0 FKs, 2 indexes)
  - `burst_id` `TEXT` PK NOT NULL
  - `source_module` `TEXT` NOT NULL
  - `event_type` `TEXT` NOT NULL
  - `burst_start_time` `TEXT` NOT NULL
  - `burst_end_time` `TEXT` NOT NULL
  - `event_count` `INTEGER` NOT NULL
  - `baseline_rate` `REAL` NOT NULL
  - `current_rate` `REAL` NOT NULL
  - `severity` `TEXT` NOT NULL
  - `root_cause_hint` `TEXT` NOT NULL DEFAULT ''
  - `details_json` `TEXT` NOT NULL DEFAULT '{}'
  - `confidence_score` `REAL` NOT NULL DEFAULT 0.72
  - INDEX `idx_workflow_failure_burst_module_time` (cols=['source_module', 'event_type', None])

### TABLE: `workflow_failure_forecast` (14 cols, 0 FKs, 2 indexes)
  - `forecast_id` `TEXT` PK NOT NULL
  - `source_module` `TEXT` NOT NULL
  - `event_type` `TEXT` NOT NULL
  - `forecast_time` `TEXT` NOT NULL
  - `predicted_failure_probability` `REAL` NOT NULL
  - `confidence_score` `REAL` NOT NULL
  - `forecast_horizon_minutes` `INTEGER` NOT NULL DEFAULT 30
  - `details_json` `TEXT` NOT NULL DEFAULT '{}'
  - `primary_trigger` `TEXT` NOT NULL DEFAULT ''
  - `secondary_triggers_json` `TEXT` NOT NULL DEFAULT '{}'
  - `trend_summary` `TEXT` NOT NULL DEFAULT ''
  - `recommended_actions_json` `TEXT` NOT NULL DEFAULT '[]'
  - `action_priority` `TEXT` NOT NULL DEFAULT 'MEDIUM'
  - `action_effectiveness_score` `REAL` 
  - INDEX `idx_wf_failure_forecast_scope_time` (cols=['source_module', 'event_type', None])

### TABLE: `workflow_failure_forecast_metrics` (6 cols, 0 FKs, 1 indexes)
  - `metric_date` `TEXT` PK NOT NULL
  - `forecasts_generated` `INTEGER` NOT NULL DEFAULT 0
  - `forecast_accuracy` `REAL` NOT NULL DEFAULT 0
  - `forecast_false_positive_rate` `REAL` NOT NULL DEFAULT 0
  - `prediction_accuracy_score` `REAL` NOT NULL DEFAULT 0
  - `updated_at` `TEXT` NOT NULL

### TABLE: `workflow_forecast_action_log` (5 cols, 0 FKs, 4 indexes)
  - `log_id` `TEXT` PK NOT NULL
  - `forecast_id` `TEXT` NOT NULL
  - `action_taken` `TEXT` NOT NULL
  - `acknowledged_at` `TEXT` NOT NULL
  - `caller_role` `TEXT` NOT NULL DEFAULT ''
  - INDEX `uq_wf_forecast_action_log_forecast_id` (UNIQUE cols=['forecast_id'])
  - INDEX `idx_wf_forecast_action_log_time` (cols=[None])
  - INDEX `idx_wf_forecast_action_log_forecast` (cols=['forecast_id'])

### TABLE: `workflow_forecast_action_metrics` (5 cols, 0 FKs, 1 indexes)
  - `metric_date` `TEXT` PK NOT NULL
  - `actions_generated` `INTEGER` NOT NULL DEFAULT 0
  - `actions_acknowledged` `INTEGER` NOT NULL DEFAULT 0
  - `actions_effective` `INTEGER` NOT NULL DEFAULT 0
  - `updated_at` `TEXT` NOT NULL

### TABLE: `workflow_forecast_explanation_metrics` (5 cols, 0 FKs, 1 indexes)
  - `metric_date` `TEXT` PK NOT NULL
  - `explanations_generated` `INTEGER` NOT NULL DEFAULT 0
  - `accurate_explanations` `INTEGER` NOT NULL DEFAULT 0
  - `misleading_explanations` `INTEGER` NOT NULL DEFAULT 0
  - `updated_at` `TEXT` NOT NULL

### TABLE: `workflow_forecast_explanation_score` (5 cols, 0 FKs, 0 indexes)
  - `id` `INTEGER` PK NOT NULL
  - `explanation_accuracy_score` `REAL` NOT NULL DEFAULT 0
  - `accurate_explanations` `INTEGER` NOT NULL DEFAULT 0
  - `total_explanations` `INTEGER` NOT NULL DEFAULT 0
  - `updated_at` `TEXT` NOT NULL

### TABLE: `workflow_forecast_feedback` (6 cols, 0 FKs, 3 indexes)
  - `feedback_id` `TEXT` PK NOT NULL
  - `forecast_id` `TEXT` NOT NULL
  - `feedback_kind` `TEXT` NOT NULL
  - `notes` `TEXT` 
  - `created_at` `TEXT` NOT NULL
  - `caller_role` `TEXT` NOT NULL DEFAULT ''
  - INDEX `idx_wf_forecast_feedback_created` (cols=[None])
  - INDEX `idx_wf_forecast_feedback_forecast` (cols=['forecast_id'])

### TABLE: `workflow_forecast_risk_score` (5 cols, 0 FKs, 0 indexes)
  - `id` `INTEGER` PK NOT NULL
  - `forecast_risk_score` `REAL` NOT NULL DEFAULT 0
  - `high_risk_forecasts` `INTEGER` NOT NULL DEFAULT 0
  - `total_forecasts` `INTEGER` NOT NULL DEFAULT 0
  - `updated_at` `TEXT` NOT NULL

### TABLE: `workflow_incident_correlation_metrics` (7 cols, 0 FKs, 1 indexes)
  - `metric_date` `TEXT` PK NOT NULL
  - `alerts_grouped` `INTEGER` NOT NULL DEFAULT 0
  - `incidents_created` `INTEGER` NOT NULL DEFAULT 0
  - `noise_reduction_ratio` `REAL` NOT NULL DEFAULT 0
  - `burst_signals_emitted` `INTEGER` NOT NULL DEFAULT 0
  - `updated_at` `TEXT` NOT NULL
  - `bursts_detected` `INTEGER` NOT NULL DEFAULT 0

### TABLE: `workflow_incident_history` (6 cols, 1 FKs, 2 indexes)
  - `history_id` `TEXT` PK NOT NULL
  - `incident_id` `TEXT` NOT NULL
  - `event_type` `TEXT` NOT NULL
  - `event_timestamp` `TEXT` NOT NULL
  - `notes` `TEXT` 
  - `details_json` `TEXT` NOT NULL DEFAULT '{}'
  - FK: `incident_id` -> `workflow_incidents(incident_id)` (ON UPDATE NO ACTION, ON DELETE CASCADE)
  - INDEX `idx_workflow_incident_history_incident` (cols=['incident_id', None])

### TABLE: `workflow_incident_metrics` (6 cols, 0 FKs, 1 indexes)
  - `metric_date` `TEXT` PK NOT NULL
  - `incidents_created_today` `INTEGER` NOT NULL DEFAULT 0
  - `incidents_resolved_today` `INTEGER` NOT NULL DEFAULT 0
  - `avg_resolution_time` `REAL` NOT NULL DEFAULT 0
  - `critical_incident_count` `INTEGER` NOT NULL DEFAULT 0
  - `updated_at` `TEXT` NOT NULL

### TABLE: `workflow_incident_noise_score` (5 cols, 0 FKs, 1 indexes)
  - `metric_date` `TEXT` PK NOT NULL
  - `noise_score` `REAL` NOT NULL DEFAULT 0
  - `total_alerts` `INTEGER` NOT NULL DEFAULT 0
  - `alerts_grouped` `INTEGER` NOT NULL DEFAULT 0
  - `updated_at` `TEXT` NOT NULL

### TABLE: `workflow_incident_persistence` (10 cols, 0 FKs, 3 indexes)
  - `persistence_id` `TEXT` PK NOT NULL
  - `correlation_id` `TEXT` 
  - `incident_id` `TEXT` NOT NULL
  - `source_module` `TEXT` NOT NULL
  - `event_type` `TEXT` NOT NULL
  - `persistence_detected_at` `TEXT` NOT NULL
  - `failure_rate` `REAL` NOT NULL
  - `expected_rate` `REAL` NOT NULL
  - `severity` `TEXT` NOT NULL
  - `details_json` `TEXT` NOT NULL DEFAULT '{}'
  - INDEX `idx_wf_persistence_scope` (cols=['source_module', 'event_type'])
  - INDEX `idx_wf_persistence_detected` (cols=[None])

### TABLE: `workflow_incident_regression` (9 cols, 0 FKs, 3 indexes)
  - `regression_id` `TEXT` PK NOT NULL
  - `correlation_id` `TEXT` 
  - `incident_id` `TEXT` NOT NULL
  - `source_module` `TEXT` NOT NULL
  - `event_type` `TEXT` NOT NULL
  - `regression_detected_at` `TEXT` NOT NULL
  - `time_since_stabilization_minutes` `INTEGER` NOT NULL
  - `severity` `TEXT` NOT NULL
  - `details_json` `TEXT` NOT NULL DEFAULT '{}'
  - INDEX `idx_wf_regression_scope` (cols=['source_module', 'event_type'])
  - INDEX `idx_wf_regression_detected` (cols=[None])

### TABLE: `workflow_incident_regression_metrics` (5 cols, 0 FKs, 1 indexes)
  - `metric_date` `TEXT` PK NOT NULL
  - `regressions_detected` `INTEGER` NOT NULL DEFAULT 0
  - `avg_regression_time_minutes` `REAL` NOT NULL DEFAULT 0
  - `regression_frequency` `REAL` NOT NULL DEFAULT 0
  - `updated_at` `TEXT` NOT NULL

### TABLE: `workflow_incident_stabilization` (10 cols, 0 FKs, 2 indexes)
  - `stabilization_id` `TEXT` PK NOT NULL
  - `correlation_id` `TEXT` 
  - `incident_id` `TEXT` 
  - `source_module` `TEXT` NOT NULL
  - `event_type` `TEXT` NOT NULL
  - `stabilization_start` `TEXT` NOT NULL
  - `stabilization_confirmed` `TEXT` 
  - `stability_duration_minutes` `INTEGER` NOT NULL DEFAULT 0
  - `confidence_score` `REAL` NOT NULL DEFAULT 0
  - `created_at` `TEXT` NOT NULL
  - INDEX `idx_wf_stabilization_scope` (cols=['source_module', 'event_type', None])

### TABLE: `workflow_incident_stabilization_metrics` (6 cols, 0 FKs, 1 indexes)
  - `metric_date` `TEXT` PK NOT NULL
  - `stabilizations_detected` `INTEGER` NOT NULL DEFAULT 0
  - `avg_stabilization_time` `REAL` NOT NULL DEFAULT 0
  - `false_recovery_rate` `REAL` NOT NULL DEFAULT 0
  - `stability_confidence_avg` `REAL` NOT NULL DEFAULT 0
  - `updated_at` `TEXT` NOT NULL

### TABLE: `workflow_incident_suppression` (13 cols, 0 FKs, 2 indexes)
  - `suppression_id` `TEXT` PK NOT NULL
  - `correlation_id` `TEXT` 
  - `incident_id` `TEXT` 
  - `source_module` `TEXT` NOT NULL
  - `event_type` `TEXT` NOT NULL
  - `suppression_start` `TEXT` NOT NULL
  - `suppression_end` `TEXT` NOT NULL
  - `last_activity_at` `TEXT` NOT NULL
  - `suppressed_event_count` `INTEGER` NOT NULL DEFAULT 0
  - `confidence_score` `REAL` NOT NULL DEFAULT 0.55
  - `release_history_logged` `INTEGER` NOT NULL DEFAULT 0
  - `reason` `TEXT` NOT NULL DEFAULT ''
  - `created_at` `TEXT` NOT NULL
  - INDEX `idx_wf_suppression_lookup` (cols=['source_module', 'event_type', None])

### TABLE: `workflow_incident_suppression_metrics` (6 cols, 0 FKs, 1 indexes)
  - `metric_date` `TEXT` PK NOT NULL
  - `alerts_suppressed` `INTEGER` NOT NULL DEFAULT 0
  - `suppression_windows` `INTEGER` NOT NULL DEFAULT 0
  - `noise_reduction_gain` `REAL` NOT NULL DEFAULT 0
  - `confidence_score` `REAL` NOT NULL DEFAULT 0.55
  - `updated_at` `TEXT` NOT NULL

### TABLE: `workflow_incidents` (14 cols, 0 FKs, 6 indexes)
  - `incident_id` `TEXT` PK NOT NULL
  - `created_at` `TEXT` NOT NULL
  - `severity` `TEXT` NOT NULL
  - `status` `TEXT` NOT NULL
  - `error_context_json` `TEXT` NOT NULL DEFAULT '{}'
  - `source_module` `TEXT` NOT NULL
  - `linked_alert_id` `TEXT` NOT NULL
  - `trigger_event_type` `TEXT` NOT NULL
  - `correlation_id` `TEXT` NOT NULL
  - `resolved_at` `TEXT` 
  - `root_cause_summary` `TEXT` 
  - `correlation_key` `TEXT` NOT NULL DEFAULT ''
  - `correlated_event_count` `INTEGER` NOT NULL DEFAULT 1
  - `last_correlated_at` `TEXT` 
  - INDEX `idx_workflow_incidents_correlation_key` (cols=['correlation_key', 'status'])
  - INDEX `idx_workflow_incidents_correlation` (cols=['correlation_id'])
  - INDEX `idx_workflow_incidents_created_at` (cols=[None])
  - INDEX `idx_workflow_incidents_severity` (cols=['severity'])
  - INDEX `idx_workflow_incidents_status` (cols=['status'])

### TABLE: `workflow_persistence_risk_score` (5 cols, 0 FKs, 0 indexes)
  - `id` `INTEGER` PK NOT NULL
  - `persistence_risk` `REAL` NOT NULL DEFAULT 0
  - `persistent_failures_detected` `INTEGER` NOT NULL DEFAULT 0
  - `total_incidents` `INTEGER` NOT NULL DEFAULT 0
  - `updated_at` `TEXT` NOT NULL

### TABLE: `workflow_persistent_failure_metrics` (5 cols, 0 FKs, 1 indexes)
  - `metric_date` `TEXT` PK NOT NULL
  - `persistent_failures_detected` `INTEGER` NOT NULL DEFAULT 0
  - `avg_persistence_duration` `REAL` NOT NULL DEFAULT 0
  - `persistence_frequency` `REAL` NOT NULL DEFAULT 0
  - `updated_at` `TEXT` NOT NULL

### TABLE: `workflow_preventive_reliability_score` (5 cols, 0 FKs, 0 indexes)
  - `id` `INTEGER` PK NOT NULL
  - `preventive_reliability_score` `REAL` NOT NULL DEFAULT 0
  - `prevented_failures` `INTEGER` NOT NULL DEFAULT 0
  - `total_forecasts` `INTEGER` NOT NULL DEFAULT 0
  - `updated_at` `TEXT` NOT NULL

### TABLE: `workflow_regression_risk_score` (5 cols, 0 FKs, 0 indexes)
  - `id` `INTEGER` PK NOT NULL
  - `regression_risk` `REAL` NOT NULL DEFAULT 0
  - `regressions_detected` `INTEGER` NOT NULL DEFAULT 0
  - `stabilizations_detected` `INTEGER` NOT NULL DEFAULT 0
  - `updated_at` `TEXT` NOT NULL

### TABLE: `workflow_structured_regression_metrics` (5 cols, 0 FKs, 1 indexes)
  - `metric_date` `TEXT` PK NOT NULL
  - `structured_regressions_detected` `INTEGER` NOT NULL DEFAULT 0
  - `avg_structured_regression_time` `REAL` NOT NULL DEFAULT 0
  - `structured_regression_ratio` `REAL` NOT NULL DEFAULT 0
  - `updated_at` `TEXT` NOT NULL

### TABLE: `workflow_system_stability_score` (5 cols, 0 FKs, 0 indexes)
  - `id` `INTEGER` PK NOT NULL
  - `stability_score` `REAL` NOT NULL DEFAULT 0
  - `successful_stabilizations` `INTEGER` NOT NULL DEFAULT 0
  - `total_incidents` `INTEGER` NOT NULL DEFAULT 0
  - `updated_at` `TEXT` NOT NULL
