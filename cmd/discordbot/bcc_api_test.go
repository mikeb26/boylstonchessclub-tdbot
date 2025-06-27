/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package main

import (
	"testing"
)

func TestGetBccEvents(t *testing.T) {
	events, err := getBccEvents()
	if err != nil {
		t.Fatalf("getBccEvents returned error: %v", err)
	}

	if len(events) == 0 {
		t.Fatal("expected non-zero number of events")
	}

	e := events[0]
	if e.EventID <= 0 {
		t.Errorf("expected EventID > 0, got %d", e.EventID)
	}
	if e.Title == "" {
		t.Error("expected non-empty Title")
	}
	if e.Date.IsZero() {
		t.Error("expected Date to be non-zero")
	}
	if e.StartDate.IsZero() {
		t.Error("expected StartDate to be non-zero")
	}
	if e.DayOfWeek == "" {
		t.Error("expected DayOfWeek to be non-empty")
	}
	if e.DateDisplay == "" {
		t.Error("expected DateDisplay to be non-empty")
	}
}

// TestEntryToPlayer verifies that entryToPlayer correctly parses ratings.
func TestEntryToPlayer(t *testing.T) {
	cases := []struct {
		name   string
		entry  Entry
		wantPR int
		wantSR int
	}{
		{
			name:   "both ratings with slash",
			entry:  Entry{FirstName: "John", LastName: "Doe", UscfID: 42, PrimaryRating: "559/24", SecondaryRating: "1200/15"},
			wantPR: 559,
			wantSR: 1200,
		},
		{
			name:   "ratings without slash",
			entry:  Entry{FirstName: "Jane", LastName: "Smith", UscfID: 7, PrimaryRating: "1500", SecondaryRating: "1600"},
			wantPR: 1500,
			wantSR: 1600,
		},
		{
			name:   "empty ratings",
			entry:  Entry{FirstName: "Empty", LastName: "Ratings"},
			wantPR: 0,
			wantSR: 0,
		},
		{
			name:   "malformed ratings",
			entry:  Entry{FirstName: "Bad", LastName: "Data", PrimaryRating: "abc/123", SecondaryRating: "xyz"},
			wantPR: 0,
			wantSR: 0,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := entryToPlayer(c.entry)
			if p.PrimaryRating != c.wantPR {
				t.Errorf("%s: PrimaryRating = %d; want %d", c.name, p.PrimaryRating, c.wantPR)
			}
			if p.SecondaryRating != c.wantSR {
				t.Errorf("%s: SecondaryRating = %d; want %d", c.name, p.SecondaryRating, c.wantSR)
			}
			// verify display name format
			wantName := c.entry.FirstName + " " + c.entry.LastName
			if p.DisplayName != wantName {
				t.Errorf("%s: DisplayName = %q; want %q", c.name, p.DisplayName, wantName)
			}
			// verify USCF ID is preserved
			if p.UscfID != c.entry.UscfID {
				t.Errorf("%s: UscfID = %d; want %d", c.name, p.UscfID, c.entry.UscfID)
			}
		})
	}
}

func TestGetBccEventDetail(t *testing.T) {
	detail, err := getBccEventDetail(1312)
	if err != nil {
		t.Fatalf("getBccEventDetail returned error: %v", err)
	}

	if detail.EventID != 1312 {
		t.Errorf("expected EventID == 1312, got %d", detail.EventID)
	}
	if detail.Title != "Big Money Swiss" {
		t.Error("expected non-empty Title")
	}
	if detail.StartDate.IsZero() {
		t.Error("expected StartDate to be non-zero")
	}

	if len(detail.Entries) == 0 {
		t.Error("expected at least one entry in Entries slice")
	} else {
		var andrewEntry *Entry
		for _, entry := range detail.Entries {
			if entry.FirstName == "Andrew" && entry.LastName == "Hoy" {
				andrewEntry = &entry
				break
			}
		}

		if andrewEntry == nil {
			t.Errorf("could not find andrew in the entry list as expected")
		} else if andrewEntry.UscfID != 12846607 {
			t.Errorf("wrong uscf id for andrew")
		}
	}
}

// TestGetBccTournament tests fetching tournament data and verifies that the
// list of players contains Andrew Hoy with the expected USCF ID.
func TestGetBccTournament(t *testing.T) {
	tourney, err := getBccTournament(1358)
	if err != nil {
		t.Fatalf("getBccTournament returned error: %v", err)
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
