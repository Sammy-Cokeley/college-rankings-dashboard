// Command migrate applies the db/migrations/*.sql files to a Postgres database.
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
	dir := flag.String("dir", "../db/migrations", "directory of *.sql migration files")
	flag.Parse()

	db, err := store.OpenForMigrations(store.ResolveDBURL(*dbURL))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := store.ApplyMigrations(context.Background(), db, *dir); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Printf("migrations applied from %s", *dir)
}
