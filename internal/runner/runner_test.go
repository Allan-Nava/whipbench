package runner_test

// End-to-end: a real publisher → relay → viewers round trip over loopback, with
// the in-process relay from internal/testserver. No external server, no network
// beyond the loopback interface.

import (
	"context"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Allan-Nava/whipbench"
	"github.com/Allan-Nava/whipbench/internal/clip"
	"github.com/Allan-Nava/whipbench/internal/fingerprint"
	"github.com/Allan-Nava/whipbench/internal/publisher"
	"github.com/Allan-Nava/whipbench/internal/reassembler"
	"github.com/Allan-Nava/whipbench/internal/report"
	"github.com/Allan-Nava/whipbench/internal/rtc"
	"github.com/Allan-Nava/whipbench/internal/runner"
	"github.com/Allan-Nava/whipbench/internal/scenario"
	"github.com/Allan-Nava/whipbench/internal/testserver"
	"github.com/Allan-Nava/whipbench/internal/viewer"
)

const (
	secretQuery  = "QUERYSECRET"
	secretBearer = "BEARERSECRET"
)

var loop = rtc.Options{LoopbackOnly: true}

func run(t *testing.T, opt testserver.Options, sc scenario.Scenario, bearer string) *report.Report {
	t.Helper()
	opt.RTC = loop
	srv, err := testserver.New(opt)
	if err != nil {
		t.Fatal(err)
	}
	hs := httptest.NewServer(srv)
	t.Cleanup(func() { hs.Close(); srv.Close() })

	// The endpoints carry a token in the query string, the way some servers take
	// one; the report must not.
	sc.WHIP = hs.URL + "/whip?token=" + secretQuery
	sc.WHEP = hs.URL + "/whep?token=" + secretQuery
	c, err := clip.Load(sc.Codec, map[string][]byte{"vp8": whipbench.ClipVP8, "h264": whipbench.ClipH264}[sc.Codec])
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rep, err := runner.Run(ctx, runner.Options{Command: "run", Scenario: sc, Bearer: bearer, Clip: c, RTC: loop, Logf: t.Logf})
	if err != nil {
		t.Fatal(err)
	}
	assertNoSecrets(t, rep)
	return rep
}

func assertNoSecrets(t *testing.T, rep *report.Report) {
	t.Helper()
	js, err := rep.JSON()
	if err != nil {
		t.Fatal(err)
	}
	for name, doc := range map[string]string{"json": string(js), "markdown": rep.Markdown()} {
		for _, bad := range []string{secretQuery, secretBearer, "token=", "/whep", "/whip?"} {
			if strings.Contains(doc, bad) {
				t.Errorf("%s report contains %q", name, bad)
			}
		}
	}
	if strings.Contains(rep.Server.WHEPHost, "/") || strings.Contains(rep.Scenario.WHEP, "?") {
		t.Errorf("endpoint not reduced to its host: %+v", rep.Server)
	}
}

func base(codec string, viewers int) scenario.Scenario {
	return scenario.Scenario{Name: "test", Codec: codec, Viewers: viewers, RampSeconds: 1, HoldSeconds: 3, WarmupSeconds: 0.5, JoinTimeoutSeconds: 5}
}

func TestRoundTripVP8(t *testing.T) {
	t.Parallel()
	rep := run(t, testserver.Options{Token: secretBearer}, base("vp8", 4), secretBearer)
	a := rep.Aggregate
	if !a.Valid || a.Joined != 4 || a.Failed != 0 {
		t.Fatalf("aggregate: %+v\nerrors: %v", a, rep.Errors)
	}
	if rep.Publisher == nil || !rep.Publisher.Stamped || rep.Publisher.FramesSent < 60 {
		t.Fatalf("publisher: %+v", rep.Publisher)
	}
	if !a.PacketTransit.Available || a.PacketTransit.Viewers != 4 || a.PacketTransit.Ms == nil {
		t.Fatalf("packet transit over loopback through the relay must be available: %+v", a.PacketTransit)
	}
	if l := a.PacketTransit.Ms; l.P50 <= 0 || l.P99 > 1000 {
		t.Fatalf("implausible loopback delay: %+v", l)
	}
	for _, v := range rep.Viewers {
		if v.Codec != "vp8" || v.RTP.Received == 0 || v.RTP.Lost != 0 {
			t.Errorf("viewer %d: codec %q, %d received, %d lost", v.ID, v.Codec, v.RTP.Received, v.RTP.Lost)
		}
		if v.FirstRTPMs == nil || v.FirstKeyframeMs == nil || *v.FirstKeyframeMs < *v.FirstRTPMs {
			t.Errorf("viewer %d: join times %v %v", v.ID, v.FirstRTPMs, v.FirstKeyframeMs)
		}
		// The clip's GOP is 30 frames at 30 fps; RTP time does not jitter.
		if v.RTP.Keyframes >= 2 && math.Abs(v.RTP.KeyframeIntervalMeanS-1) > 1e-9 {
			t.Errorf("viewer %d: keyframe interval %v s, want 1", v.ID, v.RTP.KeyframeIntervalMeanS)
		}
	}
	if len(rep.Timeline) == 0 {
		t.Error("no timeline")
	}
	// No rampOffsetSeed: the plain ramp, as before WB-8.
	starts := scenario.Starts(4, time.Second)
	for _, v := range rep.Viewers {
		if v.RampOffsetMs != nil || v.StartOffsetMs != float64(starts[v.ID])/float64(time.Millisecond) {
			t.Errorf("viewer %d: start %v ms, offset %v, want the plain ramp", v.ID, v.StartOffsetMs, v.RampOffsetMs)
		}
	}
	// WB-38: one-way delay by frame fingerprint, pooled and per viewer.
	p := a.FingerprintDelay()
	if !p.Available || p.Viewers != 4 || p.ViewersByFrameEnd["marker"] != 4 || p.Ms == nil {
		t.Fatalf("pooled one-way delay over loopback through the relay must be available: %+v", p)
	}
	if p.Ms.P50 <= 0 || p.Ms.P99 >= 1000 {
		t.Errorf("implausible loopback one-way delay: %+v", p.Ms)
	}
	if p.Invalid != 0 || p.UnmatchedFrames != 0 || p.LoopFrames != 120 || p.DuplicateFrames != 0 {
		t.Errorf("pooled counts: %+v", p)
	}
	if p.LoopMinMs == nil || *p.LoopMinMs <= 3000 {
		t.Errorf("loopMinMs %v, want > 3000", p.LoopMinMs)
	}
	// WB-40: one process, one monotonic clock, so the figure is comparable.
	if rep.Topology != report.TopologySingleProcess || rep.Clock.Method != report.ClockMonotonic || rep.Clock.UncertaintyMs == nil {
		t.Errorf("topology %q, clock %+v", rep.Topology, rep.Clock)
	}
	if !p.Comparable || p.UncertaintyMs == nil || *p.UncertaintyMs != 0 {
		t.Errorf("a single-process fingerprint figure must be comparable: %+v", p)
	}
	checkFingerprintViewers(t, rep)
}

// checkFingerprintViewers asserts every viewer carries one available fingerprint block
// that found frame ends by marker and has neither invalid nor unmatched frames.
func checkFingerprintViewers(t *testing.T, rep *report.Report) {
	t.Helper()
	for _, v := range rep.Viewers {
		if len(v.OneWayDelay) != 1 {
			t.Errorf("viewer %d: %d one-way delay blocks, want 1", v.ID, len(v.OneWayDelay))
			continue
		}
		b := v.OneWayDelay[0]
		if b.Source != viewer.SourceFingerprint || !b.Available || b.FrameEnd != reassembler.EndMarker ||
			b.Samples == 0 || b.Invalid != 0 || b.UnmatchedFrames != 0 {
			t.Errorf("viewer %d: one-way delay block %+v", v.ID, b)
		}
	}
}

// WB-8: a seeded offset moves each viewer's start, and the report says by how much.
func TestRampOffsetsReachTheReport(t *testing.T) {
	t.Parallel()
	sc := base("vp8", 3)
	seed := int64(11)
	sc.RampOffsetSeed, sc.RampOffsetMaxSeconds = &seed, 0.5
	rep := run(t, testserver.Options{}, sc, "")
	if !rep.Aggregate.Valid || rep.Aggregate.Joined != 3 {
		t.Fatalf("aggregate: %+v\nerrors: %v", rep.Aggregate, rep.Errors)
	}
	if rep.Scenario.RampOffsetSeed == nil || *rep.Scenario.RampOffsetSeed != 11 || rep.Scenario.RampOffsetMaxSeconds != 0.5 {
		t.Fatalf("scenario in the report: %+v", rep.Scenario)
	}
	starts := scenario.Starts(3, time.Second)
	offsets := scenario.RampOffsets(3, 11, 500*time.Millisecond)
	ms := func(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
	for _, v := range rep.Viewers {
		if v.RampOffsetMs == nil || *v.RampOffsetMs != ms(offsets[v.ID]) || v.StartOffsetMs != ms(starts[v.ID]+offsets[v.ID]) {
			t.Errorf("viewer %d: start %v ms, offset %v, want %v + %v", v.ID, v.StartOffsetMs, v.RampOffsetMs, starts[v.ID], offsets[v.ID])
		}
	}
}

func TestRoundTripH264(t *testing.T) {
	t.Parallel()
	rep := run(t, testserver.Options{}, base("h264", 2), "")
	if !rep.Aggregate.Valid || rep.Aggregate.Joined != 2 {
		t.Fatalf("aggregate: %+v\nerrors: %v", rep.Aggregate, rep.Errors)
	}
	for _, v := range rep.Viewers {
		if v.Codec != "h264" || v.RTP.Lost != 0 || !v.PacketTransit.Available {
			t.Errorf("viewer %d: %+v", v.ID, v)
		}
	}
	checkFingerprintViewers(t, rep)
	if p := rep.Aggregate.FingerprintDelay(); !p.Available || p.Viewers != 2 {
		t.Errorf("pooled one-way delay: %+v", p)
	}
}

// publishFor streams the embedded VP8 clip for one second to a relay, with the given
// send log (nil is publish's path), and returns the publisher's result.
func publishFor(t *testing.T, log *fingerprint.SendLog) publisher.Result {
	t.Helper()
	srv, err := testserver.New(testserver.Options{RTC: loop})
	if err != nil {
		t.Fatal(err)
	}
	hs := httptest.NewServer(srv)
	t.Cleanup(func() { hs.Close(); srv.Close() })
	c, err := clip.Load("vp8", whipbench.ClipVP8)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pub, err := publisher.Connect(ctx, publisher.Config{WHIP: hs.URL + "/whip", Clip: c, RTC: loop, SendLog: log})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	sctx, scancel := context.WithTimeout(ctx, time.Second)
	defer scancel()
	return pub.Stream(sctx)
}

func TestRoundTripPublisherWithoutASendLog(t *testing.T) {
	t.Parallel()
	res := publishFor(t, nil)
	if res.FramesSent < 20 || res.Error != "" {
		t.Fatalf("publisher without a send log: %+v", res)
	}
}

func TestRoundTripPublisherFillsTheSendLog(t *testing.T) {
	t.Parallel()
	log := fingerprint.NewSendLog(120)
	res := publishFor(t, log)
	if log.Recorded() != res.FramesSent || res.FramesSent < 20 {
		t.Fatalf("send log recorded %d frames, publisher sent %d", log.Recorded(), res.FramesSent)
	}
	if _, _, ok := log.Latest(0, time.Now()); !ok {
		t.Error("the send log has no send of clip frame 0")
	}
}

func TestMethodStatesTheReassemblyWindow(t *testing.T) {
	if reassembler.Window != time.Second {
		t.Fatalf("reassembler.Window is %v; the Method line says 1 s", reassembler.Window)
	}
	if !strings.Contains(report.OneWayDelayMethod, "incomplete 1 s after its first packet") {
		t.Error("OneWayDelayMethod does not state the reassembly window")
	}
}

func TestStrippedExtensionIsUnavailableNotGuessed(t *testing.T) {
	t.Parallel()
	rep := run(t, testserver.Options{StripExtensions: true}, base("vp8", 2), "")
	a := rep.Aggregate
	if !a.Valid {
		t.Fatalf("stripping the stamp must not invalidate the run: %+v", a)
	}
	if a.PacketTransit.Available || a.PacketTransit.Ms != nil {
		t.Fatalf("packet transit must be unavailable when the stamp is stripped: %+v", a.PacketTransit)
	}
	if !strings.Contains(a.PacketTransit.Reason, "no stamps arrived") {
		t.Fatalf("reason %q", a.PacketTransit.Reason)
	}
	if md := rep.Markdown(); !strings.Contains(md, "**Packet transit: unavailable**") {
		t.Fatalf("the Markdown must say so:\n%s", md)
	}
}

func TestNoVerdictWhenTooManyViewersFail(t *testing.T) {
	t.Parallel()
	// The relay takes 8 viewers and refuses the rest: 2 of 10 is 20%, above 10%.
	rep := run(t, testserver.Options{MaxViewers: 8}, base("vp8", 10), "")
	a := rep.Aggregate
	if a.Valid || a.Joined != 8 || a.Failed != 2 || rep.Errors["http_503"] != 2 {
		t.Fatalf("aggregate: %+v errors %v", a, rep.Errors)
	}
	if !strings.HasPrefix(a.Verdict, "no verdict: 2 of 10 viewers (20%) failed to join") {
		t.Fatalf("verdict %q", a.Verdict)
	}
	if md := rep.Markdown(); strings.Contains(md, "## Aggregate") {
		t.Fatal("an invalid run must not print the aggregate table")
	}
}

func TestLossThroughALossyRelay(t *testing.T) {
	t.Parallel()
	rep := run(t, testserver.Options{DropEvery: 50}, base("vp8", 2), "")
	for _, v := range rep.Viewers {
		if !v.Joined {
			continue // a lost keyframe packet can delay joining; the loss figure is what is checked
		}
		if v.RTP.LossPercent < 1 || v.RTP.LossPercent > 3 {
			t.Errorf("viewer %d: loss %.2f%%, want about 2%% (every 50th packet dropped, less what NACK recovers)", v.ID, v.RTP.LossPercent)
		}
		if len(v.OneWayDelay) != 1 {
			t.Errorf("viewer %d: %d one-way delay blocks, want 1", v.ID, len(v.OneWayDelay))
			continue
		}
		// An incomplete frame that reached a hash would almost surely miss the clip
		// table, so zero unmatched shows that none did.
		b := v.OneWayDelay[0]
		if b.IncompleteFrames == 0 || !b.Available || b.Samples == 0 || b.Samples > b.CompleteFrames || b.UnmatchedFrames != 0 {
			t.Errorf("viewer %d: one-way delay block %+v", v.ID, b)
		}
		t.Logf("viewer %d: received %d, lost %d, about %d dropped by the relay", v.ID, v.RTP.Received, v.RTP.Lost, (v.RTP.Received+v.RTP.Lost)/50)
	}
}

// Without a marker no keyframe completes (internal/rtpstats/rtpstats.go:248-252), so
// no viewer joins and the pooled block says "no viewer joined": this test asserts
// neither Joined, the verdict nor the pooled block (P8). What it checks is that every
// viewer's reassembler fell back to the next RTP timestamp to end a frame (D5), and
// that the frames it closed that way still hash to the clip.
func TestMarkerlessRelay(t *testing.T) {
	t.Parallel()
	rep := run(t, testserver.Options{ClearMarker: true}, base("vp8", 2), "")
	for _, v := range rep.Viewers {
		if len(v.OneWayDelay) != 1 {
			t.Errorf("viewer %d: %d one-way delay blocks, want 1", v.ID, len(v.OneWayDelay))
			continue
		}
		b := v.OneWayDelay[0]
		if b.FrameEnd != reassembler.EndTimestamp || !b.Available || b.Samples == 0 || b.Invalid != 0 || b.UnmatchedFrames != 0 {
			t.Errorf("viewer %d: one-way delay block %+v", v.ID, b)
		}
		t.Logf("viewer %d: %d samples, %d complete, %d incomplete", v.ID, b.Samples, b.CompleteFrames, b.IncompleteFrames)
	}
}

func TestMetricsEndpointDuringARun(t *testing.T) {
	t.Parallel()
	srv, err := testserver.New(testserver.Options{RTC: loop})
	if err != nil {
		t.Fatal(err)
	}
	hs := httptest.NewServer(srv)
	defer func() { hs.Close(); srv.Close() }()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	sc := base("vp8", 2)
	sc.WHIP, sc.WHEP = hs.URL+"/whip", hs.URL+"/whep"
	c, _ := clip.Load("vp8", whipbench.ClipVP8)
	done := make(chan *report.Report, 1)
	go func() {
		rep, _ := runner.Run(context.Background(), runner.Options{Scenario: sc, Clip: c, RTC: loop, MetricsListener: ln})
		done <- rep
	}()
	url := "http://" + ln.Addr().String() + "/metrics"
	deadline := time.Now().Add(10 * time.Second)
	var body string
	for time.Now().Before(deadline) {
		if resp, err := http.Get(url); err == nil { //nolint:noctx // test
			b, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			body = string(b)
			// WB-42: the headline shows while the run runs, not only in the report.
			if strings.Contains(body, "whipbench_viewers_joined_total 2\n") &&
				strings.Contains(body, `whipbench_one_way_delay_seconds_count{source="fingerprint"} `) &&
				!strings.Contains(body, `whipbench_one_way_delay_seconds_count{source="fingerprint"} 0`+"\n") {
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !strings.Contains(body, "whipbench_viewers_joined_total 2\n") || !strings.Contains(body, "whipbench_viewers_target 2\n") ||
		strings.Contains(body, `whipbench_one_way_delay_seconds_count{source="fingerprint"} 0`+"\n") {
		t.Fatalf("metrics during the run:\n%s", body)
	}
	if rep := <-done; rep == nil || !rep.Aggregate.Valid {
		t.Fatalf("run: %+v", rep)
	}
}
