package scraper

import (
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

// appStateScriptID is the id of the <script> tag holding Angular's
// server-rendered transfer state on a FloWrestling rankings page.
const appStateScriptID = "flo-app-state"

// entitySubs are FloWrestling's transfer-state character escapes, in the order
// they must be reversed. "&a;" -> "&" is applied LAST: it is the ampersand
// escape, so decoding it earlier would corrupt the other tokens
// (docs/sources/flowrestling.md).
var entitySubs = []struct{ encoded, decoded string }{
	{"&q;", `"`},
	{"&l;", "<"},
	{"&g;", ">"},
	{"&s;", "'"},
	{"&a;", "&"},
}

// DecodeAppState extracts the ranking container from a live FloWrestling page.
// It finds the <script id="flo-app-state"> blob, reverses the entity escaping,
// unmarshals the transfer-state map, locates the ranking-containers entry, and
// returns its .body.data as a Container.
//
// This is the live-page path. The committed fixture is already decoded to the
// container level, so tests of container/table logic use ParseContainer
// directly; DecodeAppState is exercised with a small synthetic page.
func DecodeAppState(page []byte) (Container, error) {
	blob, err := extractAppState(page)
	if err != nil {
		return Container{}, err
	}

	decoded := blob
	for _, sub := range entitySubs {
		decoded = strings.ReplaceAll(decoded, sub.encoded, sub.decoded)
	}

	return containerFromTransferState([]byte(decoded))
}

// extractAppState returns the raw text content of the flo-app-state script tag.
func extractAppState(page []byte) (string, error) {
	node, err := html.Parse(strings.NewReader(string(page)))
	if err != nil {
		return "", fmt.Errorf("parse page html: %w", err)
	}

	var blob string
	var found bool
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found {
			return
		}
		if n.Type == html.ElementNode && n.Data == "script" && attr(n, "id") == appStateScriptID {
			blob = textOf(n)
			found = true
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(node)

	if !found {
		return "", fmt.Errorf("no <script id=%q> in page", appStateScriptID)
	}
	return blob, nil
}

// containerFromTransferState parses the decoded transfer-state JSON. The object
// is keyed by "G.<api-url>"; the ranking container is the value whose key
// contains "ranking-containers", with the payload at .body.data.
//
// The map is decoded value-by-value (json.RawMessage), not into one strongly
// typed struct map: Angular's transfer state mixes value shapes — alongside the
// API-response objects, some entries are bare strings/bools — so unmarshalling
// every value into the body/data struct would fail the whole document. Only the
// matched ranking-containers entry is typed.
func containerFromTransferState(decoded []byte) (Container, error) {
	var state map[string]json.RawMessage
	if err := json.Unmarshal(decoded, &state); err != nil {
		return Container{}, fmt.Errorf("unmarshal transfer state: %w", err)
	}

	for key, raw := range state {
		if !strings.Contains(key, "ranking-containers") {
			continue
		}
		var entry struct {
			Body struct {
				Data json.RawMessage `json:"data"`
			} `json:"body"`
		}
		if err := json.Unmarshal(raw, &entry); err != nil {
			return Container{}, fmt.Errorf("unmarshal ranking-containers entry %q: %w", key, err)
		}
		if len(entry.Body.Data) == 0 {
			return Container{}, fmt.Errorf("ranking-containers entry %q has no body.data", key)
		}
		return ParseContainer(entry.Body.Data)
	}
	return Container{}, fmt.Errorf("no ranking-containers key in transfer state")
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
