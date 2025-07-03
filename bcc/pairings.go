/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package bcc

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mikeb26/boylstonchessclub-tdbot/internal"
)

// buildPairingsOutput formats pairings into grouped, aligned string output
func BuildPairingsOutput(t *Tournament) string {
	// Group pairings by section
	sections := make(map[string][]Pairing)
	for _, p := range t.CurrentPairings {
		sections[p.Section] = append(sections[p.Section], p)
	}
	// Sort section names using custom criteria
	var sectionNames []string
	for sec := range sections {
		sectionNames = append(sectionNames, sec)
	}
	// Use named sectionSorter instead of anonymous comparator
	sort.Sort(SectionSorter(sectionNames))
	var sb strings.Builder

	sb.WriteString("* Please note that pairings are tentative and subject to change before the start of the round.\n\n")

	if len(t.CurrentPairings) > 0 {
		if t.IsPredicted() {
			sb.WriteString(fmt.Sprintf("Round %v pairings are not yet posted, but here are my predicted round %v pairings:\n\n",
				t.CurrentPairings[0].RoundNumber,
				t.CurrentPairings[0].RoundNumber))
		} else {
			sb.WriteString(fmt.Sprintf("Posted Round %v Pairings:\n\n",
				t.CurrentPairings[0].RoundNumber))
		}
	} else {
		sb.WriteString("No pairings posted nor predicted")
	}

	for _, sec := range sectionNames {
		list := sections[sec]
		// Sort by board number
		sort.Slice(list, func(i, j int) bool {
			// 0 means bye
			return list[i].BoardNumber != 0 &&
				list[i].BoardNumber < list[j].BoardNumber
		})

		type row struct{ board, white, black string }
		var rows []row
		for _, p := range list {
			var w, b, bl string
			w = fmt.Sprintf("%s(%d %v)", p.WhitePlayer.DisplayName,
				p.WhitePlayer.PrimaryRating,
				internal.ScoreToString(p.WhitePlayer.CurrentScore))
			if p.IsByePairing {
				b = "n/a"
				if p.WhitePoints != nil && *p.WhitePoints == 1.0 {
					bl = "BYE(1)"
				} else {
					bl = "BYE(½)"
				}
			} else {
				b = fmt.Sprintf("%d.", p.BoardNumber)
				bl = fmt.Sprintf("%s(%d %v)", p.BlackPlayer.DisplayName,
					p.BlackPlayer.PrimaryRating, internal.ScoreToString(p.BlackPlayer.CurrentScore))
			}
			rows = append(rows, row{board: b, white: w, black: bl})
		}

		// Compute column widths
		maxB, maxW, maxBl := len("Board"), len("White"), len("Black")
		for _, r := range rows {
			if l := len(r.board); l > maxB {
				maxB = l
			}
			if l := len(r.white); l > maxW {
				maxW = l
			}
			if l := len(r.black); l > maxBl {
				maxBl = l
			}
		}

		// Write section header and table
		if len(sectionNames) > 1 {
			if sec == "" {
				sec = "UNNAMED"
			}
			sb.WriteString(fmt.Sprintf("%s Section\n", sec))
		}
		sb.WriteString(fmt.Sprintf("%-*s  %-*s  %-*s\n", maxB, "Board", maxW,
			"White", maxBl, "Black"))
		for _, r := range rows {
			sb.WriteString(fmt.Sprintf("%-*s  %-*s  %-*s\n", maxB, r.board,
				maxW, r.white, maxBl, r.black))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
