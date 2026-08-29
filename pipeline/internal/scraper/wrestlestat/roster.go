package wrestlestat

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// Row is one roster entry parsed from a team's profile/roster table
// (docs/sources/wrestlestat.md). Name is the wrestler-identity cell's own
// text only — WrestleStat renders a "ST"/"RS" status badge as a sibling of
// the name's <a> inside the same cell, and naïvely taking the whole
// cell's text would fuse the badge onto the name (e.g. "#135 Tocci, Nico ST"
// instead of "#135 Tocci, Nico"). Raw is the whole cell's text, verbatim,
// including any badge — never discarded, just not conflated with Name.
type Row struct {
	Weight int
	Name   string // e.g. "#135 Tocci, Nico" — seed prefix and "Last, First" order still present; ingest cleans this further
	Class  string // eligibility year, e.g. "SR", "RSFR"
	Raw    string // full published cell text, verbatim (includes ST/RS badge text if present)
}

const (
	colWeight = "weight"
	colName   = "name"
	colClass  = "class"
)

// ParseRoster parses a team profile page's roster table. It's header-driven
// like internal/scraper's ParseTable (never assume column position), but
// scoped to the #roster tab-pane specifically — the full profile page has
// many other <table>s (schedule, injury history, nationals history, ...)
// before and after it, unlike a FloWrestling edition where the content HTML
// *is* just the one table.
func ParseRoster(page []byte) ([]Row, error) {
	node, err := html.Parse(strings.NewReader(string(page)))
	if err != nil {
		return nil, fmt.Errorf("parse team profile page: %w", err)
	}

	pane := findByID(node, "roster")
	if pane == nil {
		return nil, fmt.Errorf("team profile page: #roster tab-pane not found")
	}
	table := firstElement(pane, "table")
	if table == nil {
		return nil, fmt.Errorf("team profile page: no <table> inside #roster")
	}

	trs := rows(table)
	if len(trs) == 0 {
		return nil, fmt.Errorf("roster table: no rows found")
	}

	index, err := headerIndex(trs[0])
	if err != nil {
		return nil, err
	}

	out := make([]Row, 0, len(trs)-1)
	for i, cells := range trs[1:] {
		if len(cells) != len(trs[0]) {
			return nil, fmt.Errorf("row %d: has %d cells, header has %d", i+1, len(cells), len(trs[0]))
		}
		row, err := buildRow(cells, index)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", i+1, err)
		}
		out = append(out, row)
	}
	return out, nil
}

func headerIndex(header []*html.Node) (map[string]int, error) {
	index := make(map[string]int, len(header))
	for i, cell := range header {
		key := strings.ToLower(normalizeCell(textOf(cell)))
		if key == "" {
			continue
		}
		if _, dup := index[key]; dup {
			return nil, fmt.Errorf("duplicate header column %q", key)
		}
		index[key] = i
	}
	for _, required := range []string{colWeight, colName, colClass} {
		if _, ok := index[required]; !ok {
			return nil, fmt.Errorf("missing required column %q", required)
		}
	}
	return index, nil
}

func buildRow(cells []*html.Node, index map[string]int) (Row, error) {
	get := func(col string) *html.Node {
		i, ok := index[col]
		if !ok || i >= len(cells) {
			return nil
		}
		return cells[i]
	}

	weightCell := get(colWeight)
	weightStr := normalizeCell(textOf(weightCell))
	weight, err := strconv.Atoi(weightStr)
	if err != nil {
		return Row{}, fmt.Errorf("non-numeric weight %q", weightStr)
	}

	nameCell := get(colName)
	rawName := normalizeCell(textOf(nameCell))
	name := rawName
	if a := firstElement(nameCell, "a"); a != nil {
		// Prefer the <a>'s own text: excludes sibling ST/RS badge markup that
		// lives in the same <td> (see Row's doc comment).
		name = normalizeCell(textOf(a))
	}

	return Row{
		Weight: weight,
		Name:   name,
		Class:  normalizeCell(textOf(get(colClass))),
		Raw:    rawName,
	}, nil
}

// rows returns every <tr>'s cells, scoped to table (not descending into a
// nested table reached outside a <tr>), mirroring internal/scraper's
// extractRows/firstTable but returning nodes (not pre-flattened strings) since
// buildRow needs to reach into the Name cell's <a> specifically.
func rows(table *html.Node) [][]*html.Node {
	var trs [][]*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch {
		case n.Type == html.ElementNode && n.Data == "tr":
			trs = append(trs, cellNodes(n))
			return
		case n.Type == html.ElementNode && n.Data == "table" && n != table:
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(table)
	return trs
}

func cellNodes(tr *html.Node) []*html.Node {
	var cells []*html.Node
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
			cells = append(cells, c)
		}
	}
	return cells
}
