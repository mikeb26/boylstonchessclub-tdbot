/* Copyright © 2026 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */

// Package uscfutils contains presentation helpers built on uschess-go's API
// models. It intentionally does not duplicate US Chess API client models.
package uscfutils

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mikeb26/boylstonchessclub-tdbot/internal"
	"github.com/mikeb26/boylstonchessclub-tdbot/internal/httpcache"
	uschess "github.com/mikeb26/uschess-go"
	"golang.org/x/sync/errgroup"
)

// NewClient creates a US Chess API client using the application's S3-backed
// HTTP cache.
func NewClient(ctx context.Context) (*uschess.ClientWithResponses, error) {
	httpClient := httpcache.NewCachedHttpClient(ctx, 24*time.Hour)
	return uschess.NewDefaultClient(
		uschess.WithHTTPClient(httpClient),
		uschess.WithUserAgent(internal.UserAgent),
	)
}

// BuildCrossTableOutput formats one section's standings as a monospace table.
// A nonempty filterPlayerID includes that player and their opponents only.
func BuildCrossTableOutput(section uschess.MinimalSection,
	standings uschess.StandingsOneSection, includeSectionHeader bool,
	filterPlayerID uschess.MemberID) (string, string) {

	var includeSet map[int32]bool
	var filteredOrdinal int32
	if filterPlayerID != "" {
		includeSet = make(map[int32]bool)
		for _, entry := range standings {
			if entry.MemberId != filterPlayerID {
				continue
			}
			filteredOrdinal = entry.Ordinal
			includeSet[entry.Ordinal] = true
			for _, outcome := range entry.RoundOutcomes {
				if outcome.OpponentOrdinal > 0 {
					includeSet[outcome.OpponentOrdinal] = true
				}
			}
			break
		}
	}

	var sb strings.Builder
	if includeSectionHeader {
		sb.WriteString(fmt.Sprintf("Section %s\n", section.Name))
	}

	numRounds := 0
	for _, entry := range standings {
		if len(entry.RoundOutcomes) > numRounds {
			numRounds = len(entry.RoundOutcomes)
		}
	}
	headers := []string{"No", "Name", "Rating", "Pts"}
	for round := 1; round <= numRounds; round++ {
		headers = append(headers, fmt.Sprintf("R%d", round))
	}

	ratingPost := "<unknown>"
	forfeitFound := false
	rows := make([][]string, 0, len(standings))
	for _, entry := range standings {
		if includeSet != nil && !includeSet[entry.Ordinal] {
			continue
		}

		name := internal.NormalizeName(entry.FirstName + " " + entry.LastName)
		preRating, postRating := regularRating(entry.Ratings)
		if entry.Ordinal == filteredOrdinal {
			name = fmt.Sprintf("**%s**", name)
			ratingPost = postRating
		}

		row := []string{
			fmt.Sprintf("%d.", entry.Ordinal),
			name,
			fmt.Sprintf("%s->%s", preRating, postRating),
			internal.ScoreToString(float64(entry.Score)),
		}
		for _, outcome := range entry.RoundOutcomes {
			cell, isForfeit := formatOutcome(outcome)
			forfeitFound = forfeitFound || isForfeit
			row = append(row, cell)
		}
		rows = append(rows, row)
	}

	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = len(header)
	}
	for _, row := range rows {
		for i, cell := range row {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	var format strings.Builder
	for _, width := range widths {
		format.WriteString(fmt.Sprintf("%%-%ds  ", width))
	}
	formatString := strings.TrimRight(format.String(), " ") + "\n"
	sb.WriteString(fmt.Sprintf(formatString, stringsToAny(headers)...))
	for _, row := range rows {
		sb.WriteString(fmt.Sprintf(formatString, stringsToAny(row)...))
	}
	if forfeitFound {
		sb.WriteString("* indicates game was decided by forfeit\n")
	}
	sb.WriteString("\n")

	return sb.String(), ratingPost
}

// BuildPlayerReport retrieves and formats a player's current rating and recent
// Regular-rated event crosstables.
func BuildPlayerReport(ctx context.Context, client *uschess.ClientWithResponses,
	memberID uschess.MemberID, eventCount int) (string, error) {

	opts := &uschess.GetPlayerOptions{
		IncludeSupplements: true,
		IncludeEvents:      true,
		IncludeLiveRatings: true,
	}
	player, err := client.GetPlayer(ctx, memberID, opts)
	if err != nil {
		return "", err
	}

	liveRating, err := playerRegularLiveRating(player)
	if err != nil {
		return "", err
	}
	supplementRating, supplementDate := playerRegularSupplement(player)

	// MemberEvents are ordered most recent first. Only retrieve the events the
	// caller requested for the report; older events may no longer be available
	// from the US Chess API.
	events := player.MemberEvents
	if len(events) > eventCount {
		events = events[:eventCount]
	}
	tournaments := make([]*uschess.Tournament, len(events))
	group, groupCtx := errgroup.WithContext(ctx)
	for index, event := range events {
		index, event := index, event
		group.Go(func() error {
			tournament, err := client.GetTournament(groupCtx, event.Id)
			if err != nil {
				return fmt.Errorf("fetching crosstables for event %s: %w", event.Id, err)
			}
			tournaments[index] = tournament
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return "", err
	}

	var eventOutput strings.Builder
	outputCount := 0
	firstEvent := true
	for _, tournament := range tournaments {
		if outputCount >= eventCount {
			break
		}

		wroteEvent := false
		for index, standings := range tournament.SectionStandings {
			if !sectionIsRegular(standings) || !sectionContainsPlayer(standings, memberID) {
				continue
			}
			if !wroteEvent {
				eventOutput.WriteString(fmt.Sprintf("%s - %s\n",
					tournament.EndDate.Time.Format("2006-01-02"), tournament.Name))
				outputCount++
				wroteEvent = true
			}
			section := tournament.Sections[index]
			output, postRating := BuildCrossTableOutput(section, standings, true, memberID)
			if firstEvent {
				liveRating = postRating
				firstEvent = false
			}
			eventOutput.WriteString(output)
		}
	}

	name := internal.NormalizeName(player.FirstName + " " + player.LastName)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Player: %s\n", name))
	sb.WriteString(fmt.Sprintf("USCF ID: %s\n", player.Id))
	sb.WriteString(fmt.Sprintf("Rating:\n\tLive: %s\n", liveRating))
	sb.WriteString(fmt.Sprintf("\t%s Supplement: %s\n", supplementDate.Format("Jan"), supplementRating))
	sb.WriteString(fmt.Sprintf("Rated Events: %d\n", len(player.MemberEvents)))
	if eventOutput.Len() > 0 {
		sb.WriteString(fmt.Sprintf("Most Recent(%d) Classical Events:\n\n", eventCount))
	}
	sb.WriteString(eventOutput.String())
	return sb.String(), nil
}

func sectionContainsPlayer(standings uschess.StandingsOneSection, memberID uschess.MemberID) bool {
	for _, entry := range standings {
		if entry.MemberId == memberID {
			return true
		}
	}
	return false
}

func sectionIsRegular(standings uschess.StandingsOneSection) bool {
	foundRating := false
	for _, entry := range standings {
		for _, rating := range entry.Ratings {
			foundRating = true
			if rating.RatingType == uschess.RatingTypeR {
				return true
			}
		}
	}
	// Match the legacy formatter's behavior: in the absence of rating data,
	// preserve the default assumption that the section is Regular-rated.
	return !foundRating
}

func regularRating(ratings []uschess.RatingRecord) (string, string) {
	for _, rating := range ratings {
		if rating.RatingType == uschess.RatingTypeR {
			return formatRating(rating.PreRating, 0),
				formatRating(rating.PostRating, rating.PostProvisionalGameCount)
		}
	}
	if len(ratings) == 0 {
		return "", ""
	}
	rating := ratings[0]
	return formatRating(rating.PreRating, 0),
		formatRating(rating.PostRating, rating.PostProvisionalGameCount)
}

func formatRating(rating, provisionalGames int32) string {
	if rating == 0 {
		return ""
	}
	if provisionalGames > 0 {
		return fmt.Sprintf("%dP%d", rating, provisionalGames)
	}
	return fmt.Sprintf("%d", rating)
}

func playerRegularLiveRating(player *uschess.Player) (string, error) {
	ratings, err := player.LiveRatings()
	if err != nil {
		return "", err
	}
	for _, rating := range ratings {
		if rating.RatingType == uschess.RatingTypeR {
			return formatRating(rating.Rating, rating.ProvisionalGameCount), nil
		}
	}
	return "<unrated>", nil
}

func playerRegularSupplement(player *uschess.Player) (string, time.Time) {
	if len(player.RatingSupplements) == 0 {
		return "<unrated>", time.Time{}
	}
	supplement := player.RatingSupplements[0]
	for _, rating := range supplement.Ratings {
		if rating.RatingType == uschess.RatingTypeR {
			return formatRating(rating.Rating, rating.ProvisionalGameCount), supplement.RatingSupplementDate.Time
		}
	}
	return "<unrated>", supplement.RatingSupplementDate.Time
}

func formatOutcome(outcome uschess.StandingsRound) (string, bool) {
	color := ""
	switch strings.ToLower(string(outcome.Color)) {
	case "white":
		color = "(w)"
	case "black":
		color = "(b)"
	}

	switch outcome.Outcome {
	case uschess.PlayerOutcomeWin, uschess.PlayerOutcomeWinAsym:
		return fmt.Sprintf("W%d%s", outcome.OpponentOrdinal, color), false
	case uschess.PlayerOutcomeLoss, uschess.PlayerOutcomeLossAsym:
		return fmt.Sprintf("L%d%s", outcome.OpponentOrdinal, color), false
	case uschess.PlayerOutcomeDraw, uschess.PlayerOutcomeDrawAsym:
		return fmt.Sprintf("D%d%s", outcome.OpponentOrdinal, color), false
	case uschess.PlayerOutcomeWinForfeit:
		return "W*", true
	case uschess.PlayerOutcomeForfeit:
		return "L*", true
	case uschess.PlayerOutcomeByeFull:
		return "BYE(1)", false
	case uschess.PlayerOutcomeByeHalf:
		return "BYE(½)", false
	case uschess.PlayerOutcomeUnpaired:
		return "BYE(0)", false
	default:
		return "?", false
	}
}

func stringsToAny(values []string) []any {
	result := make([]any, len(values))
	for i, value := range values {
		result[i] = value
	}
	return result
}
