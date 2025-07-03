/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package internal

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"testing"
	"time"
)

func TestHttpClient(t *testing.T) {
	ctx := context.Background()
	client := NewCachedHttpClient(ctx, 5*time.Minute)

	if client == http.DefaultClient {
		t.Skip("Skipping test because http client is uncached")
	}
	id := 12912297
	url := fmt.Sprintf("https://www.uschess.org/msa/XtblMain.php?%v.0", id)

	for i := 0; i < 3; i++ {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Printf("discordbot.test: unable to fetch uscf crosstable (new): %v", err)
			return
		}
		req.Header.Set("User-Agent", UserAgent)
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("discordbot.test: unable to fetch uscf crosstable (do): %v", err)
			return
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Errorf("Failed to read response body")
		}
		if len(data) == 0 {
			t.Errorf("Empty data")
		}
		if i > 0 {
			if resp.Header.Get("X-From-Cache") != "1" {
				t.Errorf("object not cached")
			}
		}
		resp.Body.Close()
	}
}
