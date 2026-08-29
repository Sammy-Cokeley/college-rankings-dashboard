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
)

// fullFixtureFile is the whole-season, all-10-weights corpus: the only fixture
// that contains the Penn/Pennsylvania false split and the five genuine transfers.
var fullFixtureFile = filepath.Join("..", "scraper", "testdata", "ranking_container_14300895_10weights.json")

func ingestFullFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	data, err := os.ReadFile(fullFixtureFile)
	if err != nil {
		t.Fatalf("read full fixture: %v", err)
	}
	c, err := scraper.ParseContainer(data)
	if err != nil {
		t.Fatalf("parse container: %v", err)
	}
	if _, err := ingest.Container(context.Background(), db, c, 2026, time.Now()); err != nil {
		t.Fatalf("ingest: %v", err)
	}
}

func distinctWrestlers(t *testing.T, db *sql.DB, name string) int {
	t.Helper()
	return count(t, db,
		`SELECT COUNT(DISTINCT wrestler_id) FROM ranking_entries WHERE raw_source_string = $1`, name)
}

// School canonicalization must merge the one false split (Mougalian, published
// as Penn and Pennsylvania) into a single identity, while leaving the five real
// transfers fragmented into two identities each. Net effect on the corpus:
// exactly one fewer distinct wrestler than before the change (520 -> 519, see
// docs/sources/flowrestling-validation.md).
func TestSource_CanonicalizesSchoolFalseSplit(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	ingestFullFixture(t, db)

	if _, err := Source(ctx, db, sourceName); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	// The fix: one canonical wrestler across both spellings.
	if got := distinctWrestlers(t, db, "Evan Mougalian"); got != 1 {
		t.Errorf("Evan Mougalian maps to %d wrestlers, want 1 (Penn/Pennsylvania merged)", got)
	}

	// The guardrail: genuine transfers must still fragment into two identities.
	for _, name := range []string{
		"Carter Young", "James Conway", "Patrick Brophy", "Teague Travis", "Wynton Denkins",
	} {
		if got := distinctWrestlers(t, db, name); got != 2 {
			t.Errorf("%s maps to %d wrestlers, want 2 (transfer stays fragmented)", name, got)
		}
	}

	// Whole-corpus count: the Mougalian merge is the ONLY identity change vs the
	// pre-canonicalization baseline of 520.
	if got := count(t, db, `SELECT COUNT(*) FROM wrestlers`); got != 519 {
		t.Errorf("wrestlers = %d, want 519 (520 baseline minus the one Mougalian merge)", got)
	}
	// Both raw spellings are still recorded as aliases of the one wrestler — the
	// raw string is never lost.
	if got := count(t, db, `SELECT COUNT(*) FROM wrestler_aliases`); got != 521 {
		t.Errorf("aliases = %d, want 521 (both Mougalian spellings retained)", got)
	}
	// Nothing left unresolved.
	if got := count(t, db, `SELECT COUNT(*) FROM ranking_entries WHERE wrestler_id IS NULL`); got != 0 {
		t.Errorf("unresolved entries = %d, want 0", got)
	}
}
