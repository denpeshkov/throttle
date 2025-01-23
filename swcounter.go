package throttle

import (
	"context"
	"crypto/sha1" //nolint:gosec
	_ "embed"
	"encoding/hex"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed swcounter.lua
var swCounterScript string

// SWCounterLimiter implements a rate limiter using the Sliding-Window Counter algorithm.
// It works with a 1ms resolution.
type SWCounterLimiter struct {
	rds    Rediser
	sha    string
	keyTTL time.Duration
	clock  func() time.Time

	mu  sync.Mutex
	lim Limit
}

// NewSWCounterLimiter returns a new configured [SWCounterLimiter].
func NewSWCounterLimiter(rds Rediser, limit Limit, opts ...Option) (*SWCounterLimiter, error) {
	if err := limit.Valid(); err != nil {
		return nil, err
	}

	options := options{
		keyTTL: 1 * time.Second,
		clock:  time.Now,
	}
	for _, o := range opts {
		o.apply(&options)
	}

	h := sha1.New() //nolint:gosec
	_, _ = io.WriteString(h, swCounterScript)

	return &SWCounterLimiter{
		rds:    rds,
		sha:    hex.EncodeToString(h.Sum(nil)),
		keyTTL: options.keyTTL,
		clock:  options.clock,
		lim:    limit,
	}, nil
}

// Allow determines whether the event for the specified key is permitted at the current time.
func (l *SWCounterLimiter) Allow(ctx context.Context, key string) (Status, error) {
	l.mu.Lock()
	lim := l.lim
	now := l.clock().UTC().UnixMilli()
	interval := lim.Interval.Milliseconds()
	prevWindow, curWindow := strconv.FormatInt((now-interval)/interval, 10), strconv.FormatInt(now/interval, 10)
	ttl := max(2*lim.Interval+l.keyTTL, 2*lim.Interval)
	l.mu.Unlock()

	if lim.Events == 0 {
		return Status{Limited: true, Remaining: 0, Delay: Inf}, nil
	}

	keys := []string{key, prevWindow, curWindow}
	args := []any{lim.Events, interval, now, ttl.Milliseconds()}

	v, err := l.execScript(ctx, keys, args)
	if err != nil {
		return Status{}, err
	}
	values := v.([]interface{})
	return Status{
		Limited:   values[0].(int64) != 0,
		Remaining: int(values[1].(int64)),
		Delay:     time.Duration(values[2].(int64)) * time.Millisecond,
	}, nil
}

// Limit returns the current limit.
func (l *SWCounterLimiter) Limit() Limit {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lim
}

// SetLimit sets a new limit.
func (l *SWCounterLimiter) SetLimit(ctx context.Context, newLimit Limit) error {
	if err := newLimit.Valid(); err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.lim = newLimit
	return nil
}

// Reset clears all limitations and previous usage for the specified keys.
// If no keys are provided, it's a no-op.
func (l *SWCounterLimiter) Reset(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	_, err := l.rds.Del(ctx, keys...)
	return err
}

func (l *SWCounterLimiter) execScript(ctx context.Context, keys []string, args ...any) (any, error) {
	v, err := l.rds.EvalSHA(ctx, l.sha, keys, args...)
	if err != nil && strings.HasPrefix(err.Error(), "NOSCRIPT") {
		if _, err := l.rds.ScriptLoad(ctx, swCounterScript); err != nil {
			return nil, err
		}
		v, err = l.rds.EvalSHA(ctx, l.sha, keys, args...)
	}
	if err != nil {
		return nil, err
	}
	return v, nil
}
