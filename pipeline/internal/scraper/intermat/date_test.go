package intermat

import "testing"

func TestResolveDate(t *testing.T) {
	got, err := ResolveDate("20260106175039")
	if err != nil {
		t.Fatalf("ResolveDate: %v", err)
	}
	if got != "2026-01-06" {
		t.Errorf("ResolveDate = %q, want %q", got, "2026-01-06")
	}
}

func TestResolveDate_Malformed(t *testing.T) {
	if _, err := ResolveDate("not-a-timestamp"); err == nil {
		t.Fatal("expected error for a malformed timestamp")
	}
}

func TestSeasonFromDate(t *testing.T) {
	cases := []struct {
		date string
		want int
	}{
		{"2026-09-17", 2027}, // preseason, real (r78, fetched live 2026-09-19)
		{"2025-08-01", 2026}, // first day of August still counts
		{"2025-07-31", 2025}, // last day of July is still the prior season
		{"2026-01-06", 2026}, // mid-season
		{"2026-03-24", 2026}, // postseason
	}
	for _, c := range cases {
		got, err := SeasonFromDate(c.date)
		if err != nil {
			t.Fatalf("SeasonFromDate(%q): %v", c.date, err)
		}
		if got != c.want {
			t.Errorf("SeasonFromDate(%q) = %d, want %d", c.date, got, c.want)
		}
	}
}

func TestSeasonFromDate_Malformed(t *testing.T) {
	if _, err := SeasonFromDate("not-a-date"); err == nil {
		t.Fatal("expected error for a malformed date")
	}
}
