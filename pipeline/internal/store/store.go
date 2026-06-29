// Package store is the SQLite persistence layer shared by the pipeline binaries.
//
// It depends only on database/sql; the concrete driver (modernc.org/sqlite) is
// blank-imported by the cmd/* binaries and the tests, never by this package.
// That keeps the storage contract driver-agnostic and the cgo-free driver
// choice an edge-of-the-program detail.
package store

import (
	"database/sql"
	"fmt"
)

// dsn builds the modernc.org/sqlite connection string. The pragmas are passed
// per-connection (via _pragma) so every pooled connection enforces foreign
// keys, uses WAL, and waits out a busy database rather than erroring instantly.
func dsn(path string) string {
	return fmt.Sprintf(
		"file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)",
		path,
	)
}

// Open returns a *sql.DB for the SQLite file at path. The "sqlite" driver name
// must already be registered by the caller (blank-import modernc.org/sqlite).
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}
	// SQLite allows only one writer at a time. Capping the pool at a single
	// connection serializes writes in-process and avoids "database is locked".
	db.SetMaxOpenConns(1)
	return db, nil
}
