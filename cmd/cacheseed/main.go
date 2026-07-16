/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package main

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/mikeb26/boylstonchessclub-tdbot/bcc"
	"github.com/mikeb26/boylstonchessclub-tdbot/internal"
	"github.com/mikeb26/boylstonchessclub-tdbot/uscfutils"
	uschess "github.com/mikeb26/uschess-go"
)

// this program exists just to seed the http cache for bcc members
var uschessClient *uschess.ClientWithResponses

func main() {
	ctx := context.Background()

	var err error
	uschessClient, err = uscfutils.NewClient(context.Background())
	if err != nil {
		return
	}

	for _, memId := range bcc.ActivePlayerMemIds() {
		player, err := uschessClient.GetPlayer(ctx, memId, nil)
		time.Sleep(2 * time.Second) // avoid pegging uschess.org
		if err != nil {
			// best effort
			continue
		}

		fmt.Printf("seeded %v %v player data\n", player.FirstName, player.LastName)
	}

	for _, tid := range bcc.ActivePlayerTIds() {
		tourney, err := uschessClient.GetTournament(ctx, tid)
		time.Sleep(2 * time.Second) // avoid pegging uschess.org
		if err != nil {
			// best effort
			continue
		}

		fmt.Printf("seeded ev:%v\n", tourney.Name)
	}

	events, err := uschessClient.GetAllAffiliateRatedEvents(ctx,
		uschess.AffiliateID(internal.BccUSCFAffiliateID), nil)
	if err != nil {
		// best effort
		return
	}
	for _, event := range events {
		_, err := uschessClient.GetTournament(ctx, event.Id)
		time.Sleep(2 * time.Second) // avoid pegging uschess.org
		if err != nil {
			// best effort
			continue
		}

		fmt.Printf("seeded ev:%v\n", event.Name)
	}
}
