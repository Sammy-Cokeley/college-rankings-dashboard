package resolve

// canonicalSchools collapses confirmed spelling/abbreviation variants of the
// SAME school onto one canonical form, so a wrestler published under two
// spellings resolves to one identity instead of a false fragment. Keys and
// values are in normalizeToken form (lowercased, punctuation-stripped,
// whitespace-collapsed) and matched on the WHOLE school string — never a
// substring — so "penn" collapses without touching "penn state".
//
// Curated conservatively from the 10-weight Flo validation corpus, where the
// only false split found was Evan Mougalian, published as both "Penn" and
// "Pennsylvania" (docs/sources/flowrestling-validation.md). The five genuine
// transfers in that corpus are different schools entirely and are intentionally
// absent, so they stay fragmented into two identities each.
//
// NOTE: this is a single-source (FloWrestling) map. A second source (InterMat,
// deferred) will bring its own school spellings; this table will need to grow —
// and at that point a proper schools/school_aliases dimension keyed on a
// school_id is likely the better home than an in-code map.
var canonicalSchools = map[string]string{
	"penn": "pennsylvania", // U. of Pennsylvania; distinct from "penn state".
}

// canonicalSchool returns the canonical form of a normalized school string, or
// the input unchanged when it has no known variant.
func canonicalSchool(normalized string) string {
	if c, ok := canonicalSchools[normalized]; ok {
		return c
	}
	return normalized
}
