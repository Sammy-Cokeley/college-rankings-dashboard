// Package wrestlestat is a pure fetch+parse layer for WrestleStat team rosters
// (docs/sources/wrestlestat.md): no DB access here, same separation as
// internal/scraper for FloWrestling. Team()/Roster() take already-fetched page
// bytes; the actual net/http calls live in cmd/roster.
package wrestlestat

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// Team is one D1 program from the team-select dropdown.
type Team struct {
	ID   int
	Name string
}

// ParseTeamList extracts every team from the team-select page's
// <select id="homepage-team_select"> option list (docs/sources/wrestlestat.md:
// one page, all D1 programs, no pagination). The blank "Select a team..."
// option (value="") is skipped.
func ParseTeamList(page []byte) ([]Team, error) {
	node, err := html.Parse(strings.NewReader(string(page)))
	if err != nil {
		return nil, fmt.Errorf("parse team select page: %w", err)
	}

	sel := findByID(node, "homepage-team_select")
	if sel == nil {
		return nil, fmt.Errorf("team select page: #homepage-team_select not found")
	}

	var teams []Team
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "option" {
			val := attr(n, "value")
			if val != "" {
				id, err := strconv.Atoi(val)
				if err == nil {
					teams = append(teams, Team{ID: id, Name: normalizeCell(textOf(n))})
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(sel)

	if len(teams) == 0 {
		return nil, fmt.Errorf("team select page: no <option> teams found")
	}
	return teams, nil
}
