package report

import (
	"strings"
	"testing"
	"time"

	"github.com/Allan-Nava/whipbench/internal/viewer"
)

func TestStepped(t *testing.T) {
	for _, c := range []struct {
		name       string
		wall, mono time.Duration
		want       bool
	}{
		{"equal", time.Second, time.Second, false},
		// One second allows 0.1 ms plus a 500 ppm slew: 0.6 ms.
		{"0.6 ms in a second: the allowance itself", time.Second + 600*time.Microsecond, time.Second, false},
		{"0.601 ms in a second", time.Second + 601*time.Microsecond, time.Second, true},
		{"a step back counts too", time.Second - 601*time.Microsecond, time.Second, true},
		{"a 1 ms step in a second", time.Second + time.Millisecond, time.Second, true},
		// The drift measured on 2026-10-04, 2.8 ppm, over a 42 s run is 0.12 ms: the old
		// whole-run bound of 0.1 ms called it a step; per interval it never is one.
		{"2.8 ppm over 42 s", 42*time.Second + 118*time.Microsecond, 42 * time.Second, false},
		{"2.8 ppm over one second", time.Second + 3*time.Microsecond, time.Second, false},
		{"a 0.1 ms step in an instant", 100*time.Microsecond + 10*time.Millisecond, 10 * time.Millisecond, false},
		{"0.2 ms in 10 ms", 200*time.Microsecond + 10*time.Millisecond, 10 * time.Millisecond, true},
	} {
		if got := stepped(c.wall, c.mono); got != c.want {
			t.Errorf("%s: stepped(%v, %v) = %v, want %v", c.name, c.wall, c.mono, got, c.want)
		}
	}
}

func TestStepWatch(t *testing.T) {
	start := time.Now()
	w := NewStepWatch(start)
	for i := 1; i <= 60; i++ {
		w.Observe(start.Add(time.Duration(i) * time.Second))
	}
	if w.Stepped() {
		t.Error("readings whose wall and monotonic parts agree reported a step")
	}
	// A reading without a monotonic part is subtracted as wall clock on both sides.
	w = NewStepWatch(start.Round(0))
	w.Observe(start.Round(0).Add(time.Minute))
	if w.Stepped() {
		t.Error("readings without a monotonic part reported a step")
	}
	// A wall clock stepped 1 s forward in one interval, after a minute of steady ones: one
	// interval is enough, and later steady ones do not clear it.
	w = NewStepWatch(start)
	for i := 0; i < 60; i++ {
		w.interval(time.Second+3*time.Microsecond, time.Second)
	}
	if w.Stepped() {
		t.Error("a minute at 3 ppm reported a step")
	}
	w.interval(2*time.Second, time.Second)
	w.interval(time.Second, time.Second)
	if !w.Stepped() {
		t.Error("a 1 s wall-clock jump went unseen")
	}
}

func TestMonotonicClock(t *testing.T) {
	c := MonotonicClock(false)
	if c.Method != ClockMonotonic || c.OffsetMs == nil || *c.OffsetMs != 0 || c.UncertaintyMs == nil || *c.UncertaintyMs != 0 || c.StepDetected {
		t.Errorf("%+v", c)
	}
}

func TestComparability(t *testing.T) {
	mono := Clock{Method: ClockMonotonic, OffsetMs: f(0), UncertaintyMs: f(0)}
	wall := func(u float64, step bool) Clock {
		return Clock{Method: ClockSameWallClock, OffsetMs: f(0), UncertaintyMs: f(u), StepDetected: step}
	}
	stepMono := mono
	stepMono.StepDetected = true
	exchange := func(u float64, step bool) Clock {
		return *ExchangeClock([]ClockPoint{{TS: 0.1, OffsetMs: 250, RTTMs: 2 * u}}, u, step)
	}
	for _, c := range []struct {
		name      string
		available bool
		clock     Clock
		ok        bool
		unc       bool
		reason    string
	}{
		{"monotonic", true, mono, true, true, ""},
		{"monotonic is immune to a step", true, stepMono, true, true, ""},
		{"wall clock at the limit", true, wall(1, false), true, true, ""},
		{"wall clock above the limit", true, wall(1.5, false), false, true, "clock uncertainty 1.5 ms is above the 1 ms"},
		{"wall clock stepped", true, wall(0.2, true), false, true, "stepped"},
		{"exchange", true, exchange(0.3, false), true, true, ""},
		{"exchange above the limit", true, exchange(4, false), false, true, "clock uncertainty 4 ms is above"},
		{"exchange stepped, a wall-clock method", true, exchange(0.3, true), false, true, "stepped"},
		{"clock peer silent", true, *UnmeasuredClock(NoAnswerReason, false), false, false, NoClockReason + ": " + NoAnswerReason},
		{"unavailable", false, mono, false, false, "unavailable"},
		{"clock none", true, Clock{Method: ClockNone}, false, false, NoClockReason},
		{"no method at all", true, Clock{}, false, false, NoClockReason},
		{"uncertainty not measured", true, Clock{Method: ClockSameWallClock}, false, false, "not measured"},
	} {
		unc, ok, reason := comparability(c.available, c.clock)
		if ok != c.ok || (unc != nil) != c.unc || !strings.Contains(reason, c.reason) || (c.ok && reason != "") {
			t.Errorf("%s: ok %v unc %v reason %q", c.name, ok, unc, reason)
		}
	}
}

func buildClock(command, topology string, c *Clock, withSamples bool) *Report {
	vs := viewers(3, 0)
	if withSamples {
		for i := range vs {
			vs[i].OneWayDelay = []viewer.OneWayDelay{{Source: viewer.SourceFingerprint, FrameEnd: "marker", CompleteFrames: 2, Hist: hist(10, 20)}}
		}
	}
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	var fp *Fingerprint
	if topology == TopologySingleProcess {
		fp = &Fingerprint{LoopFrames: 120, LoopMin: 3990 * time.Millisecond}
	}
	return Build(Input{Command: command, Scenario: sc(len(vs)), Viewers: vs, StartedAt: now, FinishedAt: now.Add(time.Minute),
		Fingerprint: fp, Topology: topology, Clock: c})
}

func TestReportCarriesTopologyAndClock(t *testing.T) {
	r := buildClock("run", TopologySingleProcess, MonotonicClock(false), true)
	if r.Topology != TopologySingleProcess || r.Clock.Method != ClockMonotonic || r.Clock.StepDetected {
		t.Errorf("run: %q %+v", r.Topology, r.Clock)
	}
	p := r.Aggregate.FingerprintDelay()
	if !p.Comparable || p.NotComparableReason != "" || p.UncertaintyMs == nil || *p.UncertaintyMs != 0 {
		t.Errorf("run pooled block: %+v", p)
	}
	for _, v := range r.Viewers {
		if o := v.OneWayDelay[0]; !o.Comparable || o.UncertaintyMs == nil {
			t.Errorf("run viewer %d: %+v", v.ID, o)
		}
	}
	b, err := r.JSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"topology": "single-process"`, `"method": "monotonic"`, `"offsetMs": 0`, `"stepDetected": false`, `"comparable": true`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("missing %s in\n%s", want, b)
		}
	}
}

func TestViewHasNoClockAndNothingComparable(t *testing.T) {
	r := buildClock("view", "", nil, false)
	if r.Topology != TopologySplit || r.Clock.Method != ClockNone || r.Clock.OffsetMs != nil || r.Clock.UncertaintyMs != nil {
		t.Errorf("view: %q %+v", r.Topology, r.Clock)
	}
	p := r.Aggregate.FingerprintDelay()
	if p.Comparable || p.UncertaintyMs != nil || p.NotComparableReason == "" {
		t.Errorf("view pooled block: %+v", p)
	}
	b, err := r.JSON()
	if err != nil {
		t.Fatal(err)
	}
	js := string(b)
	for _, banned := range []string{`"offsetMs"`, `"uncertaintyMs"`, `"comparable": true`} {
		if strings.Contains(js, banned) {
			t.Errorf("a split run without a measured clock carries %s:\n%s", banned, js)
		}
	}
	if !strings.Contains(js, `"method": "none"`) || !strings.Contains(js, `"topology": "split"`) {
		t.Errorf("view clock or topology missing:\n%s", js)
	}
}

// An available figure under an unmeasured clock — what a split run with a send log of
// its own would be — is a number, but not a comparable one.
func TestAvailableButNotComparable(t *testing.T) {
	r := buildClock("run", TopologySingleProcess, &Clock{Method: ClockNone}, true)
	p := r.Aggregate.FingerprintDelay()
	if !p.Available || p.Comparable || p.UncertaintyMs != nil || p.NotComparableReason != NoClockReason {
		t.Errorf("%+v", p)
	}
	md := r.Markdown()
	for _, want := range []string{"| topology | single-process |", "| clock | none — " + NoClockReason + " |", "not comparable** with another report's — " + NoClockReason} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown misses %q:\n%s", want, md)
		}
	}
}

func TestExchangeClock(t *testing.T) {
	pts := []ClockPoint{{TS: 0.2, OffsetMs: 250.012, RTTMs: 0.08}, {TS: 30.2, OffsetMs: 250.4, RTTMs: 0.1}}
	c := ExchangeClock(pts, 0.05, false)
	if c.Method != ClockExchange || *c.OffsetMs != 250.012 || *c.UncertaintyMs != 0.05 || len(c.Points) != 2 || c.Reason != "" {
		t.Fatalf("exchange clock %+v", c)
	}
	r := buildClock("view", TopologySplit, c, false)
	md := r.Markdown()
	if !strings.Contains(md, "| clock | exchange, offset 250.012 ms ± 0.05 ms, 2 points, no step |") {
		t.Errorf("markdown:\n%s", md)
	}
	js, err := r.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(js), `"points": [`) || !strings.Contains(string(js), `"tS": 30.2`) || !strings.Contains(string(js), `"rttMs": 0.08`) {
		t.Errorf("json:\n%s", js)
	}
	if !strings.Contains(strings.Join(r.Method, "\n"), "Clock exchange:") {
		t.Error("an exchange report does not carry the exchange's definition")
	}

	// No point at all: none, with the reason, and no number standing in for one.
	none := ExchangeClock(nil, 0, false)
	if none.Method != ClockNone || none.Reason != NoAnswerReason || none.OffsetMs != nil || none.UncertaintyMs != nil || none.Points != nil {
		t.Fatalf("no points: %+v", none)
	}
	r = buildClock("view", TopologySplit, none, false)
	if md := r.Markdown(); !strings.Contains(md, "| clock | none — publisher clock not measured: clock peer did not answer |") {
		t.Errorf("markdown:\n%s", md)
	}
	js, _ = r.JSON()
	if strings.Contains(string(js), "offsetMs\": 0") || !strings.Contains(string(js), `"reason": "clock peer did not answer"`) {
		t.Errorf("json:\n%s", js)
	}
}

func TestMarkdownShowsTheClock(t *testing.T) {
	md := buildClock("run", TopologySingleProcess, MonotonicClock(false), true).Markdown()
	if !strings.Contains(md, "| clock | monotonic, offset 0 ms ± 0 ms, no step |") {
		t.Errorf("markdown:\n%s", md)
	}
	if strings.Contains(md, "not comparable") {
		t.Errorf("a comparable figure marked not comparable:\n%s", md)
	}
}

func TestRankable(t *testing.T) {
	run := func() *Report {
		return buildClock("run", TopologySingleProcess, MonotonicClock(false), true)
	}
	fp := viewer.SourceFingerprint
	for _, c := range []struct {
		name   string
		a, b   func() *Report
		source string
		ok     bool
		reason string
	}{
		{"same scenario, other host", run, func() *Report {
			r := run()
			r.Scenario.WHEP, r.Server.WHEPHost = "other.test", "other.test"
			return r
		}, fp, true, ""},
		{"a report missing", run, func() *Report { return nil }, fp, false, "missing"},
		{"source missing", run, run, "stamp", false, `no "stamp" source`},
		{"no verdict", run, func() *Report {
			r := run()
			r.Aggregate.Valid, r.Aggregate.Verdict = false, "no verdict: the run was interrupted"
			return r
		}, fp, false, "report B has no verdict"},
		{"view against run", func() *Report { return buildClock("view", "", nil, false) }, run, fp, false, "report A's fingerprint figure is not comparable"},
		{"uncertainty above 1 ms", run, func() *Report {
			r := run()
			r.Aggregate.OneWayDelay[0].UncertaintyMs = f(1.5) // a report that claims comparable anyway
			return r
		}, fp, false, "no clock uncertainty of 1 ms or less"},
		{"a report from before WB-40", run, func() *Report {
			r := run()
			r.Aggregate.OneWayDelay[0].Comparable, r.Aggregate.OneWayDelay[0].NotComparableReason = false, ""
			return r
		}, fp, false, "no comparability verdict"},
		{"codec differs", run, func() *Report {
			r := run()
			r.Scenario.Codec = "h264"
			return r
		}, fp, false, "clips differ"},
		{"loop differs", run, func() *Report {
			r := run()
			r.Aggregate.OneWayDelay[0].LoopFrames = 240
			return r
		}, fp, false, "clips differ"},
		{"viewers differ", run, func() *Report {
			r := run()
			r.Scenario.Viewers = 50
			return r
		}, fp, false, "scenarios differ on viewers"},
		{"duration differs", run, func() *Report {
			r := run()
			r.Scenario.HoldSeconds = 30
			return r
		}, fp, false, "scenarios differ on holdSeconds"},
		{"name, bearer variable and metrics address are not compared", run, func() *Report {
			r := run()
			r.Scenario.Name, r.Scenario.BearerEnv, r.Scenario.Metrics = "other", "TOKEN", "127.0.0.1:9464"
			return r
		}, fp, true, ""},
		{"includeLoopback is compared", run, func() *Report {
			r := run()
			r.Scenario.IncludeLoopback = !r.Scenario.IncludeLoopback
			return r
		}, fp, false, "scenarios differ on includeLoopback"},
		{"the WHIP host is not compared", run, func() *Report {
			r := run()
			r.Scenario.WHIP = "whip.test"
			return r
		}, fp, true, ""},
	} {
		ok, reason := Rankable(c.a(), c.b(), c.source)
		if ok != c.ok || !strings.Contains(reason, c.reason) || (ok && reason != "") {
			t.Errorf("%s: %v %q", c.name, ok, reason)
		}
	}
}
