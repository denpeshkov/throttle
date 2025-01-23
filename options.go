package throttle

import "time"

// Option configures a limiter.
type Option interface {
	apply(*options)
}

type options struct {
	keyTTL time.Duration
	clock  func() time.Time
}

type optionFunc func(*options)

func (f optionFunc) apply(o *options) { f(o) }

// WithKeyTTL sets the duration for which the data structures associated with the limiter
// are held in Redis after they become inactive (expire).
// This is primarily used for testing purposes.
func WithKeyTTL(d time.Duration) Option {
	return optionFunc(func(o *options) {
		o.keyTTL = d
	})
}

// WithClock sets a custom function to return the current time.
// This is primarily used for testing purposes.
func WithClock(f func() time.Time) Option {
	return optionFunc(func(o *options) {
		o.clock = f
	})
}
