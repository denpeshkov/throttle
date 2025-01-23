package throttle

import (
	"context"
	"crypto/sha1" //nolint:gosec
	_ "embed"
	"encoding/hex"
	"io"
	"strings"
	"sync"
	"time"
)

//go:embed swlog.lua
var swLogScript string

// SWLogLimiter implements a rate limiter using the Sliding-Window Log algorithm.
// It works with a 1 ms resolution.
type SWLogLimiter struct {
	rds    Rediser
	sha    string
	keyTTL time.Duration
	clock  func() time.Time

	mu  sync.Mutex
	lim Limit
}

// NewSWLogLimiter returns a new configured [SWLogLimiter].
func NewSWLogLimiter(rds Rediser, limit Limit, opts ...Option) (*SWLogLimiter, error) {
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
	_, _ = io.WriteString(h, swLogScript)

	return &SWLogLimiter{
		rds:    rds,
		sha:    hex.EncodeToString(h.Sum(nil)),
		keyTTL: options.keyTTL,
		clock:  options.clock,
		lim:    limit,
	}, nil
}

// Allow determines whether the event for the specified key is permitted at the current time.
func (l *SWLogLimiter) Allow(ctx context.Context, key string) (Status, error) {
	l.mu.Lock()
	lim := l.lim
	now := l.clock()
	ttl := max(lim.Interval+l.keyTTL, lim.Interval)
	l.mu.Unlock()

	if lim.Events == 0 {
		return Status{Limited: true, Remaining: 0, Delay: Inf}, nil
	}

	keys := []string{key}
	args := []any{lim.Events, lim.Interval.Milliseconds(), now.UTC().UnixMilli(), ttl.Milliseconds()}
	v, err := l.execScript(ctx, keys, args...)
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
func (l *SWLogLimiter) Limit() Limit {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lim
}

// SetLimit sets a new limit.
func (l *SWLogLimiter) SetLimit(ctx context.Context, newLimit Limit) error {
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
func (l *SWLogLimiter) Reset(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	_, err := l.rds.Del(ctx, keys...)
	return err
}

func (l *SWLogLimiter) execScript(ctx context.Context, keys []string, args ...any) (any, error) {
	v, err := l.rds.EvalSHA(ctx, l.sha, keys, args...)
	if err != nil && strings.HasPrefix(err.Error(), "NOSCRIPT") {
		if _, err := l.rds.ScriptLoad(ctx, swLogScript); err != nil {
			return nil, err
		}
		v, err = l.rds.EvalSHA(ctx, l.sha, keys, args...)
	}
	if err != nil {
		return nil, err
	}
	return v, nil
}
