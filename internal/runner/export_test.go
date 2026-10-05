package runner

import "time"

// WithClockEvery shortens the interval between mid-run clock points for a test.
func WithClockEvery(o Options, d time.Duration) Options {
	o.clockEvery = d
	return o
}
