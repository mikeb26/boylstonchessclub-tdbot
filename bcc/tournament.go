/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package bcc

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/mikeb26/boylstonchessclub-tdbot/internal"
)

// vended by https://beta.boylstonchess.org/api/event/<eventId>/tournament
// Tournament represents the players and current pairings for a specific event.
type Tournament struct {
	Players         []Player  `json:"players"`
	CurrentPairings []Pairing `json:"currentPairings"`

	isPredicted bool
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

// GetTournament fetches the tournament data (players and pairings) for a
// given eventId.
func GetTournament(eventId int64) (*Tournament, error) {
	url := fmt.Sprintf("https://beta.boylstonchess.org/api/event/%d/tournament",
		eventId)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return &Tournament{}, fmt.Errorf("unable to fetch bcc tournament (new): %w",
			err)
	}
	req.Header.Set("User-Agent", internal.UserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return &Tournament{}, fmt.Errorf("unable to fetch bcc tournament (do): %w",
			err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		detail, err := GetEventDetail(eventId)
		if err == nil {
			return eventDetailToTournament(detail)
		} else {
			err = fmt.Errorf("unable to fetch %v: http status: %v", url,
				resp.StatusCode)
		}

		return &Tournament{}, err
	}

	tourney := &Tournament{}
	if err := json.NewDecoder(resp.Body).Decode(&tourney); err != nil {
		return &Tournament{}, fmt.Errorf("unable to parse bcc tournament: %w",
			err)
	}
	return tourney, nil
}

func (t Tournament) IsPredicted() bool {
	return t.isPredicted
}
