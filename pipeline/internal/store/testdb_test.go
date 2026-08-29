package store

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// freshDB opens a *sql.DB scoped to a brand-new, empty Postgres schema (never
// reused, dropped on test cleanup) with migrations + seeds already applied.
// Postgres has no SQLite-style "throwaway temp file" — a per-test schema on a
// shared instance (docker-compose.yml; connection via $TEST_DATABASE_URL) is
// the equivalent isolation: nothing a test does is visible to any other test.
func freshDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()

	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL not set — start Postgres (docker compose up -d) and set it; see .env.example")
	}

	schema := fmt.Sprintf("test_%d_%d", os.Getpid(), rand.Int63())
	scopedURL, err := withSchema(base, schema)
	if err != nil {
		t.Fatalf("scope test db url: %v", err)
	}

	db, err := OpenForMigrations(scopedURL)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() {
		// Best-effort: drop the schema so a long test run doesn't accumulate
		// hundreds of them, then close. A failure here doesn't fail the test —
		// the schema is still uniquely named and harmless to leave behind.
		_, _ = db.ExecContext(ctx, `DROP SCHEMA IF EXISTS "`+schema+`" CASCADE`)
		db.Close()
	})

	if _, err := db.ExecContext(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	if err := ApplyMigrations(ctx, db, migrationsDir); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	if err := ApplySeeds(ctx, db, seedDir); err != nil {
		t.Fatalf("seeds: %v", err)
	}
	return db
}

// withSchema sets the connection's search_path to schema, so every unqualified
// table/statement (including the migration/seed files themselves) lands there
// instead of the default "public" schema.
func withSchema(connURL, schema string) (string, error) {
	u, err := url.Parse(connURL)
	if err != nil {
		return "", fmt.Errorf("parse connection url: %w", err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String(), nil
}
