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
	TdAboutCmd     TdSubCommand = "about"
	TdHelpCmd      TdSubCommand = "help"
	TdCalCmd       TdSubCommand = "cal"
	TdEventCmd     TdSubCommand = "event"
	TdPairingsCmd  TdSubCommand = "pairings"
	TdStandingsCmd TdSubCommand = "standings"
)

var tdSubCmdHdlrs = map[TdSubCommand]CmdHandler{
	TdAboutCmd:     tdAboutCmdHandler,
	TdHelpCmd:      tdHelpCmdHandler,
	TdCalCmd:       tdCalCmdHandler,
	TdEventCmd:     tdEventCmdHandler,
	TdPairingsCmd:  tdPairingsCmdHandler,
	TdStandingsCmd: tdStandingsCmdHandler,
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
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	}

	resp.Data.Content = aboutText

	return resp
}

//go:embed help.md
var helpText string

func tdHelpCmdHandler(inter *discordgo.Interaction) *discordgo.InteractionResponse {
	resp := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	}

	resp.Data.Content = helpText
	return resp
}

func tdCalCmdHandler(inter *discordgo.Interaction) *discordgo.InteractionResponse {
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

	if broadcast {
		resp.Data.Flags = 0
	}

	return resp
}

func tdEventCmdHandler(inter *discordgo.Interaction) *discordgo.InteractionResponse {
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
			return resp
		}
	} else {
		resp.Data.Content = "Please provide an event ID."
		return resp
	}

	detail, err := getBccEventDetail(eventID)
	if err != nil {
		resp.Data.Content = fmt.Sprintf("Error fetching event %d: %v", eventID, err)
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
func tdPairingsCmdHandler(inter *discordgo.Interaction) *discordgo.InteractionResponse {
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
			return resp
		}
	} else {
		resp.Data.Content = "Please provide an event ID."
		return resp
	}
	tourney, err := getBccTournament(eventID)
	if err != nil {
		resp.Data.Content = fmt.Sprintf("Error fetching pairings for event %d: %v",
			eventID, err)
		return resp
	}
	if len(tourney.CurrentPairings) == 0 {
		resp.Data.Content = fmt.Sprintf("No pairings found for event %d.",
			eventID)
		return resp
	}
	resp.Data.Content = buildPairingsOutput(tourney)

	if broadcast {
		resp.Data.Flags = 0
	}

	return resp
}

// buildStandingsOutput formats standings into grouped, aligned string output
func buildStandingsOutput(t *Tournament) string {
	if len(t.CurrentPairings) == 0 {
		return "Cannot determine standings without current pairings"
	}
	secPlayers := getPlayersBySection(t)
	// Sort section names using custom criteria
	var sectionNames []string
	for sec := range secPlayers {
		sectionNames = append(sectionNames, sec)
	}
	// Use named sectionSorter instead of anonymous comparator
	sort.Sort(sectionSorter(sectionNames))
	var sb strings.Builder

	for sec, players := range secPlayers {
		sort.Slice(players, func(i, j int) bool {
			return players[i].PlaceNumber < players[j].PlaceNumber
		})

		type row struct{ rank, player, score string }
		var rows []row
		priorScore := -1.0
		for idx, p := range players {
			var rank string
			if idx != 0 && p.CurrentScoreAG == priorScore {
				rank = ""
			} else {
				rank = fmt.Sprintf("%v.", p.PlaceNumber)
				priorScore = p.CurrentScoreAG
			}
			r := row{
				rank:   rank,
				player: p.DisplayName,
				score:  fmt.Sprintf("%.1f", p.CurrentScoreAG),
			}
			rows = append(rows, r)
		}

		// Compute column widths
		maxP, maxN, maxS := len("Place"), len("Name"), len("Score")
		for _, r := range rows {
			if l := len(r.rank); l > maxP {
				maxP = l
			}
			if l := len(r.player); l > maxN {
				maxN = l
			}
			if l := len(r.score); l > maxS {
				maxS = l
			}
		}

		// Write section header and table
		if len(sectionNames) > 1 {
			if sec == "" {
				sec = "UNNAMED"
			}
			sb.WriteString(fmt.Sprintf("%s Section\n", sec))
		}
		sb.WriteString(fmt.Sprintf("%-*s  %-*s  %-*s\n", maxP, "Place", maxN,
			"Name", maxS, "Score"))
		for _, r := range rows {
			sb.WriteString(fmt.Sprintf("%-*s  %-*s  %-*s\n", maxP, r.rank,
				maxN, r.player, maxS, r.score))
		}
		sb.WriteString("\n")
	}
	// Wrap output in code block for monospace formatting in Discord
	return fmt.Sprintf("```\n%s```", sb.String())
}

// tdStandingsCmdHandler handles the /td pairings command to display current
// standings
func tdStandingsCmdHandler(inter *discordgo.Interaction) *discordgo.InteractionResponse {
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
			return resp
		}
	} else {
		resp.Data.Content = "Please provide an event ID."
		return resp
	}
	tourney, err := getBccTournament(eventID)
	if err != nil {
		resp.Data.Content = fmt.Sprintf("Error fetching standings for event %d: %v",
			eventID, err)
		return resp
	}

	resp.Data.Content = buildStandingsOutput(tourney)

	if broadcast {
		resp.Data.Flags = 0
	}

	return resp
}

func getPlayersBySection(t *Tournament) map[string][]Player {
	secPlayers := make(map[string][]Player)
	for _, pairing := range t.CurrentPairings {
		secPlayers[pairing.Section] = append(secPlayers[pairing.Section],
			pairing.WhitePlayer)
		if !pairing.IsByePairing {
			secPlayers[pairing.Section] = append(secPlayers[pairing.Section],
				pairing.BlackPlayer)
		}
	}

	return secPlayers
}

// buildPairingsOutput formats pairings into grouped, aligned string output
func buildPairingsOutput(t *Tournament) string {
	// Group pairings by section
	sections := make(map[string][]Pairing)
	for _, p := range t.CurrentPairings {
		sections[p.Section] = append(sections[p.Section], p)
	}
	// Sort section names using custom criteria
	var sectionNames []string
	for sec := range sections {
		sectionNames = append(sectionNames, sec)
	}
	// Use named sectionSorter instead of anonymous comparator
	sort.Sort(sectionSorter(sectionNames))
	var sb strings.Builder

	sb.WriteString("* Please note that pairings are tentative and subject to change before the start of the round.\n\n")

	if len(t.CurrentPairings) > 0 {
		if t.IsPredicted() {
			sb.WriteString(fmt.Sprintf("Round %v pairings are not yet posted, but here are my predicted round %v pairings:\n\n",
				t.CurrentPairings[0].RoundNumber,
				t.CurrentPairings[0].RoundNumber))
		} else {
			sb.WriteString(fmt.Sprintf("Posted Round %v Pairings:\n\n",
				t.CurrentPairings[0].RoundNumber))
		}
	} else {
		sb.WriteString("No pairings posted nor predicted")
	}

	for _, sec := range sectionNames {
		list := sections[sec]
		// Sort by board number
		sort.Slice(list, func(i, j int) bool {
			// 0 means bye
			return list[i].BoardNumber != 0 &&
				list[i].BoardNumber < list[j].BoardNumber
		})

		type row struct{ board, white, black string }
		var rows []row
		for _, p := range list {
			var w, b, bl string
			w = fmt.Sprintf("%s(%d %.1f)", p.WhitePlayer.DisplayName,
				p.WhitePlayer.PrimaryRating, p.WhitePlayer.CurrentScore)
			if p.IsByePairing {
				b = "n/a"
				if p.WhitePoints != nil && *p.WhitePoints == 1.0 {
					bl = "BYE(1.0)"
				} else {
					bl = "BYE(0.5)"
				}
			} else {
				b = fmt.Sprintf("%d.", p.BoardNumber)
				bl = fmt.Sprintf("%s(%d %.1f)", p.BlackPlayer.DisplayName,
					p.BlackPlayer.PrimaryRating, p.BlackPlayer.CurrentScore)
			}
			rows = append(rows, row{board: b, white: w, black: bl})
		}

		// Compute column widths
		maxB, maxW, maxBl := len("Board"), len("White"), len("Black")
		for _, r := range rows {
			if l := len(r.board); l > maxB {
				maxB = l
			}
			if l := len(r.white); l > maxW {
				maxW = l
			}
			if l := len(r.black); l > maxBl {
				maxBl = l
			}
		}

		// Write section header and table
		if len(sectionNames) > 1 {
			if sec == "" {
				sec = "UNNAMED"
			}
			sb.WriteString(fmt.Sprintf("%s Section\n", sec))
		}
		sb.WriteString(fmt.Sprintf("%-*s  %-*s  %-*s\n", maxB, "Board", maxW,
			"White", maxBl, "Black"))
		for _, r := range rows {
			sb.WriteString(fmt.Sprintf("%-*s  %-*s  %-*s\n", maxB, r.board,
				maxW, r.white, maxBl, r.black))
		}
		sb.WriteString("\n")
	}
	// Wrap output in code block for monospace formatting in Discord
	return fmt.Sprintf("```\n%s```", sb.String())
}

// sectionSorter implements sort.Interface for custom section ordering
// Order: "Open" first, then U<Number> sections descending by number, then
// others lexicographically
type sectionSorter []string

func (s sectionSorter) Len() int { return len(s) }

func (s sectionSorter) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

func (s sectionSorter) Less(i, j int) bool {
	a, b := s[i], s[j]
	// "Open" or "Championship" always first
	if a == "Open" && b != "Open" {
		return true
	}
	if b == "Open" && a != "Open" {
		return false
	}
	if a == "Championship" && b != "Championship" {
		return true
	}
	if b == "Championship" && a != "Championship" {
		return false
	}
	ua, ub := strings.HasPrefix(a, "U"), strings.HasPrefix(b, "U")
	// Both U-sections: compare numeric suffix descending
	if ua && ub {
		ai, errA := strconv.Atoi(strings.TrimPrefix(a, "U"))
		bi, errB := strconv.Atoi(strings.TrimPrefix(b, "U"))
		if errA == nil && errB == nil {
			return ai > bi
		}
	}
	// U-sections before non-U (after Championship)
	if ua != ub {
		return ua
	}
	// Fallback lexicographical
	return a < b
}
