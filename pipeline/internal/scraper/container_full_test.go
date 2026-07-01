package scraper

import (
	"os"
	"testing"
)

// fullFixtureFile is the whole-season, all-10-weights validation corpus (see
// testdata/README.md). It is the executable form of the live-container audit:
// every weight's real table shapes must parse, and the known real-world
// anomalies (a tie rank) must survive ingestion rather than being rejected.
const fullFixtureFile = "testdata/ranking_container_14300895_10weights.json"

func loadFullFixture(t *testing.T) Container {
	t.Helper()
	data, err := os.ReadFile(fullFixtureFile)
	if err != nil {
		t.Fatalf("read full fixture: %v", err)
	}
	c, err := ParseContainer(data)
	if err != nil {
		t.Fatalf("ParseContainer: %v", err)
	}
	return c
}

// The full container has all 10 weight sections, 22 editions each, and every one
// of the 220 editions' content tables parses into non-empty ranked rows. This is
// the cross-weight table-shape guard the 2-weight fixture cannot give.
func TestFullContainer_AllEditionsParse(t *testing.T) {
	c := loadFullFixture(t)

	we := c.WeightEditions()
	if len(we) != 220 {
		t.Fatalf("weight-editions = %d, want 220 (10 weights x 22)", len(we))
	}

	weights := map[int]int{}
	for _, w := range we {
		weights[w.WeightClass]++
		rows, err := ParseTable(w.Edition.Content)
		if err != nil {
			t.Errorf("weight %d %s: parse: %v", w.WeightClass, w.Edition.PublishDate, err)
			continue
		}
		if len(rows) == 0 {
			t.Errorf("weight %d %s: no rows", w.WeightClass, w.Edition.PublishDate)
		}
		for _, r := range rows {
			if r.Rank < 1 || r.Name == "" || r.School == "" {
				t.Errorf("weight %d %s: bad row %+v", w.WeightClass, w.Edition.PublishDate, r)
			}
		}
	}
	for _, wc := range []int{125, 133, 141, 149, 157, 165, 174, 184, 197, 285} {
		if weights[wc] != 22 {
			t.Errorf("weight %d has %d editions, want 22", wc, weights[wc])
		}
	}
}

// The 197 2026-03-27 edition publishes two wrestlers at rank 21 (a hand-entered
// tie). The parser must surface both rows verbatim — never renumber or drop one
// — so the store can hold the tie (schema.md §7). This is the concrete case that
// broke the UNIQUE(snapshot_id, rank) assumption.
func TestFullContainer_TieRankPreserved(t *testing.T) {
	c := loadFullFixture(t)

	var found bool
	for _, w := range c.WeightEditions() {
		if w.WeightClass != 197 || w.Edition.PublishDate != "2026-03-27" {
			continue
		}
		found = true
		rows, err := ParseTable(w.Edition.Content)
		if err != nil {
			t.Fatalf("parse 197 2026-03-27: %v", err)
		}
		var atRank21 []string
		for _, r := range rows {
			if r.Rank == 21 {
				atRank21 = append(atRank21, r.Name)
			}
		}
		if len(atRank21) != 2 {
			t.Fatalf("rank 21 has %d wrestlers %v, want 2 (the tie)", len(atRank21), atRank21)
		}
	}
	if !found {
		t.Fatal("197 2026-03-27 edition not present in fixture")
	}
}
