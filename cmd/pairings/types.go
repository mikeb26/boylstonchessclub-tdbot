/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package main

type RatingType int

const (
	RatingTypeReported RatingType = iota
	RatingTypeActual
)

const RatingUnrated = 0

type ByeReason int

const (
	ByeReasonNone ByeReason = iota
	ByeReasonRequested
	ByeReasonOdd
)

type Color int

const (
	White Color = iota
	Black
)

type Player struct {
	UscfID  string
	Name    string
	Rating  int
	RType   RatingType
	BReason ByeReason
}

type Pairing [2]Player

type Section struct {
	Name     string
	Pairings []Pairing
	Byes     []Player
}
