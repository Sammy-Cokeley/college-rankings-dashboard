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

// ballotPicks reads each contributor's MOST RECENT ballot_submissions row
// for this weight+season (not the live, still-rolling ballots/ballot_entries
// table) — a user who submitted in an earlier week but hasn't resubmitted
// since still counts (carry-over, decided with the user), and an edit made
// to their currently-open ballot after this week's aggregation already ran
// never retroactively changes a published week. ROW_NUMBER, not MAX(id) or
// similar, so the tie is broken consistently even if submitted_at were ever
// equal (shouldn't happen — timestamps are set server-side per request —
// but id DESC as the tiebreak costs nothing and removes the ambiguity).
func ballotPicks(ctx context.Context, db *sql.DB, weight, season int) ([]BallotPick, error) {
	rows, err := db.QueryContext(ctx, `
WITH latest AS (
  SELECT id, user_id,
         ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY submitted_at DESC, id DESC) AS rn
  FROM ballot_submissions
  WHERE weight_class = $1 AND season = $2
)
SELECT bse.submission_id, bse.rank, bse.wrestler_id, w.full_name
FROM ballot_submission_entries bse
JOIN latest l ON l.id = bse.submission_id AND l.rn = 1
JOIN wrestlers w ON w.id = bse.wrestler_id`, weight, season)
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

// CurrentBallotSeason returns the newest season with any ballot SUBMISSION
// (not just a live, still-in-progress draft — a draft with no Submit yet
// contributes nothing to any published week, so it shouldn't drive which
// season a run picks either).
func CurrentBallotSeason(ctx context.Context, db *sql.DB) (*int, error) {
	var season sql.NullInt64
	if err := db.QueryRowContext(ctx, `SELECT MAX(season) FROM ballot_submissions`).Scan(&season); err != nil {
		return nil, fmt.Errorf("current ballot season: %w", err)
	}
	if !season.Valid {
		return nil, nil
	}
	s := int(season.Int64)
	return &s, nil
}
