/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package bcc

import (
	_ "embed"
	"fmt"
	"strings"

	"bufio"
	"strconv"
)

//go:embed active_mem_ids.txt
var activePlayerMemIds string

//go:embed active_tids.txt
var activePlayerTIds string

// as of 7/2/2025 the following is the set of USCF member ids of individuals who
// have either signed up for or played in a BCC event since 2024-10-11
// (2024 October Friday Night Blitz event id 1200)
func ActivePlayerMemIds() []int {
	return stringToIntSlice("memid", activePlayerMemIds)
}

// as of 7/2/2025 the following a set of USCF tournament ids containing
// the three most recent tournaments which for every member of
// ActivePlayerMemIds() has played in.
func ActivePlayerTIds() []int {
	return stringToIntSlice("tid", activePlayerTIds)
}

func stringToIntSlice(n string, s string) []int {
	var out []int
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		n, err := strconv.Atoi(scanner.Text())
		if err != nil {
			panic(fmt.Sprintf("failed to parse %v: %v", n, err))
		}
		out = append(out, n)
	}
	if scanner.Err() != nil {
		panic(fmt.Sprintf("failed to parse %v: %v", n, scanner.Err()))
	}

	return out
}
