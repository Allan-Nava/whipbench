package runner

import (
	"time"

	"github.com/Allan-Nava/whipbench/internal/metrics"
)

// WithClockEvery shortens the interval between mid-run clock points for a test.
func WithClockEvery(o Options, d time.Duration) Options {
	o.clockEvery = d
	return o
}

// WithLive makes the run record its live metrics in l, which the test reads afterwards.
func WithLive(o Options, l *metrics.Live) Options {
	o.live = l
	return o
}
