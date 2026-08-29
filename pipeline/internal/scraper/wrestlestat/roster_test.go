package wrestlestat

import "testing"

func teamProfilePage(rosterRows string) string {
	// Mirrors the real page's shape enough to exercise the scoping logic: a
	// <table> BEFORE #roster (must be ignored) and the roster table nested one
	// level inside the tab-pane (must not assume "first table in document").
	return `<!doctype html><html><body>
<table><tbody><tr><td>Unrelated Schedule Table</td></tr></tbody></table>
<div class="tab-pane fade active show" id="roster" role="tabpanel">
<div class="row"><div class="col">
<table class="table table-sm table-hover table-striped">
<thead><tr><th>Weight</th><th>Name</th><th>Class</th><th>Record</th><th>Action</th><th>Videos</th></tr></thead>
<tbody>
` + rosterRows + `
</tbody>
</table>
</div></div>
</div>
</body></html>`
}

func TestParseRoster(t *testing.T) {
	page := teamProfilePage(`
<tr class="bg-starter">
  <td class="align-middle" itemprop="weight">125</td>
  <td class="align-middle" itemprop="url">
    <a href="/wrestler/73466/tocci-nico/profile" class="h4 text-primary">#135 Tocci, Nico</a>
    <span>&nbsp;</span><small class="badge bg-success">ST</small>
  </td>
  <td class="align-middle">SR</td>
  <td class="align-middle">0 - 0</td>
  <td>actions</td>
  <td>videos</td>
</tr>
<tr>
  <td class="align-middle" itemprop="weight">133</td>
  <td class="align-middle" itemprop="url">
    <a href="/wrestler/83285/caprella-gavin/profile" class="h4 text-primary">#93 Caprella, Gavin</a>
  </td>
  <td class="align-middle">RSFR</td>
  <td class="align-middle">0 - 0</td>
  <td>actions</td>
  <td>videos</td>
</tr>`)

	rows, err := ParseRoster([]byte(page))
	if err != nil {
		t.Fatalf("ParseRoster: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}

	// The ST badge must NOT be fused onto the name (it lives in a sibling
	// <span>/<small>, not inside the <a>).
	if rows[0].Name != "#135 Tocci, Nico" {
		t.Errorf("row 0 Name = %q, want %q (badge text must not leak in)", rows[0].Name, "#135 Tocci, Nico")
	}
	if rows[0].Weight != 125 || rows[0].Class != "SR" {
		t.Errorf("row 0 = %+v", rows[0])
	}
	if rows[1].Name != "#93 Caprella, Gavin" || rows[1].Weight != 133 || rows[1].Class != "RSFR" {
		t.Errorf("row 1 = %+v", rows[1])
	}
}

func TestParseRoster_IgnoresTablesOutsideRosterPane(t *testing.T) {
	// teamProfilePage already prepends an unrelated table; a naive
	// "first table in the document" parse would read ITS one cell as a
	// header and fail on the required-column check. Confirms scoping to
	// #roster is load-bearing, not incidental.
	page := teamProfilePage(`
<tr>
  <td itemprop="weight">125</td>
  <td itemprop="url"><a href="/wrestler/1/a/profile">A, One</a></td>
  <td>SR</td><td>0-0</td><td></td><td></td>
</tr>`)
	rows, err := ParseRoster([]byte(page))
	if err != nil {
		t.Fatalf("ParseRoster: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
}

func TestParseRoster_MissingRosterPane(t *testing.T) {
	_, err := ParseRoster([]byte(`<!doctype html><html><body>no roster div here</body></html>`))
	if err == nil {
		t.Fatal("expected an error when #roster is absent")
	}
}

func TestParseRoster_RaggedRowFailsLoud(t *testing.T) {
	page := teamProfilePage(`
<tr><td itemprop="weight">125</td><td itemprop="url"><a href="/wrestler/1/a/profile">A, One</a></td></tr>`)
	_, err := ParseRoster([]byte(page))
	if err == nil {
		t.Fatal("expected an error: row has fewer cells than the header")
	}
}

func TestParseRoster_PlaceholderSlot(t *testing.T) {
	// A real anomaly seen on live pages (docs/sources/wrestlestat.md): an
	// unfilled roster slot named after the school itself. The parser must
	// not choke on it — flagging/filtering it is ingest's job (roster.go
	// there has the school-name context this package doesn't), not the
	// scraper's.
	page := teamProfilePage(`
<tr>
  <td itemprop="weight">125</td>
  <td itemprop="url"><a href="/wrestler/16541/air-force-air-force/profile">(Air Force), (Air Force)</a></td>
  <td>SO</td><td>0 - 0</td><td></td><td></td>
</tr>`)
	rows, err := ParseRoster([]byte(page))
	if err != nil {
		t.Fatalf("ParseRoster: %v", err)
	}
	if len(rows) != 1 || rows[0].Name != "(Air Force), (Air Force)" {
		t.Errorf("rows = %+v", rows)
	}
}
