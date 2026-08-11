/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package bcc

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

// TestGetTournament tests fetching tournament data and verifies that the
// list of players contains Andrew Hoy with the expected USCF ID.
func TestGetTournament(t *testing.T) {
	tourney, err := GetTournament(1358)
	if err != nil {
		t.Fatalf("GetTournament returned error: %v", err)
	}

	var found bool
	for _, p := range tourney.Players {
		if p.DisplayName == "Andrew Hoy" {
			found = true
			if p.UscfID != 12846607 {
				t.Errorf("expected USCF ID 12846607 for Andrew Hoy, got %d", p.UscfID)
			}
			break
		}
	}
	if !found {
		t.Errorf("could not find player Andrew Hoy in tournament players")
	}
}

func TestParseAndMergeStandings(t *testing.T) {
	const standingsHTML = `
<div id="standings">
  <h1>Standings after Round 1</h1>
  <table>
    <tr><td>#</td><td>Place</td><td>Name</td><td>Rtng</td><td>Post</td><td>Rd 1</td><td>Tot</td></tr>
    <tr><td>1</td><td>1-2</td><td>ALICE SMITH</td><td>1800</td><td>1805</td><td>W2</td><td>1.0</td></tr>
    <tr><td>2</td><td></td><td>BOB JONES</td><td>1700</td><td>1695</td><td>L1</td><td>0.0</td></tr>
  </table>
  <h3>SwissSys Standings: U1500</h3>
  <table>
    <tr><td>#</td><td>Place</td><td>Name</td><td>Rtng</td><td>Post</td><td>Rd 1</td><td>Tot</td></tr>
    <tr><td>1</td><td>1</td><td>CAROL WHITE</td><td>1400</td><td>1410</td><td>W2</td><td>1.0</td></tr>
    <tr><td>2</td><td>2</td><td>DAN BROWN</td><td>1300</td><td>1290</td><td>L1</td><td>0.0</td></tr>
  </table>
</div>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(standingsHTML))
	if err != nil {
		t.Fatal(err)
	}
	standings := &Tournament{}
	if err := parseStandings(doc, standings); err != nil {
		t.Fatalf("parseStandings returned error: %v", err)
	}
	if got, want := len(standings.Players), 4; got != want {
		t.Fatalf("parsed %d players; want %d", got, want)
	}
	if got, want := standings.Players[2].SectionName, "U1500"; got != want {
		t.Errorf("section = %q; want %q", got, want)
	}

	tourney := &Tournament{Players: []Player{
		{DisplayName: "Alice Smith", SectionName: "Open"},
		{DisplayName: "Bob Jones", SectionName: "Open"},
		{DisplayName: "Carol White", SectionName: "U1500"},
		{DisplayName: "Dan Brown", SectionName: "U1500"},
	}}
	mergeStandings(tourney, standings)

	for _, player := range tourney.Players {
		if player.DisplayName == "Alice Smith" || player.DisplayName == "Carol White" {
			if got, want := player.CurrentScoreAG, 1.0; got != want {
				t.Errorf("%s score = %v; want %v", player.DisplayName, got, want)
			}
		}
	}
	if got, want := tourney.Players[2].PlaceNumber, 1; got != want {
		t.Errorf("Carol's place = %d; want %d", got, want)
	}
}

func TestMergeStandingsMatchesMinorNameDifferences(t *testing.T) {
	tourney := &Tournament{Players: []Player{
		{FirstName: "Michael", LastName: "Brown", DisplayName: "Michael F Brown", SectionName: "U1910"},
		{FirstName: "Nathan", LastName: "Ruesch", DisplayName: "Nathan Ruesch", SectionName: "U1910"},
		{FirstName: "Jack", LastName: "Elsey", DisplayName: "John Elsey", SectionName: "U1510"},
	}}
	standings := &Tournament{Players: []Player{
		{DisplayName: "Michael Brown", SectionName: "U1910", CurrentScore: 1, CurrentScoreAG: 1},
		{DisplayName: "Nathan Reusch", SectionName: "U1910", CurrentScore: 1, CurrentScoreAG: 1},
		{DisplayName: "Jack Elsey", SectionName: "U1510", CurrentScore: 0.5, CurrentScoreAG: 0.5},
	}}

	mergeStandings(tourney, standings)

	for _, player := range tourney.Players {
		if player.CurrentScoreAG == 0 {
			t.Errorf("%s was not matched to its standing", player.DisplayName)
		}
	}
}

func TestParsePlayersReadsSection(t *testing.T) {
	const entriesHTML = `
<table id="members"><tbody>
  <tr><td>13</td><td>Mike Brown</td><td>1675</td><td>12689073</td><td>U1910</td></tr>
</tbody></table>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(entriesHTML))
	if err != nil {
		t.Fatal(err)
	}
	tourney := &Tournament{}
	if err := parsePlayers(doc, tourney); err != nil {
		t.Fatalf("parsePlayers returned error: %v", err)
	}
	if got, want := tourney.Players[0].SectionName, "U1910"; got != want {
		t.Errorf("section = %q; want %q", got, want)
	}
}

func TestApplyStandingsReplacesWebPairingScoresWithSwissSysScores(t *testing.T) {
	// Pairing parsing can retain an unmatched player's zero score while still
	// having the same total as SwissSys. The standings page must win when
	// constructing the website source; freshness comparison is only needed
	// later, when comparing website and API sources.
	tourney := &Tournament{Players: []Player{
		{FirstName: "Michael", LastName: "Brown", DisplayName: "Michael F Brown", CurrentScore: 0, CurrentScoreAG: 0},
		{DisplayName: "Other Player", CurrentScore: 1, CurrentScoreAG: 1},
	}}
	standings := &Tournament{Players: []Player{
		{DisplayName: "Michael Brown", SectionName: "U1910", CurrentScore: 1, CurrentScoreAG: 1},
		{DisplayName: "Other Player", CurrentScore: 0, CurrentScoreAG: 0},
	}}

	if !applyStandings(tourney, standings) {
		t.Fatal("applyStandings did not report merged standings")
	}
	if got, want := tourney.Players[0].CurrentScore, 1.0; got != want {
		t.Errorf("Michael Brown score = %v; want %v", got, want)
	}
}

func TestMergeStandingsUsesSourceWithHigherScoreTotal(t *testing.T) {
	tests := []struct {
		name            string
		apiScore        float64
		webScore        float64
		wantMergedScore float64
	}{
		{
			name:            "API standings are newer",
			apiScore:        1.5,
			webScore:        1.0,
			wantMergedScore: 1.5,
		},
		{
			name:            "web standings are newer",
			apiScore:        1.0,
			webScore:        1.5,
			wantMergedScore: 1.5,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			api := &Tournament{Players: []Player{{
				DisplayName:    "Foo",
				CurrentScore:   test.apiScore,
				CurrentScoreAG: test.apiScore,
				PlaceNumber:    1,
			}}}
			web := &Tournament{Players: []Player{{
				DisplayName:    "Foo",
				CurrentScore:   test.webScore,
				CurrentScoreAG: test.webScore,
				PlaceNumber:    2,
			}}}

			mergeStandings(api, web)

			if got := api.Players[0].CurrentScore; got != test.wantMergedScore {
				t.Errorf("score = %v; want %v", got, test.wantMergedScore)
			}
		})
	}
}

func TestMergeStandingsReportsWhetherItMergedData(t *testing.T) {
	api := &Tournament{Players: []Player{{
		DisplayName: "Foo", CurrentScore: 1.5, CurrentScoreAG: 1.5,
	}}}
	web := &Tournament{Players: []Player{{
		DisplayName: "Foo", CurrentScore: 1, CurrentScoreAG: 1,
	}}}
	if mergeStandings(api, web) {
		t.Error("mergeStandings reported a merge when API standings were newer")
	}

	web.Players[0].CurrentScore = 2
	web.Players[0].CurrentScoreAG = 2
	if !mergeStandings(api, web) {
		t.Error("mergeStandings did not report a merge when web standings were newer")
	}
}
