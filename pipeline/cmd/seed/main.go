// Command seed applies the db/seed/*.sql files to a Postgres database. Seed
// files are idempotent, so this is safe to run repeatedly.
package main

import (
	"context"
	"flag"
	"log"

	_ "github.com/jackc/pgx/v5/stdlib"

	"pipeline/internal/store"
)

func main() {
	dbURL := flag.String("db", "", "Postgres connection string (default: $DATABASE_URL)")
	dir := flag.String("dir", "../db/seed", "directory of *.sql seed files")
	flag.Parse()

	db, err := store.OpenForMigrations(store.ResolveDBURL(*dbURL))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := store.ApplySeeds(context.Background(), db, *dir); err != nil {
		log.Fatalf("seed: %v", err)
	}
	log.Printf("seeds applied from %s", *dir)
}
