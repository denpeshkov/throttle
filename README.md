# Throttle

[![CI](https://github.com/denpeshkov/throttle/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/denpeshkov/throttle/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/denpeshkov/throttle)](https://goreportcard.com/report/github.com/denpeshkov/throttle)
[![Go Reference](https://pkg.go.dev/badge/github.com/denpeshkov/throttle.svg)](https://pkg.go.dev/github.com/denpeshkov/throttle)

A Go library that implements a rate limiter using various algorithms, backed by [Redis](https://redis.io).

Currently implemented rate-limiting algorithms:
- Sliding Window Log
- Sliding Window Counter
- Token Bucket

For an overview of the pros and cons of different rate-limiting algorithms, you can check out this reference: [How to Design a Scalable Rate Limiting Algorithm with Kong API](https://konghq.com/blog/engineering/how-to-design-a-scalable-rate-limiting-algorithm)

# Usage

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/denpeshkov/throttle"
	"github.com/redis/go-redis/v9"
)

// Redis is a wrapper for the go-redis client, implementing the throttle.Rediser interface.
type Redis struct {
	rdb *redis.Client
}

func (r Redis) ScriptLoad(ctx context.Context, script string) (string, error) {
	return r.rdb.ScriptLoad(ctx, script).Result()
}
func (r Redis) EvalSHA(ctx context.Context, sha1 string, keys []string, args ...any) (any, error) {
	return r.rdb.EvalSha(ctx, sha1, keys, args...).Result()
}
func (r Redis) Del(ctx context.Context, keys ...string) (int64, error) {
	return r.rdb.Del(ctx, keys...).Result()
}

func main() {
	// Create a Redis client
	rdb := Redis{
		rdb: redis.NewClient(&redis.Options{Addr: "localhost:6379"}),
	}

	limit := throttle.Limit{
		Events:   5,               // Allow 5 requests
		Interval: 1 * time.Minute, // Per minute
	}

	// Create a Sliding-Window Log rate limiter
	limiter, err := throttle.NewSWLogLimiter(rdb, limit)
	if err != nil {
		log.Fatalf("failed to create limiter: %v", err)
	}

	const key = "user:123"
	status, err := limiter.Allow(context.Background(), key)
	if err != nil {
		log.Fatalf("failed to check rate limit: %v", err)
	}

	// Whether the request was limited.
	_ = status.Limited
	// The number of requests remaining in the current rate limit window
	_ = status.Remaining
	// The duration until the next event is permitted.
	_ = status.Delay
}
```