/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package bcc

import (
	"fmt"
	"strconv"
	"strings"
)

// Construct an artificial Tournament from an EventDetail
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

// Construct an artificial Player from an Entry
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

// SectionSorter implements sort.Interface for custom section ordering
// Order: "Open" first, then U<Number> sections descending by number, then
// others lexicographically
type SectionSorter []string

func (s SectionSorter) Len() int { return len(s) }

func (s SectionSorter) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

func (s SectionSorter) Less(i, j int) bool {
	a, b := s[i], s[j]
	// "Open" or "Championship" always first
	if a == "Open" && b != "Open" {
		return true
	}
	if b == "Open" && a != "Open" {
		return false
	}
	if a == "Championship" && b != "Championship" {
		return true
	}
	if b == "Championship" && a != "Championship" {
		return false
	}
	ua, ub := strings.HasPrefix(a, "U"), strings.HasPrefix(b, "U")
	// Both U-sections: compare numeric suffix descending
	if ua && ub {
		ai, errA := strconv.Atoi(strings.TrimPrefix(a, "U"))
		bi, errB := strconv.Atoi(strings.TrimPrefix(b, "U"))
		if errA == nil && errB == nil {
			return ai > bi
		}
	}
	// U-sections before non-U (after Championship)
	if ua != ub {
		return ua
	}
	// Fallback lexicographical
	return a < b
}
