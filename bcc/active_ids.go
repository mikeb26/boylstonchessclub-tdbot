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

	uschess "github.com/mikeb26/uschess-go"
)

//go:embed active_mem_ids.txt
var activePlayerMemIds string

//go:embed active_tids.txt
var activePlayerTIds string

// ActivePlayerMemIds returns USCF member IDs of individuals active since
// 10/11/2024 (2024 October Friday Night Blitz event id 1200). This list is
// up to date as of 7/2/2025
func ActivePlayerMemIds() []uschess.MemberID {
	return stringToIDSlice[uschess.MemberID]("memid", activePlayerMemIds)
}

// ActivePlayerTIds returns USCF event IDs for the 3 most recent tournaments
// per active player.
func ActivePlayerTIds() []uschess.EventID {
	return stringToIDSlice[uschess.EventID]("tid", activePlayerTIds)
}

// stringToIDSlice parses newline-delimited identifiers in s.
func stringToIDSlice[T ~string](name string, s string) []T {
	var out []T
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		id := strings.TrimSpace(scanner.Text())
		_, err := strconv.Atoi(id)
		if err != nil {
			panic(fmt.Sprintf("failed to parse %v: %v", name, err))
		}
		out = append(out, T(id))
	}
	if err := scanner.Err(); err != nil {
		panic(fmt.Sprintf("failed to parse %v: %v", name, err))
	}
	return out
}
