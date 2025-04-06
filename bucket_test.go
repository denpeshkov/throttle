package throttle

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestInvalidLimit_BucketLimiter(t *testing.T) {
	t.Parallel()

	t.Run("invalid interval", func(t *testing.T) {
		t.Parallel()

		intervals := []time.Duration{-1, 0, 999 * time.Nanosecond}
		for _, in := range intervals {
			lim := Limit{Events: 1, Interval: in}
			l, err := NewBucketLimiter(nil, lim, 1)

			if !errors.Is(err, errInvalidInterval) {
				t.Fatalf("NewBucketLimiter(%v) error %v, want %v", lim, err, errInvalidInterval)
			}
			if err := l.SetLimit(context.Background(), lim); !errors.Is(err, errInvalidInterval) {
				t.Fatalf("SetLimit(%v) error %v, want %v", lim, err, errInvalidInterval)
			}
		}
	})

	t.Run("invalid events", func(t *testing.T) {
		t.Parallel()

		lim := Limit{Events: -1, Interval: 1 * time.Second}
		l, err := NewBucketLimiter(nil, lim, 1)

		if !errors.Is(err, errInvalidEvents) {
			t.Fatalf("NewBucketLimiter(%v) error %v, want %v", lim, err, errInvalidEvents)
		}
		if err := l.SetLimit(context.Background(), lim); !errors.Is(err, errInvalidEvents) {
			t.Fatalf("SetLimit(%v) error %v, want %v", lim, err, errInvalidEvents)
		}
	})
}

func TestInvalidBurst_BucketLimiter(t *testing.T) {
	t.Parallel()

	lim := Limit{Events: 1, Interval: 1 * time.Second}
	if _, err := NewBucketLimiter(nil, lim, -1); err == nil {
		t.Fatalf("NewBucketLimiter(%v, %v) didn't fail", lim, -1)
	}
}
