package ingest

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pipeline/internal/scraper/intermat"
)

var (
	intermatMidSeasonFixture = filepath.Join("..", "scraper", "intermat", "testdata", "record_r63_2weights.html")
	intermatPreseasonFixture = filepath.Join("..", "scraper", "intermat", "testdata", "record_r53_2weights.html")
)

func loadIntermatRecord(t *testing.T, fixtureFile, publishedDate string) intermat.Record {
	t.Helper()
	data, err := os.ReadFile(fixtureFile)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	weights, err := intermat.ParseRecord(data)
	if err != nil {
		t.Fatalf("ParseRecord: %v", err)
	}
	return intermat.Record{PublishedDate: publishedDate, Weights: weights}
}

// The mid-season fixture (r63, 2026-01-06) has no data gaps: both weights
// must ingest cleanly.
func TestIntermatRecord_IngestsFixture(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	rec := loadIntermatRecord(t, intermatMidSeasonFixture, "2026-01-06")

	res, err := IntermatRecord(ctx, db, rec, 2026, time.Now())
	if err != nil {
		t.Fatalf("IntermatRecord: %v", err)
	}
	if res.SnapshotsCreated != 2 {
		t.Errorf("snapshots created = %d, want 2", res.SnapshotsCreated)
	}
	if res.EntriesCreated != 66 { // 33 rows x 2 weights
		t.Errorf("entries created = %d, want 66", res.EntriesCreated)
	}
	if len(res.Failures) != 0 {
		t.Errorf("failures = %v, want none", res.Failures)
	}

	if got := intermatCount(t, db, `SELECT COUNT(*) FROM snapshots`); got != 2 {
		t.Errorf("snapshots in db = %d, want 2", got)
	}
	if got := intermatCount(t, db, `SELECT COUNT(*) FROM ranking_entries`); got != 66 {
		t.Errorf("entries in db = %d, want 66", got)
	}
	if got := intermatCount(t, db, `SELECT COUNT(*) FROM ranking_entries WHERE wrestler_id IS NOT NULL`); got != 0 {
		t.Errorf("resolved entries after ingest = %d, want 0", got)
	}

	// Conference/Record land in their own columns (db/migrations/0005), not
	// folded into raw_source_string — the name stays clean, matching Flo's
	// shape, since the rankings table renders raw_source_string directly.
	var name, conference, record string
	if err := db.QueryRow(
		`SELECT raw_source_string, raw_conference, raw_record FROM ranking_entries WHERE rank = 1 AND raw_school = 'NC State'`,
	).Scan(&name, &conference, &record); err != nil {
		t.Fatalf("query rank-1 entry: %v", err)
	}
	if name != "Vincent Robinson" {
		t.Errorf("raw_source_string = %q, want %q", name, "Vincent Robinson")
	}
	if conference != "ACC" {
		t.Errorf("raw_conference = %q, want %q", conference, "ACC")
	}
	if record != "9-1" {
		t.Errorf("raw_record = %q, want %q", record, "9-1")
	}
}

func TestIntermatRecord_Idempotent(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	rec := loadIntermatRecord(t, intermatMidSeasonFixture, "2026-01-06")

	if _, err := IntermatRecord(ctx, db, rec, 2026, time.Now()); err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	res2, err := IntermatRecord(ctx, db, rec, 2026, time.Now())
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	if res2.SnapshotsCreated != 0 || res2.EntriesCreated != 0 {
		t.Errorf("re-run created %d snapshots / %d entries, want 0/0", res2.SnapshotsCreated, res2.EntriesCreated)
	}
	if res2.SnapshotsSkipped != 2 {
		t.Errorf("re-run skipped %d, want 2", res2.SnapshotsSkipped)
	}
	if got := intermatCount(t, db, `SELECT COUNT(*) FROM ranking_entries`); got != 66 {
		t.Errorf("entries after re-run = %d, want 66", got)
	}
}

// The real preseason fixture (r53) has a genuine ragged row at 125lbs rank
// 19 (see scraper/intermat/testdata/README.md) — 125lbs must land in
// Result.Failures while 285lbs, which has no such gap, still ingests. Same
// per-weight isolation contract Container already provides for Flo's
// per-edition failures.
func TestIntermatRecord_IsolatesBadWeight(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	rec := loadIntermatRecord(t, intermatPreseasonFixture, "2025-09-05")

	res, err := IntermatRecord(ctx, db, rec, 2026, time.Now())
	if err != nil {
		t.Fatalf("IntermatRecord should not hard-error on a per-weight failure: %v", err)
	}
	if res.SnapshotsCreated != 1 {
		t.Errorf("snapshots created = %d, want 1 (285lbs only)", res.SnapshotsCreated)
	}
	if len(res.Failures) != 1 {
		t.Fatalf("failures = %d, want 1", len(res.Failures))
	}
	if res.Failures[0].WeightClass != 125 {
		t.Errorf("failed weight = %d, want 125", res.Failures[0].WeightClass)
	}

	if got := intermatCount(t, db, `SELECT COUNT(*) FROM snapshots`); got != 1 {
		t.Errorf("snapshots in db = %d, want 1", got)
	}
	if got := intermatCount(t, db, `SELECT COUNT(DISTINCT weight_class) FROM snapshots WHERE weight_class = 285`); got != 1 {
		t.Errorf("285lbs snapshot missing")
	}
}

func intermatCount(t *testing.T, db *sql.DB, query string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", query, err)
	}
	return n
}
