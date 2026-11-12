# Unified test runner enabling SQLite JSON1, FTS5, ForeignKeys and Go race detector
$ErrorActionPreference = "Stop"
$env:CGO_ENABLED = "1"
go test -tags "json1,sqlite_fts5,sqlite_foreign_keys" -race ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Push-Location (Join-Path $PSScriptRoot "..\frontend")
try {
  npm run type-check
  if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
  npm test
  if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
} finally {
  Pop-Location
}
