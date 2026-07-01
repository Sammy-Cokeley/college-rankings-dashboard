// Package resolve is the second pass: it reconciles each source's raw
// name+school strings to a canonical wrestler identity. Ingestion never blocks
// on this — entries land with a NULL wrestler_id and are resolved here, so the
// pass can be re-run safely against the same data at any time.
//
// v0 strategy (single source, highly consistent strings): two tiers only —
//
//  1. exact alias hit on (source, raw_name, raw_school);
//  2. normalized-key match (case/whitespace/punctuation-insensitive).
//
// Identity is (name, canonical school): the school is normalized and then mapped
// through a curated variant table (schools.go) so confirmed spellings of one
// school collapse. There is intentionally no fuzzy matching and no name-only
// cross-school merge, so two distinct wrestlers are never silently merged; the
// cost is that a mid-season transfer fragments into two identities until a later
// merge (transfer feed / manual review) attaches the second school string as an
// alias of the first wrestler.
package resolve

import (
	"context"
	"database/sql"
	"fmt"

	"pipeline/internal/store"
)

// Result summarizes a resolution pass.
type Result struct {
	EntriesResolved  int // entries given a wrestler_id this run
	WrestlersCreated int // new canonical identities created this run
	AliasesCreated   int // new raw-string aliases recorded this run
}

// Source resolves every unresolved entry for the named source in one
// transaction. It seeds an in-memory identity index from existing aliases (so
// identities are reused across runs and across weight classes), then for each
// unresolved entry: reuses an exact alias, else matches the normalized key to a
// known identity (recording the new raw string as an alias), else mints a new
// wrestler. Entries that resolve get their wrestler_id set; nothing is left
// ambiguous in this single-source design, but any future no-match path simply
// leaves the entry NULL for a later pass.
func Source(ctx context.Context, db *sql.DB, sourceName string) (Result, error) {
	sourceID, err := store.SourceID(ctx, db, sourceName)
	if err != nil {
		return Result{}, err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()

	idx, err := newIndex(ctx, tx, sourceID)
	if err != nil {
		return Result{}, err
	}

	unresolved, err := store.ListUnresolvedEntries(ctx, tx, sourceID)
	if err != nil {
		return Result{}, err
	}

	var res Result
	for _, e := range unresolved {
		wrestlerID, err := idx.resolve(ctx, tx, sourceID, e)
		if err != nil {
			return Result{}, err
		}
		if err := store.SetEntryWrestler(ctx, tx, e.EntryID, wrestlerID); err != nil {
			return Result{}, err
		}
		res.EntriesResolved++
	}
	res.WrestlersCreated = idx.wrestlersCreated
	res.AliasesCreated = idx.aliasesCreated

	if err := tx.Commit(); err != nil {
		return Result{}, fmt.Errorf("commit resolution: %w", err)
	}
	return res, nil
}

// index is the in-memory identity map for one source, built from existing
// aliases and grown as new identities/aliases are created during the pass.
type index struct {
	exact      map[string]int64 // raw_name \x00 raw_school -> wrestler_id
	normalized map[string]int64 // normalizeKey(name, school) -> wrestler_id

	wrestlersCreated int
	aliasesCreated   int
}

func newIndex(ctx context.Context, q store.DBTX, sourceID int64) (*index, error) {
	aliases, err := store.ListAliases(ctx, q, sourceID)
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

// resolve returns the canonical wrestler id for an entry, creating the wrestler
// and/or alias as needed and keeping the in-memory index in sync.
func (idx *index) resolve(ctx context.Context, q store.DBTX, sourceID int64, e store.UnresolvedEntry) (int64, error) {
	ek := exactKey(e.RawName, e.RawSchool)
	if id, ok := idx.exact[ek]; ok {
		return id, nil // tier 1: known raw string
	}

	nk := normalizeKey(e.RawName, e.RawSchool)
	if id, ok := idx.normalized[nk]; ok {
		// tier 2: same identity, new raw spelling -> record the alias so the
		// next run hits tier 1.
		if err := idx.addAlias(ctx, q, id, sourceID, e); err != nil {
			return 0, err
		}
		idx.exact[ek] = id
		return id, nil
	}

	// new identity: create the wrestler and its first alias.
	grade := ""
	if e.RawGrade != nil {
		grade = *e.RawGrade
	}
	id, err := store.InsertWrestler(ctx, q, e.RawName, grade)
	if err != nil {
		return 0, err
	}
	idx.wrestlersCreated++
	if err := idx.addAlias(ctx, q, id, sourceID, e); err != nil {
		return 0, err
	}
	idx.exact[ek] = id
	idx.normalized[nk] = id
	return id, nil
}

func (idx *index) addAlias(ctx context.Context, q store.DBTX, wrestlerID, sourceID int64, e store.UnresolvedEntry) error {
	if err := store.InsertAlias(ctx, q, wrestlerID, sourceID, e.RawName, e.RawSchool); err != nil {
		return err
	}
	idx.aliasesCreated++
	return nil
}

// exactKey is the literal raw-string key (NUL-joined to avoid name/school
// boundary collisions); a nil school is distinct from an empty one only in that
// both map to the empty segment, which is acceptable for v0.
func exactKey(name string, school *string) string {
	s := ""
	if school != nil {
		s = *school
	}
	return name + "\x00" + s
}
