package throttletest

import (
	"context"
	"math"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/denpeshkov/throttle"
)

const (
	delta  = 1 * time.Millisecond // Limiter resolution.
	infTTL = time.Duration(math.MaxInt64)
)

func allow(
	t *testing.T,
	l interface {
		Allow(context.Context, string) (throttle.Status, error)
	},
) throttle.Status {
	t.Helper()

	s, err := l.Allow(context.Background(), t.Name())
	if err != nil {
		t.Fatalf("Allow() failed %v", err)
	}
	return s
}

func allowN(
	t *testing.T,
	l interface {
		AllowN(context.Context, string, int) (throttle.Status, error)
	},
	n int,
) throttle.Status {
	t.Helper()

	s, err := l.AllowN(context.Background(), t.Name(), n)
	if err != nil {
		t.Fatalf("AllowN(%v) failed %v", n, err)
	}
	return s
}

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func newClock() *clock {
	return &clock{t: time.Now()}
}

func (c *clock) String() string {
	return c.t.Format("15:04:05.000")
}

func (c *clock) time() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = t
}

func (c *clock) add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func verifyNoLeaks(t *testing.T) {
	goleak.VerifyNone(t,
		goleak.IgnoreTopFunction("github.com/jackc/pgx/v5/pgxpool.(*Pool).backgroundHealthCheck"),
		goleak.IgnoreAnyFunction("github.com/jackc/pgx/v5/pgxpool.(*Pool).trggerHealthCheck.func1"),
		goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),    // ignore cached connections
		goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"), // ignore cached connections
	)
}
