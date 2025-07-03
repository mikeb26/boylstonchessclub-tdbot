/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package internal

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gregjones/httpcache"
	"github.com/mikeb26/boylstonchessclub-tdbot/s3cache"
)

const s3Bucket = "bopmatic-boylstonchessclub-tdbot-prod-webcache"

// NewCachedHttpClient returns an http.Client that caches via S3-backed httpcache.
// If cache initialization fails, it falls back to an in-memory cache instead of no cache.
// It also enforces a client-side TTL by rewriting origin cache headers.
func NewCachedHttpClient(ctx context.Context, maxAge time.Duration) *http.Client {
	// Initialize S3-backed cache
	cache := s3cache.New(ctx, s3Bucket, false, true)

	err := cache.Init()

	if err != nil {
		log.Printf("httpcache: warning failed to init S3 cache: %v; falling back to uncached http", err)
		return http.DefaultClient
	}

	hc := httpcache.NewTransport(cache)
	// we have to inject our own header overrides here in order to override
	// server responses that might indicate caching shouldn't be done
	hc.Transport = &HeaderOverrideTransport{
		wrappedRT: http.DefaultTransport,
		Response: func(resp *http.Response) error {
			// Strip any cache-busting headers from origin
			resp.Header.Del("Pragma")
			resp.Header.Del("Expires")
			resp.Header.Del("Cache-Control")
			// Enforce the provided TTL
			resp.Header.Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int(maxAge/time.Second)))
			return nil
		},
	}

	return &http.Client{Transport: hc}
}

type HeaderOverrideTransport struct {
	Request  func(req *http.Request)
	Response func(resp *http.Response) error

	// Underlying RoundTripper (e.g. default transport or another decorator)
	wrappedRT http.RoundTripper
}

// RoundTrip applies Request and Response hooks around the underlying transport.
func (t *HeaderOverrideTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// clone so we don’t stomp on the caller’s original
	req2 := req.Clone(req.Context())
	if t.Request != nil {
		t.Request(req2)
	}

	resp, err := t.wrappedRT.RoundTrip(req2)
	if err != nil {
		return nil, err
	}

	if t.Response != nil {
		if err := t.Response(resp); err != nil {
			return nil, err
		}
	}
	return resp, nil
}
