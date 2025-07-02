/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package uschess

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/mikeb26/boylstonchessclub-tdbot/internal"
)

// Player holds information about a USCF member.
type Player struct {
	MemberID    string
	Name        string
	RegRating   string
	QuickRating string
	BlitzRating string
	Events      []Event
}

// FetchPlayer retrieves player information for the given USCF member ID using
// the "Member Tournament History" endpoint
// (https://www.uschess.org/msa/MbrDtlTnmtHst.php). I alternatively tested
// https://www.uschess.org/msa/thin.php, https://www.uschess.org/msa/thin3.php,
// and ttps://www.uschess.org/msa/MbrDtlMain.php but all had terrible
// latency (>2s). I also considered
// https://new.uschess.org/civicrm/player-search but this seems like it would
// have required a headless browser to utilize.
func FetchPlayer(memberID string) (*Player, error) {
	endpoint := fmt.Sprintf("https://www.uschess.org/msa/MbrDtlTnmtHst.php?%s", memberID)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", internal.UserAgent)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing HTTP GET: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return parsePlayer(memberID, resp.Body)
}

// parsePlayerName finds the player's name in a bold tag: "<b>memberID: NAME</b>".
func parsePlayerName(memberID string, doc *goquery.Document) string {
	name := ""
	doc.Find("b").EachWithBreak(func(i int, s *goquery.Selection) bool {
		text := strings.TrimSpace(s.Text())
		prefix := memberID + ":"
		if strings.HasPrefix(text, prefix) {
			name = strings.TrimSpace(strings.TrimPrefix(text, prefix))
			name = normalizeName(name)
			return false // stop iteration
		}
		return true // continue
	})
	return name
}

// parsePlayer parses HTML and extracts the player's name, current ratings, and event history.
func parsePlayer(memberID string, body io.ReadCloser) (*Player, error) {
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return nil, fmt.Errorf("parsing HTML: %w", err)
	}

	player := Player{MemberID: memberID}
	player.Name = parsePlayerName(memberID, doc)
	if player.Name == "" {
		return nil, fmt.Errorf("player name not found in page for %s", memberID)
	}

	if err := parseTournamentHistory(&player, doc); err != nil {
		return nil, err
	}
	return &player, nil
}

// parseTournamentHistory fills the Player's Events slice and current ratings.
func parseTournamentHistory(player *Player, doc *goquery.Document) error {
	// Find the tournament history table
	table := doc.Find("table[border='1'][width='960']").First()
	if table.Length() == 0 {
		return fmt.Errorf("tournament history table not found")
	}

	rows := table.Find("tr")
	if rows.Length() <= 1 {
		return fmt.Errorf("no tournament entries found for player %s", player.MemberID)
	}

	var events []Event
	rows.Each(func(i int, row *goquery.Selection) {
		// Skip header row
		if i == 0 {
			return
		}
		tds := row.Find("td")
		if tds.Length() < 5 {
			return
		}

		// Parse end date and ID from first cell
		dateTd := tds.Eq(0)
		// Text node before <small>
		dateStr := strings.TrimSpace(
			dateTd.Contents().FilterFunction(func(i int, s *goquery.Selection) bool {
				return goquery.NodeName(s) == "#text"
			}).Text(),
		)
		id := strings.TrimSpace(dateTd.Find("small").Text())
		endDate, err := internal.ParseDateOrZero(dateStr)
		if err != nil {
			endDate = time.Time{}
		}

		// Parse event name from second cell link
		name := strings.TrimSpace(tds.Eq(1).Find("a").Text())

		// On first entry, extract current ratings
		if i == 1 {
			player.RegRating = getRatingFromCell(tds.Eq(2))
			player.QuickRating = getRatingFromCell(tds.Eq(3))
			player.BlitzRating = getRatingFromCell(tds.Eq(4))
		}

		events = append(events, Event{
			EndDate: endDate,
			Name:    name,
			ID:      id,
		})
	})
	sort.Slice(events, func(i, j int) bool {
		return events[j].EndDate.Before(events[i].EndDate)
	})
	player.Events = events
	return nil
}

// getRatingFromCell extracts the post-event rating (bold) or returns trimmed cell text.
func getRatingFromCell(sel *goquery.Selection) string {
	ret := ""

	b := sel.Find("b").First()
	if b.Length() > 0 {
		ret = strings.TrimSpace(b.Text())
	} else {
		ret = strings.TrimSpace(sel.Text())
	}

	if ret == "" {
		ret = "<unrated>"
	}

	return ret
}
