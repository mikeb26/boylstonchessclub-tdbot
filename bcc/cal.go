/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package bcc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/mikeb26/boylstonchessclub-tdbot/internal"
)

// vended by https://beta.boylstonchess.org/api/events
// Event represents a summary of an event in the Boylston Chess API
type Event struct {
	EventID     int       `json:"eventId"`
	Title       string    `json:"title"`
	Date        time.Time `json:"date"`
	StartDate   time.Time `json:"startDate"`
	EndDate     time.Time `json:"endDate"`
	DayOfWeek   string    `json:"dayOfWeek"`
	DateDisplay string    `json:"dateDisplay"`
}

// GetEvents fetches events from the Boylston Chess API and returns a slice
// of Event.
func GetEvents() ([]Event, error) {
	const url = "https://beta.boylstonchess.org/api/events"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch bcc events (new): %w", err)
	}
	req.Header.Set("User-Agent", internal.UserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch bcc events (do): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unable to fetch bcc events (http): %v", resp.StatusCode)
	}

	var events []Event
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		return nil, fmt.Errorf("unable to parse bcc events: %w", err)
	}

	return events, nil
}

// Custom unmarshaller to handle non-RFC3339 timestamps, "null", and empty strings.
func (e *Event) UnmarshalJSON(data []byte) error {
	type Alias Event
	aux := &struct {
		Date      string `json:"date"`
		StartDate string `json:"startDate"`
		EndDate   string `json:"endDate"`
		*Alias
	}{
		Alias: (*Alias)(e),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("Event unmarshal: %w", err)
	}
	var err error
	// Parse Date
	e.Date, err = parseDateOrZero(aux.Date)
	if err != nil {
		return fmt.Errorf("parsing Event.Date: %w", err)
	}
	// Parse StartDate
	e.StartDate, err = parseDateOrZero(aux.StartDate)
	if err != nil {
		return fmt.Errorf("parsing Event.StartDate: %w", err)
	}
	// Parse EndDate
	e.EndDate, err = parseDateOrZero(aux.EndDate)
	if err != nil {
		return fmt.Errorf("parsing Event.EndDate: %w", err)
	}
	return nil
}
