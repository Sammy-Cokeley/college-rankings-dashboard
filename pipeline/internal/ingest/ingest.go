// Package ingest maps a decoded FloWrestling Container into the store: each
// weight edition becomes a snapshot and each ranked row a ranking_entry, with
// wrestler_id left NULL for the later resolution pass. Re-runs are idempotent
// (see store.IngestEdition).
package ingest

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"pipeline/internal/scraper"
	"pipeline/internal/store"
)

// sourceName is the seeded source these editions belong to.
const sourceName = "FloWrestling"

// Result summarizes one ingestion run.
type Result struct {
	SnapshotsCreated int // editions newly written this run
	SnapshotsSkipped int // editions already present (idempotent re-run)
	EntriesCreated   int
	Failures         []EditionFailure // editions that could not be ingested (fatal)
	Anomalies        []EditionAnomaly // editions that ingested but look off (non-fatal)
}

// EditionFailure records an edition that failed to ingest (e.g. a malformed
// table: a ragged row, a missing required column, or a non-numeric rank).
// Failures are isolated per edition so one bad edition never blocks the rest of
// the run; the caller surfaces them for manual handling. (A tie rank is NOT a
// failure — the schema stores ties verbatim; see schema.md §7.)
type EditionFailure struct {
	WeightClass   int
	PublishedDate string
	Err           error
}

func (f EditionFailure) String() string {
	return fmt.Sprintf("weight %d %s: %v", f.WeightClass, f.PublishedDate, f.Err)
}

// Container ingests every weight edition in c for the given season. capturedAt
// is stamped on each newly created snapshot. The season is supplied by the
// caller (the container title carries "2025-26" but the ending-year integer is
// an explicit per-run input — see cmd/scrape).
//
// A per-edition failure (malformed table) is isolated: the edition is recorded
// in Result.Failures and the run continues, so one bad edition never blocks the
// other weights/weeks. The returned error is reserved for systemic failures
// (e.g. the source isn't seeded) that abort the whole run.
func Container(ctx context.Context, db *sql.DB, c scraper.Container, season int, capturedAt time.Time) (Result, error) {
	sourceID, err := store.SourceID(ctx, db, sourceName)
	if err != nil {
		return Result{}, err
	}
	captured := capturedAt.UTC().Format(time.RFC3339)

	var res Result
	for _, we := range c.WeightEditions() {
		if err := ingestOne(ctx, db, sourceID, we, season, captured, &res); err != nil {
			res.Failures = append(res.Failures, EditionFailure{
				WeightClass:   we.WeightClass,
				PublishedDate: we.Edition.PublishDate,
				Err:           err,
			})
		}
	}
	return res, nil
}

// ingestOne parses and ingests a single edition, updating res on success.
func ingestOne(ctx context.Context, db *sql.DB, sourceID int64, we scraper.WeightEdition, season int, captured string, res *Result) error {
	rows, err := scraper.ParseTable(we.Edition.Content)
	if err != nil {
		return fmt.Errorf("parse table: %w", err)
	}

	// Non-fatal: surface rank oddities (ties/gaps) for eyeballing without
	// blocking ingestion. Detected on every run, including idempotent skips.
	if issues := detectAnomalies(rows); len(issues) > 0 {
		res.Anomalies = append(res.Anomalies, EditionAnomaly{
			WeightClass:   we.WeightClass,
			PublishedDate: we.Edition.PublishDate,
			Issues:        issues,
		})
	}

	snap := store.Snapshot{
		SourceID:      sourceID,
		WeightClass:   we.WeightClass,
		Season:        season,
		PublishedDate: we.Edition.PublishDate,
		CapturedAt:    captured,
	}
	entries := toEntries(rows)

	_, created, err := store.IngestEdition(ctx, db, snap, entries)
	if err != nil {
		return fmt.Errorf("ingest edition: %w", err)
	}
	if created {
		res.SnapshotsCreated++
		res.EntriesCreated += len(entries)
	} else {
		res.SnapshotsSkipped++
	}
	return nil
}

// toEntries maps parsed rows to unresolved ranking entries. raw_source_string
// is the published name; school and grade are kept in their own point-in-time
// columns, NULL when a row omits them. wrestler_id stays nil until resolution.
func toEntries(rows []scraper.Row) []store.RankingEntry {
	out := make([]store.RankingEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, store.RankingEntry{
			Rank:            r.Rank,
			RawSourceString: r.Name,
			RawSchool:       optional(r.School),
			RawGrade:        optional(r.Grade),
		})
	}
	return out
}

// optional returns a pointer to s, or nil when s is empty, so empty cells land
// as SQL NULL rather than "".
func optional(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}

// SeasonFromTitle extracts the ending-year season from a container title like
// "2025-26 NCAA DI Wrestling Rankings" -> 2026. It is a convenience for the CLI;
// callers may always override with an explicit value.
func SeasonFromTitle(title string) (int, bool) {
	// Find the first "YYYY-YY" token.
	fields := strings.FieldsFunc(title, func(r rune) bool { return r == ' ' })
	for _, f := range fields {
		start, end, ok := strings.Cut(f, "-")
		if !ok || len(start) != 4 || len(end) != 2 {
			continue
		}
		startYear, err := strconv.Atoi(start)
		if err != nil {
			continue
		}
		// "2025-26" -> ending year 2026: century of start + the two-digit end.
		endingYear := (startYear/100)*100 + atoiOr(end, -1)
		if endingYear <= startYear {
			endingYear += 100
		}
		return endingYear, true
	}
	return 0, false
}

func atoiOr(s string, fallback int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}
