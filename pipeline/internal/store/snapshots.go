package store

import (
	"context"
	"database/sql"
	"fmt"
)

// Snapshot is one published ranking list: a source, a weight class, a date.
// Dates are ISO-8601 TEXT, matching the SQLite schema (see db/migrations).
type Snapshot struct {
	ID            int64
	SourceID      int64
	WeightClass   int
	Season        int
	PublishedDate string // ISO-8601 date, e.g. "2026-01-13"
	CapturedAt    string // ISO-8601 datetime, e.g. "2026-01-13T18:04:00Z"
}

// RankingEntry is a wrestler's position within one snapshot. WrestlerID is nil
// until the raw entry is resolved to a canonical wrestler. RawSourceString is
// the published identity string (e.g. the name cell); RawSchool and RawGrade
// hold the school and eligibility as published that week (point-in-time, hence
// on the entry, not the canonical wrestler). All three are retained verbatim;
// RawSchool/RawGrade are nil for sources that don't split those fields.
type RankingEntry struct {
	ID              int64
	SnapshotID      int64
	WrestlerID      *int64
	Rank            int
	RawSourceString string
	RawSchool       *string
	RawGrade        *string
}

// Movement pairs an entry's current rank with that wrestler's previous rank for
// the same source+weight+season, derived via LAG over published_date
// (schema.md §5). PrevRank is nil for a wrestler's first appearance.
type Movement struct {
	SnapshotID      int64
	PublishedDate   string
	WrestlerID      *int64
	Rank            int
	PrevRank        *int64
	RawSourceString string
}

// InsertSnapshot inserts a snapshot row and returns its new id.
func InsertSnapshot(ctx context.Context, db *sql.DB, s Snapshot) (int64, error) {
	res, err := db.ExecContext(ctx,
		`INSERT INTO snapshots (source_id, weight_class, season, published_date, captured_at)
		 VALUES (?, ?, ?, ?, ?)`,
		s.SourceID, s.WeightClass, s.Season, s.PublishedDate, s.CapturedAt)
	if err != nil {
		return 0, fmt.Errorf("insert snapshot: %w", err)
	}
	return res.LastInsertId()
}

// InsertRankingEntry inserts an entry row and returns its new id. A nil
// WrestlerID is stored as NULL (unresolved).
func InsertRankingEntry(ctx context.Context, db *sql.DB, e RankingEntry) (int64, error) {
	res, err := db.ExecContext(ctx,
		`INSERT INTO ranking_entries (snapshot_id, wrestler_id, rank, raw_source_string, raw_school, raw_grade)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		e.SnapshotID, e.WrestlerID, e.Rank, e.RawSourceString, e.RawSchool, e.RawGrade)
	if err != nil {
		return 0, fmt.Errorf("insert ranking entry: %w", err)
	}
	return res.LastInsertId()
}

// GetSnapshot reads a single snapshot by id.
func GetSnapshot(ctx context.Context, db *sql.DB, id int64) (Snapshot, error) {
	var s Snapshot
	err := db.QueryRowContext(ctx,
		`SELECT id, source_id, weight_class, season, published_date, captured_at
		 FROM snapshots WHERE id = ?`, id).
		Scan(&s.ID, &s.SourceID, &s.WeightClass, &s.Season, &s.PublishedDate, &s.CapturedAt)
	if err != nil {
		return Snapshot{}, fmt.Errorf("get snapshot %d: %w", id, err)
	}
	return s, nil
}

// ListEntries reads all entries for a snapshot, ordered by rank then
// raw_source_string — the latter keeps a tie (two entries sharing a rank; see
// schema.md §7) in a stable, deterministic order rather than SQLite's arbitrary
// order for equal keys.
func ListEntries(ctx context.Context, db *sql.DB, snapshotID int64) ([]RankingEntry, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, snapshot_id, wrestler_id, rank, raw_source_string, raw_school, raw_grade
		 FROM ranking_entries WHERE snapshot_id = ? ORDER BY rank, raw_source_string`, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("list entries for snapshot %d: %w", snapshotID, err)
	}
	defer rows.Close()

	var out []RankingEntry
	for rows.Next() {
		var e RankingEntry
		if err := rows.Scan(&e.ID, &e.SnapshotID, &e.WrestlerID, &e.Rank, &e.RawSourceString, &e.RawSchool, &e.RawGrade); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// MovementForWeight returns every entry for one source/weight/season with the
// wrestler's previous rank attached via LAG over published_date (schema.md §5).
// Rows are ordered by published_date, then rank, then raw_source_string — the
// last tiebreaker keeps a tie rank (schema.md §7) deterministically ordered.
func MovementForWeight(ctx context.Context, db *sql.DB, sourceID int64, weightClass, season int) ([]Movement, error) {
	rows, err := db.QueryContext(ctx, `
SELECT e.snapshot_id, s.published_date, e.wrestler_id, e.rank,
       LAG(e.rank) OVER (
         PARTITION BY s.source_id, s.weight_class, s.season, e.wrestler_id
         ORDER BY s.published_date
       ) AS prev_rank,
       e.raw_source_string
FROM ranking_entries e
JOIN snapshots s ON s.id = e.snapshot_id
WHERE s.source_id = ? AND s.weight_class = ? AND s.season = ?
ORDER BY s.published_date, e.rank, e.raw_source_string`,
		sourceID, weightClass, season)
	if err != nil {
		return nil, fmt.Errorf("movement query: %w", err)
	}
	defer rows.Close()

	var out []Movement
	for rows.Next() {
		var m Movement
		if err := rows.Scan(&m.SnapshotID, &m.PublishedDate, &m.WrestlerID, &m.Rank, &m.PrevRank, &m.RawSourceString); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
