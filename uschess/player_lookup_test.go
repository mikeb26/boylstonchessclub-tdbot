/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package uschess

import (
	"testing"
)

// Note: This test performs a live lookup against the USCF "thin3" endpoint.
// Ensure internet connectivity and endpoint availability.
func TestFetchPlayer(t *testing.T) {
	const memberID = "12689073"
	const expectedName = "Michael Brown"
	const expectedMinEventCount = 48
	const expectedFirstEventId = "199703255100"

	player, err := FetchPlayer(memberID)
	if err != nil {
		t.Fatalf("FetchPlayer(%q) returned error: %v", memberID, err)
	}

	if player.MemberID != memberID {
		t.Errorf("expected MemberID %q, got %q", memberID, player.MemberID)
	}
	if player.Name != expectedName {
		t.Errorf("expected name '%v' but got '%v'", expectedName, player.Name)
	}

	// Verify ratings or known defaults
	if player.RegRating == "" {
		t.Errorf("expected a regular rating or placeholder, got empty")
	}

	if len(player.Events) < expectedMinEventCount {
		t.Errorf("expected a minimum of %v events, got %v instead",
			expectedMinEventCount, len(player.Events))
	}

	firstEvent := &player.Events[len(player.Events)-1]
	if firstEvent.ID != expectedFirstEventId {
		t.Errorf("expected a first event id %v, got %v instead",
			expectedFirstEventId, firstEvent.ID)
	}

	t.Logf("Fetched player: %+v", player)
}
