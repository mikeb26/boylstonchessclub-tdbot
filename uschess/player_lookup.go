/* Copyright © 2025-2026 Mike Brown. All Rights Reserved.
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
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/mikeb26/boylstonchessclub-tdbot/internal"
	"golang.org/x/sync/errgroup"
)

type MemID int

// Player holds information about a USCF member.
type Player struct {
	MemberID        MemID
	Name            string
	RegRating       string
	QuickRating     string
	BlitzRating     string
	RegSupplement   RatingSupplement
	QuickSupplement RatingSupplement
	BlitzSupplement RatingSupplement
	TotalEvents     int
	// up to 50
	RecentEvents []Event
}

type RatingSupplement struct {
	Rating string
	Date   time.Time
}

// apiMemberResponse represents the JSON response from the member API endpoint
type apiMemberResponse struct {
	ID            string `json:"id"`
	FideID        string `json:"fideId"`
	Gender        string `json:"gender"`
	Rank          int    `json:"rank"`
	StateRank     int    `json:"stateRank"`
	FirstName     string `json:"firstName"`
	LastName      string `json:"lastName"`
	StateRep      string `json:"stateRep"`
	Jurisdiction  string `json:"jurisdiction"`
	MemStatus     string `json:"status"`
	MemExpireDate string `json:"expirationDate"`
	MemUpdated    string `json:"lastChangedDate"`
	Ratings       []struct {
		Rating        int    `json:"rating"`
		RatingSystem  string `json:"ratingSystem"`
		IsProvisional bool   `json:"isProvisional"`
		GamesPlayed   int    `json:"gamesPlayed"`
		Floor         int    `json:"floor"`
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

type apiRatingSupplementsResponse struct {
	Items []struct {
		RatingSupplementDate string `json:"ratingSupplementDate"`
		Ratings              []struct {
			Source               string `json:"source"`
			Rating               *int   `json:"rating"`
			ProvisionalGameCount *int   `json:"provisionalGameCount"`
		} `json:"ratings"`
	} `json:"items"`
}

// FetchPlayer retrieves player information for the given USCF member ID using
// the ratings API (https://ratings-api.uschess.org/api/v1/members/).
func (client *Client) FetchPlayer(ctx context.Context,
	memberID MemID, fetchEvents bool) (*Player, error) {

	var apiMember *apiMemberResponse
	var apiMemberEvents *apiEventsResponse
	var apiRatingSupplements *apiRatingSupplementsResponse

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var err error
		apiMember, err = client.fetchMemberProfile(ctx, memberID)
		return err
	})

	if fetchEvents {
		g.Go(func() error {
			var err error
			apiMemberEvents, err = client.fetchMemberEvents(ctx, memberID)
			return err
		})
	}

	g.Go(func() error {
		var err error
		apiRatingSupplements, err = client.fetchMemberRatingSupplements(ctx,
			memberID)
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	player := apiMemberToPlayer(memberID, apiMember)
	if fetchEvents {
		addApiMemberEventsToPlayer(player, apiMemberEvents)
	}
	addApiRatingSupplementsToPlayer(player, apiRatingSupplements)

	return player, nil
}

func (client *Client) fetchMemberProfile(ctx context.Context,
	memberID MemID) (*apiMemberResponse, error) {

	profileEndpoint :=
		fmt.Sprintf("https://ratings-api.uschess.org/api/v1/members/%v",
			memberID)
	req, err := http.NewRequestWithContext(ctx, "GET", profileEndpoint, nil)
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
		return nil, fmt.Errorf("unexpected profile status %d: %s",
			resp.StatusCode, string(body))
	}

	var memberData apiMemberResponse
	if err := json.NewDecoder(resp.Body).Decode(&memberData); err != nil {
		return nil, fmt.Errorf("decoding profile JSON: %w", err)
	}

	return &memberData, nil
}

func (client *Client) fetchMemberEvents(ctx context.Context,
	memberID MemID) (*apiEventsResponse, error) {

	eventsEndpoint :=
		fmt.Sprintf("https://ratings-api.uschess.org/api/v1/members/%v/events",
			memberID)
	eventsReq, err := http.NewRequestWithContext(ctx, "GET", eventsEndpoint, nil)
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
		return nil, fmt.Errorf("unexpected events status %d: %s",
			eventsResp.StatusCode, string(body))
	}

	var eventsData apiEventsResponse
	if err := json.NewDecoder(eventsResp.Body).Decode(&eventsData); err != nil {
		return nil, fmt.Errorf("decoding events JSON: %w", err)
	}

	return &eventsData, nil
}

func (client *Client) fetchMemberRatingSupplements(ctx context.Context,
	memberID MemID) (*apiRatingSupplementsResponse, error) {

	supplementsURL, err := url.Parse(fmt.Sprintf("https://ratings-api.uschess.org/api/v1/members/%v/rating-supplements",
		memberID))
	if err != nil {
		return nil, fmt.Errorf("parsing rating supplements URL: %w", err)
	}
	q := supplementsURL.Query()
	q.Set("Size", "1")
	supplementsURL.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET",
		supplementsURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating rating supplements request: %w", err)
	}
	req.Header.Set("User-Agent", internal.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := client.httpClient1day.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing rating supplements HTTP GET: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected rating supplements status %d: %s",
			resp.StatusCode, string(body))
	}

	var supplementsData apiRatingSupplementsResponse
	if err := json.NewDecoder(resp.Body).Decode(&supplementsData); err != nil {
		return nil, fmt.Errorf("decoding rating supplements JSON: %w", err)
	}

	return &supplementsData, nil
}

func apiMemberToPlayer(memberID MemID, memberData *apiMemberResponse) *Player {
	player := &Player{
		MemberID: memberID,
		Name: internal.NormalizeName(memberData.FirstName + " " +
			memberData.LastName),
		RegRating:   "<unrated>",
		QuickRating: "<unrated>",
		BlitzRating: "<unrated>",
	}

	// Extract ratings
	for _, rating := range memberData.Ratings {
		switch rating.RatingSystem {
		case "R":
			if rating.Rating != 0 {
				if rating.IsProvisional {
					player.RegRating = fmt.Sprintf("%vP%v", rating.Rating,
						rating.GamesPlayed)
				} else {
					player.RegRating = strconv.Itoa(rating.Rating)
				}
			}
		case "Q":
			if rating.Rating != 0 {
				if rating.IsProvisional {
					player.QuickRating = fmt.Sprintf("%vP%v", rating.Rating,
						rating.GamesPlayed)
				} else {
					player.QuickRating = strconv.Itoa(rating.Rating)
				}
			}
		case "B":
			if rating.Rating != 0 {
				if rating.IsProvisional {
					player.BlitzRating = fmt.Sprintf("%vP%v", rating.Rating,
						rating.GamesPlayed)
				} else {
					player.BlitzRating = strconv.Itoa(rating.Rating)
				}
			}
		}
	}

	return player
}

func addApiRatingSupplementsToPlayer(player *Player,
	supplementsData *apiRatingSupplementsResponse) {

	if supplementsData == nil || len(supplementsData.Items) == 0 {
		return
	}

	latest := supplementsData.Items[0]
	supplementDate, err := internal.ParseDateOrZero(latest.RatingSupplementDate)
	if err != nil {
		return
	}

	for _, rating := range latest.Ratings {
		supplement := RatingSupplement{
			Rating: formatRating(rating.Rating, rating.ProvisionalGameCount),
			Date:   supplementDate,
		}

		switch rating.Source {
		case "R":
			player.RegSupplement = supplement
		case "Q":
			player.QuickSupplement = supplement
		case "B":
			player.BlitzSupplement = supplement
		}
	}
}

func formatRating(rating *int, provisionalGameCount *int) string {
	if rating == nil || *rating == 0 {
		return "<unrated>"
	}
	if provisionalGameCount != nil {
		return fmt.Sprintf("%vP%v", *rating, *provisionalGameCount)
	}
	return strconv.Itoa(*rating)
}

func addApiMemberEventsToPlayer(player *Player, eventsData *apiEventsResponse) {
	player.TotalEvents = len(eventsData.Items)
	for _, item := range eventsData.Items {
		eventID, err := strconv.Atoi(item.ID)
		if err != nil {
			// Skip events with invalid IDs
			continue
		}
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
}
