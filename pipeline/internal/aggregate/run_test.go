package aggregate

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"pipeline/internal/storetest"
)

const season = 2027

// mustUser/mustWrestler/mustBallot are raw-SQL fixture builders: ballots are
// entirely web-owned (pipeline/internal/store has no Go-side ballot helpers
// by design — the aggregation job only ever reads them), so tests seed the
// same way the real web app's schema expects, by hand.
func mustUser(t *testing.T, ctx context.Context, db *sql.DB, email string) int64 {
	t.Helper()
	var id int64
	err := db.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash, created_at) VALUES ($1, 'x', $2) RETURNING id`,
		email, time.Now().UTC().Format(time.RFC3339)).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustWrestler(t *testing.T, ctx context.Context, db *sql.DB, name string) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRowContext(ctx,
		`INSERT INTO wrestlers (full_name) VALUES ($1) RETURNING id`, name).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// mustBallot creates a ballot for userID at weight/season with the given
// wrestlers in rank order (index 0 = rank 1).
func mustBallot(t *testing.T, ctx context.Context, db *sql.DB, userID int64, weight int, wrestlerIDs ...int64) {
	t.Helper()
	var ballotID int64
	err := db.QueryRowContext(ctx,
		`INSERT INTO ballots (user_id, weight_class, season, updated_at) VALUES ($1, $2, $3, $4) RETURNING id`,
		userID, weight, season, time.Now().UTC().Format(time.RFC3339)).Scan(&ballotID)
	if err != nil {
		t.Fatal(err)
	}
	for i, wid := range wrestlerIDs {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO ballot_entries (ballot_id, rank, wrestler_id) VALUES ($1, $2, $3)`,
			ballotID, i+1, wid); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRun_PublishesAboveThresholdSkipsBelow(t *testing.T) {
	ctx := context.Background()
	db := storetest.NewDB(t)

	alice := mustWrestler(t, ctx, db, "Alice")
	bob := mustWrestler(t, ctx, db, "Bob")

	// 125: three ballots (meets a threshold of 3).
	mustBallot(t, ctx, db, mustUser(t, ctx, db, "a@example.com"), 125, alice, bob)
	mustBallot(t, ctx, db, mustUser(t, ctx, db, "b@example.com"), 125, bob, alice)
	mustBallot(t, ctx, db, mustUser(t, ctx, db, "c@example.com"), 125, alice, bob)
	// 133: one ballot only (below the threshold of 3).
	mustBallot(t, ctx, db, mustUser(t, ctx, db, "d@example.com"), 133, alice)

	res, err := Run(ctx, db, season, 3, time.Now())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(res.WeightsPublished) != 1 || res.WeightsPublished[0] != 125 {
		t.Errorf("WeightsPublished = %v, want [125]", res.WeightsPublished)
	}
	if len(res.WeightsSkipped) != 1 || res.WeightsSkipped[0].WeightClass != 133 {
		t.Errorf("WeightsSkipped = %+v, want one entry for 133", res.WeightsSkipped)
	}

	// The published weight must land as an ordinary snapshot/ranking_entries
	// row set under the Fan Poll source — that's the whole point (reuse the
	// existing display stack for free).
	var sourceName, rawName string
	var rank int
	err = db.QueryRowContext(ctx, `
SELECT s.name, e.rank, e.raw_source_string
FROM ranking_entries e
JOIN snapshots s2 ON s2.id = e.snapshot_id
JOIN sources s ON s.id = s2.source_id
WHERE s2.weight_class = 125 AND e.rank = 1`).Scan(&sourceName, &rank, &rawName)
	if err != nil {
		t.Fatalf("published entry not found: %v", err)
	}
	if sourceName != "Fan Poll" {
		t.Errorf("source = %q, want Fan Poll", sourceName)
	}
	if rawName != "Alice" {
		t.Errorf("rank 1 = %q, want Alice (2 first-place votes to Bob's 1)", rawName)
	}
}

func TestRun_IdempotentSameDay(t *testing.T) {
	ctx := context.Background()
	db := storetest.NewDB(t)

	alice := mustWrestler(t, ctx, db, "Alice")
	for _, email := range []string{"a@example.com", "b@example.com", "c@example.com"} {
		mustBallot(t, ctx, db, mustUser(t, ctx, db, email), 149, alice)
	}

	now := time.Now()
	if _, err := Run(ctx, db, season, 3, now); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if _, err := Run(ctx, db, season, 3, now); err != nil {
		t.Fatalf("second run: %v", err)
	}

	var count int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM snapshots WHERE weight_class = 149`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("snapshots for 149 = %d, want 1 (same published_date, idempotent)", count)
	}
}

func TestCurrentBallotSeason(t *testing.T) {
	ctx := context.Background()
	db := storetest.NewDB(t)

	got, err := CurrentBallotSeason(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("with no ballots, want nil, got %v", *got)
	}

	alice := mustWrestler(t, ctx, db, "Alice")
	mustBallot(t, ctx, db, mustUser(t, ctx, db, "a@example.com"), 125, alice)

	got, err = CurrentBallotSeason(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || *got != season {
		t.Errorf("got %v, want %d", got, season)
	}
}
