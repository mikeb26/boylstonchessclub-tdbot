/* Copyright © 2026 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package uschess

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	defaultMaxAttempts = 4
	defaultBaseDelay   = 1 * time.Second
	defaultMaxDelay    = 30 * time.Second
)

func newRetryingClient(base *http.Client) *http.Client {
	if base == nil {
		base = http.DefaultClient
	}

	transport := base.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	return &http.Client{
		Transport: &retryTransport{
			next:        transport,
			maxAttempts: defaultMaxAttempts,
			baseDelay:   defaultBaseDelay,
			maxDelay:    defaultMaxDelay,
			sleep:       sleepContext,
			jitter:      fullJitter,
		},
		Timeout:       base.Timeout,
		Jar:           base.Jar,
		CheckRedirect: base.CheckRedirect,
	}
}

type retryTransport struct {
	next        http.RoundTripper
	maxAttempts int
	baseDelay   time.Duration
	maxDelay    time.Duration
	sleep       func(context.Context, time.Duration) error
	jitter      func(time.Duration) time.Duration
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	next := t.next
	if next == nil {
		next = http.DefaultTransport
	}
	maxAttempts := t.maxAttempts
	if maxAttempts <= 0 {
		maxAttempts = defaultMaxAttempts
	}
	baseDelay := t.baseDelay
	if baseDelay <= 0 {
		baseDelay = defaultBaseDelay
	}
	maxDelay := t.maxDelay
	if maxDelay <= 0 {
		maxDelay = defaultMaxDelay
	}
	sleep := t.sleep
	if sleep == nil {
		sleep = sleepContext
	}
	jitter := t.jitter
	if jitter == nil {
		jitter = fullJitter
	}

	for attempt := 1; ; attempt++ {
		reqCopy := req.Clone(req.Context())
		resp, err := next.RoundTrip(reqCopy)

		if !shouldRetry(reqCopy.Method, resp, err) {
			return resp, err
		}

		if attempt >= maxAttempts {
			if err != nil {
				return nil, err
			}
			return resp, nil
		}

		delay := retryDelay(resp, attempt, baseDelay, maxDelay, jitter)
		if resp != nil && resp.Body != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
		if err := sleep(req.Context(), delay); err != nil {
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}
			return nil, err
		}
	}
}

func shouldRetry(method string, resp *http.Response, err error) bool {
	if !isRetryableMethod(method) {
		return false
	}

	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false
		}
		var netErr net.Error
		if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
			return true
		}
		return true
	}

	if resp == nil {
		return false
	}

	switch resp.StatusCode {
	case http.StatusTooManyRequests,
		http.StatusRequestTimeout,
		http.StatusTooEarly,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func isRetryableMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

func retryDelay(resp *http.Response, attempt int, baseDelay, maxDelay time.Duration,
	jitter func(time.Duration) time.Duration) time.Duration {
	if resp != nil {
		if delay, ok := parseRetryAfter(resp.Header.Get("Retry-After")); ok {
			if delay > maxDelay {
				return maxDelay
			}
			if delay < 0 {
				return 0
			}
			return delay
		}
	}

	delay := baseDelay
	for i := 1; i < attempt; i++ {
		if delay > maxDelay/2 {
			return maxDelay
		}
		delay *= 2
	}
	if delay > maxDelay {
		return maxDelay
	}
	if jitter != nil {
		delay = jitter(delay)
	}
	return delay
}

func parseRetryAfter(value string) (time.Duration, bool) {
	if value == "" {
		return 0, false
	}

	if secs, err := strconv.Atoi(value); err == nil {
		if secs < 0 {
			return 0, true
		}
		return time.Duration(secs) * time.Second, true
	}

	if deadline, err := http.ParseTime(value); err == nil {
		return time.Until(deadline), true
	}

	return 0, false
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

var (
	jitterRandMu sync.Mutex
	jitterRand   = rand.New(rand.NewSource(time.Now().UnixNano()))
)

func fullJitter(maxDelay time.Duration) time.Duration {
	if maxDelay <= 0 {
		return 0
	}

	jitterRandMu.Lock()
	n := jitterRand.Int63n(int64(maxDelay) + 1)
	jitterRandMu.Unlock()

	return time.Duration(n)
}
