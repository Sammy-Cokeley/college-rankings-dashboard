package wrestlestat

import (
	"strings"

	"golang.org/x/net/html"
)

// findByID returns the first descendant element (or root itself) whose id
// attribute equals id, or nil.
func findByID(root *html.Node, id string) *html.Node {
	if root.Type == html.ElementNode && attr(root, "id") == id {
		return root
	}
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		if found := findByID(c, id); found != nil {
			return found
		}
	}
	return nil
}

// firstElement returns the first descendant element with the given tag name,
// or nil. Used to reach into a cell for its <a> specifically (see roster.go).
func firstElement(root *html.Node, tag string) *html.Node {
	if root.Type == html.ElementNode && root.Data == tag {
		return root
	}
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		if found := firstElement(c, tag); found != nil {
			return found
		}
	}
	return nil
}

func attr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

// textOf concatenates all descendant text of a node (cells may wrap content in
// stray inline markup — <span>, <b>, etc.). Mirrors internal/scraper's helper
// of the same name; duplicated rather than imported because internal/scraper's
// version is unexported and this is a sibling package, not a subpackage of it.
func textOf(n *html.Node) string {
	var b []byte
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b = append(b, n.Data...)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return string(b)
}

// normalizeCell turns non-breaking spaces into ordinary spaces, collapses
// internal whitespace runs, and trims. html.Parse already decodes entities, so
// &nbsp; arrives here as U+00A0. Mirrors internal/scraper's helper of the same
// name (see textOf's doc comment for why it's duplicated, not imported).
func normalizeCell(s string) string {
	s = strings.ReplaceAll(s, " ", " ")
	return strings.Join(strings.Fields(s), " ")
}
