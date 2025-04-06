// Based on: https://cs.opensource.google/go/x/time/+/refs/tags/v0.8.0:rate/rate_test.go

package throttletest

import (
	"context"
	"testing"
	"time"

	"github.com/denpeshkov/throttle"
)

func newBucketLimiter(t *testing.T, rds rediser, lim throttle.Limit, burst int, opts ...throttle.Option) *throttle.BucketLimiter {
	t.Helper()

	l, err := throttle.NewBucketLimiter(rds, lim, burst, opts...)
	if err != nil {
		t.Fatalf("NewBucketLimiter(%v, %v) failed %v", lim, burst, err)
	}
	return l
}

const d = 100 * time.Millisecond

//nolint:gochecknoglobals
var (
	t0 = time.Now()
	t1 = t0.Add(time.Duration(1) * d)
	t2 = t0.Add(time.Duration(2) * d)
	t3 = t0.Add(time.Duration(3) * d)
	t4 = t0.Add(time.Duration(4) * d)
	t9 = t0.Add(time.Duration(9) * d)
)

type hit struct {
	t      time.Time
	n      int
	status throttle.Status
}

func TestBucketLimiter_Allow(t *testing.T) {
	t.Cleanup(func() { verifyNoLeaks(t) })

	rds := setupRedis(t)

	t.Run("disallow all events zero", func(t *testing.T) {
		l := newBucketLimiter(t, rds, throttle.Limit{Events: 0, Interval: 1 * time.Second}, 1)

		got := allowN(t, l, 1)
		want := throttle.Status{Limited: true, Remaining: 0, Delay: throttle.Inf}
		if got != want {
			t.Errorf("Allow() = %v, want %v", got, want)
		}
	})

	t.Run("disallow all burst zero", func(t *testing.T) {
		l := newBucketLimiter(t, rds, throttle.Limit{Events: 1, Interval: 1 * time.Second}, 0)

		got := allowN(t, l, 1)
		want := throttle.Status{Limited: true, Remaining: 0, Delay: throttle.Inf}
		if got != want {
			t.Errorf("Allow() = %v, want %v", got, want)
		}
	})

	t.Run("burst 1", func(t *testing.T) {
		clock := newClock()

		l := newBucketLimiter(t, rds,
			throttle.Limit{Events: 10, Interval: 1 * time.Second}, 1,
			throttle.WithClock(clock.time))

		allows := []hit{
			{t0, 1, throttle.Status{Limited: false, Remaining: 0, Delay: d}},
			{t0, 1, throttle.Status{Limited: true, Remaining: 0, Delay: d}},
			{t0, 1, throttle.Status{Limited: true, Remaining: 0, Delay: d}},
			{t1, 1, throttle.Status{Limited: false, Remaining: 0, Delay: d}},
			{t1, 1, throttle.Status{Limited: true, Remaining: 0, Delay: d}},
			{t1, 1, throttle.Status{Limited: true, Remaining: 0, Delay: d}},
			{t2, 2, throttle.Status{Limited: true, Remaining: 1, Delay: throttle.Inf}}, // burst size is 1, so n=2 always fails
			{t2, 1, throttle.Status{Limited: false, Remaining: 0, Delay: d}},
			{t2, 1, throttle.Status{Limited: true, Remaining: 0, Delay: d}},
		}

		for i, allow := range allows {
			clock.set(allow.t)
			s := allowN(t, l, allow.n)
			if s != allow.status {
				t.Errorf("step %d: AllowN(%v) = %v want %v", i, allow.n, s, allow.status)
			}
		}
	})

	t.Run("burst 3", func(t *testing.T) {
		clock := new(clock)
		clock.set(time.Now())
		l := newBucketLimiter(t, rds,
			throttle.Limit{Events: 10, Interval: 1 * time.Second}, 3,
			throttle.WithClock(clock.time))

		allows := []hit{
			{t0, 2, throttle.Status{Limited: false, Remaining: 1, Delay: 0}},
			{t0, 2, throttle.Status{Limited: true, Remaining: 1, Delay: d}},
			{t0, 1, throttle.Status{Limited: false, Remaining: 0, Delay: d}},
			{t0, 1, throttle.Status{Limited: true, Remaining: 0, Delay: d}},
			{t1, 4, throttle.Status{Limited: true, Remaining: 1, Delay: throttle.Inf}},
			{t2, 1, throttle.Status{Limited: false, Remaining: 1, Delay: 0}},
			{t3, 1, throttle.Status{Limited: false, Remaining: 1, Delay: 0}},
			{t4, 1, throttle.Status{Limited: false, Remaining: 1, Delay: 0}},
			{t4, 1, throttle.Status{Limited: false, Remaining: 0, Delay: d}},
			{t4, 1, throttle.Status{Limited: true, Remaining: 0, Delay: d}},
			{t4, 1, throttle.Status{Limited: true, Remaining: 0, Delay: d}},
			{t9, 3, throttle.Status{Limited: false, Remaining: 0, Delay: 3 * d}},
			{t9, 0, throttle.Status{Limited: false, Remaining: 0, Delay: 0}},
		}

		for i, allow := range allows {
			clock.set(allow.t)
			s := allowN(t, l, allow.n)
			if s != allow.status {
				t.Errorf("step %d: AllowN(%v) = %v want %v", i, allow.n, s, allow.status)
			}
		}
	})

	t.Run("jump backwards", func(t *testing.T) {
		clock := new(clock)
		clock.set(time.Now())
		l := newBucketLimiter(t, rds,
			throttle.Limit{Events: 10, Interval: 1 * time.Second}, 3,
			throttle.WithClock(clock.time))

		allows := []hit{
			{t1, 1, throttle.Status{Limited: false, Remaining: 2, Delay: 0}}, // start at t1
			{t0, 1, throttle.Status{Limited: false, Remaining: 1, Delay: 0}}, // jump back to t0, two tokens remain
			{t0, 1, throttle.Status{Limited: false, Remaining: 0, Delay: d}},
			{t0, 1, throttle.Status{Limited: true, Remaining: 0, Delay: d}},
			{t0, 1, throttle.Status{Limited: true, Remaining: 0, Delay: d}},
			{t1, 1, throttle.Status{Limited: false, Remaining: 0, Delay: d}}, // got a token
			{t1, 1, throttle.Status{Limited: true, Remaining: 0, Delay: d}},
			{t1, 1, throttle.Status{Limited: true, Remaining: 0, Delay: d}},
			{t2, 1, throttle.Status{Limited: false, Remaining: 0, Delay: d}}, // got another token
			{t2, 1, throttle.Status{Limited: true, Remaining: 0, Delay: d}},
			{t2, 1, throttle.Status{Limited: true, Remaining: 0, Delay: d}},
		}

		for i, allow := range allows {
			clock.set(allow.t)
			s := allowN(t, l, allow.n)
			if s != allow.status {
				t.Errorf("step %d: AllowN(%v) = %v want %v", i, allow.n, s, allow.status)
			}
		}
	})
}

func TestBucketLimiter_Reset(t *testing.T) {
	t.Cleanup(func() { verifyNoLeaks(t) })

	rds := setupRedis(t)

	tests := []struct {
		name   string
		events int
	}{
		{"limited", 1},
		{"unlimited", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := newBucketLimiter(t, rds, throttle.Limit{Events: 1, Interval: 1 * time.Second}, 1)

			// First event.
			s1 := allowN(t, l, 1)
			// After Reset() second event should return the same status as the first event.
			if err := l.Reset(context.Background(), t.Name()); err != nil {
				t.Fatalf("Reset() failed %v", err)
			}
			s2 := allowN(t, l, 1)
			if s1 != s2 {
				t.Errorf("Limit not reset. Allow() = %v, want %v", s2, s1)
			}
		})
	}

	t.Run("no keys", func(t *testing.T) {
		l := newBucketLimiter(t, rds, throttle.Limit{Events: 1, Interval: 1 * time.Second}, 1)
		if err := l.Reset(context.Background()); err != nil {
			t.Fatalf("Reset() with no keys failed: %v", err)
		}
	})
}

func TestBucketLimiter_KeyTTL(t *testing.T) {
	t.Cleanup(func() { verifyNoLeaks(t) })

	rds := setupRedis(t)

	l := newBucketLimiter(t, rds,
		throttle.Limit{Events: 1, Interval: 1 * time.Millisecond}, 1,
		throttle.WithKeyTTL(0))

	s1 := allowN(t, l, 1)

	time.Sleep(10 * time.Millisecond) // Wait for the keyTTL duration to allow the key to expire.

	s2 := allowN(t, l, 1)
	if s2 != s1 {
		t.Errorf("Allow() = %v, want %v after key expiration", s2, s1)
	}
}
