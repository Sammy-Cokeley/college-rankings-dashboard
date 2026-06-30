package scraper

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// Row is one ranked wrestler parsed from an edition's content table. Previous
// is captured verbatim as published (numeric, "NR", or an oddity like
// "8 (197)") and kept opaque — movement is derived ourselves, not read from it.
// Previous is empty when the edition's table has no Previous column (the
// preseason shape).
type Row struct {
	Rank     int
	Grade    string
	Name     string
	School   string
	Previous string
}

// column header labels, matched case-insensitively against the table's first row.
const (
	colRank     = "rank"
	colGrade    = "grade"
	colName     = "name"
	colSchool   = "school"
	colPrevious = "previous"
)

// ParseTable parses an edition's content HTML <table> into ranked rows. It is
// header-driven: the first table row supplies the column order, and every
// subsequent row is read by header name — never by fixed position or count,
// because both the column set and row count vary across editions
// (docs/sources/flowrestling.md). Rank/Grade/Name/School are required; Previous
// is optional. Cells are de-marked-up and whitespace-normalized (incl. &nbsp;).
func ParseTable(content string) ([]Row, error) {
	node, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("parse content html: %w", err)
	}

	rows := extractRows(node)
	if len(rows) == 0 {
		return nil, fmt.Errorf("no table rows found")
	}

	header := rows[0]
	index, err := headerIndex(header)
	if err != nil {
		return nil, err
	}

	out := make([]Row, 0, len(rows)-1)
	for i, cells := range rows[1:] {
		// A data row must have exactly the header's column count. A ragged row
		// (truncated/extra cell from hand-entered markup) would otherwise be
		// read against the wrong columns and silently drop the name — and the
		// project's hard rule is that raw_source_string is never lost. Fail loud.
		if len(cells) != len(header) {
			return nil, fmt.Errorf("row %d: has %d cells, header has %d: %v",
				i+1, len(cells), len(header), cells)
		}
		row, err := buildRow(cells, index)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", i+1, err)
		}
		out = append(out, row)
	}
	return out, nil
}

// headerIndex maps lowercased header labels to their column position and
// verifies the required columns are present. A duplicate non-empty label is
// rejected rather than silently overwriting the earlier column, so a row can
// never be parsed against the wrong column without a diagnostic.
func headerIndex(header []string) (map[string]int, error) {
	index := make(map[string]int, len(header))
	for i, label := range header {
		key := strings.ToLower(label)
		if key == "" {
			continue
		}
		if _, dup := index[key]; dup {
			return nil, fmt.Errorf("duplicate header column %q in header %v", label, header)
		}
		index[key] = i
	}
	for _, required := range []string{colRank, colGrade, colName, colSchool} {
		if _, ok := index[required]; !ok {
			return nil, fmt.Errorf("missing required column %q in header %v", required, header)
		}
	}
	return index, nil
}

func buildRow(cells []string, index map[string]int) (Row, error) {
	get := func(col string) string {
		i, ok := index[col]
		if !ok || i >= len(cells) {
			return ""
		}
		return cells[i]
	}

	rankStr := get(colRank)
	rank, err := strconv.Atoi(rankStr)
	if err != nil {
		return Row{}, fmt.Errorf("non-numeric rank %q", rankStr)
	}
	return Row{
		Rank:     rank,
		Grade:    get(colGrade),
		Name:     get(colName),
		School:   get(colSchool),
		Previous: get(colPrevious),
	}, nil
}

// extractRows returns each <tr>'s cell text from the FIRST <table> in the
// document only. Scoping to one table means stray sibling tables in the content
// HTML can't flatten their rows into this one's. It reads both <td> and <th> so
// it is agnostic to how Flo marks the header row (the real tables use <td> for
// the header too). Cell text is normalized.
func extractRows(root *html.Node) [][]string {
	table := firstTable(root)
	if table == nil {
		return nil
	}

	var rows [][]string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch {
		case n.Type == html.ElementNode && n.Data == "tr":
			// Stop at the row: a nested table inside a cell is not descended
			// into, so its rows don't leak into this table.
			rows = append(rows, rowCells(n))
			return
		case n.Type == html.ElementNode && n.Data == "table" && n != table:
			// A nested table reached outside a <tr> — skip it.
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(table)
	return rows
}

// firstTable returns the first <table> element in document order, or nil.
func firstTable(root *html.Node) *html.Node {
	if root.Type == html.ElementNode && root.Data == "table" {
		return root
	}
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		if t := firstTable(c); t != nil {
			return t
		}
	}
	return nil
}

func rowCells(tr *html.Node) []string {
	var cells []string
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
			cells = append(cells, normalizeCell(textOf(c)))
		}
	}
	return cells
}

// textOf concatenates all descendant text of a node (cells may wrap content in
// stray inline markup — <span>, <b>, etc.).
func textOf(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

// normalizeCell turns non-breaking spaces into ordinary spaces, collapses
// internal whitespace runs, and trims. html.Parse already decodes entities, so
// &nbsp; arrives here as U+00A0.
func normalizeCell(s string) string {
	s = strings.ReplaceAll(s, " ", " ")
	return strings.Join(strings.Fields(s), " ")
}
