/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package uschess

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

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

// FetchCrossTables retrieves a Tournament with all sections' cross tables for the given event id.
func (client *Client) FetchCrossTables(ctx context.Context,
	id EventID) (*Tournament, error) {

	url := fmt.Sprintf("https://www.uschess.org/msa/XtblMain.php?%v.0", id)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch uscf crosstable (new): %w", err)
	}
	req.Header.Set("User-Agent", internal.UserAgent)

	// these are rarely (if ever) updated so 1 month cache is fine for our use
	// case
	resp, err := client.httpClient30day.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch uscf crosstable (do): %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Extract section names from section headers
	var sectionNames []string
	doc.Find("td[bgcolor=DDDDFF] b").Each(func(_ int, s *goquery.Selection) {
		txt := strings.TrimSpace(s.Text())
		if strings.HasPrefix(txt, "Section ") {
			sectionNames = append(sectionNames, txt)
		}
	})

	// Extract all <pre> blocks (one per section)
	pres := doc.Find("pre")
	if pres.Length() != len(sectionNames) {
		// mismatch is unexpected but proceed with min length
		log.Printf("warning: found %d sections but %d <pre> blocks",
			len(sectionNames), pres.Length())
	}

	var cts []*CrossTable
	count := pres.Length()
	if count > len(sectionNames) {
		count = len(sectionNames)
	}

	for i := 0; i < count; i++ {
		ct := parseOneCrossTable(pres.Eq(i), sectionNames[i])
		cts = append(cts, ct)
	}

	// Build Tournament object
	rawTitle := strings.TrimSpace(doc.Find("title").First().Text())
	// Extract event name between "Cross Table for " and "(Event"
	eventName := rawTitle
	prefix := "Cross Table for "
	if idx := strings.Index(rawTitle, prefix); idx != -1 {
		if idxEvent := strings.LastIndex(rawTitle, "(Event"); idxEvent != -1 && idxEvent > idx {
			eventName = strings.TrimSpace(rawTitle[idx+len(prefix) : idxEvent])
		}
	}

	// Extract event end date from summary
	var endDate time.Time
	doc.Find("td").Each(func(_ int, s *goquery.Selection) {
		if strings.TrimSpace(s.Text()) == "Event Date(s)" {
			// skip any empty cell, find next non-empty text cell
			nxt := s.Next()
			for nxt.Length() > 0 && strings.TrimSpace(nxt.Text()) == "" {
				nxt = nxt.Next()
			}
			raw := strings.TrimSpace(nxt.Text())
			parts := strings.Split(raw, "thru")
			if len(parts) == 2 {
				dt, err := internal.ParseDateOrZero(strings.TrimSpace(parts[1]))
				if err == nil {
					endDate = dt
				}
			} else {
				dt, err := internal.ParseDateOrZero(strings.TrimSpace(parts[0]))
				if err == nil {
					endDate = dt
				}
			}
		}
	})

	t := &Tournament{
		Event: Event{
			EndDate: endDate,
			Name:    eventName,
			ID:      id,
		},
		NumSections: len(cts),
		CrossTables: cts,
	}

	return t, nil
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

	// Locate header lines
	head1Idx, head2Idx := -1, -1
	for ln, l := range lines {
		if head1Idx < 0 && strings.Contains(l, "Pair") && strings.Contains(l, "|") {
			head1Idx = ln
			continue
		}
		if head1Idx >= 0 && head2Idx < 0 && strings.Contains(l, "USCF ID") && strings.Contains(l, "|") {
			head2Idx = ln
			break
		}
	}
	if head1Idx < 0 || head2Idx < 0 {
		return nil
	}

	// Count rounds and determine data start
	numRounds := strings.Count(lines[head1Idx], "Round")
	start := head2Idx + 2

	entries := parseCrossTableEntries(start, lines, numRounds)

	return &CrossTable{
		SectionName:   sectionName,
		NumRounds:     numRounds,
		NumPlayers:    len(entries),
		RType:         rType,
		PlayerEntries: entries,
	}
}

// parseCrossTableEntries splits each pair of lines for one player by '|' to extract fields.
func parseCrossTableEntries(
	start int,
	lines []string,
	numRounds int,
) []CrossTableEntry {
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

		// Split fields by pipe
		parts1 := strings.Split(l1, "|")
		parts2 := strings.Split(l2, "|")
		if len(parts1) < 3+numRounds || len(parts2) < 3+numRounds {
			continue
		}

		// Parse basic fields
		pairNum, err := strconv.Atoi(strings.TrimSpace(parts1[0]))
		if err != nil {
			continue
		}
		name := strings.TrimSpace(parts1[1])
		totalPts, err := strconv.ParseFloat(strings.TrimSpace(parts1[2]), 64)
		if err != nil {
			totalPts = 0
		}

		// Parse ID and ratings
		m := idRe.FindStringSubmatch(strings.TrimSpace(parts2[1]))
		if len(m) != 4 {
			continue
		}
		playerID, _ := strconv.Atoi(m[1])
		preRating := strings.TrimSpace(m[2])
		postRating := strings.TrimSpace(m[3])

		// Parse round results
		var results []RoundResult
		for r := 0; r < numRounds; r++ {
			cellRes := strings.TrimSpace(parts1[3+r])
			cellCol := strings.TrimSpace(parts2[3+r])
			var outcome Result
			var opNum int
			if cellRes != "" && strings.ContainsAny(cellRes, "WLDUXFHB") {
				op := digitsRe.FindString(cellRes)
				opNum, _ = strconv.Atoi(op)
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
			} else {
				outcome = ResultUnknown
			}
			col := ""
			if strings.ToUpper(cellCol) == "W" {
				col = "white"
			} else if strings.ToUpper(cellCol) == "B" {
				col = "black"
			}
			results = append(results, RoundResult{
				OpponentPairNum: opNum,
				Outcome:         outcome,
				Color:           col,
			})
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
				cell = fmt.Sprintf("W%d(%c)", res.OpponentPairNum, res.Color[0])
			case ResultWinByForfeit:
				forfeitFound = true
				cell = "W*"
			case ResultLoss:
				cell = fmt.Sprintf("L%d(%c)", res.OpponentPairNum, res.Color[0])
			case ResultLossByForfeit:
				forfeitFound = true
				cell = "L*"
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
