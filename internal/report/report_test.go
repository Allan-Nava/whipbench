package report

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Allan-Nava/whipbench/internal/publisher"
	"github.com/Allan-Nava/whipbench/internal/rtpstats"
	"github.com/Allan-Nava/whipbench/internal/scenario"
	"github.com/Allan-Nava/whipbench/internal/viewer"
)

func f(v float64) *float64 { return &v }

// viewers returns n results of which the first failed did not join.
func viewers(n, failed int) []viewer.Result {
	out := make([]viewer.Result, n)
	for i := range out {
		if i < failed {
			out[i] = viewer.Result{ID: i, ErrorKind: "http_503", Error: "POST to example.test: HTTP 503"}
			continue
		}
		out[i] = viewer.Result{
			ID: i, Joined: true, Codec: "vp8",
			SignallingMs: f(5), FirstRTPMs: f(float64(10 + i)), FirstKeyframeMs: f(float64(100 + i)),
			RTP: rtpstats.Summary{Received: 1000, Expected: 1000, Keyframes: 3, KeyframeIntervalMeanS: 1},
		}
	}
	return out
}

func build(sc scenario.Scenario, vs []viewer.Result) *Report {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	return Build(Input{Command: "run", Version: "v0.0.1-test", StartedAt: now, FinishedAt: now.Add(time.Minute), Scenario: sc, Viewers: vs})
}

func sc(n int) scenario.Scenario {
	return scenario.Scenario{WHEP: "https://example.test/whep", Viewers: n, HoldSeconds: 1, Codec: "vp8"}
}

func TestNoVerdictBoundary(t *testing.T) {
	for _, tc := range []struct {
		n, failed int
		valid     bool
	}{
		{10, 0, true},
		{10, 1, true},  // exactly 10%: allowed
		{10, 2, false}, // 20%
		{20, 2, true},  // exactly 10%
		{20, 3, false}, // 15%
		{100, 11, false},
		{1, 1, false},
	} {
		r := build(sc(tc.n), viewers(tc.n, tc.failed))
		if r.Aggregate.Valid != tc.valid {
			t.Errorf("%d of %d failed: valid=%v, want %v (%s)", tc.failed, tc.n, r.Aggregate.Valid, tc.valid, r.Aggregate.Verdict)
		}
		if !tc.valid && !strings.HasPrefix(r.Aggregate.Verdict, "no verdict") {
			t.Errorf("verdict %q", r.Aggregate.Verdict)
		}
		if !tc.valid && strings.Contains(r.Markdown(), "## Aggregate") {
			t.Errorf("%d of %d failed: the Markdown shows aggregates for an invalid run", tc.failed, tc.n)
		}
	}
}

func TestViewersThatNeverStartedCountAsFailed(t *testing.T) {
	// The scenario asked for 10; only 9 results exist (one goroutine never ran).
	// The denominator is what was asked for, not what came back.
	r := build(sc(10), viewers(9, 0))
	if r.Aggregate.Viewers != 10 || r.Aggregate.Failed != 1 || !r.Aggregate.Valid {
		t.Fatalf("aggregate %+v", r.Aggregate)
	}
}

func TestAggregatesAreOverJoinedViewers(t *testing.T) {
	r := build(sc(10), viewers(10, 1))
	a := r.Aggregate
	if a.Joined != 9 || a.FirstKeyframeMs.N != 9 || a.FirstKeyframeMs.Min != 101 || a.FirstKeyframeMs.Max != 109 || a.FirstKeyframeMs.P50 != 105 {
		t.Fatalf("join summary %+v", a.FirstKeyframeMs)
	}
	if r.Errors["http_503"] != 1 {
		t.Fatalf("errors %v", r.Errors)
	}
}

func TestPacketTransitUnavailableIsNeverANumber(t *testing.T) {
	vs := viewers(3, 0)
	for i := range vs {
		vs[i].PacketTransit = viewer.PacketTransit{Negotiated: false, Reason: "not negotiated: the WHEP answer did not accept abs-capture-time"}
	}
	r := build(sc(3), vs)
	if r.Aggregate.PacketTransit.Available || r.Aggregate.PacketTransit.Ms != nil {
		t.Fatalf("packet transit %+v", r.Aggregate.PacketTransit)
	}
	md := r.Markdown()
	if !strings.Contains(md, "**Packet transit: unavailable** — not negotiated") || strings.Contains(md, "packet transit (stamped") {
		t.Fatalf("markdown:\n%s", md)
	}
}

// The 0.0.1 figure is packetTransit in the JSON and in the Method lines; no key, no definition is called latency (WB-1, D1).
func TestPacketTransitKeys(t *testing.T) {
	r := build(sc(3), viewers(3, 0))
	b, err := r.JSON()
	if err != nil {
		t.Fatal(err)
	}
	js := string(b)
	if n := strings.Count(js, "\"packetTransit\": {"); n != 4 {
		t.Errorf("want 4 packetTransit objects (aggregate + 3 viewers), got %d:\n%s", n, js)
	}
	if strings.Contains(strings.ToLower(js), "latency") {
		t.Errorf("the 0.0.1 key or wording is back:\n%s", js)
	}
	if strings.Contains(js, "one-way delay") {
		t.Errorf("that name is reserved for the per-frame figure of WB-38:\n%s", js)
	}
}

func TestPublisherThatNeverStreamedHasNoVerdict(t *testing.T) {
	now := time.Now()
	r := Build(Input{Scenario: sc(5), StartedAt: now, FinishedAt: now,
		Publisher: &publisher.Result{ErrorKind: "http_status", Error: "POST to example.test: HTTP 401"}})
	if r.Aggregate.Valid || !strings.Contains(r.Aggregate.Verdict, "publisher never streamed") || r.Errors["publisher_http_status"] != 1 {
		t.Fatalf("aggregate %+v errors %v", r.Aggregate, r.Errors)
	}
}

func TestInterruptedRunHasNoVerdict(t *testing.T) {
	now := time.Now()
	r := Build(Input{Scenario: sc(2), Viewers: viewers(2, 0), StartedAt: now, FinishedAt: now, Interrupted: true})
	if r.Aggregate.Valid {
		t.Fatal("an interrupted run is not a result")
	}
}

func TestReportKeepsOnlyTheHost(t *testing.T) {
	s := sc(1)
	s.WHIP = "https://user:pw@ingest.example.test:8443/live/STREAMKEY/whip?token=TOKENVALUE"
	s.WHEP = "https://edge.example.test/live/STREAMKEY/whep?access_token=TOKENVALUE#frag"
	s.BearerEnv = "WHIPBENCH_TOKEN"
	r := build(s, viewers(1, 0))
	js, err := r.JSON()
	if err != nil {
		t.Fatal(err)
	}
	for name, doc := range map[string]string{"json": string(js), "markdown": r.Markdown()} {
		for _, bad := range []string{"STREAMKEY", "TOKENVALUE", "token=", "user:pw", "pw@", "/live/", "#frag"} {
			if strings.Contains(doc, bad) {
				t.Errorf("%s contains %q", name, bad)
			}
		}
		if !strings.Contains(doc, "ingest.example.test:8443") || !strings.Contains(doc, "edge.example.test") {
			t.Errorf("%s lost the host", name)
		}
	}
	if r.Server.WHIPHost != "ingest.example.test:8443" || r.Scenario.WHEP != "edge.example.test" {
		t.Fatalf("server %+v scenario %q", r.Server, r.Scenario.WHEP)
	}
}

// WB-8: a run with a ramp offset records the seed, the bound and every viewer's
// offset, so the same starts can be scheduled again.
func TestReportCarriesTheRampOffsets(t *testing.T) {
	s := sc(3)
	seed := int64(7)
	s.RampOffsetSeed, s.RampOffsetMaxSeconds = &seed, 1
	vs := viewers(3, 0)
	offsets := scenario.RampOffsets(3, seed, time.Second)
	for i := range vs {
		vs[i].RampOffsetMs = f(float64(offsets[i]) / float64(time.Millisecond))
		vs[i].StartOffsetMs = *vs[i].RampOffsetMs
	}
	r := build(s, vs)
	b, err := r.JSON()
	if err != nil {
		t.Fatal(err)
	}
	js := string(b)
	for _, want := range []string{`"rampOffsetSeed": 7`, `"rampOffsetMaxSeconds": 1`} {
		if !strings.Contains(js, want) {
			t.Errorf("json lacks %s", want)
		}
	}
	if n := strings.Count(js, `"rampOffsetMs": `); n != 3 {
		t.Errorf("want one rampOffsetMs per viewer, got %d", n)
	}
	var back Report
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	for i, v := range back.Viewers {
		if v.RampOffsetMs == nil || time.Duration(*v.RampOffsetMs*float64(time.Millisecond)) != offsets[i] {
			t.Errorf("viewer %d: offset %v, want %v", i, v.RampOffsetMs, offsets[i])
		}
	}
	md := r.Markdown()
	if !strings.Contains(md, "ramp offset seed 7, up to 1s") {
		t.Errorf("markdown does not state the seed:\n%s", md)
	}
	if !strings.Contains(md, RampOffsetMethod) || len(r.Method) != len(Method)+1 {
		t.Errorf("method lines %v", r.Method)
	}
}

func TestReportWithoutTheRampOffsetIsUnchanged(t *testing.T) {
	r := build(sc(3), viewers(3, 0))
	b, err := r.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "rampOffset") || strings.Contains(r.Markdown(), "ramp offset") {
		t.Errorf("a run without the key mentions a ramp offset:\n%s", b)
	}
	if len(r.Method) != len(Method) {
		t.Errorf("method lines %v", r.Method)
	}
}
