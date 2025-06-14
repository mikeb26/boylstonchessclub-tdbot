/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package main

import (
	_ "embed"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
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

	// Fetch events from BCC API
	events, err := getBccEvents()
	if err != nil {
		resp.Data.Content = fmt.Sprintf("Error fetching events: %v", err)
		return resp
	}

	// Filter and group events by date
	eventsByDate := make(map[string][]Event)
	for _, ev := range events {
		if ev.Date.Before(now) || ev.Date.After(end) {
			continue
		}
		key := ev.Date.Format("2006-01-02")
		eventsByDate[key] = append(eventsByDate[key], ev)
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
			sb.WriteString(fmt.Sprintf("- %v (EventID:%v)\n", ev.Title, ev.EventID))
		}
	}
	sb.WriteString("\nRun /td event <EventID> to get details on a specific event\n")

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

	eventIDStr := data.Options[0].Options[0].StringValue()
	id, err := strconv.Atoi(eventIDStr)
	if err != nil {
		resp.Data.Content = "Please provide a valid event ID."
		return resp
	}

	detail, err := getBccEventDetail(id)
	if err != nil {
		resp.Data.Content = fmt.Sprintf("Error fetching event %d: %v", id, err)
		return resp
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**EventID**: %d\n", detail.EventID))
	sb.WriteString(fmt.Sprintf("**Title**: %s\n", detail.Title))
	sb.WriteString(fmt.Sprintf("**Date**: %s\n", detail.DateDisplay))
	sb.WriteString(fmt.Sprintf("**Format**: %s\n", detail.EventFormat))
	sb.WriteString(fmt.Sprintf("**Time Control**: %s\n", detail.TimeControl))
	if detail.SectionDisplay != "" {
		sb.WriteString(fmt.Sprintf("**Sections**: %s\n", detail.SectionDisplay))
	}
	sb.WriteString(fmt.Sprintf("**Entry Fee**: %s\n", detail.EntryFeeSummary))
	sb.WriteString(fmt.Sprintf("**Prizes**: %s\n", detail.PrizeSummary))
	sb.WriteString(fmt.Sprintf("**Registration Time**: %s\n", detail.RegistrationTime))
	sb.WriteString(fmt.Sprintf("**Round Times**: %s\n", detail.RoundTimes))
	sb.WriteString(fmt.Sprintf("**Description**: %s\n", detail.Description))
	embed := &discordgo.MessageEmbed{
		Title: detail.Title,
		URL:   fmt.Sprintf("https://boylstonchess.org/events/%d", detail.EventID),
		Type:  discordgo.EmbedTypeLink,
	}
	resp.Data.Embeds = []*discordgo.MessageEmbed{embed}
	resp.Data.Content = sb.String()

	return resp
}
