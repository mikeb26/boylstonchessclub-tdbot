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
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/mikeb26/boylstonchessclub-tdbot/internal"
)

// Result represents the outcome of a round.
type Result int

const (
	ResultWin Result = iota
	ResultLoss
	ResultDraw
	ResultFullBye
	ResultHalfBye
	ResultLossByForfeit
	ResultWinByForfeit
	ResultUnplayedGame
	ResultUnknown
)

// RoundResult holds the result of a single round for a player.
type RoundResult struct {
	OpponentPairNum int
	Outcome         Result
	Color           string
}

// CrossTableEntry holds the data for one player in the cross table.
type CrossTableEntry struct {
	PairNum          int
	PlayerName       string
	PlayerId         MemID
	PlayerRatingPre  string
	PlayerRatingPost string
	TotalPoints      float64
	Results          []RoundResult
}

type RatingType int

const (
	RatingTypeRegular RatingType = iota
	RatingTypeQuick
	RatingTypeBlitz
)

// CrossTable holds the full cross table data, one per section.
type CrossTable struct {
	SectionName   string
	NumRounds     int
	NumPlayers    int
	RType         RatingType
	PlayerEntries []CrossTableEntry
}

// Tournament encapsulates the overall event and its cross tables.
type Tournament struct {
	Event       Event
	NumSections int

	CrossTables []*CrossTable
}

// API response structures for rated events JSON API
type apiRatedEventResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	StartDate    string `json:"startDate"`
	EndDate      string `json:"endDate"`
	SectionCount int    `json:"sectionCount"`
	Sections     []struct {
		ID     string `json:"id"`
		Number int    `json:"number"`
		Name   string `json:"name"`
	} `json:"sections"`
}

type apiStandingsResponse struct {
	Items []apiStandingItem `json:"items"`
}

type apiStandingItem struct {
	Ordinal       int               `json:"ordinal"`
	PairingNumber int               `json:"pairingNumber"`
	MemberID      string            `json:"memberId"`
	FirstName     string            `json:"firstName"`
	LastName      string            `json:"lastName"`
	Score         float64           `json:"score"`
	RoundOutcomes []apiRoundOutcome `json:"roundOutcomes"`
	Ratings       []apiRatingChange `json:"ratings"`
}

type apiRoundOutcome struct {
	RoundNumber           int    `json:"roundNumber"`
	Outcome               string `json:"outcome"`
	Color                 string `json:"color"`
	OpponentOrdinal       int    `json:"opponentOrdinal"`
	OpponentPairingNumber int    `json:"opponentPairingNumber"`
}

type apiRatingChange struct {
	PreRating    int    `json:"preRating"`
	PostRating   int    `json:"postRating"`
	RatingSystem string `json:"ratingSystem"`
}

// FetchCrossTables retrieves a Tournament with all sections' cross tables for the given event id.
func (client *Client) FetchCrossTables(ctx context.Context,
	id EventID) (*Tournament, error) {

	eventData, err := client.fetchRatedEvent(ctx, id)
	if err != nil {
		return nil, err
	}

	// Fetch standings for each section
	standingsData := make(map[string]*apiStandingsResponse)
	for _, section := range eventData.Sections {
		oneStandingsData, err := client.fetchSectionStandings(ctx, id,
			section.Number, section.Name)
		if err != nil {
			log.Printf("warning: failed to fetch section %d: %v",
				section.Number, err)
			continue
		}
		standingsData[section.Name] = oneStandingsData
	}

	crossTables := convertStandingsToCrossTables(standingsData)

	endDate, err := internal.ParseDateOrZero(eventData.EndDate)
	if err != nil {
		log.Printf("warning: unable to parse event end date %v: %v",
			eventData.EndDate, err)
	}

	return &Tournament{
		Event: Event{
			EndDate: endDate,
			Name:    eventData.Name,
			ID:      id,
		},
		NumSections: len(crossTables),
		CrossTables: crossTables,
	}, nil
}

func (client *Client) fetchRatedEvent(ctx context.Context,
	id EventID) (*apiRatedEventResponse, error) {

	eventURL :=
		fmt.Sprintf("https://ratings-api.uschess.org/api/v1/rated-events/%v",
			id)
	req, err := http.NewRequest("GET", eventURL, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create event request: %w", err)
	}
	req.Header.Set("User-Agent", internal.UserAgent)
	req.Header.Set("Accept", "application/json")

	// these are rarely (if ever) updated so 1 month cache is fine for our use case
	resp, err := client.httpClient30day.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected event status %d: %s",
			resp.StatusCode, string(body))
	}

	var eventData apiRatedEventResponse
	if err := json.NewDecoder(resp.Body).Decode(&eventData); err != nil {
		return nil, fmt.Errorf("failed to parse event JSON: %w", err)
	}

	return &eventData, nil
}

func (client *Client) fetchSectionStandings(ctx context.Context,
	eventID EventID, sectionNum int,
	sectionName string) (*apiStandingsResponse, error) {

	url := fmt.Sprintf("https://ratings-api.uschess.org/api/v1/rated-events/%v/sections/%d/standings",
		eventID, sectionNum)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create standings request: %w", err)
	}
	req.Header.Set("User-Agent", internal.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := client.httpClient30day.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch standings: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected standings status %d: %s",
			resp.StatusCode, string(body))
	}

	var standingsData apiStandingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&standingsData); err != nil {
		return nil, fmt.Errorf("failed to parse standings JSON: %w", err)
	}

	return &standingsData, nil
}

func convertStandingsToCrossTables(standings map[string]*apiStandingsResponse) []*CrossTable {
	xts := make([]*CrossTable, 0)

	for secName, _ := range standings {
		xt := convertStandingsToCrossTable(standings[secName], secName)
		xts = append(xts, xt)
	}

	return xts
}

func convertStandingsToCrossTable(standings *apiStandingsResponse, sectionName string) *CrossTable {
	var entries []CrossTableEntry
	var numRounds int
	var ratingType RatingType = RatingTypeRegular

	for _, item := range standings.Items {
		// Determine rating type from first player's rating system
		// For dual-rated sections, prefer Regular rating
		if len(entries) == 0 && len(item.Ratings) > 0 {
			foundRating := false
			// First look for Regular rating in dual-rated sections
			for _, rating := range item.Ratings {
				if rating.RatingSystem == "R" || rating.RatingSystem == "D" {
					ratingType = RatingTypeRegular
					foundRating = true
					break
				}
			}
			// If no Regular rating found, use the first rating system
			if !foundRating {
				switch item.Ratings[0].RatingSystem {
				case "B":
					ratingType = RatingTypeBlitz
				case "Q":
					ratingType = RatingTypeQuick
				default:
					ratingType = RatingTypeRegular
				}
			}
		}

		// Convert round outcomes to RoundResults
		var results []RoundResult
		for _, outcome := range item.RoundOutcomes {
			result := RoundResult{
				OpponentPairNum: outcome.OpponentOrdinal,
				Outcome:         convertOutcome(outcome.Outcome),
				Color:           convertColor(outcome.Color),
			}
			results = append(results, result)
		}

		if len(results) > numRounds {
			numRounds = len(results)
		}

		// Get pre and post ratings - prefer the rating that matches our ratingType
		var preRating, postRating string
		for _, rating := range item.Ratings {
			shouldUse := false
			switch ratingType {
			case RatingTypeRegular:
				shouldUse = (rating.RatingSystem == "R" || rating.RatingSystem == "D")
			case RatingTypeBlitz:
				shouldUse = (rating.RatingSystem == "B")
			case RatingTypeQuick:
				shouldUse = (rating.RatingSystem == "Q")
			}
			if shouldUse {
				if rating.PreRating > 0 {
					preRating = strconv.Itoa(rating.PreRating)
				}
				if rating.PostRating > 0 {
					postRating = strconv.Itoa(rating.PostRating)
				}
				break
			}
		}

		// Convert member ID to int
		memberID, err := strconv.Atoi(item.MemberID)
		if err != nil {
			log.Printf("warning: failed to convert member ID %v to int: %v", item.MemberID, err)
		}

		entry := CrossTableEntry{
			PairNum:          item.Ordinal,
			PlayerName:       internal.NormalizeName(item.FirstName + " " + item.LastName),
			PlayerId:         MemID(memberID),
			PlayerRatingPre:  preRating,
			PlayerRatingPost: postRating,
			TotalPoints:      item.Score,
			Results:          results,
		}
		entries = append(entries, entry)
	}

	return &CrossTable{
		SectionName:   fmt.Sprintf("Section %s", sectionName),
		NumRounds:     numRounds,
		NumPlayers:    len(entries),
		RType:         ratingType,
		PlayerEntries: entries,
	}
}

func convertOutcome(outcome string) Result {
	switch outcome {
	case "Win":
		return ResultWin
	case "Loss":
		return ResultLoss
	case "Draw":
		return ResultDraw
	case "ByeFull":
		return ResultFullBye
	case "ByeHalf":
		return ResultHalfBye
	case "LossByForfeit":
		fallthrough
	case "LossForfeit":
		return ResultLossByForfeit
	case "WinForfeit":
		fallthrough
	case "WinByForfeit":
		return ResultWinByForfeit
	case "Unplayed":
		fallthrough
	case "Unpaired":
		return ResultUnplayedGame
	default:
		return ResultUnknown
	}
}

func convertColor(color string) string {
	switch strings.ToLower(color) {
	case "white":
		return "white"
	case "black":
		return "black"
	default:
		return ""
	}
}

func BuildOneCrossTableOutput(xt *CrossTable,
	includeSectionHeader bool, filterPlayerID MemID) (string, string) {

	// If filtering, determine which pair numbers to include (player + opponents)
	var includeSet map[int]bool
	var filteredPlayerPairNum int
	if filterPlayerID != 0 {
		includeSet = make(map[int]bool)
		// find player entry
		for _, e := range xt.PlayerEntries {
			if e.PlayerId == filterPlayerID {
				filteredPlayerPairNum = e.PairNum
				includeSet[e.PairNum] = true
				// record opponents
				for _, res := range e.Results {
					if res.OpponentPairNum > 0 {
						includeSet[res.OpponentPairNum] = true
					}
				}
				break
			}
		}
	}

	var sb strings.Builder

	if includeSectionHeader {
		sb.WriteString(fmt.Sprintf("%v\n", xt.SectionName))
	}

	// Build headers
	numRounds := xt.NumRounds
	headers := []string{"No", "Name", "Rating", "Pts"}
	for i := 1; i <= numRounds; i++ {
		headers = append(headers, fmt.Sprintf("R%d", i))
	}

	// Build rows
	ratingPost := "<unknown>"
	forfeitFound := false
	var rows [][]string
	for _, e := range xt.PlayerEntries {
		playerName := e.PlayerName

		// apply filter
		if includeSet != nil {
			if !includeSet[e.PairNum] {
				continue
			}
			if filteredPlayerPairNum == e.PairNum {
				playerName = fmt.Sprintf("**%v**", playerName)
				ratingPost = e.PlayerRatingPost
			}
		}

		row := []string{
			fmt.Sprintf("%d.", e.PairNum),
			playerName,
			fmt.Sprintf("%v->%v", e.PlayerRatingPre, e.PlayerRatingPost),
			fmt.Sprintf("%v", internal.ScoreToString(e.TotalPoints)),
		}
		for _, res := range e.Results {
			var cell string
			switch res.Outcome {
			case ResultWin:
				cell = fmt.Sprintf("W%d", res.OpponentPairNum)
				cell += fmt.Sprintf("(%c)", res.Color[0])
			case ResultWinByForfeit:
				forfeitFound = true
				cell = fmt.Sprintf("W*")
			case ResultLoss:
				cell = fmt.Sprintf("L%d", res.OpponentPairNum)
				cell += fmt.Sprintf("(%c)", res.Color[0])
			case ResultLossByForfeit:
				forfeitFound = true
				cell = fmt.Sprintf("L*")
			case ResultDraw:
				cell = fmt.Sprintf("D%d", res.OpponentPairNum)
				cell += fmt.Sprintf("(%c)", res.Color[0])
			case ResultFullBye:
				cell = "BYE(1)"
			case ResultHalfBye:
				cell = "BYE(½)"
			case ResultUnplayedGame:
				cell = "BYE(0)"
			default:
				cell = "?"
			}
			row = append(row, cell)
		}
		rows = append(rows, row)
	}

	// Compute column widths
	colWidths := make([]int, len(headers))
	for i, h := range headers {
		colWidths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	// Build format string
	var fmtStrBuilder strings.Builder
	for _, w := range colWidths {
		fmtStrBuilder.WriteString(fmt.Sprintf("%%-%ds  ", w))
	}
	fmtStr := strings.TrimRight(fmtStrBuilder.String(), " ") + "\n"

	// Write header
	sb.WriteString(fmt.Sprintf(fmtStr, toAnySlice(headers)...))
	// Write rows
	for _, row := range rows {
		sb.WriteString(fmt.Sprintf(fmtStr, toAnySlice(row)...))
	}
	if forfeitFound {
		sb.WriteString("* indicates game was decided by forfeit\n")
	}
	sb.WriteString("\n")

	return sb.String(), ratingPost
}
