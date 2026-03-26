package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/WAY29/SimplePool/internal/apperr"
	"github.com/WAY29/SimplePool/store/migrations"
	_ "modernc.org/sqlite"
)

type Migrator struct {
	fsys fs.FS
}

type migrationFile struct {
	version  string
	checksum string
	sql      string
}

func Open(ctx context.Context, path string) (*sql.DB, error) {
	const op = "sqlite.Open"

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, apperr.Wrap(apperr.CodeStore, op, err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeStore, op, err)
	}

	db.SetMaxOpenConns(1)

	for _, statement := range []string{
		"PRAGMA foreign_keys = ON;",
		"PRAGMA journal_mode = WAL;",
		"PRAGMA busy_timeout = 5000;",
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			_ = db.Close()
			return nil, apperr.Wrap(apperr.CodeStore, op, fmt.Errorf("exec %q: %w", statement, err))
		}
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, apperr.Wrap(apperr.CodeStore, op, err)
	}

	return db, nil
}

func NewMigrator(fsys fs.FS) *Migrator {
	return &Migrator{fsys: fsys}
}

func Migrate(ctx context.Context, db *sql.DB) error {
	return NewMigrator(migrations.Files).Apply(ctx, db)
}

func (m *Migrator) Apply(ctx context.Context, db *sql.DB) error {
	const op = "sqlite.Migrator.Apply"

	if m == nil || m.fsys == nil {
		return apperr.New(apperr.CodeStore, op, "migration filesystem is required")
	}

	if err := ensureMigrationTable(ctx, db); err != nil {
		return apperr.Wrap(apperr.CodeStore, op, err)
	}

	applied, err := loadAppliedMigrations(ctx, db)
	if err != nil {
		return apperr.Wrap(apperr.CodeStore, op, err)
	}

	files, err := loadMigrationFiles(m.fsys)
	if err != nil {
		return apperr.Wrap(apperr.CodeStore, op, err)
	}

	for _, file := range files {
		if checksum, ok := applied[file.version]; ok {
			if checksum != file.checksum {
				return apperr.New(apperr.CodeStore, op, fmt.Sprintf("migration %s checksum mismatch", file.version))
			}
			continue
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return apperr.Wrap(apperr.CodeStore, op, err)
		}

		if _, err := tx.ExecContext(ctx, file.sql); err != nil {
			_ = tx.Rollback()
			return apperr.Wrap(apperr.CodeStore, op, fmt.Errorf("apply migration %s: %w", file.version, err))
		}

		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO schema_migrations(version, checksum, applied_at) VALUES(?, ?, ?)`,
			file.version,
			file.checksum,
			time.Now().UTC().Format(time.RFC3339Nano),
		); err != nil {
			_ = tx.Rollback()
			return apperr.Wrap(apperr.CodeStore, op, fmt.Errorf("record migration %s: %w", file.version, err))
		}

		if err := tx.Commit(); err != nil {
			return apperr.Wrap(apperr.CodeStore, op, err)
		}
	}

	return nil
}

func ensureMigrationTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    checksum TEXT NOT NULL,
    applied_at TEXT NOT NULL
);`)
	return err
}

func loadAppliedMigrations(ctx context.Context, db *sql.DB) (map[string]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT version, checksum FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]string)
	for rows.Next() {
		var version string
		var checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			return nil, err
		}

		applied[version] = checksum
	}

	return applied, rows.Err()
}

func loadMigrationFiles(fsys fs.FS) ([]migrationFile, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}

	sort.Strings(names)

	files := make([]migrationFile, 0, len(names))
	for _, name := range names {
		content, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}

		sum := sha256.Sum256(content)
		files = append(files, migrationFile{
			version:  strings.TrimSuffix(name, filepath.Ext(name)),
			checksum: hex.EncodeToString(sum[:]),
			sql:      string(content),
		})
	}

	return files, nil
}
