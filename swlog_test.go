package throttle

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestInvalidLimit_SWLogLimiter(t *testing.T) {
	t.Parallel()

	t.Run("invalid interval", func(t *testing.T) {
		t.Parallel()

		intervals := []time.Duration{-1, 0, 999 * time.Nanosecond}
		for _, in := range intervals {
			lim := Limit{Events: 1, Interval: in}
			l, err := NewSWLogLimiter(nil, lim)

			if !errors.Is(err, errInvalidInterval) {
				t.Fatalf("NewSWLogLimiter(%s) error %v, want %v", lim, err, errInvalidInterval)
			}
			if err := l.SetLimit(context.Background(), lim); !errors.Is(err, errInvalidInterval) {
				t.Fatalf("SetLimit(%s) error %v, want %v", lim, err, errInvalidInterval)
			}
		}
	})

	t.Run("invalid events", func(t *testing.T) {
		t.Parallel()

		lim := Limit{Events: -1, Interval: 1 * time.Second}
		l, err := NewSWLogLimiter(nil, lim)

		if !errors.Is(err, errInvalidEvents) {
			t.Fatalf("NewSWLogLimiter(%s) error %v, want %v", lim, err, errInvalidEvents)
		}
		if err := l.SetLimit(context.Background(), lim); !errors.Is(err, errInvalidEvents) {
			t.Fatalf("SetLimit(%s) error %v, want %v", lim, err, errInvalidEvents)
		}
	})
}
