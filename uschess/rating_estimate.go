/* Copyright © 2026 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */

package uschess

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"golang.org/x/sync/errgroup"
)

// USCF Rating estimator based on:
//   1. https://new.uschess.org/sites/default/files/media/documents/the-us-chess-rating-system-revised-september-2020.pdf
//   2. https://new.uschess.org/news/change-us-chess-ratings-bonus-threshold-lowered-2025
//
// We support provisionally rated players* but not unrated players. Estimating
// unrated players' requires age as an input which isn't available in the
// uschess player api, and we do not want to burden the user with inputting
// age. Estimating new rating for establish players with unrated opponents would
// also require the full tournament results and a 2-pass algorithm. This too
// would be burdensome to require users to input, so currently the rating
// estimator does not support any unrated inputs.
//
// * with 1 edge case, which would be a provisionally rated player who has
// either won every game or lost every game they have played so far. It may
// be possible to address this with win/loss data from the player api, but
// for now we ignore this edge case.

func expectedScore(myRating float64, oppRating float64) float64 {
	// 1/(exp(ln(10)*((opp-my)/400))+1) == 1/(10^((opp-my)/400)+1)
	exp := math.Pow(10, (oppRating-myRating)/400.0)
	return 1.0 / (exp + 1.0)
}

// provisionalWinningExpectancy computes PWe (Section 4.1) used by the special
// rating formula for players with N<=8.
func provisionalWinningExpectancy(R float64, Ri float64) float64 {
	if R <= Ri-400.0 {
		return 0.0
	}
	if R >= Ri+400.0 {
		return 1.0
	}
	return 0.5 + (R-Ri)/800.0
}

// effectiveGames computes N₀ (Section 3)
func effectiveGames(myOldRating float64, priorGames int) float64 {
	var nStar float64
	if myOldRating <= 2355 {
		nStar = 50.0 / math.Sqrt(
			0.662+0.00000739*math.Pow(2569.0-myOldRating, 2),
		)
	} else {
		nStar = 50.0
	}

	if float64(priorGames) < nStar {
		return float64(priorGames)
	}
	return nStar
}

// KFactor computes K for the standard rating formula (Section 4.2).
//
// If dualRatedOTBR is true, then the OTBR dual-rated K adjustment for players >2200
// is applied.
func KFactor(myOldRating float64, N0 float64, m int, dualRatedOTBR bool) float64 {
	denom := N0 + float64(m)
	if denom <= 0 {
		return 0
	}
	if dualRatedOTBR && myOldRating > 2200.0 {
		if myOldRating >= 2500.0 {
			return 200.0 / denom
		}
		return 800.0 * (6.5 - 0.0025*myOldRating) / denom
	}
	return 800.0 / denom
}

// specialRatingEstimate computes the post-event rating via the "special" rating
// formula (Section 4.1). This applies to players with N<=8.
//
// Note: the PDF also specifies special handling for players with all prior wins or
// all prior losses; we do not currently track that history, so this estimate only
// covers the common case.
func specialRatingEstimate(
	myOldRating float64,
	N0 float64,
	score float64,
	opponentRatings []float64,
) float64 {
	R00 := myOldRating
	S0 := score + (N0 / 2.0)

	minR := R00
	maxR := R00
	for _, r := range opponentRatings {
		if r < minR {
			minR = r
		}
		if r > maxR {
			maxR = r
		}
	}

	// Choose bounds that ensure all PWe terms saturate.
	lo := minR - 1000.0
	hi := maxR + 1000.0

	f := func(R float64) float64 {
		sum := N0 * provisionalWinningExpectancy(R, R00)
		for _, r := range opponentRatings {
			sum += provisionalWinningExpectancy(R, r)
		}
		return sum - S0
	}

	const eps = 1e-7
	fl := f(lo)
	fh := f(hi)
	for i := 0; i < 10 && fl > 0; i++ {
		lo -= 1000
		fl = f(lo)
	}
	for i := 0; i < 10 && fh < 0; i++ {
		hi += 1000
		fh = f(hi)
	}

	for i := 0; i < 200 && hi-lo > 1e-9; i++ {
		mid := (lo + hi) / 2.0
		fm := f(mid)
		if math.Abs(fm) <= eps {
			lo, hi = mid, mid
			break
		}
		if fm < 0 {
			lo = mid
		} else {
			hi = mid
		}
	}

	out := (lo + hi) / 2.0
	if out < 100.0 {
		out = 100.0
	}
	if out > 2700.0 {
		out = 2700.0
	}
	return out
}

// getRatingEstimate computes the post-event rating
// using the standard US Chess rating formula with bonus.
func getRatingEstimate(
	myOldRating float64,
	priorGames int,
	score float64,
	opponentRatings []float64,
	dualRatedOTBR bool,
) (float64, error) {

	numGames := len(opponentRatings)
	if numGames == 0 {
		return myOldRating, nil
	}

	N0 := effectiveGames(myOldRating, priorGames)
	if priorGames <= 8 {
		return specialRatingEstimate(myOldRating, N0, score, opponentRatings), nil
	}

	// Step 2: Expected score
	expected := 0.0
	for _, oRating := range opponentRatings {
		expected += expectedScore(myOldRating, oRating)
	}

	// Step 3: K-factor
	K := KFactor(myOldRating, N0, numGames, dualRatedOTBR)

	// Step 4: Base rating change
	delta := K * (score - expected)

	bonus := calcBonus(numGames, delta)

	// Final rating
	return myOldRating + delta + bonus, nil
}

func calcBonus(numGames int, delta float64) float64 {
	// sep2020 pdf used 14.0, but jun2025 blog lowered it to 10.0
	const B = 10.0

	// Bonus calculation (Section 4.2)
	// Bonus applies if numGames >= 3 and unique opponents (assumed)
	bonus := 0.0
	if numGames >= 3 {
		m0 := numGames
		if m0 < 4 {
			m0 = 4
		}
		bonus = math.Max(0.0, delta-B*math.Sqrt(float64(m0)))
	}

	return bonus
}

func isUnrated(p *Player) bool {
	return strings.TrimSpace(p.RegRating) == "<unrated>"
}

func regRatingFloat(p *Player) (float64, error) {
	base, _, _, err := parseRatingWithProvisionalGames(p.RegRating)
	if err != nil {
		return 0, fmt.Errorf("parsing reg rating %q for member %v: %w", p.RegRating, p.MemberID, err)
	}
	return float64(base), nil
}

func (client *Client) fetchOneRatedPlayer(ctx context.Context,
	playerID MemID) (*Player, error) {

	p, err := client.FetchPlayer(ctx, playerID)
	if err != nil {
		return nil, err
	}
	if isUnrated(p) {
		return nil, fmt.Errorf("player %v is unrated", playerID)
	}

	return p, nil
}

// GetRatingEstimate retrieves the player's and opponents' Regular rating and
// number of games, then computes a post-event rating estimate.
//
// If the player or any opponent is unrated (RegRating=="<unrated>" OR missing/zero
// games), this returns an error.
//
// dualRatedOTBR is assumed false.
func (client *Client) GetRatingEstimate(
	ctx context.Context,
	playerID MemID,
	opponentIDs []MemID,
	score float64,
) (float64, error) {

	var player *Player
	opponents := make([]*Player, len(opponentIDs))

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		p, err := client.fetchOneRatedPlayer(ctx, playerID)
		if err != nil {
			return err
		}
		player = p
		return nil
	})

	for i := range opponentIDs {
		i := i
		g.Go(func() error {
			p, err := client.fetchOneRatedPlayer(ctx, opponentIDs[i])
			if err != nil {
				return err
			}
			opponents[i] = p
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return 0, err
	}

	myOld, err := regRatingFloat(player)
	if err != nil {
		return 0, err
	}

	opponentRatings := make([]float64, len(opponents))
	for i, opp := range opponents {
		r, err := regRatingFloat(opp)
		if err != nil {
			return 0, err
		}
		opponentRatings[i] = r
	}

	priorGames := estimateGameCount(player)
	return getRatingEstimate(myOld, priorGames, score, opponentRatings,
		false)
}

func estimateGameCount(player *Player) int {
	// If the player is provisionally rated, the rating string includes a suffix
	// like "1234P10" where the number after P is the number of games.
	_, provGames, hasProv, err := parseRatingWithProvisionalGames(player.RegRating)
	if err == nil && hasProv {
		return provGames
	}
	return player.TotalEvents * 4
}

func parseRatingWithProvisionalGames(s string) (base int, provGames int, hasProv bool, err error) {
	if i := strings.IndexByte(s, 'P'); i >= 0 {
		baseStr := strings.TrimSpace(s[:i])
		gamesStr := strings.TrimSpace(s[i+1:])
		if baseStr == "" || gamesStr == "" {
			return 0, 0, false, fmt.Errorf("invalid provisional rating format")
		}
		base, err = strconv.Atoi(baseStr)
		if err != nil {
			return 0, 0, false, err
		}
		provGames, err = strconv.Atoi(gamesStr)
		if err != nil {
			return 0, 0, false, err
		}
		return base, provGames, true, nil
	}

	base, err = strconv.Atoi(s)
	if err != nil {
		return 0, 0, false, err
	}
	return base, 0, false, nil
}
