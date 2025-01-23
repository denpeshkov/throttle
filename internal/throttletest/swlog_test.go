package throttletest

import (
	"context"
	"testing"
	"time"

	"github.com/denpeshkov/throttle"
)

func newSWLogLimiter(t *testing.T, rds rediser, lim throttle.Limit, opts ...throttle.Option) *throttle.SWLogLimiter {
	t.Helper()

	l, err := throttle.NewSWLogLimiter(rds, lim, opts...)
	if err != nil {
		t.Fatalf("NewSWLogLimiter(%v) failed %v", lim, err)
	}
	return l
}

func TestSWLogLimiter_Allow(t *testing.T) {
	t.Cleanup(func() { verifyNoLeaks(t) })

	rds := setupRedis(t)

	t.Run("disallow all", func(t *testing.T) {
		clock := newClock()

		l := newSWLogLimiter(t, rds,
			throttle.Limit{Events: 0, Interval: 1 * time.Second},
			throttle.WithClock(clock.time))

		got := allow(t, l)
		want := throttle.Status{Limited: true, Remaining: 0, Delay: infTTL}
		if got != want {
			t.Errorf("Allow() = %v, want %v", got, want)
		}
	})

	t.Run("basic", func(t *testing.T) {
		lims := []throttle.Limit{
			{Events: 1, Interval: 10 * time.Millisecond},
			{Events: 2, Interval: 10 * time.Millisecond},
			{Events: 10, Interval: 10 * time.Millisecond},
			{Events: 1, Interval: 999 * time.Millisecond},
			{Events: 2, Interval: 999 * time.Millisecond},
			{Events: 10, Interval: 999 * time.Millisecond},
			{Events: 1, Interval: 1 * time.Second},
			{Events: 2, Interval: 1 * time.Second},
			{Events: 10, Interval: 1 * time.Second},
		}

		for _, lim := range lims {
			t.Run(lim.String(), func(t *testing.T) {
				clock := newClock()
				t.Logf("now: %v", clock)

				l := newSWLogLimiter(t, rds, lim, throttle.WithClock(clock.time))

				// Hit until last unlimited event.
				want := throttle.Status{Limited: false, Remaining: lim.Events - 1, Delay: 0}
				for want.Remaining > 0 {
					got := allow(t, l)
					if got != want {
						t.Errorf("Allow() = %v, want %v", got, want)
					}
					want.Remaining--
					clock.add(delta)
				}

				// Last unlimited event should have some positive delay.
				s := allow(t, l)
				if s.Limited || s.Delay <= 0 || s.Remaining > 0 {
					t.Errorf("Allow(%v) = %v, want (unlimited, 0 req, positive delay)", clock, s)
				}

				// These events should be limited.
				stopTime := clock.time().Add(s.Delay)
				clock.add(delta)
				want = throttle.Status{Limited: true, Remaining: 0, Delay: s.Delay - delta}
				for clock.time().Before(stopTime) {
					got := allow(t, l)
					if got != want {
						t.Errorf("Allow() = %v, want %v", got, want)
					}
					want.Delay -= delta
					clock.add(delta)
				}
			})
		}
	})

	t.Run("delta exceeds interval", func(t *testing.T) {
		lims := []throttle.Limit{
			{Events: 1, Interval: 1 * time.Millisecond},
			{Events: 2, Interval: 1 * time.Millisecond},
			{Events: 1, Interval: 1 * time.Second},
			{Events: 2, Interval: 1 * time.Second},
		}

		for _, lim := range lims {
			t.Run(lim.String(), func(t *testing.T) {
				clock := newClock()
				t.Logf("now: %v", clock)

				l := newSWLogLimiter(t, rds, lim, throttle.WithClock(clock.time))

				// First hit.
				s1 := allow(t, l)

				// Second hit. Limit should be reset.
				clock.add(lim.Interval + 1*time.Millisecond)
				s2 := allow(t, l)
				if s2 != s1 {
					t.Errorf("Limit not reset. Allow() = %v, want %v", s2, s1)
				}
			})
		}
	})
}

func TestSWLogLimiter_SetLimit(t *testing.T) {
	t.Cleanup(func() { verifyNoLeaks(t) })

	rds := setupRedis(t)

	clock := newClock()

	lim := throttle.Limit{Events: 1, Interval: 1 * time.Second}
	l := newSWLogLimiter(t, rds, lim, throttle.WithClock(clock.time))

	if err := l.SetLimit(context.Background(), lim); err != nil {
		t.Fatalf("SetLimit(%v) failed %v", lim, err)
	}
	if l.Limit() != lim {
		t.Errorf("SetLimit(%v) failed to set limit; Limit() = %v, want %v", lim, l.Limit(), lim)
	}

	lim = throttle.Limit{Events: 5, Interval: 10 * time.Second}
	if err := l.SetLimit(context.Background(), lim); err != nil {
		t.Fatalf("SetLimit(%v) failed %v", lim, err)
	}
	if l.Limit() != lim {
		t.Errorf("SetLimit(%v) failed to set limit; Limit() = %v, want %v", lim, l.Limit(), lim)
	}
}

func TestSWLogLimiter_Reset(t *testing.T) {
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
			clock := newClock()

			l := newSWLogLimiter(t, rds,
				throttle.Limit{Events: 1, Interval: 1 * time.Second},
				throttle.WithClock(clock.time))

			// First event.
			s1 := allow(t, l)

			// After Reset() second event should return the same status as the first event.
			if err := l.Reset(context.Background(), t.Name()); err != nil {
				t.Fatalf("Reset() failed %v", err)
			}
			s2 := allow(t, l)
			if s1 != s2 {
				t.Errorf("Limit not reset. Allow() = %v, want %v", s2, s1)
			}
		})
	}

	t.Run("no keys", func(t *testing.T) {
		clock := newClock()

		l := newSWLogLimiter(t, rds,
			throttle.Limit{Events: 1, Interval: 1 * time.Second},
			throttle.WithClock(clock.time))
		if err := l.Reset(context.Background()); err != nil {
			t.Fatalf("Reset() with no keys failed %v", err)
		}
	})
}

func TestSWLogLimiter_KeyTTL(t *testing.T) {
	t.Cleanup(func() { verifyNoLeaks(t) })

	rds := setupRedis(t)

	clock := newClock()

	l := newSWLogLimiter(t, rds,
		throttle.Limit{Events: 1, Interval: 1 * time.Millisecond},
		throttle.WithClock(clock.time),
		throttle.WithKeyTTL(0))

	s1 := allow(t, l)

	time.Sleep(10 * time.Millisecond) // Wait for the keyTTL duration to allow the key to expire.

	s2 := allow(t, l)
	if s2 != s1 {
		t.Errorf("Allow() = %v, want %v after key expiration", s2, s1)
	}
}
