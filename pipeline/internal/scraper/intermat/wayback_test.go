package intermat

import "testing"

// Sample lines shaped exactly like the real CDX API response (verified
// 2026-09-12 — see wayback.go's doc comment): plain space-separated text,
// fields urlkey/timestamp/original/mimetype/statuscode/digest/length.
const sampleCDX = `com,intermatwrestle)/rankings.html/ncaa-di-r63 20260106175039 https://intermatwrestle.com/rankings.html/ncaa-di-r63/ text/html 200 JJWCR2EBJ5T2M7CMHUNN34374PMEBKRE 22334
com,intermatwrestle)/rankings.html/ncaa-di-r63 20260109120000 https://intermatwrestle.com/rankings.html/ncaa-di-r63/ text/html 200 JJWCR2EBJ5T2M7CMHUNN34374PMEBKRE 22334
com,intermatwrestle)/rankings.html/ncaa-di-r64 20260113155355 https://intermatwrestle.com/rankings.html/ncaa-di-r64/ text/html 200 QZTFYFBJRI4Y27H46E6OP6CS3ZOGUR6K 28055`

func TestParseCDX(t *testing.T) {
	snapshots, err := parseCDX([]byte(sampleCDX))
	if err != nil {
		t.Fatalf("parseCDX: %v", err)
	}
	if len(snapshots) != 3 {
		t.Fatalf("got %d snapshots, want 3", len(snapshots))
	}
	if snapshots[0].Timestamp != "20260106175039" {
		t.Errorf("Timestamp = %q", snapshots[0].Timestamp)
	}
	if snapshots[0].OriginalURL != "https://intermatwrestle.com/rankings.html/ncaa-di-r63/" {
		t.Errorf("OriginalURL = %q", snapshots[0].OriginalURL)
	}
}

func TestParseCDX_EmptyResponse(t *testing.T) {
	snapshots, err := parseCDX([]byte(""))
	if err != nil {
		t.Fatalf("parseCDX: %v", err)
	}
	if len(snapshots) != 0 {
		t.Errorf("got %d snapshots, want 0", len(snapshots))
	}
}

func TestParseCDX_MalformedLine(t *testing.T) {
	if _, err := parseCDX([]byte("only two")); err == nil {
		t.Fatal("expected error for a line with too few fields")
	}
}

func TestDedupeByRecordID(t *testing.T) {
	snapshots, err := parseCDX([]byte(sampleCDX))
	if err != nil {
		t.Fatalf("parseCDX: %v", err)
	}
	deduped := DedupeByRecordID(snapshots)
	if len(deduped) != 2 {
		t.Fatalf("got %d deduped snapshots, want 2 (one per record id)", len(deduped))
	}
	// r63's EARLIEST capture (…175039) must be kept, not the later one (…120000+3 days).
	for _, s := range deduped {
		if s.OriginalURL == "https://intermatwrestle.com/rankings.html/ncaa-di-r63/" {
			if s.Timestamp != "20260106175039" {
				t.Errorf("r63 timestamp = %q, want the earliest capture 20260106175039", s.Timestamp)
			}
		}
	}
	// Sorted by timestamp ascending: r63 before r64.
	if deduped[0].OriginalURL != "https://intermatwrestle.com/rankings.html/ncaa-di-r63/" {
		t.Errorf("deduped[0] = %+v, want r63 first (earlier timestamp)", deduped[0])
	}
}

func TestSnapshotURL(t *testing.T) {
	s := CDXSnapshot{Timestamp: "20260106175039", OriginalURL: "https://intermatwrestle.com/rankings.html/ncaa-di-r63/"}
	want := "https://web.archive.org/web/20260106175039id_/https://intermatwrestle.com/rankings.html/ncaa-di-r63/"
	if got := SnapshotURL(s); got != want {
		t.Errorf("SnapshotURL = %q, want %q", got, want)
	}
}
