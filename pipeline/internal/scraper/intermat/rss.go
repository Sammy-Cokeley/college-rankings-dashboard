package intermat

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
)

const rssURL = "https://intermatwrestle.com/articles.html/college/?d=1&rss=1"

// LatestEdition is the most recent DI rankings edition discoverable via
// InterMat's RSS feed: the record page to fetch, and the precise publish
// date pinned by the announcing article's title — the record page itself
// carries no date of its own (docs/sources/intermat.md), so this is the
// only reliable source of one for live (non-Wayback) fetching.
type LatestEdition struct {
	PublishedDate string // YYYY-MM-DD
	RecordURL     string
}

// diTitleRe matches a DI-rankings-announcement RSS title carrying a
// "(M/D/YYYY)" date suffix. InterMat does NOT use one consistent title
// across the season — "NCAA DI Rankings Updated (1/6/2026)" in-season vs.
// "NCAA DI Preseason Rankings Released (9/17/2026)" at kickoff, both
// confirmed live — so this matches loosely on "NCAA DI ... Rankings ...
// (date)" rather than one fixed phrase. A title with a date suffix but no
// "NCAA DI"/"Rankings" (e.g. "The 2026 Offseason Coaching Carousel
// (9/9/2026)", also observed live) must NOT match.
var diTitleRe = regexp.MustCompile(`^NCAA DI .*Rankings.*\((\d{1,2})/(\d{1,2})/(\d{4})\)`)

// diRecordLinkRe finds the DI record URL embedded in an item's body. Every
// observed rankings-announcement article links straight to the record that
// was current at publish (docs/sources/intermat.md).
var diRecordLinkRe = regexp.MustCompile(`https://intermatwrestle\.com/rankings\.html/ncaa-di-r\d+/`)

type rssItem struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
}

type rssFeed struct {
	Items []rssItem `xml:"channel>item"`
}

// FetchLatestEdition fetches InterMat's articles RSS feed and returns the
// most recent DI rankings announcement's record URL and precise publish
// date.
func FetchLatestEdition(ctx context.Context, client *http.Client, userAgent string) (LatestEdition, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rssURL, nil)
	if err != nil {
		return LatestEdition{}, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return LatestEdition{}, fmt.Errorf("fetch RSS: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return LatestEdition{}, fmt.Errorf("fetch RSS: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return LatestEdition{}, err
	}
	return parseLatestEdition(body)
}

// parseLatestEdition is the pure parsing half, unit-testable without a
// network fetch. RSS items are newest-first (standard convention, confirmed
// live), so the first title match wins — an older matching item further
// down the feed must never override a newer one.
func parseLatestEdition(body []byte) (LatestEdition, error) {
	var feed rssFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return LatestEdition{}, fmt.Errorf("parse RSS: %w", err)
	}
	for _, item := range feed.Items {
		m := diTitleRe.FindStringSubmatch(item.Title)
		if m == nil {
			continue
		}
		link := diRecordLinkRe.FindString(item.Description)
		if link == "" {
			// A DI rankings announcement with no discoverable record link is
			// unexpected — skip rather than guess, try the next item.
			continue
		}
		date, err := titleDate(m)
		if err != nil {
			return LatestEdition{}, err
		}
		return LatestEdition{PublishedDate: date, RecordURL: link}, nil
	}
	return LatestEdition{}, fmt.Errorf("no DI rankings announcement found in feed")
}

func titleDate(m []string) (string, error) {
	month, err := strconv.Atoi(m[1])
	if err != nil {
		return "", fmt.Errorf("parse month %q: %w", m[1], err)
	}
	day, err := strconv.Atoi(m[2])
	if err != nil {
		return "", fmt.Errorf("parse day %q: %w", m[2], err)
	}
	year, err := strconv.Atoi(m[3])
	if err != nil {
		return "", fmt.Errorf("parse year %q: %w", m[3], err)
	}
	return fmt.Sprintf("%04d-%02d-%02d", year, month, day), nil
}
