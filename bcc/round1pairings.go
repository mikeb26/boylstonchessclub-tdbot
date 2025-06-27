/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package bcc

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type section struct {
	Players  []Entry
	Pairings []Pairing
}

type color int

const (
	white color = iota
	black
)

func predictRound1Pairings(entries []Entry) []Pairing {
	sections := buildSections(entries)

	pairings := make([]Pairing, 0)
	for _, sec := range sections {
		pairings = append(pairings, sec.Pairings...)
	}

	return pairings
}

func buildSections(entries []Entry) map[string]section {
	sections := make(map[string]section)

	for _, entry := range entries {
		sec, ok := sections[entry.SectionName]
		if !ok {
			sections[entry.SectionName] = section{
				Players: []Entry{entry},
			}
			sec, _ = sections[entry.SectionName]
		} else {
			sec.Players = append(sec.Players, entry)
		}
		sections[entry.SectionName] = sec
	}

	// Sort sections
	var sectionNames []string
	for sec := range sections {
		sectionNames = append(sectionNames, sec)
	}
	sort.Sort(SectionSorter(sectionNames))

	boardNum := 1
	for _, key := range sectionNames {
		sec := sections[key]
		buildPairingsInSection(&sec, &boardNum)
		sections[key] = sec
	}

	return sections
}

func buildPairingsInSection(sec *section, boardNum *int) {
	sec.Pairings = make([]Pairing, 0)
	requestedByes := make([]Entry, 0)
	var oddBye *Entry
	remainingPlayers := make([]Entry, 0)

	for _, entry := range sec.Players {
		if round1ByeRequested(entry.ByeRequests) {
			requestedByes = append(requestedByes, entry)
		} else {
			remainingPlayers = append(remainingPlayers, entry)
		}
	}
	sort.Slice(remainingPlayers, func(i, j int) bool {
		return strRatingToInt(remainingPlayers[i].PrimaryRating) >
			strRatingToInt(remainingPlayers[j].PrimaryRating)
	})
	if len(remainingPlayers)%2 == 1 {
		last := remainingPlayers[len(remainingPlayers)-1]
		oddBye = &last
		remainingPlayers = remainingPlayers[:len(remainingPlayers)-1]
	}

	// build pairings from the remaining even set of players
	// highest rated player gets white against (n/2)-th highest
	// rated player. 2nd highest rated player gets black against
	// (n/2 + 1)-th highest rated player. & so on.
	lastTopColor := black
	for len(remainingPlayers) >= 2 {
		n := len(remainingPlayers)
		top := remainingPlayers[0]
		opp := remainingPlayers[n/2]
		if lastTopColor == black {
			lastTopColor = white
			sec.Pairings = append(sec.Pairings, buildOnePairing(top, opp,
				boardNum))
		} else {
			lastTopColor = black
			sec.Pairings = append(sec.Pairings, buildOnePairing(opp, top,
				boardNum))
		}
		remainingPlayers = removeIndex(remainingPlayers, n/2)
		remainingPlayers = removeIndex(remainingPlayers, 0)
	}
	for _, p := range requestedByes {
		sec.Pairings = append(sec.Pairings, buildOneBye(p, 0.5))
	}
	if oddBye != nil {
		sec.Pairings = append(sec.Pairings, buildOneBye(*oddBye, 1.0))
	}
}

func buildOnePairing(w, b Entry, boardNum *int) Pairing {
	var p Pairing

	p.WhitePlayer = entryToPlayer(w)
	p.BlackPlayer = entryToPlayer(b)
	p.Section = w.SectionName
	p.RoundNumber = 1
	p.BoardNumber = *boardNum
	(*boardNum)++
	p.IsByePairing = false

	return p
}

func buildOneBye(w Entry, points float64) Pairing {
	var p Pairing

	p.WhitePlayer = entryToPlayer(w)
	p.Section = w.SectionName
	p.RoundNumber = 1
	p.BoardNumber = 0
	p.IsByePairing = true
	p.WhitePoints = &points

	return p
}

func round1ByeRequested(req string) bool {
	s := strings.TrimSpace(req)
	if s == "" {
		return false
	}
	// If input is just a number, e.g., "1"
	numOnly := regexp.MustCompile(`^\d+$`)
	if numOnly.MatchString(s) {
		if n, err := strconv.Atoi(s); err == nil && n == 1 {
			return true
		}
	}

	// Look for patterns like "round 1,5" or "rnds 1&4"
	sl := strings.ToLower(s)
	listRe := regexp.MustCompile(`(?i)\b(?:round|rnd|rounds|rnds)\b[\s:]*((?:\d+(?:\s*[,&;/]\s*\d+)*))`)
	if matches := listRe.FindStringSubmatch(sl); matches != nil {
		nums := regexp.MustCompile(`\d+`).FindAllString(matches[1], -1)
		for _, m := range nums {
			if n, err := strconv.Atoi(m); err == nil && n == 1 {
				return true
			}
		}
	}

	return false
}

func removeIndex(s []Entry, i int) []Entry {
	return append(s[:i], s[i+1:]...)
}
