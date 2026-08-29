// Command roster pulls every D1 team's current roster from WrestleStat
// (docs/sources/wrestlestat.md) and ingests it: one fetch for the team list,
// then one fetch per team's profile page (unlike cmd/scrape's single-page
// FloWrestling pull — WrestleStat has no combined-season container). A
// per-team failure (fetch or parse) is isolated so one bad team never blocks
// the rest of the run, same isolation philosophy as cmd/scrape's per-edition
// handling.
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
	"pipeline/internal/scraper/wrestlestat"
	"pipeline/internal/store"
)

const (
	userAgent    = "collegiate-wrestling-rankings-board/0.1 (+contact: sammy.cokeley@gmail.com)"
	teamListURL  = "https://www.wrestlestat.com/d1/team/select"
	teamProfileF = "https://www.wrestlestat.com/team/%d/school/profile"
)

func main() {
	dbURL := flag.String("db", "", "Postgres connection string (default: $DATABASE_URL)")
	season := flag.Int("season", 0, "season ending year, e.g. 2027 (required — no title to infer from, unlike cmd/scrape)")
	delay := flag.Duration("delay", 500*time.Millisecond, "polite delay between per-team requests")
	flag.Parse()

	if *season == 0 {
		log.Fatal("roster: -season is required")
	}

	if err := run(store.ResolveDBURL(*dbURL), *season, *delay); err != nil {
		log.Fatalf("roster: %v", err)
	}
}

func run(dbURL string, season int, delay time.Duration) error {
	ctx := context.Background()

	db, err := store.Open(dbURL)
	if err != nil {
		return err
	}
	defer db.Close()

	teamListPage, err := fetch(ctx, teamListURL)
	if err != nil {
		return fmt.Errorf("fetch team list: %w", err)
	}
	teams, err := wrestlestat.ParseTeamList(teamListPage)
	if err != nil {
		return fmt.Errorf("parse team list: %w", err)
	}
	log.Printf("found %d D1 teams", len(teams))

	var (
		entriesUpserted, placeholdersSkipped int
		failures                             []string
	)
	for i, team := range teams {
		if i > 0 {
			time.Sleep(delay)
		}

		profileURL := fmt.Sprintf(teamProfileF, team.ID)
		page, err := fetch(ctx, profileURL)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s (id %d): fetch: %v", team.Name, team.ID, err))
			continue
		}

		rows, err := wrestlestat.ParseRoster(page)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s (id %d): parse: %v", team.Name, team.ID, err))
			continue
		}

		res, err := ingest.RosterForTeam(ctx, db, team.Name, season, rows, time.Now())
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s (id %d): ingest: %v", team.Name, team.ID, err))
			continue
		}
		entriesUpserted += res.EntriesUpserted
		placeholdersSkipped += res.PlaceholdersSkipped
	}

	log.Printf("ingested %d teams: %d roster entries upserted, %d placeholder slots skipped",
		len(teams)-len(failures), entriesUpserted, placeholdersSkipped)

	if len(failures) > 0 {
		for _, f := range failures {
			log.Printf("FAILED team (needs manual handling): %s", f)
		}
		return fmt.Errorf("%d team(s) failed to ingest", len(failures))
	}
	return nil
}

// fetch does a single polite GET (real UA, timeout) for a page's HTML. The
// standard client follows redirects automatically — the team profile URL
// 301s from /team/{id}/school/profile to the slugged /team/{id}/{slug}/profile
// (docs/sources/wrestlestat.md), so this never needs to know the slug.
func fetch(ctx context.Context, url string) ([]byte, error) {
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
