package intermat

import (
	"bytes"
	"fmt"

	"golang.org/x/net/html"
)

// RawWeightTable is one weight class's UN-PARSED table rows from a record
// page: the header row (Rows[0]) plus every data row, each as raw cell text.
// Row parsing (header validation, per-row struct building via ParseRows) is
// deliberately deferred to the ingest layer (internal/ingest/intermat.go) —
// exactly how scraper.Edition keeps its Content as raw HTML for
// ingest.ingestOne to parse per-edition. That deferral is what lets one
// weight's malformed table be isolated as a single ingest.EditionFailure
// without aborting the whole record — the same per-item isolation contract
// ingest.Container already provides for Flo.
type RawWeightTable struct {
	WeightClass int
	Rows        [][]string
}

// Record is one point-in-time InterMat rankings record: a resolved
// PublishedDate plus every weight's raw table. InterMat's equivalent of
// Flo's Container, but structurally simpler — one Record IS one edition (all
// weights), not many dated editions bundled in one blob, because a Wayback
// snapshot already gives one blob PER date via separate fetches.
// PublishedDate is not set by ParseRecord — the page carries no reliable
// date of its own; callers derive it externally (see date.go) and assemble
// the Record.
type Record struct {
	PublishedDate string
	Weights       []RawWeightTable
}

// ParseRecord splits a full InterMat rankings-record page into one
// RawWeightTable per recognized weight class. A table whose banner isn't a
// recognizable "NNNlbs" label (Tournament/Dual tabs; DII/DIII are different
// record pages entirely, not tables within this one) is silently skipped —
// mirrors how Flo's unknown ranking-section keys are skipped
// (scraper/container.go's sectionWeights). Fetch-source-agnostic: works
// identically on live InterMat HTML or a Wayback snapshot (Wayback's
// asset-URL rewriting leaves <table> markup intact — docs/sources/intermat.md).
//
// The returned error is reserved for a systemic failure (no recognizable
// weight tables at all — e.g. the wrong page was fetched); a single
// malformed table's rows are returned as-is (ParseRows on them fails later,
// per weight, in the ingest layer) rather than aborting every other weight
// in the same record.
func ParseRecord(page []byte) ([]RawWeightTable, error) {
	node, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return nil, fmt.Errorf("parse record html: %w", err)
	}

	var out []RawWeightTable
	for _, table := range allTables(node) {
		rows := directRows(table)
		if len(rows) == 0 {
			continue
		}
		weightClass, ok := parseWeightBanner(rows[0])
		if !ok {
			continue // Tournament/Dual or other non-weight table — out of scope
		}
		out = append(out, RawWeightTable{WeightClass: weightClass, Rows: rows[1:]})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no weight tables found")
	}
	return out, nil
}
