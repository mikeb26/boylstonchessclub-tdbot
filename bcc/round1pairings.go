/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package bcc

import (
	"context"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mikeb26/boylstonchessclub-tdbot/uschess"
)

const (
	round1PairingCorrectionConcurrency = 8
	round1PairingCorrectionTimeout     = 30 * time.Second
)

type uschessRatingProfileLookup func(context.Context,
	uschess.MemID, bool) (*uschess.Player, error)

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

func correctRound1PairingEntries(entries []Entry) []Entry {
	ctx, cancel := context.WithTimeout(context.Background(),
		round1PairingCorrectionTimeout)
	defer cancel()

	client := uschess.NewClient(ctx)
	return correctRound1PairingEntriesWithLookup(ctx, entries,
		client.FetchPlayer)
}

func correctRound1PairingEntriesWithLookup(ctx context.Context, entries []Entry,
	lookup uschessRatingProfileLookup) []Entry {

	corrected := make([]Entry, len(entries))
	copy(corrected, entries)

	if lookup == nil {
		return corrected
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, round1PairingCorrectionConcurrency)

	for idx, entry := range entries {
		if entry.UscfID <= 0 {
			continue
		}

		idx, entry := idx, entry
		wg.Add(1)
		go func() {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			player, err := lookup(ctx, uschess.MemID(entry.UscfID), false)
			if err != nil {
				return
			}

			updated, ok := applyUSChessRound1Correction(entry, player)
			if !ok {
				return
			}

			mu.Lock()
			corrected[idx] = updated
			mu.Unlock()
		}()
	}

	wg.Wait()

	return corrected
}

func applyUSChessRound1Correction(entry Entry, player *uschess.Player) (Entry,
	bool) {

	if player == nil || player.MemberID == 0 {
		return entry, false
	}

	rating := strings.TrimSpace(player.RegSupplement.Rating)
	if !isUsableSupplementRating(rating) {
		return entry, false
	}

	entry.PrimaryRating = rating
	entry.PrimaryRatingType = "regular"
	if !player.RegSupplement.Date.IsZero() {
		entry.PrimaryRatingDate = player.RegSupplement.Date.Format("2006-01-02")
	}

	firstName, lastName := splitUSChessPlayerName(player.Name)
	if firstName != "" {
		entry.FirstName = firstName
	}
	if lastName != "" {
		entry.LastName = lastName
	}

	return entry, true
}

func isUsableSupplementRating(rating string) bool {
	rating = strings.TrimSpace(rating)
	return rating != "" && rating != "<unrated>" && strRatingToInt(rating) > 0
}

func splitUSChessPlayerName(name string) (string, string) {
	parts := strings.Fields(strings.TrimSpace(name))
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}

	return parts[0], strings.Join(parts[1:], " ")
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
