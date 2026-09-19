package intermat

import (
	"os"
	"path/filepath"
	"testing"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func weightTable(t *testing.T, weights []RawWeightTable, weightClass int) RawWeightTable {
	t.Helper()
	for _, w := range weights {
		if w.WeightClass == weightClass {
			return w
		}
	}
	t.Fatalf("weight %d not found", weightClass)
	return RawWeightTable{}
}

// r53 is the 2025-26 season's very first DI record (2025-09-05, preseason —
// no matches played yet, hence RECORD="0-0" and no LAST column: nothing to
// compare against). r63 is a mid-season record (2026-01-06) with the full
// column set. Both are real, trimmed Wayback captures (see testdata/README.md).

func TestParseRecord_Preseason(t *testing.T) {
	weights, err := ParseRecord(loadFixture(t, "record_r53_2weights.html"))
	if err != nil {
		t.Fatalf("ParseRecord: %v", err)
	}
	if len(weights) != 2 {
		t.Fatalf("got %d weight tables, want 2", len(weights))
	}
	for _, w := range weights {
		if w.WeightClass != 125 && w.WeightClass != 285 {
			t.Errorf("unexpected weight class %d", w.WeightClass)
		}
		// header + 33 data rows.
		if len(w.Rows) != 34 {
			t.Errorf("weight %d: got %d raw rows (incl. header), want 34", w.WeightClass, len(w.Rows))
		}
	}
}

// 285lbs in the preseason record has no data gaps, so it must parse cleanly
// end-to-end (ParseRecord's raw split + table.ParseRows' header-driven pass)
// with no LAST column (nothing to compare against in the season's first week).
func TestParseRecord_Preseason_285lbs_ParsesCleanly(t *testing.T) {
	weights, err := ParseRecord(loadFixture(t, "record_r53_2weights.html"))
	if err != nil {
		t.Fatalf("ParseRecord: %v", err)
	}
	w := weightTable(t, weights, 285)
	rows, err := ParseRows(w.Rows)
	if err != nil {
		t.Fatalf("ParseRows(285lbs): %v", err)
	}
	if len(rows) != 33 {
		t.Fatalf("got %d rows, want 33", len(rows))
	}
	for _, r := range rows {
		if r.Previous != "" {
			t.Errorf("Previous = %q, want empty (preseason has no LAST column)", r.Previous)
		}
	}
	// Every row in this real fixture is genuinely "0-0" (no matches played
	// yet) — confirms the RECORD column mapped at all, not just that it's
	// syntactically present.
	if rows[0].Record != "0-0" {
		t.Errorf("rank 1 Record = %q, want %q", rows[0].Record, "0-0")
	}
}

// The real r53 page has a genuine hand-entry gap: 125lbs rank 19 (Brendan
// McCrone) is published with no RECORD cell at all — a true ragged row (5
// cells where every other row in that table has 6), not a trimming
// artifact (verified against the raw Wayback capture). ParseRows must fail
// loud on it, exactly the project's established rule for Flo
// (scraper.ParseTable's own doc comment) — surfacing as a single-weight
// ingest.EditionFailure a human can review (internal/ingest/intermat.go),
// never silently misaligning or dropping the missing field. Crucially, this
// must NOT prevent 285lbs (a different table in the same record) from
// parsing — see the test above.
func TestParseRecord_Preseason_125lbs_HasRealRaggedRow(t *testing.T) {
	weights, err := ParseRecord(loadFixture(t, "record_r53_2weights.html"))
	if err != nil {
		t.Fatalf("ParseRecord: %v", err)
	}
	w := weightTable(t, weights, 125)
	if _, err := ParseRows(w.Rows); err == nil {
		t.Fatal("expected ParseRows to fail loud on the real ragged row at rank 19 (Brendan McCrone)")
	}
}

func TestParseRecord_MidSeason(t *testing.T) {
	weights, err := ParseRecord(loadFixture(t, "record_r63_2weights.html"))
	if err != nil {
		t.Fatalf("ParseRecord: %v", err)
	}
	if len(weights) != 2 {
		t.Fatalf("got %d weight tables, want 2", len(weights))
	}

	w := weightTable(t, weights, 125)
	rows, err := ParseRows(w.Rows)
	if err != nil {
		t.Fatalf("ParseRows(125lbs): %v", err)
	}
	if len(rows) != 33 {
		t.Fatalf("got %d rows, want 33", len(rows))
	}
	// Rank 1 matches the recon doc's own sampled row (docs/sources/intermat.md).
	if got := rows[0]; got.Name != "Vincent Robinson" || got.School != "NC State" || got.Record != "9-1" || got.Previous != "1" {
		t.Errorf("rank 1 = %+v, want Vincent Robinson/NC State/9-1/1", got)
	}
	// Rank 33 matches the recon doc's own sampled row too.
	if got := rows[32]; got.Name != "Greg Diakomihalis" || got.School != "Cornell" || got.Previous != "NR" {
		t.Errorf("rank 33 = %+v, want Greg Diakomihalis/Cornell/NR", got)
	}
}

// r76 is the real 2025-26 postseason-final edition — recovered from
// InterMat's LIVE site (2026-09-12), not Wayback, which never captured it
// (docs/sources/intermat.md). 16 rows/weight, FINISH header, and — a real
// wrinkle discovered fetching it — non-numeric R12/R16 codes for the eight
// non-All-American finishers (see finishCodeRanks in table.go).
func TestParseRecord_Postseason(t *testing.T) {
	weights, err := ParseRecord(loadFixture(t, "record_r76_2weights.html"))
	if err != nil {
		t.Fatalf("ParseRecord: %v", err)
	}
	w := weightTable(t, weights, 125)
	rows, err := ParseRows(w.Rows)
	if err != nil {
		t.Fatalf("ParseRows(125lbs): %v", err)
	}
	if len(rows) != 16 {
		t.Fatalf("got %d rows, want 16", len(rows))
	}
	// The real 2026 NCAA DI 125lbs champion.
	if got := rows[0]; got.Rank != 1 || got.Name != "Luke Lilledahl" || got.School != "Penn State" {
		t.Errorf("rank 1 = %+v, want Luke Lilledahl/Penn State", got)
	}
	// R12/R16 codes correctly mapped to tie-group ranks 9/13, no LAST/RECORD
	// (postseason-final columns).
	var sawRank9, sawRank13 bool
	for _, r := range rows {
		if r.Rank == 9 {
			sawRank9 = true
		}
		if r.Rank == 13 {
			sawRank13 = true
		}
		if r.Previous != "" || r.Record != "" {
			t.Errorf("row %+v: postseason-final has no LAST/RECORD columns", r)
		}
	}
	if !sawRank9 || !sawRank13 {
		t.Errorf("expected both an R12-mapped (rank 9) and R16-mapped (rank 13) row, sawRank9=%v sawRank13=%v", sawRank9, sawRank13)
	}
}

func TestParseRecord_NoWeightTables(t *testing.T) {
	if _, err := ParseRecord([]byte(`<html><body><table><tr><td>Tournament Rankings</td></tr></table></body></html>`)); err == nil {
		t.Fatal("expected error when no recognizable weight table is present")
	}
}
