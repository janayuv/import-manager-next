package migrations_test

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mutecomm/go-sqlcipher/v4"
	"github.com/janayuv/import-manager-next/backend/database/migrations"
)

func TestReconciledAuthoritativeTableInventory(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	res, err := migrations.Run(ctx, db, "2.1.0", nil)
	if err != nil {
		t.Fatalf("migrations.Run failed: %v", err)
	}

	if res.CurrentVersion != 20 {
		t.Fatalf("expected 20 applied migrations, got %d", res.CurrentVersion)
	}

	rows, err := db.QueryContext(ctx, `
		SELECT type, name FROM sqlite_schema
		WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%'
		ORDER BY type ASC, name ASC
	`)
	if err != nil {
		t.Fatalf("failed to query sqlite_schema: %v", err)
	}
	defer rows.Close()

	tables := make([]string, 0)
	views := make([]string, 0)

	for rows.Next() {
		var objType, name string
		if err := rows.Scan(&objType, &name); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
		if objType == "table" {
			tables = append(tables, name)
		} else if objType == "view" {
			views = append(views, name)
		}
	}

	// Verify exact count: 172 tables, 1 view
	if len(tables) != 172 {
		t.Errorf("expected exactly 172 tables, got %d", len(tables))
	}
	if len(views) != 1 || views[0] != "report_view" {
		t.Errorf("expected exactly 1 view ('report_view'), got %v", views)
	}
}
