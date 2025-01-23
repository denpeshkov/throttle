package throttletest

import (
	"context"
	"testing"
	"time"

	"github.com/denpeshkov/throttle"
)

func newSWCounterLimiter(t *testing.T, rds rediser, lim throttle.Limit, opts ...throttle.Option) *throttle.SWCounterLimiter {
	t.Helper()

	l, err := throttle.NewSWCounterLimiter(rds, lim, opts...)
	if err != nil {
		t.Fatalf("NewSWCounterLimiter(%v) failed %v", lim, err)
	}
	return l
}

func TestSWCounterLimiter_Allow(t *testing.T) {
	t.Cleanup(func() { verifyNoLeaks(t) })

	rds := setupRedis(t)

	t.Run("disallow all", func(t *testing.T) {
		clock := newClock()

		l := newSWCounterLimiter(t, rds,
			throttle.Limit{Events: 0, Interval: 1 * time.Second},
			throttle.WithClock(clock.time))

		got := allow(t, l)
		want := throttle.Status{Limited: true, Remaining: 0, Delay: throttle.Inf}
		if got != want {
			t.Errorf("Allow() = %v, want %v", got, want)
		}
	})

	t.Run("basic", func(t *testing.T) {
		limits := []throttle.Limit{
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

		for _, lim := range limits {
			t.Run(lim.String(), func(t *testing.T) {
				// We want clock to be at the start of the window.
				clock := newClock()
				clock.set(time.Now().Truncate(lim.Interval))

				t.Logf("now: %v", clock)

				l := newSWCounterLimiter(t, rds, lim, throttle.WithClock(clock.time))

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
					t.Errorf("Allow() = %v, want (unlimited, 0 req, positive delay)", s)
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

	t.Run("manual", func(t *testing.T) {
		now := time.Now().Truncate(24 * time.Hour)

		clock := newClock()

		type hit struct {
			t      time.Time
			status throttle.Status
		}
		tests := []struct {
			lim  throttle.Limit
			hits []hit
		}{
			{
				lim: throttle.Limit{Events: 1, Interval: 5 * time.Second},
				hits: []hit{
					{t: now, status: throttle.Status{Limited: false, Remaining: 0, Delay: 5 * time.Second}},
					{t: now.Add(5*time.Second - 1*time.Millisecond), status: throttle.Status{Limited: true, Remaining: 0, Delay: 1 * time.Millisecond}},
					{t: now.Add(5*time.Second + 1*time.Millisecond), status: throttle.Status{Limited: false, Remaining: 0, Delay: 5*time.Second - 1*time.Millisecond}},
				},
			},
			{
				lim: throttle.Limit{Events: 2, Interval: 4 * time.Second},
				hits: []hit{
					{t: now, status: throttle.Status{Limited: false, Remaining: 1, Delay: 0}},
					{t: now.Add(1 * time.Millisecond), status: throttle.Status{Limited: false, Remaining: 0, Delay: 4*time.Second - 1*time.Millisecond}},
					{t: now.Add(4*time.Second - 1*time.Millisecond), status: throttle.Status{Limited: true, Remaining: 0, Delay: 1 * time.Millisecond}},
					{t: now.Add(4*time.Second + 1*time.Millisecond), status: throttle.Status{Limited: false, Remaining: 0, Delay: 2*time.Second - 1*time.Millisecond}},
				},
			},
			{
				lim: throttle.Limit{Events: 1, Interval: 1 * time.Second},
				hits: []hit{
					{t: now.Add(1*time.Second - 1*time.Millisecond), status: throttle.Status{Limited: false, Remaining: 0, Delay: 1 * time.Millisecond}},
					{t: now.Add(1*time.Second + 1*time.Millisecond), status: throttle.Status{Limited: false, Remaining: 0, Delay: 1*time.Second - 1*time.Millisecond}},
				},
			},
		}

		for i, tt := range tests {
			l := newSWCounterLimiter(t, rds, tt.lim, throttle.WithClock(clock.time))
			for j, h := range tt.hits {
				clock.set(h.t)
				got := allow(t, l)
				if got != h.status {
					t.Errorf("case=%d hit=%d; Allow() = %v, want %v", i, j, got, h.status)
				}
			}
		}
	})
}

func TestSWCounterLimiter_SetLimit(t *testing.T) {
	t.Cleanup(func() { verifyNoLeaks(t) })

	rds := setupRedis(t)

	clock := newClock()
	clock.set(time.Now().Truncate(24 * time.Hour))

	lim := throttle.Limit{Events: 1, Interval: 1 * time.Second}
	l := newSWCounterLimiter(t, rds, lim, throttle.WithClock(clock.time))

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

func TestSWCounterLimiter_Reset(t *testing.T) {
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

			l := newSWCounterLimiter(t, rds,
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

		l := newSWCounterLimiter(t, rds,
			throttle.Limit{Events: 1, Interval: 1 * time.Second},
			throttle.WithClock(clock.time))
		if err := l.Reset(context.Background()); err != nil {
			t.Fatalf("Reset() with no keys failed %v", err)
		}
	})
}

func TestSWCounterLimiter_KeyTTL(t *testing.T) {
	t.Cleanup(func() { verifyNoLeaks(t) })

	rds := setupRedis(t)

	clock := newClock()

	l := newSWCounterLimiter(t, rds,
		throttle.Limit{Events: 1, Interval: 1 * time.Millisecond},
		throttle.WithClock(clock.time), throttle.WithKeyTTL(0))

	s1 := allow(t, l)

	time.Sleep(10 * time.Millisecond) // Wait for the keyTTL duration to allow the key to expire.

	s2 := allow(t, l)
	if s2 != s1 {
		t.Errorf("Allow() = %v, want %v after key expiration", s2, s1)
	}
}
