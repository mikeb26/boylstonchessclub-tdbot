/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package bcc

import (
	"fmt"
	"log"
	"sort"
	"strings"
)

// buildEntriesOutput formats entries into grouped, aligned string output
func BuildEntriesOutput(t *Tournament) string {
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

	if len(t.CurrentPairings) == 0 {
		sb.WriteString("** error: missing pairings")
		log.Printf("bcc: entries: empty pairings")
		return sb.String()
	}

	for _, sec := range sectionNames {
		list := sections[sec]

		type row struct {
			player, rating   string
			memid, ratingInt int
		}
		var rows []row
		for _, p := range list {
			player := &p.WhitePlayer
			for {
				n := player.DisplayName
				r := "unrated"
				if player.PrimaryRating != 0 {
					r = fmt.Sprintf("%v", player.PrimaryRating)
				}
				id := player.UscfID
				rows = append(rows, row{player: n, rating: r, memid: id,
					ratingInt: player.PrimaryRating})
				if player == &p.BlackPlayer || p.IsByePairing {
					break
				}
				player = &p.BlackPlayer
			}
		}

		sort.Slice(rows, func(i, j int) bool {
			return rows[i].ratingInt > rows[j].ratingInt
		})

		// Compute column widths
		maxP, maxR, maxM := len("Player"), len("Rating"), len("USCF memid")
		for _, r := range rows {
			if l := len(r.player); l > maxP {
				maxP = l
			}
			if l := len(r.rating); l > maxR {
				maxR = l
			}
			if l := len(fmt.Sprintf("%v", r.memid)); l > maxM {
				maxM = l
			}
		}

		// Write section header and table
		if len(sectionNames) > 1 {
			if sec == "" {
				sec = "UNNAMED"
			}
			sb.WriteString(fmt.Sprintf("%s Section\n", sec))
		}
		sb.WriteString(fmt.Sprintf("%-*s  %-*s  %-*s\n", maxP, "Player", maxR,
			"Rating", maxM, "USCF memid"))
		for _, r := range rows {
			sb.WriteString(fmt.Sprintf("%-*s  %-*s  %-*v\n", maxP, r.player,
				maxR, r.rating, maxM, r.memid))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
