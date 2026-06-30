// Package scraper turns a FloWrestling rankings page into typed weekly ranking
// editions. It is pure: it fetches/decodes/parses but never touches the
// database. The flow is page HTML -> flo-app-state blob (decode.go) -> Container
// -> per-edition wrestler rows (table.go).
package scraper

import (
	"encoding/json"
	"fmt"
	"sort"
)

// Container is FloWrestling's season "ranking container" (the value at
// .body.data of the transfer-state blob). One container holds every weight's
// full weekly edition history. The committed test fixture is a trimmed,
// already-decoded container that unmarshals directly into this type.
type Container struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	// RankingSections is keyed "1".."12"; see sectionWeights. Each value is the
	// weight's list of weekly editions.
	RankingSections map[string][]Edition `json:"ranking_sections"`
}

// Edition is one dated weekly ranking for one weight. Content is an HTML
// <table> of the ranked wrestlers, parsed by ParseTable.
type Edition struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	PublishDate string `json:"publish_date"`
	Content     string `json:"content"`
}

// sectionWeights maps the ranking_sections keys to NCAA DI weight classes. Keys
// "1" (Pound-For-Pound) and "12" (Team Tournament) are intentionally absent:
// they rank a different entity than a wrestler-at-weight and are out of v0
// scope (docs/sources/flowrestling.md). The section key is authoritative for
// weight — more reliable than the edition's free-text Name field.
var sectionWeights = map[string]int{
	"2":  125,
	"3":  133,
	"4":  141,
	"5":  149,
	"6":  157,
	"7":  165,
	"8":  174,
	"9":  184,
	"10": 197,
	"11": 285,
}

// WeightEdition is one edition tagged with its resolved weight class.
type WeightEdition struct {
	WeightClass int
	Edition     Edition
}

// WeightEditions flattens the container's weight sections into a flat,
// deterministically ordered list (by weight, then publish date). P4P and Team
// Tournament sections are skipped. Unknown section keys are ignored rather than
// erroring, so a future Flo section addition won't break ingestion.
func (c Container) WeightEditions() []WeightEdition {
	var out []WeightEdition
	for key, editions := range c.RankingSections {
		weight, ok := sectionWeights[key]
		if !ok {
			continue
		}
		for _, e := range editions {
			out = append(out, WeightEdition{WeightClass: weight, Edition: e})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].WeightClass != out[j].WeightClass {
			return out[i].WeightClass < out[j].WeightClass
		}
		return out[i].Edition.PublishDate < out[j].Edition.PublishDate
	})
	return out
}

// ParseContainer unmarshals an already-decoded container JSON document (the
// shape of the test fixture: top-level object whose keys include
// ranking_sections). For the live page->container path, see DecodeAppState.
func ParseContainer(data []byte) (Container, error) {
	var c Container
	if err := json.Unmarshal(data, &c); err != nil {
		return Container{}, fmt.Errorf("unmarshal container: %w", err)
	}
	if len(c.RankingSections) == 0 {
		return Container{}, fmt.Errorf("container has no ranking_sections")
	}
	return c, nil
}
