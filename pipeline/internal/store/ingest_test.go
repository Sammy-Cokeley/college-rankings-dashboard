package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func freshDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := ApplyMigrations(ctx, db, migrationsDir); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	if err := ApplySeeds(ctx, db, seedDir); err != nil {
		t.Fatalf("seeds: %v", err)
	}
	return db
}

func TestIngestEdition_CreatesThenSkips(t *testing.T) {
	ctx := context.Background()
	db := freshDB(t)

	sourceID, err := SourceID(ctx, db, "FloWrestling")
	if err != nil {
		t.Fatalf("SourceID: %v", err)
	}

	school := "NC State"
	grade := "SO"
	snap := Snapshot{
		SourceID: sourceID, WeightClass: 125, Season: 2026,
		PublishedDate: "2025-09-29", CapturedAt: "2026-06-29T00:00:00Z",
	}
	entries := []RankingEntry{
		{Rank: 1, RawSourceString: "Vincent Robinson", RawSchool: &school, RawGrade: &grade},
		{Rank: 2, RawSourceString: "Troy Spratley", RawSchool: &school, RawGrade: &grade},
	}

	id, created, err := IngestEdition(ctx, db, snap, entries)
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	if !created {
		t.Error("first ingest: created = false, want true")
	}

	got, err := ListEntries(ctx, db, id)
	if err != nil {
		t.Fatalf("list entries: %v", err)
	}
	if len(got) != 2 || got[0].RawSourceString != "Vincent Robinson" {
		t.Fatalf("entries = %+v", got)
	}

	// Re-ingest the same edition: no new snapshot, no new entries.
	id2, created2, err := IngestEdition(ctx, db, snap, entries)
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	if created2 {
		t.Error("second ingest: created = true, want false")
	}
	if id2 != id {
		t.Errorf("second ingest id = %d, want existing %d", id2, id)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM ranking_entries`); n != 2 {
		t.Errorf("entries after re-ingest = %d, want 2", n)
	}
}

func TestResolutionCRUD(t *testing.T) {
	ctx := context.Background()
	db := freshDB(t)
	sourceID, _ := SourceID(ctx, db, "FloWrestling")

	// One snapshot + one unresolved entry.
	snap := Snapshot{SourceID: sourceID, WeightClass: 125, Season: 2026,
		PublishedDate: "2025-09-29", CapturedAt: "2026-06-29T00:00:00Z"}
	school, grade := "NC State", "SO"
	id, _, err := IngestEdition(ctx, db, snap,
		[]RankingEntry{{Rank: 1, RawSourceString: "Vincent Robinson", RawSchool: &school, RawGrade: &grade}})
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := ListEntries(ctx, db, id)
	entryID := entries[0].ID

	// Initially listed as unresolved.
	un, err := ListUnresolvedEntries(ctx, db, sourceID)
	if err != nil {
		t.Fatalf("list unresolved: %v", err)
	}
	if len(un) != 1 || un[0].EntryID != entryID {
		t.Fatalf("unresolved = %+v", un)
	}

	// Create wrestler + alias, attach to entry.
	wid, err := InsertWrestler(ctx, db, "Vincent Robinson", "SO")
	if err != nil {
		t.Fatalf("insert wrestler: %v", err)
	}
	if err := InsertAlias(ctx, db, wid, sourceID, "Vincent Robinson", &school); err != nil {
		t.Fatalf("insert alias: %v", err)
	}
	if err := SetEntryWrestler(ctx, db, entryID, wid); err != nil {
		t.Fatalf("set entry wrestler: %v", err)
	}

	// Now resolved: gone from unresolved list, present in aliases.
	un, _ = ListUnresolvedEntries(ctx, db, sourceID)
	if len(un) != 0 {
		t.Errorf("unresolved after attach = %d, want 0", len(un))
	}
	aliases, err := ListAliases(ctx, db, sourceID)
	if err != nil {
		t.Fatalf("list aliases: %v", err)
	}
	if len(aliases) != 1 || aliases[0].WrestlerID != wid || aliases[0].RawName != "Vincent Robinson" {
		t.Errorf("aliases = %+v", aliases)
	}
}

// Documents a known v0 limitation: the schema's UNIQUE(snapshot_id, rank) means
// an edition containing tie ranks (which docs/sources/flowrestling.md flags as
// possible in Flo's hand-entered tables) cannot be ingested. The behavior must
// be fail-loud AND data-safe: the whole edition's transaction rolls back, so no
// partial snapshot or entries are left behind. A real tie needs a schema/product
// decision (the constraint can't hold two rank-5 rows) — not a silent offset.
func TestIngestEdition_TieRanksFailAtomically(t *testing.T) {
	ctx := context.Background()
	db := freshDB(t)
	sourceID, _ := SourceID(ctx, db, "FloWrestling")

	school := "NC State"
	_, _, err := IngestEdition(ctx, db, Snapshot{
		SourceID: sourceID, WeightClass: 125, Season: 2026,
		PublishedDate: "2025-09-29", CapturedAt: "2026-06-29T00:00:00Z",
	}, []RankingEntry{
		{Rank: 5, RawSourceString: "Wrestler A", RawSchool: &school},
		{Rank: 5, RawSourceString: "Wrestler B", RawSchool: &school}, // tie
	})
	if err == nil {
		t.Fatal("expected a UNIQUE(snapshot_id, rank) error on tie ranks")
	}
	// Atomic rollback: nothing persisted.
	if n := countRows(t, db, `SELECT COUNT(*) FROM snapshots`); n != 0 {
		t.Errorf("snapshots after failed tie ingest = %d, want 0 (rolled back)", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM ranking_entries`); n != 0 {
		t.Errorf("entries after failed tie ingest = %d, want 0 (rolled back)", n)
	}
}

func countRows(t *testing.T, db *sql.DB, query string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}
