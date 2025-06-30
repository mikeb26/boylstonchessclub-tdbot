/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package internal

import (
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
