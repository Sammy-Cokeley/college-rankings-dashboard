package intermat

import (
	"fmt"
	"time"
)

// ResolveDate derives a snapshot's published_date from a Wayback capture
// timestamp (e.g. "20260106175039" -> "2026-01-06") — the date portion of the
// EARLIEST 200-status capture for a record id (see DedupeByRecordID). A
// simple, deliberate v0 tiebreak: cross-referencing the weekly article's
// title date would pin the true publish date more precisely (InterMat's
// weekly cadence means a capture can lag the actual update by a day or two),
// but that refinement is deferred — see docs/sources/intermat.md open
// question #2. This value feeds snapshots' UNIQUE(source_id, weight_class,
// season, published_date), so it directly controls backfill idempotency.
func ResolveDate(timestamp string) (string, error) {
	t, err := time.Parse("20060102150405", timestamp)
	if err != nil {
		return "", fmt.Errorf("parse wayback timestamp %q: %w", timestamp, err)
	}
	return t.Format("2006-01-02"), nil
}

// SeasonFromDate derives the season ending-year from a published_date
// (YYYY-MM-DD), matching this project's Aug-Jul academic-year convention
// (same rule ingest.SeasonFromTitle applies for Flo, just from a real date
// instead of a container title string): August or later belongs to the
// season ending the FOLLOWING year (e.g. 2026-09-17 -> season 2027);
// January-July belongs to the season ending that same year.
func SeasonFromDate(date string) (int, error) {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return 0, fmt.Errorf("parse date %q: %w", date, err)
	}
	if t.Month() >= time.August {
		return t.Year() + 1, nil
	}
	return t.Year(), nil
}
