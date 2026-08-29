package resolve

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pipeline/internal/ingest"
	"pipeline/internal/scraper"
	"pipeline/internal/store"
	"pipeline/internal/storetest"
)

var fixtureFile = filepath.Join("..", "scraper", "testdata", "ranking_container_14300895.json")

const sourceName = "FloWrestling"

func newDB(t *testing.T) *sql.DB {
	t.Helper()
	return storetest.NewDB(t)
}

func ingestFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	data, err := os.ReadFile(fixtureFile)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	c, err := scraper.ParseContainer(data)
	if err != nil {
		t.Fatalf("parse container: %v", err)
	}
	if _, err := ingest.Container(context.Background(), db, c, 2026, time.Now()); err != nil {
		t.Fatalf("ingest: %v", err)
	}
}

func count(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", query, err)
	}
	return n
}

func TestSource_ResolvesFixture(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	ingestFixture(t, db)

	totalEntries := count(t, db, `SELECT COUNT(*) FROM ranking_entries`)
	// Distinct raw identities in this clean single-source corpus equals the
	// number of canonical wrestlers the pass should create (no over-merge, no
	// under-merge).
	distinctRaw := count(t, db,
		`SELECT COUNT(*) FROM (SELECT DISTINCT raw_source_string, raw_school FROM ranking_entries) AS raw_identities`)

	res, err := Source(ctx, db, sourceName)
	if err != nil {
		t.Fatalf("Source: %v", err)
	}
	if res.EntriesResolved != totalEntries {
		t.Errorf("entries resolved = %d, want all %d", res.EntriesResolved, totalEntries)
	}
	if res.WrestlersCreated != distinctRaw {
		t.Errorf("wrestlers created = %d, want %d (distinct raw identities)",
			res.WrestlersCreated, distinctRaw)
	}

	// Nothing left NULL; wrestler/alias rows match expectations.
	if got := count(t, db, `SELECT COUNT(*) FROM ranking_entries WHERE wrestler_id IS NULL`); got != 0 {
		t.Errorf("unresolved entries after pass = %d, want 0", got)
	}
	if got := count(t, db, `SELECT COUNT(*) FROM wrestlers`); got != distinctRaw {
		t.Errorf("wrestlers in db = %d, want %d", got, distinctRaw)
	}
	if got := count(t, db, `SELECT COUNT(*) FROM wrestler_aliases`); got != distinctRaw {
		t.Errorf("aliases in db = %d, want %d", got, distinctRaw)
	}

	// A wrestler appearing every week collapses to a single identity with many
	// linked entries.
	robinsonEntries := count(t, db, `
SELECT COUNT(*) FROM ranking_entries
WHERE raw_source_string = 'Vincent Robinson' AND raw_school = 'NC State'`)
	robinsonWrestlers := count(t, db, `
SELECT COUNT(DISTINCT wrestler_id) FROM ranking_entries
WHERE raw_source_string = 'Vincent Robinson' AND raw_school = 'NC State'`)
	if robinsonWrestlers != 1 {
		t.Errorf("Vincent Robinson maps to %d wrestlers, want 1", robinsonWrestlers)
	}
	if robinsonEntries < 20 {
		t.Errorf("Vincent Robinson linked to %d entries, want many", robinsonEntries)
	}
}

func TestSource_Idempotent(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	ingestFixture(t, db)

	if _, err := Source(ctx, db, sourceName); err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	wrestlers1 := count(t, db, `SELECT COUNT(*) FROM wrestlers`)

	res2, err := Source(ctx, db, sourceName)
	if err != nil {
		t.Fatalf("second resolve: %v", err)
	}
	if res2.EntriesResolved != 0 || res2.WrestlersCreated != 0 || res2.AliasesCreated != 0 {
		t.Errorf("re-run did work: %+v, want all zero", res2)
	}
	if got := count(t, db, `SELECT COUNT(*) FROM wrestlers`); got != wrestlers1 {
		t.Errorf("wrestlers after re-run = %d, want %d", got, wrestlers1)
	}
}

// Identity = (name, school): same person same school across snapshots is one
// wrestler; a mid-season transfer (same name, new school) deliberately becomes
// two identities in v0.
func TestSource_IdentityIsNameAndSchool(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	sourceID, err := store.SourceID(ctx, db, sourceName)
	if err != nil {
		t.Fatal(err)
	}

	mkSnap := func(date string) int64 {
		_, _, err := store.IngestEdition(ctx, db, store.Snapshot{
			SourceID: sourceID, WeightClass: 125, Season: 2026,
			PublishedDate: date, CapturedAt: "2026-01-01T00:00:00Z",
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		var id int64
		if err := db.QueryRow(
			`SELECT id FROM snapshots WHERE published_date = $1`, date).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	addEntry := func(snap int64, rank int, name, school string) {
		if _, err := store.InsertRankingEntry(ctx, db, store.RankingEntry{
			SnapshotID: snap, Rank: rank, RawSourceString: name, RawSchool: &school,
		}); err != nil {
			t.Fatal(err)
		}
	}

	s1, s2 := mkSnap("2026-01-06"), mkSnap("2026-01-13")
	// Stayer: same name+school in both weeks -> 1 identity.
	addEntry(s1, 1, "Vincent Robinson", "NC State")
	addEntry(s2, 1, "Vincent Robinson", "NC State")
	// Transfer: same name, different school -> 2 identities.
	addEntry(s1, 2, "John Doe", "Iowa")
	addEntry(s2, 2, "John Doe", "Penn State")
	// Case/whitespace drift on the stayer's school must still match.
	addEntry(s2, 3, "Vincent Robinson", "nc  state")

	if _, err := Source(ctx, db, sourceName); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	stayer := count(t, db, `
SELECT COUNT(DISTINCT wrestler_id) FROM ranking_entries WHERE raw_source_string = 'Vincent Robinson'`)
	if stayer != 1 {
		t.Errorf("stayer (incl. drift) maps to %d wrestlers, want 1", stayer)
	}
	transfer := count(t, db, `
SELECT COUNT(DISTINCT wrestler_id) FROM ranking_entries WHERE raw_source_string = 'John Doe'`)
	if transfer != 2 {
		t.Errorf("transfer maps to %d wrestlers, want 2 (v0 fragments transfers)", transfer)
	}
}
