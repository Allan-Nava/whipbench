package report

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Allan-Nava/whipbench/internal/publisher"
	"github.com/Allan-Nava/whipbench/internal/rtpstats"
	"github.com/Allan-Nava/whipbench/internal/scenario"
	"github.com/Allan-Nava/whipbench/internal/stats"
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

// The 0.0.1 figure is packetTransit in the JSON and in the Method lines; no key, no
// definition is called latency (WB-1, D1), and "one-way delay" names one thing only: the
// OneWayDelayMethod line, never a reason, a constant or packet transit (D8).
func TestPacketTransitKeys(t *testing.T) {
	if n := strings.Count(strings.ToLower(OneWayDelayMethod), "one-way delay"); n != 1 {
		t.Errorf("OneWayDelayMethod says one-way delay %d times, want 1", n)
	}
	for name, r := range map[string]*Report{
		"pools":       buildPools(),
		"no send log": buildWithoutASendLog(),
		"no track":    buildDelay("run", &Fingerprint{LoopFrames: 120}, viewers(3, 0)),
	} {
		b, err := r.JSON()
		if err != nil {
			t.Fatal(err)
		}
		js := string(b)
		if n := strings.Count(strings.ToLower(js), "one-way delay"); n != 1 {
			t.Errorf("%s: want one-way delay exactly once (the Method line), got %d:\n%s", name, n, js)
		}
		if n := strings.Count(js, "\"packetTransit\": {"); n != 4 {
			t.Errorf("%s: want 4 packetTransit objects (aggregate + 3 viewers), got %d:\n%s", name, n, js)
		}
		if strings.Contains(strings.ToLower(js), "latency") {
			t.Errorf("%s: the 0.0.1 key or wording is back:\n%s", name, js)
		}
	}
}

func hist(vs ...float64) *stats.Histogram {
	h := stats.NewHistogram()
	for _, v := range vs {
		h.Add(v)
	}
	return h
}

func buildDelay(command string, fp *Fingerprint, vs []viewer.Result) *Report {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	return Build(Input{Command: command, Scenario: sc(len(vs)), Viewers: vs, StartedAt: now, FinishedAt: now, Fingerprint: fp})
}

func buildBlocks() *Report {
	vs := viewers(3, 0)
	vs[0].OneWayDelay = []viewer.OneWayDelay{{Source: "fingerprint", FrameEnd: "marker", CompleteFrames: 2, Hist: hist(10, 20)}}
	return buildDelay("run", &Fingerprint{LoopFrames: 120, LoopMin: 3990 * time.Millisecond}, vs)
}

func buildPools() *Report {
	vs := viewers(3, 0)
	vs[0].OneWayDelay = []viewer.OneWayDelay{{Source: "fingerprint", FrameEnd: "marker", CompleteFrames: 2, Hist: hist(10, 20)}}
	vs[1].OneWayDelay = []viewer.OneWayDelay{{FrameEnd: "timestamp", CompleteFrames: 1, Hist: hist(30)}}
	vs[2].OneWayDelay = []viewer.OneWayDelay{{CompleteFrames: 5, UnmatchedFrames: 5, Hist: hist()}}
	return buildDelay("run", &Fingerprint{LoopFrames: 120, LoopMin: 3990 * time.Millisecond}, vs)
}

func buildWithoutASendLog() *Report {
	return buildDelay("view", nil, viewers(3, 0))
}

const noValidSample = "no valid sample: 5 complete frames (5 unmatched, 0 invalid), 0 incomplete"

func TestOneWayDelayBlocks(t *testing.T) {
	r := buildBlocks()
	b, err := r.JSON()
	if err != nil {
		t.Fatal(err)
	}
	js := string(b)
	if n := strings.Count(js, "\"oneWayDelay\": ["); n != 4 {
		t.Errorf("want 4 oneWayDelay arrays (3 viewers + aggregate), got %d:\n%s", n, js)
	}
	for _, want := range []string{
		`"source": "fingerprint"`, `"frameEnd": "marker"`, `"completeFrames"`, `"incompleteFrames"`,
		`"samples"`, `"invalid"`, `"unmatchedFrames"`, `"viewersByFrameEnd"`, `"duplicateFrames"`,
		`"loopFrames": 120`, `"loopMinMs": 3990`,
	} {
		if !strings.Contains(js, want) {
			t.Errorf("missing %s in\n%s", want, js)
		}
	}
	if r.Schema != "whipbench.report/v0" {
		t.Errorf("schema %q", r.Schema)
	}
	if n := strings.Count(js, "\"packetTransit\": {"); n != 4 {
		t.Errorf("want 4 packetTransit objects, got %d", n)
	}
}

func TestOneWayDelayPools(t *testing.T) {
	r := buildPools()
	if n := len(r.Aggregate.OneWayDelay); n != 1 {
		t.Fatalf("want one pooled block, got %d", n)
	}
	p := r.Aggregate.FingerprintDelay()
	if !p.Available || p.Samples != 3 || p.Ms == nil || p.Ms.N != 3 || p.Ms.Min != 10 || p.Ms.Max != 30 {
		t.Errorf("pooled samples: %+v ms %+v", p, p.Ms)
	}
	if p.Viewers != 2 || p.ViewersByFrameEnd["marker"] != 1 || p.ViewersByFrameEnd["timestamp"] != 1 || len(p.ViewersByFrameEnd) != 2 {
		t.Errorf("pooled viewers: %d %v", p.Viewers, p.ViewersByFrameEnd)
	}
	if p.CompleteFrames != 8 || p.UnmatchedFrames != 5 {
		t.Errorf("pooled counts: complete %d unmatched %d", p.CompleteFrames, p.UnmatchedFrames)
	}
	if want := "pooled over 2 of 3 joined viewers; the other 1 had none (" + noValidSample + ")"; p.Reason != want {
		t.Errorf("reason %q, want %q", p.Reason, want)
	}
}

// buildSplit is buildPools with every sample also split by frame kind, as the viewer
// records it: viewer 0 has one keyframe and one delta frame, viewer 1 a delta frame only,
// viewer 2 no sample.
func buildSplit() *Report {
	vs := viewers(3, 0)
	vs[0].OneWayDelay = []viewer.OneWayDelay{{Source: "fingerprint", FrameEnd: "marker", CompleteFrames: 2,
		Hist: hist(10, 20), KeyHist: hist(20), DeltaHist: hist(10)}}
	vs[1].OneWayDelay = []viewer.OneWayDelay{{FrameEnd: "timestamp", CompleteFrames: 1,
		Hist: hist(30), KeyHist: hist(), DeltaHist: hist(30)}}
	vs[2].OneWayDelay = []viewer.OneWayDelay{{CompleteFrames: 5, UnmatchedFrames: 5,
		Hist: hist(), KeyHist: hist(), DeltaHist: hist()}}
	return buildDelay("run", &Fingerprint{LoopFrames: 120, LoopMin: 3990 * time.Millisecond}, vs)
}

// WB-44: keyframes and delta frames, per viewer and pooled, add up to the block's samples;
// a half with no sample has no summary; the pooled figure is unchanged by the split.
func TestOneWayDelaySplitsByFrameKind(t *testing.T) {
	r := buildSplit()
	for _, v := range r.Viewers {
		b := v.OneWayDelay[0]
		if b.Keyframes.Samples+b.DeltaFrames.Samples != b.Samples {
			t.Errorf("viewer %d: keyframes %d + delta frames %d != samples %d", v.ID, b.Keyframes.Samples, b.DeltaFrames.Samples, b.Samples)
		}
		for name, d := range map[string]viewer.FrameKindDelay{"keyframes": b.Keyframes, "deltaFrames": b.DeltaFrames} {
			if (d.Ms == nil) != (d.Samples == 0) || (d.Ms != nil && uint64(d.Ms.N) != d.Samples) {
				t.Errorf("viewer %d %s: %d samples, ms %+v", v.ID, name, d.Samples, d.Ms)
			}
		}
	}
	if k := r.Viewers[0].OneWayDelay[0].Keyframes; k.Ms == nil || k.Ms.Min != 20 || k.Ms.Max != 20 {
		t.Errorf("viewer 0 keyframes: %+v", k.Ms)
	}
	if k := r.Viewers[1].OneWayDelay[0].Keyframes; k.Samples != 0 || k.Ms != nil {
		t.Errorf("viewer 1 has no keyframe sample, got %+v", k)
	}
	p := r.Aggregate.FingerprintDelay()
	if p.Samples != 3 || p.Keyframes.Samples != 1 || p.DeltaFrames.Samples != 2 || p.Keyframes.Samples+p.DeltaFrames.Samples != p.Samples {
		t.Errorf("pooled split: samples %d, keyframes %d, delta frames %d", p.Samples, p.Keyframes.Samples, p.DeltaFrames.Samples)
	}
	if p.Keyframes.Ms == nil || p.Keyframes.Ms.Max != 20 || p.DeltaFrames.Ms == nil || p.DeltaFrames.Ms.Min != 10 || p.DeltaFrames.Ms.Max != 30 {
		t.Errorf("pooled split summaries: keyframes %+v, delta frames %+v", p.Keyframes.Ms, p.DeltaFrames.Ms)
	}
	if before := buildPools().Aggregate.FingerprintDelay(); *p.Ms != *before.Ms || p.Comparable != before.Comparable || p.Reason != before.Reason {
		t.Errorf("the split changed the pooled figure: %+v, was %+v", p.Ms, before.Ms)
	}
	b, err := r.JSON()
	if err != nil {
		t.Fatal(err)
	}
	js := string(b)
	if n := strings.Count(js, `"keyframes": {`); n != 4 {
		t.Errorf("want 4 keyframes objects (3 viewers + aggregate), got %d:\n%s", n, js)
	}
	if n := strings.Count(js, `"deltaFrames": {`); n != 4 {
		t.Errorf("want 4 deltaFrames objects (3 viewers + aggregate), got %d:\n%s", n, js)
	}
	md := r.Markdown()
	d := strings.Index(md, "| one-way delay (fingerprint, per frame) | 3 |")
	k := strings.Index(md, "| one-way delay, keyframes | 1 | 20.0 ms |")
	df := strings.Index(md, "| one-way delay, delta frames | 2 |")
	if d < 0 || k < d || df < k {
		t.Errorf("split rows at %d and %d, pooled at %d:\n%s", k, df, d, md)
	}
}

// WB-44: an unavailable block's split never carries a summary, whatever its histograms
// hold, and a half with no sample prints n 0 under an available figure.
func TestOneWayDelaySplitIsNeverANumberWithoutASample(t *testing.T) {
	vs := viewers(2, 0)
	vs[0].OneWayDelay = []viewer.OneWayDelay{{Reason: "codec \"av1\" has no fingerprint", Hist: hist(10), KeyHist: hist(10), DeltaHist: hist()}}
	vs[1].OneWayDelay = []viewer.OneWayDelay{{CompleteFrames: 2, Hist: hist(10, 11), KeyHist: hist(), DeltaHist: hist(10, 11)}}
	r := buildDelay("run", &Fingerprint{LoopFrames: 120}, vs)
	if b := r.Viewers[0].OneWayDelay[0]; b.Available || b.Keyframes.Ms != nil || b.DeltaFrames.Ms != nil || b.Keyframes.Samples != 1 {
		t.Errorf("unavailable viewer block: %+v", b)
	}
	p := r.Aggregate.FingerprintDelay()
	if p.Samples != 2 || p.Keyframes.Samples != 0 || p.Keyframes.Ms != nil || p.DeltaFrames.Samples != 2 {
		t.Errorf("pooled split: %+v", p)
	}
	if md := r.Markdown(); !strings.Contains(md, "| one-way delay, keyframes | 0 | — | — | — | — | — |") {
		t.Errorf("an empty half must print n 0:\n%s", md)
	}
	none := buildDelay("run", &Fingerprint{LoopFrames: 120}, viewers(2, 0)).Aggregate.FingerprintDelay()
	if none.Keyframes != (viewer.FrameKindDelay{}) || none.DeltaFrames != (viewer.FrameKindDelay{}) {
		t.Errorf("no sample at all: %+v %+v", none.Keyframes, none.DeltaFrames)
	}
}

func TestOneWayDelayZeroSamplesIsNeverANumber(t *testing.T) {
	vs := viewers(1, 0)
	vs[0].OneWayDelay = []viewer.OneWayDelay{{CompleteFrames: 5, UnmatchedFrames: 5, Hist: hist()}}
	r := buildDelay("run", &Fingerprint{LoopFrames: 120}, vs)
	for name, b := range map[string]OneWayDelay{"pooled": r.Aggregate.FingerprintDelay()} {
		if b.Available || b.Ms != nil || b.Reason != noValidSample {
			t.Errorf("%s: %+v", name, b)
		}
	}
	v := r.Viewers[0].OneWayDelay
	if len(v) != 1 || v[0].Available || v[0].Ms != nil || v[0].Reason != noValidSample || v[0].Source != viewer.SourceFingerprint {
		t.Errorf("viewer: %+v", v)
	}
	b, err := r.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `"ms":`) {
		t.Errorf("a one-way delay number with no sample:\n%s", b)
	}
}

func TestOneWayDelayWithoutASendLog(t *testing.T) {
	r := buildWithoutASendLog()
	const want = "the send log lives in the `run` process"
	for _, v := range r.Viewers {
		if len(v.OneWayDelay) != 1 || v.OneWayDelay[0].Available || !strings.Contains(v.OneWayDelay[0].Reason, want) {
			t.Errorf("viewer %d: %+v", v.ID, v.OneWayDelay)
		}
		if v.PacketTransit.Available {
			t.Errorf("viewer %d: packet transit changed: %+v", v.ID, v.PacketTransit)
		}
	}
	if p := r.Aggregate.FingerprintDelay(); p.Available || !strings.Contains(p.Reason, want) {
		t.Errorf("pooled: %+v", p)
	}
	if pt, before := r.Aggregate.PacketTransit, build(sc(3), viewers(3, 0)).Aggregate.PacketTransit; pt != before {
		t.Errorf("packet transit changed: %+v, was %+v", pt, before)
	}
}

func TestOneWayDelayNoTrack(t *testing.T) {
	r := buildDelay("run", &Fingerprint{LoopFrames: 120}, viewers(3, 0))
	for _, v := range r.Viewers {
		if len(v.OneWayDelay) != 1 || v.OneWayDelay[0].Reason != NoTrackReason {
			t.Errorf("viewer %d: %+v", v.ID, v.OneWayDelay)
		}
	}
	if p := r.Aggregate.FingerprintDelay(); p.Reason != NoTrackReason {
		t.Errorf("pooled reason %q", p.Reason)
	}
}

func TestOneWayDelayMethodLine(t *testing.T) {
	for _, want := range []string{
		"first-packet send to last-packet arrival", "incomplete 1 s after its first packet",
		"pending when it stops are not counted", "a viewer's first frame", "is not measured",
	} {
		if !strings.Contains(OneWayDelayMethod, want) {
			t.Errorf("OneWayDelayMethod misses %q", want)
		}
	}
	at, transit := -1, -1
	for i, m := range Method {
		if m == OneWayDelayMethod {
			at = i
		}
		if strings.HasPrefix(m, "Packet transit:") {
			transit = i
		}
	}
	if at < 0 || transit < 0 || at >= transit {
		t.Errorf("OneWayDelayMethod at %d, packet transit at %d", at, transit)
	}
}

func TestMarkdownLeadsWithOneWayDelay(t *testing.T) {
	md := buildPools().Markdown()
	d, j := strings.Index(md, "| one-way delay (fingerprint, per frame) |"), strings.Index(md, "| join: first keyframe |")
	if d < 0 || j < 0 || d > j {
		t.Errorf("one-way delay row at %d, join at %d:\n%s", d, j, md)
	}
	if !strings.Contains(md, "| delay p50 | delay p99 | transit p50 |") {
		t.Errorf("viewers header:\n%s", md)
	}
	md = buildWithoutASendLog().Markdown()
	d, p := strings.Index(md, "**One-way delay: unavailable** — no send log"), strings.Index(md, "**Packet transit: unavailable**")
	if d < 0 || p < 0 || d > p {
		t.Errorf("unavailable lines at %d and %d:\n%s", d, p, md)
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
