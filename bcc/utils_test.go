/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package bcc

import (
	"testing"
)

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
