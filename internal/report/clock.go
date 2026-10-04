package report

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/Allan-Nava/whipbench/internal/viewer"
)

// Topology says whether the two ends of a delay ran in one process (WB-40, D6).
const (
	// TopologySingleProcess: this process published and viewed — `run` with a WHIP
	// endpoint — so one clock reads both ends.
	TopologySingleProcess = "single-process"
	// TopologySplit: the publisher ran elsewhere — `view`, or `run` without a WHIP
	// endpoint — so the two ends sit on different clocks.
	TopologySplit = "split"
)

// The clock methods a report can carry (D6).
const (
	// ClockMonotonic: one process's monotonic clock reads both ends; immune to steps.
	ClockMonotonic = "monotonic"
	// ClockSameWallClock: one host's wall clock reads both ends; WB-39's stamp.
	ClockSameWallClock = "same-wall-clock"
	// ClockNone: the publisher's clock was not measured, so no delay is comparable.
	ClockNone = "none"
)

// StepThreshold is how far wall-clock and monotonic elapsed time may drift apart over a
// run before the report records a step.
const StepThreshold = 100 * time.Microsecond

// MaxRankUncertaintyMs is the largest clock uncertainty a figure may carry and still be
// ranked against another report's (D8, Q7).
const MaxRankUncertaintyMs = 1.0

// NoClockReason: a split run whose publisher clock nobody measured (D6). WB-3's
// exchange is what will measure it.
const NoClockReason = "publisher clock not measured"

// Clock is how the two ends of a delay were put on one time base. OffsetMs and
// UncertaintyMs are absent when the method is none: a missing measurement is never a 0.
type Clock struct {
	Method        string   `json:"method"`
	OffsetMs      *float64 `json:"offsetMs,omitempty"`
	UncertaintyMs *float64 `json:"uncertaintyMs,omitempty"`
	// StepDetected: wall-clock and monotonic elapsed time over the run differ by more
	// than StepThreshold, so the wall clock was stepped or slewed while it ran.
	StepDetected bool `json:"stepDetected"`
}

// MonotonicClock is the clock of a single-process run: both ends on one monotonic clock,
// so the offset and its uncertainty are zero by construction, not by measurement.
func MonotonicClock(started, finished time.Time) *Clock {
	zero, unc := 0.0, 0.0
	return &Clock{Method: ClockMonotonic, OffsetMs: &zero, UncertaintyMs: &unc, StepDetected: StepDetected(started, finished)}
}

// StepDetected compares the wall-clock and the monotonic time elapsed between two
// instants taken with time.Now. Without a monotonic reading on both, Go subtracts wall
// clocks, the two are equal and no step can be seen.
func StepDetected(started, finished time.Time) bool {
	return stepped(finished.Round(0).Sub(started.Round(0)), finished.Sub(started))
}

// stepped is the comparison itself: more than StepThreshold apart, either way.
func stepped(wall, mono time.Duration) bool {
	d := wall - mono
	return d > StepThreshold || d < -StepThreshold
}

// comparability is a source block's verdict under the report's clock (D8): comparable
// only when the block is available, the clock was measured, its uncertainty is at most
// MaxRankUncertaintyMs and, for a wall-clock method, it did not step. The uncertainty is
// returned only for an available block, so an unavailable one carries no number.
func comparability(available bool, c Clock) (unc *float64, ok bool, reason string) {
	if !available {
		return nil, false, "no figure: the block is unavailable"
	}
	if c.Method == ClockNone || c.Method == "" {
		return nil, false, NoClockReason
	}
	if c.UncertaintyMs == nil {
		return nil, false, "clock uncertainty not measured"
	}
	u := *c.UncertaintyMs
	if u > MaxRankUncertaintyMs {
		return &u, false, fmt.Sprintf("clock uncertainty %s ms is above the %s ms a ranking allows", trimMs(u), trimMs(MaxRankUncertaintyMs))
	}
	if c.StepDetected && c.Method != ClockMonotonic {
		return &u, false, "the wall clock stepped during the run"
	}
	return &u, true, ""
}

func trimMs(v float64) string { return fmt.Sprintf("%g", v) }

// judgeViewer sets a viewer block's comparability under the report's clock.
func judgeViewer(o viewer.OneWayDelay, c Clock) viewer.OneWayDelay {
	o.UncertaintyMs, o.Comparable, o.NotComparableReason = comparability(o.Available, c)
	return o
}

// judge sets a pooled block's comparability under the report's clock.
func judge(o OneWayDelay, c Clock) OneWayDelay {
	o.UncertaintyMs, o.Comparable, o.NotComparableReason = comparability(o.Available, c)
	return o
}

// Rankable says whether two reports' one-way delay from one source may be ranked against
// each other, and when not, why (D8). Both blocks must be comparable with an uncertainty
// of at most MaxRankUncertaintyMs, both aggregates valid, the clips the same — codec and
// loop length, the clip identity a report carries — and every scenario key equal except
// the two endpoint hosts, which are what a comparison varies. Nothing is averaged: a
// source missing from either report refuses the ranking rather than falling back.
func Rankable(a, b *Report, source string) (bool, string) {
	if a == nil || b == nil {
		return false, "a report is missing"
	}
	reps := [2]*Report{a, b}
	var blocks [2]OneWayDelay
	for i, r := range reps {
		k := slices.IndexFunc(r.Aggregate.OneWayDelay, func(o OneWayDelay) bool { return o.Source == source })
		if k < 0 {
			return false, fmt.Sprintf("report %c has no %q source", 'A'+i, source)
		}
		blocks[i] = r.Aggregate.OneWayDelay[k]
	}
	for i, r := range reps {
		if !r.Aggregate.Valid {
			return false, fmt.Sprintf("report %c has no verdict: %s", 'A'+i, r.Aggregate.Verdict)
		}
	}
	for i, o := range blocks {
		if !o.Comparable {
			reason := o.NotComparableReason
			if reason == "" {
				reason = "it carries no comparability verdict"
			}
			return false, fmt.Sprintf("report %c's %s figure is not comparable: %s", 'A'+i, source, reason)
		}
	}
	for i, o := range blocks {
		if o.UncertaintyMs == nil || *o.UncertaintyMs > MaxRankUncertaintyMs {
			return false, fmt.Sprintf("report %c's %s figure has no clock uncertainty of %s ms or less", 'A'+i, source, trimMs(MaxRankUncertaintyMs))
		}
	}
	if a.Scenario.Codec != b.Scenario.Codec || blocks[0].LoopFrames != blocks[1].LoopFrames {
		return false, fmt.Sprintf("clips differ: %s of %d frames, %s of %d frames",
			a.Scenario.Codec, blocks[0].LoopFrames, b.Scenario.Codec, blocks[1].LoopFrames)
	}
	if k, ok := scenarioDiff(a, b); !ok {
		return false, "scenarios differ on " + k
	}
	return true, ""
}

// scenarioDiff compares the two scenarios key by key and returns the first that differs.
// Left out are the keys that label or reach a run without shaping the load: the endpoint
// hosts — two servers are what a ranking compares — the name, the bearer variable and
// the metrics address.
func scenarioDiff(a, b *Report) (string, bool) {
	m := [2]map[string]any{}
	for i, r := range [2]*Report{a, b} {
		s := r.Scenario
		s.WHIP, s.WHEP, s.Name, s.BearerEnv, s.Metrics = "", "", "", "", ""
		raw, err := json.Marshal(s)
		if err != nil {
			return "an unreadable scenario", false
		}
		if err := json.Unmarshal(raw, &m[i]); err != nil {
			return "an unreadable scenario", false
		}
	}
	keys := make([]string, 0, len(m[0])+len(m[1]))
	for k := range m[0] {
		keys = append(keys, k)
	}
	for k := range m[1] {
		if _, ok := m[0][k]; !ok {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	for _, k := range keys {
		va, oka := m[0][k]
		vb, okb := m[1][k]
		if oka != okb || fmt.Sprint(va) != fmt.Sprint(vb) {
			return k, false
		}
	}
	return "", true
}
