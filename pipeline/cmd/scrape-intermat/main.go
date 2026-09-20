// Command scrape-intermat fetches InterMat's current NCAA DI rankings
// record and ingests it — the in-season counterpart to cmd/backfill-intermat
// (which recovers past seasons from the Wayback Machine; InterMat itself
// keeps no history, so this only ever sees whatever is live right now).
//
// The record page carries no date of its own, so the current record and its
// precise publish date are both discovered via the RSS feed (the announcing
// article's title date, cross-referenced with its embedded record link —
// docs/sources/intermat.md). Meant to run on a schedule matching InterMat's
// own weekly cadence; safe to re-run (store.IngestEdition is idempotent).
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"pipeline/internal/ingest"
	"pipeline/internal/scraper/intermat"
	"pipeline/internal/store"
)

const userAgent = "collegiate-wrestling-rankings-board/0.1 (+contact: sammy.cokeley@gmail.com)"

func main() {
	dbURL := flag.String("db", "", "Postgres connection string (default: $DATABASE_URL)")
	flag.Parse()

	if err := run(store.ResolveDBURL(*dbURL)); err != nil {
		log.Fatalf("scrape-intermat: %v", err)
	}
}

func run(dbURL string) error {
	ctx := context.Background()

	db, err := store.Open(dbURL)
	if err != nil {
		return err
	}
	defer db.Close()

	client := &http.Client{Timeout: 30 * time.Second}

	edition, err := intermat.FetchLatestEdition(ctx, client, userAgent)
	if err != nil {
		return fmt.Errorf("fetch latest edition: %w", err)
	}
	season, err := intermat.SeasonFromDate(edition.PublishedDate)
	if err != nil {
		return fmt.Errorf("derive season: %w", err)
	}
	log.Printf("latest edition: %s (published %s, season %d)", edition.RecordURL, edition.PublishedDate, season)

	page, err := fetch(ctx, client, edition.RecordURL)
	if err != nil {
		return fmt.Errorf("fetch record: %w", err)
	}
	weights, err := intermat.ParseRecord(page)
	if err != nil {
		return fmt.Errorf("parse record: %w", err)
	}

	rec := intermat.Record{PublishedDate: edition.PublishedDate, Weights: weights}
	res, err := ingest.IntermatRecord(ctx, db, rec, season, time.Now())
	if err != nil {
		return fmt.Errorf("ingest: %w", err)
	}
	log.Printf("ingested %s: %d snapshots created, %d entries created, %d already present",
		edition.RecordURL, res.SnapshotsCreated, res.EntriesCreated, res.SnapshotsSkipped)
	for _, f := range res.Failures {
		log.Printf("WEIGHT FAILED (needs manual handling): %s", f)
	}
	for _, a := range res.Anomalies {
		log.Printf("anomaly: %s", a)
	}
	if len(res.Failures) > 0 {
		return fmt.Errorf("%d weight(s) failed to ingest", len(res.Failures))
	}
	return nil
}

// fetch does a single polite GET (real UA, timeout) for a page's HTML.
func fetch(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %q: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %q: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
