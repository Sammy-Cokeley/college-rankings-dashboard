package resolve

import "testing"

func TestCanonicalSchool_CollapsesVariant(t *testing.T) {
	// The known Flo variant: both spellings key to one canonical school.
	if got := canonicalSchool("penn"); got != "pennsylvania" {
		t.Errorf("canonicalSchool(penn) = %q, want pennsylvania", got)
	}
	if got := canonicalSchool("pennsylvania"); got != "pennsylvania" {
		t.Errorf("canonicalSchool(pennsylvania) = %q, want pennsylvania", got)
	}
	// Whole-string match only: a different school that merely starts with "penn"
	// must be left alone — never merged.
	if got := canonicalSchool("penn state"); got != "penn state" {
		t.Errorf("canonicalSchool(penn state) = %q, want penn state (unchanged)", got)
	}
}

// The false split fix, at the key level: Mougalian's two published schools must
// now produce one identity key, while Penn State stays distinct from Penn.
func TestNormalizeKey_CollapsesSchoolVariant(t *testing.T) {
	if normalizeKey("Evan Mougalian", ptr("Penn")) != normalizeKey("Evan Mougalian", ptr("Pennsylvania")) {
		t.Error("Penn/Pennsylvania should collapse to one key for the same wrestler")
	}
	if normalizeKey("Evan Mougalian", ptr("Penn")) == normalizeKey("Evan Mougalian", ptr("Penn State")) {
		t.Error("Penn and Penn State are different schools and must not collapse")
	}
}
