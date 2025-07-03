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
	"github.com/mikeb26/boylstonchessclub-tdbot/internal/httpcache"
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
	PlayerId         int
	PlayerRatingPre  string
	PlayerRatingPost string
	TotalPoints      float64
	Results          []RoundResult
}

// CrossTable holds the full cross table data, one per section.
type CrossTable struct {
	SectionName   string
	NumRounds     int
	NumPlayers    int
	PlayerEntries []CrossTableEntry
}

// FetchCrossTable retrieves all sections' cross tables from the given id.
func FetchCrossTables(ctx context.Context, id int) ([]*CrossTable, error) {
	url := fmt.Sprintf("https://www.uschess.org/msa/XtblMain.php?%v.0", id)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch uscf crosstable (new): %w", err)
	}
	req.Header.Set("User-Agent", internal.UserAgent)

	// these are rarely (if ever) updated so 1 month cache is fine for our use
	// case
	client := httpcache.NewCachedHttpClient(ctx, 30*24*time.Hour)
	resp, err := client.Do(req)
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

	return cts, nil
}

func parseOneCrossTable(sel *goquery.Selection, sectionName string) *CrossTable {
	// Clean links and italics
	sel.Find("a, i").Each(func(_ int, s *goquery.Selection) {
		s.ReplaceWithHtml(s.Text())
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
			PlayerName:       normalizeName(name),
			PlayerId:         playerID,
			PlayerRatingPre:  preRating,
			PlayerRatingPost: postRating,
			TotalPoints:      totalPts,
			Results:          results,
		})
	}

	return entries
}

func BuildOneCrossTableOutput(xt *CrossTable,
	includeSectionHeader bool, filterPlayerID int) string {

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
				playerName = fmt.Sprintf("++%v++", playerName)
			}
		}

		row := []string{
			fmt.Sprintf("%d.", e.PairNum),
			playerName,
			fmt.Sprintf("%v->%v", e.PlayerRatingPre, e.PlayerRatingPost),
			fmt.Sprintf("%.1f", e.TotalPoints),
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
				cell = "BYE(1.0)"
			case ResultHalfBye:
				cell = "BYE(0.5)"
			case ResultUnplayedGame:
				cell = "BYE(0.0)"
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
