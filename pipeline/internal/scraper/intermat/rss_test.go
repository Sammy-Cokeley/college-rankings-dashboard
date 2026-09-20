package intermat

import (
	"os"
	"path/filepath"
	"testing"
)

// The real RSS feed (fetched 2026-09-19): item order is [DI preseason
// announcement (newest, real, r78), a same-week decoy with a date suffix
// but no DI ranking (real, "Offseason Coaching Carousel"), a synthetic
// older DI "Rankings Updated" item (proves the OTHER real title pattern
// this feed has used in-season also matches, and that it's correctly
// passed over in favor of the newer item first in the feed)].
func loadRSSFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "rss_feed.xml"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return data
}

func TestParseLatestEdition(t *testing.T) {
	got, err := parseLatestEdition(loadRSSFixture(t))
	if err != nil {
		t.Fatalf("parseLatestEdition: %v", err)
	}
	want := LatestEdition{
		PublishedDate: "2026-09-17",
		RecordURL:     "https://intermatwrestle.com/rankings.html/ncaa-di-r78/",
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestParseLatestEdition_IgnoresNonDITitlesWithDateSuffix(t *testing.T) {
	// The decoy item ("Offseason Coaching Carousel (9/9/2026)") sits between
	// the two real DI items in the fixture; if the title filter were too
	// loose (matching on the date suffix alone) it would win over both real
	// DI announcements by virtue of position. It must never be selected.
	got, err := parseLatestEdition(loadRSSFixture(t))
	if err != nil {
		t.Fatalf("parseLatestEdition: %v", err)
	}
	if got.RecordURL == "https://intermatwrestle.com/articles.html/college/the-2026-offseason-coaching-carousel-992026-r101212/" {
		t.Fatal("selected the non-DI decoy item")
	}
}

func TestParseLatestEdition_NoDIItem(t *testing.T) {
	const feed = `<rss><channel><item><title>Some Other Article (9/9/2026)</title><description>no rankings here</description></item></channel></rss>`
	if _, err := parseLatestEdition([]byte(feed)); err == nil {
		t.Fatal("expected error when no DI rankings item is present")
	}
}

func TestParseLatestEdition_SkipsMatchWithNoRecordLink(t *testing.T) {
	// A DI title match with no discoverable record link must be skipped
	// (not guessed at) in favor of the next real match in the feed.
	const feed = `<rss><channel>
<item><title>NCAA DI Rankings Updated (9/20/2026)</title><description>no link in this one</description></item>
<item><title>NCAA DI Rankings Updated (9/13/2026)</title><description><![CDATA[<a href="https://intermatwrestle.com/rankings.html/ncaa-di-r79/">here</a>]]></description></item>
</channel></rss>`
	got, err := parseLatestEdition([]byte(feed))
	if err != nil {
		t.Fatalf("parseLatestEdition: %v", err)
	}
	want := LatestEdition{PublishedDate: "2026-09-13", RecordURL: "https://intermatwrestle.com/rankings.html/ncaa-di-r79/"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
