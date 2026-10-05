package report

import (
	"encoding/json"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// WB-17: CPU seconds over the run, utilisation over every core, peak goroutines.
func TestResourceWatch(t *testing.T) {
	start := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	cpu := 3 * time.Second // CPU the process spent before the run: not counted
	g := []int{12, 40, 25}
	w := newResourceWatch(start, func() (time.Duration, bool) { return cpu, true }, func() int { n := g[0]; g = g[1:]; return n })
	w.Observe() // 40
	cpu += 2 * time.Second
	r := w.Finish(start.Add(10 * time.Second)) // 25
	if r.CPUSeconds == nil || *r.CPUSeconds != 2 || r.CPUReason != "" {
		t.Fatalf("cpu seconds %v, reason %q", r.CPUSeconds, r.CPUReason)
	}
	if want := 100 * 2 / (10 * float64(runtime.NumCPU())); r.CPUUtilisationPercent == nil || *r.CPUUtilisationPercent != want {
		t.Errorf("utilisation %v, want %v", r.CPUUtilisationPercent, want)
	}
	if r.PeakGoroutines != 40 {
		t.Errorf("peak goroutines %d, want 40", r.PeakGoroutines)
	}
}

// Without a CPU reading the figures are absent with the reason, never a 0.
func TestResourcesWithoutCPUAreNeverZero(t *testing.T) {
	start := time.Now()
	w := newResourceWatch(start, func() (time.Duration, bool) { return 0, false }, func() int { return 7 })
	r := w.Finish(start.Add(time.Second))
	if r.CPUSeconds != nil || r.CPUUtilisationPercent != nil || r.CPUReason != NoCPUReason || r.PeakGoroutines != 7 {
		t.Fatalf("%+v", r)
	}
	rep := build(sc(1), viewers(1, 0))
	withResources(rep, r)
	js, err := json.Marshal(rep.Client)
	if err != nil {
		t.Fatal(err)
	}
	if s := string(js); strings.Contains(s, "cpuSeconds") || strings.Contains(s, "cpuUtilisationPercent") || !strings.Contains(s, `"peakGoroutines":7`) {
		t.Errorf("client JSON %s", s)
	}
	if md := rep.Markdown(); !strings.Contains(md, "| client resources | CPU "+NoCPUReason) {
		t.Errorf("markdown has no unavailable CPU row:\n%s", md)
	}
}

func TestResourcesInTheReport(t *testing.T) {
	now := time.Now()
	two, pc := 2.0, 12.5
	in := Input{Command: "run", StartedAt: now, FinishedAt: now.Add(time.Minute), Scenario: sc(1), Viewers: viewers(1, 0),
		Resources: &Resources{CPUSeconds: &two, CPUUtilisationPercent: &pc, PeakGoroutines: 90}}
	seed := int64(1)
	in.Scenario.RampOffsetSeed = &seed // Build rebuilds Method for it; the resources line must survive
	r := Build(in)
	if r.Client.Resources == nil || *r.Client.Resources.CPUSeconds != 2 {
		t.Fatalf("client resources %+v", r.Client.Resources)
	}
	if !slices.Contains(r.Method, ResourcesMethod) || !slices.Contains(r.Method, RampOffsetMethod) {
		t.Errorf("method lines: %q", r.Method)
	}
	if md := r.Markdown(); !strings.Contains(md, "| client resources | 2.0 CPU s, 12% of all cores, peak 90 goroutines") {
		t.Errorf("markdown:\n%s", md)
	}
	if b := Build(Input{Scenario: sc(1)}); b.Client.Resources != nil || slices.Contains(b.Method, ResourcesMethod) || strings.Contains(b.Markdown(), "client resources") {
		t.Error("a report built without resources carries them")
	}
	var nilWatch *ResourceWatch
	nilWatch.Observe()
	if nilWatch.Finish(now) != nil {
		t.Error("a nil watch finished with resources")
	}
}
