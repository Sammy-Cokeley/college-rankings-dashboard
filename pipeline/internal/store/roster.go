package store

import (
	"context"
	"fmt"
)

// RosterEntry is one wrestler's listing on a school's roster for a season —
// not a ranked, dated edition (schema.md §8), so it doesn't reuse
// Snapshot/RankingEntry's shape. WrestlerID is nil until resolved.
type RosterEntry struct {
	ID              int64
	SchoolID        int64
	WrestlerID      *int64
	Season          int
	WeightClass     *int
	RawName         string // cleaned name used for matching (ingest/roster.go)
	RawSourceString string // full published cell text, verbatim
	CapturedAt      string
}

// UpsertRosterEntry inserts or updates a roster entry, keyed by
// UNIQUE(school_id, season, raw_name). Unlike ranking snapshots (immutable
// once ingested), a roster is current state re-scraped periodically — a
// wrestler's weight_class can and does change through the season
// (docs/sources/wrestlestat.md), so a re-scrape must update the existing row,
// not skip it.
func UpsertRosterEntry(ctx context.Context, q DBTX, e RosterEntry) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `
INSERT INTO roster_entries (school_id, season, weight_class, raw_name, raw_source_string, captured_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (school_id, season, raw_name) DO UPDATE SET
  weight_class      = EXCLUDED.weight_class,
  raw_source_string = EXCLUDED.raw_source_string,
  captured_at       = EXCLUDED.captured_at
RETURNING id`,
		e.SchoolID, e.Season, e.WeightClass, e.RawName, e.RawSourceString, e.CapturedAt).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("upsert roster entry %q: %w", e.RawName, err)
	}
	return id, nil
}

// ListUnresolvedRosterEntries returns every roster entry with a NULL
// wrestler_id for the given season, with each entry's already-resolved school
// name attached (roster_entries.school_id is never NULL — school resolution
// happens inline at ingest, unlike wrestler resolution's separate pass; see
// FindOrCreateSchool) so the caller can feed (RawName, school name) straight
// into the same matcher wrestler resolution uses.
func ListUnresolvedRosterEntries(ctx context.Context, q DBTX, season int) ([]UnresolvedEntry, error) {
	rows, err := q.QueryContext(ctx, `
SELECT re.id, re.raw_name, s.name
FROM roster_entries re
JOIN schools s ON s.id = re.school_id
WHERE re.wrestler_id IS NULL AND re.season = $1
ORDER BY re.id`, season)
	if err != nil {
		return nil, fmt.Errorf("list unresolved roster entries: %w", err)
	}
	defer rows.Close()

	var out []UnresolvedEntry
	for rows.Next() {
		var e UnresolvedEntry
		var school string
		if err := rows.Scan(&e.EntryID, &e.RawName, &school); err != nil {
			return nil, err
		}
		e.RawSchool = &school
		out = append(out, e)
	}
	return out, rows.Err()
}

// SetRosterEntryWrestler resolves a roster entry by attaching its canonical
// wrestler, mirroring SetEntryWrestler for ranking_entries.
func SetRosterEntryWrestler(ctx context.Context, q DBTX, entryID, wrestlerID int64) error {
	if _, err := q.ExecContext(ctx,
		`UPDATE roster_entries SET wrestler_id = $1 WHERE id = $2`, wrestlerID, entryID); err != nil {
		return fmt.Errorf("set roster entry %d wrestler: %w", entryID, err)
	}
	return nil
}
