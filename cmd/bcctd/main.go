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
	"time"

	"github.com/mikeb26/boylstonchessclub-tdbot/bcc"
	"github.com/mikeb26/boylstonchessclub-tdbot/internal"
	"github.com/mikeb26/boylstonchessclub-tdbot/uschess"
)

//go:embed help.txt
var helpText string

// cmdHandler defines the signature for command handler functions.
type cmdHandler func(ctx context.Context, args []string)

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
	ctx := context.Background()

	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	cmd := os.Args[1]
	if handler, ok := commands[cmd]; ok {
		handler(ctx, os.Args[2:])
	} else {
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Printf("%v", helpText)
}

func handleHelp(ctx context.Context, args []string) {
	usage()
}

func handleCal(ctx context.Context, args []string) {
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

func handleEvent(ctx context.Context, args []string) {
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

func handlePairings(ctx context.Context, args []string) {
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

func handleStandings(ctx context.Context, args []string) {
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

func handleCrossTable(ctx context.Context, args []string) {
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

	xTables, err := uschess.FetchCrossTables(ctx, uschess.EventID(*tid))
	if err != nil {
		log.Fatalf("Error fetching cross tables %d: %v", *tid, err)
	}

	for _, xt := range xTables {
		output := uschess.BuildOneCrossTableOutput(xt, len(xTables) > 1, 0)
		fmt.Printf(output)
	}
}

func handleHistory(ctx context.Context, args []string) {
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

	events, err := uschess.GetAffiliateEvents(ctx, *aid)
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

func handlePlayer(ctx context.Context, args []string) {
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

	report, err := uschess.GetPlayerReport(ctx, uschess.MemID(*memberID),
		*eventCount)
	if err != nil {
		log.Fatalf("Error fetching player %v: %v", memberID, err)
	}

	fmt.Printf("%v", report)
}
