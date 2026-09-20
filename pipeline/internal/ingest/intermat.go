// intermat.go ingests a decoded InterMat Record the same way ingest.go
// ingests a Flo Container: one weight's table becomes a snapshot plus its
// ranking_entries, with wrestler_id left NULL for the later resolution pass.
// A malformed weight table is isolated per-weight into Result.Failures,
// exactly like Container's per-edition isolation — one bad table in a
// record never blocks the other weights in the same record.
package ingest

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"pipeline/internal/scraper/intermat"
	"pipeline/internal/store"
)

// intermatSourceName is the seeded source InterMat records belong to.
const intermatSourceName = "InterMat"

// IntermatRecord ingests every weight table in rec for the given season.
// capturedAt is stamped on each newly created snapshot. rec.PublishedDate is
// shared by every weight (an InterMat record is one point-in-time edition
// covering all weights, unlike Flo's Container which bundles many dated
// editions per weight — docs/sources/intermat.md).
func IntermatRecord(ctx context.Context, db *sql.DB, rec intermat.Record, season int, capturedAt time.Time) (Result, error) {
	sourceID, err := store.SourceID(ctx, db, intermatSourceName)
	if err != nil {
		return Result{}, err
	}
	captured := capturedAt.UTC().Format(time.RFC3339)

	var res Result
	for _, wt := range rec.Weights {
		if err := ingestIntermatWeight(ctx, db, sourceID, rec.PublishedDate, wt, season, captured, &res); err != nil {
			res.Failures = append(res.Failures, EditionFailure{
				WeightClass:   wt.WeightClass,
				PublishedDate: rec.PublishedDate,
				Err:           err,
			})
		}
	}
	return res, nil
}

// ingestIntermatWeight parses and ingests a single weight's raw table,
// updating res on success.
func ingestIntermatWeight(ctx context.Context, db *sql.DB, sourceID int64, publishedDate string, wt intermat.RawWeightTable, season int, captured string, res *Result) error {
	rows, err := intermat.ParseRows(wt.Rows)
	if err != nil {
		return fmt.Errorf("parse table: %w", err)
	}

	// Non-fatal: surface rank oddities (ties/gaps) for eyeballing without
	// blocking ingestion, same signal Flo's ingest already surfaces.
	if issues := detectAnomalies(intermatRowRanks(rows)); len(issues) > 0 {
		res.Anomalies = append(res.Anomalies, EditionAnomaly{
			WeightClass:   wt.WeightClass,
			PublishedDate: publishedDate,
			Issues:        issues,
		})
	}

	snap := store.Snapshot{
		SourceID:      sourceID,
		WeightClass:   wt.WeightClass,
		Season:        season,
		PublishedDate: publishedDate,
		CapturedAt:    captured,
	}
	entries := toIntermatEntries(rows)

	_, created, err := store.IngestEdition(ctx, db, snap, entries)
	if err != nil {
		return fmt.Errorf("ingest edition: %w", err)
	}
	if created {
		res.SnapshotsCreated++
		res.EntriesCreated += len(entries)
	} else {
		res.SnapshotsSkipped++
	}
	return nil
}

// intermatRowRanks extracts published ranks for detectAnomalies (source-
// agnostic — see anomaly.go).
func intermatRowRanks(rows []intermat.Row) []int {
	ranks := make([]int, len(rows))
	for i, r := range rows {
		ranks[i] = r.Rank
	}
	return ranks
}

// toIntermatEntries maps parsed rows to unresolved ranking entries.
// RawSourceString is the published name, matching Flo's shape exactly.
// Conference and Record — columns FloWrestling's schema never needed — get
// their own columns (raw_conference/raw_record, db/migrations/0005) rather
// than being folded into the name string; an earlier version of this
// function folded them in as a display-breaking workaround before those
// columns existed (docs/decisions.md).
func toIntermatEntries(rows []intermat.Row) []store.RankingEntry {
	out := make([]store.RankingEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, store.RankingEntry{
			Rank:            r.Rank,
			RawSourceString: r.Name,
			RawSchool:       optional(r.School),
			RawGrade:        optional(r.Grade),
			RawConference:   optional(r.Conference),
			RawRecord:       optional(r.Record),
		})
	}
	return out
}
