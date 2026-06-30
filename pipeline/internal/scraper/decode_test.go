package scraper

import (
	"strings"
	"testing"
)

// syntheticPage builds a minimal Flo-style page: a flo-app-state script holding
// entity-escaped transfer-state JSON keyed by a ranking-containers URL. The
// escaping mirrors the live blob (quotes as &q;, ampersands as &a;, etc.).
func syntheticPage() string {
	// Plain JSON we want to recover after decoding.
	plain := `{` +
		`"G.https://api.flowrestling.org/.../ranking-containers/14300895":` +
		`{"body":{"data":{"id":14300895,"title":"2025-26 NCAA DI & friends",` +
		`"ranking_sections":{"2":[{"id":56108,"name":"125","publish_date":"2025-06-19",` +
		`"content":"<table><tbody><tr><td>Rank</td><td>Grade</td><td>Name</td><td>School</td></tr>` +
		`<tr><td>1</td><td>SO</td><td>Vincent Robinson</td><td>NC State</td></tr></tbody></table>"}]}}}},` +
		`"G.https://api.flowrestling.org/.../other":{"body":{"data":{"unrelated":true}}}` +
		`}`

	// Apply the escaping the SSR backend uses (reverse of entitySubs).
	escaped := plain
	escaped = strings.ReplaceAll(escaped, "&", "&a;") // ampersand first when ENCODING
	escaped = strings.ReplaceAll(escaped, `"`, "&q;")
	escaped = strings.ReplaceAll(escaped, "<", "&l;")
	escaped = strings.ReplaceAll(escaped, ">", "&g;")
	escaped = strings.ReplaceAll(escaped, "'", "&s;")

	return `<!doctype html><html><head></head><body>` +
		`<script id="flo-app-state" type="application/json">` + escaped + `</script>` +
		`</body></html>`
}

func TestDecodeAppState(t *testing.T) {
	c, err := DecodeAppState([]byte(syntheticPage()))
	if err != nil {
		t.Fatalf("DecodeAppState: %v", err)
	}
	if c.ID != 14300895 {
		t.Errorf("id = %d, want 14300895", c.ID)
	}
	// The title contains a literal ampersand, proving &a; decoded correctly.
	if c.Title != "2025-26 NCAA DI & friends" {
		t.Errorf("title = %q", c.Title)
	}
	rows, err := ParseTable(c.RankingSections["2"][0].Content)
	if err != nil {
		t.Fatalf("ParseTable on decoded content: %v", err)
	}
	if len(rows) != 1 || rows[0].Name != "Vincent Robinson" {
		t.Errorf("decoded rows = %+v", rows)
	}
}

func TestDecodeAppState_NoScript(t *testing.T) {
	page := `<html><body><p>no app state here</p></body></html>`
	if _, err := DecodeAppState([]byte(page)); err == nil {
		t.Fatal("expected error when flo-app-state script absent")
	}
}

func TestDecodeAppState_NoRankingContainer(t *testing.T) {
	page := `<html><body><script id="flo-app-state">` +
		`{&q;G.https://api/other&q;:{&q;body&q;:{&q;data&q;:{}}}}` +
		`</script></body></html>`
	if _, err := DecodeAppState([]byte(page)); err == nil {
		t.Fatal("expected error when no ranking-containers key present")
	}
}
