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
	if e.EndDate.IsZero() {
		t.Error("expected EndDate to be non-zero")
	}
	if e.DayOfWeek == "" {
		t.Error("expected DayOfWeek to be non-empty")
	}
	if e.DateDisplay == "" {
		t.Error("expected DateDisplay to be non-empty")
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
