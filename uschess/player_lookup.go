/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package uschess

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/mikeb26/boylstonchessclub-tdbot/internal"
)

type MemID int

// Player holds information about a USCF member.
type Player struct {
	MemberID    MemID
	Name        string
	RegRating   string
	QuickRating string
	BlitzRating string
	TotalEvents int
	// up to 50
	RecentEvents []Event
}

// apiMemberResponse represents the JSON response from the member API endpoint
type apiMemberResponse struct {
	ID        string `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Ratings   []struct {
		Rating       int    `json:"rating"`
		RatingSystem string `json:"ratingSystem"`
	} `json:"ratings"`
}

// apiEventsResponse represents the JSON response from the events API endpoint
type apiEventsResponse struct {
	Items []struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		EndDate string `json:"endDate"`
	} `json:"items"`
}

// FetchPlayer retrieves player information for the given USCF member ID using
// the ratings API (https://ratings-api.uschess.org/api/v1/members/).
func (client *Client) FetchPlayer(ctx context.Context,
	memberID MemID) (*Player, error) {

	// Fetch member profile
	profileEndpoint := fmt.Sprintf("https://ratings-api.uschess.org/api/v1/members/%v", memberID)
	req, err := http.NewRequest("GET", profileEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating profile request: %w", err)
	}
	req.Header.Set("User-Agent", internal.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := client.httpClient1day.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing profile HTTP GET: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected profile status %d: %s", resp.StatusCode, string(body))
	}

	var memberData apiMemberResponse
	if err := json.NewDecoder(resp.Body).Decode(&memberData); err != nil {
		return nil, fmt.Errorf("decoding profile JSON: %w", err)
	}

	// Build player from profile data
	player := &Player{
		MemberID:    memberID,
		Name:        internal.NormalizeName(memberData.FirstName + " " + memberData.LastName),
		RegRating:   "<unrated>",
		QuickRating: "<unrated>",
		BlitzRating: "<unrated>",
	}

	// Extract ratings
	for _, rating := range memberData.Ratings {
		if rating.Rating == 0 {
			continue
		}
		ratingStr := strconv.Itoa(rating.Rating)
		switch rating.RatingSystem {
		case "R":
			player.RegRating = ratingStr
		case "Q":
			player.QuickRating = ratingStr
		case "B":
			player.BlitzRating = ratingStr
		}
	}

	// Fetch events
	eventsEndpoint := fmt.Sprintf("https://ratings-api.uschess.org/api/v1/members/%v/events", memberID)
	eventsReq, err := http.NewRequest("GET", eventsEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating events request: %w", err)
	}
	eventsReq.Header.Set("User-Agent", internal.UserAgent)
	eventsReq.Header.Set("Accept", "application/json")

	eventsResp, err := client.httpClient1day.Do(eventsReq)
	if err != nil {
		return nil, fmt.Errorf("performing events HTTP GET: %w", err)
	}
	defer eventsResp.Body.Close()

	if eventsResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(eventsResp.Body)
		return nil, fmt.Errorf("unexpected events status %d: %s", eventsResp.StatusCode, string(body))
	}

	var eventsData apiEventsResponse
	if err := json.NewDecoder(eventsResp.Body).Decode(&eventsData); err != nil {
		return nil, fmt.Errorf("decoding events JSON: %w", err)
	}

	// Convert events
	player.TotalEvents = len(eventsData.Items)
	for _, item := range eventsData.Items {
		eventID, _ := strconv.Atoi(item.ID)
		endDate, _ := internal.ParseDateOrZero(item.EndDate)
		player.RecentEvents = append(player.RecentEvents, Event{
			ID:      EventID(eventID),
			Name:    item.Name,
			EndDate: endDate,
		})
	}

	// Sort events by date (most recent first)
	sort.Slice(player.RecentEvents, func(i, j int) bool {
		return player.RecentEvents[j].EndDate.Before(player.RecentEvents[i].EndDate)
	})

	return player, nil
}

// parsePlayerName finds the player's name in a bold tag: "<b>memberID: NAME</b>".
func parsePlayerName(memberID MemID, doc *goquery.Document) string {
	name := ""
	doc.Find("b").EachWithBreak(func(i int, s *goquery.Selection) bool {
		text := strings.TrimSpace(s.Text())
		prefix := fmt.Sprintf("%v:", memberID)
		if strings.HasPrefix(text, prefix) {
			name = strings.TrimSpace(strings.TrimPrefix(text, prefix))
			name = internal.NormalizeName(name)
			return false // stop iteration
		}
		return true // continue
	})
	return name
}

// parseTotalEvents finds the total number of events listed on the page.
func parseTotalEvents(player *Player, doc *goquery.Document) {
	doc.Find("b").EachWithBreak(func(i int, s *goquery.Selection) bool {
		text := strings.TrimSpace(s.Text())
		if strings.HasPrefix(text, "Events for this player") {
			// Expect format: "Events for this player since late 1991: 583"
			parts := strings.Split(text, ":")
			if len(parts) >= 2 {
				numStr := strings.TrimSpace(parts[len(parts)-1])
				n, err := strconv.Atoi(numStr)
				if err == nil {
					player.TotalEvents = n
				}
			}
			return false // found, stop
		}
		return true // continue
	})
}

// parsePlayer parses HTML and extracts the player's name, current ratings, and event history.
func parsePlayer(memberID MemID, body io.ReadCloser) (*Player, error) {
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return nil, fmt.Errorf("parsing HTML: %w", err)
	}

	player := Player{MemberID: memberID}
	player.Name = parsePlayerName(memberID, doc)
	if player.Name == "" {
		return nil, fmt.Errorf("player name not found in page for %v", memberID)
	}

	// Populate total events
	parseTotalEvents(&player, doc)

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
		return fmt.Errorf("no tournament entries found for player %v", player.MemberID)
	}

	var events []Event
	rows.Each(func(_ int, row *goquery.Selection) {
		tds := row.Find("td")
		if tds.Length() < 5 {
			return
		}

		// Determine if this is an event row by parsing the small tag
		dateTd := tds.Eq(0)

		// Extract event ID, ignore any trailing non-digit suffix (e.g., "**")
		rawID := strings.TrimSpace(dateTd.Find("small").Text())
		fields := strings.Fields(rawID)
		if len(fields) == 0 {
			return
		}
		idStr := fields[0]
		id, err := strconv.Atoi(idStr)
		if err != nil {
			// skip header or non-event rows
			return
		}

		// Parse end date
		dateStr := strings.TrimSpace(
			dateTd.Contents().FilterFunction(func(i int, s *goquery.Selection) bool {
				return goquery.NodeName(s) == "#text"
			}).Text(),
		)
		endDate, err := internal.ParseDateOrZero(dateStr)
		if err != nil {
			endDate = time.Time{}
		}

		// Parse event name from second cell link
		name := strings.TrimSpace(tds.Eq(1).Find("a").Text())

		// Extract current ratings: first non-unrated encountered is
		// considered current
		if player.RegRating == "" {
			r := getRatingFromCell(tds.Eq(2))
			if r != "<unrated>" {
				player.RegRating = r
			}
		}
		if player.QuickRating == "" {
			q := getRatingFromCell(tds.Eq(3))
			if q != "<unrated>" {
				player.QuickRating = q
			}
		}
		if player.BlitzRating == "" {
			b := getRatingFromCell(tds.Eq(4))
			if b != "<unrated>" {
				player.BlitzRating = b
			}
		}

		events = append(events, Event{
			EndDate: endDate,
			Name:    name,
			ID:      EventID(id),
		})
	})

	// Ensure ratings are set
	if player.RegRating == "" {
		player.RegRating = "<unrated>"
	}
	if player.QuickRating == "" {
		player.QuickRating = "<unrated>"
	}
	if player.BlitzRating == "" {
		player.BlitzRating = "<unrated>"
	}

	sort.Slice(events, func(i, j int) bool {
		return events[j].EndDate.Before(events[i].EndDate)
	})
	player.RecentEvents = events
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
