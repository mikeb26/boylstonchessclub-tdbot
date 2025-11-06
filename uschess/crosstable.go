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
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
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
	Ordinal       int                `json:"ordinal"`
	PairingNumber int                `json:"pairingNumber"`
	MemberID      string             `json:"memberId"`
	FirstName     string             `json:"firstName"`
	LastName      string             `json:"lastName"`
	Score         float64            `json:"score"`
	RoundOutcomes []apiRoundOutcome  `json:"roundOutcomes"`
	Ratings       []apiRatingChange  `json:"ratings"`
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

	// Fetch event metadata
	eventURL := fmt.Sprintf("https://ratings-api.uschess.org/api/v1/rated-events/%v", id)
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

	// Parse event end date
	endDate, err := internal.ParseDateOrZero(eventData.EndDate)
	if err != nil {
		log.Printf("warning: unable to parse event end date %v: %v", eventData.EndDate, err)
	}

	// Fetch standings for each section
	var crossTables []*CrossTable
	for _, section := range eventData.Sections {
		ct, err := client.fetchSectionStandings(ctx, id, section.Number, section.Name)
		if err != nil {
			log.Printf("warning: failed to fetch section %d: %v", section.Number, err)
			continue
		}
		crossTables = append(crossTables, ct)
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

func (client *Client) fetchSectionStandings(ctx context.Context,
	eventID EventID, sectionNum int, sectionName string) (*CrossTable, error) {

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

	return convertStandingsToCrossTable(standingsData, sectionName), nil
}

func convertStandingsToCrossTable(standings apiStandingsResponse, sectionName string) *CrossTable {
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
		return ResultLossByForfeit
	case "WinByForfeit":
		return ResultWinByForfeit
	case "Unplayed":
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

func parseOneCrossTable(sel *goquery.Selection, sectionName string) *CrossTable {
	// Clean links and italics
	sel.Find("a, i").Each(func(_ int, s *goquery.Selection) {
		s.ReplaceWithHtml(s.Text())
	})

	// Determine rating type for this section
	rType := RatingTypeRegular
	// locate section header table
	parentTr := sel.Parent().Parent()
	headerTr := parentTr.Prev()
	headerTbl := headerTr.Find("table").First()
	headerTbl.Find("td").Each(func(_ int, s *goquery.Selection) {
		if strings.TrimSpace(s.Text()) == "Stats" {
			// find next non-empty cell
			nxt := s.Next()
			for nxt.Length() > 0 && strings.TrimSpace(nxt.Text()) == "" {
				nxt = nxt.Next()
			}
			txt := nxt.Text()
			if idx := strings.Index(txt, "Rating Sys:"); idx != -1 {
				val := strings.TrimSpace(txt[idx+len("Rating Sys:"):])
				if len(val) > 0 {
					switch val[0] {
					case 'R', 'D':
						rType = RatingTypeRegular
					case 'B':
						rType = RatingTypeBlitz
					case 'Q':
						rType = RatingTypeQuick
					}
				}
			}
		}
	})

	text := sel.Text()
	lines := strings.Split(text, "\n")

	// Locate header line and record '|' positions
	headerIdx := -1
	var separators []int
	for ln, l := range lines {
		if strings.Contains(l, "Pair") && strings.Contains(l, "|") {
			headerIdx = ln
			for idx, r := range l {
				if r == '|' {
					separators = append(separators, idx)
				}
			}
			break
		}
	}
	if headerIdx < 0 || len(separators) < 4 {
		return nil
	}

	// Build column boundaries
	boundaries := []int{-1}
	boundaries = append(boundaries, separators...)
	endPos := len(lines[headerIdx])
	boundaries = append(boundaries, endPos)
	numCols := len(boundaries) - 1

	// Count rounds
	numRounds := strings.Count(lines[headerIdx], "Round")
	start := headerIdx + 2

	entries := parseCrossTableEntries(start, numCols, lines, boundaries,
		numRounds)

	return &CrossTable{
		SectionName:   sectionName,
		NumRounds:     numRounds,
		NumPlayers:    len(entries),
		RType:         rType,
		PlayerEntries: entries,
	}
}

func parseCrossTableEntries(start, numCols int,
	lines []string, boundaries []int, numRounds int) []CrossTableEntry {

	// Prepare regexes
	digitsRe := regexp.MustCompile(`\d+`)
	dataLineRe := regexp.MustCompile(`^\s*\d+\s*\|`)
	// Match playerID and pre->post ratings, capturing raw rating strings
	idRe := regexp.MustCompile(`^(\d+)\s*/\s*[^:]+:\s*(.*?)\s*->\s*(.*?)$`)

	var entries []CrossTableEntry
	for j := start; j+1 < len(lines); j++ {
		l1 := lines[j]
		if strings.TrimSpace(l1) == "" ||
			strings.HasPrefix(strings.TrimSpace(l1), "Note:") {
			break
		}
		if !dataLineRe.MatchString(l1) {
			continue
		}
		l2 := lines[j+1]

		// Split fields by column boundaries
		c1 := make([]string, numCols)
		c2 := make([]string, numCols)
		for k := 0; k < numCols; k++ {
			sp := boundaries[k] + 1
			ep := boundaries[k+1]
			if sp < 0 {
				sp = 0
			}
			if ep > len(l1) {
				ep = len(l1)
			}
			c1[k] = strings.TrimSpace(l1[sp:ep])
			if ep > len(l2) {
				ep = len(l2)
			}
			c2[k] = strings.TrimSpace(l2[sp:ep])
		}

		// Extract player ID and ratings
		m := idRe.FindStringSubmatch(c2[1])
		if len(m) != 4 {
			continue
		}
		// Player ID
		playerID, _ := strconv.Atoi(m[1])
		// Keep full rating strings including any provisional suffixes
		preRating := strings.TrimSpace(m[2])
		postRating := strings.TrimSpace(m[3])
		totalPts, _ := strconv.ParseFloat(c1[2], 64)
		pairNum, _ := strconv.Atoi(c1[0])
		name := c1[1]

		// Parse round results
		var results []RoundResult
		for r := 0; r < numRounds; r++ {
			cellRes := strings.TrimSpace(c1[3+r])
			cellCol := strings.TrimSpace(c2[3+r])
			if cellRes == "" || !strings.ContainsAny(cellRes, "WLDUXFHB") {
				results = append(results, RoundResult{Outcome: ResultUnknown})
				continue
			}
			op := digitsRe.FindString(cellRes)
			opNum, _ := strconv.Atoi(op)
			var outcome Result
			switch cellRes[0] {
			case 'W':
				outcome = ResultWin
			case 'L':
				outcome = ResultLoss
			case 'D':
				outcome = ResultDraw
			case 'U':
				outcome = ResultUnplayedGame
			case 'X':
				outcome = ResultWinByForfeit
			case 'F':
				outcome = ResultLossByForfeit
			case 'H':
				outcome = ResultHalfBye
			case 'B':
				outcome = ResultFullBye
			default:
				outcome = ResultUnknown
			}
			col := ""
			if strings.ToUpper(cellCol) == "W" {
				col = "white"
			} else if strings.ToUpper(cellCol) == "B" {
				col = "black"
			}
			results = append(results, RoundResult{OpponentPairNum: opNum,
				Outcome: outcome, Color: col})
		}

		entries = append(entries, CrossTableEntry{
			PairNum:          pairNum,
			PlayerName:       internal.NormalizeName(name),
			PlayerId:         MemID(playerID),
			PlayerRatingPre:  preRating,
			PlayerRatingPost: postRating,
			TotalPoints:      totalPts,
			Results:          results,
		})
	}

	return entries
}

func BuildOneCrossTableOutput(xt *CrossTable,
	includeSectionHeader bool, filterPlayerID MemID) string {

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

	return sb.String()
}
