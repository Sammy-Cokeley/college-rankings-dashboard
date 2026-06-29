// Command migrate applies the db/migrations/*.sql files to a SQLite database.
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
	dir := flag.String("dir", "../db/migrations", "directory of *.sql migration files")
	flag.Parse()

	db, err := store.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := store.ApplyMigrations(context.Background(), db, *dir); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Printf("migrations applied from %s to %s", *dir, *dbPath)
}
