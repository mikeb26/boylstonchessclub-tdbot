/* Copyright © 2026 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package uschess

import (
	"strings"
	"testing"
	"time"
)

func TestBuildPlayerReportIncludesLatestSupplement(t *testing.T) {
	player := &Player{
		MemberID:    12689073,
		Name:        "Michael Brown",
		RegRating:   "1679",
		QuickRating: "1654P11",
		BlitzRating: "<unrated>",
		RegSupplement: RatingSupplement{
			Rating: "1709",
			Date:   time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		},
		QuickSupplement: RatingSupplement{
			Rating: "1654P11",
			Date:   time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		},
		BlitzSupplement: RatingSupplement{
			Rating: "<unrated>",
			Date:   time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		},
		TotalEvents: 48,
	}

	report := buildPlayerReport(player, nil, 0)

	for _, want := range []string{
		"Rating:\n\tLive: 1679\n",
		"\tApr Supplement: 1709\n",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("report missing %q:\n%s", want, report)
		}
	}
}
