// Command scrape ingests a FloWrestling season ranking container into the
// database. By default it reads the committed offline fixture (so `make scrape`
// works with no network); pass -url to fetch a live page, or -page to read a
// saved page HTML file.
//
// The season is manual-per-season config (decisions.md): pass -season, or let
// it be inferred from the container title. The container id only matters for
// -url, where it forms part of the page URL.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	_ "modernc.org/sqlite"

	"pipeline/internal/ingest"
	"pipeline/internal/scraper"
	"pipeline/internal/store"
)

const userAgent = "collegiate-wrestling-rankings-board/0.1 (+contact: sammy.cokeley@gmail.com)"

func main() {
	dbPath := flag.String("db", "rankings.db", "path to the SQLite database file")
	fixture := flag.String("fixture", "internal/scraper/testdata/ranking_container_14300895.json",
		"decoded container JSON to ingest (default offline path)")
	page := flag.String("page", "", "saved page HTML file to decode instead of the fixture")
	url := flag.String("url", "", "live FloWrestling rankings page URL to fetch and decode")
	season := flag.Int("season", 0, "season ending year, e.g. 2026 (0 = infer from container title)")
	flag.Parse()

	if err := run(*dbPath, *fixture, *page, *url, *season); err != nil {
		log.Fatalf("scrape: %v", err)
	}
}

func run(dbPath, fixture, page, url string, season int) error {
	ctx := context.Background()

	container, err := loadContainer(ctx, fixture, page, url)
	if err != nil {
		return err
	}

	if season == 0 {
		inferred, ok := ingest.SeasonFromTitle(container.Title)
		if !ok {
			return fmt.Errorf("could not infer season from title %q; pass -season", container.Title)
		}
		season = inferred
	}

	db, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	res, err := ingest.Container(ctx, db, container, season, time.Now())
	if err != nil {
		return err
	}
	log.Printf("ingested season %d from %q: %d snapshots created, %d skipped, %d entries",
		season, container.Title, res.SnapshotsCreated, res.SnapshotsSkipped, res.EntriesCreated)

	// Per-edition failures (e.g. a malformed table) don't abort the run, but
	// they must be surfaced loudly for manual handling — and exit non-zero so a
	// cron run is flagged rather than passing silently.
	if len(res.Failures) > 0 {
		for _, f := range res.Failures {
			log.Printf("FAILED edition (needs manual handling): %s", f)
		}
		return fmt.Errorf("%d edition(s) failed to ingest", len(res.Failures))
	}
	return nil
}

// loadContainer resolves the input source in priority order: live url, saved
// page file, then the decoded fixture.
func loadContainer(ctx context.Context, fixture, page, url string) (scraper.Container, error) {
	switch {
	case url != "":
		html, err := fetchPage(ctx, url)
		if err != nil {
			return scraper.Container{}, err
		}
		return scraper.DecodeAppState(html)
	case page != "":
		html, err := os.ReadFile(page)
		if err != nil {
			return scraper.Container{}, fmt.Errorf("read page %q: %w", page, err)
		}
		return scraper.DecodeAppState(html)
	default:
		data, err := os.ReadFile(fixture)
		if err != nil {
			return scraper.Container{}, fmt.Errorf("read fixture %q: %w", fixture, err)
		}
		return scraper.ParseContainer(data)
	}
}

// fetchPage does a single polite GET (real UA, timeout) for the SSR page HTML.
func fetchPage(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %q: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %q: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
