package ingest

import (
	"context"
	"testing"
	"time"

	"pipeline/internal/scraper/wrestlestat"
	"pipeline/internal/storetest"
)

func TestCleanName(t *testing.T) {
	cases := []struct {
		published string
		want      string
	}{
		{"#135 Tocci, Nico", "Nico Tocci"},
		{"Tocci, Nico", "Nico Tocci"},
		{"#93 Caprella, Gavin", "Gavin Caprella"},
		{"Franklin & Marshall", "Franklin & Marshall"}, // no comma: left as published
	}
	for _, c := range cases {
		if got := cleanName(c.published); got != c.want {
			t.Errorf("cleanName(%q) = %q, want %q", c.published, got, c.want)
		}
	}
}

func TestIsPlaceholderSlot(t *testing.T) {
	if !isPlaceholderSlot("(Air Force), (Air Force)") {
		t.Error("expected the school-name-repeated pattern to be detected as a placeholder")
	}
	// The actual bug this regression-tests: the placeholder's embedded school
	// string doesn't always match the team's dropdown name used elsewhere in
	// ingestion (confirmed live) — "North Carolina State" vs the dropdown's
	// "NC State", "Army" vs "Army West Point", "Southern Illinois
	// Edwardsville" vs "SIU Edwardsville". A same-as-teamName check misses all
	// of these; this must catch them without knowing the team name at all.
	if !isPlaceholderSlot("(North Carolina State), (North Carolina State)") {
		t.Error("must detect a placeholder whose embedded name differs from the team's dropdown name")
	}
	if !isPlaceholderSlot("(Army), (Army)") {
		t.Error("must detect a placeholder using a shorter school form than the dropdown name")
	}
	if isPlaceholderSlot("#135 Tocci, Nico") {
		t.Error("a real wrestler's name must not be flagged as a placeholder")
	}
	if isPlaceholderSlot("Air Force") {
		t.Error("a single-comma-free token must not be flagged (no comma to split on)")
	}
	if isPlaceholderSlot("(Smith), (Jones)") {
		t.Error("two DIFFERENT parenthesized halves must not be flagged")
	}
}

func TestRosterForTeam(t *testing.T) {
	ctx := context.Background()
	// storetest.NewDB applies db/seed/*.sql, which already seeds the
	// WrestleStat source row (0002_wrestlestat_source.sql).
	db := storetest.NewDB(t)

	rows := []wrestlestat.Row{
		{Weight: 125, Name: "#135 Tocci, Nico", Class: "SR", Raw: "#135 Tocci, Nico"},
		{Weight: 125, Name: "(Air Force), (Air Force)", Class: "SO", Raw: "(Air Force), (Air Force)"},
		{Weight: 133, Name: "#93 Caprella, Gavin", Class: "SR", Raw: "#93 Caprella, Gavin"},
	}

	res, err := RosterForTeam(ctx, db, "Air Force", 2027, rows, time.Now())
	if err != nil {
		t.Fatalf("RosterForTeam: %v", err)
	}
	if res.EntriesUpserted != 2 {
		t.Errorf("EntriesUpserted = %d, want 2", res.EntriesUpserted)
	}
	if res.PlaceholdersSkipped != 1 {
		t.Errorf("PlaceholdersSkipped = %d, want 1", res.PlaceholdersSkipped)
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM roster_entries`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("roster_entries rows = %d, want 2 (placeholder must not be stored)", count)
	}

	var schoolCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schools WHERE name = 'Air Force'`).Scan(&schoolCount); err != nil {
		t.Fatal(err)
	}
	if schoolCount != 1 {
		t.Errorf("schools rows for Air Force = %d, want 1 (find-or-create)", schoolCount)
	}

	var weight int
	var rawName string
	if err := db.QueryRowContext(ctx,
		`SELECT weight_class, raw_name FROM roster_entries WHERE raw_name = 'Nico Tocci'`).
		Scan(&weight, &rawName); err != nil {
		t.Fatalf("Nico Tocci not stored with cleaned name: %v", err)
	}
	if weight != 125 {
		t.Errorf("Nico Tocci weight = %d, want 125", weight)
	}
}

func TestRosterForTeam_ReScrapeUpdatesWeight(t *testing.T) {
	// A roster is current state, re-scraped periodically — not an immutable
	// dated edition like a ranking snapshot. A wrestler moving weight through
	// the season must UPDATE the existing row, not create a second one
	// (schema.md §8; UNIQUE(school_id, season, raw_name)).
	ctx := context.Background()
	db := storetest.NewDB(t)

	row := wrestlestat.Row{Weight: 125, Name: "Tocci, Nico", Class: "SR", Raw: "Tocci, Nico"}
	if _, err := RosterForTeam(ctx, db, "Air Force", 2027, []wrestlestat.Row{row}, time.Now()); err != nil {
		t.Fatalf("first ingest: %v", err)
	}

	row.Weight = 133 // moved up a weight class
	if _, err := RosterForTeam(ctx, db, "Air Force", 2027, []wrestlestat.Row{row}, time.Now()); err != nil {
		t.Fatalf("second ingest: %v", err)
	}

	var count, weight int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM roster_entries WHERE raw_name = 'Nico Tocci'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("roster_entries rows for Nico Tocci = %d, want 1 (updated, not duplicated)", count)
	}
	if err := db.QueryRowContext(ctx, `SELECT weight_class FROM roster_entries WHERE raw_name = 'Nico Tocci'`).Scan(&weight); err != nil {
		t.Fatal(err)
	}
	if weight != 133 {
		t.Errorf("weight_class after re-scrape = %d, want 133 (updated)", weight)
	}
}
