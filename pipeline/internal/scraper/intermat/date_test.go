package intermat

import "testing"

func TestResolveDate(t *testing.T) {
	got, err := ResolveDate("20260106175039")
	if err != nil {
		t.Fatalf("ResolveDate: %v", err)
	}
	if got != "2026-01-06" {
		t.Errorf("ResolveDate = %q, want %q", got, "2026-01-06")
	}
}

func TestResolveDate_Malformed(t *testing.T) {
	if _, err := ResolveDate("not-a-timestamp"); err == nil {
		t.Fatal("expected error for a malformed timestamp")
	}
}
