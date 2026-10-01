package report

import (
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

func TestLatencyUnavailableIsNeverANumber(t *testing.T) {
	vs := viewers(3, 0)
	for i := range vs {
		vs[i].Latency = viewer.Latency{Negotiated: false, Reason: "not negotiated: the WHEP answer did not accept abs-capture-time"}
	}
	r := build(sc(3), vs)
	if r.Aggregate.Latency.Available || r.Aggregate.Latency.Ms != nil {
		t.Fatalf("latency %+v", r.Aggregate.Latency)
	}
	md := r.Markdown()
	if !strings.Contains(md, "**Latency: unavailable** — not negotiated") || strings.Contains(md, "one-way delay (stamped") {
		t.Fatalf("markdown:\n%s", md)
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
