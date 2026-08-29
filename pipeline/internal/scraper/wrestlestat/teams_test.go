package wrestlestat

import "testing"

func teamSelectPage(options string) string {
	return `<!doctype html><html><body>
<select class="form-control" id="homepage-team_select" name="homepage-team_select">
<option value="">Select a team...</option>
` + options + `
</select>
</body></html>`
}

func TestParseTeamList(t *testing.T) {
	page := teamSelectPage(`
<option value="1">Air Force</option>
<option value="27">Franklin &amp; Marshall</option>
<option value="2">American</option>`)

	teams, err := ParseTeamList([]byte(page))
	if err != nil {
		t.Fatalf("ParseTeamList: %v", err)
	}
	want := []Team{{1, "Air Force"}, {27, "Franklin & Marshall"}, {2, "American"}}
	if len(teams) != len(want) {
		t.Fatalf("teams = %+v, want %+v", teams, want)
	}
	for i, tm := range teams {
		if tm != want[i] {
			t.Errorf("team %d = %+v, want %+v", i, tm, want[i])
		}
	}
}

func TestParseTeamList_SkipsBlankOption(t *testing.T) {
	page := teamSelectPage(`<option value="5">Army West Point</option>`)
	teams, err := ParseTeamList([]byte(page))
	if err != nil {
		t.Fatalf("ParseTeamList: %v", err)
	}
	if len(teams) != 1 || teams[0].ID != 5 {
		t.Errorf("teams = %+v, want exactly Army West Point (5)", teams)
	}
}

func TestParseTeamList_MissingSelect(t *testing.T) {
	_, err := ParseTeamList([]byte(`<!doctype html><html><body>no select here</body></html>`))
	if err == nil {
		t.Fatal("expected an error when #homepage-team_select is absent")
	}
}
