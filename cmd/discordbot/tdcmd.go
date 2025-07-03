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
	TdAboutCmd     TdSubCommand = "about"
	TdHelpCmd      TdSubCommand = "help"
	TdCalCmd       TdSubCommand = "cal"
	TdEventCmd     TdSubCommand = "event"
	TdPairingsCmd  TdSubCommand = "pairings"
	TdStandingsCmd TdSubCommand = "standings"
	TdPlayerCmd    TdSubCommand = "player"
)

var tdSubCmdHdlrs = map[TdSubCommand]CmdHandler{
	TdAboutCmd:     tdAboutCmdHandler,
	TdHelpCmd:      tdHelpCmdHandler,
	TdCalCmd:       tdCalCmdHandler,
	TdEventCmd:     tdEventCmdHandler,
	TdPairingsCmd:  tdPairingsCmdHandler,
	TdStandingsCmd: tdStandingsCmdHandler,
	TdPlayerCmd:    tdPlayerCmdHandler,
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

	resp.Data.Content = truncateContent(aboutText)

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

	resp.Data.Content = truncateContent(helpText)
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
	end := now.AddDate(0, 0, int(days))

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
		if ev.Date.Before(now) || ev.Date.After(end) {
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
	resp.Data.Content = truncateContent(sb.String())

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

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**EventID**: %d\n", detail.EventID))
	sb.WriteString(fmt.Sprintf("**Date**: %s\n", detail.DateDisplay))
	if detail.EventFormat != "" {
		sb.WriteString(fmt.Sprintf("**Format**: %s\n", detail.EventFormat))
	}
	if detail.TimeControl != "" {
		sb.WriteString(fmt.Sprintf("**Time Control**: %s\n",
			detail.TimeControl))
	}
	if detail.SectionDisplay != "" {
		sb.WriteString(fmt.Sprintf("**Sections**: %s\n", detail.SectionDisplay))
	}
	sb.WriteString(fmt.Sprintf("**Entry Fee**: %s\n", detail.EntryFeeSummary))
	if detail.PrizeSummary != "" {
		sb.WriteString(fmt.Sprintf("**Prizes**: %s\n", detail.PrizeSummary))
	}
	if detail.RegistrationTime != "" {
		sb.WriteString(fmt.Sprintf("**Registration Time**: %s\n",
			detail.RegistrationTime))
	}
	sb.WriteString(fmt.Sprintf("**Round Times**: %s\n", detail.RoundTimes))
	sb.WriteString(fmt.Sprintf("**Description**: %s\n", detail.Description))
	embed := &discordgo.MessageEmbed{
		Title:       detail.Title,
		URL:         fmt.Sprintf("https://boylstonchess.org/events/%d", detail.EventID),
		Type:        discordgo.EmbedTypeLink,
		Description: sb.String(),
	}
	resp.Data.Embeds = []*discordgo.MessageEmbed{embed}
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
	resp.Data.Content = fmt.Sprintf("```\n%s```",
		truncateContent(bcc.BuildPairingsOutput(tourney)))

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
	resp.Data.Content =
		fmt.Sprintf("```\n%s```",
			truncateContent(bcc.BuildStandingsOutput(tourney)))

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

	report, err := uschess.GetPlayerReport(ctx, int(memID), 3 /* eventCount */)
	if err != nil {
		resp.Data.Content = fmt.Sprintf("Error fetching player %v report: %v",
			memID, err)
		log.Printf("discordbot.player: %v", resp.Data.Content)
		return resp
	}

	// Wrap output in code block for monospace formatting in Discord
	resp.Data.Content = fmt.Sprintf("```\n%s```", truncateContent(report))

	if broadcast {
		resp.Data.Flags = 0
	}

	return resp
}

// https://discord.com/developers/docs/resources/channel#start-thread-in-forum-or-media-channel-forum-and-media-thread-message-params-object
// limits messages to 2k characters
func truncateContent(s string) string {
	const MsgLimit = 1988 // keep space for newlines and markdown
	runes := []rune(s)
	if len(runes) > MsgLimit {
		s = fmt.Sprintf("%v...", string(runes[:MsgLimit]))
	}
	return s
}
