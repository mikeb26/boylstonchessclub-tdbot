/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package internal

import (
	"fmt"
	"math"
	"time"

	"github.com/araddon/dateparse"
)

// ParseDateOrZero returns a parsed time or zero if input is empty or "null".
func ParseDateOrZero(s string) (time.Time, error) {
	if s == "" || s == "null" {
		return time.Time{}, nil
	}
	return dateparse.ParseAny(s)
}

// ScoreToString returns a score as an integer or integer plus ½ if applicable.
// Assumes score is always an integer or integer plus 0.5.
func ScoreToString(score float64) string {
	intPart, frac := math.Modf(score)
	// Integer score
	if frac == 0 {
		return fmt.Sprintf("%d", int(intPart))
	}
	// Half score
	if frac == 0.5 {
		if int(intPart) == 0 {
			return "½"
		} else {
			return fmt.Sprintf("%d½", int(intPart))
		}
	}
	// Fallback to one decimal place
	return fmt.Sprintf("%.1f", score)
}
