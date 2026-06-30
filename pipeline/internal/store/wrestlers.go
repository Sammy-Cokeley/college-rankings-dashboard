package store

import (
	"context"
	"database/sql"
	"fmt"
)

// DBTX is the subset of *sql.DB / *sql.Tx that the resolution helpers need.
// Accepting it lets the resolution pass run every read and write inside one
// transaction while the same functions also work against a bare *sql.DB.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// UnresolvedEntry is a ranking entry awaiting resolution: it has no wrestler_id
// yet. The raw identity fields are what the matcher works from.
type UnresolvedEntry struct {
	EntryID   int64
	RawName   string
	RawSchool *string
	RawGrade  *string
}

// Alias is one source raw-string -> canonical wrestler mapping.
type Alias struct {
	WrestlerID int64
	RawName    string
	RawSchool  *string
}

// ListUnresolvedEntries returns every entry with a NULL wrestler_id whose
// snapshot belongs to the given source, ordered by entry id for deterministic
// processing.
func ListUnresolvedEntries(ctx context.Context, q DBTX, sourceID int64) ([]UnresolvedEntry, error) {
	rows, err := q.QueryContext(ctx, `
SELECT e.id, e.raw_source_string, e.raw_school, e.raw_grade
FROM ranking_entries e
JOIN snapshots s ON s.id = e.snapshot_id
WHERE s.source_id = ? AND e.wrestler_id IS NULL
ORDER BY e.id`, sourceID)
	if err != nil {
		return nil, fmt.Errorf("list unresolved entries: %w", err)
	}
	defer rows.Close()

	var out []UnresolvedEntry
	for rows.Next() {
		var e UnresolvedEntry
		if err := rows.Scan(&e.EntryID, &e.RawName, &e.RawSchool, &e.RawGrade); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ListAliases returns all known aliases for a source, used to seed the matcher's
// in-memory index so identities are reused across runs.
func ListAliases(ctx context.Context, q DBTX, sourceID int64) ([]Alias, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT wrestler_id, raw_name, raw_school FROM wrestler_aliases WHERE source_id = ?`,
		sourceID)
	if err != nil {
		return nil, fmt.Errorf("list aliases: %w", err)
	}
	defer rows.Close()

	var out []Alias
	for rows.Next() {
		var a Alias
		if err := rows.Scan(&a.WrestlerID, &a.RawName, &a.RawSchool); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// InsertWrestler creates a canonical wrestler and returns its id.
// current_school_id is left NULL (school canonicalization is out of v0 scope);
// eligibility is informational and may be empty.
func InsertWrestler(ctx context.Context, q DBTX, fullName, eligibility string) (int64, error) {
	var elig any
	if eligibility != "" {
		elig = eligibility
	}
	res, err := q.ExecContext(ctx,
		`INSERT INTO wrestlers (full_name, current_school_id, eligibility_year) VALUES (?, NULL, ?)`,
		fullName, elig)
	if err != nil {
		return 0, fmt.Errorf("insert wrestler %q: %w", fullName, err)
	}
	return res.LastInsertId()
}

// InsertAlias records a source raw-string -> wrestler mapping. The table's
// UNIQUE(source_id, raw_name, raw_school) makes a duplicate alias an error;
// callers add an alias only when one does not already exist.
func InsertAlias(ctx context.Context, q DBTX, wrestlerID, sourceID int64, rawName string, rawSchool *string) error {
	if _, err := q.ExecContext(ctx,
		`INSERT INTO wrestler_aliases (wrestler_id, source_id, raw_name, raw_school) VALUES (?, ?, ?, ?)`,
		wrestlerID, sourceID, rawName, rawSchool); err != nil {
		return fmt.Errorf("insert alias %q/%v: %w", rawName, rawSchool, err)
	}
	return nil
}

// SetEntryWrestler resolves an entry by attaching its canonical wrestler.
func SetEntryWrestler(ctx context.Context, q DBTX, entryID, wrestlerID int64) error {
	if _, err := q.ExecContext(ctx,
		`UPDATE ranking_entries SET wrestler_id = ? WHERE id = ?`, wrestlerID, entryID); err != nil {
		return fmt.Errorf("set entry %d wrestler: %w", entryID, err)
	}
	return nil
}
