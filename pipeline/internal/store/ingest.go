package store

import (
	"context"
	"database/sql"
	"fmt"
)

// IngestEdition writes one published ranking list — a snapshot plus all its
// entries — atomically and idempotently.
//
// Idempotency keys off the snapshot's UNIQUE(source_id, weight_class, season,
// published_date). Because the snapshot and its entries are committed in a
// single transaction, a snapshot's existence implies its entries are already
// fully present, so a re-run that finds the snapshot returns (id, false, nil)
// without re-inserting anything. The caller passes a Snapshot with ID unset and
// entries with SnapshotID/ID unset (and WrestlerID nil — resolution is a later
// pass); IngestEdition fills SnapshotID on each entry before insert.
//
// The returned bool is true when a new snapshot (and its entries) were written,
// false when the edition was already present.
func IngestEdition(ctx context.Context, db *sql.DB, snap Snapshot, entries []RankingEntry) (int64, bool, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, err
	}
	defer tx.Rollback() // no-op after a successful Commit

	if id, found, err := existingSnapshotID(ctx, tx, snap); err != nil {
		return 0, false, err
	} else if found {
		return id, false, nil
	}

	var snapshotID int64
	err = tx.QueryRowContext(ctx,
		`INSERT INTO snapshots (source_id, weight_class, season, published_date, captured_at)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		snap.SourceID, snap.WeightClass, snap.Season, snap.PublishedDate, snap.CapturedAt).Scan(&snapshotID)
	if err != nil {
		return 0, false, fmt.Errorf("insert snapshot: %w", err)
	}

	for _, e := range entries {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO ranking_entries (snapshot_id, wrestler_id, rank, raw_source_string, raw_school, raw_grade)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			snapshotID, e.WrestlerID, e.Rank, e.RawSourceString, e.RawSchool, e.RawGrade); err != nil {
			return 0, false, fmt.Errorf("insert entry (rank %d, %q): %w", e.Rank, e.RawSourceString, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, false, fmt.Errorf("commit edition: %w", err)
	}
	return snapshotID, true, nil
}

func existingSnapshotID(ctx context.Context, tx *sql.Tx, snap Snapshot) (int64, bool, error) {
	var id int64
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM snapshots
		 WHERE source_id = $1 AND weight_class = $2 AND season = $3 AND published_date = $4`,
		snap.SourceID, snap.WeightClass, snap.Season, snap.PublishedDate).Scan(&id)
	switch {
	case err == nil:
		return id, true, nil
	case err == sql.ErrNoRows:
		return 0, false, nil
	default:
		return 0, false, fmt.Errorf("lookup snapshot: %w", err)
	}
}

// SourceID returns the id of a source by its unique name (e.g. "FloWrestling").
func SourceID(ctx context.Context, db *sql.DB, name string) (int64, error) {
	var id int64
	if err := db.QueryRowContext(ctx,
		`SELECT id FROM sources WHERE name = $1`, name).Scan(&id); err != nil {
		return 0, fmt.Errorf("source %q: %w", name, err)
	}
	return id, nil
}
