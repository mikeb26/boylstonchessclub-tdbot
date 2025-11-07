/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package uschess

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
)

// Retrieve player information including crosstables from the specified number
// of most recent events. Return the result in string form.
func (client *Client) GetPlayerReport(ctx context.Context, memberID MemID,
	eventCount int) (string, error) {

	player, err := client.FetchPlayer(ctx, memberID)
	if err != nil {
		return "", err
	}

	const extraEvents = 2 // in case we filter out any blitz
	xTables, err := client.fetchRecentPlayerCrossTables(ctx, player,
		eventCount+extraEvents)
	if err != nil {
		return "", err
	}

	output := buildPlayerReport(player, xTables, eventCount)

	return output, nil
}

func buildPlayerReport(player *Player,
	xTables map[EventID][]CrossTable,
	eventCount int) string {

	// Sort events by date
	var eventIDs []EventID
	for id := range xTables {
		eventIDs = append(eventIDs, id)
	}
	sort.Slice(eventIDs, func(i, j int) bool {
		evI := getEventFromId(player.RecentEvents, eventIDs[i])
		evJ := getEventFromId(player.RecentEvents, eventIDs[j])
		return evI.EndDate.After(evJ.EndDate)
	})

	var eventSB strings.Builder
	outputCount := 0
	firstEvent := true
	for _, eventId := range eventIDs {
		if outputCount >= eventCount {
			break
		}
		event := getEventFromId(player.RecentEvents, eventId)

		firstTableInEvent := true
		// a player can have multiple cross tables from a single event
		// if they play in multiple sections. e.g. the player switched
		// sections during an event, or a td created a side games section
		for _, xt := range xTables[eventId] {
			if xt.RType != RatingTypeRegular {
				continue
			}
			if firstTableInEvent {
				eventSB.WriteString(fmt.Sprintf("%v - %v\n",
					event.EndDate.Format("2006-01-02"), event.Name))
				outputCount++
				firstTableInEvent = false
			}
			output, postRtg := BuildOneCrossTableOutput(&xt, true,
				player.MemberID)
			if firstEvent {
				player.RegRating = postRtg
				firstEvent = false
			}
			eventSB.WriteString(output)
		}
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Player ID:%v:\n", player.MemberID))
	sb.WriteString(fmt.Sprintf("Name: %v\n", player.Name))
	sb.WriteString(fmt.Sprintf("Live Rating(reg): %v\n", player.RegRating))
	sb.WriteString(fmt.Sprintf("Live Rating(quick): %v\n", player.QuickRating))
	sb.WriteString(fmt.Sprintf("Live Rating(blitz): %v\n", player.BlitzRating))
	sb.WriteString(fmt.Sprintf("Rated Events: %v\n", player.TotalEvents))
	if len(xTables) > 0 {
		sb.WriteString(fmt.Sprintf("Most Recent(%v) Classical Events:\n\n",
			eventCount))
	}
	sb.WriteString(eventSB.String())

	return sb.String()
}

func getEventFromId(events []Event, eventId EventID) Event {
	for _, event := range events {
		if event.ID == eventId {
			return event
		}
	}

	panic("BUG: invariant: eventId should be present in events slice")
}

func (client *Client) fetchRecentPlayerCrossTables(ctx context.Context,
	player *Player, eventCount int) (map[EventID][]CrossTable, error) {

	xTables := make(map[EventID][]CrossTable)
	var mu sync.Mutex
	g, _ := errgroup.WithContext(ctx)
	count := 0
	for _, ev := range player.RecentEvents {
		if count >= eventCount {
			break
		}
		count++
		g.Go(func() error {
			t, err := client.FetchCrossTables(ctx, ev.ID)
			if err != nil {
				return fmt.Errorf("error fetching cross tables for event %v: %w",
					ev.ID, err)
			}
			var xts []CrossTable
			for _, section := range t.CrossTables {
				for _, entry := range section.PlayerEntries {
					if entry.PlayerId == player.MemberID {
						xts = append(xts, *section)
						break
					}
				}
			}
			if len(xts) > 0 {
				mu.Lock()
				xTables[ev.ID] = xts
				mu.Unlock()
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return xTables, nil
}
