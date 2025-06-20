/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/araddon/dateparse"
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

// vended by https://beta.boylstonchess.org/api/event/<eventId>/tournament
// Tournament represents the players and current pairings for a specific event.
type Tournament struct {
	Players         []Player  `json:"players"`
	CurrentPairings []Pairing `json:"currentPairings"`

	isPredicted bool
}

// Player represents a participant in the tournament.
type Player struct {
	FirstName            string  `json:"firstName"`
	MiddleName           string  `json:"middleName"`
	LastName             string  `json:"lastName"`
	NameTitle            string  `json:"nameTitle"`
	NameSuffix           string  `json:"nameSuffix"`
	ChessTitle           string  `json:"chessTitle"`
	DisplayName          string  `json:"displayName"`
	UscfID               int     `json:"uscfId"`
	FideID               int     `json:"fideId"`
	FideCountry          string  `json:"fideCountry"`
	PrimaryRating        int     `json:"primaryRating"`
	SecondaryRating      int     `json:"secondaryRating"`
	LiveRating           int     `json:"liveRating"`
	LiveRatingProvo      int     `json:"liveRatingProvo"`
	PostEventRating      int     `json:"postEventRating"`
	PostEventRatingProvo int     `json:"postEventRatingProvo"`
	PostEventBonusPoints int     `json:"postEventBonusPoints"`
	RatingChange         int     `json:"ratingChange"`
	PairingNumber        int     `json:"pairingNumber"`
	CurrentScore         float64 `json:"currentScore"`
	CurrentScoreAG       float64 `json:"currentScoreAfterGame"`
	GamesCompleted       int     `json:"gamesCompleted"`
	Place                string  `json:"place"`
	PlaceNumber          int     `json:"placeNumber"`
}

// Pairing represents a single board pairing in the tournament.
type Pairing struct {
	WhitePlayer  Player   `json:"whitePlayer"`
	BlackPlayer  Player   `json:"blackPlayer"`
	Section      string   `json:"section"`
	RoundNumber  int      `json:"roundNumber"`
	BoardNumber  int      `json:"boardNumber"`
	IsByePairing bool     `json:"isByePairing"`
	WhitePoints  *float64 `json:"whitePoints"`
	BlackPoints  *float64 `json:"blackPoints"`
	ResultCode   string   `json:"resultCode"`
	WhiteResult  *string  `json:"whiteResult"`
	BlackResult  *string  `json:"blackResult"`
	GameLink     string   `json:"gameLink"`
}

// getBccEvents fetches events from the Boylston Chess API and returns a slice
// of Event.
func getBccEvents() ([]Event, error) {
	const url = "https://beta.boylstonchess.org/api/events"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch bcc events (new): %w", err)
	}
	req.Header.Set("User-Agent", UserAgent)

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

// getBccEventDetail fetches detailed event info from the Boylston Chess API
// for a given eventId and returns an EventDetail.
func getBccEventDetail(eventId int64) (EventDetail, error) {
	url := fmt.Sprintf("https://beta.boylstonchess.org/api/event/%d", eventId)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return EventDetail{}, fmt.Errorf("unable to fetch bcc event detail (new): %w", err)
	}
	req.Header.Set("User-Agent", UserAgent)

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

// getBccTournament fetches the tournament data (players and pairings) for a
// given eventId.
func getBccTournament(eventId int64) (*Tournament, error) {
	url := fmt.Sprintf("https://beta.boylstonchess.org/api/event/%d/tournament",
		eventId)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return &Tournament{}, fmt.Errorf("unable to fetch bcc tournament (new): %w",
			err)
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return &Tournament{}, fmt.Errorf("unable to fetch bcc tournament (do): %w",
			err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		detail, err := getBccEventDetail(eventId)
		if err == nil {
			return eventDetailToTournament(detail)
		} else {
			err = fmt.Errorf("unable to fetch %v: http status: %v", url,
				resp.StatusCode)
		}

		return &Tournament{}, err
	}

	tourney := &Tournament{}
	if err := json.NewDecoder(resp.Body).Decode(&tourney); err != nil {
		return &Tournament{}, fmt.Errorf("unable to parse bcc tournament: %w",
			err)
	}
	return tourney, nil
}

func eventDetailToTournament(eventDetail EventDetail) (*Tournament, error) {
	// Build tournament players list from event details entries
	tourney := &Tournament{}
	for _, entry := range eventDetail.Entries {
		tourney.Players = append(tourney.Players, entryToPlayer(entry))
	}

	tourney.CurrentPairings = predictRound1Pairings(eventDetail.Entries)
	tourney.isPredicted = true

	return tourney, nil
}

func (t Tournament) IsPredicted() bool {
	return t.isPredicted
}

func strRatingToInt(rating string) int {
	r := 0
	if rating != "" {
		// handle formats like "559/24"
		if idx := strings.Index(rating, "/"); idx != -1 {
			rating = rating[:idx]
		}
		if v, err := strconv.Atoi(strings.TrimSpace(rating)); err == nil {
			r = v
		}
	}

	return r
}

func entryToPlayer(entry Entry) Player {
	displayName := fmt.Sprintf("%s %s", entry.FirstName, entry.LastName)

	return Player{
		FirstName:       entry.FirstName,
		LastName:        entry.LastName,
		NameTitle:       entry.ChessTitle,
		DisplayName:     displayName,
		UscfID:          entry.UscfID,
		PrimaryRating:   strRatingToInt(entry.PrimaryRating),
		SecondaryRating: strRatingToInt(entry.SecondaryRating),
	}
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
	ed.StartDate, err = parseDateOrZero(aux.StartDate)
	if err != nil {
		return fmt.Errorf("parsing EventDetail.StartDate: %w", err)
	}
	ed.EndDate, err = parseDateOrZero(aux.EndDate)
	if err != nil {
		return fmt.Errorf("parsing EventDetail.EndDate: %w", err)
	}
	ed.RegistrationEndDate, err = parseDateOrZero(aux.RegistrationEndDate)
	if err != nil {
		return fmt.Errorf("parsing EventDetail.RegistrationEndDate: %w", err)
	}
	ed.CreationDate, err = parseDateOrZero(aux.CreationDate)
	if err != nil {
		return fmt.Errorf("parsing EventDetail.CreationDate: %w", err)
	}
	ed.LastChangeDate, err = parseDateOrZero(aux.LastChangeDate)
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
	e.RegistrationDate, err = parseDateOrZero(aux.RegistrationDate)
	if err != nil {
		return fmt.Errorf("parsing Entry.RegistrationDate: %w", err)
	}
	return nil
}

// parseDateOrZero returns a parsed time or zero if input is empty or "null".
func parseDateOrZero(s string) (time.Time, error) {
	if s == "" || s == "null" {
		return time.Time{}, nil
	}
	return dateparse.ParseAny(s)
}
