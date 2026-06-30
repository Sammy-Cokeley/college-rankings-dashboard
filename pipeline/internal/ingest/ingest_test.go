package ingest

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"pipeline/internal/scraper"
	"pipeline/internal/store"
)

var (
	migrationsDir = filepath.Join("..", "..", "..", "db", "migrations")
	seedDir       = filepath.Join("..", "..", "..", "db", "seed")
	fixtureFile   = filepath.Join("..", "scraper", "testdata", "ranking_container_14300895.json")
)

func newDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.ApplyMigrations(ctx, db, migrationsDir); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	if err := store.ApplySeeds(ctx, db, seedDir); err != nil {
		t.Fatalf("seeds: %v", err)
	}
	return db
}

func loadFixture(t *testing.T) scraper.Container {
	t.Helper()
	data, err := os.ReadFile(fixtureFile)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	c, err := scraper.ParseContainer(data)
	if err != nil {
		t.Fatalf("parse container: %v", err)
	}
	return c
}

// The fixture is 2 weights x 22 editions. Row counts per edition: 24 (06-19) +
// 24 (07-02) + 20 editions x 33 = 708 entries per weight, 1416 total.
const (
	wantSnapshots = 44
	wantEntries   = 1416
)

func TestContainer_IngestsFixture(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	c := loadFixture(t)

	res, err := Container(ctx, db, c, 2026, time.Now())
	if err != nil {
		t.Fatalf("Container: %v", err)
	}
	if res.SnapshotsCreated != wantSnapshots {
		t.Errorf("snapshots created = %d, want %d", res.SnapshotsCreated, wantSnapshots)
	}
	if res.EntriesCreated != wantEntries {
		t.Errorf("entries created = %d, want %d", res.EntriesCreated, wantEntries)
	}

	// Row counts actually landed in the DB.
	if got := count(t, db, `SELECT COUNT(*) FROM snapshots`); got != wantSnapshots {
		t.Errorf("snapshots in db = %d, want %d", got, wantSnapshots)
	}
	if got := count(t, db, `SELECT COUNT(*) FROM ranking_entries`); got != wantEntries {
		t.Errorf("entries in db = %d, want %d", got, wantEntries)
	}
	// Every entry is unresolved immediately after ingestion.
	if got := count(t, db, `SELECT COUNT(*) FROM ranking_entries WHERE wrestler_id IS NOT NULL`); got != 0 {
		t.Errorf("resolved entries after ingest = %d, want 0", got)
	}

	// Both weights present; preseason edition stored with its real date.
	if got := count(t, db, `SELECT COUNT(DISTINCT weight_class) FROM snapshots`); got != 2 {
		t.Errorf("distinct weights = %d, want 2", got)
	}
	if got := count(t, db,
		`SELECT COUNT(*) FROM snapshots WHERE published_date = '2025-06-19'`); got != 2 {
		t.Errorf("preseason snapshots = %d, want 2 (one per weight)", got)
	}
}

func TestContainer_Idempotent(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	c := loadFixture(t)

	if _, err := Container(ctx, db, c, 2026, time.Now()); err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	res2, err := Container(ctx, db, c, 2026, time.Now())
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	if res2.SnapshotsCreated != 0 || res2.EntriesCreated != 0 {
		t.Errorf("re-run created %d snapshots / %d entries, want 0/0",
			res2.SnapshotsCreated, res2.EntriesCreated)
	}
	if res2.SnapshotsSkipped != wantSnapshots {
		t.Errorf("re-run skipped %d, want %d", res2.SnapshotsSkipped, wantSnapshots)
	}
	// No duplication.
	if got := count(t, db, `SELECT COUNT(*) FROM ranking_entries`); got != wantEntries {
		t.Errorf("entries after re-run = %d, want %d", got, wantEntries)
	}
}

// A single malformed edition must be isolated: it lands in Result.Failures
// while every other edition still ingests. (Tie ranks are the documented real
// case; a duplicate-rank table is the same failure class and easy to construct.)
func TestContainer_IsolatesBadEdition(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)

	good := `<table><tbody>
<tr><td>Rank</td><td>Grade</td><td>Name</td><td>School</td></tr>
<tr><td>1</td><td>SO</td><td>Vincent Robinson</td><td>NC State</td></tr></tbody></table>`
	// Tie rank: violates UNIQUE(snapshot_id, rank) at ingest time.
	tie := `<table><tbody>
<tr><td>Rank</td><td>Grade</td><td>Name</td><td>School</td></tr>
<tr><td>1</td><td>SO</td><td>Wrestler A</td><td>Iowa</td></tr>
<tr><td>1</td><td>JR</td><td>Wrestler B</td><td>Ohio State</td></tr></tbody></table>`

	c := scraper.Container{
		ID:    1,
		Title: "2025-26 NCAA DI Wrestling Rankings",
		RankingSections: map[string][]scraper.Edition{
			"2":  {{Name: "125", PublishDate: "2025-09-29", Content: good}},
			"11": {{Name: "285", PublishDate: "2025-09-29", Content: tie}},
		},
	}

	res, err := Container(ctx, db, c, 2026, time.Now())
	if err != nil {
		t.Fatalf("Container should not hard-error on a per-edition failure: %v", err)
	}
	if res.SnapshotsCreated != 1 {
		t.Errorf("snapshots created = %d, want 1 (the good edition)", res.SnapshotsCreated)
	}
	if len(res.Failures) != 1 {
		t.Fatalf("failures = %d, want 1", len(res.Failures))
	}
	if res.Failures[0].WeightClass != 285 {
		t.Errorf("failed edition weight = %d, want 285", res.Failures[0].WeightClass)
	}
	// Good edition persisted; failed one left nothing behind.
	if got := count(t, db, `SELECT COUNT(*) FROM snapshots`); got != 1 {
		t.Errorf("snapshots in db = %d, want 1", got)
	}
	if got := count(t, db, `SELECT COUNT(*) FROM ranking_entries`); got != 1 {
		t.Errorf("entries in db = %d, want 1", got)
	}
}

func count(t *testing.T, db *sql.DB, query string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", query, err)
	}
	return n
}

func TestSeasonFromTitle(t *testing.T) {
	cases := []struct {
		title string
		want  int
		ok    bool
	}{
		{"2025-26 NCAA DI Wrestling Rankings", 2026, true},
		{"2019-20 NCAA DI Wrestling Rankings", 2020, true},
		{"1999-00 Something", 2000, true},
		{"No year here", 0, false},
	}
	for _, c := range cases {
		got, ok := SeasonFromTitle(c.title)
		if ok != c.ok || got != c.want {
			t.Errorf("SeasonFromTitle(%q) = %d,%v want %d,%v", c.title, got, ok, c.want, c.ok)
		}
	}
}
