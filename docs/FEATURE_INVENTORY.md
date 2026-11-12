# Feature Inventory – Import Manager (Legacy Functional Specification)

> **Source of Truth:** Audited legacy codebase (`src/`, `src-tauri/src/`, `test-data/`).  
> Every module below covers all 11 required specification facets: **Existing Functionality**, **Business Purpose**, **User Operations**, **Input Fields**, **Validation Rules**, **Database Tables**, **Business Calculations**, **Output & Reports**, **Error Handling**, **Security Requirements**, and **Acceptance Tests**.

---

## Module 1: Dashboard & KPI Analytics

- **Existing Functionality:** Operational overview across active FY/month filters with KPI cards, monthly import/duty trends, supplier analysis, shipment status breakdown, recent activity feed, alert/exception summary, workflow health/efficiency scores, and cache/rebuild controls.
- **Business Purpose:** Real-time visibility into import pipeline volume, pending customs clearances, duty liabilities, expenses, and operational anomalies.
- **User Operations:** Filter by financial year/date range; inspect KPI cards & Recharts trend visualizations; drill into pending shipments/BOEs/exceptions; trigger cache invalidation or daily snapshot rebuild (`ops_admin`).
- **Input Fields:** Financial year selector, date range bounds, refresh trigger.
- **Validation Rules:** Date strings normalized to `YYYY-MM-DD`; cache TTL clamped/checked against `180s` freshness window.
- **Database Tables:** `shipments`, `suppliers`, `invoices`, `invoice_line_items`, `boe_details`, `boe_calculations`, `expenses`, `dashboard_metrics_cache`, `dashboard_kpi_snapshot`, `dashboard_monthly_snapshot`, `dashboard_activity_log`, `dashboard_exception_snapshot`, `dashboard_exception_sample_cache`, `dashboard_workflow_snapshot`, `dashboard_workflow_sample_cache`, `kpi_metadata`, `kpi_daily_snapshots`, `kpi_alert_rules`, `kpi_alert_log`.
- **Business Calculations:** Aggregates real DB sums/counts (zero fabricated values): total suppliers, shipments (pending vs completed), pending BOEs, total import value, customs duty totals, expenses totals, monthly buckets, KPI YoY/MoM deltas, workflow health & efficiency scores.
- **Output & Reports:** Interactive KPI cards, Recharts area/bar/pie charts, recent activity timeline, exception/alert badges.
- **Error Handling:** Falls back cleanly from snapshot cache to live query if stale/missing; structured IPC error strings on DB query failure.
- **Security Requirements:** Requires active authenticated desktop session (`dashboard.view` / `role_dashboard_permissions`); rebuild actions require admin role.
- **Acceptance Tests:** (1) Metrics match exact SQL sums across seeded shipments/BOEs/expenses; (2) Cache hit within 180s and invalidation on mutation; (3) FY filter changes aggregated totals accurately; (4) Empty DB renders zeroes, never mock numbers.

---

## Module 2: Supplier Management

- **Existing Functionality:** Full CRUD, soft-delete, pagination, search/sorting, banking details, CSV import/export, duplicate detection, and linked shipment/invoice history inspection.
- **Business Purpose:** Maintain canonical overseas vendor profiles, banking/SWIFT metadata, and settlement terms referenced across shipments and invoices.
- **User Operations:** List/search/paginate suppliers; add/edit supplier; view supplier details & linked shipments/invoices; soft-delete (move to Recycle Bin); import CSV; export CSV.
- **Input Fields:** `id`, `supplier_name`, `short_name`, `country`, `email`, `phone`, `beneficiary_name`, `bank_name`, `branch`, `bank_address`, `account_no`, `iban`, `swift_code`, `is_active`. (Legacy CSV 13 headers: `supplierName,shortName,country,email,phone,beneficiaryName,bankName,branch,bankAddress,accountNo,iban,swiftCode,isActive`).
- **Validation Rules:** Required: `supplierName`, `country`; unique supplier identity / duplicate warning on matching `supplierName`; CSV file `<= 10 MB`, `<= 10,000` rows, column length `<= 1,000` chars; `DOMPurify` strip on HTML tags during export. *(Next Gen fix: use RFC 4180 CSV parser instead of legacy naive `.split(',')` so quoted addresses/names do not split columns).*
- **Database Tables:** `suppliers`, `countries`, `shipments`, `invoices`, `audit_logs`.
- **Business Calculations:** Historical shipment count, total invoice value per supplier, paginated slice counts.
- **Output & Reports:** Paginated DataTable (default 10 rows/page configurable in Settings), supplier detail drawer/view, CSV export & downloadable CSV template.
- **Error Handling:** Duplicate/validation error messages returned inline; soft-delete blocked or warned when active references exist during hard delete.
- **Security Requirements:** Authenticated session + `supplier.*` RBAC permissions; audit logging of create/update/soft-delete.
- **Acceptance Tests:** (1) Create/edit/soft-delete supplier; (2) CSV import with valid/invalid/quoted-comma rows; (3) Duplicate supplier name detection; (4) Pagination + sort + search accuracy.

---

## Module 3: Shipment Management

- **Existing Functionality:** Create, view, edit, freeze/unfreeze, status transitions, bulk CSV/Excel import, date normalization, search/filter/pagination, related invoices/BOEs/expenses inspection, soft-delete.
- **Business Purpose:** Central operational entity tracking physical consignments from documentation receipt through transit, customs clearance, delivery, and financial freeze.
- **User Operations:** Create shipment; update status/dates/vessel/container; attach/inspect invoices, BOEs, and expenses; freeze shipment (`freeze_shipment`); import shipment CSV batch; filter/sort/paginate.
- **Input Fields:** `id`, `supplier_id`, `invoice_number`, `invoice_date`, `goods_category`, `invoice_value`, `invoice_currency`, `incoterm`, `shipment_mode`, `shipment_type`, `bl_awb_number`, `bl_awb_date`, `vessel_name`, `container_number`, `gross_weight_kg`, `etd`, `eta`, `status`, `date_of_delivery`, `is_frozen`.
- **Validation Rules:** Required `invoiceNumber`, `invoiceDate`, `invoiceValue`; allowed `status` values (`docs-rcvd`, `docu-received`, `in-transit`, `customs-clearance`, `ready-dly`, `delivered`, `cancelled`, `finalized`, `closed`, `completed`); `is_frozen == 1` locks commercial edits; dates normalized (`YYYY-MM-DD`) including Excel serial date conversion (e.g. `45124`); CSV delimiter auto-detection (`\t`, `;`, `,`) and `HEADER_ALIASES` canonicalization.
- **Database Tables:** `shipments`, `shipment_import_log`, `shipment_date_normalization_audit`, `shipment_compliance_score`, `incoterms`, `shipment_modes`, `shipment_types`, `shipment_statuses`, `currencies`, `categories`, `audit_logs`.
- **Business Calculations:** Compliance scoring, automatic status transitions (`in-transit` on invoice add; `customs-clearance` on BOE add; `ready-dly` when `be_date + 7 days <= today`), ETA overdue exception detection.
- **Output & Reports:** Shipment list & detail view, CSV import preview & validation summary, UTF-8 BOM `\r\n` template CSV, print/export views.
- **Error Handling:** Per-row CSV validation error report (`shipment_import_log`), frozen-shipment mutation rejection, invalid status error.
- **Security Requirements:** `shipment.*` RBAC permissions; freeze state enforced in backend before mutation; activity & audit log trails.
- **Acceptance Tests:** (1) Full lifecycle `docs-rcvd -> in-transit -> customs-clearance -> ready-dly -> delivered`; (2) Freeze blocks edits/invoice mutations; (3) CSV stream import with `\t`/`;`/`,` & Excel serial dates; (4) `ready-dly` 7-day BOE rule trigger.

---

## Module 4: Commercial Invoice Management & Invoice Wizard

- **Existing Functionality:** 3-step Invoice Wizard (`Invoice Header -> Invoice Lines -> Review & Save`), multi-line paste parser, draft persistence in `localStorage` (up to 20 drafts, 30-day TTL), unit price drift warnings vs Item Master, per-invoice decimal precision (`0` or `2` decimals), draft save vs finalize vs shipment freeze.
- **Business Purpose:** Capture accurate item-level commercial invoice values, HS codes, and base duty percentages before customs valuation and landed cost reporting.
- **User Operations:** Select active (non-terminal, non-frozen) shipment; enter/paste line items; lookup from Item Master; save as Draft; Finalize Invoice; optionally Finalize/Freeze Shipment; view/edit existing invoices.
- **Input Fields:** `invoice_no`, `invoice_date`, `shipment_id`, `supplier_id`, `currency`, line items (`part_no`, `description`, `unit`, `qty`, `unit_price`, `hsn_code`, `duty_percent`, `sws_percent`, `igst_percent`), precision settings (`lineTotalDecimals`, `invoiceTotalDecimals`).
- **Validation Rules:** Multi-line paste supports `\t`, `,`, `;`, `|` delimiters & auto-maps Formats A/B; rejects missing `partNumber`, `qty <= 0`, `unitPrice < 0`; **Finalize gate:** computed integer-cent line total must exactly match `shipment.invoice_value` (Draft save bypasses total match).
- **Database Tables:** `invoices`, `invoice_line_items`, `items`, `shipments`, `app_settings` (precision settings).
- **Business Calculations:** `line_total = round(qty * unit_price, lineTotalDecimals)`; `invoice_total = round(sum(line_totals), invoiceTotalDecimals)`; integer-cent comparison for Finalize check; automatically transitions shipment status from `docs-rcvd` to `in-transit` (if not `delivered`).
- **Output & Reports:** Invoice review summary, price discrepancy warning badges, draft switcher, invoice list.
- **Error Handling:** Blocks finalize with mismatch delta message when line sum != shipment invoice value; warns (non-blocking) when `unitPrice` diverges from Item Master; rejects edits on frozen shipments.
- **Security Requirements:** `invoice.*` RBAC permissions; backend precision validation (`0 | 2` only); audit logs.
- **Acceptance Tests:** (1) Multi-line paste formats A & B; (2) Draft save allowed with mismatched total vs Finalize blocked on mismatch; (3) Draft saved with 20-cap & restored cleanly; (4) Auto-transition shipment to `in-transit`.

---

## Module 5: Item Master & Lookup Options

- **Existing Functionality:** Item catalog CRUD, part number duplicate blocking, soft delete, search/pagination, photo path attachment metadata, bulk CSV/Excel import/export (`DOMPurify` stripped exports), and 13 lookup option tables (`units`, `currencies`, `countries`, `bcd_rates`, `sws_rates`, `igst_rates`, `categories`, `end_uses`, `purchase_uoms`, `incoterms`, `shipment_modes`, `shipment_types`, `shipment_statuses`).
- **Business Purpose:** Provide standardized HS codes, tax/duty baseline rates, weights, units, and prices for invoices, BOEs, and customs reconciliation.
- **User Operations:** Create/edit/soft-delete items; search by part no/description/HSN; import/export CSV; add dropdown options cleanly via modal/inline add.
- **Input Fields:** `id` (`ITM-XXX`), `part_number`, `item_description`, `unit`, `currency`, `unit_price`, `hsn_code`, `supplier_id`, `supplier_name`, `is_active`, `country_of_origin`, `bcd`, `sws`, `igst`, `technical_write_up`, `category`, `end_use`, `net_weight_kg`, `purchase_uom`, `gross_weight_per_uom_kg`, `photo_path`.
- **Validation Rules:** Required: `partNumber, itemDescription, unit, currency, unitPrice`; unique `partNumber` (intra-batch and DB check); case-insensitive `supplierName` lookup if `supplierId` omitted; `isActive` boolean parsing (`'false'`/`'0'` -> false, else true).
- **Database Tables:** `items`, all 13 lookup option tables, `suppliers`.
- **Business Calculations:** Sequential ID assignment (`ITM-001`, ...), weight per UOM metadata for logistics.
- **Output & Reports:** Paginated item catalog, 20-column CSV export & template CSV.
- **Error Handling:** Skips duplicate `partNumber` on import with per-row error/warning list; warns on unmatched `supplierName`.
- **Security Requirements:** `item.*` RBAC permissions; sanitized CSV exports; soft-delete audit logs.
- **Acceptance Tests:** (1) Duplicate `partNumber` rejected; (2) CSV import resolves `supplierId` by ID or Name; (3) Dynamic option creation immediately appears across forms.

---

## Module 6: Bill of Entry (BOE) Registry & Customs Duty Calculator

- **Existing Functionality:**
  - **BOE Registry:** Track customs Bills of Entry (`be_number`, `be_date`, `location`, `total_assessment_value`, `duty_amount`, `payment_date`, `duty_paid`, `challan_number`, `ref_id`, `transaction_id`), PDF/document attachments (`boe_attachments`), reconciliation statuses (`Awaiting BOE Data`, `Discrepancy Found`, `Reconciled`, `Investigation`, `Closed`), crash-recovery journal (`boe_write_recovery`).
  - **BOE Duty Entry Calculator:** Interactive & CSV-overridable line-by-line customs duty engine supporting `Standard`, `CEPA`, and `Rodtep` methods across virtualized line-item tables (`>25` rows virtualized), plus Excel (`.xlsx`) & 3-part ICEGATE-style PDF exports.
- **Business Purpose:** Compute exact customs assessable values and BCD/SWS/IGST duty totals, reconcile computed duties against customs challan `duty_paid`, and unlock landed cost calculations.
- **User Operations:** Register/edit BOE; upload attachments; select supplier/shipment/invoice in BOE Entry; enter `exchangeRate`, `freightCost`, `exwCost`, `insuranceRate`, `interest`; choose `Standard | CEPA | Rodtep` per item; import rate override CSV; verify BCD discrepancy warnings; link BOE & save calculation; export Excel/PDF.
- **Input Fields:** `supplierName`, `shipmentId`, `exchangeRate` (`>=0`, default `83.5`), `freightCost` (`>=0`, INR), `exwCost` (`>=0`, INR), `insuranceRate` (`>=0`, default `0.015%`), `interest` (`>=0`), per-item `{calculationMethod, boeBcdRate, boeSwsRate}`, override CSV (`partNo, calculationMethod, boeBcdRate, boeSwsRate, boeIgstRate`).
- **Validation Rules:**
  - `incoterm === 'CIF'` zero-locks and disables `freightCost`, `exwCost`, `insuranceRate`.
  - Override CSV skips rows with unknown `partNo` or where `boeIgstRate !== actualIgstRate`.
  - **Save Blockers:** (a) any item with `boeBcdRate > 0 && boeBcdRate > actualBcdRate`; (b) if `selectedBoeId` is linked, `round2(dutyPaid) !== round2(customsDutyTotal)`.
  - JSON payload guard: hard max `1,048,576` B, warn threshold `786,432` B (throttled after 3 streak).
- **Database Tables:** `boe_details`, `boe_calculations`, `boe_items`, `boe_attachments`, `boe_write_recovery`, `shipments`, `invoices`, `invoice_line_items`.
- **Business Calculations (exact precision rules):**
  - `itemForexValue = round(qty * unitPrice, 2)`; `itemValueInr = round(itemForexValue * exRate, 2)`
  - `itemFreight = round((freightCost / totalInvoiceValue) * itemForexValue, 2)` *(direct INR, not multiplied by `exRate`)*
  - `itemExw = round((exwCost / totalInvoiceValue) * itemForexValue, 2)` *(direct INR)*
  - `itemInsurance = round((itemValueInr + itemExw) * (insuranceRate / 100), 1)`
  - `assessableValue = round(itemValueInr + itemFreight + itemExw + itemInsurance, 1)`
  - `Standard`/`CEPA`: `bcd = av * boeBcd%`, `sws = bcd * boeSws%`, `igst = (av + bcd + sws) * boeIgst%`
  - `Rodtep`: `bcd = av * boeBcd%`, `sws = (av * actualBcd%) * boeSws%`, `igst = (av + av * actualBcd% + sws) * boeIgst%`
  - Line duties rounded to `1` decimal; `bcdTotal`, `swsTotal`, `igstTotal` rounded to `0` decimals (`Math.round`); `customsDutyTotal = Math.round(bcdTotal + swsTotal + igstTotal + interest)`.
- **Output & Reports:** 3-sheet Excel `BOE-CALC-<invoice>-<date>.xlsx` (`Summary`, `Item Duties`, `Input Parameters`); 3-part ICEGATE A4 PDF `BOE-CALC-<invoice>-<date>.pdf`. *(Next Gen improvement: label INR units clearly, display dynamic `invoiceCurrency`, and make PDF port/origin/UQC configurable).*
- **Error Handling:** Discrepancy banners, CSV row skip warnings, JSON size cap errors, crash-recovery reconciliation via `boe_write_recovery`.
- **Security Requirements:** `boe.*` RBAC permissions; server-side recalculation/reconciliation check; attachment path safety.
- **Acceptance Tests:** (1) Exact match against `Standard`, `CEPA`, `Rodtep` reference vectors; (2) CIF zero-lock enforcement; (3) Save blocked on `boeBcdRate > actualBcdRate` or `dutyPaid` mismatch; (4) `ready-dly` & `customs-clearance` shipment status side-effects; (5) Excel 2016 compatibility & PDF generation.

---

## Module 7: Import Expenses & Service Providers

- **Existing Functionality:** Manage Service Providers, Expense Types (with default CGST/SGST/IGST/TDS rates), multi-line Expense Invoice entry (integer paise & basis points, idempotency keys, duplicate merging), preview tax calculation, line editing/deletion, file attachments (`expense_attachments`), bulk CSV/Excel import, and admin repair tools.
- **Business Purpose:** Record customs clearance, freight forwarding, CHA, CFS, transport, and handling invoices per shipment in exact paise without floating-point drift, for statutory TDS/GST accounting and proportional Landed Cost allocation.
- **User Operations:** Select active shipment; enter multi-line provider invoice or single edit; preview paise tax breakdown; merge duplicate expense types on an invoice; import `.csv`/`.xlsx`/`.xls`; manage Expense Types & Service Providers.
- **Input Fields:** `shipment_id`, `service_provider_id`, `invoice_no`, `invoice_date`, `currency`, `idempotency_key`, lines (`expense_type_id`, `amount_paise`, `cgst_rate_bp`, `sgst_rate_bp`, `igst_rate_bp`, `tds_rate_bp`, `remarks`). Bulk template headers (11 cols): `Expense Type, Service Provider, Invoice No, Invoice Date, Amount, CGST Amount, SGST Amount, IGST Amount, TDS Amount, Total Amount, Remarks`.
- **Validation Rules:** `amount > 0`; taxes/TDS `>= 0`; exact case-insensitive match of `Expense Type` and `Service Provider` against master lists; idempotent insert via `idempotency_key`; duplicate line warning with 1-click combine. *(Next Gen fix: use RFC 4180 CSV parsing for CSV import and implement full idempotent update cleanly instead of legacy `"Update not implemented"` error).*
- **Database Tables:** `expenses`, `expense_invoices`, `expense_types`, `expense_attachments`, `service_providers`.
- **Business Calculations:** `10000 bp = 100%`; integer division `tax_paise = (amount_paise * rate_bp) / 10000`; `total_paise = amount_paise + cgst_paise + sgst_paise + igst_paise`; `net_paise = total_paise - tds_paise`.
- **Output & Reports:** Preview breakdown table, shipment expense summary card, bulk import status report, admin counts/orphan cleanup diagnostics.
- **Error Handling:** Rejects negative amounts, unknown provider/expense type names, missing invoice numbers, duplicate idempotency conflicts cleanly.
- **Security Requirements:** `expense.*` RBAC permissions; integer paise enforcement in Go backend; audit log entries.
- **Acceptance Tests:** (1) Paise/BP tax calculation matches integer arithmetic to the exact paisa; (2) Idempotency key prevents duplicate posting on double-submit; (3) Duplicate expense lines combine accurately; (4) CSV/XLSX bulk import with valid/invalid fixtures.

---

## Module 8: Landed Cost Report & Expense Reports

- **Existing Functionality:**
  - **Landed Cost Report:** Filterable report backed by `report_view` showing item-level landed cost per quantity (`LDC per qty`) combining Assessable Value, BCD, SWS, and proportionally allocated Basic Expenses (excluding IGST).
  - **Expense Reports:** 5 analytical views (`detailed`, `summary-by-type`, `summary-by-provider`, `summary-by-shipment`, `summary-by-month`) with paise min/max filters, Recharts charts, CSV/Excel/HTML printable exports.
- **Business Purpose:** Provide finance & procurement teams exact per-unit landed cost for inventory valuation/pricing and spend analytics across CHAs/forwarders/expense heads.
- **User Operations:** Filter by date range, supplier, invoice no, part no, expense type/provider, min/max amount; switch report tabs; export CSV, Excel 2016 `.xlsx`, or printable PDF/HTML.
- **Input Fields:** `startDate`, `endDate`, `supplier`, `invoiceNo`, `partNo`, `viewMode`, `minAmount` (paise), `maxAmount` (paise).
- **Validation Rules:** Valid date ordering (`startDate <= endDate` if both provided); numeric bounds on amount filters.
- **Database Tables:** `report_view` (over `shipments`, `suppliers`, `invoices`, `invoice_line_items`, `boe_details`, `boe_calculations`, `boe_items`, `expenses`, `items`), `expense_invoices`, `expense_types`, `service_providers`.
- **Business Calculations (`report_view`):**
  - Sums basic expense amount (`SUM(e.amount)`, excl. GST) per shipment.
  - Allocates shipment basic expenses proportionally to each BOE item by `item_assessable_value / shipment_boe_assessable_total`.
  - `ldc_per_qty = (assessable_value + bcd_amount + sws_amount + allocated_basic_expenses) / qty` (IGST excluded as input tax credit).
- **Output & Reports:** 6 KPI summary cards + 14-column Landed Cost CSV/table (`Supplier, Invoice No, Date, Part No, Description, Unit, Qty, Unit Price, Assessable Value, BCD, SWS, IGST, Expenses (Basic), LDC per qty`), 5 Expense Report views + CSV/XLSX/HTML exports.
- **Error Handling:** Handles shipments without expenses (`0` allocated) or division-by-zero guards (`qty > 0`, `total_av > 0`).
- **Security Requirements:** `report.*` RBAC permission; SQL parameterization against injection.
- **Acceptance Tests:** (1) Multi-item shipment expense proportional allocation sums back to exact shipment basic expenses; (2) `LDC per qty` excludes IGST; (3) CSV & Excel 2016 exports open cleanly with exact totals; (4) Expense report 5 views match raw ledger.

---

## Module 9: Database Management, Encrypted Backups & Restore

- **Existing Functionality:** 8-tab admin console (`overview`, `browse`, `bulk`, `backup`, `schedules`, `roles`, `audit`, `settings`), live table browser/inline editor, dependency reference scanner, chunked bulk delete (`500` batch size) with `5-minute` undo token, PIN-gated permanent delete (`'DELETE'` confirmation when deps `> 5000`), `IMBK2` AES-256-GCM encrypted backups + `.sha256` sidecars, `.imkey` key export/import via Windows Credential Manager, scheduled cron backups, dry-run restore simulation, crash-safe restore with global lock (`restore_control`) & automatic pre-restore safety backup.
- **Business Purpose:** Safeguard mission-critical ERP data with tamper-evident encrypted backups, safe administrative bulk maintenance, and zero-corruption restore workflows.
- **User Operations:** Inspect DB stats/table row counts; browse/edit rows; preview dependencies & bulk soft-delete/undo or PIN-verified hard-delete; create/verify/restore local encrypted backups; manage `.imkey`; configure scheduled backup crons; simulate restore.
- **Input Fields:** Table selector, search/pagination, selected row IDs, PIN for hard delete, `'DELETE'` confirm string, backup password / key, schedule cron expression & retention count, restore file path.
- **Validation Rules:** PIN verification with failed-attempt lockout; hard delete blocked if dependency scanner finds referencing children (`DEPENDENCY:` error); `IMBK2` header + SHA-256 checksum validation before restore; pre-restore safety backup mandatory.
- **Database Tables:** `backups`, `backup_schedules`, `app_metadata`, `audit_logs`, all domain tables.
- **Business Calculations:** `IMBK2` KDF (`PBKDF2-HMAC-SHA256`, `600,000` iterations, 16B random salt, 12B nonce, AES-256-GCM; legacy read-only `IMBK1` uses `100,000` iters); SHA-256 integrity hash; chunked progress (`500` rows, `50,000` warning, `100,000` auto-select-all).
- **Output & Reports:** Backup history table, SHA-256 verification status, restore dry-run simulation report, undo countdown timer.
- **Error Handling:** Lockout on bad PIN; `restore_control` pauses background jobs and blocks concurrent writes during restore; rolls back or recovers via safety backup on failure.
- **Security Requirements:** Admin/DB Manager RBAC permissions; `backup_encryption_password` stored in Windows Credential Manager (`service="ImportManager"`); password never logged in plaintext or audit tables.
- **Acceptance Tests:** (1) Create `IMBK2` backup & restore to clean DB; (2) Verify corrupted `.sha256` or wrong password fails cleanly; (3) Restore legacy `IMBK1` backup; (4) Bulk delete + 5-min undo restores exact rows; (5) PIN lockout triggers on repeated bad PINs.

---

## Module 10: Google Drive OAuth & Cloud Backup Sync

- **Existing Functionality:** OAuth2 PKCE/code flow via local loopback HTTP server (`http://127.0.0.1:8765/`), runtime OAuth credential storage, Drive API v3 uploads/downloads/listing in `ImportManagerBackups` folder (`gdrive:<fileId>` virtual path format), configurable retention cap (max `30` backups), email display.
- **Business Purpose:** Provide off-machine encrypted cloud redundancy without external servers.
- **User Operations:** Configure OAuth client ID/secret in Settings (or bundled defaults); authorize Google account in browser; upload backup to Google Drive; list/download/restore remote `gdrive:<fileId>` backup; revoke/clear credentials.
- **Input Fields:** Google OAuth `client_id`, `client_secret`, backup ID / `gdrive:<fileId>`.
- **Validation Rules:** Scopes `https://www.googleapis.com/auth/drive` & `https://www.googleapis.com/auth/userinfo.email`; fixed loopback redirect `http://127.0.0.1:8765/`; validates `.sha256` / `IMBK2` payload after download.
- **Database Tables:** `backups`, `backup_schedules`, `app_metadata`.
- **Business Calculations:** Prunes oldest cloud backups beyond `30` (or configured retention limit).
- **Output & Reports:** Connected Google account email, Drive sync status, `GDRIVE_ERROR:` structured error banners.
- **Error Handling:** Parses OAuth timeout/port-8765 conflict/token expiry/network errors cleanly into `GDRIVE_ERROR:` prefixed actionable messages.
- **Security Requirements:** Refresh token & user email stored exclusively in Windows Credential Manager; only encrypted `IMBK2` files + `.sha256` uploaded to Drive (never raw plaintext `.db`).
- **Acceptance Tests:** (1) Loopback OAuth mock exchange saves token in Credential Manager; (2) Encrypted upload/download roundtrip; (3) Retention pruning keeps latest N backups.

---

## Module 11: Recycle Bin & Soft-Delete Recovery

- **Existing Functionality:** Centralized soft-delete explorer (`PAGE_SIZE = 50`) across the 11 soft-deletable core tables (`get_soft_delete_tables`), aggregate count verification (`get_recycle_bin_deleted_count` mismatch banner), restore with missing-parent detection (`missing_parent` grouped by table/IDs with 1-click copy), permanent purge gated on FK dependency check (`DEPENDENCY:` blocker), retention cleanup (`recycle_retention_days`).
- **Business Purpose:** Prevent accidental permanent data loss while preserving referential integrity during restoration or permanent removal.
- **User Operations:** Switch soft-deleted entity table; inspect `deleted_at`/`deleted_by`; restore selected records; permanently delete expired/unreferenced records; run retention cleanup.
- **Input Fields:** Target table name, selected record IDs, pagination page/size (`50`).
- **Validation Rules:** Restore blocked if parent record is itself deleted or missing (returns structured `missing_parent` table+ID list); permanent delete blocked if child references exist (`DEPENDENCY:`); validates table name against fixed allowlist (prevents SQL injection).
- **Database Tables:** `suppliers`, `items`, `shipments`, `invoices`, `invoice_line_items`, `boe_details`, `boe_calculations`, `expenses`, `expense_invoices`, `service_providers`, `expense_types`, `app_metadata`.
- **Business Calculations:** Aggregate deleted count reconciliation across tables vs table pagination totals.
- **Output & Reports:** Recycle Bin table grid, count mismatch alert banner, `missing_parent` copyable error modal, `DEPENDENCY:` blocker detail.
- **Error Handling:** Structured `missing_parent` & `DEPENDENCY:` payloads parsed cleanly by frontend.
- **Security Requirements:** Authenticated session + delete/restore RBAC permissions; allowlist-only SQL identifiers; audit logging.
- **Acceptance Tests:** (1) Soft-deleted shipment prevents child invoice restore until shipment is restored first; (2) Permanent delete blocked if active children exist; (3) Aggregate count matches per-table sum.

---

## Module 12: Security Center, RBAC, Auth & Emergency Recovery

- **Existing Functionality:** Desktop login (`Argon2id` + legacy `bcrypt` verify), first-admin bootstrap (`bootstrap_first_admin_if_eligible`), 4 canonical roles (`administrator`, `manager`, `operator`, `viewer`) across 25 fine-grained permissions, session lifecycle (`12h` TTL / `30d` remember_me / `30m` idle timeout), admin session termination, password policy engine + history (`default 5`, `[1..24]`), lockout policy (`5` fails / `15m` window / `30m` lock), security trend metrics, Emergency Recovery Mode (`--recovery` or `IMPORT_MANAGER_RECOVERY=1`).
- **Business Purpose:** Enforce least-privilege access control, strong local credential hygiene, brute-force resistance, and deterministic self-recovery if locked out.
- **User Operations:** Sign in/out; change own/user password; view active sessions & terminate session; reset locked account; configure password/lockout/idle policies; manage user roles; trigger recovery actions when launched in recovery mode.
- **Input Fields:** Username/email, password, remember_me, role assignment, policy settings (`minLength >= 8`, history depth `[1..24]`, lockout attempts/window/duration, idle minutes).
- **Validation Rules:** Complexity: `>= 8` chars, `>= 1` uppercase, `>= 1` lowercase, `>= 1` digit, `>= 1` symbol, no whitespace; no reuse of last N historical hashes; recovery IPC commands fail cleanly when recovery mode flag is inactive.
- **Database Tables:** `user_roles`, `role_dashboard_permissions`, `auth_failed_attempts`, `auth_lockout_state`, `auth_password_history`, `security_policy_versions`, `audit_logs`, `user_activity_audit_logs`.
- **Business Calculations:** Lockout sliding window count, consecutive failure threshold check, session expiry/idle timestamps, security score & failed-login trend series.
- **Output & Reports:** Security Center dashboard, active sessions table, recent auth events feed, Role Permissions Matrix (`4 x 25`).
- **Error Handling:** Lockout countdown error with retry timestamp, password policy violation list, unauthorized role error.
- **Security Requirements:** Zero plaintext password storage; Argon2id memory-hard hashing; backend-verified session context for every RPC (fixing legacy `caller_role` parameter spoofing risk); recovery mode strictly gated to startup CLI/env flag.
- **Acceptance Tests:** (1) 5 failed logins lock account for 30m & auto-unlock after window; (2) Password reuse within history depth rejected; (3) All 4 roles enforce exact 25-permission matrix; (4) Idle timeout expires session; (5) Recovery flag resets admin password/lockout without exposing normal DB unlock if encrypted without key.

---

## Module 13: Settings & Appearance

- **Existing Functionality:** Theme preset (`default` | `steel`) & mode (`light` | `dark` | `system`), per-module table/field display customization across 7 modules (`shipment`, `invoice`, `boe`, `boeSummary`, `supplier`, `itemMaster`, `expenses`), Invoice Calculation precision (`0 | 2`), Number/Date/Text formatting rules, Google Drive OAuth runtime config.
- **Business Purpose:** Tailor ERP formatting, regional date/number display, pagination density, and cloud OAuth parameters to operator workflow.
- **User Operations:** Switch theme/mode; toggle field visibility/text case/decimal precision/`itemsPerPage` (`default 10`); set invoice precision; reset module defaults; save Google Drive client credentials.
- **Input Fields:** `theme`, `mode`, `lineTotalDecimals` (`0|2`), `invoiceTotalDecimals` (`0|2`), `numberFormat`, `dateFormat` (`DD/MM/YYYY` default), `textFormat`, `itemsPerPage`, Google OAuth client ID/secret.
- **Validation Rules:** `shipment.fields.shipmentType.case` locked to `'uppercase'`; `lineTotalDecimals`/`invoiceTotalDecimals` restricted to `{0, 2}`; settings schema versioned (`SETTINGS_SCHEMA_VERSION = 1`).
- **Database Tables:** `app_settings`, `app_metadata` (+ Windows Credential Manager for OAuth tokens).
- **Business Calculations:** Dynamic number/currency/date formatting transforms.
- **Output & Reports:** Live UI preview & persistent settings profiles.
- **Error Handling:** Invalid precision or empty OAuth client fields rejected with clear inline errors.
- **Security Requirements:** OAuth secrets redacted after entry / stored securely via Credential Manager; settings changes audited.
- **Acceptance Tests:** (1) Settings persist across restart; (2) Invoice decimal precision immediately affects Invoice Wizard rounding; (3) Reset module restores defaults.

---

## Module 14: Activity Log & User Activity Audit

- **Existing Functionality:** Immutable table mutation audit (`audit_logs` storing `before_json`, `after_json`, `table_name`, `row_id`, `action`, `user_id`) and paginated user activity stream (`user_activity_audit_logs` with `severity`), filterable & exportable from `/admin/activity-log` and `/admin/user-activity`.
- **Business Purpose:** Compliance traceability answering who changed what record, when, and what before/after field values changed.
- **User Operations:** Filter by user, action, table, severity, date range; inspect JSON before/after diffs; export CSV.
- **Input Fields:** Filter criteria, page/limit.
- **Validation Rules:** Read-only UI; append-only writes from backend services.
- **Database Tables:** `audit_logs`, `user_activity_audit_logs`.
- **Business Calculations:** Audit verification hash/checksum validation counts (`get_audit_verification_summary`).
- **Output & Reports:** Filterable log tables, JSON diff viewer, CSV export.
- **Error Handling:** Graceful truncation/formatting for large JSON payloads.
- **Security Requirements:** Gated on `admin.activity_log` and `admin.user_activity`; sensitive credential/password hashes redacted before logging.
- **Acceptance Tests:** (1) Create/update/delete on core entities writes accurate `before_json`/`after_json`; (2) Non-admin without permission cannot access endpoints.

---

## Module 15: Error Center, Bug Notes, Diagnostics & System Agent

- **Existing Functionality:** Runtime error deduplication & tracking (`error_memory`, retention cleanup/pruning), developer/operator Bug Notes (`bug_notes`), redacted Diagnostics ZIP bundle export (`export_diagnostics_bundle`), runtime log reader (`logs.rs`), and DeepSeek-powered AI System Agent (`system_agent.rs`, DeepSeek API key in Windows Credential Manager, explain-graph / chat audit in `system_agent_audit_log`).
- **Business Purpose:** Capture recurring operational faults, track triage notes, bundle sanitized diagnostic evidence for support, and assist admins with root-cause explanation.
- **User Operations:** Filter/resolve/prune `error_memory`; create/update/resolve `bug_notes`; export diagnostics ZIP; read log tail; configure DeepSeek API key (optional) & query System Agent.
- **Input Fields:** Error filters/status, bug note title/description/severity/status, diagnostics export path, System Agent prompt & DeepSeek API key.
- **Validation Rules:** Keyring validation for DeepSeek key; redacted export ensures no secrets/passwords enter ZIP.
- **Database Tables:** `error_memory`, `error_memory_cleanup_runs`, `error_memory_pruned_groups`, `bug_notes`, `system_agent_audit_log`.
- **Business Calculations:** Error recurrence occurrence counters & cleanup pruning counts.
- **Output & Reports:** Error Center list/detail, Diagnostics `.zip` bundle, Log viewer, Agent Q&A panel.
- **Error Handling:** Handles missing API key or offline DeepSeek gracefully without blocking ERP operations.
- **Security Requirements:** `audit.view` for diagnostics bundle; `automation.system_agent` for System Agent; DeepSeek key stored in Windows Credential Manager.
- **Acceptance Tests:** (1) Duplicate errors increment occurrence count cleanly; (2) Diagnostics ZIP contains DB schema stats/logs with zero credentials/keys; (3) Offline System Agent returns clear error.

---

## Module 16: System Health, Platform Reliability & Admin System Tools

- **Existing Functionality:** Startup/on-demand crash-recovery for interrupted writes (`write_recovery_journal`, `boe_write_recovery`), system integrity validator, memory watermark guard (`1,200,000,000` bytes), Expense Data Manager Admin (`clear_expense_data`, `cleanup_orphaned_expenses`, `cleanup_orphaned_expense_invoices`, `fix_expense_types`, `fix_existing_expenses`, `fix_lcl_charges_rate`, `debug_expense_data*`).
- **Business Purpose:** Detect and repair storage/journal inconsistencies, monitor host RAM/DB health, and provide safe admin repair utilities for historical expense anomalies.
- **User Operations:** Inspect System Health warnings & reliability metrics; run integrity check / write-journal recovery; invoke targeted expense cleanup/repair tools under confirmation.
- **Input Fields:** Repair tool selection, confirmation prompt, memory threshold checks.
- **Validation Rules:** Destructive admin tools gated by admin permission + explicit confirmation; disabled test-only resets in production builds.
- **Database Tables:** `write_recovery_journal`, `boe_write_recovery`, `system_integrity_snapshot`, `system_reliability_score`, `expenses`, `expense_invoices`, `expense_types`, `index_recommendation_history`.
- **Business Calculations:** Runtime anomaly score, platform health status, integrity violation counts.
- **Output & Reports:** System Health card grid, integrity verification report, orphan cleanup summary counts.
- **Error Handling:** Transactional rollback on repair failure; logs repair actions to audit log.
- **Security Requirements:** `admin.system_health` and `admin.system_tools`; `test_reset.rs` non-functional in production builds.
- **Acceptance Tests:** (1) Pending `write_recovery_journal` entry recovers cleanly on startup; (2) Orphaned expense cleanup removes only unlinked/dangling rows cleanly.

---

## Module 17: Workflow Exception Engine, Automation Rules & Multi-Env Deployment

- **Existing Functionality:** Exception detector (`OVERDUE_ETA`, `MISSING_BOE`, `MISSING_EXPENSE`, cap `150` per cycle), SLA tracker (`ON_TIME`, `AT_RISK`, `BREACHED`), priority escalation & recalculation scoring, self-heal repairs (`>14d` `IN_PROGRESS` reset, duplicate open suppression), configurable decision rules, cost limits & ROI/economics/capacity load indexing, round-robin auto-assignment (`LIMIT 40`), rollback capability (`AUTO_RESOLVE`, `AUTO_ASSIGN`, `PRIORITY_ADJUST`), policy guardrails (`automation_policy.rs`), rule versioning, staging (`DRAFT -> TESTING -> READY -> DEPLOYED | FAILED`), approval workflow (`PENDING -> APPROVED | REJECTED`), canary FNV-1a bucket hashing, deployment risk scoring (`<25 LOW .. >=75 CRITICAL`), conflict detection, deployment freeze switch, multi-tenant & multi-environment promotion (`DEV -> TEST -> PROD` one step at a time).
- **Business Purpose:** Automate detection and triage of stalled shipments/missing customs/expense paperwork with enterprise change-control guardrails, cost caps, and zero-surprise rollouts.
- **User Operations:** Triage exceptions (`OPEN -> IN_PROGRESS -> RESOLVED | IGNORED`), add notes, bulk resolve; toggle/edit/simulate/cost-cap rules; stage/approve/canary/promote/rollback rule versions across `DEV -> TEST -> PROD`; toggle master switch or deployment freeze.
- **Input Fields:** Case updates, notes, rule conditions/actions/weights/cost limits, staging statuses, approval decisions, canary percentages, environment/tenant selectors, admin override flag.
- **Validation Rules:** Exact status transition graph; policy guardrails (`DISABLE_ALL_AUTOMATION`, `CYCLE_RATE_LIMITED` >=3/min, `CANNOT_DISABLE_MAINTENANCE_JOB`); sequential `DEV -> TEST -> PROD` promotion; `env-prod` blocks `HIGH`/`CRITICAL` risk unless `admin` overrides; freeze blocks deploy/promote/rollback.
- **Database Tables:** 43 tables across `exception_*`, `automation_*`, `rule_*`, `canary_*`, `deployment_*`, `rollback_*`, `workflow_decision_rules`, `workflow_rule_*`, `workflow_environment*`, `workflow_tenants`, `workflow_automation_log`, `workflow_simulation_reports`, `adaptive_sla_adjustments`, `daily_*`.
- **Business Calculations:** Priority score (`0..1 -> LOW`, `2..3 -> MEDIUM`, `4..5 -> HIGH`, `>=6 -> CRITICAL`), Adaptive SLA (`clamp(120d_avg * 1.15, base, base*3)`), Efficiency/Capacity/Economics/Safety/Risk formulas (detailed in `BUSINESS_RULES.md`).
- **Output & Reports:** Exception queues, Automation Center, Rule Performance & Cost-vs-Benefit dashboards, Dry-run & Safety Audit reports, Simulation reports.
- **Error Handling:** Strict state-transition & freeze/approval rejection messages; auto-pause on hourly auto-resolve (`40`) or cost-limit breaches (`60m` default pause).
- **Security Requirements:** `automation.view`, `automation.mutate`, `role.write` permissions enforced server-side from authenticated session (remediating legacy `caller_role` parameter passing).
- **Acceptance Tests:** (1) `OVERDUE_ETA`/`MISSING_BOE`/`MISSING_EXPENSE` auto-detection & SLA escalation; (2) Policy guardrail blocks disabling last rule / `maintenance_cleanup`; (3) `DEV -> PROD` direct skip rejected; (4) FNV-1a canary determinism; (5) Rollback restores pre-action exception state cleanly.

---

## Module 18: Operations Center – Background Job Monitoring & Incident Management

- **Existing Functionality:** Monitors 6 registered background jobs (`automation_cycle`, `cost_metrics_aggregation`, `observability_update`, `maintenance_cleanup`, `deployment_safety_checks`, `risk_evaluation`), schedule expectation anchors, missed-run detection, auto-recovery with infinite-loop circuit breaker (`RECOVERY_GUARD_STOP` disables job after exceeding `max_recovery_attempts` in 24h), schedule drift detection, structured event logging & severity threshold alert scanning (`JOB_FAILURE_THRESHOLD`, `MISSED_JOB_SPIKE`, `SYSTEM_HEALTH_DEGRADED`, `GLOBAL_JOB_FAILURE_PRESSURE`), incident correlation (`source_module:event_type:entity_id`), burst detection (`10m` window, `>= 5` & `>= 2x` baseline), suppression windows (`10..30m`), stabilization phase detection (`10m` stabilizing -> `20m` confirmed), regression/persistence detection, 30-minute weighted failure probability forecasting (`0.4*slope + 0.3*reg + 0.3*persist`, emit `>= 0.6`), explainability & preventive action recommendations + feedback accuracy tracking.
- **Business Purpose:** Ensure self-monitoring operational resilience of unattended background jobs and surface actionable root-cause/forecast insights in `/admin/operations-center`.
- **User Operations:** Inspect job health/timelines/dependencies tree; enable/disable jobs; override `RECOVERY_GUARD_STOP` (admin); adjust grace/recovery expectations; manual retry; inspect correlated incidents; append resolution notes (`>=10` chars); resolve incidents (root cause `>=50` chars + quiet stream check); trigger manual suppression; acknowledge/rate forecasts; export CSV reports.
- **Input Fields:** `job_id`, `grace_period_minutes [1..10080]`, `recovery_delay_sec [10..86400]`, `max_recovery_attempts [1..50]`, incident note/root-cause summary, forecast feedback (`accurate | misleading`).
- **Validation Rules:** Timeout triggered at `> 2 * expected_ms` or `RUNNING > 2h`; resolve requires `root_cause_summary.len() >= 50` + existing `>= 10`-char note + quiet correlation stream (unless skipped); forecast feedback allowed once per forecast.
- **Database Tables:** 48 tables across `workflow_job_*`, `workflow_background_jobs`, `workflow_incident*`, `workflow_failure_*`, `workflow_forecast*`, `workflow_structured_event_log`, `workflow_alert_signal_log`, `workflow_runtime_metrics`, `performance_timing_metrics`, `daily_*`, `system_*`.
- **Business Calculations:** Job reliability & recovery scores, system reliability score, runtime anomaly score, weighted forecast probability `[0.0..0.99]`.
- **Output & Reports:** Operations Center dashboard, job dependency tree & execution timeline, correlated incident timeline, CSV exports (job recovery log, metrics snapshot, incidents report).
- **Error Handling:** Explicit circuit-breaker/guard errors, resolve-gate validation errors, debug failure injection restricted to admin/debug mode.
- **Security Requirements:** `automation.ops_center` / `admin.activity_log` / `admin` gates verified server-side.
- **Acceptance Tests:** (1) Stuck `RUNNING > 2h` marks `TIMEOUT`; (2) Exceeding `max_recovery_attempts` in 24h flips `is_enabled = 0` & blocks retry until admin override; (3) Incident resolution blocked below 50-char root cause or missing note; (4) Systemic burst promotes incident & opens 10–30m suppression cleanly.
