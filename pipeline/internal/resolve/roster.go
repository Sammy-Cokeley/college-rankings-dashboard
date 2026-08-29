package resolve

import (
	"context"
	"database/sql"
	"fmt"

	"pipeline/internal/store"
)

// Roster resolves every unresolved roster_entries row for the given season
// against the GLOBAL wrestler identity pool (every source's aliases seed the
// matcher — see newGlobalIndex), not just aliases already recorded under
// "WrestleStat" the way Source() scopes to one ranking source.
//
// The distinction matters: a roster entry has never been "published" under
// any source before it's first seen (it's WrestleStat's own listing, not a
// republish of someone else's), so an exact alias hit essentially never fires
// on a first run. Tier-2 normalized-key matching against every already-known
// identity — most usefully FloWrestling's — is what lets "WrestleStat: Nico
// Tocci, Air Force" resolve to the same canonical wrestler as "Flo: Nico
// Tocci, Air Force" instead of minting a duplicate.
func Roster(ctx context.Context, db *sql.DB, sourceName string, season int) (Result, error) {
	sourceID, err := store.SourceID(ctx, db, sourceName)
	if err != nil {
		return Result{}, err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()

	idx, err := newGlobalIndex(ctx, tx)
	if err != nil {
		return Result{}, err
	}

	unresolved, err := store.ListUnresolvedRosterEntries(ctx, tx, season)
	if err != nil {
		return Result{}, err
	}

	var res Result
	for _, e := range unresolved {
		wrestlerID, err := idx.resolve(ctx, tx, sourceID, e)
		if err != nil {
			return Result{}, err
		}
		if err := store.SetRosterEntryWrestler(ctx, tx, e.EntryID, wrestlerID); err != nil {
			return Result{}, err
		}
		res.EntriesResolved++
	}
	res.WrestlersCreated = idx.wrestlersCreated
	res.AliasesCreated = idx.aliasesCreated

	if err := tx.Commit(); err != nil {
		return Result{}, fmt.Errorf("commit roster resolution: %w", err)
	}
	return res, nil
}

// newGlobalIndex is newIndex without the source_id scope — see Roster's doc
// comment for why cross-source seeding is required here specifically.
func newGlobalIndex(ctx context.Context, q store.DBTX) (*index, error) {
	aliases, err := store.ListAllAliases(ctx, q)
	if err != nil {
		return nil, err
	}
	idx := &index{
		exact:      make(map[string]int64, len(aliases)),
		normalized: make(map[string]int64, len(aliases)),
	}
	for _, a := range aliases {
		idx.exact[exactKey(a.RawName, a.RawSchool)] = a.WrestlerID
		idx.normalized[normalizeKey(a.RawName, a.RawSchool)] = a.WrestlerID
	}
	return idx, nil
}
