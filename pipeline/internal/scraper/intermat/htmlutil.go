package intermat

import (
	"strings"

	"golang.org/x/net/html"
)

// textOf concatenates all descendant text of a node (cells may wrap content in
// stray inline markup — <font>, <b>, etc.). Mirrors internal/scraper's helper
// of the same name; duplicated rather than imported because internal/scraper's
// version is unexported and this is a sibling package, not a subpackage of it
// (same convention wrestlestat/htmlutil.go already established).
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
// internal whitespace runs, and trims. Mirrors internal/scraper's helper of
// the same name (see textOf's doc comment for why it's duplicated).
func normalizeCell(s string) string {
	s = strings.ReplaceAll(s, " ", " ")
	return strings.Join(strings.Fields(s), " ")
}

// allTables returns every <table> element in document order. Unlike Flo's
// firstTable (one table per content blob), an InterMat record page embeds one
// table per weight class (plus Tournament/Dual, filtered out by the caller),
// so every table must be enumerated, not just the first.
func allTables(root *html.Node) []*html.Node {
	var tables []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "table" {
			tables = append(tables, n)
			return // nested tables (none observed, but matches Flo's non-descent rule)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return tables
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

// directRows returns the cell text of every non-empty <tr> that is a direct
// descendant of table's row structure (table > tr, or table > tbody > tr —
// both occur in the wild). Does not descend into nested tables, matching
// scraper.extractRows. A <tr> with zero <td>/<th> children (real, observed
// in InterMat's own markup — a stray trailing blank row before </table>) is
// dropped here rather than surfacing as a "ragged row" downstream: it is
// structural padding, not a data row, so skipping it drops nothing that was
// ever raw_source_string in the first place. This is NOT the same as a row
// with the wrong non-zero cell count, which must still fail loud.
func directRows(table *html.Node) [][]string {
	var rows [][]string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch {
		case n.Type == html.ElementNode && n.Data == "tr":
			if cells := rowCells(n); len(cells) > 0 {
				rows = append(rows, cells)
			}
			return
		case n.Type == html.ElementNode && n.Data == "table" && n != table:
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(table)
	return rows
}
