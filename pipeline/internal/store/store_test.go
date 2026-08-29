package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

// repo-root-relative dirs, from pipeline/internal/store.
var (
	migrationsDir = filepath.Join("..", "..", "..", "db", "migrations")
	seedDir       = filepath.Join("..", "..", "..", "db", "seed")
)

// TestStoreSmoke exercises the full store contract against a fresh Postgres
// schema (freshDB, testdb_test.go): apply migrations + seed, insert two
// consecutive snapshots for one weight class (with a wrestler who only appears
// in the second), read them back, and verify MovementForWeight derives the
// right previous ranks — including the NULL-previous-rank case for a newcomer.
func TestStoreSmoke(t *testing.T) {
	ctx := context.Background()
	db := freshDB(t)

	// Seed landed.
	var sourceID int64
	if err := db.QueryRowContext(ctx,
		`SELECT id FROM sources WHERE name = 'FloWrestling'`).Scan(&sourceID); err != nil {
		t.Fatalf("FloWrestling not seeded: %v", err)
	}

	// Seeds are idempotent: a second run must not duplicate.
	if err := ApplySeeds(ctx, db, seedDir); err != nil {
		t.Fatalf("seeds (2nd run): %v", err)
	}
	var floCount int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sources WHERE name = 'FloWrestling'`).Scan(&floCount); err != nil {
		t.Fatal(err)
	}
	if floCount != 1 {
		t.Fatalf("FloWrestling rows after re-seed: want 1, got %d", floCount)
	}

	// A school and three wrestlers to reference from entries.
	schoolID := mustExec(t, ctx, db,
		`INSERT INTO schools (name, division) VALUES ('Iowa', 'DI') RETURNING id`)
	alice := mustExec(t, ctx, db,
		`INSERT INTO wrestlers (full_name, current_school_id, eligibility_year) VALUES ('Alice Adams', $1, 'SR') RETURNING id`, schoolID)
	bob := mustExec(t, ctx, db,
		`INSERT INTO wrestlers (full_name, current_school_id, eligibility_year) VALUES ('Bob Barnes', $1, 'JR') RETURNING id`, schoolID)
	cara := mustExec(t, ctx, db,
		`INSERT INTO wrestlers (full_name, current_school_id, eligibility_year) VALUES ('Cara Cole', $1, 'FR') RETURNING id`, schoolID)

	const weight, season = 125, 2026

	// Snapshot 1: Alice #1, Bob #2.
	snap1, err := InsertSnapshot(ctx, db, Snapshot{
		SourceID: sourceID, WeightClass: weight, Season: season,
		PublishedDate: "2026-01-06", CapturedAt: "2026-01-06T12:00:00Z",
	})
	if err != nil {
		t.Fatalf("insert snap1: %v", err)
	}
	// Snapshot 2: Bob #1 (up), Alice #2 (down), Cara #3 (new -> no previous).
	snap2, err := InsertSnapshot(ctx, db, Snapshot{
		SourceID: sourceID, WeightClass: weight, Season: season,
		PublishedDate: "2026-01-13", CapturedAt: "2026-01-13T12:00:00Z",
	})
	if err != nil {
		t.Fatalf("insert snap2: %v", err)
	}

	insertEntry(t, ctx, db, snap1, &alice, 1, "Alice Adams", "Iowa", "SR")
	insertEntry(t, ctx, db, snap1, &bob, 2, "Bob Barnes", "Iowa", "JR")
	insertEntry(t, ctx, db, snap2, &bob, 1, "Bob Barnes", "Iowa", "JR")
	insertEntry(t, ctx, db, snap2, &alice, 2, "Alice Adams", "Iowa", "SR")
	insertEntry(t, ctx, db, snap2, &cara, 3, "Cara Cole", "Iowa", "FR")

	// Read-back: GetSnapshot + ListEntries.
	s2, err := GetSnapshot(ctx, db, snap2)
	if err != nil {
		t.Fatalf("get snap2: %v", err)
	}
	if s2.PublishedDate != "2026-01-13" || s2.WeightClass != weight {
		t.Errorf("get snap2: unexpected %+v", s2)
	}

	entries1, err := ListEntries(ctx, db, snap1)
	if err != nil {
		t.Fatalf("list entries snap1: %v", err)
	}
	if len(entries1) != 2 {
		t.Fatalf("snap1 entries: want 2, got %d", len(entries1))
	}
	if entries1[0].Rank != 1 || entries1[0].WrestlerID == nil || *entries1[0].WrestlerID != alice {
		t.Errorf("snap1 rank-1 entry wrong: %+v", entries1[0])
	}
	if entries1[0].RawSourceString != "Alice Adams" {
		t.Errorf("raw_source_string not preserved: %q", entries1[0].RawSourceString)
	}
	if entries1[0].RawSchool == nil || *entries1[0].RawSchool != "Iowa" {
		t.Errorf("raw_school not preserved: %v", entries1[0].RawSchool)
	}
	if entries1[0].RawGrade == nil || *entries1[0].RawGrade != "SR" {
		t.Errorf("raw_grade not preserved: %v", entries1[0].RawGrade)
	}

	// Movement: 5 rows, previous ranks derived per wrestler.
	moves, err := MovementForWeight(ctx, db, sourceID, weight, season)
	if err != nil {
		t.Fatalf("movement: %v", err)
	}
	if len(moves) != 5 {
		t.Fatalf("movement rows: want 5, got %d", len(moves))
	}

	type key struct{ snap, wrestler int64 }
	got := make(map[key]Movement, len(moves))
	for _, m := range moves {
		if m.WrestlerID == nil {
			t.Fatalf("unexpected nil wrestler_id in movement row %+v", m)
		}
		got[key{m.SnapshotID, *m.WrestlerID}] = m
	}

	// Snapshot 1: nobody has a previous rank.
	assertNoPrev(t, "alice@snap1", got[key{snap1, alice}], 1)
	assertNoPrev(t, "bob@snap1", got[key{snap1, bob}], 2)

	// Snapshot 2: returning wrestlers carry their snapshot-1 rank.
	assertPrev(t, "bob@snap2", got[key{snap2, bob}], 1, 2)     // rank 1, was 2 -> up 1
	assertPrev(t, "alice@snap2", got[key{snap2, alice}], 2, 1) // rank 2, was 1 -> down 1

	// The newcomer has no previous rank.
	assertNoPrev(t, "cara@snap2", got[key{snap2, cara}], 3)
}

func assertPrev(t *testing.T, label string, m Movement, wantRank int, wantPrev int64) {
	t.Helper()
	if m.Rank != wantRank {
		t.Errorf("%s rank: want %d, got %d", label, wantRank, m.Rank)
	}
	if m.PrevRank == nil {
		t.Errorf("%s prev_rank: want %d, got nil", label, wantPrev)
		return
	}
	if *m.PrevRank != wantPrev {
		t.Errorf("%s prev_rank: want %d, got %d", label, wantPrev, *m.PrevRank)
	}
}

func assertNoPrev(t *testing.T, label string, m Movement, wantRank int) {
	t.Helper()
	if m.Rank != wantRank {
		t.Errorf("%s rank: want %d, got %d", label, wantRank, m.Rank)
	}
	if m.PrevRank != nil {
		t.Errorf("%s prev_rank: want nil, got %d", label, *m.PrevRank)
	}
}

// mustExec runs a RETURNING-id insert and returns the new id. Postgres has no
// LastInsertId, so test fixture setup uses RETURNING directly rather than
// going through the store package's own insert helpers (which don't exist for
// schools/wrestlers-with-explicit-school, since v0 resolution never sets
// current_school_id — see wrestlers.go).
func mustExec(t *testing.T, ctx context.Context, db *sql.DB, query string, args ...any) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRowContext(ctx, query, args...).Scan(&id); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
	return id
}

func insertEntry(t *testing.T, ctx context.Context, db *sql.DB, snapshotID int64, wrestlerID *int64, rank int, raw, school, grade string) {
	t.Helper()
	if _, err := InsertRankingEntry(ctx, db, RankingEntry{
		SnapshotID: snapshotID, WrestlerID: wrestlerID, Rank: rank,
		RawSourceString: raw, RawSchool: &school, RawGrade: &grade,
	}); err != nil {
		t.Fatalf("insert entry %q: %v", raw, err)
	}
}
