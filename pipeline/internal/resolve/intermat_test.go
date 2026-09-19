package resolve

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"pipeline/internal/ingest"
	"pipeline/internal/scraper/intermat"
)

const intermatSourceName = "InterMat"

var intermatFixtureFile = "../scraper/intermat/testdata/record_r63_2weights.html"

// ingestIntermatFixture ingests the mid-season fixture (both weights parse
// cleanly — see scraper/intermat/testdata/README.md for the preseason
// fixture's known real ragged row, not used here).
func ingestIntermatFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	data, err := os.ReadFile(intermatFixtureFile)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	weights, err := intermat.ParseRecord(data)
	if err != nil {
		t.Fatalf("ParseRecord: %v", err)
	}
	rec := intermat.Record{PublishedDate: "2026-01-06", Weights: weights}
	if _, err := ingest.IntermatRecord(context.Background(), db, rec, 2026, time.Now()); err != nil {
		t.Fatalf("IntermatRecord: %v", err)
	}
}

// resolve.Source is fully source-agnostic already — this is a smoke test
// confirming InterMat entries resolve through it with no source-specific
// resolve code needed (per the plan: no schools.go fork, canonicalSchools
// grows in place only if real InterMat spellings actually diverge from
// Flo's, which this fixture's real school names do not — docs/sources/
// intermat.md notes InterMat's school naming matches Flo's short style).
func TestSource_ResolvesIntermatFixture(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	ingestIntermatFixture(t, db)

	totalEntries := count(t, db, `SELECT COUNT(*) FROM ranking_entries`)
	distinctRaw := count(t, db,
		`SELECT COUNT(*) FROM (SELECT DISTINCT raw_source_string, raw_school FROM ranking_entries) AS raw_identities`)

	res, err := Source(ctx, db, intermatSourceName)
	if err != nil {
		t.Fatalf("Source: %v", err)
	}
	if res.EntriesResolved != totalEntries {
		t.Errorf("entries resolved = %d, want all %d", res.EntriesResolved, totalEntries)
	}
	if res.WrestlersCreated != distinctRaw {
		t.Errorf("wrestlers created = %d, want %d (distinct raw identities)", res.WrestlersCreated, distinctRaw)
	}
	if got := count(t, db, `SELECT COUNT(*) FROM ranking_entries WHERE wrestler_id IS NULL`); got != 0 {
		t.Errorf("unresolved entries after pass = %d, want 0", got)
	}
}
