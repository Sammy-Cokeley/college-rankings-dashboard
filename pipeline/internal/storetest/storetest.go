// Package storetest provides shared Postgres test-database setup for the
// pipeline's internal packages (ingest, resolve) that need a real, isolated
// database per test. It duplicates a small amount of logic from
// store/testdb_test.go rather than depending on store's test files — Go
// doesn't let one package import another package's _test.go files, and having
// store's own tests import this package (which imports store) would be an
// import cycle.
package storetest

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"pipeline/internal/store"
)

// repo-root-relative dirs. Relative paths resolve against the calling test's
// own package directory (go test's cwd), which is fine here because store,
// ingest, resolve, and storetest all sit at the same depth under
// pipeline/internal/.
var (
	MigrationsDir = filepath.Join("..", "..", "..", "db", "migrations")
	SeedDir       = filepath.Join("..", "..", "..", "db", "seed")
)

// NewDB opens a *sql.DB scoped to a brand-new, empty Postgres schema (never
// reused, dropped on test cleanup) with migrations + seeds already applied.
// See store/testdb_test.go's freshDB for the full rationale.
func NewDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()

	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL not set — start Postgres (docker compose up -d) and set it; see .env.example")
	}

	schema := fmt.Sprintf("test_%d_%d", os.Getpid(), rand.Int63())
	u, err := url.Parse(base)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()

	db, err := store.OpenForMigrations(u.String())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DROP SCHEMA IF EXISTS "`+schema+`" CASCADE`)
		db.Close()
	})

	if _, err := db.ExecContext(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	if err := store.ApplyMigrations(ctx, db, MigrationsDir); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	if err := store.ApplySeeds(ctx, db, SeedDir); err != nil {
		t.Fatalf("seeds: %v", err)
	}
	return db
}
