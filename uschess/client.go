/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package uschess

import (
	"context"
	"net/http"
	"time"

	"github.com/mikeb26/boylstonchessclub-tdbot/internal/httpcache"
)

type Client struct {
	httpClient30day *http.Client
	httpClient1day  *http.Client
}

func NewClient(ctx context.Context) *Client {
	ret := &Client{
		httpClient30day: httpcache.NewCachedHttpClient(ctx, 30*24*time.Hour),
	}
	if ret.httpClient30day != http.DefaultClient {
		ret.httpClient1day = httpcache.NewCachedHttpClient(ctx, 24*time.Hour)
	} else {
		ret.httpClient1day = http.DefaultClient
	}

	return ret
}
