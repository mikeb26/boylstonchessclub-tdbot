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
	base30day := httpcache.NewCachedHttpClient(ctx, 30*24*time.Hour)
	base1day := http.DefaultClient
	if base30day != http.DefaultClient {
		base1day = httpcache.NewCachedHttpClient(ctx, 24*time.Hour)
	}

	return &Client{
		httpClient30day: newRetryingClient(base30day),
		httpClient1day:  newRetryingClient(base1day),
	}
}
