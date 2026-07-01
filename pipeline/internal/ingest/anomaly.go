package ingest

import (
	"fmt"
	"sort"

	"pipeline/internal/scraper"
)

// EditionAnomaly records a non-fatal structural oddity in an edition that still
// ingested cleanly: something worth a human eyeballing (typically a hand-entry
// typo in Flo's tables) but never a reason to reject the edition or fail the
// run. It is deliberately distinct from EditionFailure, which is fatal and exits
// non-zero. One record per anomalous edition; Issues bundles every oddity found
// in that edition, so an edition that trips several checks is still one record.
type EditionAnomaly struct {
	WeightClass   int
	PublishedDate string
	Issues        []string
}

func (a EditionAnomaly) String() string {
	return fmt.Sprintf("weight %d %s: %v", a.WeightClass, a.PublishedDate, a.Issues)
}

// detectAnomalies scans an edition's parsed rows for non-fatal rank oddities and
// returns a human-readable description of each, or nil when the sequence is
// clean. Two checks, both against the published ranks only:
//
//   - duplicate ranks — two rows share a rank (a tie; e.g. 197 2026-03-27, where
//     Flo hand-entered two wrestlers at 21). The schema stores ties verbatim
//     (schema.md §7); this just makes them visible on a run.
//   - rank gaps — a rank in [1..max] with no row (e.g. the same edition skips 23).
//
// Both are surfaced, never corrected: the rule is to store what the source
// published and flag it, not to silently renumber.
func detectAnomalies(rows []scraper.Row) []string {
	counts := make(map[int]int, len(rows))
	maxRank := 0
	for _, r := range rows {
		counts[r.Rank]++
		if r.Rank > maxRank {
			maxRank = r.Rank
		}
	}

	var dups []int
	for rank, n := range counts {
		if n > 1 {
			dups = append(dups, rank)
		}
	}
	sort.Ints(dups)

	var gaps []int
	for rank := 1; rank <= maxRank; rank++ {
		if counts[rank] == 0 {
			gaps = append(gaps, rank)
		}
	}

	var issues []string
	for _, rank := range dups {
		issues = append(issues, fmt.Sprintf("duplicate rank %d (%d rows)", rank, counts[rank]))
	}
	if len(gaps) > 0 {
		issues = append(issues, fmt.Sprintf("rank gap: missing %v", gaps))
	}
	return issues
}
