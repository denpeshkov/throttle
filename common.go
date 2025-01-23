package throttle

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"math"
	"time"
)

// Inf is the infinite duration.
const Inf = time.Duration(math.MaxInt64)

var (
	errInvalidInterval = errors.New("limit interval below 1 ms")
	errInvalidEvents   = errors.New("limit events is negative")
)

// Rediser defines an interface to abstract a Redis client.
// Implementations must be safe for concurrent use by multiple goroutines.
type Rediser interface {
	// ScriptLoad preloads a Lua script into Redis and returns its SHA-1 hash.
	ScriptLoad(ctx context.Context, script string) (string, error)
	// EvalSHA executes a preloaded Lua script using its SHA-1 hash.
	EvalSHA(ctx context.Context, sha1 string, keys []string, args ...any) (any, error)
	// Del removes the specified keys. A key is ignored if it does not exist.
	Del(ctx context.Context, keys ...string) (int64, error)
}

// Limit defines the maximum number of events allowed within a specified time interval.
// Setting Events to zero disallows all events. Interval must be at least 1 millisecond.
type Limit struct {
	Events   int
	Interval time.Duration
}

// Valid validates the Limit.
func (l Limit) Valid() error {
	if l.Interval.Milliseconds() <= 0 {
		return errInvalidInterval
	}
	if l.Events < 0 {
		return errInvalidEvents
	}
	return nil
}

func (l Limit) String() string {
	return fmt.Sprintf("%d req in %s", l.Events, l.Interval.String())
}

// Status represents the result of evaluating the rate limit.
type Status struct {
	// Limited indicates whether the current event was limited.
	Limited bool
	// Remaining specifies the number of events left in the current limit window.
	Remaining int
	// Delay is the duration until the next event is permitted.
	// A zero duration means the event can occur immediately.
	// An [Inf] duration indicates that no events are allowed.
	Delay time.Duration
}

func (s Status) String() string {
	d := s.Delay.String()
	if s.Delay == Inf {
		d = "Inf"
	}
	l := "unlimited"
	if s.Limited {
		l = "limited"
	}
	return fmt.Sprintf("(%s, %d req, %s)", l, s.Remaining, d)
}
