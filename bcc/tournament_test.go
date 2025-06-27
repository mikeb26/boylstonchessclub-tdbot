/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package bcc

import (
	"testing"
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
