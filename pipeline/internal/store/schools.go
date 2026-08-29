package store

import (
	"context"
	"database/sql"
	"fmt"
)

// FindOrCreateSchool resolves a school by an exact alias hit for
// (sourceID, rawName); if none exists, it mints a new canonical schools row
// (using rawName as the canonical name verbatim) and records the alias. v0
// has exactly one source populating schools (WrestleStat), so there is no
// cross-source spelling conflict to reconcile yet — this is intentionally
// simpler than the two-tier normalized-key matcher wrestler resolution uses
// (resolve/resolve.go); revisit if/when a second school-name source exists.
func FindOrCreateSchool(ctx context.Context, q DBTX, sourceID int64, rawName string) (int64, error) {
	var schoolID int64
	err := q.QueryRowContext(ctx,
		`SELECT school_id FROM school_aliases WHERE source_id = $1 AND raw_name = $2`,
		sourceID, rawName).Scan(&schoolID)
	switch {
	case err == nil:
		return schoolID, nil
	case err != sql.ErrNoRows:
		return 0, fmt.Errorf("lookup school alias %q: %w", rawName, err)
	}

	if err := q.QueryRowContext(ctx,
		`INSERT INTO schools (name, division) VALUES ($1, 'DI') RETURNING id`,
		rawName).Scan(&schoolID); err != nil {
		return 0, fmt.Errorf("insert school %q: %w", rawName, err)
	}
	if _, err := q.ExecContext(ctx,
		`INSERT INTO school_aliases (school_id, source_id, raw_name) VALUES ($1, $2, $3)`,
		schoolID, sourceID, rawName); err != nil {
		return 0, fmt.Errorf("insert school alias %q: %w", rawName, err)
	}
	return schoolID, nil
}
