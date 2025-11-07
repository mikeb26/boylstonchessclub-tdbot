/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/mikeb26/boylstonchessclub-tdbot/bcc"
	"github.com/mikeb26/boylstonchessclub-tdbot/uschess"
)

type TdSubCommand string

const (
	TdAboutCmd      TdSubCommand = "about"
	TdHelpCmd       TdSubCommand = "help"
	TdCalCmd        TdSubCommand = "cal"
	TdEntriesCmd    TdSubCommand = "entries"
	TdEventCmd      TdSubCommand = "event"
	TdPairingsCmd   TdSubCommand = "pairings"
	TdStandingsCmd  TdSubCommand = "standings"
	TdPlayerCmd     TdSubCommand = "player"
	TdCrossTableCmd TdSubCommand = "crosstable"
)

var tdSubCmdHdlrs = map[TdSubCommand]CmdHandler{
	TdAboutCmd:      tdAboutCmdHandler,
	TdHelpCmd:       tdHelpCmdHandler,
	TdCalCmd:        tdCalCmdHandler,
	TdEntriesCmd:    tdEntriesCmdHandler,
	TdEventCmd:      tdEventCmdHandler,
	TdPairingsCmd:   tdPairingsCmdHandler,
	TdStandingsCmd:  tdStandingsCmdHandler,
	TdPlayerCmd:     tdPlayerCmdHandler,
	TdCrossTableCmd: tdCrossTableCmdHandler,
}

func tdCmdHandler(ctx context.Context,
	inter *discordgo.Interaction) *discordgo.InteractionResponse {

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
	return hdlr(ctx, inter)
}

//go:embed about.txt
var aboutText string

func tdAboutCmdHandler(ctx context.Context,
	inter *discordgo.Interaction) *discordgo.InteractionResponse {

	resp := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	}

	resp.Data.Content, _ = truncateContent(aboutText)

	return resp
}

//go:embed help.md
var helpText string

func tdHelpCmdHandler(ctx context.Context,
	inter *discordgo.Interaction) *discordgo.InteractionResponse {

	resp := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	}

	resp.Data.Content, _ = truncateContent(helpText)
	return resp
}

func tdCalCmdHandler(ctx context.Context,
	inter *discordgo.Interaction) *discordgo.InteractionResponse {

	resp := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	}

	data := inter.ApplicationCommandData()
	days := int64(14)  // default
	broadcast := false // default
	if len(data.Options) > 0 {
		for _, opt := range data.Options[0].Options {
			if opt.Name == "days" {
				days = opt.IntValue()
			} else if opt.Name == "broadcast" {
				broadcast = opt.BoolValue()
			}
		}
	}
	// enforce bounds
	if days <= 0 {
		days = 14
	} else if days > 60 {
		days = 60
	}

	now := time.Now()
	nowDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end := nowDate.AddDate(0, 0, int(days))

	// Fetch events from BCC API
	events, err := bcc.GetEvents()
	if err != nil {
		resp.Data.Content = fmt.Sprintf("Error fetching events: %v", err)
		log.Printf("discordbot.cal: %v", resp.Data.Content)
		return resp
	}

	// Filter and group events by date
	eventsByDate := make(map[string][]bcc.Event)
	for _, ev := range events {
		// truncate event date to local date for inclusive comparison
		evDate := time.Date(ev.Date.Year(), ev.Date.Month(), ev.Date.Day(), 0, 0, 0, 0, nowDate.Location())
		if evDate.Before(nowDate) || evDate.After(end) {
			continue
		}
		key := ev.Date.Format("2006-01-02")
		eventsByDate[key] = append(eventsByDate[key], ev)
	}

	if len(eventsByDate) == 0 {
		resp.Data.Content = fmt.Sprintf("No events found in the next %d days.", days)
		log.Printf("discordbot.cal: %v", resp.Data.Content)
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
			sb.WriteString(fmt.Sprintf("- %v (EventID:%v)\n", ev.Title, ev.EventID))
		}
	}
	sb.WriteString("\nRun /td event <EventID> to get details on a specific event\n")
	resp.Data.Content, _ = truncateContent(sb.String())

	if broadcast {
		resp.Data.Flags = 0
	}

	return resp
}

func tdEventCmdHandler(ctx context.Context,
	inter *discordgo.Interaction) *discordgo.InteractionResponse {

	resp := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	}

	data := inter.ApplicationCommandData()
	broadcast := false // default
	var eventID int64
	if len(data.Options) > 0 {
		found := false
		for _, opt := range data.Options[0].Options {
			if opt.Name == "eventid" {
				eventID = opt.IntValue()
				found = true
			} else if opt.Name == "broadcast" {
				broadcast = opt.BoolValue()
			}
		}
		if !found {
			resp.Data.Content = "Please provide an event ID."
			log.Printf("discordbot.event: %v", resp.Data.Content)
			return resp
		}
	} else {
		resp.Data.Content = "Please provide an event ID."
		log.Printf("discordbot.event: %v", resp.Data.Content)
		return resp
	}

	detail, err := bcc.GetEventDetail(eventID)
	if err != nil {
		resp.Data.Content = fmt.Sprintf("Error fetching event %d: %v", eventID, err)
		log.Printf("discordbot.event: %v", resp.Data.Content)
		return resp
	}

	embed := &discordgo.MessageEmbed{
		Title:       detail.Title,
		URL:         fmt.Sprintf("https://boylstonchess.org/events/%d", detail.EventID),
		Type:        discordgo.EmbedTypeLink,
		Description: bcc.BuildEventOutput(&detail, "**", false, false),
	}
	resp.Data.Embeds = []*discordgo.MessageEmbed{embed}
	if broadcast {
		resp.Data.Flags = 0
	}

	return resp
}

func tdCrossTableCmdHandler(ctx context.Context,
	inter *discordgo.Interaction) *discordgo.InteractionResponse {

	resp := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	}

	data := inter.ApplicationCommandData()
	broadcast := false // default
	section := ""
	var eventID int64
	if len(data.Options) > 0 {
		found := false
		for _, opt := range data.Options[0].Options {
			if opt.Name == "eventid" {
				eventID = opt.IntValue()
				found = true
			} else if opt.Name == "broadcast" {
				broadcast = opt.BoolValue()
			} else if opt.Name == "section" {
				section = opt.StringValue()
			}
		}
		if !found {
			resp.Data.Content = "Please provide an event ID."
			log.Printf("discordbot.xt: %v", resp.Data.Content)
			return resp
		}
	} else {
		resp.Data.Content = "Please provide an event ID."
		log.Printf("discordbot.xt: %v", resp.Data.Content)
		return resp
	}

	detail, err := bcc.GetEventDetail(eventID)
	if err != nil {
		resp.Data.Content = fmt.Sprintf("Error fetching event %d: %v", eventID, err)
		log.Printf("discordbot.xt: %v", resp.Data.Content)
		return resp
	}

	if detail.UscfTid == 0 {
		resp.Data.Content = fmt.Sprintf("The club has not yet filed event %v with USCF. crosstable currently only works for events filed with USCF; please try again once the club files it.",
			eventID)
		log.Printf("discordbot.xt: %v", resp.Data.Content)
		return resp
	}
	t, err := uschessClient.FetchCrossTables(ctx, uschess.EventID(detail.UscfTid))
	if err != nil {
		resp.Data.Content = fmt.Sprintf("Error fetching crosstables for eventid %d: %v", eventID, err)
		log.Printf("discordbot.xt: %v", resp.Data.Content)
		return resp
	}

	var sb strings.Builder
	sectionList := ""
	sectionCount := 0
	for _, xt := range t.CrossTables {
		if section != "" &&
			!strings.Contains(strings.ToLower(xt.SectionName), strings.ToLower(section)) {
			continue
		}
		if sectionList == "" {
			sectionList = xt.SectionName
		} else {
			sectionList = fmt.Sprintf("%v, %v", sectionList, xt.SectionName)
		}
		output, _ := uschess.BuildOneCrossTableOutput(xt, len(t.CrossTables) > 1, 0)
		sb.WriteString(output)
		sectionCount++
	}

	// Wrap output in code block for monospace formatting in Discord
	content, truncated := truncateContent(sb.String())
	resp.Data.Content = fmt.Sprintf("```\n%s```", content)
	if truncated && section == "" && sectionCount > 1 {
		resp.Data.Content = fmt.Sprintf("Too much data. Please try again and specify one of the following sections: %v", sectionList)
		log.Printf("discordbot.xt: %v", resp.Data.Content)
		return resp
	}

	if broadcast {
		resp.Data.Flags = 0
	}

	return resp
}

// tdPairingsCmdHandler handles the /td pairings command to display current pairings
func tdPairingsCmdHandler(ctx context.Context,
	inter *discordgo.Interaction) *discordgo.InteractionResponse {

	resp := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	}
	data := inter.ApplicationCommandData()
	broadcast := false // default
	var eventID int64
	if len(data.Options) > 0 {
		found := false
		for _, opt := range data.Options[0].Options {
			if opt.Name == "eventid" {
				eventID = opt.IntValue()
				found = true
			} else if opt.Name == "broadcast" {
				broadcast = opt.BoolValue()
			}
		}
		if !found {
			resp.Data.Content = "Please provide an event ID."
			log.Printf("discordbot.pairings: %v", resp.Data.Content)
			return resp
		}
	} else {
		resp.Data.Content = "Please provide an event ID."
		log.Printf("discordbot.pairings: %v", resp.Data.Content)
		return resp
	}
	tourney, err := bcc.GetTournament(eventID)
	if err != nil {
		resp.Data.Content = fmt.Sprintf("Error fetching pairings for event %d: %v",
			eventID, err)
		log.Printf("discordbot.pairings: %v", resp.Data.Content)
		return resp
	}
	if len(tourney.CurrentPairings) == 0 {
		resp.Data.Content = fmt.Sprintf("No pairings found for event %d.",
			eventID)
		log.Printf("discordbot.pairings: %v", resp.Data.Content)
		return resp
	}
	// Wrap output in code block for monospace formatting in Discord
	content, _ := truncateContent(bcc.BuildPairingsOutput(tourney))
	resp.Data.Content = fmt.Sprintf("```\n%s```", content)

	if broadcast {
		resp.Data.Flags = 0
	}

	return resp
}

// tdEntriesCmdHandler handles the /td entries command to display current entries
func tdEntriesCmdHandler(ctx context.Context,
	inter *discordgo.Interaction) *discordgo.InteractionResponse {

	resp := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	}
	data := inter.ApplicationCommandData()
	broadcast := false // default
	var eventID int64
	if len(data.Options) > 0 {
		found := false
		for _, opt := range data.Options[0].Options {
			if opt.Name == "eventid" {
				eventID = opt.IntValue()
				found = true
			} else if opt.Name == "broadcast" {
				broadcast = opt.BoolValue()
			}
		}
		if !found {
			resp.Data.Content = "Please provide an event ID."
			log.Printf("discordbot.pairings: %v", resp.Data.Content)
			return resp
		}
	} else {
		resp.Data.Content = "Please provide an event ID."
		log.Printf("discordbot.pairings: %v", resp.Data.Content)
		return resp
	}
	tourney, err := bcc.GetTournament(eventID)
	if err != nil {
		resp.Data.Content = fmt.Sprintf("Error fetching pairings for event %d: %v",
			eventID, err)
		log.Printf("discordbot.pairings: %v", resp.Data.Content)
		return resp
	}
	if len(tourney.CurrentPairings) == 0 {
		resp.Data.Content = fmt.Sprintf("No pairings found for event %d.",
			eventID)
		log.Printf("discordbot.pairings: %v", resp.Data.Content)
		return resp
	}
	// Wrap output in code block for monospace formatting in Discord
	content, _ := truncateContent(bcc.BuildEntriesOutput(tourney))
	resp.Data.Content = fmt.Sprintf("```\n%s```", content)

	if broadcast {
		resp.Data.Flags = 0
	}

	return resp
}

// tdStandingsCmdHandler handles the /td pairings command to display current
// standings
func tdStandingsCmdHandler(ctx context.Context,
	inter *discordgo.Interaction) *discordgo.InteractionResponse {

	resp := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	}
	data := inter.ApplicationCommandData()
	broadcast := false // default
	var eventID int64
	if len(data.Options) > 0 {
		found := false
		for _, opt := range data.Options[0].Options {
			if opt.Name == "eventid" {
				eventID = opt.IntValue()
				found = true
			} else if opt.Name == "broadcast" {
				broadcast = opt.BoolValue()
			}
		}
		if !found {
			resp.Data.Content = "Please provide an event ID."
			log.Printf("discordbot.standings: %v", resp.Data.Content)
			return resp
		}
	} else {
		resp.Data.Content = "Please provide an event ID."
		log.Printf("discordbot.standings: %v", resp.Data.Content)
		return resp
	}
	tourney, err := bcc.GetTournament(eventID)
	if err != nil {
		resp.Data.Content = fmt.Sprintf("Error fetching standings for event %d: %v",
			eventID, err)
		log.Printf("discordbot.standings: %v", resp.Data.Content)
		return resp
	}

	// Wrap output in code block for monospace formatting in Discord
	content, _ := truncateContent(bcc.BuildStandingsOutput(tourney))
	resp.Data.Content = fmt.Sprintf("```\n%s```", content)

	if broadcast {
		resp.Data.Flags = 0
	}

	return resp
}

// tdPlayerCmdHandler handles the /td player command to display information
// regarding a specific player
func tdPlayerCmdHandler(ctx context.Context,
	inter *discordgo.Interaction) *discordgo.InteractionResponse {

	resp := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	}
	data := inter.ApplicationCommandData()
	broadcast := false // default
	var memID int64
	if len(data.Options) > 0 {
		found := false
		for _, opt := range data.Options[0].Options {
			if opt.Name == "memid" {
				memID = opt.IntValue()
				found = true
			} else if opt.Name == "broadcast" {
				broadcast = opt.BoolValue()
			}
		}
		if !found {
			resp.Data.Content = "Please provide a USCF member ID."
			log.Printf("discordbot.player: %v", resp.Data.Content)
			return resp
		}
	} else {
		resp.Data.Content = "Please provide a USCF member ID."
		log.Printf("discordbot.player: %v", resp.Data.Content)
		return resp
	}

	report, err := uschessClient.GetPlayerReport(ctx, uschess.MemID(memID),
		3 /* eventCount */)
	if err != nil {
		resp.Data.Content = fmt.Sprintf("Error fetching player %v report: %v",
			memID, err)
		log.Printf("discordbot.player: %v", resp.Data.Content)
		return resp
	}

	// Wrap output in code block for monospace formatting in Discord
	content, _ := truncateContent(report)
	resp.Data.Content = fmt.Sprintf("```\n%s```", content)

	if broadcast {
		resp.Data.Flags = 0
	}

	return resp
}

// https://discord.com/developers/docs/resources/channel#start-thread-in-forum-or-media-channel-forum-and-media-thread-message-params-object
// limits messages to 2k characters
func truncateContent(s string) (string, bool) {
	truncated := false
	const MsgLimit = 1988 // keep space for newlines and markdown
	runes := []rune(s)
	if len(runes) > MsgLimit {
		s = fmt.Sprintf("%v...", string(runes[:MsgLimit]))
		truncated = true
	}
	return s, truncated
}
