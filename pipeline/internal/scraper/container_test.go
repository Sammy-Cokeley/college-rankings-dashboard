package scraper

import (
	"os"
	"testing"
)

const fixtureFile = "testdata/ranking_container_14300895.json"

func loadFixture(t *testing.T) Container {
	t.Helper()
	data, err := os.ReadFile(fixtureFile)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	c, err := ParseContainer(data)
	if err != nil {
		t.Fatalf("ParseContainer: %v", err)
	}
	return c
}

func TestParseContainer_Fixture(t *testing.T) {
	c := loadFixture(t)
	if c.ID != 14300895 {
		t.Errorf("container id = %d, want 14300895", c.ID)
	}
	// Fixture is trimmed to sections "2" (125) and "11" (285).
	if _, ok := c.RankingSections["2"]; !ok {
		t.Error("missing section 2")
	}
	if _, ok := c.RankingSections["11"]; !ok {
		t.Error("missing section 11")
	}
}

func TestWeightEditions_FlattensAndOrders(t *testing.T) {
	c := loadFixture(t)
	we := c.WeightEditions()

	// 2 weights x 22 editions.
	if len(we) != 44 {
		t.Fatalf("got %d weight-editions, want 44", len(we))
	}

	// Ordered by weight then date: first is 125, last is 285.
	if we[0].WeightClass != 125 {
		t.Errorf("first weight = %d, want 125", we[0].WeightClass)
	}
	if we[len(we)-1].WeightClass != 285 {
		t.Errorf("last weight = %d, want 285", we[len(we)-1].WeightClass)
	}

	// Dates ascend within a weight.
	for i := 1; i < len(we); i++ {
		if we[i].WeightClass == we[i-1].WeightClass &&
			we[i].Edition.PublishDate < we[i-1].Edition.PublishDate {
			t.Errorf("editions out of date order at %d: %s before %s",
				i, we[i-1].Edition.PublishDate, we[i].Edition.PublishDate)
		}
	}
}

func TestWeightEditions_SkipsNonWeightSections(t *testing.T) {
	c := Container{
		ID: 1,
		RankingSections: map[string][]Edition{
			"1":  {{PublishDate: "2025-06-19"}}, // P4P
			"2":  {{PublishDate: "2025-06-19"}}, // 125
			"12": {{PublishDate: "2025-06-19"}}, // Team Tournament
			"99": {{PublishDate: "2025-06-19"}}, // unknown
		},
	}
	we := c.WeightEditions()
	if len(we) != 1 || we[0].WeightClass != 125 {
		t.Fatalf("got %+v, want only the 125 section", we)
	}
}

// Every edition in the fixture must parse into ranked rows. This is the
// table-shape audit as an executable guard: both the 4-col preseason shape and
// the 5-col in-season shape, and the 24->33 row-count change, must parse.
func TestParseTable_AllFixtureEditions(t *testing.T) {
	c := loadFixture(t)
	for _, we := range c.WeightEditions() {
		rows, err := ParseTable(we.Edition.Content)
		if err != nil {
			t.Fatalf("weight %d %s: %v", we.WeightClass, we.Edition.PublishDate, err)
		}
		if len(rows) == 0 {
			t.Errorf("weight %d %s: no rows", we.WeightClass, we.Edition.PublishDate)
		}
		// Ranks are contiguous from 1.
		for i, r := range rows {
			if r.Rank != i+1 {
				t.Errorf("weight %d %s: row %d has rank %d", we.WeightClass, we.Edition.PublishDate, i, r.Rank)
			}
			if r.Name == "" || r.School == "" {
				t.Errorf("weight %d %s rank %d: empty name/school", we.WeightClass, we.Edition.PublishDate, r.Rank)
			}
		}
	}
}

func TestParseContainer_Empty(t *testing.T) {
	if _, err := ParseContainer([]byte(`{}`)); err == nil {
		t.Fatal("expected error for container without ranking_sections")
	}
}
