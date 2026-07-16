/* Copyright © 2026 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package uscfutils

import (
	"strings"
	"testing"

	uschess "github.com/mikeb26/uschess-go"
)

func TestBuildCrossTableOutput(t *testing.T) {
	section := uschess.MinimalSection{Name: "Open"}
	standings := uschess.StandingsOneSection{
		{
			Ordinal:       1,
			PairingNumber: 1,
			FirstName:     "Alice",
			LastName:      "Player",
			MemberId:      "1",
			Score:         1.5,
			Ratings: []uschess.RatingRecord{{
				RatingType: uschess.RatingTypeR, PreRating: 1500, PostRating: 1510,
			}},
			RoundOutcomes: []uschess.StandingsRound{
				{Outcome: uschess.PlayerOutcomeWin, OpponentOrdinal: 2, Color: "White"},
				{Outcome: uschess.PlayerOutcomeByeHalf},
			},
		},
		{
			Ordinal:       2,
			PairingNumber: 2,
			FirstName:     "Bob",
			LastName:      "Player",
			MemberId:      "2",
			Score:         0,
			Ratings: []uschess.RatingRecord{{
				RatingType: uschess.RatingTypeR, PreRating: 1400, PostRating: 1390,
			}},
			RoundOutcomes: []uschess.StandingsRound{
				{Outcome: uschess.PlayerOutcomeLoss, OpponentOrdinal: 1, Color: "Black"},
				{Outcome: uschess.PlayerOutcomeForfeit},
			},
		},
	}

	output, ratingPost := BuildCrossTableOutput(section, standings, true, "1")
	for _, want := range []string{
		"Section Open",
		"**Alice Player**",
		"W2(w)",
		"BYE(½)",
		"Bob Player",
		"L1(b)",
		"L*",
		"* indicates game was decided by forfeit",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
	if ratingPost != "1510" {
		t.Fatalf("rating post = %q; want 1510", ratingPost)
	}
}

func TestBuildCrossTableOutputFiltersByOrdinal(t *testing.T) {
	section := uschess.MinimalSection{Name: "Open"}
	standings := uschess.StandingsOneSection{
		{
			Ordinal:       1,
			PairingNumber: 50,
			FirstName:     "Target",
			LastName:      "Player",
			MemberId:      "1",
			RoundOutcomes: []uschess.StandingsRound{
				{Outcome: uschess.PlayerOutcomeWin, OpponentOrdinal: 2, Color: "White"},
			},
		},
		{
			Ordinal:       2,
			PairingNumber: 99,
			FirstName:     "Actual",
			LastName:      "Opponent",
			MemberId:      "2",
		},
		{
			Ordinal:       99,
			PairingNumber: 2,
			FirstName:     "Incorrect",
			LastName:      "Selection",
			MemberId:      "3",
		},
	}

	output, _ := BuildCrossTableOutput(section, standings, false, "1")
	for _, want := range []string{"1.  **Target Player**", "2.  Actual Opponent", "W2(w)"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "Incorrect Selection") {
		t.Fatalf("output included a row selected by pairing number:\n%s", output)
	}
}
