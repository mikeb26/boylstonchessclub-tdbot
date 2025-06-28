/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package uschess

import (
	"strings"
	"testing"
)

func TestFetchCrossTables202506242722(t *testing.T) {
	cts, err := FetchCrossTables(202506242722)
	if err != nil {
		t.Fatalf("FetchCrossTables error: %v", err)
	}
	// Verify number of sections
	if len(cts) != 5 {
		t.Fatalf("expected 5 sections, got %d", len(cts))
	}
	// Locate OPEN section
	var openCT *CrossTable
	for i := range cts {
		if strings.Contains(strings.ToUpper(cts[i].SectionName), "OPEN") {
			openCT = cts[i]
			break
		}
	}
	if openCT == nil {
		t.Fatalf("OPEN section not found among sections: %+v", cts)
	}
	// Locate Rufus' entry
	var entry *CrossTableEntry
	for i := range openCT.PlayerEntries {
		if openCT.PlayerEntries[i].PlayerName == "Rufus Behr" {
			entry = &openCT.PlayerEntries[i]
			break
		}
	}
	if entry == nil {
		t.Fatalf("RUFUS BEHR not found in OPEN section entries: %+v", openCT.PlayerEntries)
	}
	// Verify player attributes
	if entry.PlayerId != 16438266 {
		t.Errorf("expected PlayerId 16438266, got %d", entry.PlayerId)
	}
	if entry.PlayerRatingPre != "1735" {
		t.Errorf("expected PlayerRatingPre 1735, got %v", entry.PlayerRatingPre)
	}
	if entry.PlayerRatingPost != "1751" {
		t.Errorf("expected PlayerRatingPost 1751, got %v", entry.PlayerRatingPost)
	}
	if entry.TotalPoints != 2.0 {
		t.Errorf("expected TotalPoints 2.0, got %f", entry.TotalPoints)
	}
	// Verify round results
	if len(entry.Results) < 4 {
		t.Errorf("expected at least 4 rounds of results, got %d", len(entry.Results))
	} else {
		// Round 1: bye
		r := entry.Results[0]
		if r.Outcome != ResultFullBye {
			t.Errorf("round 1: expected full bye, got %+v", r)
		}
		// Round 2: loss to 8 with black
		r = entry.Results[1]
		if r.Outcome != ResultLoss || r.OpponentPairNum != 8 || r.Color != "black" {
			t.Errorf("round 2: expected loss to 8 with black, got %+v", r)
		}
		// Round 3: win against 11 with white
		r = entry.Results[2]
		if r.Outcome != ResultWin || r.OpponentPairNum != 11 || r.Color != "white" {
			t.Errorf("round 3: expected win against 11 with white, got %+v", r)
		}
		// Round 4: loss to 7 with black
		r = entry.Results[3]
		if r.Outcome != ResultLoss || r.OpponentPairNum != 7 || r.Color != "black" {
			t.Errorf("round 4: expected loss to 7 with black, got %+v", r)
		}
	}
}
