/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package bcc

import (
	"testing"
)

func TestGetEventDetail(t *testing.T) {
	detail, err := GetEventDetail(1312)
	if err != nil {
		t.Fatalf("GetEventDetail returned error: %v", err)
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
