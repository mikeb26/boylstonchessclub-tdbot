/* Copyright (c) 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file in the current directory for license terms
 */
package s3cache

import (
	"context"
	"fmt"
	"testing"

	"github.com/gregjones/httpcache/test"
	"github.com/mikeb26/boylstonchessclub-tdbot/internal"
)

func TestS3Cache(t *testing.T) {
	// Initialize S3-backed cache
	cache := New(context.Background(), internal.WebCacheBucket, false, true)
	err := cache.Init()
	if err != nil {
		t.Skip(fmt.Sprintf("Skipping test due to lack of access to %v: %v",
			internal.WebCacheBucket, err))
	}

	test.Cache(t, cache)
}

func TestS3CacheWithGzip(t *testing.T) {
	// Initialize S3-backed cache
	cache := New(context.Background(), internal.WebCacheBucket, true, true)
	err := cache.Init()
	if err != nil {
		t.Skip(fmt.Sprintf("Skipping test due to lack of access to %v: %v",
			internal.WebCacheBucket, err))
	}

	test.Cache(t, cache)
}
