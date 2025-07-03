/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package main

import (
	"context"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestTdCalCmdHandler(t *testing.T) {
	ctx := context.Background()

	// Construct a fake interaction for an application command with no options
	inter := &discordgo.Interaction{
		Type: discordgo.InteractionApplicationCommand,
		Data: discordgo.ApplicationCommandInteractionData{
			Options: []*discordgo.ApplicationCommandInteractionDataOption{},
		},
	}

	resp := tdCalCmdHandler(ctx, inter)
	if resp == nil {
		t.Fatal("Expected non-nil response")
	}

	// Verify response type is a channel message with source
	if resp.Type != discordgo.InteractionResponseChannelMessageWithSource {
		t.Errorf("Expected response type %v, got %v", discordgo.InteractionResponseChannelMessageWithSource, resp.Type)
	}

	// Ensure Data is set
	if resp.Data == nil {
		t.Fatal("Expected non-nil Data in response")
	}

	// Content should not be empty (either an event list, no-events message, or error)
	if resp.Data.Content == "" {
		t.Error("Expected non-empty response content")
	}
}

func TestTdEventCmdHandler(t *testing.T) {
	ctx := context.Background()

	// Construct a fake interaction for an application command: /td event 1312
	inter := &discordgo.Interaction{
		Type: discordgo.InteractionApplicationCommand,
		Data: discordgo.ApplicationCommandInteractionData{
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{
					Name: "event",
					Type: discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandInteractionDataOption{
						{
							Name:  "eventid",
							Type:  discordgo.ApplicationCommandOptionInteger,
							Value: 1312.0,
						},
					},
				},
			},
		},
	}

	resp := tdEventCmdHandler(ctx, inter)
	if resp == nil {
		t.Fatal("Expected non-nil response")
	}
	if resp.Data == nil {
		t.Fatal("Expected non-nil Data in response")
	}

	if len(resp.Data.Embeds) != 1 {
		t.Fatal("Expected 1 embed in response")
	}
	title := resp.Data.Embeds[0].Title
	// Expect that the event title 'Big Money Swiss' appears in the output
	if !strings.Contains(title, "Big Money Swiss") {
		t.Errorf("Expected response content to contain 'Big Money Swiss', got %q", title)
	}
}
