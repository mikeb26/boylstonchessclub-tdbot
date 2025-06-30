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

// vended by https://beta.boylstonchess.org/api/event/<eventId>
// EventDetail represents detailed information about a specific event.
type EventDetail struct {
	EventID             int       `json:"eventId"`
	Title               string    `json:"title"`
	StartDate           time.Time `json:"startDate"`
	EndDate             time.Time `json:"endDate"`
	Dates               []string  `json:"dates"`
	DateDisplay         string    `json:"dateDisplay"`
	Description         string    `json:"description"`
	DescriptionHTML     string    `json:"descriptionHtml"`
	Sections            []string  `json:"sections"`
	SectionDisplay      string    `json:"sectionDisplay"`
	IsRegistrationOpen  bool      `json:"isRegistrationOpen"`
	RegistrationEndDate time.Time `json:"registrationEndDate"`
	EntryFeeSummary     string    `json:"entryFeeSummary"`
	PrizeSummary        string    `json:"prizeSummary"`
	EventFormat         string    `json:"eventFormat"`
	TimeControl         string    `json:"timeControl"`
	RegistrationTime    string    `json:"registrationTime"`
	RoundTimes          string    `json:"roundTimes"`
	CreationDate        time.Time `json:"creationDate"`
	LastChangeDate      time.Time `json:"lastChangeDate"`
	NumEntries          int       `json:"numEntries"`
	Entries             []Entry   `json:"entries"`
}

// Entry represents a single registration entry for an event.
type Entry struct {
	FirstName           string    `json:"firstName"`
	LastName            string    `json:"lastName"`
	UscfID              int       `json:"uscfId"`
	ChessTitle          string    `json:"chessTitle"`
	WomensChessTitle    string    `json:"womensChessTitle"`
	UscfPeakRating      int       `json:"uscfPeakRating"`
	SectionName         string    `json:"sectionName"`
	RegistrationDate    time.Time `json:"registrationDate"`
	ByeRequests         string    `json:"byeRequests"`
	PrimaryRating       string    `json:"primaryRating"`
	PrimaryRatingType   string    `json:"primaryRatingType"`
	PrimaryRatingDate   string    `json:"primaryRatingDate"`
	SecondaryRating     string    `json:"secondaryRating"`
	SecondaryRatingType string    `json:"secondaryRatingType"`
	SecondaryRatingDate string    `json:"secondaryRatingDate"`
}

// GetEventDetail fetches detailed event info from the Boylston Chess API
// for a given eventId and returns an EventDetail.
func GetEventDetail(eventId int64) (EventDetail, error) {
	url := fmt.Sprintf("https://beta.boylstonchess.org/api/event/%d", eventId)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return EventDetail{}, fmt.Errorf("unable to fetch bcc event detail (new): %w", err)
	}
	req.Header.Set("User-Agent", internal.UserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return EventDetail{}, fmt.Errorf("unable to fetch bcc event detail (do): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return EventDetail{}, fmt.Errorf("unable to fetch bcc event detail (http): %v", resp.StatusCode)
	}

	var detail EventDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return EventDetail{}, fmt.Errorf("unable to parse bcc event detail: %w", err)
	}

	return detail, nil
}

// Custom unmarshaller for EventDetail to handle flexible date parsing.
func (ed *EventDetail) UnmarshalJSON(data []byte) error {
	type Alias EventDetail
	aux := &struct {
		StartDate           string  `json:"startDate"`
		EndDate             string  `json:"endDate"`
		RegistrationEndDate string  `json:"registrationEndDate"`
		CreationDate        string  `json:"creationDate"`
		LastChangeDate      string  `json:"lastChangeDate"`
		Entries             []Entry `json:"entries"`
		*Alias
	}{
		Alias: (*Alias)(ed),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("EventDetail unmarshal: %w", err)
	}
	var err error
	ed.StartDate, err = internal.ParseDateOrZero(aux.StartDate)
	if err != nil {
		return fmt.Errorf("parsing EventDetail.StartDate: %w", err)
	}
	ed.EndDate, err = internal.ParseDateOrZero(aux.EndDate)
	if err != nil {
		return fmt.Errorf("parsing EventDetail.EndDate: %w", err)
	}
	ed.RegistrationEndDate, err = internal.ParseDateOrZero(aux.RegistrationEndDate)
	if err != nil {
		return fmt.Errorf("parsing EventDetail.RegistrationEndDate: %w", err)
	}
	ed.CreationDate, err = internal.ParseDateOrZero(aux.CreationDate)
	if err != nil {
		return fmt.Errorf("parsing EventDetail.CreationDate: %w", err)
	}
	ed.LastChangeDate, err = internal.ParseDateOrZero(aux.LastChangeDate)
	if err != nil {
		return fmt.Errorf("parsing EventDetail.LastChangeDate: %w", err)
	}
	// copy parsed entries
	ed.Entries = aux.Entries
	return nil
}

// Custom unmarshaller for Entry to handle flexible date parsing.
func (e *Entry) UnmarshalJSON(data []byte) error {
	type Alias Entry
	aux := &struct {
		RegistrationDate string `json:"registrationDate"`
		*Alias
	}{
		Alias: (*Alias)(e),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("Entry unmarshal: %w", err)
	}
	var err error
	e.RegistrationDate, err = internal.ParseDateOrZero(aux.RegistrationDate)
	if err != nil {
		return fmt.Errorf("parsing Entry.RegistrationDate: %w", err)
	}
	return nil
}
