// Package aggregate turns individual Fan Poll ballots into one published
// "Fan Poll" ranking per weight class — the only place user ballots are
// combined. Individual ballots are never stored as ranking_entries
// themselves (schema.md §6/§8: no stored composite); only this derived,
// periodic snapshot is.
package aggregate

import "sort"

// WeightClasses are the ten NCAA DI weight classes this project tracks.
// Mirrors web/utils/weights.ts's WEIGHT_CLASSES.
var WeightClasses = [10]int{125, 133, 141, 149, 157, 165, 174, 184, 197, 285}

// BallotPick is one (ballot, rank, wrestler) row feeding the scorer — a
// flattened join of ballot_entries -> wrestlers for one weight+season.
type BallotPick struct {
	BallotID   int64
	Rank       int // 1..33, as constrained by ballot_entries.rank
	WrestlerID int64
	FullName   string
}

// ScoredWrestler is one wrestler's aggregated standing for a weight class.
type ScoredWrestler struct {
	WrestlerID int64
	FullName   string
	Points     int
	Rank       int // competition ranking (ties share a rank; the next distinct value skips ahead — e.g. 1,1,3), not dense ranking: this is a fresh ranking we mint, not a third-party's published one, so there's no "never renumber" constraint to preserve (schema.md §7) — this is simply the clearest convention for a top-N leaderboard.
}

// maxSlots is the ballot size (top 33) and therefore the published poll's
// size too — the same "33" the ballot_entries.rank CHECK constraint enforces.
const maxSlots = 33

// ScoreWeight aggregates every pick for one weight class into a ranked,
// published-size list: points = 34 - rank per pick (33 for a #1 vote, 1 for a
// #33 vote), summed per wrestler, ranked by total points, truncated to the
// top 33. Ties are stored verbatim — two wrestlers tied for the last spot
// both appear or both don't; never split by an arbitrary tiebreaker.
func ScoreWeight(picks []BallotPick) []ScoredWrestler {
	type accum struct {
		fullName string
		points   int
	}
	byWrestler := make(map[int64]*accum)
	var order []int64 // first-seen order, for a deterministic base before sorting
	for _, p := range picks {
		a, ok := byWrestler[p.WrestlerID]
		if !ok {
			a = &accum{fullName: p.FullName}
			byWrestler[p.WrestlerID] = a
			order = append(order, p.WrestlerID)
		}
		a.points += (maxSlots + 1) - p.Rank
	}

	scored := make([]ScoredWrestler, 0, len(order))
	for _, id := range order {
		a := byWrestler[id]
		scored = append(scored, ScoredWrestler{WrestlerID: id, FullName: a.fullName, Points: a.points})
	}

	// Points desc, then name asc — deterministic ordering among ties so the
	// truncation point and display order never depend on map/join iteration
	// order.
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Points != scored[j].Points {
			return scored[i].Points > scored[j].Points
		}
		return scored[i].FullName < scored[j].FullName
	})

	if len(scored) > maxSlots {
		scored = scored[:maxSlots]
	}

	rank := 1
	for i := range scored {
		if i > 0 && scored[i].Points != scored[i-1].Points {
			rank = i + 1
		}
		scored[i].Rank = rank
	}
	return scored
}

// DistinctBallots counts the distinct ballots contributing to picks — the
// "how many people actually voted" figure the publish threshold checks
// against (a ballot with zero entries contributes no picks and so doesn't
// count, without needing a separate query).
func DistinctBallots(picks []BallotPick) int {
	seen := make(map[int64]struct{}, len(picks))
	for _, p := range picks {
		seen[p.BallotID] = struct{}{}
	}
	return len(seen)
}
