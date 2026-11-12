# Migration & Compatibility Plan – Import Manager Next Generation

> **Objective:** Guarantee deterministic, non-destructive import and upgrade of legacy Import Manager databases (`.db`) and encrypted backups (`IMBK1`, `IMBK2`, `.imkey`) into **Import Manager Next Generation's** SQLCipher-encrypted SQLite schema.

---

## 1. Next-Generation Schema Migration Engine

Unlike the legacy 3-layer split (`Refinery V1..V80` + `post_refinery_migrations` runtime DDL + `init_schema` runtime DDL), **Import Manager Next Generation** enforces a single, strict Go migration engine (`backend/database/migrations`):

- **Schema Tracking Table (`next_schema_migrations`):**
  - `version INTEGER PRIMARY KEY`
  - `name TEXT NOT NULL`
  - `checksum_sha256 TEXT NOT NULL`
  - `app_version TEXT NOT NULL`
  - `executed_at TEXT NOT NULL`
  - `duration_ms INTEGER NOT NULL`
  - `status TEXT NOT NULL` (`'APPLIED' | 'FAILED'`)
- **Guarantees:**
  1. **Sequential Versions & Checksums:** Every migration script's SHA-256 hash is validated against `next_schema_migrations` at startup; tampering or drifted history aborts startup cleanly.
  2. **Downgrade Protection:** If DB max applied version > binary's embedded max migration version, refuse opening and prompt user to upgrade app rather than corrupting data.
  3. **Pre-Migration Safety Backup:** Before applying pending upgrades to an existing Next Gen DB, create an automatic pre-migration snapshot / `IMBK2` copy.
  4. **Zero Runtime DDL:** Frontend and service code NEVER execute `ALTER TABLE` / `CREATE TABLE`; 100% of schema evolution lives in transactional versioned SQL migrations.
  5. **Foreign Keys & WAL:** `PRAGMA foreign_keys = ON` enforced per connection; WAL mode enabled after compatibility verification during startup.

---

## 2. Legacy Database Detection & Key Validation Workflow

When a user selects a legacy `.db` file (or upgrades an existing legacy AppData database):

1. **Read-Only Copy Staging (Non-Destructive Guarantee):**
   - **Never mutate or overwrite the source legacy file.**
   - Inspect file header bytes `[0..16]`:
     - Exact match `"SQLite format 3\x00"` -> classify as **Plaintext Legacy SQLite** (remember: legacy `encryption.rs` was a stub when compiled with `LIBSQLITE3_SYS_BUNDLED=1`, so most legacy DBs on disk are plaintext SQLite headers).
     - Non-matching magic bytes -> classify as **SQLCipher Encrypted** (or invalid/corrupted file).
2. **Encryption Key Validation (Encrypted Sources):**
   - Prompt user for legacy database passphrase / key (or try key from Windows Credential Manager `service="ImportManager"`).
   - Attempt read-only open with SQLCipher `PRAGMA key` + run `SELECT count(*) FROM sqlite_master;` and `PRAGMA integrity_check;`.
   - If key fails (`file is not a database`), reject immediately with explicit key-validation error — never touch source file.
3. **Schema & Three-Layer Drift Inspection:**
   - Do **NOT** rely solely on `refinery_schema_history` version numbers (since legacy `V52`, `V55`, `V80` relied on runtime `ALTER TABLE` additions).
   - Introspect live source metadata using `sqlite_master` + `PRAGMA table_info(<table>)` for every table to determine exact column presence (`deleted_at`, `deleted_by`, `*_paise`, `*_bp`, `duty_percent`, `tableName`, etc.).
   - Verify source `PRAGMA integrity_check == 'ok'` before proceeding.

---

## 3. Legacy-to-Next-Gen Schema Mapping Matrix

| Legacy Domain / Table | Next-Gen Mapping Strategy | Primary Key & FK Handling | Special Legacy Drift / Normalization |
| :--- | :--- | :--- | :--- |
| `suppliers`, `items` | Direct 1:1 table mapping | Preserve `id` (`Sup-XXX`, `ITM-XXX`), `part_number`, `deleted_at`/`deleted_by` | Coalesce missing optional cols (`deleted_at -> NULL`) |
| 13 Lookup Option Tables | Direct 1:1 mapping | Preserve `value` / `label` | Ensure seed defaults exist cleanly |
| `shipments` | Direct 1:1 mapping | Preserve `id`, `supplier_id` FK | Normalize `status == 'docu-received'` alias optionally or preserve; preserve `is_frozen` |
| `invoices`, `invoice_line_items` | Direct 1:1 mapping | Preserve `id`, `shipment_id` FK, `item_id` FK | Coalesce `duty_percent`, `sws_percent`, `igst_percent` from `items` if absent |
| `boe_details`, `boe_calculations`, `boe_items`, `boe_attachments` | Direct 1:1 mapping | Preserve `id`, `boe_id` FK | Validate `boe_items` JSON backfill parity with `form_values_json` |
| `service_providers`, `expense_types`, `expense_invoices`, `expenses`, `expense_attachments` | Normalized canonical expense schema | Preserve `id`, `shipment_id`, `expense_invoice_id`, `service_provider_id`, `expense_type_id` | Reconcile dual columns: if `amount_paise` null/0 and `amount > 0`, compute `round(amount * 100)`; coalesce `invoice_number` vs `invoice_no`; backfill basis-point rates `(rate * 100)` when `*_bp` null |
| `user_roles`, `auth_*`, `security_policy_versions` | Canonical RBAC & Argon2id auth | Preserve `user_id` & hashes | Canonicalize role names (`admin -> administrator`); retain legacy `$2b$`/`$2a$` bcrypt hashes for seamless 1st-login transparent re-hash or immediate verification |
| `audit_logs`, `user_activity_audit_logs` | Preserve historical audit trail | Preserve numeric `id` & timestamps | Map legacy camelCase `"tableName"` -> `table_name` if `table_name` empty |
| `backups`, `backup_schedules`, `app_metadata`, `app_settings` | Direct mapping + KV preservation | Preserve backup history & metadata keys | Convert `gdrive:<fileId>` paths intact |
| `exception_*`, `workflow_*`, `automation_*`, `rule_*`, `canary_*`, `deployment_*` (all ~90 workflow tables) | Preserved cleanly across modular tables | Preserve rule versions, incidents, job logs, forecasts | Coalesce missing new columns with spec defaults |
| `refinery_schema_history`, `boe_write_recovery`, `write_recovery_journal` | Audited pre-import | Do not overwrite Next Gen migration table | Warn user if `write_recovery_journal` / `boe_write_recovery` contains `pending` rows |

---

## 4. Dry-Run Simulation & Post-Import Verification Protocol

Every migration executes in **two modes**: `--dry-run` (default first step in Wizard UI) and `--commit`:

### 4.1 Dry-Run Execution (`rollback / temp target`)
1. Opens source DB read-only.
2. Creates an ephemeral in-memory / temp SQLCipher target DB with Next Gen migrations applied (`foreign_keys = OFF` temporarily only during ordered parent-before-child copy, then `PRAGMA foreign_key_check` with `foreign_keys = ON`).
3. Executes full mapping & transformation and collects diagnostic metrics without modifying production DB.

### 4.2 Mandatory Integrity & Financial Reconciliation Report
Generated for both Dry-Run and Commit:
1. **Row Count Parity:** Exact `source_count == imported_count` (and skipped/coerced warnings if any duplicate or orphan existed in a legacy FK-disabled DB) for every table across all 155 tables.
2. **Financial Totals Verification:** Exact equality comparison (tolerance `0` paise / `<= 0.01` for float legacy decimals):
   - Total `shipments.invoice_value` (active vs soft-deleted)
   - Total `invoice_line_items` computed invoice sums
   - Total `boe_details.total_assessment_value`, `boe_details.duty_paid`, and `boe_calculations` (`customs_duty_total`)
   - Total `expenses` basic `amount` (`SUM(amount_paise)`), CGST, SGST, IGST, TDS, and Net Payable (`SUM(net_amount_paise)`)
   - Landed Cost `report_view` spot-check sample across shipments
3. **Referential Integrity Check:**
   - Run `PRAGMA foreign_key_check;` on target SQLCipher DB -> must return `0` violations (if legacy source had orphaned child rows from prior SQLite runs without `foreign_keys=ON`, flag explicitly in report with repair options: Quarantine / Nullify / Auto-Create Stub vs Abort).
   - Run `PRAGMA integrity_check;` -> must return `"ok"`.
4. **Migration Report Export:** JSON & human-readable Markdown/HTML report saved alongside Next Gen logs (`docs`/AppData import report).

---

## 5. Encrypted Backup (`IMBK1` / `IMBK2`) & `.imkey` Compatibility

| Format | Support Level in Next Gen | Cryptographic Specification |
| :--- | :--- | :--- |
| **`IMBK2` Backup Files** | **Read & Write (Default)** | `5B` `"IMBK2"` + `16B` salt + `12B` nonce + AES-256-GCM (`PBKDF2-HMAC-SHA256`, `600,000` iters) + `.sha256` sidecar. Decrypting a legacy `IMBK2` extracts the legacy `.db` into staging, then runs the Section 2–4 Legacy Import Pipeline automatically! |
| **`IMBK1` Backup Files** | **Read-Only Restore** | `5B` `"IMBK1"` + `16B` salt + `12B` nonce + AES-256-GCM (`PBKDF2-HMAC-SHA256`, `100,000` iters). Automatically detected, decrypted, and fed through the Legacy Import Pipeline. |
| **`.imkey` Key Files** | **Read & Write** | 32-byte Base64-encoded backup key synced with Windows Credential Manager (`service="ImportManager"`, account=`backup_encryption_password`). |
| **Google Drive Backups** | **Read & Write** | Compatible with existing `ImportManagerBackups` Drive folder & `gdrive:<fileId>` identifiers using existing OAuth scopes and `http://127.0.0.1:8765/` loopback flow. |
