package intermat

import "testing"

const inSeasonTable = `<table border="1"><tbody>
<tr><td colspan="7"><font size="5">125lbs</font></td></tr>
<tr><td><b>RANK</b></td><td><b>WRESTLER</b></td><td><b>SCHOOL</b></td><td><b>CLASS</b></td><td><b>CONFERENCE</b></td><td><b>RECORD</b></td><td><b>LAST</b></td></tr>
<tr><td>1</td><td>Vincent Robinson</td><td>NC State</td><td>Sophomore</td><td>ACC</td><td>9-1</td><td>1</td></tr>
<tr><td>2</td><td>Luke Lilledahl</td><td>Penn State</td><td>Sophomore</td><td>Big Ten</td><td>8-0</td><td>2</td></tr>
</tbody></table>`

const preseasonTable = `<table border="1"><tbody>
<tr><td colspan="7"><font size="5">125lbs</font></td></tr>
<tr><td><b>RANK</b></td><td><b>WRESTLER</b></td><td><b>SCHOOL</b></td><td><b>CLASS</b></td><td><b>CONFERENCE</b></td><td><b>RECORD</b></td></tr>
<tr><td>1</td><td>Vincent Robinson</td><td>NC State</td><td>Sophomore</td><td>ACC</td><td>0-0</td></tr>
</tbody></table>`

const postseasonTable = `<table border="1"><tbody>
<tr><td colspan="5"><font size="5">125lbs</font></td></tr>
<tr><td><b>FINISH</b></td><td><b>WRESTLER</b></td><td><b>SCHOOL</b></td><td><b>CLASS</b></td><td><b>CONFERENCE</b></td></tr>
<tr><td>1</td><td>Vincent Robinson</td><td>NC State</td><td>Sophomore</td><td>ACC</td></tr>
</tbody></table>`

func TestParseTable_InSeason_AllColumns(t *testing.T) {
	rows, err := ParseTable(inSeasonTable)
	if err != nil {
		t.Fatalf("ParseTable: %v", err)
	}
	want := []Row{
		{Rank: 1, Name: "Vincent Robinson", School: "NC State", Grade: "Sophomore", Conference: "ACC", Record: "9-1", Previous: "1"},
		{Rank: 2, Name: "Luke Lilledahl", School: "Penn State", Grade: "Sophomore", Conference: "Big Ten", Record: "8-0", Previous: "2"},
	}
	if len(rows) != len(want) {
		t.Fatalf("got %d rows, want %d", len(rows), len(want))
	}
	for i, w := range want {
		if rows[i] != w {
			t.Errorf("row %d = %+v, want %+v", i, rows[i], w)
		}
	}
}

// The banner row's colspan ("7") does NOT reflect the real column count once
// LAST is absent — the header row itself, not colspan, is authoritative.
func TestParseTable_Preseason_NoLastColumn(t *testing.T) {
	rows, err := ParseTable(preseasonTable)
	if err != nil {
		t.Fatalf("ParseTable: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].Previous != "" {
		t.Errorf("Previous = %q, want empty when LAST column absent", rows[0].Previous)
	}
	if rows[0].Record != "0-0" {
		t.Errorf("Record = %q, want %q", rows[0].Record, "0-0")
	}
}

// The postseason final edition heads its rank column FINISH instead of RANK,
// and drops RECORD/LAST entirely — both must still parse.
func TestParseTable_Postseason_FinishHeaderNoRecordOrLast(t *testing.T) {
	rows, err := ParseTable(postseasonTable)
	if err != nil {
		t.Fatalf("ParseTable: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	want := Row{Rank: 1, Name: "Vincent Robinson", School: "NC State", Grade: "Sophomore", Conference: "ACC"}
	if rows[0] != want {
		t.Errorf("got %+v, want %+v", rows[0], want)
	}
}

// The real 2025-26 postseason-final edition (r76, fetched live 2026-09-12 —
// docs/sources/intermat.md) has non-numeric FINISH codes for the bottom half
// of each weight's 16 rows: only 1-8 (the All-Americans) are plain digits;
// 9-16 are reported as "R12"/"R16" bracket-elimination codes, not numbers.
func TestParseTable_Postseason_FinishCodesR12R16(t *testing.T) {
	const tbl = `<table><tbody>
<tr><td>FINISH</td><td>WRESTLER</td><td>SCHOOL</td></tr>
<tr><td>8</td><td>Eighth Placer</td><td>Team A</td></tr>
<tr><td>R12</td><td>Round Of Twelve</td><td>Team B</td></tr>
<tr><td>r16</td><td>Round Of Sixteen Lowercase</td><td>Team C</td></tr>
</tbody></table>`
	rows, err := ParseTable(tbl)
	if err != nil {
		t.Fatalf("ParseTable: %v", err)
	}
	want := []Row{
		{Rank: 8, Name: "Eighth Placer", School: "Team A"},
		{Rank: 9, Name: "Round Of Twelve", School: "Team B"},
		{Rank: 13, Name: "Round Of Sixteen Lowercase", School: "Team C"}, // lowercase "r16" must still map
	}
	for i, w := range want {
		if rows[i] != w {
			t.Errorf("row %d = %+v, want %+v", i, rows[i], w)
		}
	}
}

func TestParseTable_UnrecognizedFinishCodeStillFailsLoud(t *testing.T) {
	const tbl = `<table><tbody>
<tr><td>FINISH</td><td>WRESTLER</td><td>SCHOOL</td></tr>
<tr><td>DNS</td><td>Did Not Start</td><td>Team A</td></tr>
</tbody></table>`
	if _, err := ParseTable(tbl); err == nil {
		t.Fatal("expected error for an unrecognized non-numeric finish code")
	}
}

func TestParseTable_MissingRequiredColumn(t *testing.T) {
	const tbl = `<table><tbody>
<tr><td>RANK</td><td>SCHOOL</td></tr>
<tr><td>1</td><td>NC State</td></tr>
</tbody></table>`
	if _, err := ParseTable(tbl); err == nil {
		t.Fatal("expected error for missing WRESTLER column")
	}
}

func TestParseTable_MissingRankOrFinish(t *testing.T) {
	const tbl = `<table><tbody>
<tr><td>WRESTLER</td><td>SCHOOL</td></tr>
<tr><td>Vincent Robinson</td><td>NC State</td></tr>
</tbody></table>`
	if _, err := ParseTable(tbl); err == nil {
		t.Fatal("expected error when neither RANK nor FINISH is present")
	}
}

// A ragged row must fail loudly, same hard rule as scraper.ParseTable.
func TestParseTable_RaggedRow(t *testing.T) {
	const tbl = `<table><tbody>
<tr><td>RANK</td><td>WRESTLER</td><td>SCHOOL</td></tr>
<tr><td>1</td><td>Vincent Robinson</td></tr>
</tbody></table>`
	if _, err := ParseTable(tbl); err == nil {
		t.Fatal("expected error for a row shorter than the header")
	}
}

func TestParseTable_NonNumericRank(t *testing.T) {
	const tbl = `<table><tbody>
<tr><td>RANK</td><td>WRESTLER</td><td>SCHOOL</td></tr>
<tr><td>NR</td><td>Vincent Robinson</td><td>NC State</td></tr>
</tbody></table>`
	if _, err := ParseTable(tbl); err == nil {
		t.Fatal("expected error for non-numeric rank")
	}
}

func TestParseWeightBanner(t *testing.T) {
	cases := []struct {
		cells []string
		want  int
		ok    bool
	}{
		{[]string{"125lbs"}, 125, true},
		{[]string{"285 lbs"}, 285, true},
		{[]string{"Tournament Rankings"}, 0, false},
		{[]string{"Dual Rankings"}, 0, false},
		{[]string{"125lbs", "extra"}, 0, false}, // not a single-cell banner
	}
	for _, c := range cases {
		got, ok := parseWeightBanner(c.cells)
		if got != c.want || ok != c.ok {
			t.Errorf("parseWeightBanner(%v) = %d,%v want %d,%v", c.cells, got, ok, c.want, c.ok)
		}
	}
}
