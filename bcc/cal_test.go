/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package bcc

import (
	"testing"
)

func TestGetEvents(t *testing.T) {
	events, err := GetEvents()
	if err != nil {
		t.Fatalf("GetEvents returned error: %v", err)
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
