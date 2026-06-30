package resolve

import "testing"

func ptr(s string) *string { return &s }

func TestNormalizeKey_CollapsesDrift(t *testing.T) {
	same := [][2]struct {
		name   string
		school *string
	}{
		{{"Vincent Robinson", ptr("NC State")}, {"vincent robinson", ptr("nc state")}},
		{{"Vincent Robinson", ptr("NC State")}, {" Vincent  Robinson ", ptr("NC  State")}},
		{{"Marc-Anthony McGowan", ptr("Princeton")}, {"Marc Anthony McGowan", ptr("Princeton")}},
		{{"St. John", ptr("Army")}, {"St John", ptr("Army")}},
	}
	for _, pair := range same {
		a := normalizeKey(pair[0].name, pair[0].school)
		b := normalizeKey(pair[1].name, pair[1].school)
		if a != b {
			t.Errorf("keys differ but should match:\n %q -> %q\n %q -> %q",
				pair[0].name, a, pair[1].name, b)
		}
	}
}

func TestNormalizeKey_DistinguishesDifferent(t *testing.T) {
	// Different school => different identity (transfers stay distinct).
	if normalizeKey("John Doe", ptr("Iowa")) == normalizeKey("John Doe", ptr("Penn State")) {
		t.Error("same name at different schools must produce different keys")
	}
	// Different name => different identity.
	if normalizeKey("John Doe", ptr("Iowa")) == normalizeKey("Jane Doe", ptr("Iowa")) {
		t.Error("different names must produce different keys")
	}
	// Name/school boundary must not be ambiguous.
	if normalizeKey("A B", ptr("C")) == normalizeKey("A", ptr("B C")) {
		t.Error("name/school boundary collision")
	}
}

func TestNormalizeKey_NilSchool(t *testing.T) {
	if normalizeKey("John Doe", nil) == normalizeKey("John Doe", ptr("Iowa")) {
		t.Error("nil school should differ from a named school")
	}
}
