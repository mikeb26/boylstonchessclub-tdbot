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
	"github.com/mikeb26/boylstonchessclub-tdbot/uschess"
)

// this program exists just to seed the http cache for bcc members
var uschessClient *uschess.Client

func main() {
	ctx := context.Background()

	uschessClient = uschess.NewClient(context.Background())

	for _, memId := range bcc.ActivePlayerMemIds() {
		player, err := uschessClient.FetchPlayer(ctx, memId)
		time.Sleep(2 * time.Second) // avoid pegging uschess.org
		if err != nil {
			// best effort
			continue
		}

		fmt.Printf("seeded %v player data\n", player.Name)
	}

	for _, tid := range bcc.ActivePlayerTIds() {
		tourney, err := uschessClient.FetchCrossTables(ctx, tid)
		time.Sleep(2 * time.Second) // avoid pegging uschess.org
		if err != nil {
			// best effort
			continue
		}

		fmt.Printf("seeded ev:%v\n", tourney.Event.Name)
	}

	events, err := uschessClient.GetAffiliateEvents(ctx,
		internal.BccUSCFAffiliateID)
	if err != nil {
		// best effort
		return
	}
	for _, event := range events {
		_, err := uschessClient.FetchCrossTables(ctx, event.ID)
		time.Sleep(2 * time.Second) // avoid pegging uschess.org
		if err != nil {
			// best effort
			continue
		}

		fmt.Printf("seeded ev:%v\n", event.Name)
	}
}
