package aggregate

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"pipeline/internal/store"
)

const sourceName = "Fan Poll"

// WeightSkip records a weight class that had ballots but not enough to
// publish (below minBallots) — not a failure, just not yet a real signal.
type WeightSkip struct {
	WeightClass int
	BallotCount int
	MinRequired int
}

func (s WeightSkip) String() string {
	return fmt.Sprintf("weight %d: %d ballot(s), need %d to publish", s.WeightClass, s.BallotCount, s.MinRequired)
}

// Result summarizes one aggregation run.
type Result struct {
	WeightsPublished []int
	WeightsSkipped   []WeightSkip
	EntriesCreated   int
}

// Run aggregates every weight class's current ballots for season into a
// published Fan Poll snapshot, one per weight, skipping any weight with
// fewer than minBallots distinct contributing ballots. Idempotent per
// (weight, published date) via store.IngestEdition — re-running the same day
// updates nothing, matching the ranking sources' own idempotency.
func Run(ctx context.Context, db *sql.DB, season, minBallots int, now time.Time) (Result, error) {
	sourceID, err := store.SourceID(ctx, db, sourceName)
	if err != nil {
		return Result{}, fmt.Errorf("Fan Poll source not seeded: %w", err)
	}

	published := now.UTC().Format("2006-01-02")
	captured := now.UTC().Format(time.RFC3339)

	var res Result
	for _, weight := range WeightClasses {
		picks, err := ballotPicks(ctx, db, weight, season)
		if err != nil {
			return Result{}, fmt.Errorf("weight %d: %w", weight, err)
		}

		if n := DistinctBallots(picks); n < minBallots {
			if n > 0 {
				res.WeightsSkipped = append(res.WeightsSkipped, WeightSkip{
					WeightClass: weight, BallotCount: n, MinRequired: minBallots,
				})
			}
			continue
		}

		scored := ScoreWeight(picks)
		entries := make([]store.RankingEntry, 0, len(scored))
		for _, s := range scored {
			wid := s.WrestlerID
			entries = append(entries, store.RankingEntry{
				WrestlerID:      &wid,
				Rank:            s.Rank,
				RawSourceString: s.FullName,
			})
		}

		_, _, err = store.IngestEdition(ctx, db, store.Snapshot{
			SourceID:      sourceID,
			WeightClass:   weight,
			Season:        season,
			PublishedDate: published,
			CapturedAt:    captured,
		}, entries)
		if err != nil {
			return Result{}, fmt.Errorf("weight %d: ingest: %w", weight, err)
		}
		res.WeightsPublished = append(res.WeightsPublished, weight)
		res.EntriesCreated += len(entries)
	}
	return res, nil
}

func ballotPicks(ctx context.Context, db *sql.DB, weight, season int) ([]BallotPick, error) {
	rows, err := db.QueryContext(ctx, `
SELECT be.ballot_id, be.rank, be.wrestler_id, w.full_name
FROM ballot_entries be
JOIN ballots b ON b.id = be.ballot_id
JOIN wrestlers w ON w.id = be.wrestler_id
WHERE b.weight_class = $1 AND b.season = $2`, weight, season)
	if err != nil {
		return nil, fmt.Errorf("query ballot picks: %w", err)
	}
	defer rows.Close()

	var out []BallotPick
	for rows.Next() {
		var p BallotPick
		if err := rows.Scan(&p.BallotID, &p.Rank, &p.WrestlerID, &p.FullName); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CurrentBallotSeason returns the newest season with any ballot data, or nil
// if no ballots exist yet at all.
func CurrentBallotSeason(ctx context.Context, db *sql.DB) (*int, error) {
	var season sql.NullInt64
	if err := db.QueryRowContext(ctx, `SELECT MAX(season) FROM ballots`).Scan(&season); err != nil {
		return nil, fmt.Errorf("current ballot season: %w", err)
	}
	if !season.Valid {
		return nil, nil
	}
	s := int(season.Int64)
	return &s, nil
}
