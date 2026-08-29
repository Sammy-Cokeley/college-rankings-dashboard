// Package store is the Postgres persistence layer shared by the pipeline
// binaries.
//
// It depends only on database/sql; the concrete driver (pgx's stdlib adapter)
// is blank-imported by the cmd/* binaries and the tests, never by this
// package. That keeps the storage contract driver-agnostic and the driver
// choice an edge-of-the-program detail.
package store

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
)

// ResolveDBURL returns flagValue if non-empty, else $DATABASE_URL. Every
// cmd/* binary's -db flag defaults to "" and resolves through this so the
// connection string can come from the environment (the normal way a PaaS host
// configures it) without every command re-implementing the fallback.
func ResolveDBURL(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return os.Getenv("DATABASE_URL")
}

// Open returns a *sql.DB for the given Postgres connection string (a
// "postgres://user:pass@host:port/dbname?sslmode=..." URL, or any DSN pgx
// accepts). Unlike the SQLite predecessor, Postgres handles real concurrent
// writers natively, so the pool is left at its default size — no artificial
// single-connection cap.
func Open(connURL string) (*sql.DB, error) {
	db, err := sql.Open("pgx", connURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	return db, nil
}

// OpenForMigrations is like Open, but forces pgx's simple query protocol.
// ApplyMigrations/ApplySeeds exec a whole *.sql file as one string, and a file
// may contain several statements (several CREATE TABLEs, a table-rebuild,
// etc.) — Postgres's extended protocol (pgx's default, used by Open) rejects
// more than one statement per Parse message. The simple protocol has no such
// limit, at the cost of server-side statement caching, which migrate/seed
// don't need anyway (each file runs at most once per process).
func OpenForMigrations(connURL string) (*sql.DB, error) {
	withMode, err := withQueryExecMode(connURL, "simple_protocol")
	if err != nil {
		return nil, fmt.Errorf("open postgres for migrations: %w", err)
	}
	return Open(withMode)
}

func withQueryExecMode(connURL, mode string) (string, error) {
	u, err := url.Parse(connURL)
	if err != nil {
		return "", fmt.Errorf("parse connection url: %w", err)
	}
	q := u.Query()
	q.Set("default_query_exec_mode", mode)
	u.RawQuery = q.Encode()
	return u.String(), nil
}
