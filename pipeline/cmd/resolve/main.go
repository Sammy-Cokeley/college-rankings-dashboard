// Command resolve runs the second-pass entity resolution over ingested
// entries, reconciling raw name+school strings to canonical wrestlers. Safe
// to re-run: already-resolved entries are skipped.
//
// Always resolves FloWrestling's ranking entries. Pass -roster-season to also
// resolve WrestleStat's roster entries for that season — optional and
// separate because roster ingestion (cmd/roster) may not have run yet, and
// this command must stay usable without it.
package main

import (
	"context"
	"flag"
	"log"

	_ "github.com/jackc/pgx/v5/stdlib"

	"pipeline/internal/resolve"
	"pipeline/internal/store"
)

const (
	sourceName       = "FloWrestling"
	rosterSourceName = "WrestleStat"
)

func main() {
	dbURL := flag.String("db", "", "Postgres connection string (default: $DATABASE_URL)")
	rosterSeason := flag.Int("roster-season", 0, "also resolve WrestleStat roster entries for this season (0 = skip)")
	flag.Parse()

	db, err := store.Open(store.ResolveDBURL(*dbURL))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()

	res, err := resolve.Source(ctx, db, sourceName)
	if err != nil {
		log.Fatalf("resolve %s: %v", sourceName, err)
	}
	log.Printf("resolved %d %s entries: %d wrestlers created, %d aliases recorded",
		res.EntriesResolved, sourceName, res.WrestlersCreated, res.AliasesCreated)

	if *rosterSeason != 0 {
		rres, err := resolve.Roster(ctx, db, rosterSourceName, *rosterSeason)
		if err != nil {
			log.Fatalf("resolve roster: %v", err)
		}
		log.Printf("resolved %d roster entries (season %d): %d wrestlers created, %d aliases recorded",
			rres.EntriesResolved, *rosterSeason, rres.WrestlersCreated, rres.AliasesCreated)
	}
}
