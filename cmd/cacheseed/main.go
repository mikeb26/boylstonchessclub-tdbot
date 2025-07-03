/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package main

import (
	_ "embed"
	"fmt"
	"time"

	"github.com/mikeb26/boylstonchessclub-tdbot/bcc"
	"github.com/mikeb26/boylstonchessclub-tdbot/internal"
	"github.com/mikeb26/boylstonchessclub-tdbot/uschess"
)

// this program exists just to seed the http cache for bcc members

func main() {
	for _, memId := range bcc.ActivePlayerMemIds() {
		player, err := uschess.FetchPlayer(memId)
		time.Sleep(2 * time.Second) // avoid pegging uschess.org
		if err != nil {
			// best effort
			continue
		}

		fmt.Printf("seeded %v player data\n", player.Name)
	}

	for _, tid := range bcc.ActivePlayerTIds() {
		_, err := uschess.FetchCrossTables(tid)
		time.Sleep(2 * time.Second) // avoid pegging uschess.org
		if err != nil {
			// best effort
			continue
		}

		fmt.Printf("seeded tid:%v\n", tid)
	}

	events, err := uschess.GetAffiliateEvents(internal.BccUSCFAffiliateID)
	if err != nil {
		// best effort
		return
	}
	for _, event := range events {
		_, err := uschess.FetchCrossTables(event.ID)
		time.Sleep(2 * time.Second) // avoid pegging uschess.org
		if err != nil {
			// best effort
			continue
		}

		fmt.Printf("seeded ev:%v\n", event.Name)
	}
}
