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
