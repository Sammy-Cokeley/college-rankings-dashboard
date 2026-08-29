package resolve

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"pipeline/internal/store"
	"pipeline/internal/storetest"
)

// The actual point of cross-source resolution: a wrestler WrestleStat lists
// on a roster must resolve to the SAME canonical identity FloWrestling
// already established for that person, not a duplicate — this is what
// newGlobalIndex (seeded from every source's aliases, not just WrestleStat's
// own empty-on-first-run set) exists for.
func TestRoster_MatchesExistingFloWrestler(t *testing.T) {
	ctx := context.Background()
	db := storetest.NewDB(t)

	floSourceID, err := store.SourceID(ctx, db, "FloWrestling")
	if err != nil {
		t.Fatal(err)
	}
	school := "NC State"
	if _, _, err := store.IngestEdition(ctx, db, store.Snapshot{
		SourceID: floSourceID, WeightClass: 125, Season: 2027,
		PublishedDate: "2026-11-01", CapturedAt: "2026-11-01T00:00:00Z",
	}, []store.RankingEntry{
		{Rank: 1, RawSourceString: "Vincent Robinson", RawSchool: &school},
	}); err != nil {
		t.Fatalf("ingest flo edition: %v", err)
	}
	if _, err := Source(ctx, db, "FloWrestling"); err != nil {
		t.Fatalf("resolve flo: %v", err)
	}
	var floWrestlerID int64
	if err := db.QueryRowContext(ctx,
		`SELECT wrestler_id FROM ranking_entries WHERE raw_source_string = 'Vincent Robinson'`).
		Scan(&floWrestlerID); err != nil {
		t.Fatalf("flo entry not resolved: %v", err)
	}

	// Same person, same school, in the "First Last" order ingest.cleanName
	// already produces before a roster entry ever reaches resolve — this
	// test feeds that reordered form directly (unit-testing resolve.Roster in
	// isolation from ingest.RosterForTeam).
	mustIngestRoster(t, ctx, db, school, 2027, "Vincent Robinson")

	res, err := Roster(ctx, db, "WrestleStat", 2027)
	if err != nil {
		t.Fatalf("Roster: %v", err)
	}
	if res.WrestlersCreated != 0 {
		t.Errorf("WrestlersCreated = %d, want 0 (should resolve onto the existing Flo wrestler, not mint a new one)", res.WrestlersCreated)
	}
	if res.EntriesResolved != 1 {
		t.Errorf("EntriesResolved = %d, want 1", res.EntriesResolved)
	}

	var rosterWrestlerID int64
	if err := db.QueryRowContext(ctx,
		`SELECT wrestler_id FROM roster_entries WHERE raw_name = 'Vincent Robinson'`).
		Scan(&rosterWrestlerID); err != nil {
		t.Fatalf("roster entry not resolved: %v", err)
	}
	if rosterWrestlerID != floWrestlerID {
		t.Errorf("roster wrestler_id = %d, want %d (the SAME wrestler Flo already resolved)", rosterWrestlerID, floWrestlerID)
	}
}

func TestRoster_NoMatchMintsNewWrestler(t *testing.T) {
	ctx := context.Background()
	db := storetest.NewDB(t)

	mustIngestRoster(t, ctx, db, "Iowa", 2027, "Brand New Guy")

	res, err := Roster(ctx, db, "WrestleStat", 2027)
	if err != nil {
		t.Fatalf("Roster: %v", err)
	}
	if res.WrestlersCreated != 1 {
		t.Errorf("WrestlersCreated = %d, want 1", res.WrestlersCreated)
	}
}

func TestRoster_Idempotent(t *testing.T) {
	ctx := context.Background()
	db := storetest.NewDB(t)
	mustIngestRoster(t, ctx, db, "Iowa", 2027, "Brand New Guy")

	if _, err := Roster(ctx, db, "WrestleStat", 2027); err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	res2, err := Roster(ctx, db, "WrestleStat", 2027)
	if err != nil {
		t.Fatalf("second resolve: %v", err)
	}
	if res2.EntriesResolved != 0 || res2.WrestlersCreated != 0 {
		t.Errorf("re-run did work: %+v, want all zero", res2)
	}
}

// mustIngestRoster inserts one already-resolved-school roster entry directly
// (bypassing ingest.RosterForTeam, which resolve doesn't import — avoids a
// resolve->ingest dependency purely for test setup) with wrestler_id left
// NULL, exactly the shape resolve.Roster expects to find.
func mustIngestRoster(t *testing.T, ctx context.Context, db *sql.DB, schoolName string, season int, rawName string) int64 {
	t.Helper()
	sourceID, err := store.SourceID(ctx, db, "WrestleStat")
	if err != nil {
		t.Fatal(err)
	}
	schoolID, err := store.FindOrCreateSchool(ctx, db, sourceID, schoolName)
	if err != nil {
		t.Fatal(err)
	}
	weight := 125
	id, err := store.UpsertRosterEntry(ctx, db, store.RosterEntry{
		SchoolID: schoolID, Season: season, WeightClass: &weight,
		RawName: rawName, RawSourceString: rawName, CapturedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}
