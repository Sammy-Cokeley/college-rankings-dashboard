// Command seed applies the db/seed/*.sql files to a SQLite database. Seed files
// are idempotent, so this is safe to run repeatedly.
package main

import (
	"context"
	"flag"
	"log"

	_ "modernc.org/sqlite"

	"pipeline/internal/store"
)

func main() {
	dbPath := flag.String("db", "rankings.db", "path to the SQLite database file")
	dir := flag.String("dir", "../db/seed", "directory of *.sql seed files")
	flag.Parse()

	db, err := store.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := store.ApplySeeds(context.Background(), db, *dir); err != nil {
		log.Fatalf("seed: %v", err)
	}
	log.Printf("seeds applied from %s to %s", *dir, *dbPath)
}
