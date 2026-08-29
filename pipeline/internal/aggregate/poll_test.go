package aggregate

import (
	"reflect"
	"testing"
)

func TestScoreWeight_BasicPoints(t *testing.T) {
	// #1 vote = 33 points, #33 vote = 1 point.
	picks := []BallotPick{
		{BallotID: 1, Rank: 1, WrestlerID: 10, FullName: "Alice"},
		{BallotID: 1, Rank: 33, WrestlerID: 20, FullName: "Bob"},
	}
	got := ScoreWeight(picks)
	want := []ScoredWrestler{
		{WrestlerID: 10, FullName: "Alice", Points: 33, Rank: 1},
		{WrestlerID: 20, FullName: "Bob", Points: 1, Rank: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ScoreWeight = %+v, want %+v", got, want)
	}
}

func TestScoreWeight_SumsAcrossBallots(t *testing.T) {
	picks := []BallotPick{
		{BallotID: 1, Rank: 1, WrestlerID: 10, FullName: "Alice"}, // 33
		{BallotID: 2, Rank: 2, WrestlerID: 10, FullName: "Alice"}, // 32
		{BallotID: 3, Rank: 1, WrestlerID: 20, FullName: "Bob"},   // 33
	}
	got := ScoreWeight(picks)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	// Alice: 33+32=65, beats Bob's 33.
	if got[0].WrestlerID != 10 || got[0].Points != 65 || got[0].Rank != 1 {
		t.Errorf("got[0] = %+v", got[0])
	}
	if got[1].WrestlerID != 20 || got[1].Points != 33 || got[1].Rank != 2 {
		t.Errorf("got[1] = %+v", got[1])
	}
}

func TestScoreWeight_TiesShareRankAndSkipTheNext(t *testing.T) {
	// Alice and Bob both get exactly 33 points (one #1 vote each); Cara gets
	// 32. Competition ranking: 1, 1, 3 — not 1, 1, 2 (dense) and not silently
	// broken by an arbitrary tiebreaker.
	picks := []BallotPick{
		{BallotID: 1, Rank: 1, WrestlerID: 10, FullName: "Bob"},
		{BallotID: 2, Rank: 1, WrestlerID: 20, FullName: "Alice"},
		{BallotID: 3, Rank: 2, WrestlerID: 30, FullName: "Cara"},
	}
	got := ScoreWeight(picks)
	ranks := make([]int, len(got))
	for i, s := range got {
		ranks[i] = s.Rank
	}
	if !reflect.DeepEqual(ranks, []int{1, 1, 3}) {
		t.Errorf("ranks = %v, want [1 1 3]", ranks)
	}
	// Deterministic tiebreak order: alphabetical by name among equal points.
	if got[0].FullName != "Alice" || got[1].FullName != "Bob" {
		t.Errorf("tie order = [%s, %s], want [Alice, Bob]", got[0].FullName, got[1].FullName)
	}
}

func TestScoreWeight_TruncatesToTop33(t *testing.T) {
	var picks []BallotPick
	// 40 distinct wrestlers, each with a single #1..#40 vote is impossible
	// (rank is capped at 33) — instead give 40 wrestlers descending points via
	// 40 separate ballots each voting a different wrestler at rank 1..33,
	// cycling ranks so points still vary and produce 40 distinct totals is
	// overkill; simpler: 40 wrestlers, each picked at rank i%33+1 by a unique
	// ballot, giving each at least one non-zero, mostly-distinct point total.
	for i := range 40 {
		picks = append(picks, BallotPick{
			BallotID: int64(i), Rank: (i % 33) + 1, WrestlerID: int64(i), FullName: "W",
		})
	}
	got := ScoreWeight(picks)
	if len(got) != 33 {
		t.Errorf("len = %d, want 33 (truncated)", len(got))
	}
}

func TestDistinctBallots(t *testing.T) {
	picks := []BallotPick{
		{BallotID: 1, Rank: 1, WrestlerID: 10},
		{BallotID: 1, Rank: 2, WrestlerID: 20},
		{BallotID: 2, Rank: 1, WrestlerID: 20},
	}
	if got := DistinctBallots(picks); got != 2 {
		t.Errorf("DistinctBallots = %d, want 2", got)
	}
	if got := DistinctBallots(nil); got != 0 {
		t.Errorf("DistinctBallots(nil) = %d, want 0", got)
	}
}
