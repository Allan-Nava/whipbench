package report

import (
	"strings"
	"testing"
	"time"

	"github.com/Allan-Nava/whipbench/internal/viewer"
)

func TestStepped(t *testing.T) {
	for _, c := range []struct {
		wall, mono time.Duration
		want       bool
	}{
		{time.Minute, time.Minute, false},
		{time.Minute + 100*time.Microsecond, time.Minute, false}, // exactly 0.1 ms: not more than
		{time.Minute + 101*time.Microsecond, time.Minute, true},
		{time.Minute - 101*time.Microsecond, time.Minute, true}, // a step back counts too
		{time.Minute + time.Second, time.Minute, true},
	} {
		if got := stepped(c.wall, c.mono); got != c.want {
			t.Errorf("stepped(%v, %v) = %v, want %v", c.wall, c.mono, got, c.want)
		}
	}
}

func TestStepDetectedOnRealTimes(t *testing.T) {
	start := time.Now()
	if StepDetected(start, start.Add(time.Minute)) {
		t.Error("monotonic readings that agree with the wall clock reported a step")
	}
	// Without monotonic readings Go subtracts wall clocks on both sides: no step is seen.
	if StepDetected(start.Round(0), start.Round(0).Add(time.Minute)) {
		t.Error("times without a monotonic reading reported a step")
	}
	if StepDetected(start, start.Round(0).Add(time.Minute)) {
		t.Error("one time without a monotonic reading reported a step")
	}
}

func TestMonotonicClock(t *testing.T) {
	start := time.Now()
	c := MonotonicClock(start, start.Add(time.Second))
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
	now := time.Now()
	r := buildClock("run", TopologySingleProcess, MonotonicClock(now, now.Add(time.Minute)), true)
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

func TestMarkdownShowsTheClock(t *testing.T) {
	now := time.Now()
	md := buildClock("run", TopologySingleProcess, MonotonicClock(now, now.Add(time.Minute)), true).Markdown()
	if !strings.Contains(md, "| clock | monotonic, offset 0 ms ± 0 ms, no step |") {
		t.Errorf("markdown:\n%s", md)
	}
	if strings.Contains(md, "not comparable") {
		t.Errorf("a comparable figure marked not comparable:\n%s", md)
	}
}

func TestRankable(t *testing.T) {
	now := time.Now()
	run := func() *Report {
		return buildClock("run", TopologySingleProcess, MonotonicClock(now, now.Add(time.Minute)), true)
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
		{"name differs", run, func() *Report {
			r := run()
			r.Scenario.Name = "other"
			return r
		}, fp, false, "scenarios differ on name"},
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
