package runner_test

// WB-3: a split run — viewers only, the publisher fed to the relay separately, the way
// `view` runs against another host's `publish` — measuring the publisher's clock.

import (
	"context"
	"fmt"
	"math"
	"net"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Allan-Nava/whipbench"
	"github.com/Allan-Nava/whipbench/internal/clip"
	"github.com/Allan-Nava/whipbench/internal/clocksync"
	"github.com/Allan-Nava/whipbench/internal/publisher"
	"github.com/Allan-Nava/whipbench/internal/report"
	"github.com/Allan-Nava/whipbench/internal/runner"
	"github.com/Allan-Nava/whipbench/internal/scenario"
	"github.com/Allan-Nava/whipbench/internal/testserver"
)

// published starts a relay with a publisher streaming into it until the test ends, and
// returns the relay's WHEP endpoint.
func published(t *testing.T) string {
	t.Helper()
	srv, err := testserver.New(testserver.Options{RTC: loop})
	if err != nil {
		t.Fatal(err)
	}
	hs := httptest.NewServer(srv)
	c, err := clip.Load("vp8", whipbench.ClipVP8)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	pub, err := publisher.Connect(ctx, publisher.Config{WHIP: hs.URL + "/whip", Clip: c, RTC: loop})
	if err != nil {
		cancel()
		t.Fatalf("connect: %v", err)
	}
	done := make(chan struct{})
	go func() { pub.Stream(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done; hs.Close(); srv.Close() })
	select {
	case <-srv.Ready():
	case <-time.After(10 * time.Second):
		t.Fatal("the publisher's track never reached the relay")
	}
	return hs.URL + "/whep"
}

// splitRun runs viewers only against whep with the given clock peer, and returns the
// report and every log line.
func splitRun(t *testing.T, whep, peer string, every time.Duration) (*report.Report, string) {
	t.Helper()
	var mu sync.Mutex
	var logs strings.Builder
	logf := func(format string, a ...any) {
		mu.Lock()
		defer mu.Unlock()
		fmt.Fprintf(&logs, format+"\n", a...)
	}
	sc := scenario.Scenario{Name: "split", Codec: "vp8", Viewers: 2, HoldSeconds: 2, JoinTimeoutSeconds: 5, WHEP: whep}
	opt := runner.WithClockEvery(runner.Options{Command: "view", Scenario: sc, RTC: loop, Logf: logf, ClockPeer: peer}, every)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rep, err := runner.Run(ctx, opt)
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	return rep, logs.String()
}

// assertNoPeer: the peer's address reaches neither the report nor the log.
func assertNoPeer(t *testing.T, rep *report.Report, logs string, peer string) {
	t.Helper()
	js, err := rep.JSON()
	if err != nil {
		t.Fatal(err)
	}
	_, port, _ := net.SplitHostPort(peer)
	for name, doc := range map[string]string{"json": string(js), "markdown": rep.Markdown(), "log": logs} {
		if strings.Contains(doc, peer) || strings.Contains(doc, ":"+port) {
			t.Errorf("the %s names the clock peer", name)
		}
	}
}

func TestSplitRunMeasuresTheClockPeer(t *testing.T) {
	t.Parallel()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() {
		served <- clocksync.Serve(ctx, conn, func() time.Time { return time.Now().Add(250 * time.Millisecond) })
	}()
	t.Cleanup(func() { cancel(); <-served; _ = conn.Close() })
	peer := conn.LocalAddr().String()

	rep, logs := splitRun(t, published(t), peer, 700*time.Millisecond)
	if !rep.Aggregate.Valid || rep.Aggregate.Joined != 2 {
		t.Fatalf("aggregate: %+v\nerrors: %v", rep.Aggregate, rep.Errors)
	}
	c := rep.Clock
	if rep.Topology != report.TopologySplit || c.Method != report.ClockExchange {
		t.Fatalf("topology %q, clock %+v", rep.Topology, c)
	}
	// Start, at least one mid-run point over a 2 s hold, and the end.
	if len(c.Points) < 3 || c.OffsetMs == nil || c.UncertaintyMs == nil || c.Reason != "" {
		t.Fatalf("clock %+v", c)
	}
	if *c.UncertaintyMs <= 0 || *c.UncertaintyMs > 25 || *c.OffsetMs != c.Points[0].OffsetMs {
		t.Errorf("offset %v ± %v", *c.OffsetMs, *c.UncertaintyMs)
	}
	for i, p := range c.Points {
		if math.Abs(p.OffsetMs-250) > p.RTTMs/2+0.002 {
			t.Errorf("point %d: offset %v ms, want 250 ± %v", i, p.OffsetMs, p.RTTMs/2)
		}
		if p.RTTMs/2 > *c.UncertaintyMs+0.001 {
			t.Errorf("point %d: rtt/2 %v above the uncertainty %v", i, p.RTTMs/2, *c.UncertaintyMs)
		}
		if i > 0 && p.TS <= c.Points[i-1].TS {
			t.Errorf("point %d at %v s, not after the one before", i, p.TS)
		}
	}
	// tS is rounded to the millisecond, and the report's start and end are wall readings
	// (UTC drops the monotonic one), so the end is allowed that millisecond.
	if c.Points[0].TS < 0 || c.Points[len(c.Points)-1].TS > rep.FinishedAt.Sub(rep.StartedAt).Seconds()+0.001 {
		t.Errorf("points outside the run: %+v", c.Points)
	}
	if !strings.Contains(rep.Markdown(), fmt.Sprintf("%d points", len(c.Points))) {
		t.Error("the Markdown clock row does not count the points")
	}
	if !strings.Contains(strings.Join(rep.Method, "\n"), "Clock exchange:") {
		t.Error("the report does not carry the exchange's definition")
	}
	// No split figure applies the offset yet: one-way delay is unavailable in `view`.
	if p := rep.Aggregate.FingerprintDelay(); p.Available || p.Comparable {
		t.Errorf("a split run's fingerprint block: %+v", p)
	}
	assertNoPeer(t, rep, logs, peer)
}

func TestSplitRunWithASilentClockPeer(t *testing.T) {
	t.Parallel()
	silent, err := net.ListenPacket("udp", "127.0.0.1:0") // nobody answers on it
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = silent.Close() })
	peer := silent.LocalAddr().String()

	rep, logs := splitRun(t, published(t), peer, time.Hour)
	c := rep.Clock
	if c.Method != report.ClockNone || c.Reason != report.NoAnswerReason || c.OffsetMs != nil || c.UncertaintyMs != nil || len(c.Points) != 0 {
		t.Fatalf("clock %+v", c)
	}
	if !strings.Contains(rep.Markdown(), report.NoClockReason+": "+report.NoAnswerReason) {
		t.Error("the Markdown clock row does not say the peer did not answer")
	}
	if !strings.Contains(logs, "no point, skipped") {
		t.Errorf("a failed point is not logged:\n%s", logs)
	}
	assertNoPeer(t, rep, logs, peer)
}

// Without a clock peer a split run stays method none, as before WB-3.
func TestSplitRunWithoutAClockPeer(t *testing.T) {
	t.Parallel()
	rep, _ := splitRun(t, published(t), "", 0)
	if c := rep.Clock; c.Method != report.ClockNone || c.Reason != "" || c.OffsetMs != nil || len(c.Points) != 0 {
		t.Fatalf("clock %+v", c)
	}
	if strings.Contains(strings.Join(rep.Method, "\n"), "Clock exchange:") {
		t.Error("a run without a clock peer carries the exchange's definition")
	}
}
