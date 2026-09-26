// Command backfill-intermat recovers past InterMat NCAA DI rankings records
// from the Wayback Machine (InterMat itself keeps no history — each weekly
// "rankings record" page is replaced and the old one hard-404s, so this is
// the only way to get last season's data — docs/sources/intermat.md).
//
// Shape mirrors cmd/roster (many fetches, per-item delay, per-item failure
// isolation) more than cmd/scrape's single-fetch pull: FetchCDX enumerates
// every weekly record's Wayback snapshot, then each is fetched, parsed, and
// ingested independently, so one bad/missing snapshot never blocks the rest
// of the season.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"pipeline/internal/ingest"
	"pipeline/internal/scraper/intermat"
	"pipeline/internal/store"
)

const (
	userAgent = "collegiate-wrestling-rankings-board/0.1 (+contact: sammy.cokeley@gmail.com)"
	// "-r" (not just "ncaa-di*") is required: as a CDX prefix match,
	// "ncaa-di*" also matches "ncaa-dii-*" and "ncaa-diii-*" — confirmed live
	// (see intermat.CDXQuery's doc comment).
	cdxURLPattern = "intermatwrestle.com/rankings.html/ncaa-di-r*"
)

func main() {
	dbURL := flag.String("db", "", "Postgres connection string (default: $DATABASE_URL)")
	season := flag.Int("season", 0, "season ending year, e.g. 2026 (required — a record page carries no season of its own)")
	delay := flag.Duration("delay", 2*time.Second, "polite delay between archive.org requests (NOT InterMat's own Crawl-Delay:20 — backfill never fetches InterMat itself, only Wayback snapshots)")
	recoverURL := flag.String("recover-url", "", "fetch and ingest exactly ONE record page directly, bypassing CDX — for a Wayback gap that's still live on intermatwrestle.com (e.g. a not-yet-replaced postseason-final edition Wayback never captured). Requires -recover-date. Mutually exclusive with -recover-file.")
	recoverFile := flag.String("recover-file", "", "ingest exactly ONE already-saved record page from a local HTML file instead of fetching — for a gap recovered earlier (e.g. the live page was saved before InterMat replaced it) and no longer fetchable at all. Requires -recover-date. Mutually exclusive with -recover-url.")
	recoverDate := flag.String("recover-date", "", "YYYY-MM-DD published date for -recover-url/-recover-file — required with either; the page itself carries no reliable date, so this is an operator judgment call, not derived (see docs/sources/intermat.md for how the date was picked in practice)")
	flag.Parse()

	if *season == 0 {
		log.Fatal("backfill-intermat: -season is required")
	}
	if *recoverURL != "" && *recoverFile != "" {
		log.Fatal("backfill-intermat: -recover-url and -recover-file are mutually exclusive")
	}
	if (*recoverURL != "" || *recoverFile != "") && *recoverDate == "" {
		log.Fatal("backfill-intermat: -recover-date is required with -recover-url or -recover-file")
	}

	var err error
	switch {
	case *recoverFile != "":
		err = runRecoverFile(store.ResolveDBURL(*dbURL), *season, *recoverFile, *recoverDate)
	case *recoverURL != "":
		err = runRecover(store.ResolveDBURL(*dbURL), *season, *recoverURL, *recoverDate)
	default:
		err = run(store.ResolveDBURL(*dbURL), *season, *delay)
	}
	if err != nil {
		log.Fatalf("backfill-intermat: %v", err)
	}
}

func run(dbURL string, season int, delay time.Duration) error {
	ctx := context.Background()

	db, err := store.Open(dbURL)
	if err != nil {
		return err
	}
	defer db.Close()

	client := &http.Client{Timeout: 30 * time.Second}

	from, to := seasonDateRange(season)
	query := intermat.CDXQuery{URLPattern: cdxURLPattern, From: from, To: to}
	snapshots, err := intermat.FetchCDX(ctx, client, userAgent, query)
	if err != nil {
		return fmt.Errorf("fetch CDX: %w", err)
	}
	snapshots = intermat.DedupeByRecordID(snapshots)
	log.Printf("found %d distinct InterMat rankings records on the Wayback Machine", len(snapshots))

	var (
		snapshotsCreated, entriesCreated int
		failures                         []string
	)
	for i, snap := range snapshots {
		if i > 0 {
			time.Sleep(delay)
		}

		publishedDate, err := intermat.ResolveDate(snap.Timestamp)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: resolve date: %v", snap.OriginalURL, err))
			continue
		}

		// Fetch via the Wayback replay URL, but attribute to the real,
		// original intermatwrestle.com URL (snap.OriginalURL) — those two
		// differ for every backfilled edition; readers should land on the
		// live site, not an archive.org mirror.
		res, err := fetchParseIngest(ctx, db, client, intermat.SnapshotURL(snap), snap.OriginalURL, publishedDate, season)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", snap.OriginalURL, err))
			continue
		}
		snapshotsCreated += res.SnapshotsCreated
		entriesCreated += res.EntriesCreated
		logPerWeightIssues(snap.OriginalURL, res)
	}

	log.Printf("processed %d records: %d snapshots created, %d entries created",
		len(snapshots)-len(failures), snapshotsCreated, entriesCreated)

	if len(failures) > 0 {
		for _, f := range failures {
			log.Printf("FAILED record (needs manual handling): %s", f)
		}
		return fmt.Errorf("%d record(s) failed to ingest", len(failures))
	}
	return nil
}

// runRecover ingests exactly one record page fetched directly (not through
// CDX) — the escape hatch for a Wayback gap that turns out to still be live
// (see -recover-url's flag description). No delay/looping: this is a
// single, deliberate, operator-initiated fetch.
func runRecover(dbURL string, season int, url, publishedDate string) error {
	ctx := context.Background()

	db, err := store.Open(dbURL)
	if err != nil {
		return err
	}
	defer db.Close()

	client := &http.Client{Timeout: 30 * time.Second}

	log.Printf("recovering %s as published_date=%s (operator-supplied, not derived)", url, publishedDate)
	res, err := fetchParseIngest(ctx, db, client, url, url, publishedDate, season)
	if err != nil {
		return fmt.Errorf("%s: %w", url, err)
	}
	log.Printf("recovered %s: %d snapshots created, %d entries created", url, res.SnapshotsCreated, res.EntriesCreated)
	logPerWeightIssues(url, res)
	if len(res.Failures) > 0 {
		return fmt.Errorf("%d weight(s) failed to ingest", len(res.Failures))
	}
	return nil
}

// runRecoverFile ingests exactly one record page already saved to a local
// HTML file — the escape hatch for a gap that was recovered once (e.g. a
// live page fetched and saved before InterMat replaced it) and is no longer
// fetchable at all, live or via Wayback, by the time it's actually ingested.
func runRecoverFile(dbURL string, season int, path, publishedDate string) error {
	ctx := context.Background()

	db, err := store.Open(dbURL)
	if err != nil {
		return err
	}
	defer db.Close()

	page, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	log.Printf("recovering %s (local file) as published_date=%s (operator-supplied, not derived)", path, publishedDate)
	// No live URL exists for a page recovered from a local file — the whole
	// point of this path is that it's no longer fetchable anywhere. Empty,
	// not a guess: ingest.IntermatRecord/optional() turns "" into NULL.
	res, err := parseAndIngest(ctx, db, page, "", publishedDate, season)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	log.Printf("recovered %s: %d snapshots created, %d entries created", path, res.SnapshotsCreated, res.EntriesCreated)
	logPerWeightIssues(path, res)
	if len(res.Failures) > 0 {
		return fmt.Errorf("%d weight(s) failed to ingest", len(res.Failures))
	}
	return nil
}

// fetchParseIngest fetches one record page over HTTP (from fetchURL), then
// parses and ingests it, attributed to sourceURL — the single-record unit
// both run and runRecover repeat. fetchURL and sourceURL differ for a
// Wayback-backfilled edition (replay URL vs. the real original page) and
// are identical for a direct recovery fetch.
func fetchParseIngest(ctx context.Context, db *sql.DB, client *http.Client, fetchURL, sourceURL, publishedDate string, season int) (ingest.Result, error) {
	page, err := fetch(ctx, client, fetchURL)
	if err != nil {
		return ingest.Result{}, fmt.Errorf("fetch: %w", err)
	}
	return parseAndIngest(ctx, db, page, sourceURL, publishedDate, season)
}

// parseAndIngest is the fetch-source-agnostic core shared by the live-fetch
// and local-file recovery paths — parsing itself never cared whether the
// bytes came from Wayback, a live fetch, or disk (intermat.ParseRecord's own
// design goal). sourceURL is the attribution link, empty when genuinely
// unknown (a local-file recovery with no live URL left).
func parseAndIngest(ctx context.Context, db *sql.DB, page []byte, sourceURL, publishedDate string, season int) (ingest.Result, error) {
	weights, err := intermat.ParseRecord(page)
	if err != nil {
		return ingest.Result{}, fmt.Errorf("parse: %w", err)
	}
	rec := intermat.Record{PublishedDate: publishedDate, Weights: weights}
	res, err := ingest.IntermatRecord(ctx, db, rec, season, time.Now(), sourceURL)
	if err != nil {
		return ingest.Result{}, fmt.Errorf("ingest: %w", err)
	}
	return res, nil
}

// logPerWeightIssues logs per-weight failures/anomalies within an
// otherwise-successful record — non-fatal to the overall run (same
// isolation ingest.IntermatRecord already provides).
func logPerWeightIssues(sourceURL string, res ingest.Result) {
	for _, f := range res.Failures {
		log.Printf("WEIGHT FAILED (needs manual handling): %s %s", sourceURL, f)
	}
	for _, a := range res.Anomalies {
		log.Printf("anomaly: %s %s", sourceURL, a)
	}
}

// seasonDateRange bounds a CDX query to one NCAA season, generously: August 1
// of the season's starting year through July 31 of its ending year — wide
// enough to catch a preseason edition as early as late August and a
// postseason edition as late as June, without pulling in adjacent seasons
// (CDX has no season concept of its own — see CDXQuery's own doc comment on
// why this bound is required, not optional; an earlier unscoped run pulled
// records back to 2023 before this existed).
func seasonDateRange(season int) (from, to string) {
	return fmt.Sprintf("%d0801", season-1), fmt.Sprintf("%d0731", season)
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
