package intermat

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// CDXSnapshot is one Wayback Machine capture of an InterMat rankings-record
// page, as returned by the CDX API.
type CDXSnapshot struct {
	Timestamp   string // e.g. "20260106175039"
	OriginalURL string // e.g. "https://intermatwrestle.com/rankings.html/ncaa-di-r63/"
}

// CDXQuery scopes a CDX search. URLPattern must be specific enough to avoid
// unrelated matches — "intermatwrestle.com/rankings.html/ncaa-di*" (no
// trailing "-r") is a trap: as a CDX prefix match it also matches
// "ncaa-dii-*" and "ncaa-diii-*", since "ncaa-di" is a literal string prefix
// of both (confirmed the hard way — see intermat's git history). Use
// "ncaa-di-r*" to scope to DI only. From/To (YYYYMMDD, both optional) scope
// by capture date — CDX has NO season/record-id-range concept of its own, so
// without a date bound a query like this also returns every other season
// InterMat has ever published, not just the one being backfilled.
type CDXQuery struct {
	URLPattern string
	From, To   string
}

// FetchCDX queries the CDX API for every 200-status capture matching q and
// returns them parsed. Verified against the real endpoint (2026-09-12):
// plain space-separated text, one capture per line, fields urlkey/timestamp/
// original/mimetype/statuscode/digest/length — no &output=json needed.
// collapse=urlkey asks the API to keep only the first (earliest, since
// results are timestamp-ordered within a urlkey) capture per record id
// server-side, matching what DedupeByRecordID also enforces client-side.
func FetchCDX(ctx context.Context, client *http.Client, userAgent string, q CDXQuery) ([]CDXSnapshot, error) {
	u := "https://web.archive.org/cdx/search/cdx?url=" + q.URLPattern +
		"&filter=statuscode:200&collapse=urlkey"
	if q.From != "" {
		u += "&from=" + q.From
	}
	if q.To != "" {
		u += "&to=" + q.To
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch CDX %+v: %w", q, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch CDX %+v: status %d", q, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return parseCDX(body)
}

// parseCDX parses the CDX API's plain-text response. Pure and
// unit-testable without a network fetch.
func parseCDX(body []byte) ([]CDXSnapshot, error) {
	var out []CDXSnapshot
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return nil, fmt.Errorf("malformed CDX line (want >=3 fields): %q", line)
		}
		out = append(out, CDXSnapshot{Timestamp: fields[1], OriginalURL: fields[2]})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan CDX response: %w", err)
	}
	return out, nil
}

// DedupeByRecordID keeps the earliest 200-status capture per distinct
// OriginalURL (a single weekly record is typically captured 2-3 times by
// Wayback before InterMat replaces it — same content each time, so
// re-fetching every capture wastes archive.org requests for zero new data).
// A defensive, independently testable pass — FetchCDX already requests
// collapse=urlkey server-side, but this is the safety net if a caller
// queries CDX without that param, or feeds results from elsewhere.
func DedupeByRecordID(snapshots []CDXSnapshot) []CDXSnapshot {
	earliest := make(map[string]CDXSnapshot, len(snapshots))
	for _, s := range snapshots {
		cur, ok := earliest[s.OriginalURL]
		if !ok || s.Timestamp < cur.Timestamp {
			earliest[s.OriginalURL] = s
		}
	}
	out := make([]CDXSnapshot, 0, len(earliest))
	for _, s := range earliest {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp < out[j].Timestamp })
	return out
}

// SnapshotURL builds the fetchable web.archive.org URL for one snapshot,
// using the "id_" modifier — raw captured bytes, no link-rewriting/banner
// injection. Confirmed necessary, not just tidier: the plain (unmodified)
// replay URL for at least one real 2025-26 capture (r57, 2025-11-19) 404s
// consistently despite being CDX-indexed as a 200, while its "id_" URL
// serves the real page fine — a Wayback playback quirk specific to the
// rewriting path, not the underlying capture. Verified "id_" also serves
// already-working captures (e.g. r63) identically, so this is a strict
// improvement, not a special case to maintain.
func SnapshotURL(s CDXSnapshot) string {
	return fmt.Sprintf("https://web.archive.org/web/%sid_/%s", s.Timestamp, s.OriginalURL)
}
