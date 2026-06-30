package resolve

import (
	"strings"
	"unicode"
)

// normalizeKey builds the comparison key for entity matching from a raw name
// and raw school. Two raw strings that differ only in case, surrounding/internal
// whitespace, or punctuation collapse to the same key — enough to absorb
// week-to-week editorial drift within a single source. It deliberately does NOT
// canonicalize school abbreviations or do fuzzy matching: identity is (name,
// school), so a genuine school change stays a distinct key (a transfer
// fragments into two identities until merged later).
func normalizeKey(name string, school *string) string {
	var s string
	if school != nil {
		s = *school
	}
	return normalizeToken(name) + "\x00" + normalizeToken(s)
}

// normalizeToken lowercases, removes punctuation (keeping alphanumerics and
// spaces), and collapses whitespace. ASCII-only is sufficient for the v0 Flo
// corpus (no accented characters present); a diacritic fold can be added here
// if a future source introduces them, without changing callers.
func normalizeToken(s string) string {
	s = strings.ToLower(s)

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			// Everything else (whitespace and punctuation alike) becomes a
			// separator, then runs collapse below. So "Marc-Anthony" and
			// "Marc Anthony" agree, as do "Castro-Sandoval"/"Castro Sandoval"
			// and "St. John's"/"St Johns" — the hyphen-vs-space drift that is
			// most likely between hand-entered editorial names.
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
