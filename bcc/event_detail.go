/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package bcc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
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

// BuildPairingsOutput formats an EventDetail into a pretty printed string output
func BuildEventOutput(detail *EventDetail, boldTag string, includeTitle,
	includeUrl bool) string {

	var sb strings.Builder

	if includeTitle {
		sb.WriteString(fmt.Sprintf("%vTitle%v: %v\n", boldTag, boldTag,
			detail.Title))
	}
	if includeUrl {
		sb.WriteString(fmt.Sprintf("%vURL%v: https://boylstonchess.org/events/%d\n",
			boldTag, boldTag, detail.EventID))
	}

	sb.WriteString(fmt.Sprintf("%vEventID%v: %d [Register](https://boylstonchess.org/tournament/register/%v)\n",
		boldTag, boldTag, detail.EventID, detail.EventID))
	sb.WriteString(fmt.Sprintf("%vDate%v: %s\n", boldTag, boldTag, detail.DateDisplay))
	if detail.EventFormat != "" {
		sb.WriteString(fmt.Sprintf("%vFormat%v: %s\n", boldTag, boldTag,
			detail.EventFormat))
	}
	if detail.TimeControl != "" {
		sb.WriteString(fmt.Sprintf("%vTime Control%v: %s\n", boldTag, boldTag,
			detail.TimeControl))
	}
	if detail.SectionDisplay != "" {
		sb.WriteString(fmt.Sprintf("%vSections%v: %s\n", boldTag, boldTag,
			detail.SectionDisplay))
	}
	sb.WriteString(fmt.Sprintf("%vEntry Fee%v: %s\n", boldTag, boldTag,
		detail.EntryFeeSummary))
	if detail.PrizeSummary != "" {
		sb.WriteString(fmt.Sprintf("%vPrizes%v: %s\n", boldTag, boldTag,
			detail.PrizeSummary))
	}
	if detail.RegistrationTime != "" {
		sb.WriteString(fmt.Sprintf("%vRegistration Time%v: %s\n", boldTag,
			boldTag, detail.RegistrationTime))
	}
	sb.WriteString(fmt.Sprintf("%vRound Times%v: %s\n", boldTag, boldTag,
		detail.RoundTimes))
	sb.WriteString(fmt.Sprintf("%v[Entries](https://boylstonchess.org/tournament/entries/%v)%v: %v\n",
		boldTag, detail.EventID, boldTag, buildEntriesString(detail)))
	sb.WriteString(fmt.Sprintf("%vDescription%v: %s\n", boldTag, boldTag,
		detail.Description))

	return sb.String()
}

// buildEntriesString formats a pretty printed string describing the entries
func buildEntriesString(detail *EventDetail) string {
	var sb strings.Builder

	t := eventDetailToTournament(detail)
	secPlayers := getPlayersBySection(t)
	sb.WriteString(fmt.Sprintf("%v", len(detail.Entries)))
	if len(secPlayers) > 1 || len(detail.Sections) > 1 {
		// Sort section names using custom criteria
		var sectionNames []string
		for sec := range secPlayers {
			sectionNames = append(sectionNames, sec)
		}
		// Use named sectionSorter instead of anonymous comparator
		sort.Sort(SectionSorter(sectionNames))

		sb.WriteString(" (")
		isFirst := true
		for _, k := range sectionNames {
			if !isFirst {
				sb.WriteString(" ")
			} else {
				isFirst = false
			}
			sb.WriteString(fmt.Sprintf("%v:%v", k, len(secPlayers[k])))
		}
		sb.WriteString(")")

	}

	return sb.String()
}
