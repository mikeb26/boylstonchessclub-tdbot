/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package main

import (
	_ "embed"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/bwmarrin/discordgo"
	"golang.org/x/sync/errgroup"
)

type TdSubCommand string

const (
	TdAboutCmd TdSubCommand = "about"
	TdHelpCmd  TdSubCommand = "help"
	TdCalCmd   TdSubCommand = "cal"
	TdEventCmd TdSubCommand = "event"
	// pairings is a work in progress; see ../pairings
)

var tdSubCmdHdlrs = map[TdSubCommand]CmdHandler{
	TdAboutCmd: tdAboutCmdHandler,
	TdHelpCmd:  tdHelpCmdHandler,
	TdCalCmd:   tdCalCmdHandler,
	TdEventCmd: tdEventCmdHandler,
}

func tdCmdHandler(inter *discordgo.Interaction) *discordgo.InteractionResponse {
	data := inter.ApplicationCommandData()
	hdlr := tdHelpCmdHandler
	if len(data.Options) > 0 {
		if subName := data.Options[0].Name; subName != "" {
			h, ok := tdSubCmdHdlrs[TdSubCommand(subName)]
			if ok {
				hdlr = h
			}
		}
	}
	return hdlr(inter)
}

//go:embed about.txt
var aboutText string

func tdAboutCmdHandler(inter *discordgo.Interaction) *discordgo.InteractionResponse {
	resp := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{},
	}

	resp.Data.Content = aboutText

	return resp
}

//go:embed help.md
var helpText string

func tdHelpCmdHandler(inter *discordgo.Interaction) *discordgo.InteractionResponse {
	resp := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{},
	}

	resp.Data.Content = helpText
	return resp
}

// getEventsByDate retrieves events from the calendar between now and end dates.
func getEventsByDate(now, end time.Time) (map[string][]string, error) {
	eventsByDate := make(map[string][]string)
	loc := now.Location()

	// Build list of months to fetch
	var months []time.Time
	current := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	for m := current; !m.After(end); m = m.AddDate(0, 1, 0) {
		months = append(months, m)
	}

	var mu sync.Mutex
	var g errgroup.Group

	// Fetch and parse each month concurrently
	for _, month := range months {
		m := month
		g.Go(func() error {
			monthName := strings.ToLower(m.Month().String())
			url := fmt.Sprintf("https://boylstonchess.org/calendar/%s-%d",
				monthName, m.Year())

			httpResp, err := http.Get(url)
			if err != nil {
				return fmt.Errorf("failed to fetch %s: %w", url, err)
			}
			defer httpResp.Body.Close()
			if httpResp.StatusCode != http.StatusOK {
				return fmt.Errorf("calendar page %s returned status %d",
					url, httpResp.StatusCode)
			}

			doc, err := goquery.NewDocumentFromReader(httpResp.Body)
			if err != nil {
				return fmt.Errorf("failed to parse calendar page %s: %w",
					url, err)
			}

			// Select date rows and corresponding body rows
			dates := doc.Find("tr.dates")
			bodies := doc.Find("tr.calbody")

			dates.Each(func(i int, dateRow *goquery.Selection) {
				bodyRow := bodies.Eq(i)
				// Parse entries for this row, synchronizing writes
				parseDateEntries(dateRow, bodyRow, m, now, end, loc, &mu, eventsByDate)
			})

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return eventsByDate, err
	}

	return eventsByDate, nil
}

// parseDateEntries processes a row of dates and corresponding events.
func parseDateEntries(
	dateRow, bodyRow *goquery.Selection,
	monthStart, now time.Time,
	end time.Time,
	loc *time.Location,
	mu *sync.Mutex,
	eventsByDate map[string][]string,
) {
	dateRow.Find("td").Each(func(j int, cell *goquery.Selection) {
		dayText := strings.TrimSpace(cell.Text())
		if dayText == "" {
			return
		}
		day, err := strconv.Atoi(dayText)
		if err != nil {
			return
		}
		// skip days outside current month
		if cls, _ := cell.Attr("class"); strings.Contains(cls, "om") {
			return
		}

		date := time.Date(monthStart.Year(), monthStart.Month(), day, 0, 0, 0, 0, loc)
		if date.Before(now) || date.After(end) {
			return
		}

		eventCell := bodyRow.Find("td").Eq(j)
		eventCell.Find("a").Each(func(_ int, link *goquery.Selection) {
			title := strings.TrimSpace(link.Text())
			href, ok := link.Attr("href")
			if !ok {
				return
			}
			parts := strings.Split(href, "/")
			eventID := ""
			if len(parts) > 1 {
				eventID = parts[len(parts)-2]
			}

			key := date.Format("2006-01-02")

			mu.Lock()
			eventsByDate[key] = append(eventsByDate[key], fmt.Sprintf("%v (eventId:%v)",
				title, eventID))
			mu.Unlock()
		})
	})
}

func tdCalCmdHandler(inter *discordgo.Interaction) *discordgo.InteractionResponse {
	resp := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{},
	}

	data := inter.ApplicationCommandData()
	days := 14 // default
	if len(data.Options) != 0 && len(data.Options[0].Options) != 0 {
		daysStr := data.Options[0].Options[0].StringValue()
		d, err := strconv.Atoi(daysStr)
		if err == nil && d > 0 {
			days = d
		}
	}

	now := time.Now()
	end := now.AddDate(0, 0, days)

	eventsByDate, err := getEventsByDate(now, end)
	if err != nil {
		resp.Data.Content = fmt.Sprintf("Error fetching events: %v", err)
		return resp
	}

	if len(eventsByDate) == 0 {
		resp.Data.Content = fmt.Sprintf("No events found in the next %d days.", days)
		return resp
	}

	// Build sorted output
	var datesList []string
	for d := range eventsByDate {
		datesList = append(datesList, d)
	}
	sort.Strings(datesList)
	var sb strings.Builder
	for _, d := range datesList {
		sb.WriteString(fmt.Sprintf("**%s**\n", d))
		for _, ev := range eventsByDate[d] {
			sb.WriteString(fmt.Sprintf("- %s\n", ev))
		}
	}
	sb.WriteString("\nRun /td event <eventId> to get details on a specific event\n")

	resp.Data.Content = sb.String()
	return resp
}

func tdEventCmdHandler(inter *discordgo.Interaction) *discordgo.InteractionResponse {
	resp := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{},
	}

	data := inter.ApplicationCommandData()
	if len(data.Options) == 0 || len(data.Options[0].Options) == 0 {
		resp.Data.Content = "Please provide an event ID."
		return resp
	}

	eventID := data.Options[0].Options[0].StringValue()

	url := fmt.Sprintf("https://boylstonchess.org/events/%s", eventID)
	urlResp, err := http.Get(url)
	if err != nil {
		resp.Data.Content = fmt.Sprintf("Could not fetch event %v: %v", eventID,
			err)
		return resp
	}
	defer urlResp.Body.Close()
	if urlResp.StatusCode != http.StatusOK {
		resp.Data.Content = fmt.Sprintf("Could not find event %v", eventID)
		return resp
	}

	doc, err := goquery.NewDocumentFromReader(urlResp.Body)
	if err != nil {
		resp.Data.Content =
			fmt.Sprintf("Error parsing event page: err:%v page:%v", err, url)
		return resp
	}

	// Extract title
	title := strings.TrimSpace(doc.Find("h1").First().Text())

	// Extract event details from the definition list
	var sb strings.Builder
	if title != "" {
		sb.WriteString(fmt.Sprintf("**%s**\n", title))
	}
	dl := doc.Find("dl.event-info.box")
	dl.Find("dt").Each(func(i int, dt *goquery.Selection) {
		key := strings.TrimSpace(dt.Text())
		dd := dt.NextFiltered("dd")
		val := strings.TrimSpace(dd.Text())
		if key != "" && val != "" {
			sb.WriteString(fmt.Sprintf("**%s**: %s\n", key, val))
		}
	})

	result := sb.String()
	if result == "" {
		resp.Data.Content =
			fmt.Sprintf("No event information found. Try visiting %v\n", url)
	} else {
		sb.WriteString(fmt.Sprintf("**To Register**: %v\n", url))
		resp.Data.Content = sb.String()
	}

	return resp
}
