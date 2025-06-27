/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package uschess

import (
	"strings"
	"unicode"
)

func normalizeName(s string) string {
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return ""
	}
	first := parts[0]
	last := first
	if len(parts) > 1 {
		last = parts[len(parts)-1]
	}
	firstLower := strings.ToLower(first)
	lastLower := strings.ToLower(last)
	fn := []rune(firstLower)
	ln := []rune(lastLower)
	firstTitle := string(unicode.ToUpper(fn[0])) + string(fn[1:])
	lastTitle := string(unicode.ToUpper(ln[0])) + string(ln[1:])
	if firstTitle == lastTitle {
		return firstTitle
	}
	return firstTitle + " " + lastTitle
}
