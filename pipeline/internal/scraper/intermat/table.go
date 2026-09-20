package intermat

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// Row is one ranked wrestler parsed from a single weight's InterMat table.
// Conference and Record are captured here (parse time never drops published
// data) even though the store schema has no column for them today — the
// ingest layer decides whether/how they're persisted (docs/sources/intermat.md).
// Grade is kept as published ("Sophomore"), not remapped to Flo's "SO" form.
// Previous is InterMat's own prior rank ("LAST"); empty when the edition's
// table lacks that column (the preseason shape has no prior week to show).
type Row struct {
	Rank       int
	Name       string // WRESTLER
	School     string // SCHOOL
	Grade      string // CLASS
	Conference string // CONFERENCE — optional column
	Record     string // RECORD (W-L) — optional column
	Previous   string // LAST — optional column
}

// column header labels, matched case-insensitively. The postseason final
// edition heads its rank column "FINISH" instead of "RANK" — both alias to
// the same canonical "rank" key in headerIndex (docs/sources/intermat.md).
var rankAliases = []string{"rank", "finish"}

const (
	colWrestler   = "wrestler"
	colSchool     = "school"
	colGrade      = "class"
	colConference = "conference"
	colRecord     = "record"
	colPrevious   = "last"
)

var weightBannerRe = regexp.MustCompile(`^(\d{3})\s*lbs?$`)

// ParseTable parses one <table>'s HTML into ranked rows. Accepts either a
// bare header+data table or a full InterMat table including its leading
// title-banner row (a single cell spanning every column, e.g. "125lbs") —
// the banner, if present, is detected and stripped before header-driven
// parsing, same rule ParseRecord applies per table on a full page. Kept as a
// convenience for direct/standalone testing; ParseRecord is the real entry
// point for a full record page (many tables, one call).
func ParseTable(tableHTML string) ([]Row, error) {
	node, err := html.Parse(strings.NewReader(tableHTML))
	if err != nil {
		return nil, fmt.Errorf("parse table html: %w", err)
	}
	tables := allTables(node)
	if len(tables) == 0 {
		return nil, fmt.Errorf("no table found")
	}

	rows := directRows(tables[0])
	if len(rows) == 0 {
		return nil, fmt.Errorf("no table rows found")
	}
	if _, ok := parseWeightBanner(rows[0]); ok {
		rows = rows[1:]
	}
	return ParseRows(rows)
}

// parseWeightBanner reports whether a row is InterMat's per-table title
// banner (one cell reading e.g. "125lbs") and, if so, the weight class it
// names. A non-matching banner — "Tournament Rankings", "Dual Rankings" — is
// how ParseRecord silently skips those out-of-scope tables (mirrors Flo's
// unknown-section-key skip in container.go).
func parseWeightBanner(cells []string) (int, bool) {
	if len(cells) != 1 {
		return 0, false
	}
	m := weightBannerRe.FindStringSubmatch(strings.ToLower(cells[0]))
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	return n, true
}

// ParseRows is the header-driven engine shared by ParseTable and the ingest
// layer (which calls it per RawWeightTable — see internal/ingest/intermat.go):
// rows[0] is the header row (supplies column order — read by name, never
// fixed position, because the column set varies by edition), the rest are
// data. RANK/FINISH + WRESTLER + SCHOOL are required; CLASS/CONFERENCE/
// RECORD/LAST are optional. A ragged row (cell count != header count) fails
// loud, same hard rule as scraper.ParseTable — raw_source_string is never
// silently dropped.
func ParseRows(rows [][]string) ([]Row, error) {
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

// headerIndex maps lowercased header labels to their column position. Either
// alias for the rank column ("rank" or "finish") is also canonicalized under
// the key "rank", so buildRow always reads one key regardless of which label
// the edition used. A duplicate non-empty label is rejected rather than
// silently overwriting the earlier column.
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
	for _, alias := range rankAliases {
		if i, ok := index[alias]; ok {
			index["rank"] = i
			break
		}
	}
	for _, required := range []string{"rank", colWrestler, colSchool} {
		if _, ok := index[required]; !ok {
			return nil, fmt.Errorf("missing required column %q in header %v", required, header)
		}
	}
	return index, nil
}

// finishCodeRanks maps NCAA championship "did not place" bracket-elimination
// codes to a representative tie-group rank — observed live in the real
// 2025-26 postseason-final edition (r76), where the FINISH column is
// numeric (1-8) only for the eight actual placers; the other eight of each
// weight's 16 rows carry a code instead. R12 = eliminated short of placing
// after reaching the round that decides 5th-8th (conventionally reported as
// tied 9th-12th); R16 = eliminated one round earlier (tied 13th-16th).
// Mapped to the tie group's START rank, matching this codebase's existing
// competition-ranking convention for ties (schema.md §7 — e.g. two wrestlers
// tied for 2nd both show rank 2). Only these two observed codes are mapped;
// an unrecognized code still fails loud rather than being guessed at.
var finishCodeRanks = map[string]int{
	"R12": 9,
	"R16": 13,
}

func buildRow(cells []string, index map[string]int) (Row, error) {
	get := func(col string) string {
		i, ok := index[col]
		if !ok || i >= len(cells) {
			return ""
		}
		return cells[i]
	}

	rankStr := get("rank")
	rank, err := strconv.Atoi(rankStr)
	if err != nil {
		if mapped, ok := finishCodeRanks[strings.ToUpper(rankStr)]; ok {
			rank = mapped
		} else {
			return Row{}, fmt.Errorf("non-numeric rank %q", rankStr)
		}
	}
	return Row{
		Rank:       rank,
		Name:       get(colWrestler),
		School:     get(colSchool),
		Grade:      get(colGrade),
		Conference: get(colConference),
		Record:     get(colRecord),
		Previous:   get(colPrevious),
	}, nil
}
