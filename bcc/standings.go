/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package bcc

import (
	"fmt"
	"sort"
	"strings"
)

// buildStandingsOutput formats standings into grouped, aligned string output
func BuildStandingsOutput(t *Tournament) string {
	if len(t.CurrentPairings) == 0 {
		return "Cannot determine standings without current pairings"
	}
	secPlayers := getPlayersBySection(t)
	// Sort section names using custom criteria
	var sectionNames []string
	for sec := range secPlayers {
		sectionNames = append(sectionNames, sec)
	}
	// Use named sectionSorter instead of anonymous comparator
	sort.Sort(SectionSorter(sectionNames))
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Standings prior to Round %v:\n\n",
		t.CurrentPairings[0].RoundNumber))

	for sec, players := range secPlayers {
		sort.Slice(players, func(i, j int) bool {
			return players[i].PlaceNumber < players[j].PlaceNumber
		})

		type row struct{ rank, player, score string }
		var rows []row
		priorScore := -1.0
		for idx, p := range players {
			var rank string
			if idx != 0 && p.CurrentScoreAG == priorScore {
				rank = ""
			} else {
				rank = fmt.Sprintf("%v.", p.PlaceNumber)
				priorScore = p.CurrentScoreAG
			}
			r := row{
				rank:   rank,
				player: p.DisplayName,
				score:  fmt.Sprintf("%.1f", p.CurrentScoreAG),
			}
			rows = append(rows, r)
		}

		// Compute column widths
		maxP, maxN, maxS := len("Place"), len("Name"), len("Score")
		for _, r := range rows {
			if l := len(r.rank); l > maxP {
				maxP = l
			}
			if l := len(r.player); l > maxN {
				maxN = l
			}
			if l := len(r.score); l > maxS {
				maxS = l
			}
		}

		// Write section header and table
		if len(sectionNames) > 1 {
			if sec == "" {
				sec = "UNNAMED"
			}
			sb.WriteString(fmt.Sprintf("%s Section\n", sec))
		}
		sb.WriteString(fmt.Sprintf("%-*s  %-*s  %-*s\n", maxP, "Place", maxN,
			"Name", maxS, "Score"))
		for _, r := range rows {
			sb.WriteString(fmt.Sprintf("%-*s  %-*s  %-*s\n", maxP, r.rank,
				maxN, r.player, maxS, r.score))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func getPlayersBySection(t *Tournament) map[string][]Player {
	secPlayers := make(map[string][]Player)
	for _, pairing := range t.CurrentPairings {
		secPlayers[pairing.Section] = append(secPlayers[pairing.Section],
			pairing.WhitePlayer)
		if !pairing.IsByePairing {
			secPlayers[pairing.Section] = append(secPlayers[pairing.Section],
				pairing.BlackPlayer)
		}
	}

	return secPlayers
}
