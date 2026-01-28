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
	"net/url"
	"strconv"
	"time"

	"github.com/mikeb26/boylstonchessclub-tdbot/internal"
)

type EventID int

type Event struct {
	EndDate time.Time
	Name    string
	ID      EventID
}

type apiAffiliateEventsResponse struct {
	Items []struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		EndDate string `json:"endDate"`
	} `json:"items"`
	Offset      int  `json:"offset"`
	PageSize    int  `json:"pageSize"`
	HasNextPage bool `json:"hasNextPage"`
	HasPrevPage bool `json:"hasPreviousPage"`
}

// GetAffiliateEvents fetches and parses the Affiliate Tournament History page
// for the given affiliate code and returns a slice of Event.
func (client *Client) GetAffiliateEvents(ctx context.Context,
	affiliateCode string) ([]Event, error) {
	apiBase, err := url.Parse("https://ratings-api.uschess.org")
	if err != nil {
		return nil, err
	}

	var events []Event
	const pageSize = 100
	offset := 0

	for {
		eventsURL := apiBase.ResolveReference(&url.URL{Path: "/api/v1/affiliates/" + url.PathEscape(affiliateCode) + "/events"})
		q := eventsURL.Query()
		q.Set("offset", strconv.Itoa(offset))
		q.Set("pageSize", strconv.Itoa(pageSize))
		eventsURL.RawQuery = q.Encode()

		eventsReq, err := http.NewRequestWithContext(ctx, "GET", eventsURL.String(), nil)
		if err != nil {
			return nil, err
		}
		eventsReq.Header.Set("User-Agent", internal.UserAgent)
		eventsReq.Header.Set("Accept", "application/json")

		eventsResp, err := client.httpClient1day.Do(eventsReq)
		if err != nil {
			return nil, err
		}
		respBody, readErr := io.ReadAll(eventsResp.Body)
		eventsResp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}

		if eventsResp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %d fetching %s: %s", eventsResp.StatusCode, eventsURL.String(), string(respBody))
		}

		var eventsData apiAffiliateEventsResponse
		if err := json.Unmarshal(respBody, &eventsData); err != nil {
			return nil, fmt.Errorf("decoding affiliate events JSON from %s: %w", eventsURL.String(), err)
		}

		for _, item := range eventsData.Items {
			idInt, err := strconv.Atoi(item.ID)
			if err != nil {
				// Skip events with invalid IDs
				continue
			}
			endDate, _ := internal.ParseDateOrZero(item.EndDate)
			events = append(events, Event{
				EndDate: endDate,
				Name:    item.Name,
				ID:      EventID(idInt),
			})
		}

		if !eventsData.HasNextPage {
			break
		}
		offset += pageSize
	}

	return events, nil
}
