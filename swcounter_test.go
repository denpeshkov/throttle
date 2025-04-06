package throttle

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestInvalidLimit_SWCounterLimiter(t *testing.T) {
	t.Parallel()

	t.Run("invalid interval", func(t *testing.T) {
		t.Parallel()

		intervals := []time.Duration{-1, 0, 999 * time.Nanosecond}
		for _, in := range intervals {
			lim := Limit{Events: 1, Interval: in}
			l, err := NewSWCounterLimiter(nil, lim)

			if !errors.Is(err, errInvalidInterval) {
				t.Fatalf("NewSWCounterLimiter(%v) error %v, want %v", lim, err, errInvalidInterval)
			}
			if err := l.SetLimit(context.Background(), lim); !errors.Is(err, errInvalidInterval) {
				t.Fatalf("SetLimit(%v) error %v, want %v", lim, err, errInvalidInterval)
			}
		}
	})

	t.Run("invalid events", func(t *testing.T) {
		t.Parallel()

		lim := Limit{Events: -1, Interval: 1 * time.Second}
		l, err := NewSWCounterLimiter(nil, lim)

		if !errors.Is(err, errInvalidEvents) {
			t.Fatalf("NewSWCounterLimiter(%v) error %v, want %v", lim, err, errInvalidEvents)
		}
		if err := l.SetLimit(context.Background(), lim); !errors.Is(err, errInvalidEvents) {
			t.Fatalf("SetLimit(%v) error %v, want %v", lim, err, errInvalidEvents)
		}
	})
}
