package store

import (
	"context"
	"database/sql"
	"testing"
)

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

// A genuine tie — two wrestlers published at the same rank — must ingest, not
// fail: Flo's hand-entered tables carry them (197, 2026-03-27), and the schema
// stores the source's rank verbatim (schema.md §7). Both rows land in one
// snapshot; the key is UNIQUE(snapshot_id, rank, raw_source_string), so distinct
// names at the same rank coexist.
func TestIngestEdition_AllowsTieRanks(t *testing.T) {
	ctx := context.Background()
	db := freshDB(t)
	sourceID, _ := SourceID(ctx, db, "FloWrestling")

	school := "NC State"
	_, created, err := IngestEdition(ctx, db, Snapshot{
		SourceID: sourceID, WeightClass: 125, Season: 2026,
		PublishedDate: "2025-09-29", CapturedAt: "2026-06-29T00:00:00Z",
	}, []RankingEntry{
		{Rank: 5, RawSourceString: "Wrestler A", RawSchool: &school},
		{Rank: 5, RawSourceString: "Wrestler B", RawSchool: &school}, // tie
	})
	if err != nil {
		t.Fatalf("tie edition should ingest: %v", err)
	}
	if !created {
		t.Error("created = false, want true (new snapshot written)")
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM snapshots`); n != 1 {
		t.Errorf("snapshots = %d, want 1", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM ranking_entries`); n != 2 {
		t.Errorf("entries = %d, want 2 (both tied rows stored)", n)
	}
}

// The relaxed key still guards against the realistic bug: the exact same row
// (same snapshot, rank, and raw_source_string) inserted twice is rejected.
func TestRankingEntries_RejectExactDuplicateRow(t *testing.T) {
	ctx := context.Background()
	db := freshDB(t)
	sourceID, _ := SourceID(ctx, db, "FloWrestling")

	snapID, err := InsertSnapshot(ctx, db, Snapshot{
		SourceID: sourceID, WeightClass: 125, Season: 2026,
		PublishedDate: "2025-09-29", CapturedAt: "2026-06-29T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
	entry := RankingEntry{SnapshotID: snapID, Rank: 5, RawSourceString: "Wrestler A"}
	if _, err := InsertRankingEntry(ctx, db, entry); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if _, err := InsertRankingEntry(ctx, db, entry); err == nil {
		t.Fatal("expected UNIQUE(snapshot_id, rank, raw_source_string) error on exact-duplicate row")
	}
}

// Two entries tied at one rank must come back in a stable order (rank, then
// raw_source_string), not the database's arbitrary order for equal ranks — so
// SSR output and tests are deterministic (schema.md §7). Insert B before A to
// prove the query sorts rather than echoing insertion order.
func TestListEntries_TieRankStableOrder(t *testing.T) {
	ctx := context.Background()
	db := freshDB(t)
	sourceID, _ := SourceID(ctx, db, "FloWrestling")

	snapID, err := InsertSnapshot(ctx, db, Snapshot{
		SourceID: sourceID, WeightClass: 197, Season: 2026,
		PublishedDate: "2026-03-27", CapturedAt: "2026-06-29T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
	for _, name := range []string{"Wrestler B", "Wrestler A"} { // reverse order
		if _, err := InsertRankingEntry(ctx, db,
			RankingEntry{SnapshotID: snapID, Rank: 21, RawSourceString: name}); err != nil {
			t.Fatalf("insert %q: %v", name, err)
		}
	}

	entries, err := ListEntries(ctx, db, snapID)
	if err != nil {
		t.Fatalf("list entries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
	if entries[0].RawSourceString != "Wrestler A" || entries[1].RawSourceString != "Wrestler B" {
		t.Errorf("tie order = [%q, %q], want [Wrestler A, Wrestler B]",
			entries[0].RawSourceString, entries[1].RawSourceString)
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
