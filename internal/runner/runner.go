// Package runner executes a scenario: one publisher (optional), then viewers
// started on the ramp, all held until the end, then the report.
package runner

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/Allan-Nava/whipbench/internal/clip"
	"github.com/Allan-Nava/whipbench/internal/fingerprint"
	"github.com/Allan-Nava/whipbench/internal/metrics"
	"github.com/Allan-Nava/whipbench/internal/publisher"
	"github.com/Allan-Nava/whipbench/internal/report"
	"github.com/Allan-Nava/whipbench/internal/rtc"
	"github.com/Allan-Nava/whipbench/internal/scenario"
	"github.com/Allan-Nava/whipbench/internal/version"
	"github.com/Allan-Nava/whipbench/internal/viewer"
	"github.com/Allan-Nava/whipbench/internal/whip"
)

// Options is one run.
type Options struct {
	Command  string
	Scenario scenario.Scenario
	// Bearer is the token read from Scenario.BearerEnv by the caller; it is sent and
	// never recorded.
	Bearer string
	// Clip is what the publisher streams; required when Scenario.WHIP is set.
	Clip *clip.Clip
	RTC  rtc.Options
	HTTP *http.Client
	// Logf, when set, receives progress lines.
	Logf func(format string, args ...any)
	// MetricsListener, when set, is used for /metrics instead of listening on
	// Scenario.Metrics (tests pass one on an ephemeral port).
	MetricsListener net.Listener
	// ClockPeer, in a split run, is the HOST:PORT of the publisher's clock responder
	// (`publish --clock-listen`), against which the run measures the publisher's clock
	// (WB-3); a run that publishes ignores it. It sits here, not in the scenario, because
	// the scenario is recorded in the report and a report never names a load host: the
	// address reaches no report, no metric and no log line.
	ClockPeer string
	// clockEvery replaces ClockInterval when set, so the tests need not wait 30 s.
	clockEvery time.Duration
	// live replaces the run's own metrics when set, so a test can read them after the
	// run has stopped serving them.
	live *metrics.Live
}

// Run executes the scenario and returns its report. An error means the run could
// not be attempted at all; a run that was attempted and failed is a report whose
// aggregate is not valid.
func Run(ctx context.Context, opt Options) (*report.Report, error) {
	sc := opt.Scenario
	sc.Normalise()
	if err := sc.Validate(); err != nil {
		return nil, err
	}
	if sc.WHIP != "" && opt.Clip == nil {
		return nil, errors.New("runner: a WHIP endpoint needs a clip")
	}
	// One-way delay: the clip table and the send log exist only when this run publishes.
	var table *fingerprint.Table
	var sendLog *fingerprint.SendLog
	if sc.WHIP != "" {
		var err error
		if table, err = fingerprint.NewTable(opt.Clip); err != nil {
			return nil, fmt.Errorf("runner: %w", err)
		}
		sendLog = fingerprint.NewSendLog(table.Frames())
	}
	logf := opt.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}
	rtcOpt := opt.RTC
	if sc.IncludeLoopback {
		rtcOpt.IncludeLoopback = true
	}
	if sc.LoopbackOnly {
		rtcOpt.LoopbackOnly = true
	}
	live := opt.live
	if live == nil {
		live = &metrics.Live{}
	}
	live.ViewersTarget.Store(int64(sc.Viewers))

	stopMetrics, err := serveMetrics(opt.MetricsListener, sc.Metrics, live, logf)
	if err != nil {
		return nil, err
	}
	defer stopMetrics()

	in := report.Input{Command: opt.Command, Version: version.String(), StartedAt: time.Now(), Scenario: sc,
		Topology: report.TopologySplit}
	if table != nil {
		in.Fingerprint = &report.Fingerprint{DuplicateFrames: table.DuplicateFrames(), LoopFrames: table.Frames()}
		in.Topology = report.TopologySingleProcess
	}
	// steps watches the wall clock once a second from the start (the timeline's ticker) and
	// at the end; nothing else touches it, and finish runs after the timeline has stopped.
	steps := report.NewStepWatch(in.StartedAt)
	// clk is WB-3's exchange, in a split run given a clock peer.
	var clk *clockExchange
	if opt.ClockPeer != "" {
		if in.Topology == report.TopologySplit {
			clk = startClock(opt.ClockPeer, logf)
			defer clk.close()
		} else {
			logf("clock peer ignored: this run publishes, so one monotonic clock reads both ends")
		}
	}
	// finish stamps the end; a run that publishes reads both ends on this process's
	// monotonic clock (WB-40), a split run with a clock peer carries the exchange's points
	// (WB-3), and any other split run leaves Clock nil — method none.
	finish := func() {
		in.FinishedAt = time.Now()
		switch {
		case in.Topology == report.TopologySingleProcess:
			steps.Observe(in.FinishedAt)
			in.Clock = report.MonotonicClock(steps.Stepped())
		case clk != nil:
			steps.Observe(in.FinishedAt)
			in.Clock = clk.clock(in.StartedAt, steps.Stepped())
		}
	}

	// The publisher.
	var pubDone chan publisher.Result
	pubCancel := func() {}
	if sc.WHIP != "" {
		logf("publishing %s to %s", sc.Codec, whip.Host(sc.WHIP))
		pub, err := publisher.Connect(ctx, publisher.Config{
			WHIP: sc.WHIP, Bearer: opt.Bearer, Clip: opt.Clip, RTC: rtcOpt, HTTP: opt.HTTP, Live: live,
			SendLog: sendLog,
		})
		if err != nil {
			res := pub.Result()
			in.Publisher = &res
			finish()
			logf("publisher failed: %s", res.Error)
			return report.Build(in), nil
		}
		logf("publisher connected in %.0f ms; send-time stamp negotiated: %v", pub.Result().ConnectMs, pub.Stamped())
		var pctx context.Context
		pctx, pubCancel = context.WithCancel(context.WithoutCancel(ctx))
		pubDone = make(chan publisher.Result, 1)
		go func() { pubDone <- pub.Stream(pctx) }()
		select {
		case <-time.After(sc.Warmup()):
		case <-ctx.Done():
		}
	}

	// The viewers, after the exchange's first point.
	if clk != nil {
		clk.measure(ctx, "start")
	}
	starts, offsets := sc.Schedule()
	if offsets != nil {
		logf("starting %d viewers over %gs, each offset by up to %gs (seed %d), holding %gs",
			sc.Viewers, sc.RampSeconds, sc.RampOffsetMaxSeconds, *sc.RampOffsetSeed, sc.HoldSeconds)
	} else {
		logf("starting %d viewers over %gs, holding %gs", sc.Viewers, sc.RampSeconds, sc.HoldSeconds)
	}
	vctx, vcancel := context.WithTimeout(ctx, sc.Duration())
	defer vcancel()
	t0 := time.Now()
	timeline := sampleTimeline(vctx, live, t0, steps)
	stopClock := func() {}
	if clk != nil {
		every := opt.clockEvery
		if every <= 0 {
			every = ClockInterval
		}
		stopClock = clk.every(vctx, every)
	}
	results := make([]viewer.Result, sc.Viewers)
	var wg sync.WaitGroup
	for i := range sc.Viewers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var offset *float64 // recorded even for a viewer that never started
			if offsets != nil {
				o := ms(offsets[i])
				offset = &o
			}
			select {
			case <-time.After(time.Until(t0.Add(starts[i]))):
			case <-vctx.Done():
				results[i] = viewer.Result{ID: i, RampOffsetMs: offset, ErrorKind: "not_started", Error: "the run ended before this viewer's start time"}
				return
			}
			r := viewer.Run(vctx, i, viewer.Config{
				WHEP: sc.WHEP, Bearer: opt.Bearer, RTC: rtcOpt, HTTP: opt.HTTP,
				JoinTimeout: sc.JoinTimeout(), Stall: sc.Stall(), Live: live,
				Frames: table, SendLog: sendLog,
			})
			r.StartOffsetMs = ms(starts[i])
			r.RampOffsetMs = offset
			results[i] = r
		}(i)
	}
	wg.Wait()
	vcancel()
	stopClock()
	if clk != nil {
		clk.measure(ctx, "end")
	}
	in.Timeline = <-timeline
	in.Viewers = results
	pubCancel()
	if pubDone != nil {
		res := <-pubDone
		in.Publisher = &res
	}
	finish()
	in.Interrupted = ctx.Err() != nil
	if in.Fingerprint != nil {
		if d, ok := sendLog.LoopMin(); ok {
			in.Fingerprint.LoopMin = d
		}
	}
	rep := report.Build(in)
	logf("%s", rep.Aggregate.Verdict)
	return rep, nil
}

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

func serveMetrics(ln net.Listener, addr string, live *metrics.Live, logf func(string, ...any)) (func(), error) {
	if ln == nil && addr == "" {
		return func() {}, nil
	}
	if ln == nil {
		var err error
		ln, err = net.Listen("tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("metrics: %w", err)
		}
	}
	srv := &http.Server{Handler: live.Handler(), ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	logf("metrics on http://%s/metrics", ln.Addr())
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}, nil
}

// sampleTimeline records, once a second until ctx is done, how many viewers were
// receiving and how many packets arrived in that second, and shows the second to steps.
func sampleTimeline(ctx context.Context, live *metrics.Live, t0 time.Time, steps *report.StepWatch) <-chan []report.Second {
	out := make(chan []report.Second, 1)
	go func() {
		var tl []report.Second
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		last := live.PacketsReceived.Load()
		for {
			select {
			case <-ctx.Done():
				out <- tl
				return
			case now := <-tick.C:
				steps.Observe(time.Now())
				p := live.PacketsReceived.Load()
				tl = append(tl, report.Second{T: int(now.Sub(t0).Round(time.Second) / time.Second), Active: live.ViewersActive.Load(), Packets: p - last})
				last = p
			}
		}
	}()
	return out
}
