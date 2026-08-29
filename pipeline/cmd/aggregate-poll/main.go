// Command aggregate-poll turns current Fan Poll ballots into a published
// ranking, one snapshot per weight class, under the seeded "Fan Poll" source.
// Meant to run on the same cadence as the rest of the pipeline (weekly).
package main

import (
	"context"
	"flag"
	"log"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"pipeline/internal/aggregate"
	"pipeline/internal/store"
)

func main() {
	dbURL := flag.String("db", "", "Postgres connection string (default: $DATABASE_URL)")
	season := flag.Int("season", 0, "ballot season to aggregate (0 = auto-detect the newest season with ballots)")
	minBallots := flag.Int("min-ballots", 3, "minimum distinct ballots required to publish a weight class")
	flag.Parse()

	if err := run(store.ResolveDBURL(*dbURL), *season, *minBallots); err != nil {
		log.Fatalf("aggregate-poll: %v", err)
	}
}

func run(dbURL string, season, minBallots int) error {
	ctx := context.Background()

	db, err := store.Open(dbURL)
	if err != nil {
		return err
	}
	defer db.Close()

	if season == 0 {
		detected, err := aggregate.CurrentBallotSeason(ctx, db)
		if err != nil {
			return err
		}
		if detected == nil {
			log.Println("no ballots exist yet; nothing to aggregate")
			return nil
		}
		season = *detected
	}

	res, err := aggregate.Run(ctx, db, season, minBallots, time.Now())
	if err != nil {
		return err
	}

	log.Printf("season %d: published %d weight(s) (%v), %d entries created",
		season, len(res.WeightsPublished), res.WeightsPublished, res.EntriesCreated)
	for _, skip := range res.WeightsSkipped {
		log.Printf("SKIPPED (below threshold): %s", skip)
	}
	return nil
}
