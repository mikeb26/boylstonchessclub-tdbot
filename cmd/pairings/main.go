/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
)

func main() {
	url := parseArgs()
	page, err := fetch(url)
	if err != nil {
		log.Fatalf("%v: Failed to retrieve %v: %v", os.Args[0], url, err)
	}
	secPlayers := extractPlayersFromPage(page)
	sections := buildSections(secPlayers)
	outputSectionPairings(sections)
}

// extractPlayersFromPage parses the HTML registration table and returns
// an initial Player lists grouped by section.
func extractPlayersFromPage(s string) map[string][]Player {
	sections := make(map[string][]Player)

	// Find the members table by UscfID
	reTable := regexp.MustCompile(`(?s)<table[^>]*id="members"[^>]*>(.*?)</table>`)
	mTable := reTable.FindStringSubmatch(s)
	if mTable == nil {
		return sections
	}

	tableHTML := mTable[1]

	// Extract header to locate columns
	reHead := regexp.MustCompile(`(?s)<thead.*?>(.*?)</thead>`)
	hHead := reHead.FindStringSubmatch(tableHTML)
	var idIdx, secIdx, nameIdx, rateIdx int
	// byesIdx tracks the index of the "Byes" column; default -1 if not present
	byesIdx := -1
	if hHead != nil {
		headHTML := hHead[1]
		reTh := regexp.MustCompile(`(?i)<th[^>]*>([^<]+)</th>`)
		ths := reTh.FindAllStringSubmatch(headHTML, -1)
		for i, th := range ths {
			col := strings.TrimSpace(th[1])
			switch strings.ToLower(col) {
			case "uscf id":
				idIdx = i
			case "section":
				secIdx = i
			case "name":
				nameIdx = i
			case "rating":
				rateIdx = i
			case "byes":
				byesIdx = i
			}
		}
	}

	// Extract body rows
	reBody := regexp.MustCompile(`(?s)<tbody.*?>(.*?)</tbody>`)
	bBody := reBody.FindStringSubmatch(tableHTML)
	if bBody == nil {
		return sections
	}
	bodyHTML := bBody[1]

	reRow := regexp.MustCompile(`(?s)<tr.*?>(.*?)</tr>`)
	rows := reRow.FindAllStringSubmatch(bodyHTML, -1)
	reTd := regexp.MustCompile(`(?s)<td[^>]*>(.*?)</td>`)
	reUscfID := regexp.MustCompile(`MbrDtlMain\.php\?(\d{6,8})`)
	reTag := regexp.MustCompile(`<[^>]+>`) // strip tags

	for _, rm := range rows {
		row := rm[1]
		cells := reTd.FindAllStringSubmatch(row, -1)
		if len(cells) <= idIdx || len(cells) <= secIdx ||
			len(cells) <= nameIdx || len(cells) <= rateIdx {
			continue
		}

		// USCF UscfID
		rawUscfID := cells[idIdx][1]
		mUscfID := reUscfID.FindStringSubmatch(rawUscfID)
		var id string
		if mUscfID == nil {
			id = ""
		} else {
			id = mUscfID[1]
		}

		// Section name
		rawSec := cells[secIdx][1]
		secName := strings.TrimSpace(reTag.ReplaceAllString(rawSec, ""))

		// Player name from registration
		rawName := cells[nameIdx][1]
		name := strings.TrimSpace(htmlUnescape(reTag.ReplaceAllString(rawName,
			"")))
		if name == "" {
			name = "Unknown"
		}
		name = normalizeName(name)

		// Reported rating
		rawRate := strings.TrimSpace(htmlUnescape(
			reTag.ReplaceAllString(cells[rateIdx][1], "")))
		reported := 0
		if rawRate != "" && !strings.EqualFold(rawRate, "unrated") {
			if r, err := strconv.Atoi(rawRate); err == nil {
				reported = r
			}
		}

		requestedByes := ""
		if byesIdx >= 0 && len(cells) > byesIdx {
			requestedByes = strings.TrimSpace(htmlUnescape(
				reTag.ReplaceAllString(cells[byesIdx][1], "")))
		}
		bReason := ByeReasonNone
		if round1ByeRequested(requestedByes) {
			bReason = ByeReasonRequested
		}

		// Append initial Player
		p := Player{
			UscfID:  id,
			Name:    name,
			Rating:  reported,
			RType:   RatingTypeReported,
			BReason: bReason,
		}
		sections[secName] = append(sections[secName], p)
	}

	return sections
}

func buildSections(secPlayers map[string][]Player) []Section {
	var sections []Section
	for secName, initPlayerList := range secPlayers {
		if len(initPlayerList) < 2 {
			continue
		}
		players := finalizePlayers(initPlayerList)
		if len(players) < 2 {
			continue
		}
		pairings, byes := buildPairings(players)
		sections = append(sections, Section{Name: secName,
			Pairings: pairings, Byes: byes})
	}

	return sections
}

func outputSectionPairings(sections []Section) {
	fmt.Printf("Predicted Pairings:\n")
	boardNum := 1
	for _, sec := range sections {
		if sec.Name != "" {
			fmt.Printf("Section: %s\n", sec.Name)
		}
		for _, p := range sec.Pairings {
			w := p[0]
			b := p[1]
			fmt.Printf("  Board %d: %s(%s) vs. %s(%s)\n", boardNum,
				w.Name, displayRating(w), b.Name, displayRating(b))
			boardNum++
		}
		for _, p := range sec.Byes {
			fmt.Printf("  BYE(%v): %s(%s)\n", byeValFromReason(p.BReason),
				p.Name, displayRating(p))
		}
		fmt.Printf("\n")
	}
}

// finalizePlayers augments initial Players by fetching official name/rating,
// falling back to reported on error.
func finalizePlayers(initPlayerList []Player) []Player {
	var (
		mu      sync.Mutex
		players []Player
	)
	ctx := context.Background()
	g, _ := errgroup.WithContext(ctx)

	for _, initP := range initPlayerList {
		p := initP
		g.Go(func() error {
			profURL := "https://www.uschess.org/msa/MbrDtlMain.php?" + p.UscfID
			body, err := fetch(profURL)
			if err != nil {
				// use registered values

				mu.Lock()
				players = append(players, p)
				mu.Unlock()

				return nil
			}
			if strings.Contains(body, "Could not retrieve data for") {
				// use registered values

				mu.Lock()
				players = append(players, p)
				mu.Unlock()

				return nil
			}
			// override with official data
			offName := extractName(body)
			offName = normalizeName(offName)
			offRating := extractRating(body)
			p.Name = offName
			p.Rating = offRating
			p.RType = RatingTypeActual

			mu.Lock()
			players = append(players, p)
			mu.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		log.Printf("error fetching players: %v", err)
	}

	sort.Slice(players, func(i, j int) bool {
		return players[i].Rating > players[j].Rating
	})

	return players
}

// buildPairings constructs the pairings and determines byes
func buildPairings(players []Player) ([]Pairing, []Player) {
	var byes []Player

	// first remove requested byes
	var filtered []Player
	for _, p := range players {
		if p.BReason == ByeReasonRequested {
			byes = append(byes, p)
		} else {
			filtered = append(filtered, p)
		}
	}
	players = filtered

	// next remove a bye due if there is an odd number of players
	if len(players)%2 == 1 {
		last := players[len(players)-1]
		last.BReason = ByeReasonOdd
		byes = append(byes, last)
		players = players[:len(players)-1]
	}

	// build pairings from the remaining even set of players
	// highest rated player gets white against (n/2)-th highest
	// rated player. 2nd highest rated player gets black against
	// (n/2 + 1)-th highest rated player. & so on.
	remaining := append([]Player(nil), players...)
	var pairings []Pairing
	lastTopColor := Black
	for len(remaining) >= 2 {
		n := len(remaining)
		top := remaining[0]
		opp := remaining[n/2]
		if lastTopColor == Black {
			lastTopColor = White
			pairings = append(pairings, Pairing{top, opp})
		} else {
			lastTopColor = Black
			pairings = append(pairings, Pairing{opp, top})
		}
		remaining = removeIndex(remaining, n/2)
		remaining = removeIndex(remaining, 0)
	}

	return pairings, byes
}
