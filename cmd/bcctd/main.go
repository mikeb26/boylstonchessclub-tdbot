/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/mikeb26/boylstonchessclub-tdbot/bcc"
	"github.com/mikeb26/boylstonchessclub-tdbot/internal"
	"github.com/mikeb26/boylstonchessclub-tdbot/uschess"
)

//go:embed help.txt
var helpText string

// cmdHandler defines the signature for command handler functions.
type cmdHandler func(args []string)

// commands maps command names to their respective handler functions.
var commands = map[string]cmdHandler{
	"help":       handleHelp,
	"cal":        handleCal,
	"event":      handleEvent,
	"pairings":   handlePairings,
	"standings":  handleStandings,
	"crosstable": handleCrossTable,
	"history":    handleHistory,
	"player":     handlePlayer,
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	cmd := os.Args[1]
	if handler, ok := commands[cmd]; ok {
		handler(os.Args[2:])
	} else {
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Printf("%v", helpText)
}

func handleHelp(args []string) {
	usage()
}

func handleCal(args []string) {
	fs := flag.NewFlagSet("cal", flag.ExitOnError)
	days := fs.Int("days", 14, "Number of days to retrieve (1-60)")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	// enforce bounds
	if *days < -60 {
		*days = -60
	} else if *days > 60 {
		*days = 60
	}

	var start time.Time
	now := time.Now()
	end := now.AddDate(0, 0, *days)

	if now.After(end) {
		start = end
		end = now
	} else {
		start = now
	}
	// Fetch events from BCC API
	events, err := bcc.GetEvents()
	if err != nil {
		log.Fatalf("Error fetching events: %v", err)
	}
	// Filter and group events by date
	eventsByDate := make(map[string][]bcc.Event)
	for _, ev := range events {
		if ev.Date.Before(start) || ev.Date.After(end) {
			continue
		}
		key := ev.Date.Format("2006-01-02")
		eventsByDate[key] = append(eventsByDate[key], ev)
	}

	if len(eventsByDate) == 0 {
		fmt.Printf("No events found in the next %d days.\n", *days)
		return
	}
	// Build sorted output
	var dates []string
	for d := range eventsByDate {
		dates = append(dates, d)
	}
	if start == now {
		sort.Strings(dates)
	} else {
		sort.Slice(dates, func(i, j int) bool {
			return dates[j] < dates[i]
		})
	}
	for _, d := range dates {
		fmt.Println(d)
		for _, ev := range eventsByDate[d] {
			fmt.Printf("  - %s (EventID:%d)\n", ev.Title, ev.EventID)
		}
	}
	fmt.Printf("\nRun '%s event --eventid <EventID>' to get details on a specific event\n",
		os.Args[0])
}

func handleEvent(args []string) {
	fs := flag.NewFlagSet("event", flag.ExitOnError)
	eventID := fs.Int("eventid", 0, "Event ID to fetch details for")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if *eventID <= 0 {
		fmt.Fprintln(os.Stderr, "Please provide a valid --eventid ID.")
		fs.Usage()
		os.Exit(1)
	}
	detail, err := bcc.GetEventDetail(int64(*eventID))
	if err != nil {
		log.Fatalf("Error fetching event %d: %v", *eventID, err)
	}
	// Print event details
	fmt.Printf("EventID: %d\n", detail.EventID)
	fmt.Printf("Date: %s\n", detail.DateDisplay)
	if detail.EventFormat != "" {
		fmt.Printf("Format: %s\n", detail.EventFormat)
	}
	if detail.TimeControl != "" {
		fmt.Printf("Time Control: %s\n", detail.TimeControl)
	}
	if detail.SectionDisplay != "" {
		fmt.Printf("Sections: %s\n", detail.SectionDisplay)
	}
	fmt.Printf("Entry Fee: %s\n", detail.EntryFeeSummary)
	if detail.PrizeSummary != "" {
		fmt.Printf("Prizes: %s\n", detail.PrizeSummary)
	}
	if detail.RegistrationTime != "" {
		fmt.Printf("Registration Time: %s\n", detail.RegistrationTime)
	}
	fmt.Printf("Round Times: %s\n", detail.RoundTimes)
	fmt.Printf("Description: %s\n", detail.Description)
	fmt.Printf("URL: https://boylstonchess.org/events/%d\n", detail.EventID)
}

func handlePairings(args []string) {
	fs := flag.NewFlagSet("pairings", flag.ExitOnError)
	eventID := fs.Int("eventid", 0, "Event ID to fetch pairings for")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if *eventID <= 0 {
		fmt.Fprintln(os.Stderr, "Please provide a valid --eventid ID.")
		fs.Usage()
		os.Exit(1)
	}
	tourney, err := bcc.GetTournament(int64(*eventID))
	if err != nil {
		log.Fatalf("Error fetching pairings for event %d: %v", *eventID, err)
	}
	output := bcc.BuildPairingsOutput(tourney)
	fmt.Print(output)
}

func handleStandings(args []string) {
	fs := flag.NewFlagSet("standings", flag.ExitOnError)
	eventID := fs.Int("eventid", 0, "Event ID to fetch standings for")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if *eventID <= 0 {
		fmt.Fprintln(os.Stderr, "Please provide a valid --eventid ID.")
		fs.Usage()
		os.Exit(1)
	}
	tourney, err := bcc.GetTournament(int64(*eventID))
	if err != nil {
		log.Fatalf("Error fetching standings for event %d: %v", *eventID, err)
	}
	output := bcc.BuildStandingsOutput(tourney)
	fmt.Print(output)
}

func handleCrossTable(args []string) {
	fs := flag.NewFlagSet("crosstable", flag.ExitOnError)
	tid := fs.Int("uscftid", 0, "USCF Tournament ID")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if *tid <= 0 {
		fmt.Fprintln(os.Stderr, "Please provide a valid --uscftid ID.")
		fs.Usage()
		os.Exit(1)
	}

	xTables, err := uschess.FetchCrossTables(*tid)
	if err != nil {
		log.Fatalf("Error fetching cross tables %d: %v", *tid, err)
	}

	for _, xt := range xTables {
		output := buildOneCrossTableOutput(xt, len(xTables) > 1, 0)
		fmt.Printf(output)
	}
}

func buildOneCrossTableOutput(xt *uschess.CrossTable,
	includeSectionHeader bool, filterPlayerID int) string {

	// If filtering, determine which pair numbers to include (player + opponents)
	var includeSet map[int]bool
	var filteredPlayerPairNum int
	if filterPlayerID != 0 {
		includeSet = make(map[int]bool)
		// find player entry
		for _, e := range xt.PlayerEntries {
			if e.PlayerId == filterPlayerID {
				filteredPlayerPairNum = e.PairNum
				includeSet[e.PairNum] = true
				// record opponents
				for _, res := range e.Results {
					if res.OpponentPairNum > 0 {
						includeSet[res.OpponentPairNum] = true
					}
				}
				break
			}
		}
	}

	var sb strings.Builder

	if includeSectionHeader {
		sb.WriteString(fmt.Sprintf("%v\n", xt.SectionName))
	}

	// Build headers
	numRounds := xt.NumRounds
	headers := []string{"No", "Name", "Rating", "Pts"}
	for i := 1; i <= numRounds; i++ {
		headers = append(headers, fmt.Sprintf("R%d", i))
	}

	// Build rows
	forfeitFound := false
	var rows [][]string
	for _, e := range xt.PlayerEntries {
		playerName := e.PlayerName

		// apply filter
		if includeSet != nil {
			if !includeSet[e.PairNum] {
				continue
			}
			if filteredPlayerPairNum == e.PairNum {
				playerName = fmt.Sprintf("++%v++", playerName)
			}
		}

		row := []string{
			fmt.Sprintf("%d.", e.PairNum),
			playerName,
			fmt.Sprintf("%v->%v", e.PlayerRatingPre, e.PlayerRatingPost),
			fmt.Sprintf("%.1f", e.TotalPoints),
		}
		for _, res := range e.Results {
			var cell string
			switch res.Outcome {
			case uschess.ResultWin:
				cell = fmt.Sprintf("W%d", res.OpponentPairNum)
				cell += fmt.Sprintf("(%c)", res.Color[0])
			case uschess.ResultWinByForfeit:
				forfeitFound = true
				cell = fmt.Sprintf("W*")
			case uschess.ResultLoss:
				cell = fmt.Sprintf("L%d", res.OpponentPairNum)
				cell += fmt.Sprintf("(%c)", res.Color[0])
			case uschess.ResultLossByForfeit:
				forfeitFound = true
				cell = fmt.Sprintf("L*")
			case uschess.ResultDraw:
				cell = fmt.Sprintf("D%d", res.OpponentPairNum)
				cell += fmt.Sprintf("(%c)", res.Color[0])
			case uschess.ResultFullBye:
				cell = "BYE(1.0)"
			case uschess.ResultHalfBye:
				cell = "BYE(0.5)"
			case uschess.ResultUnplayedGame:
				cell = "BYE(0.0)"
			default:
				cell = "?"
			}
			row = append(row, cell)
		}
		rows = append(rows, row)
	}

	// Compute column widths
	colWidths := make([]int, len(headers))
	for i, h := range headers {
		colWidths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	// Build format string
	var fmtStrBuilder strings.Builder
	for _, w := range colWidths {
		fmtStrBuilder.WriteString(fmt.Sprintf("%%-%ds  ", w))
	}
	fmtStr := strings.TrimRight(fmtStrBuilder.String(), " ") + "\n"

	// Write header
	sb.WriteString(fmt.Sprintf(fmtStr, toAnySlice(headers)...))
	// Write rows
	for _, row := range rows {
		sb.WriteString(fmt.Sprintf(fmtStr, toAnySlice(row)...))
	}
	if forfeitFound {
		sb.WriteString("* indicates game was decided by forfeit\n")
	}
	sb.WriteString("\n")

	return sb.String()
}

// toAnySlice converts a slice of any type to a slice of any (interface{}).
func toAnySlice[T any](slice []T) []any {
	result := make([]any, len(slice))
	for i, v := range slice {
		result[i] = v
	}
	return result
}

func handleHistory(args []string) {
	fs := flag.NewFlagSet("history", flag.ExitOnError)
	days := fs.Int("days", 14, "Number of days to retrieve (1-60)")
	aid := fs.String("uscfaid", internal.BccUSCFAffiliateID, "USCF Affiliate ID")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if *aid == "" {
		fmt.Fprintln(os.Stderr, "Please provide a valid --uscfaid ID.")
		fs.Usage()
		os.Exit(1)
	}

	// enforce bounds
	if *days <= 0 {
		*days = 14
	} else if *days > 60 {
		*days = 60
	}

	now := time.Now()
	end := now.AddDate(0, 0, -*days)

	events, err := uschess.GetAffiliateEvents(*aid)
	if err != nil {
		log.Fatalf("Error fetching events for aid:%v: %v", *aid, err)
	}

	// Filter and group events by date
	eventsByDate := make(map[string][]uschess.Event)
	for _, ev := range events {
		if ev.EndDate.Before(end) {
			continue
		}
		key := ev.EndDate.Format("2006-01-02")

		eventsByDate[key] = append(eventsByDate[key], ev)
	}

	if len(eventsByDate) == 0 {
		fmt.Printf("No recent events found for aid:%v\n", *aid)
		return
	}
	// Build sorted output
	var dates []string
	for d := range eventsByDate {
		dates = append(dates, d)
	}
	sort.Slice(dates, func(i, j int) bool {
		return dates[i] > dates[j]
	})
	for _, d := range dates {
		fmt.Println(d)
		for _, ev := range eventsByDate[d] {
			fmt.Printf("  - %s (uscftid:%v)\n", ev.Name, ev.ID)
		}
	}
	fmt.Printf("\nRun '%s crosstable --uscftid ID' to get results from a specific event\n",
		os.Args[0])
}

func handlePlayer(args []string) {
	fs := flag.NewFlagSet("player", flag.ExitOnError)
	memberID := fs.Int("id", 0, "USCF member id")
	eventCount := fs.Int("eventcount", 3,
		"Number of recent crosstables to retrieve (0-5)")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if *memberID == 0 {
		fmt.Fprintln(os.Stderr, "Please provide a valid --id <USCF member id>")
		fs.Usage()
		os.Exit(1)
	}

	// enforce bounds
	if *eventCount < 0 {
		*eventCount = 1
	} else if *eventCount > 5 {
		*eventCount = 5
	}

	player, err := uschess.FetchPlayer(*memberID)
	if err != nil {
		log.Fatalf("Error fetching player %v: %v", memberID, err)
	}

	xTables, err := fetchRecentPlayerCrossTables(player, *eventCount)
	if err != nil {
		log.Fatalf("Error fetching player %v events: %v", memberID, err)
	}
	output := buildPlayerOutput(player, xTables)

	fmt.Printf("%v", output)
}

func buildPlayerOutput(player *uschess.Player,
	xTables map[int][]uschess.CrossTable) string {

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Player ID:%v:\n", player.MemberID))
	sb.WriteString(fmt.Sprintf("Name: %v\n", player.Name))
	sb.WriteString(fmt.Sprintf("Live Rating(reg): %v\n", player.RegRating))
	sb.WriteString(fmt.Sprintf("Live Rating(quick): %v\n", player.QuickRating))
	sb.WriteString(fmt.Sprintf("Live Rating(blitz): %v\n", player.BlitzRating))
	sb.WriteString(fmt.Sprintf("Rated Events: %v\n", player.TotalEvents))
	if len(xTables) > 0 {
		sb.WriteString(fmt.Sprintf("Most Recent(%v) Results:\n\n",
			len(xTables)))
	}

	// Sort events by date
	var eventIDs []int
	for id := range xTables {
		eventIDs = append(eventIDs, id)
	}
	sort.Slice(eventIDs, func(i, j int) bool {
		evI := getEventFromId(player.RecentEvents, eventIDs[i])
		evJ := getEventFromId(player.RecentEvents, eventIDs[j])
		return evI.EndDate.After(evJ.EndDate)
	})

	for _, eventId := range eventIDs {
		event := getEventFromId(player.RecentEvents, eventId)

		sb.WriteString(fmt.Sprintf("%v - %v\n",
			event.EndDate.Format("2006-01-02"), event.Name))

		// a player can have multiple cross tables from a single event
		// if they play in multiple sections. e.g. the player switched
		// sections during an event, or a td created a side games section
		for _, xt := range xTables[eventId] {
			output := buildOneCrossTableOutput(&xt, true, player.MemberID)
			sb.WriteString(output)
		}
	}

	return sb.String()
}

func getEventFromId(events []uschess.Event, eventId int) uschess.Event {
	for _, event := range events {
		if event.ID == eventId {
			return event
		}
	}

	panic("BUG: invariant: eventId should be present in events slice")
}

func fetchRecentPlayerCrossTables(player *uschess.Player,
	eventCount int) (map[int][]uschess.CrossTable, error) {

	xTables := make(map[int][]uschess.CrossTable)
	var mu sync.Mutex
	g, _ := errgroup.WithContext(context.Background())
	count := 0
	for _, ev := range player.RecentEvents {
		if count >= eventCount {
			break
		}
		count++
		g.Go(func() error {
			sections, err := uschess.FetchCrossTables(ev.ID)
			if err != nil {
				return fmt.Errorf("error fetching cross tables for event %v: %w",
					ev.ID, err)
			}
			var xts []uschess.CrossTable
			for _, section := range sections {
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
