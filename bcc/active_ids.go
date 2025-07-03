/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package bcc

import (
	"bufio"
	_ "embed"
	"fmt"
	"strconv"
	"strings"

	"github.com/mikeb26/boylstonchessclub-tdbot/uschess"
)

//go:embed active_mem_ids.txt
var activePlayerMemIds string

//go:embed active_tids.txt
var activePlayerTIds string

// ActivePlayerMemIds returns USCF member IDs of individuals active since
// 10/11/2024 (2024 October Friday Night Blitz event id 1200). This list is
// up to date as of 7/2/2025
func ActivePlayerMemIds() []uschess.MemID {
	return stringToIntSlice[uschess.MemID]("memid", activePlayerMemIds)
}

// ActivePlayerTIds returns USCF event IDs for the 3 most recent tournaments
// per active player.
func ActivePlayerTIds() []uschess.EventID {
	return stringToIntSlice[uschess.EventID]("tid", activePlayerTIds)
}

// stringToIntSlice parses newline-delimited integers in s and returns a slice
// of type T (underlying int).
func stringToIntSlice[T ~int](name string, s string) []T {
	var out []T
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		n, err := strconv.Atoi(scanner.Text())
		if err != nil {
			panic(fmt.Sprintf("failed to parse %v: %v", name, err))
		}
		out = append(out, T(n))
	}
	if err := scanner.Err(); err != nil {
		panic(fmt.Sprintf("failed to parse %v: %v", name, err))
	}
	return out
}
