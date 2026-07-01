package scraper

import (
	"strings"
	"testing"
)

// syntheticPage builds a minimal Flo-style page: a flo-app-state script holding
// entity-escaped transfer-state JSON keyed by a ranking-containers URL. The
// escaping mirrors the live blob (quotes as &q;, ampersands as &a;, etc.).
func syntheticPage() string {
	// Plain JSON we want to recover after decoding. The transfer state mixes
	// value shapes — bare string, bool, and number siblings alongside the object
	// entries — exactly like the live blob (2 of 8 keys are bare strings). The
	// decoder must not try to type every value as a body/data object.
	plain := `{` +
		`"G.config.version":"1.33.2",` +
		`"G.flags.enabled":true,` +
		`"G.count":8,` +
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

// More than one key contains "ranking-containers": map iteration order is
// random, so the decoder must reject the ambiguity rather than silently pick one.
func TestDecodeAppState_AmbiguousRankingContainers(t *testing.T) {
	page := `<html><body><script id="flo-app-state">` +
		`{&q;G.a/ranking-containers/1&q;:{&q;body&q;:{&q;data&q;:{&q;ranking_sections&q;:{}}}},` +
		`&q;G.b/ranking-containers/2&q;:{&q;body&q;:{&q;data&q;:{&q;ranking_sections&q;:{}}}}}` +
		`</script></body></html>`
	if _, err := DecodeAppState([]byte(page)); err == nil {
		t.Fatal("expected error when multiple ranking-containers keys present")
	}
}

// The matched ranking-containers key exists but its value is a bare string, not
// a {body:{data}} object — exercises the per-entry unmarshal error path that the
// value-by-value decode introduced.
func TestDecodeAppState_MalformedRankingContainerEntry(t *testing.T) {
	page := `<html><body><script id="flo-app-state">` +
		`{&q;G.api/ranking-containers/1&q;:&q;not an object&q;}` +
		`</script></body></html>`
	if _, err := DecodeAppState([]byte(page)); err == nil {
		t.Fatal("expected error when the ranking-containers value is not a body/data object")
	}
}
