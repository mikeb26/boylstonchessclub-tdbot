/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package httpcache

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"testing"
	"time"

	cachelib "github.com/gregjones/httpcache"
	"github.com/mikeb26/boylstonchessclub-tdbot/internal"
)

func TestRateLimitedResponseIsNotCached(t *testing.T) {
	calls := 0
	origin := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		status := http.StatusTooManyRequests
		body := "rate limited"
		if calls == 2 {
			status = http.StatusOK
			body = "success"
		}
		return &http.Response{
			StatusCode: status,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString(body)),
			Request:    req,
		}, nil
	})

	cache := cachelib.NewTransport(cachelib.NewMemoryCache())
	cache.Transport = &HeaderOverrideTransport{
		wrappedRT: origin,
		Response:  cacheResponseFor(time.Hour),
	}
	client := &http.Client{Transport: cache}

	for i := 0; i < 2; i++ {
		response, err := client.Get("https://ratings-api.uschess.org/test")
		if err != nil {
			t.Fatalf("request %d: %v", i+1, err)
		}
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
		if i == 0 && response.StatusCode != http.StatusTooManyRequests {
			t.Fatalf("first response status = %d; want %d", response.StatusCode, http.StatusTooManyRequests)
		}
		if i == 1 && response.StatusCode != http.StatusOK {
			t.Fatalf("second response status = %d; want %d", response.StatusCode, http.StatusOK)
		}
	}
	if calls != 2 {
		t.Fatalf("origin calls = %d; want 2 (the 429 must not be cached)", calls)
	}
}

func TestCachedRateLimitResponseIsBypassed(t *testing.T) {
	calls := 0
	origin := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		status := http.StatusTooManyRequests
		body := "rate limited"
		if calls == 2 {
			status = http.StatusOK
			body = "success"
		}
		return &http.Response{
			StatusCode: status,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString(body)),
			Request:    req,
		}, nil
	})

	memoryCache := cachelib.NewMemoryCache()
	legacyCache := cachelib.NewTransport(memoryCache)
	legacyCache.Transport = &HeaderOverrideTransport{
		wrappedRT: origin,
		Response: func(resp *http.Response) error {
			resp.Header.Set("Cache-Control", "public, max-age=3600")
			return nil
		},
	}
	legacyClient := &http.Client{Transport: legacyCache}
	response, err := legacyClient.Get("https://ratings-api.uschess.org/test")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()

	cache := cachelib.NewTransport(memoryCache)
	cache.Transport = &HeaderOverrideTransport{
		wrappedRT: origin,
		Response:  cacheResponseFor(time.Hour),
	}
	client := &http.Client{Transport: &BypassCachedRateLimitTransport{wrappedRT: cache}}
	response, err = client.Get("https://ratings-api.uschess.org/test")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("response status = %d; want %d", response.StatusCode, http.StatusOK)
	}
	if calls != 2 {
		t.Fatalf("origin calls = %d; want 2 (the cached 429 must be bypassed)", calls)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

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
		req.Header.Set("User-Agent", internal.UserAgent)
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
