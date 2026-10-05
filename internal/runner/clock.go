package runner

import (
	"context"
	"errors"
	"math"
	"net"
	"sync"
	"time"

	"github.com/Allan-Nava/whipbench/internal/clocksync"
	"github.com/Allan-Nava/whipbench/internal/report"
)

// ClockInterval is how often a split run with a clock peer measures a point while its
// viewers run, between the one before the first viewer and the one after the last (WB-3).
const ClockInterval = 30 * time.Second

// clockExchange is a split run's side of WB-3's exchange. Its points are appended by one
// goroutine at a time: the run before and after the viewers, the ticker in between.
// Neither the peer's address nor an error that could carry it reaches a log line.
type clockExchange struct {
	conn   net.PacketConn
	peer   net.Addr
	logf   func(string, ...any)
	points []clocksync.Point
	// reason, when set, is why no exchange could even be attempted.
	reason string
}

func startClock(peer string, logf func(string, ...any)) *clockExchange {
	c := &clockExchange{logf: logf}
	addr, err := net.ResolveUDPAddr("udp", peer)
	if err != nil {
		c.reason = "clock peer address did not resolve"
		logf("%s", c.reason)
		return c
	}
	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		c.reason = "no UDP socket for the clock exchange"
		logf("%s", c.reason)
		return c
	}
	c.conn, c.peer = conn, addr
	return c
}

func (c *clockExchange) close() {
	if c.conn != nil {
		_ = c.conn.Close()
	}
}

// measure takes one point; a failed one is logged and skipped, an ended context is not
// worth a line.
func (c *clockExchange) measure(ctx context.Context, when string) {
	if c.conn == nil {
		return
	}
	p, err := clocksync.Measure(ctx, c.conn, c.peer, 0, 0)
	switch {
	case err == nil:
		c.points = append(c.points, p)
		c.logf("clock peer (%s): offset %.3f ms ± %.3f ms", when, msf(p.Offset), msf(p.RTT/2))
	case ctx.Err() != nil && errors.Is(err, ctx.Err()):
	default:
		c.logf("clock peer (%s): no point, skipped: %v", when, err)
	}
}

// every measures a point each interval until ctx is done; the returned stop waits for
// the goroutine, so nothing appends after it returns.
func (c *clockExchange) every(ctx context.Context, interval time.Duration) (stop func()) {
	if c.conn == nil {
		return func() {}
	}
	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		tick := time.NewTicker(interval)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				c.measure(ctx, "mid-run")
			}
		}
	}()
	return func() { cancel(); wg.Wait() }
}

// clock is the report's clock block: the points against the run's start, or method none
// with the reason when there is none.
func (c *clockExchange) clock(start time.Time, stepped bool) *report.Clock {
	if c.reason != "" {
		return report.UnmeasuredClock(c.reason, stepped)
	}
	pts := make([]report.ClockPoint, len(c.points))
	for i, p := range c.points {
		pts[i] = report.ClockPoint{TS: round3(p.At.Sub(start).Seconds()), OffsetMs: round3(msf(p.Offset)), RTTMs: round3(msf(p.RTT))}
	}
	// Rounded up to the microsecond: an uncertainty is never understated.
	unc := math.Ceil(msf(clocksync.Model{Points: c.points}.Uncertainty())*1000) / 1000
	return report.ExchangeClock(pts, unc, stepped)
}

func msf(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

func round3(v float64) float64 { return math.Round(v*1000) / 1000 }
