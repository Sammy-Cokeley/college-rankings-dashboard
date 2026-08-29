package ingest

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"pipeline/internal/scraper/wrestlestat"
	"pipeline/internal/store"
)

const rosterSourceName = "WrestleStat"

// TeamRosterResult summarizes ingesting one team's roster.
type TeamRosterResult struct {
	EntriesUpserted     int
	PlaceholdersSkipped int // rows detected as an unfilled roster slot, not a real wrestler
}

// RosterForTeam writes one team's parsed roster rows into roster_entries,
// resolving (find-or-create) the team's canonical school first. wrestler_id
// is left NULL — resolution is a separate pass (resolve.Roster), matching the
// project's ingest-first-resolve-second rule.
//
// A placeholder row (an unfilled roster slot, published as the school's own
// name — see docs/sources/wrestlestat.md) is detected and skipped, not
// stored: it isn't a real wrestler, and storing it would eventually mint a
// bogus canonical identity nobody's ballot could ever legitimately pick.
func RosterForTeam(ctx context.Context, db *sql.DB, teamName string, season int, rows []wrestlestat.Row, capturedAt time.Time) (TeamRosterResult, error) {
	sourceID, err := store.SourceID(ctx, db, rosterSourceName)
	if err != nil {
		return TeamRosterResult{}, err
	}
	captured := capturedAt.UTC().Format(time.RFC3339)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return TeamRosterResult{}, err
	}
	defer tx.Rollback()

	schoolID, err := store.FindOrCreateSchool(ctx, tx, sourceID, teamName)
	if err != nil {
		return TeamRosterResult{}, fmt.Errorf("resolve school %q: %w", teamName, err)
	}

	var res TeamRosterResult
	for _, r := range rows {
		if isPlaceholderSlot(r.Name) {
			res.PlaceholdersSkipped++
			continue
		}

		weight := r.Weight
		if _, err := store.UpsertRosterEntry(ctx, tx, store.RosterEntry{
			SchoolID:        schoolID,
			Season:          season,
			WeightClass:     &weight,
			RawName:         cleanName(r.Name),
			RawSourceString: r.Raw,
			CapturedAt:      captured,
		}); err != nil {
			return TeamRosterResult{}, fmt.Errorf("upsert %q: %w", r.Name, err)
		}
		res.EntriesUpserted++
	}

	if err := tx.Commit(); err != nil {
		return TeamRosterResult{}, fmt.Errorf("commit roster for %q: %w", teamName, err)
	}
	return res, nil
}

// isPlaceholderSlot reports whether a roster row is an unfilled slot rather
// than a real wrestler. Observed on live pages (docs/sources/wrestlestat.md)
// as the school's own name, published twice, each half wrapped in parens:
// "(Air Force), (Air Force)".
//
// Structural, not a match against the team's own dropdown name: WrestleStat's
// placeholder text embeds a DIFFERENT (longer, formal) school string than the
// one in the team-select dropdown ingestion already has as teamName — "NC
// State" (dropdown) vs "North Carolina State" (placeholder), "Army West
// Point" vs "Army", "SIU Edwardsville" vs "Southern Illinois Edwardsville",
// confirmed live. An exact-match-against-teamName check (the original
// version of this function) silently missed every one of these, letting
// three-plus placeholder "wrestlers" reach the ballot picker in production
// before this was caught. Checking that both comma-separated, paren-wrapped
// halves equal EACH OTHER sidesteps the whole problem — it needs no school
// name at all, so a dropdown/placeholder spelling mismatch can't hide it.
func isPlaceholderSlot(publishedName string) bool {
	halves := strings.SplitN(publishedName, ",", 2)
	if len(halves) != 2 {
		return false
	}
	parenthesized := func(s string) bool {
		s = strings.TrimSpace(s)
		return strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")")
	}
	clean := func(s string) string {
		return strings.ToLower(strings.Trim(strings.TrimSpace(s), "()"))
	}
	if !parenthesized(halves[0]) || !parenthesized(halves[1]) {
		return false
	}
	a, b := clean(halves[0]), clean(halves[1])
	return a != "" && a == b
}

// cleanName converts WrestleStat's roster name format ("#135 Tocci, Nico" or
// "Tocci, Nico") into "First Last" order, matching the word-order convention
// FloWrestling's raw_source_string already uses. This matters:
// resolve/normalize.go's normalizeToken does NOT reorder tokens, so without
// this conversion "tocci nico" and "nico tocci" would never match as the same
// person across sources — every WrestleStat wrestler would fragment into a
// second identity instead of resolving onto Flo's existing one.
func cleanName(published string) string {
	s := strings.TrimSpace(published)
	if strings.HasPrefix(s, "#") {
		if sp := strings.IndexByte(s, ' '); sp > 0 {
			if _, err := strconv.Atoi(s[1:sp]); err == nil {
				s = strings.TrimSpace(s[sp+1:])
			}
		}
	}
	last, first, ok := strings.Cut(s, ",")
	if !ok {
		return s // not "Last, First" shaped; leave as published
	}
	return strings.TrimSpace(first) + " " + strings.TrimSpace(last)
}
