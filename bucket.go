package throttle

import (
	"context"
	"crypto/sha1" //nolint:gosec
	_ "embed"     // embed lua script
	"encoding/hex"
	"errors"
	"io"
	"sync"
	"time"
)

//go:embed bucket.lua
var bucketScript string

// BucketLimiter implements a rate limiter using the Token Bucket algorithm.
// It works with a 1ms resolution.
type BucketLimiter struct {
	rds    Rediser
	script script
	keyTTL time.Duration
	clock  func() time.Time

	mu    sync.Mutex
	lim   Limit
	burst int
}

// NewBucketLimiter returns a new configured [BucketLimiter].
func NewBucketLimiter(rds Rediser, limit Limit, burst int, opts ...Option) (*BucketLimiter, error) {
	if err := limit.Valid(); err != nil {
		return nil, err
	}
	if burst < 0 {
		return nil, errors.New("burst is negative")
	}

	options := options{
		keyTTL: 1 * time.Second,
		clock:  time.Now,
	}
	for _, o := range opts {
		o.apply(&options)
	}

	h := sha1.New() //nolint:gosec
	_, _ = io.WriteString(h, bucketScript)

	script := script{
		rds:    rds,
		script: bucketScript,
		sha:    hex.EncodeToString(h.Sum(nil)),
	}

	return &BucketLimiter{
		rds:    rds,
		script: script,
		keyTTL: options.keyTTL,
		clock:  options.clock,
		lim:    limit,
		burst:  burst,
		mu:     sync.Mutex{},
	}, nil
}

// Allow determines whether the event for the specified key is permitted at the current time.
func (l *BucketLimiter) Allow(ctx context.Context, key string) (Status, error) {
	return l.AllowN(ctx, key, 1)
}

// AllowN determines whether n events for the specified key are permitted at the current time.
//
//nolint:forcetypeassert
func (l *BucketLimiter) AllowN(ctx context.Context, key string, n int) (Status, error) {
	l.mu.Lock()
	lim := l.lim
	now := l.clock()
	ttl := max(l.lim.Interval+l.keyTTL, l.lim.Interval)
	burst := l.burst
	l.mu.Unlock()

	if lim.Events == 0 || burst == 0 {
		return Status{Limited: true, Remaining: 0, Delay: Inf}, nil
	}

	keys := []string{key}
	args := []any{lim.Events, lim.Interval.Milliseconds(), now.UTC().UnixMilli(), ttl.Milliseconds(), burst, n}

	v, err := l.script.exec(ctx, keys, args)
	if err != nil {
		return Status{}, err
	}
	values := v.([]any)

	var delay time.Duration
	if v := values[2].(int64); v == -1 {
		delay = Inf
	} else {
		delay = time.Duration(v) * time.Millisecond
	}

	return Status{
		Limited:   values[0].(int64) != 0,
		Remaining: int(values[1].(int64)),
		Delay:     delay,
	}, nil
}

// Limit returns the current limit.
func (l *BucketLimiter) Limit() Limit {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lim
}

// SetLimit sets a new limit.
func (l *BucketLimiter) SetLimit(_ context.Context, newLimit Limit) error {
	if err := newLimit.Valid(); err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.lim = newLimit
	return nil
}

// Burst returns the current burst.
func (l *BucketLimiter) Burst() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.burst
}

// SetBurst sets a new burst.
func (l *BucketLimiter) SetBurst(b int) error {
	if b < 0 {
		return errors.New("burst is negative")
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.burst = b
	return nil
}

// Reset clears all limitations and previous usage for the specified keys.
// If no keys are provided, it's a no-op.
func (l *BucketLimiter) Reset(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	_, err := l.rds.Del(ctx, keys...)
	return err
}
