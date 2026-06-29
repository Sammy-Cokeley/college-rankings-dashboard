package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// ApplyMigrations runs every *.sql file in dir, in filename order, that has not
// already been recorded in schema_migrations. The schema_migrations ledger is
// bootstrapped here rather than as a numbered migration to avoid the
// chicken-and-egg problem of tracking the table that does the tracking. Each
// pending file is applied inside its own transaction.
func ApplyMigrations(ctx context.Context, db *sql.DB, dir string) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
  filename   TEXT PRIMARY KEY,
  applied_at TEXT NOT NULL
)`); err != nil {
		return fmt.Errorf("bootstrap schema_migrations: %w", err)
	}

	files, err := sqlFiles(dir)
	if err != nil {
		return err
	}

	for _, path := range files {
		name := filepath.Base(path)

		applied, err := migrationApplied(ctx, db, name)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		if err := applyFileTx(ctx, db, path, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx,
				`INSERT INTO schema_migrations (filename, applied_at) VALUES (?, ?)`,
				name, time.Now().UTC().Format(time.RFC3339))
			return err
		}); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
	}
	return nil
}

// ApplySeeds runs every *.sql file in dir unconditionally, in filename order.
// Seed files are expected to be idempotent (e.g. INSERT OR IGNORE) since they
// run on every invocation and are not tracked.
func ApplySeeds(ctx context.Context, db *sql.DB, dir string) error {
	files, err := sqlFiles(dir)
	if err != nil {
		return err
	}
	for _, path := range files {
		if err := applyFileTx(ctx, db, path, nil); err != nil {
			return fmt.Errorf("apply seed %s: %w", filepath.Base(path), err)
		}
	}
	return nil
}

func migrationApplied(ctx context.Context, db *sql.DB, name string) (bool, error) {
	var one int
	err := db.QueryRowContext(ctx,
		`SELECT 1 FROM schema_migrations WHERE filename = ?`, name).Scan(&one)
	switch {
	case err == nil:
		return true, nil
	case err == sql.ErrNoRows:
		return false, nil
	default:
		return false, fmt.Errorf("check migration %s: %w", name, err)
	}
}

func sqlFiles(dir string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", dir, err)
	}
	sort.Strings(files)
	return files, nil
}

// applyFileTx executes the SQL in path inside one transaction. The optional
// record callback runs before commit (used to mark a migration applied), so the
// file's effects and its ledger entry land atomically.
func applyFileTx(ctx context.Context, db *sql.DB, path string, record func(*sql.Tx) error) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, string(body)); err != nil {
		tx.Rollback()
		return err
	}
	if record != nil {
		if err := record(tx); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}
