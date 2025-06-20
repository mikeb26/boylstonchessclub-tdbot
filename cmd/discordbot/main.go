/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/bwmarrin/discordgo"

	_ "embed"
)

//go:embed token.priv
var botPrivToken string

//go:embed key.pub
var botPubKeyText string
var botPubKey ed25519.PublicKey

//go:embed app.id
var botAppId string

const TdCmdId = "1382811720254230578"

var client *discordgo.Session

type TopLevelCommand string

const (
	TdCmd     TopLevelCommand = "td"
	UserAgent                 = "boylstonchessclub-tdbot/0.5.2 (+https://github.com/mikeb26/boylstonchessclub-tdbot)"
)

type CmdHandler func(i *discordgo.Interaction) *discordgo.InteractionResponse

var topLevelCmdHdlrs = map[TopLevelCommand]CmdHandler{
	TdCmd: tdCmdHandler,
}

func logHeaders(r *http.Request) {
	for name, values := range r.Header {
		for _, value := range values {
			log.Printf("  %v: %v\n", name, value)
		}
	}
}

func interactionHandler(w http.ResponseWriter, r *http.Request) {
	// log.Printf("discordbot.int: processing new request HEADERS:")
	// logHeaders(r)

	if !discordgo.VerifyInteraction(r, botPubKey) {
		log.Printf("discordbot.int: failed to verify")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("discordbot.int: failed to read request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var inter discordgo.Interaction
	if err := inter.UnmarshalJSON(body); err != nil {
		log.Printf("discordbot.int: failed to unmarshal interaction: err:%v body:%v",
			err, body)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	resp := &discordgo.InteractionResponse{}
	if inter.Type == discordgo.InteractionPing {
		resp.Type = discordgo.InteractionResponsePong
	} else if inter.Type == discordgo.InteractionApplicationCommand {
		hdlr, ok :=
			topLevelCmdHdlrs[TopLevelCommand(inter.ApplicationCommandData().Name)]
		if !ok {
			resp.Type = discordgo.InteractionResponseChannelMessageWithSource
			resp.Data = &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("unknown command '%v'",
					inter.ApplicationCommandData().Name),
				Flags: discordgo.MessageFlagsEphemeral,
			}
		} else {
			resp = hdlr(&inter)
		}
	} else {
		log.Printf("discordbot.int: unimplemented interation type %v: inter:%v",
			inter.Type, inter)
		w.WriteHeader(http.StatusNotImplemented)
		return
	}

	rawResp, err := json.Marshal(resp)
	if err != nil {
		log.Printf("discordbot.int: failed to marshal resp: err:%v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_, err = w.Write(rawResp)
	if err != nil {
		log.Printf("discordbot.int: failed to write resp: err:%v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	return
}

func init() {
	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))

	pubKeyBytes, err := hex.DecodeString(botPubKeyText)
	if err != nil {
		log.Fatalf("discordbot.init: Failed to parse public key: %v", err)
	}
	botPubKey = ed25519.PublicKey(pubKeyBytes)

	client, err = discordgo.New("Bot " + botPrivToken)
	if err != nil {
		log.Fatalf("dicordbot.init: Failed to initialize discord client: %v", err)
	}
}

//go:embed lastupdate.hash
var lastCmdUpdateHash string

func shouldUpdateCmdRegistration(cmd *discordgo.ApplicationCommand) bool {
	cmdJson, err := json.Marshal(cmd)
	if err != nil {
		log.Fatalf("discordbot.reg: failed to marshal cmd: %v", err)
		return false
	}
	hasher := sha256.New()
	hasher.Write(cmdJson)
	hash := hasher.Sum(nil)
	hexString := hex.EncodeToString(hash)

	shouldUpdate := (hexString != lastCmdUpdateHash)

	if shouldUpdate {
		log.Printf("discordbot.reg: updating cmd reg; please update	lastupdate.hash to %v",
			hexString)
	}

	return shouldUpdate
}

func registerSlashCommands() {
	tdCmd := &discordgo.ApplicationCommand{
		Name:        string(TdCmd),
		Description: "Tournament director commands; try /td help to start",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        string(TdHelpCmd),
				Description: "Show usage for td",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        string(TdAboutCmd),
				Description: "Show information about boylstoness-tdbot",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        string(TdCalCmd),
				Description: "Show upcoming events on the calendar",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionInteger,
						Name:        "days",
						Description: "Number of days to retrieve (default is 14)",
						Required:    false,
					},
					{
						Type:        discordgo.ApplicationCommandOptionBoolean,
						Name:        "broadcast",
						Description: "Share with the rest of the channel instead of	only to you (default is false)",
						Required:    false,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        string(TdEventCmd),
				Description: "Get information regarding an event",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionInteger,
						Name:        "eventid",
						Description: "Event id of the tournament (as returned by cal)",
						Required:    true,
					},
					{
						Type:        discordgo.ApplicationCommandOptionBoolean,
						Name:        "broadcast",
						Description: "Share with the rest of the channel instead of	only to you (default is false)",
						Required:    false,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        string(TdPairingsCmd),
				Description: "Get current pairings for an event",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionInteger,
						Name:        "eventid",
						Description: "Event id of the tournament (as returned by cal)",
						Required:    true,
					},
					{
						Type:        discordgo.ApplicationCommandOptionBoolean,
						Name:        "broadcast",
						Description: "Share with the rest of the channel instead of	only to you (default is false)",
						Required:    false,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        string(TdStandingsCmd),
				Description: "Get current standings for an event",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionInteger,
						Name:        "eventid",
						Description: "Event id of the tournament (as returned by cal)",
						Required:    true,
					},
					{
						Type:        discordgo.ApplicationCommandOptionBoolean,
						Name:        "broadcast",
						Description: "Share with the rest of the channel instead of	only to you (default is false)",
						Required:    false,
					},
				},
			},
		},
	}

	if TdCmdId == "" {
		cmd, err := client.ApplicationCommandCreate(botAppId, "", tdCmd)
		if err != nil {
			log.Printf("discordbot.reg: failed to register %v: %v", tdCmd.Name,
				err)
			return
		}

		log.Printf("discordbot.reg: registered %v(cmdID:%v)", cmd.Name, cmd.ID)
	} else if shouldUpdateCmdRegistration(tdCmd) {
		cmd, err := client.ApplicationCommandEdit(botAppId, "", TdCmdId, tdCmd)
		if err != nil {
			log.Printf("discordbot.reg: failed to update %v: %v", tdCmd.Name,
				err)
			return
		}

		log.Printf("discordbot.reg: updated %v(cmdID:%v)", cmd.Name, cmd.ID)
	}
}

func main() {
	go registerSlashCommands()

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "localhost"
	}
	log.Printf("discordbot.main: starting server on %v:8080", hostname)

	http.HandleFunc("/DiscordBot/Interaction", interactionHandler)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("discordbot.main: Serve failed: %v", err)
	}

	log.Printf("discordbot.main: exiting")
}
