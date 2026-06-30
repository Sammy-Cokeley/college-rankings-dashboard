package scraper

import "testing"

const fiveColTable = `<table border=""><tbody>
<tr><td>Rank</td><td>Grade</td><td>Name</td><td>School</td><td>Previous</td></tr>
<tr><td>1</td><td>SO</td><td>Vincent Robinson</td><td>NC State</td><td>1</td></tr>
<tr><td>2</td><td>JR</td><td>Marc-Anthony McGowan</td><td>Princeton</td><td>NR</td></tr>
<tr><td>3</td><td>SR</td><td>Isaac Trumble</td><td>NC State</td><td>8 (197)</td></tr>
</tbody></table>`

const fourColTable = `<table border=""><tbody>
<tr><td>Rank</td><td>Grade</td><td>Name</td><td>School</td></tr>
<tr><td>1</td><td>SO</td><td>Vincent Robinson</td><td>NC State</td></tr>
<tr><td>2</td><td>JR</td><td>Troy Spratley</td><td>OK State</td></tr>
</tbody></table>`

func TestParseTable_FiveColumns(t *testing.T) {
	rows, err := ParseTable(fiveColTable)
	if err != nil {
		t.Fatalf("ParseTable: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3", len(rows))
	}

	want := []Row{
		{Rank: 1, Grade: "SO", Name: "Vincent Robinson", School: "NC State", Previous: "1"},
		{Rank: 2, Grade: "JR", Name: "Marc-Anthony McGowan", School: "Princeton", Previous: "NR"},
		{Rank: 3, Grade: "SR", Name: "Isaac Trumble", School: "NC State", Previous: "8 (197)"},
	}
	for i, w := range want {
		if rows[i] != w {
			t.Errorf("row %d = %+v, want %+v", i, rows[i], w)
		}
	}
}

func TestParseTable_FourColumns_NoPrevious(t *testing.T) {
	rows, err := ParseTable(fourColTable)
	if err != nil {
		t.Fatalf("ParseTable: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	if rows[0].Previous != "" {
		t.Errorf("Previous = %q, want empty when column absent", rows[0].Previous)
	}
	if rows[1].Name != "Troy Spratley" {
		t.Errorf("Name = %q", rows[1].Name)
	}
}

// Real Flo tables carry inline style markup and &nbsp; padding; the parser must
// read text content and normalize whitespace.
func TestParseTable_StrayMarkupAndNbsp(t *testing.T) {
	const tbl = `<table><tbody>
<tr><td style="x">Rank</td><td>Grade</td><td>Name</td><td>School</td></tr>
<tr><td style="p:2px">1</td><td>SO</td><td><span>Vincent&nbsp;Robinson</span></td><td>NC&nbsp;State</td></tr>
</tbody></table>`
	rows, err := ParseTable(tbl)
	if err != nil {
		t.Fatalf("ParseTable: %v", err)
	}
	if rows[0].Name != "Vincent Robinson" {
		t.Errorf("Name = %q, want normalized 'Vincent Robinson'", rows[0].Name)
	}
	if rows[0].School != "NC State" {
		t.Errorf("School = %q, want 'NC State'", rows[0].School)
	}
}

func TestParseTable_ColumnsByHeaderNotPosition(t *testing.T) {
	// Reordered columns must still map correctly.
	const tbl = `<table><tbody>
<tr><td>Name</td><td>Rank</td><td>School</td><td>Grade</td></tr>
<tr><td>Vincent Robinson</td><td>1</td><td>NC State</td><td>SO</td></tr>
</tbody></table>`
	rows, err := ParseTable(tbl)
	if err != nil {
		t.Fatalf("ParseTable: %v", err)
	}
	want := Row{Rank: 1, Grade: "SO", Name: "Vincent Robinson", School: "NC State"}
	if rows[0] != want {
		t.Errorf("got %+v, want %+v", rows[0], want)
	}
}

func TestParseTable_MissingRequiredColumn(t *testing.T) {
	const tbl = `<table><tbody>
<tr><td>Rank</td><td>Name</td><td>School</td></tr>
<tr><td>1</td><td>Vincent Robinson</td><td>NC State</td></tr>
</tbody></table>`
	if _, err := ParseTable(tbl); err == nil {
		t.Fatal("expected error for missing Grade column")
	}
}

// A ragged row (fewer or more cells than the header) must fail loudly rather
// than silently shifting columns and dropping the name.
func TestParseTable_RaggedRow(t *testing.T) {
	const short = `<table><tbody>
<tr><td>Rank</td><td>Grade</td><td>Name</td><td>School</td></tr>
<tr><td>1</td><td>SO</td><td>Vincent Robinson</td></tr>
</tbody></table>`
	if _, err := ParseTable(short); err == nil {
		t.Fatal("expected error for a row shorter than the header")
	}

	const long = `<table><tbody>
<tr><td>Rank</td><td>Grade</td><td>Name</td><td>School</td></tr>
<tr><td>1</td><td>SO</td><td>Vincent Robinson</td><td>NC State</td><td>stray</td></tr>
</tbody></table>`
	if _, err := ParseTable(long); err == nil {
		t.Fatal("expected error for a row longer than the header")
	}
}

func TestParseTable_DuplicateHeaderColumn(t *testing.T) {
	const tbl = `<table><tbody>
<tr><td>Rank</td><td>Grade</td><td>Name</td><td>School</td><td>School</td></tr>
<tr><td>1</td><td>SO</td><td>Vincent Robinson</td><td>NC State</td><td>NC State</td></tr>
</tbody></table>`
	if _, err := ParseTable(tbl); err == nil {
		t.Fatal("expected error for a duplicate header label")
	}
}

// Only the first table's rows are parsed; a sibling table's rows must not leak
// in. Here the second table would otherwise add a bogus rank-9 row.
func TestParseTable_IgnoresSiblingTable(t *testing.T) {
	const tbl = `<div>
<table><tbody>
<tr><td>Rank</td><td>Grade</td><td>Name</td><td>School</td></tr>
<tr><td>1</td><td>SO</td><td>Vincent Robinson</td><td>NC State</td></tr>
</tbody></table>
<table><tbody>
<tr><td>9</td><td>XX</td><td>Bogus Person</td><td>Nowhere</td></tr>
</tbody></table>
</div>`
	rows, err := ParseTable(tbl)
	if err != nil {
		t.Fatalf("ParseTable: %v", err)
	}
	if len(rows) != 1 || rows[0].Name != "Vincent Robinson" {
		t.Fatalf("got %+v, want only the first table's single row", rows)
	}
}

// A nested table inside a cell must not contribute rows either.
func TestParseTable_IgnoresNestedTableInCell(t *testing.T) {
	const tbl = `<table><tbody>
<tr><td>Rank</td><td>Grade</td><td>Name</td><td>School</td></tr>
<tr><td>1</td><td>SO</td><td>Vincent Robinson</td><td><table><tr><td>NC State</td></tr></table></td></tr>
</tbody></table>`
	rows, err := ParseTable(tbl)
	if err != nil {
		t.Fatalf("ParseTable: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].School != "NC State" {
		t.Errorf("School = %q, want nested-cell text 'NC State'", rows[0].School)
	}
}

func TestParseTable_NonNumericRank(t *testing.T) {
	const tbl = `<table><tbody>
<tr><td>Rank</td><td>Grade</td><td>Name</td><td>School</td></tr>
<tr><td>NR</td><td>SO</td><td>Vincent Robinson</td><td>NC State</td></tr>
</tbody></table>`
	if _, err := ParseTable(tbl); err == nil {
		t.Fatal("expected error for non-numeric rank")
	}
}
