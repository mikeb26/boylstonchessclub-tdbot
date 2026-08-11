/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package httpcache

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gregjones/httpcache"
	"github.com/mikeb26/boylstonchessclub-tdbot/internal"
	"github.com/mikeb26/boylstonchessclub-tdbot/s3cache"
)

// NewCachedHttpClient returns an http.Client that caches via S3-backed httpcache.
// If cache initialization fails, it falls back to an in-memory cache instead of no cache.
// It also enforces a client-side TTL by rewriting origin cache headers.
func NewCachedHttpClient(ctx context.Context, maxAge time.Duration) *http.Client {
	// Initialize S3-backed cache
	cache := s3cache.New(ctx, internal.WebCacheBucket, false, true)

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
		Response:  cacheResponseFor(maxAge),
	}

	return &http.Client{Transport: &BypassCachedRateLimitTransport{wrappedRT: hc}}
}

// cacheResponseFor applies the application's TTL only to successful origin
// responses. In particular, a 429 must not be cached: uschess-go retries 429
// responses, but caching one makes every retry read the same cached 429 rather
// than issue a request after its backoff delay.
func cacheResponseFor(maxAge time.Duration) func(*http.Response) error {
	return func(resp *http.Response) error {
		switch {
		case resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices:
			// Strip any cache-busting headers from a successful origin response.
			resp.Header.Del("Pragma")
			resp.Header.Del("Expires")
			resp.Header.Del("Cache-Control")
			resp.Header.Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int(maxAge/time.Second)))
		case resp.StatusCode != http.StatusNotModified:
			// gregjones/httpcache otherwise stores any response that does not
			// explicitly opt out, including 429 responses.
			resp.Header.Set("Cache-Control", "no-store")
		}
		return nil
	}
}

type HeaderOverrideTransport struct {
	Request  func(req *http.Request)
	Response func(resp *http.Response) error

	// Underlying RoundTripper (e.g. default transport or another decorator)
	wrappedRT http.RoundTripper
}

// BypassCachedRateLimitTransport prevents a rate-limit response cached by an
// earlier version of the client from suppressing an origin request. The cache
// transport is intentionally below this transport so a fresh cached success is
// still returned normally.
type BypassCachedRateLimitTransport struct {
	wrappedRT http.RoundTripper
}

func (t *BypassCachedRateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.wrappedRT.RoundTrip(req)
	if err != nil || resp.StatusCode != http.StatusTooManyRequests ||
		resp.Header.Get(httpcache.XFromCache) != "1" {
		return resp, err
	}

	_ = resp.Body.Close()
	req = req.Clone(req.Context())
	req.Header.Set("Cache-Control", "no-cache")
	return t.wrappedRT.RoundTrip(req)
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
