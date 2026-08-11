/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package bcc

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/mikeb26/boylstonchessclub-tdbot/internal"
)

// vended by https://beta.boylstonchess.org/api/event/<eventId>/tournament
// Tournament represents the players and current pairings for a specific event.
type Tournament struct {
	Players         []Player  `json:"players"`
	CurrentPairings []Pairing `json:"currentPairings"`

	isPredicted bool
	source      Source
}

// Player represents a participant in the tournament.
type Player struct {
	FirstName            string  `json:"firstName"`
	MiddleName           string  `json:"middleName"`
	LastName             string  `json:"lastName"`
	NameTitle            string  `json:"nameTitle"`
	NameSuffix           string  `json:"nameSuffix"`
	ChessTitle           string  `json:"chessTitle"`
	DisplayName          string  `json:"displayName"`
	UscfID               int     `json:"uscfId"`
	FideID               int     `json:"fideId"`
	FideCountry          string  `json:"fideCountry"`
	PrimaryRating        int     `json:"primaryRating"`
	SecondaryRating      int     `json:"secondaryRating"`
	LiveRating           int     `json:"liveRating"`
	LiveRatingProvo      int     `json:"liveRatingProvo"`
	PostEventRating      int     `json:"postEventRating"`
	PostEventRatingProvo int     `json:"postEventRatingProvo"`
	PostEventBonusPoints float64 `json:"postEventBonusPoints"`
	RatingChange         int     `json:"ratingChange"`
	PairingNumber        int     `json:"pairingNumber"`
	CurrentScore         float64 `json:"currentScore"`
	CurrentScoreAG       float64 `json:"currentScoreAfterGame"`
	GamesCompleted       int     `json:"gamesCompleted"`
	Place                string  `json:"place"`
	PlaceNumber          int     `json:"placeNumber"`
	SectionName          string  `json:"sectionName"`

	emptyResult bool
}

// Pairing represents a single board pairing in the tournament.
type Pairing struct {
	WhitePlayer  Player   `json:"whitePlayer"`
	BlackPlayer  Player   `json:"blackPlayer"`
	Section      string   `json:"section"`
	RoundNumber  int      `json:"roundNumber"`
	BoardNumber  int      `json:"boardNumber"`
	IsByePairing bool     `json:"isByePairing"`
	WhitePoints  *float64 `json:"whitePoints"`
	BlackPoints  *float64 `json:"blackPoints"`
	ResultCode   string   `json:"resultCode"`
	WhiteResult  *string  `json:"whiteResult"`
	BlackResult  *string  `json:"blackResult"`
	GameLink     string   `json:"gameLink"`
}

func GetTournament(eventId int64) (*Tournament, error) {
	var wg sync.WaitGroup
	var tViaApi, tViaWeb *Tournament
	var apiErr, webErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		tViaApi, apiErr = getTournamentViaApi(eventId)
	}()
	go func() {
		defer wg.Done()
		tViaWeb, webErr = getTournamentViaWeb(eventId)
	}()
	wg.Wait()

	if apiErr != nil {
		if webErr != nil {
			// both errored; prefer the api error response
			return tViaApi, apiErr
		} // else api errored; use the web response
		return tViaWeb, nil
	} else if webErr != nil {
		// web errored, use the api response
		return tViaApi, nil
	} // else both api and web were successful, prefer the api response
	// but there are situations where it returns stale data and we should
	// patch it from data from the web response.
	combinedSources := false
	if len(tViaApi.CurrentPairings) > 0 &&
		len(tViaWeb.CurrentPairings) > 0 &&
		tViaApi.CurrentPairings[0].RoundNumber <
			tViaWeb.CurrentPairings[0].RoundNumber {

		// api is returning stale pairing data, so prefer the web response
		tViaApi.CurrentPairings = tViaWeb.CurrentPairings
		combinedSources = true
	}

	// getTournamentViaWeb has already merged the live standings into the web
	// players. Preserve the API's richer player records while using those live
	// scores and places.
	if mergeStandings(tViaApi, tViaWeb) {
		combinedSources = true
	}
	if combinedSources {
		tViaApi.source = SourceBoth
	}
	return tViaApi, nil
}

// parseStandings extracts the standing tables from a public SwissSys page.
// The primary section is headed by h1; additional sections are headed by h3.
func parseStandings(doc *goquery.Document, t *Tournament) error {
	t.Players = nil
	doc.Find("div#standings h1, div#standings h3").Each(func(_ int, heading *goquery.Selection) {
		section := ""
		if goquery.NodeName(heading) == "h3" {
			text := strings.TrimSpace(heading.Text())
			if colon := strings.LastIndex(text, ":"); colon >= 0 {
				section = strings.TrimSpace(text[colon+1:])
			}
		}

		table := heading.Next()
		for table.Length() > 0 && !table.Is("table") {
			table = table.Next()
		}
		if table.Length() == 0 {
			return
		}

		position := 0
		place := 0
		priorScore := math.NaN()
		table.Find("tr").Each(func(_ int, row *goquery.Selection) {
			cells := row.Find("td")
			if cells.Length() < 7 || strings.TrimSpace(cells.Eq(0).Text()) == "#" {
				return
			}

			score, err := strconv.ParseFloat(strings.TrimSpace(cells.Eq(6).Text()), 64)
			if err != nil {
				return
			}
			position++
			if score != priorScore {
				place = position
				priorScore = score
			}
			t.Players = append(t.Players, Player{
				DisplayName:    internal.NormalizeName(cells.Eq(2).Text()),
				PrimaryRating:  strRatingToInt(cells.Eq(4).Text()),
				CurrentScore:   score,
				CurrentScoreAG: score,
				PlaceNumber:    place,
				SectionName:    section,
			})
		})
	})

	return nil
}

// mergeStandings copies scores and places onto the richer tournament records
// only when the incoming standings have a higher total score. This lets us use
// the source that has completed more games: public SwissSys standings may be
// newer than the API, but the API can also be more current.
//
// Section names on the primary SwissSys table are omitted, so players are
// matched by name when it occurs only once in the event.
func mergeStandings(t, standings *Tournament) bool {
	if tournamentScoreTotal(standings) <= tournamentScoreTotal(t) {
		return false
	}
	return applyStandings(t, standings)
}

// applyStandings copies standings onto a tournament without comparing their
// aggregate score. It is used while constructing the website result: the
// SwissSys standings page is the authoritative score source for that result,
// whereas pairing parsing is only a fallback and can fail to associate a
// player whose displayed name differs from the entries page.
func applyStandings(t, standings *Tournament) bool {
	playersByName := make(map[string][]*Player)
	for i := range t.Players {
		player := &t.Players[i]
		key := normalizedStandingName(player.DisplayName)
		playersByName[key] = append(playersByName[key], player)
	}

	merged := false
	for _, standing := range standings.Players {
		matches := playersByName[normalizedStandingName(standing.DisplayName)]
		if standing.SectionName != "" {
			var sectionMatches []*Player
			for _, player := range matches {
				if strings.EqualFold(player.SectionName, standing.SectionName) {
					sectionMatches = append(sectionMatches, player)
				}
			}
			// Entries pages do not always identify a player's section. In that
			// case, retain an unambiguous name match rather than discarding it
			// simply because the SwissSys table does identify its section.
			if len(sectionMatches) > 0 {
				matches = sectionMatches
			}
		}
		if len(matches) != 1 {
			matches = findSimilarNameMatches(t.Players, standing)
		}
		if len(matches) != 1 {
			continue
		}

		player := matches[0]
		player.CurrentScore = standing.CurrentScore
		player.CurrentScoreAG = standing.CurrentScoreAG
		player.PlaceNumber = standing.PlaceNumber
		merged = true
	}
	return merged
}

// tournamentScoreTotal is a source-independent indication of tournament
// progress. Every completed chess game adds one point across its two players,
// so the larger total has more up-to-date results without relying on an
// individual player name being present in both sources.
func tournamentScoreTotal(t *Tournament) float64 {
	var total float64
	for _, player := range t.Players {
		total += player.CurrentScore
	}
	return total
}

func normalizedStandingName(name string) string {
	return strings.ToLower(internal.NormalizeName(name))
}

// findSimilarNameMatches handles minor differences between the registration
// data and SwissSys, such as middle names, nicknames, or a transposed surname.
// It only returns a match when the abbreviated first and last names are unique
// in the relevant section.
func findSimilarNameMatches(players []Player, standing Player) []*Player {
	standingName := strings.Fields(normalizedStandingName(standing.DisplayName))
	if len(standingName) != 2 {
		return nil
	}

	var matches []*Player
	for i := range players {
		player := &players[i]
		// Entries pages may omit section names. Exclude only players that have
		// an explicitly different section, leaving an otherwise unambiguous
		// name match available to merge.
		if standing.SectionName != "" && player.SectionName != "" &&
			!strings.EqualFold(player.SectionName, standing.SectionName) {
			continue
		}
		playerFirst := strings.ToLower(player.FirstName)
		playerLast := strings.ToLower(player.LastName)
		if playerFirst == "" || playerLast == "" {
			name := strings.Fields(player.DisplayName)
			if len(name) < 2 {
				continue
			}
			playerFirst = strings.ToLower(name[0])
			playerLast = strings.ToLower(name[len(name)-1])
		}
		if playerFirst == standingName[0] && playerLast == standingName[1] {
			matches = append(matches, player)
			continue
		}
		if playerLast == standingName[1] {
			matches = append(matches, player)
			continue
		}
		if playerFirst == standingName[0] && playerLast[0] == standingName[1][0] {
			matches = append(matches, player)
		}
	}
	return matches
}

// getTournamentViaApi fetches the tournament data (players and pairings) for a
// given eventId from the JSON API.
func getTournamentViaApi(eventId int64) (*Tournament, error) {
	url := fmt.Sprintf("https://beta.boylstonchess.org/api/event/%d/tournament",
		eventId)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return &Tournament{},
			fmt.Errorf("unable to fetch bcc tournament (new): %w", err)
	}

	req.Header.Set("User-Agent", internal.UserAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return &Tournament{},
			fmt.Errorf("unable to fetch bcc tournament (do): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		detail, err := GetEventDetail(eventId)
		if err == nil {
			detail.Entries = correctRound1PairingEntries(detail.Entries)
			return eventDetailToTournament(&detail), nil
		} else {
			err = fmt.Errorf("unable to fetch %v: http status: %v", url,
				resp.StatusCode)
		}

		return &Tournament{}, err
	}

	tourney := &Tournament{
		source: SourceAPI,
	}
	if err := json.NewDecoder(resp.Body).Decode(&tourney); err != nil {
		return &Tournament{}, fmt.Errorf("unable to parse bcc tournament: %w",
			err)
	}

	if len(tourney.Players) == 0 {
		err = fmt.Errorf("bcc tournament API returned an empty response")
		return &Tournament{}, err
	}

	return tourney, nil
}

// getTournamentViaWeb fetches the tournament data by scraping the public website
// pages: entries, pairings, and standings for the given eventId.
func getTournamentViaWeb(eventId int64) (*Tournament, error) {
	// Prepare URLs
	entriesURL := fmt.Sprintf("https://boylstonchess.org/tournament/entries/%d", eventId)
	pairingsURL := fmt.Sprintf("https://boylstonchess.org/files/event/%d/pairings", eventId)
	standingsURL := fmt.Sprintf("https://boylstonchess.org/files/event/%d/standings", eventId)

	// Fetch all public tournament pages concurrently.
	var wg sync.WaitGroup
	var entriesDoc, pairingsDoc, standingsDoc *goquery.Document
	var errEntries, errPairings, errStandings error
	wg.Add(3)
	go func() {
		defer wg.Done()
		entriesDoc, errEntries = fetchDoc(entriesURL)
	}()
	go func() {
		defer wg.Done()
		pairingsDoc, errPairings = fetchDoc(pairingsURL)
	}()
	go func() {
		defer wg.Done()
		standingsDoc, errStandings = fetchDoc(standingsURL)
	}()
	wg.Wait()

	tourney := &Tournament{source: SourceWebsite}

	if errEntries != nil {
		return nil, fmt.Errorf("unable to fetch entries page: %w", errEntries)

	}
	if err := parsePlayers(entriesDoc, tourney); err != nil {
		return nil, fmt.Errorf("unable to parse players: %w", err)
	}

	if errPairings != nil {
		return nil, fmt.Errorf("unable to fetch pairings page: %w", errPairings)

	}
	if err := parsePairings(pairingsDoc, tourney); err != nil {
		return nil, fmt.Errorf("unable to parse pairings: %w", err)
	}

	// The standings page is published by SwissSys and can be newer than the
	// beta API. It is optional so a missing or malformed page does not make the
	// otherwise useful entries and pairings unavailable.
	if errStandings == nil {
		standings := &Tournament{}
		if err := parseStandings(standingsDoc, standings); err == nil && len(standings.Players) > 0 {
			applyStandings(tourney, standings)
		}
	}

	return tourney, nil
}

// parsePlayers extracts Player entries from the entries table in the document.
func parsePlayers(doc *goquery.Document, t *Tournament) error {
	t.Players = nil
	doc.Find("table#members tbody tr").Each(func(_ int, s *goquery.Selection) {
		cells := s.Find("td")
		if cells.Length() < 4 {
			return
		}
		num, _ := strconv.Atoi(strings.TrimSpace(cells.Eq(0).Text()))
		name := strings.TrimSpace(cells.Eq(1).Text())
		ratingStr := strings.TrimSpace(cells.Eq(2).Text())
		rating := 0
		if r, err := strconv.Atoi(ratingStr); err == nil {
			rating = r
		}
		uscfID, _ := strconv.Atoi(strings.TrimSpace(cells.Eq(3).Text()))
		section := ""
		if cells.Length() >= 5 {
			section = strings.TrimSpace(cells.Eq(4).Text())
		}

		p := Player{
			DisplayName:   internal.NormalizeName(name),
			PairingNumber: num,
			PrimaryRating: rating,
			UscfID:        uscfID,
			SectionName:   section,
		}
		parts := strings.Fields(p.DisplayName)
		if len(parts) > 0 {
			p.FirstName = parts[0]
		}
		if len(parts) > 1 {
			p.LastName = parts[len(parts)-1]
		}
		t.Players = append(t.Players, p)
	})

	return nil
}

// parsePairings extracts Pairing entries from the pairings tables in the document.
// Skips the main h1 section header when there are sub-sections (h2) present.
// Also handles additional malformed H3 sections for robustness.
func parsePairings(doc *goquery.Document, t *Tournament) error {
	t.CurrentPairings = nil
	// Determine if there are multiple subsections
	hasSubSections := doc.Find("div#pairings h2").Length() > 0

	// Handle h1 & h2 sections
	doc.Find("div#pairings h1, div#pairings h2").Each(func(_ int, s *goquery.Selection) {
		node := goquery.NodeName(s)
		// Skip main header when sub-sections exist
		if node == "h1" && hasSubSections {
			return
		}

		var section string
		if node == "h1" {
			// main section, use link text if available
			if title := strings.TrimSpace(s.Find("a").Text()); title != "" {
				section = title
			} else {
				section = strings.TrimSpace(s.Text())
			}
			section = strings.Replace(section, "Pairings", "", -1)
			section = strings.Trim(section, " –:\t")
		} else {
			// subsection header
			section = strings.Replace(s.Text(), "Section", "", -1)
			section = strings.TrimSpace(section)
		}

		// find the next table sibling
		tableSel := s.Next()
		for tableSel.Length() > 0 && !tableSel.Is("table") {
			tableSel = tableSel.Next()
		}
		if tableSel.Length() == 0 {
			return
		}

		parsePairingRows(tableSel, section, t)
	})

	// Handle malformed H3 sections (e.g., event 1371)
	doc.Find("h3").Each(func(_ int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if !strings.HasPrefix(text, "Pairings") {
			return
		}
		// derive section name after last colon
		colonIdx := strings.LastIndex(text, ":")
		var section string
		if colonIdx >= 0 && colonIdx < len(text)-1 {
			section = strings.TrimSpace(text[colonIdx+1:])
		} else {
			section = text
		}

		// find the next table sibling
		tableSel := s.Next()
		for tableSel.Length() > 0 && !tableSel.Is("table") {
			tableSel = tableSel.Next()
		}
		if tableSel.Length() == 0 {
			return
		}

		parsePairingRows(tableSel, section, t)
	})

	fixupStandings(t)

	return nil
}

// Determine Player.PlaceOrder and fixup CurrentScoreAG if needed
func fixupStandings(t *Tournament) {
	haveAnyEmptyResult := false

	for _, p := range t.CurrentPairings {
		if p.WhitePlayer.emptyResult ||
			(!p.IsByePairing && p.BlackPlayer.emptyResult) {
			haveAnyEmptyResult = true
		}
	}

	if haveAnyEmptyResult {
		for idx, p := range t.CurrentPairings {
			if !p.IsByePairing && !p.BlackPlayer.emptyResult {
				p.BlackPlayer.CurrentScoreAG = p.BlackPlayer.CurrentScore
			}
			if !p.WhitePlayer.emptyResult {
				p.WhitePlayer.CurrentScoreAG = p.WhitePlayer.CurrentScore
			}
			t.CurrentPairings[idx] = p
		}
	}

	updatePlayersFromPairings(t)

	// compute placeorder
	maxScore := float64(0.0)
	secPlayers := getPlayersBySection(t)
	for _, players := range secPlayers {
		sort.Slice(players, func(i, j int) bool {
			return players[i].CurrentScoreAG > players[j].CurrentScoreAG
		})
		if players[0].CurrentScoreAG > maxScore {
			maxScore = players[0].CurrentScoreAG
		}
		for idx, p := range players {
			p.PlaceNumber = idx + 1
		}
	}
	// best guess at round number
	roundNumber := int(math.Round(maxScore) + 1)
	for idx, _ := range t.CurrentPairings {
		t.CurrentPairings[idx].RoundNumber = roundNumber

	}
}

func updatePlayersFromPairings(t *Tournament) {
	for _, p := range t.CurrentPairings {
		for idx, _ := range t.Players {
			pl := &t.Players[idx]
			check := []*Player{&p.WhitePlayer, &p.BlackPlayer}
			for _, cp := range check {
				if cp.DisplayName == pl.DisplayName &&
					cp.PrimaryRating == pl.PrimaryRating {
					pl.CurrentScore = cp.CurrentScore
					pl.CurrentScoreAG = cp.CurrentScoreAG
				}
			}
		}
	}
}

// parsePairingRows iterates each row in a table and appends valid pairings to the tournament.
func parsePairingRows(tableSel *goquery.Selection, section string, t *Tournament) {
	tableSel.Find("tr").Each(func(_ int, row *goquery.Selection) {
		if pair, ok := parsePairingRow(row, section); ok {
			t.CurrentPairings = append(t.CurrentPairings, *pair)
		}
	})
}

// parsePairingRow parses a single table row into a Pairing. Returns ok=false to skip row.
func parsePairingRow(row *goquery.Selection, section string) (*Pairing, bool) {
	cells := row.Find("td")
	if cells.Length() < 5 {
		return nil, false
	}
	boardText := strings.TrimSpace(cells.Eq(0).Text())
	if strings.EqualFold(boardText, "Bd") {
		return nil, false
	}
	board, err := strconv.Atoi(boardText)
	if err != nil {
		board = 0
	}
	whiteRes := strings.TrimSpace(cells.Eq(1).Text())
	whiteName := strings.TrimSpace(cells.Eq(2).Text())
	blackRes := strings.TrimSpace(cells.Eq(3).Text())
	blackName := strings.TrimSpace(cells.Eq(4).Text())

	// parse players and initial scores
	wp := parsePlayerRef(whiteName)
	bp := parsePlayerRef(blackName)

	// pointers for raw results
	var wResPtr, bResPtr *string
	if whiteRes != "" {
		tmp := whiteRes
		wResPtr = &tmp
	}
	if blackRes != "" {
		tmp := blackRes
		bResPtr = &tmp
	}

	// adjust CurrentScoreAG based on result when present, handling “½”
	if wResPtr != nil {
		res := strings.TrimSpace(*wResPtr)
		var (
			v  float64
			ok bool
		)
		if strings.Contains(res, "½") {
			v = 0.5
			ok = true
		} else if parsed, err := strconv.ParseFloat(res, 64); err == nil {
			v = parsed
			ok = true
		}
		if ok {
			wp.CurrentScoreAG = wp.CurrentScore + v
			wp.emptyResult = false
		}
	}
	if bResPtr != nil {
		res := strings.TrimSpace(*bResPtr)
		var (
			v  float64
			ok bool
		)
		if strings.Contains(res, "½") {
			v = 0.5
			ok = true
		} else if parsed, err := strconv.ParseFloat(res, 64); err == nil {
			v = parsed
			ok = true
		}
		if ok {
			bp.CurrentScoreAG = bp.CurrentScore + v
			bp.emptyResult = false
		}
	}
	pair := Pairing{
		Section:     section,
		RoundNumber: 0,
		BoardNumber: board,
		WhitePlayer: wp,
		BlackPlayer: bp,
		ResultCode:  fmt.Sprintf("%s-%s", whiteRes, blackRes),
		WhiteResult: wResPtr,
		BlackResult: bResPtr,
	}

	// Handle bye pairings
	if bp.DisplayName == "BYE" && wp.DisplayName != "BYE" {
		pair.IsByePairing = true
		// same logic: “½”→0.5 or parse float
		if strings.Contains(whiteRes, "½") {
			pts := 0.5
			pair.WhitePoints = &pts
		} else if v, err := strconv.ParseFloat(whiteRes, 64); err == nil {
			pair.WhitePoints = &v
		}
	} else if wp.DisplayName == "BYE" && bp.DisplayName != "BYE" {
		pair.IsByePairing = true
		if strings.Contains(blackRes, "½") {
			pts := 0.5
			pair.BlackPoints = &pts
		} else if v, err := strconv.ParseFloat(blackRes, 64); err == nil {
			pair.BlackPoints = &v
		}
	}

	return &pair, true
}

// parsePlayerRef extracts a Player reference from a cell text like "12 John Doe (2250 3.0)".
func parsePlayerRef(text string) Player {
	// Handle BYE as a special case
	if strings.EqualFold(strings.TrimSpace(text), "BYE") {
		return Player{DisplayName: "BYE"}
	}

	p := Player{}
	// Pairing number and details
	fields := strings.Fields(text)
	if len(fields) > 1 {
		// pairing number
		if num, err := strconv.Atoi(fields[0]); err == nil {
			p.PairingNumber = num
		}
		// extract name before parentheses
		parenStart := strings.Index(text, "(")
		nameOnly := text
		if parenStart != -1 {
			nameOnly = strings.TrimSpace(text[:parenStart])
		}
		p.DisplayName = internal.NormalizeName(nameOnly)
		nameWords := strings.Fields(p.DisplayName)
		if len(nameWords) > 0 {
			p.FirstName = nameWords[0]
		}
		if len(nameWords) > 1 {
			p.LastName = nameWords[len(nameWords)-1]
		}
		// parse rating and current score inside parentheses
		parenEnd := strings.Index(text, ")")
		if parenStart != -1 && parenEnd > parenStart {
			inside := text[parenStart+1 : parenEnd]
			parts := strings.Fields(inside)
			if len(parts) >= 1 {
				if parts[0] != "unr." {
					if r, err := strconv.Atoi(parts[0]); err == nil {
						p.PrimaryRating = r
					}
				}
			}
			if len(parts) >= 2 {
				if score, err := strconv.ParseFloat(parts[1], 64); err == nil {
					p.CurrentScore = score
					p.CurrentScoreAG = score
					p.emptyResult = true
				}
			}
		}
	}
	return p
}

func (t Tournament) IsPredicted() bool {
	return t.isPredicted
}
