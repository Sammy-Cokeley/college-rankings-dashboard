// Command resolve runs the second-pass entity resolution over ingested entries,
// reconciling FloWrestling's raw name+school strings to canonical wrestlers.
// Safe to re-run: already-resolved entries are skipped.
package main

import (
	"context"
	"flag"
	"log"

	_ "modernc.org/sqlite"

	"pipeline/internal/resolve"
	"pipeline/internal/store"
)

const sourceName = "FloWrestling"

func main() {
	dbPath := flag.String("db", "rankings.db", "path to the SQLite database file")
	flag.Parse()

	db, err := store.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	res, err := resolve.Source(context.Background(), db, sourceName)
	if err != nil {
		log.Fatalf("resolve: %v", err)
	}
	log.Printf("resolved %d entries: %d wrestlers created, %d aliases recorded",
		res.EntriesResolved, res.WrestlersCreated, res.AliasesCreated)
}
