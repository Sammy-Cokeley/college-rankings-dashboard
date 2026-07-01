package ingest

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pipeline/internal/scraper"
)

// fullFixtureFile is the whole-season, all-10-weights validation corpus — the
// only fixture that contains the real-world tie anomaly (197 2026-03-27).
var fullFixtureFile = filepath.Join("..", "scraper", "testdata", "ranking_container_14300895_10weights.json")

func rowsAt(ranks ...int) []scraper.Row {
	out := make([]scraper.Row, 0, len(ranks))
	for _, r := range ranks {
		out = append(out, scraper.Row{Rank: r})
	}
	return out
}

func TestDetectAnomalies(t *testing.T) {
	cases := []struct {
		name  string
		rows  []scraper.Row
		wantN int // number of issue strings
	}{
		{"clean contiguous", rowsAt(1, 2, 3, 4, 5), 0},
		{"tie only", rowsAt(1, 2, 2, 3), 1},
		{"gap only", rowsAt(1, 2, 4, 5), 1},
		{"tie and gap (the 197 shape)", rowsAt(20, 21, 21, 22, 24), 2},
		{"single row", rowsAt(1), 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := detectAnomalies(c.rows)
			if len(got) != c.wantN {
				t.Errorf("detectAnomalies(%v) = %v (%d issues), want %d",
					c.rows, got, len(got), c.wantN)
			}
		})
	}
}

func loadFullFixture(t *testing.T) scraper.Container {
	t.Helper()
	data, err := os.ReadFile(fullFixtureFile)
	if err != nil {
		t.Fatalf("read full fixture: %v", err)
	}
	c, err := scraper.ParseContainer(data)
	if err != nil {
		t.Fatalf("parse container: %v", err)
	}
	return c
}

// The full 10-weight container has exactly one structural oddity across all 220
// editions: the 197 2026-03-27 tie (and its accompanying gap). It must ingest
// cleanly — zero failures — and report exactly one anomalous edition.
func TestContainer_ReportsTieAnomaly(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)

	res, err := Container(ctx, db, loadFullFixture(t), 2026, time.Now())
	if err != nil {
		t.Fatalf("Container: %v", err)
	}
	if len(res.Failures) != 0 {
		t.Errorf("failures = %d (%v), want 0", len(res.Failures), res.Failures)
	}
	if len(res.Anomalies) != 1 {
		t.Fatalf("anomalies = %d (%v), want 1", len(res.Anomalies), res.Anomalies)
	}
	a := res.Anomalies[0]
	if a.WeightClass != 197 || a.PublishedDate != "2026-03-27" {
		t.Errorf("anomaly = weight %d %s, want 197 2026-03-27", a.WeightClass, a.PublishedDate)
	}
}

// The 2-weight fixture (125 + 285) has no ties or gaps, so a run over it must be
// anomaly-free — the signal is off by default.
func TestContainer_NoAnomaliesInCleanFixture(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)

	res, err := Container(ctx, db, loadFixture(t), 2026, time.Now())
	if err != nil {
		t.Fatalf("Container: %v", err)
	}
	if len(res.Anomalies) != 0 {
		t.Errorf("anomalies = %d (%v), want 0", len(res.Anomalies), res.Anomalies)
	}
}
