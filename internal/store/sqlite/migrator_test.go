package sqlite_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/WAY29/SimplePool/internal/store/sqlite"
)

func TestMigrateCreatesAllTablesAndIsIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "simplepool.db")
	db, err := sqlite.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if err := sqlite.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() first error = %v", err)
	}

	firstCount := migrationCount(t, db)

	if err := sqlite.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() second error = %v", err)
	}

	secondCount := migrationCount(t, db)
	if secondCount != firstCount {
		t.Fatalf("migration count = %d, want %d", secondCount, firstCount)
	}

	expectedTables := []string{
		"schema_migrations",
		"admin_users",
		"sessions",
		"subscription_sources",
		"nodes",
		"groups",
		"tunnels",
		"tunnel_events",
		"latency_samples",
	}

	for _, table := range expectedTables {
		var name string
		err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
		if err != nil {
			t.Fatalf("missing table %q: %v", table, err)
		}
	}
}

func migrationCount(t *testing.T, db *sql.DB) int {
	t.Helper()

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("QueryRow() error = %v", err)
	}

	return count
}
