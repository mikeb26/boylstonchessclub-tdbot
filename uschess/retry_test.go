/* Copyright © 2026 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package uschess

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestRetryTransportHonorsRetryAfter(t *testing.T) {
	var (
		mu     sync.Mutex
		calls  int
		delays []time.Duration
	)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		calls++

		if calls == 1 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = fmt.Fprint(w, "slow down")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer ts.Close()

	rt := &retryTransport{
		next: http.DefaultTransport,
		sleep: func(_ context.Context, d time.Duration) error {
			mu.Lock()
			delays = append(delays, d)
			mu.Unlock()
			return nil
		},
	}

	client := &http.Client{Transport: rt}
	req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected final 200, got %d", resp.StatusCode)
	}

	mu.Lock()
	defer mu.Unlock()
	if calls != 2 {
		t.Fatalf("expected 2 requests, got %d", calls)
	}
	if len(delays) != 1 {
		t.Fatalf("expected 1 retry delay, got %d", len(delays))
	}
	if delays[0] != 2*time.Second {
		t.Fatalf("expected Retry-After delay of 2s, got %v", delays[0])
	}
}

func TestRetryTransportUsesBackoffWhenRetryAfterMissing(t *testing.T) {
	var calls int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprint(w, "try again")
			return
		}
		_, _ = fmt.Fprint(w, "ok")
	}))
	defer ts.Close()

	var delays []time.Duration
	rt := &retryTransport{
		next: http.DefaultTransport,
		sleep: func(_ context.Context, d time.Duration) error {
			delays = append(delays, d)
			return nil
		},
	}

	client := &http.Client{Transport: rt}
	req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected final 200, got %d", resp.StatusCode)
	}
	if calls != 3 {
		t.Fatalf("expected 3 requests, got %d", calls)
	}
	if len(delays) != 2 {
		t.Fatalf("expected 2 retry delays, got %d", len(delays))
	}
	if delays[0] != 250*time.Millisecond || delays[1] != 500*time.Millisecond {
		t.Fatalf("unexpected backoff sequence: %v", delays)
	}
}