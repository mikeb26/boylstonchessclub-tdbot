/* Copyright © 2026 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package bcc

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	uschess "github.com/mikeb26/uschess-go"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func testUSChessPlayer(memberID uschess.MemberID, firstName, lastName, rating string, date time.Time) *uschess.Player {
	supplementRating := int32(0)
	provisionalGames := int32(0)
	if rating == "1709" {
		supplementRating = 1709
	}
	if rating == "1654P11" {
		supplementRating = 1654
		provisionalGames = 11
	}
	return &uschess.Player{
		MemberDetail: uschess.MemberDetail{
			Id:        memberID,
			FirstName: firstName,
			LastName:  lastName,
		},
		RatingSupplements: []uschess.RatingSupplement{{
			RatingSupplementDate: openapi_types.Date{Time: date},
			Ratings: []uschess.RatingSupplementSystem{{
				RatingType:           uschess.RatingTypeR,
				Rating:               supplementRating,
				ProvisionalGameCount: provisionalGames,
			}},
		}},
	}
}

func TestCorrectRound1PairingEntriesWithLookupUsesUSChessSupplement(t *testing.T) {
	entries := []Entry{{
		FirstName: "Bccfirst", LastName: "Bcclast", UscfID: 12689073,
		PrimaryRating: "1200", SectionName: "Open",
	}}
	supplementDate := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	corrected := correctRound1PairingEntriesWithLookup(context.Background(), entries,
		func(_ context.Context, memberID uschess.MemberID) (*uschess.Player, error) {
			if memberID != "12689073" {
				t.Fatalf("looked up memberID %v; want 12689073", memberID)
			}
			return testUSChessPlayer(memberID, "Michael", "Brown", "1709", supplementDate), nil
		})

	if got, want := corrected[0].FirstName, "Michael"; got != want {
		t.Fatalf("FirstName = %q; want %q", got, want)
	}
	if got, want := corrected[0].LastName, "Brown"; got != want {
		t.Fatalf("LastName = %q; want %q", got, want)
	}
	if got, want := corrected[0].PrimaryRating, "1709"; got != want {
		t.Fatalf("PrimaryRating = %q; want %q", got, want)
	}
	if got, want := corrected[0].PrimaryRatingType, "regular"; got != want {
		t.Fatalf("PrimaryRatingType = %q; want %q", got, want)
	}
	if got, want := corrected[0].PrimaryRatingDate, "2026-04-01"; got != want {
		t.Fatalf("PrimaryRatingDate = %q; want %q", got, want)
	}
}

func TestCorrectRound1PairingEntriesWithLookupFallsBack(t *testing.T) {
	base := []Entry{
		{FirstName: "No", LastName: "ID", UscfID: 0, PrimaryRating: "1300"},
		{FirstName: "Bad", LastName: "Lookup", UscfID: 1, PrimaryRating: "1400"},
		{FirstName: "Unrated", LastName: "USChess", UscfID: 2, PrimaryRating: "1500"},
	}
	corrected := correctRound1PairingEntriesWithLookup(context.Background(), base,
		func(_ context.Context, memberID uschess.MemberID) (*uschess.Player, error) {
			switch memberID {
			case "1":
				return nil, errors.New("invalid member")
			case "2":
				return testUSChessPlayer(memberID, "US", "Chess", "<unrated>", time.Time{}), nil
			default:
				t.Fatalf("unexpected lookup for memberID %v", memberID)
				return nil, nil
			}
		})
	if !reflect.DeepEqual(corrected, base) {
		t.Fatalf("corrected entries changed despite fallback cases:\n got: %#v\nwant: %#v", corrected, base)
	}
}

func TestCorrectRound1PairingEntriesWithLookupAllowsProvisionalSupplement(t *testing.T) {
	entries := []Entry{{FirstName: "Old", LastName: "Name", UscfID: 3, PrimaryRating: "1100"}}
	corrected := correctRound1PairingEntriesWithLookup(context.Background(), entries,
		func(_ context.Context, memberID uschess.MemberID) (*uschess.Player, error) {
			return testUSChessPlayer(memberID, "New", "Name", "1654P11", time.Time{}), nil
		})
	if got, want := corrected[0].PrimaryRating, "1654P11"; got != want {
		t.Fatalf("PrimaryRating = %q; want %q", got, want)
	}
	if got, want := strRatingToInt(corrected[0].PrimaryRating), 1654; got != want {
		t.Fatalf("strRatingToInt(%q) = %d; want %d", corrected[0].PrimaryRating, got, want)
	}
}
