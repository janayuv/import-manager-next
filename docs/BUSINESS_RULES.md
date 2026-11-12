# Business Rules & Calculation Specifications – Import Manager

> **Source of Truth:** Audited legacy implementations (`duty-calculator.ts`, `boe-entry/form.tsx`, `expense.rs`, `decimal_money.rs`, `report_view`, `shipments.rs`, `shipment_service.rs`, `permissions.rs`, `credentials.rs`, `lockout.rs`, `utils/encryption.rs`, `workflow_*.rs`, `exception_*.rs`, `deployment_safety.rs`, `csv-helpers.ts`, `shipment-import.ts`, `multiline-paste.ts`, `export-boe-excel.ts`, `export-boe-pdf.ts`, `settings.ts`).

---

## 1. Monetary Representation & Decimal Precision

1. **Foreign Currency & Invoice Lines (`decimal_money.rs`, `InvoiceWizard.tsx`, `app_settings`):**
   - Configurable `lineTotalDecimals` and `invoiceTotalDecimals`: strictly `{0, 2}` (default `2`).
   - Invoice finalize check compares `sum(lineTotals)` vs `shipment.invoiceValue` in exact integer cents (`Math.round(v * 100)`).
   - Multi-line paste accepts `qty > 0`, `unitPrice >= 0`, normalizes `partNumber = trim().toUpperCase()`.
2. **Expenses & Statutory Taxes (`expense.rs` – Integer Paise & Basis Points):**
   - Currency amounts stored as signed 64-bit integer **paise** (`₹1 = 100 paise`).
   - Tax/TDS rates stored as **basis points (bp)** where `10,000 bp = 100%` (e.g., `18.00%` = `1800 bp`, `2.00%` = `200 bp`).
   - Per-line integer division formulas (`TaxCalculator`):
     - `cgst_paise = (amount_paise * cgst_rate_bp) / 10000`
     - `sgst_paise = (amount_paise * sgst_rate_bp) / 10000`
     - `igst_paise = (amount_paise * igst_rate_bp) / 10000`
     - `tds_paise = (amount_paise * tds_rate_bp) / 10000`
     - `total_paise = amount_paise + cgst_paise + sgst_paise + igst_paise`
     - `net_paise = total_paise - tds_paise`
   - Duplicate prevention via client-supplied `idempotency_key` (UUID v4) and optimistic concurrency via `version`.
   - When combining duplicate expense types in an invoice (`expense-multiline-form.tsx`), paise sums aggregate and effective rates recalculate cleanly.

---

## 2. Customs Duty Calculation Engine (`duty-calculator.ts` & `boe-entry/form.tsx`)

### 2.1 Inputs & Defaults
- `exchangeRate` (`>= 0`, default `83.5` INR per unit of invoice currency)
- `freightCost` (`>= 0`, **INR** — divided by total foreign invoice value, NOT multiplied by `exchangeRate`)
- `exwCost` (`>= 0`, **INR** — divided by total foreign invoice value, NOT multiplied by `exchangeRate`)
- `insuranceRate` (`>= 0`, `%`, default `0.015`)
- `interest` (`>= 0`, INR, default `0`)
- **Incoterm CIF Lock:** When `shipment.incoterm === 'CIF'`, `freightCost`, `exwCost`, and `insuranceRate` are forced to `0` and disabled.

### 2.2 Per-Item Valuation & Duty Formulas
Given invoice items `i = 1..N`:
1. `totalInvoiceValue = sum(qty_i * unitPrice_i)` (foreign currency)
2. `itemForexValue = round(qty_i * unitPrice_i, 2)`
3. `itemValueInr = round(itemForexValue * exchangeRate, 2)`
4. `itemFreight = round((freightCost / totalInvoiceValue) * itemForexValue, 2)`
5. `itemExw = round((exwCost / totalInvoiceValue) * itemForexValue, 2)`
6. `itemInsurance = round((itemValueInr + itemExw) * (insuranceRate / 100), 1)`
7. `assessableValue = round(itemValueInr + itemFreight + itemExw + itemInsurance, 1)`

Rates (`boeBcdRate`, `boeSwsRate`, `boeIgstRate`, `actualBcdRate`, `actualIgstRate`) divided by `100` to decimal fraction `r_*`:
- **Method = `Standard` or `CEPA`:**
  - `bcdRaw = assessableValue * r_boeBcd`
  - `swsRaw = bcdRaw * r_boeSws`
  - `igstRaw = (assessableValue + bcdRaw + swsRaw) * r_boeIgst`
- **Method = `Rodtep`:**
  - `bcdRaw = assessableValue * r_boeBcd` *(preferential / scrip BCD)*
  - `actualBcd = assessableValue * r_actualBcd` *(SWS & IGST base uses statutory Actual BCD)*
  - `swsRaw = actualBcd * r_boeSws`
  - `igstRaw = (assessableValue + actualBcd + swsRaw) * r_boeIgst`

### 2.3 Rounding & Save Blocking Rules
- Per-item line duties: `bcdValue = round(bcdRaw, 1)`, `swsValue = round(swsRaw, 1)`, `igstValue = round(igstRaw, 1)`.
- Excel row duty sum: `bcdValue + swsValue + igstValue`.
- Header aggregate totals: `bcdTotal = Math.round(sum(bcdValue))`, `swsTotal = Math.round(sum(swsValue))`, `igstTotal = Math.round(sum(igstValue))`.
- Grand total: `customsDutyTotal = Math.round(bcdTotal + swsTotal + igstTotal + interest)`.
- **BOE Rate Override CSV Gate:** Required columns `partNo, calculationMethod, boeBcdRate, boeSwsRate, boeIgstRate`. Unknown `partNo` skipped with warning; rows where `boeIgstRate !== actualIgstRate` skipped with warning (`boeIgstRate` cell is read-only).
- **Save Blockers:**
  1. Block save if any line item satisfies `boeBcdRate > 0 && boeBcdRate > actualBcdRate`.
  2. If linked to an existing BOE (`selectedBoeId`), block save unless `round(boeDetails.dutyPaid, 2) === round(customsDutyTotal, 2)`.
- **Payload Guard (`services/boe_service.rs`):** JSON payload max `1,048,576` bytes; warn at `786,432` bytes (throttled after 3 consecutive warnings).

---

## 3. Landed Cost Allocation (`report_view` & `reports.rs`)

1. For each shipment `S`, basic expense total is `E_basic(S) = SUM(expenses.amount)` (excludes CGST/SGST/IGST; net of nothing or basic pre-tax invoice fee).
2. Shipment assessable total across BOE items is `AV_total(S) = SUM(boe_items.assessable_value)`.
3. Allocated basic expense for BOE item `k`:
   - `allocated_basic_expenses(k) = (AV_total(S) > 0) ? E_basic(S) * (AV(k) / AV_total(S)) : 0`
4. Landed Cost per Unit (`LDC per qty`):
   - `ldc_per_qty(k) = (AV(k) + BCD(k) + SWS(k) + allocated_basic_expenses(k)) / qty(k)`
   - **Rule:** IGST is explicitly **excluded** from Landed Cost because IGST is recoverable input tax credit.

---

## 4. Shipment & BOE Lifecycle Rules

### 4.1 Shipment Statuses & Automated Transitions
- **Allowed Statuses:** `docs-rcvd` (default on creation), `docu-received` (normalized legacy alias), `in-transit`, `customs-clearance`, `ready-dly`, `delivered`, `cancelled`, `finalized`, `closed`, `completed`.
- **Auto Transitions (only when current status != `'delivered'`):**
  - Creating/saving invoice sets shipment status -> `'in-transit'`.
  - Saving BOE calculation sets shipment status -> `'customs-clearance'`.
  - Ready-for-Delivery check (`check_and_update_ready_for_delivery`): sets status -> `'ready-dly'` when `date(boe_details.be_date, '+7 days') <= date('now')`.
- **Freeze Rule (`is_frozen = 1`):** Terminal operational lock blocking commercial shipment edits, invoice line edits, and expense mutations until explicitly unfrozen (admin).

### 4.2 BOE Reconciliation Statuses
- Allowed: `'Awaiting BOE Data'`, `'Discrepancy Found'`, `'Reconciled'`, `'Investigation'`, `'Closed'`.

---

## 5. CSV / Excel Import & Export Contracts

1. **Global Limits (`csv-helpers.ts`):** `MAX_FILE_SIZE = 10 MB`, `MAX_ROWS = 10,000`, `MAX_COLUMN_LENGTH = 1,000`, allowed encodings `utf-8`, `utf-8-bom`, `iso-8859-1`; HTML sanitized via `DOMPurify` on export.
2. **Required Headers by Entity:**
   - **Items (`20` export cols):** Required `[partNumber, itemDescription, unit, currency, unitPrice]`. Sequential ID `ITM-XXX`. Deduplicates existing + intra-batch `partNumber`.
   - **Shipments (`18` template cols, `\uFEFF` BOM + `\r\n`):** Required `[invoiceNumber, invoiceDate, invoiceValue]`. Auto-detects `\t`, `;`, `,` delimiter; parses Excel serial dates (`45124`).
   - **Suppliers (`13` template cols):** Required `[supplierName, country]`. Next Gen uses RFC 4180 CSV parser (fixing legacy comma-split issue).
   - **BOE Registry (`10` template cols):** Required `[beNumber, beDate]`.
   - **Expenses Bulk (`11` cols):** `[Expense Type, Service Provider, Invoice No, Invoice Date, Amount, CGST Amount, SGST Amount, IGST Amount, TDS Amount, Total Amount, Remarks]` (+ camelCase aliases). `amount > 0`, exact case-insensitive lookup match, tax/TDS `>= 0`. Next Gen uses RFC 4180 CSV + Excel 2016 `.xlsx`.
   - **Landed Cost Export (`14` cols):** `Supplier, Invoice No, Date, Part No, Description, Unit, Qty, Unit Price, Assessable Value, BCD, SWS, IGST, Expenses (Basic), LDC per qty`.

---

## 6. Security, RBAC & Cryptographic Rules

### 6.1 RBAC Role Matrix (4 Canonical Roles × 25 Permissions)
Canonical roles (normalized from legacy aliases e.g. `admin -> administrator`):
1. `administrator`: All 25 permissions.
2. `manager`: Operational + approval + reports + backup view/create (no user-role/security-policy admin overrides).
3. `operator`: Standard CRUD for suppliers, shipments, invoices, items, BOE, expenses, reports.
4. `viewer`: Read-only access across ERP views.
*(All RPC endpoints enforce permissions strictly via backend authenticated desktop session — legacy `caller_role` string parameters are removed/ignored).*

### 6.2 Password, Lockout & Session Rules
- **Hashing:** Argon2id default (verify legacy bcrypt hashes cleanly on upgrade).
- **Complexity:** `len >= 8`, `>= 1` uppercase `[A-Z]`, `>= 1` lowercase `[a-z]`, `>= 1` digit `[0-9]`, `>= 1` special char, zero whitespace.
- **Password History:** Default `5`, configurable `[1..24]`.
- **Lockout:** Default `5` failed attempts within `15` minute window -> lock account for `30` minutes.
- **Sessions:** `12 hours` standard TTL (`30 days` with `remember_me`), `30 minutes` idle timeout.
- **Emergency Recovery Mode:** Only enabled when launched with CLI `--recovery` or `IMPORT_MANAGER_RECOVERY=1`.

### 6.3 Backup Encryption & Keyring Formats
- **File Backup (`IMBK2`):** `5B` magic `"IMBK2"` + `16B` salt + `12B` nonce + AES-256-GCM ciphertext (`PBKDF2-HMAC-SHA256`, `600,000` iterations) + `.sha256` sidecar file.
- **Legacy Read-Only (`IMBK1`):** `PBKDF2-HMAC-SHA256`, `100,000` iterations (supported for restore).
- **Windows Credential Manager (`service="ImportManager"`):** Keys `backup_encryption_password` (`.imkey` export = 32B base64), `google_drive_refresh_token`, `google_drive_user_email`, `system_agent_deepseek_api_key`.
- **Google Drive Sync:** OAuth2 loopback `http://127.0.0.1:8765/`, scopes `drive` + `userinfo.email`, folder `ImportManagerBackups`, virtual URI `gdrive:<fileId>`, max `30` backups retention.
- **Bulk Delete / Hard Delete Thresholds:** Chunk `500`, warning `50,000`, auto-select-all `100,000`, `5-minute` undo token, PIN verify + `'DELETE'` confirmation when `total_with_dependencies > 5,000`.

---

## 7. Workflow Exception, Automation, Job & Incident Formulas

1. **Exception Detection & SLA:**
   - Cycle cap `150` (`ENTITY_EXCEPTION_CAP`).
   - `OVERDUE_ETA`: `HIGH`, SLA `2` days (`eta < today` & status not `delivered`/`completed`).
   - `MISSING_BOE` / `MISSING_EXPENSE`: `MEDIUM`, SLA `1` day.
   - SLA statuses: `ON_TIME`, `AT_RISK` (`0..1` days left), `BREACHED` (`today > sla_deadline`).
   - Priority recalculation score: `+4` `BREACHED`, `+2` `AT_RISK`; `+3` (`age >= 14d`), `+2` (`>= 7d`), `+1` (`>= 3d`); `+2` `recurrence_flag` (`total_opens >= 3`); `+1` `OVERDUE_ETA`. Brackets: `0..1 LOW`, `2..3 MEDIUM`, `4..5 HIGH`, `>=6 CRITICAL`.
   - SLA Escalation cooldown `[1, 168]` hours (default `24`); Timeout scan `[1, 90]` days (default `7`); Retention `[30, 3650]` days (default `365`).
2. **Automation Guardrails & Economics:**
   - Master enabled unless `'0'`; auto-assign `LIMIT 40` round-robin (`AUTO_LOAD_BALANCED`); hourly auto-resolve cap `40` `[1..500]`; priority adjust cap `80` `[1..500]`; pause duration `60m` `[5..1440]`.
   - Default cost limits: `500,000` cost units (`(effective_ms / 1000) * weight`), `120,000 ms`, `500,000` records.
   - Policy blocks: role check, disabling last active rule (`DISABLE_ALL_AUTOMATION`), `>= 3` runs/min (`CYCLE_RATE_LIMITED`), disabling `maintenance_cleanup` (`CANNOT_DISABLE_MAINTENANCE_JOB`).
   - Adaptive SLA: `clamp(120d_avg * 1.15, base, base * 3.0)` (`base = 48h` for `OVERDUE_ETA`, else `24h`).
   - Daily efficiency: `clamp(100 - min(backlog*0.4, 25) - min(breached*1.5, 20) - min(esc_today*2, 15) - min(avg_res/120*10, 15), 0, 100)`.
   - Capacity load: `min(min(actions/800,1)*35 + min(cu/8000,1)*40 + min(peak_ms/180000,1)*25, 100)` (`>=70 HIGH`, `>=40 MEDIUM`, else `LOW`).
3. **Rule Deployment & Safety:**
   - Staging: `DRAFT | TESTING | READY | DEPLOYED | FAILED`; Approvals: `PENDING | APPROVED | REJECTED`.
   - Multi-env promotion: strict 1-step progression `DEV(0) -> TEST(1) -> PROD(2)`.
   - Canary FNV-1a hash: `(fnv1a(entity_id) % 100) < sample_pct` (`>= 99.99` -> 100%).
   - Deployment risk brackets: `<25 LOW`, `<50 MEDIUM`, `<75 HIGH`, `>=75 CRITICAL` (hard blocks: `load >= 95%`, `cost_24h > max_cu * 1.1`, `!rollback_readiness`; on `env-prod` `HIGH`/`CRITICAL` blocked unless `admin` acknowledges override).
4. **Job Monitoring & Incident Management:**
   - Registered 6 jobs: `automation_cycle`, `cost_metrics_aggregation`, `observability_update`, `maintenance_cleanup`, `deployment_safety_checks`, `risk_evaluation`.
   - Timeout at `> 2 * expected_ms` or `RUNNING > 2h`; Recovery Guard disables job (`is_enabled = 0`) when 24h recovery attempts `>= max_recovery_attempts [1..50]`.
   - Reliability scores clamped `[-1.0, 1.0]`: Job `(success_rate - fail_rate - retry_rate*0.5)`, System `(success_rate - fail_rate + recovery_success_rate*0.5)`.
   - Runtime anomaly score: `failed_1h*3 + timeout_1h*2 + journal_stuck*4 + integrity_24h*5 + (mem > 1.2e9 ? 6 : 0)` (`>=20 critical`, `>=10 high`, `>=5 medium`). Self-heal runs when `score >= 5`.
   - Incident Resolve Gate: `root_cause_summary.len() >= 50`, `RESOLUTION_NOTE`/`MANUAL_INTERVENTION` `>= 10` chars, quiet `10m` correlation stream (`skip_correlation_quiet_check` optional).
   - Systemic burst (`10m` window): triggers when `events >= 5 && events >= 2 * max(baseline, 1/144)` -> opens `10..30m` suppression; stabilization `10m -> 20m confirmed`.
   - Failure forecast (`30m` horizon): `prob = clamp(0.4*slope + 0.3*reg + 0.3*persist, 0, 0.99)`, emits when `>= 0.6` (`>=0.9 CRITICAL`, `>=0.8 HIGH`, `>=0.6 MEDIUM`), max `1000` rows retained.
