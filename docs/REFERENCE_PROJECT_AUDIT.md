# Reference Project Audit – Import Manager (Legacy Tauri/Rust)

> **Reference Repository:** `https://github.com/janayuv/import-manager` (read-only functional reference; untouched)  
> **Target System:** **Import Manager Next Generation** (`Wails 3 + Go 1.25 + React 19 + TypeScript + SQLite/SQLCipher`)

---

## 1. Executive Summary

The legacy **Import Manager** application is a single-user Windows desktop ERP built with **Tauri 2 (Rust)**, **React + TypeScript + Vite**, and **SQLite** (via `rusqlite` with `LIBSQLITE3_SYS_BUNDLED=1`). It manages end-to-end import operations across **Suppliers**, **Shipments**, **Commercial Invoices (Wizard)**, **Item Master**, **Bill of Entry (BOE) Registry & Customs Duty Calculation**, **Import Expenses (Paise/Basis-Point Engine)**, **Landed Cost & Expense Reporting**, **Encrypted Backup/Restore & Google Drive Sync**, **Soft-Delete Recycle Bin & Reference Scanning**, **RBAC & Security Center**, and an extensive **Exception Workflow, Automation Rule Deployment, Background Job Monitoring & Incident Management** subsystem.

While functionally rich (155 tables + 1 view, 80 versioned migrations, 407 registered IPC handlers), the legacy codebase exhibits structural drift across three schema initialization layers, bundled-SQLite encryption stubs (`encryption.rs` no-op when SQLCipher isn't linked), dual legacy/paise monetary columns, UI-only validation/hardcoding in parts of Duty/PDF export, and naive CSV splitting in a few importers. **Import Manager Next Generation** preserves 100% of established business behavior while re-architecting the backend cleanly in modular Go services over true SQLCipher-encrypted SQLite.

---

## 2. Repository Structure & Technology Baseline

### 2.1 Directory Topology (Legacy)

| Path | Role |
| :--- | :--- |
| `src/` | React + TypeScript frontend (`App.tsx`, `pages/`, `components/`, `lib/`, `hooks/`, `contexts/`) |
| `src/components/layout/nav-data.ts` | Sidebar navigation tree, route bindings, and permission requirements |
| `src/components/boe-entry/` | BOE Customs Duty Calculator form, virtualized items table, results card |
| `src/components/invoice/wizard/` | 3-step Commercial Invoice Wizard (`Header -> Lines -> Review & Save`) |
| `src/components/expenses/` | Multi-line paise form, legacy invoice form, Excel/CSV importer, expense reports, admin diagnostics |
| `src/pages/admin/` | 11 admin & observability consoles (Activity Log, Automation Center/Rules, Error Center, Operations Center, Roles/Permissions, Security Center, System Agent, System Health, System Tools, User Activity) |
| `src-tauri/src/` | Rust backend: `main.rs`, `db.rs`, `migrations.rs`, `encryption.rs`, `expense.rs`, `desktop_session.rs`, `restore_control.rs`, `safety.rs`, `connection_manager.rs`, `recovery_mode.rs`, `ipc_error.rs` |
| `src-tauri/src/commands/` | 41 command modules exporting 384 `#[tauri::command]` functions (+ 26 macro-generated option commands in `options.rs`) |
| `src-tauri/src/services/` | Domain & platform services (`shipment_service.rs`, `boe_service.rs`, `decimal_money.rs`, `platform_reliability.rs`, `background_health.rs`, `user_activity_audit.rs`) |
| `src-tauri/src/security/` | RBAC permissions (`permissions.rs`), password/credential policies (`credentials.rs`), failed-attempt lockout (`lockout.rs`) |
| `src-tauri/src/utils/` | Backup AES-256-GCM encryption (`encryption.rs`), Windows Credential Manager (`backup_keyring.rs`) |
| `src-tauri/migrations/` | Sequential Refinery SQL migrations `V1__initial_schema.sql` .. `V80__backfill_expense_paise_from_decimal.sql` |
| `test-data/` | Structured CSV/Excel fixtures (`supplier/`, `shipment/`, `invoice/`, `item-master/`, `boe/`, `expenses/` split into `valid/`, `invalid/`, `edge/`) |
| `.github/workflows/` | 7 workflows: `ci.yml`, `release.yml`, `nightly.yml`, `format.yml`, `gitleaks.yml`, `workflow-lint.yml`, `dependabot-auto-merge.yml` |

### 2.2 Frontend & Backend Runtime Mechanics

- **Frontend Stack:** React, TypeScript, Vite, shadcn/ui + Radix UI, Tailwind CSS, TanStack React Table, React Hook Form + Zod, Recharts, ExcelJS, PapaParse, jsPDF + AutoTable, DOMPurify, `bcryptjs`.
- **Dual Runtime Shim (`src/lib/tauri-bridge.ts`):** Detects Tauri (`globalThis.isTauri`, `window.isTauri`, `window.__TAURI__`, `window.__TAURI_INTERNALS__`) and Playwright (`VITE_PLAYWRIGHT === '1'`). Falls back to browser `<input type="file">` / Blob download and `localStorage` auth (`VITE_ADMIN_PASSWORD_HASH`) in non-Tauri preview/test runs. Rejects `..` path traversals via `assertTrustworthySavePath`.
- **IPC Command Registration (`src-tauri/src/main.rs`):** `tauri::generate_handler![]` registers **407 IPC commands**:
  - `381` explicit commands across 41 files (note: `3` `#[tauri::command]` functions in source — `delete_shipment`, `get_shipment_by_id`, `generate_id` — exist in code but are omitted from `generate_handler![]`).
  - `26` macro-generated commands from `commands/options.rs` (`13` `get_*` + `13` `add_*` lookup table accessors).

---

## 3. Application Startup, Initialization & Lifecycle

In `src-tauri/src/main.rs`:
1. **Recovery Mode Check (`recovery_mode.rs`):** Checks CLI `--recovery` or environment variable `IMPORT_MANAGER_RECOVERY=1`. Enables emergency recovery commands (`recovery_clear_lockout`, `recovery_reset_security_policy`, `recovery_set_admin_password`).
2. **Database Connection & 3-Layer Schema Init:** Opens SQLite DB in AppData, runs Refinery `V1..V80`, runs `migrations.rs::post_refinery_migrations`, then runs `db.rs::init_schema` runtime DDL + seed inserts (`Sup-001`..`Sup-011`).
3. **Security & Session Initialization:** Loads `desktop_session.rs` state, seeds `role_dashboard_permissions` / `user_roles`, initializes `security_policy_versions`.
4. **Restore Lock Coordinator (`restore_control.rs`):** Maintains global restore flag (`set_restoring`) to pause background jobs during restore and track active connection pool (`connection_manager.rs`).
5. **Crash Recovery (`write_recovery_journal` & `boe_write_recovery`):** Recovers interrupted multi-step writes on startup (`recover_interrupted_writes_command`).
6. **Background Schedulers:**
   - **Fast Tick (`60s`):** Lightweight maintenance/health hooks (`BACKGROUND_FAST_INTERVAL_SECS`).
   - **Heavy Tick (`900s` / `15m`):** Periodic background sweep (`BACKGROUND_HEAVY_INTERVAL_SECS`).
   - **Daily Dashboard & Job Tick (`run_daily_dashboard_tick_jobs`):** Gated by `app_metadata.kpi_retention_last_run`; executes `mark_stuck_jobs_timeout` → `detect_missed_job_runs` → `maintenance_cleanup` → `observability_update` → `automation_cycle` → `cost_metrics_aggregation` → `deployment_safety_checks` → `risk_evaluation` → `detect_schedule_drift` → `recover_missed_jobs_auto` → `refresh_all_job_recovery_scores` → `detect_job_failure_patterns` → `refresh_system_reliability_score` → `scan_and_emit_threshold_alert_signals` → `refresh_incident_daily_metrics` → `scan_systemic_failure_bursts`.
7. **OAuth Loopback Listener (`oauth_callback.rs`):** Ephemeral local HTTP listener on `http://127.0.0.1:8765/` for Google Drive OAuth2 authorization code exchange.

---

## 4. Security Architecture & Legacy Gaps Found

| Security Control | Legacy Implementation | Audit Findings / Gaps to Fix in Next Gen |
| :--- | :--- | :--- |
| **Database At-Rest Encryption** | `src-tauri/src/encryption.rs` | Compiled with `LIBSQLITE3_SYS_BUNDLED=1`; `encryption.rs` behaves as a stub/no-op on bundled SQLite without SQLCipher linked. **Next Gen MUST enforce real SQLCipher encryption on every DB.** |
| **Password Hashing** | Argon2id + bcrypt fallback | Supports password history (`[1..24]`, default `5`), complexity (`>=8` chars, upper, lower, digit, symbol, no whitespace), lockout (`5` failures / `15m` window / `30m` lock). Preserve exact policy and Argon2id default. |
| **Session & RBAC** | `4` roles (`administrator`, `manager`, `operator`, `viewer`), `25` permissions, `12h` TTL (`30d` remember me), `30m` idle timeout | Several `workflow_*` Rust commands accept `caller_role: String` directly from frontend IPC and match substrings (`admin`/`automationmanager`) instead of reading authoritative server session state. **Next Gen MUST derive caller identity/role solely from backend session context.** |
| **Backup Encryption** | `IMBK2` (`PBKDF2-HMAC-SHA256` 600,000 iters + 16B salt + 12B nonce + AES-256-GCM + `.sha256` sidecar); legacy read-only `IMBK1` (100,000 iters) | Strong design. Keep exact binary header/format compatibility so Next Gen can restore legacy `.imbk` / `IMBK2` / `IMBK1` backups seamlessly. |
| **Credential Storage** | Windows Credential Manager (`service="ImportManager"`) | Stores `backup_encryption_password`, `google_drive_refresh_token`, `google_drive_user_email`, `system_agent_deepseek_api_key`. Supports `.imkey` (32B base64) key export/import. |
| **Destructive Action Guards** | PIN verify + lockout for hard delete, `'DELETE'` typed confirmation (`>5000` deps), pre-restore safety backup | Keep all destructive safety gates and audit logging (`audit_logs`, `user_activity_audit_logs`). |

---

## 5. Legacy Schema & Runtime Drift Findings

Clean replay of `src-tauri/migrations/V1__initial_schema.sql` .. `V80__backfill_expense_paise_from_decimal.sql` in standard SQLite 3.45.3 succeeds on **77 of 80** migrations and fails on **3 migrations** due to legacy coupling with runtime Rust DDL:

1. **`V52__supplier_pagination_indexes.sql`**: Fails (`no such column: deleted_at`) because `suppliers.deleted_at` is added at runtime in `migrations.rs::post_refinery_migrations`, not in `V1..V51`.
2. **`V55__db_management_performance_indexes.sql`**: Fails (`no such column: deleted_at`) for the same reason on soft-deleted core tables.
3. **`V80__backfill_expense_paise_from_decimal.sql`**: Fails (`no such column: amount_paise`) because `expenses.amount_paise` (and related `*_paise` / `*_bp` columns) are added dynamically by `db.rs::init_schema` rather than an earlier `V*` migration.

**Architectural Consequence for Next Gen:**  
- **New Databases:** All schema changes must live strictly inside sequential Go SQL migrations (with SHA-256 checksums) — **zero runtime DDL** outside migrations.  
- **Legacy DB Importer:** Must inspect actual SQLite `PRAGMA table_info` / `sqlite_master` schemas across all 3 layers rather than assuming `refinery_schema_history` version alone reflects column state.

---

## 6. Observed Legacy Quirks, Inconsistencies & Bugs

1. **Duty Calculator Currency Unit Ambiguity (`duty-calculator.ts` vs `form.tsx`):** `freightCost` and `exwCost` are divided by foreign `totalInvoiceValue` and multiplied by `itemForexValue` without `exRate` — meaning inputs are treated as **INR** even though UI labels omit `(INR)` and `calculation-results.tsx` hardcodes `USD 1 = INR`. Preserve the exact calculation semantics in Next Gen while labeling units clearly and dynamically displaying `${shipment.invoiceCurrency}`.
2. **Hardcoded ICEGATE PDF Constants (`export-boe-pdf.ts`):** Port Code `INMAA1`, Origin/Consignment `KOREA, REPUBLIC OF`, Port of Loading `Busan (Korea)`, `CETH: NOEXCISE`, `UQC: PCS`, `S.UQC: KGS` are hardcoded. In Next Gen, preserve defaults while making port/origin/UQC configurable from shipment/item/settings data.
3. **`ExpenseService::create_or_update_invoice` (`expense.rs:376`):** Returns `"Update not implemented"` when provider+invoice already exists, whereas older `add_expense_invoice_with_expenses` overwrites lines across separate transactions. Dual columns exist on `expense_invoices` (`invoice_no` & `invoice_number`, `total_amount` REAL & `total_amount_paise` INTEGER).
4. **Naive CSV Parsing in Legacy Importers:** `expense-import.tsx` and Supplier CSV import use `line.split(',')`, breaking on quoted commas (unlike `shipment-import.ts` / `csv-helpers.ts` which use PapaParse). Next Gen will use proper RFC 4180 CSV parsing everywhere.
5. **Duplicated/Orphaned Backend Modules:** `items` lookups vs `options` macro, dual `expenses.rs` + `expense.rs` implementations, and extensive AI/DeepSeek `system_agent.rs` & workflow simulation tables (`~90` operational/workflow/incident tables vs `~25` core ERP tables). All must be cleanly organized by Go service boundary.

---

## 7. CI/CD & Quality Gates (Legacy Reference)

All 7 workflows target `windows-latest` (except `gitleaks.yml` on `ubuntu-latest`):
- **`ci.yml`**: `frontend-lint` (Node 20/22: eslint, prettier, tsc), `backend-check` (`cargo check`, `clippy -D warnings`, `fmt --check`, `cargo deny`, `cargo audit`, C1 gate banning `.lock().unwrap()` in `commands/*.rs`), `unit-tests` (Vitest + Cargo tests + coverage upload to Codecov), `e2e-tests` (Playwright CI), `build` (bundle MSI/NSIS EXE).
- **`release.yml`**: `v*` tag trigger, signed Windows MSI/EXE via `signtool.exe` (SHA-256 + DigiCert timestamp) and updater manifest.
- **`nightly.yml`**: `0 2 * * *` cron running unit, E2E, and performance tests.
- **`format.yml`, `gitleaks.yml`, `workflow-lint.yml`, `dependabot-auto-merge.yml`**: Auto-formatting, secret scanning (`gitleaks 8.30.1`), `actionlint 1.7.12`, Dependabot patch/minor auto-merge.

---

## 8. Technical Risks for Next Gen (Wails 3 + Go + SQLCipher on Windows)

| Risk Area | Description & Mitigation Strategy |
| :--- | :--- |
| **SQLCipher CGO on Windows** | Requires MinGW/GCC (`C:\Strawberry\c\bin\gcc.exe` verified available) with `-tags "sqlite_sqlcipher"` or `github.com/mutecomm/go-sqlcipher/v4` / `ncruces`/SQLCipher amalgamation cleanly built in Stage 1 before domain coding. Verify `PRAGMA cipher_version;` returns non-empty version and DB header is unreadable without key. |
| **WAL Mode + Encrypted Restore** | SQLite WAL creates `-wal`/`-shm` sidecars; live restore/backup must acquire application-level read/write barriers (`restore_control`), `PRAGMA wal_checkpoint(TRUNCATE)`, close pool cleanly (or use SQLite Online Backup API / table-copy transaction), verify integrity, and swap safely. |
| **Excel 2016 Compatibility** | Legacy uses `ExcelJS`; Next Gen Go/frontend exports (`.xlsx`) must avoid unsupported OOXML extensions, formulas with dynamic array `_xlfn.` prefixes, or styles rejected by Excel 2016+. |
| **Wails 3 Stability** | Verify exact Wails 3 release API (`v3` Go service bindings, native Windows file dialogs, single-instance lock, WebView2 asset serving, NSIS/MSI packaging) in Stage 1 prototype. |
