/* Copyright (c) 2013 The s3cache AUTHORS. All rights reserved.
 * Copyright (c) 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file in the current directory for license terms
 *
 * Package s3cache provides an implementation of httpcache.Cache that stores and
 * retrieves data using Amazon S3. It is based on the original
 * github.com/sourcegraph/s3cache but updated to use the more modern
 * aws-sdk-go-v2 and golang standard library functions
 */
package s3cache

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

// Cache objects store and retrieve data using Amazon S3.
type Cache struct {
	// Config is the Amazon S3 configuration.
	Config aws.Config

	// Client is the s3 client the cache should used when interacting with S3.
	// By default this is initialized in Init() with the default Config, but
	// callers can optionally override this with their own s3 client if desired.
	Client *s3.Client

	// bucketName is the name of the S3 bucket in Amazon S3
	// bucket name and the AWS region. Example: "mybucket".
	bucketName string

	// gzip indicates whether cache entries should be gzipped in Set and
	// gunzipped in Get. If true, cache entry keys will have the suffix ".gz"
	// appended.
	gzip bool

	// LogErrors controls whether errors should be logged or not
	logErrors bool

	// The context to specify when initiating s3 requests
	ctx context.Context
}

func (c *Cache) Get(key string) ([]byte, bool) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(c.bucketName),
		Key:    aws.String(c.cacheKeyToObjectKey(key)),
	}

	resp, err := c.Client.GetObject(c.ctx, input)
	if err != nil {
		if c.logErrors {
			var apiErr smithy.APIError
			// no such key just indicates a cache miss
			if !(errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchKey") {
				log.Printf("s3cache.get: failed to get object %v%v: %v", *input.Bucket,
					*input.Key, err)
			}
		}
		return []byte{}, false
	}
	defer resp.Body.Close()

	rdr := resp.Body
	if c.gzip {
		rdr, err = gzip.NewReader(rdr)
		if err != nil {
			if c.logErrors {
				log.Printf("s3cache.get: failed to open compressed object %v%v: %v",
					*input.Bucket, *input.Key, err)
			}
			return nil, false
		}

		defer rdr.Close()
	}
	data, err := io.ReadAll(rdr)
	if err != nil {
		if c.logErrors {
			log.Printf("s3cache.get: failed to read object %v%v: %v",
				*input.Bucket, *input.Key, err)
		}
	}

	return data, err == nil
}

// Set stores the provided data in the cache under the given key.
func (c *Cache) Set(key string, data []byte) {
	input := &s3.PutObjectInput{
		Bucket: aws.String(c.bucketName),
		Key:    aws.String(c.cacheKeyToObjectKey(key)),
		Body:   bytes.NewReader(data),
	}

	if c.gzip {
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		if _, err := gw.Write(data); err != nil {
			if c.logErrors {
				log.Printf("s3cache.set: failed to gzip data for %v%v: %v",
					*input.Bucket, *input.Key, err)
			}
			return
		}
		if err := gw.Close(); err != nil {
			if c.logErrors {
				log.Printf("s3cache.set: failed to close gzip writer for %v%v: %v",
					*input.Bucket, *input.Key, err)
			}
			return
		}
		input.Body = &buf
		input.ContentEncoding = aws.String("gzip")
	}

	_, err := c.Client.PutObject(c.ctx, input)
	if err != nil {
		if c.logErrors {
			log.Printf("s3cache.set: put failed for %v%v: %v", *input.Bucket,
				*input.Key, err)
		}
	}
}

func (c *Cache) Delete(key string) {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucketName),
		Key:    aws.String(c.cacheKeyToObjectKey(key)),
	}

	_, err := c.Client.DeleteObject(c.ctx, input)
	if err != nil {
		if c.logErrors {
			log.Printf("s3cache.delete: delete failed: %v", err)
		}
	}
}

func (c *Cache) cacheKeyToObjectKey(key string) string {
	const PathPrefix = "s3cache"

	h := md5.New()
	io.WriteString(h, key)
	objKey := fmt.Sprintf("/%v/%v", PathPrefix, hex.EncodeToString(h.Sum(nil)))
	if c.gzip {
		objKey += ".gz"
	}

	return objKey
}

// New returns a new Cache with underlying storage in the specified Amazon S3
// bucket. Additionally, specify whether objects persisted in the cache should
// be compressed with gzip or not. Callers should take care to invoke Init() on
// the returned Cache object before use
func New(ctxIn context.Context, bucketNameIn string, gzipIn bool,
	logErrorsIn bool) *Cache {

	return &Cache{
		ctx:        ctxIn,
		bucketName: bucketNameIn,
		gzip:       gzipIn,
		logErrors:  logErrorsIn,
	}
}

// The default configuration sources are:
// * Environment Variables (e.g. AWS_ACCESS_KEY_ID and AWS_SECRET_KEY)
// * Shared Configuration and Shared Credentials files.
// To use different credentials, modify the returned Cache object's
// Config and Client fields.
func (c *Cache) Init() error {
	var err error
	c.Config, err = config.LoadDefaultConfig(c.ctx)
	if err != nil {
		return fmt.Errorf("s3cache.init: failed to load AWS config: %w", err)
	}
	c.Client = s3.NewFromConfig(c.Config)

	// Permission check: verify bucket exists and is accessible
	if _, err = c.Client.HeadBucket(c.ctx, &s3.HeadBucketInput{
		Bucket: aws.String(c.bucketName),
	}); err != nil {
		return fmt.Errorf("s3cache.init: head bucket failed for %s: %w", c.bucketName, err)
	}

	// Permission check: verify ability to list objects (read/list permissions)
	if _, err = c.Client.ListObjectsV2(c.ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(c.bucketName),
		MaxKeys: aws.Int32(1),
	}); err != nil {
		return fmt.Errorf("s3cache.init: list objects failed for %s: %w", c.bucketName, err)
	}

	return nil
}
