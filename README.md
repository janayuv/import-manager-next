# Import Manager

Desktop and mobile ERP for import shipments, invoices, Bills of Entry (BOE), customs duty & landed cost analysis, RoDTEP scrips, and encrypted backups.

## Architecture

- **Desktop Backend (`backend/`, `main.go`, `app.go`)**: Go (`1.26`) + Wails v3 (`v3.1.0`) + SQLCipher (`go-sqlcipher/v4`) with database encryption keys stored in Windows Credential Manager (`backend/crypto/keystore.go`).
- **Desktop Frontend (`frontend/`)**: React + TypeScript + Vite (`npm` scripts in `frontend/package.json`).
- **Mobile Companion (`mobile/`)**: Flutter (`import_manager_mobile`) supporting local `.imbk2` restore, opt-in LAN sync, and user-configured Google Drive OAuth.
- **Documentation (`docs/`)**: Business rules, workflow docs, schema compatibility, and feature inventory (`docs/BUSINESS_RULES.md`, `docs/ERP_WORKFLOW_DOCUMENTATION.md`, `docs/FEATURE_INVENTORY.md`, `docs/LEGACY_DATABASE_SCHEMA.md`, `docs/MIGRATION_AND_COMPATIBILITY_PLAN.md`).

## Prerequisites

- Go `1.26.x` with CGO enabled (for SQLCipher)
- Node.js `22.x` & `npm`
- Wails 3 CLI (`v3.1.0`): `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0`
- NSIS (for Windows installer packaging)
- Flutter SDK (only for `mobile/`)

## Common Commands

### Desktop Tests & Build

```powershell
# Run all Go tests
go test -tags "json1,sqlite_fts5,sqlite_foreign_keys" ./...

# Frontend type-check, unit tests, and build
cd frontend
npm ci
npm run type-check
npm test
npm run build

# Build desktop binary / Windows installer via Wails 3
wails3 task build
wails3 task windows:package
```

### Mobile Companion Tests

```powershell
cd mobile
flutter test
```
