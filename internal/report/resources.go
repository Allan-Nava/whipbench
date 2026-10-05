package report

import (
	"fmt"
	"runtime"
	"slices"
	"time"

	"github.com/Allan-Nava/whipbench/internal/procstat"
)

// Resources is what the whipbench process spent over the run (WB-17), so a saturated
// client is visible in the report and not only in Prometheus. It describes the whole
// process — publisher, viewers and reassembly — and never the server.
type Resources struct {
	// CPUSeconds is user plus system CPU time between the run's start and its end, and
	// CPUUtilisationPercent is 100·CPUSeconds / (wall time × Client.CPUs). Both are absent,
	// with CPUReason, where the platform gives no reading — never a 0.
	CPUSeconds            *float64 `json:"cpuSeconds,omitempty"`
	CPUUtilisationPercent *float64 `json:"cpuUtilisationPercent,omitempty"`
	CPUReason             string   `json:"cpuReason,omitempty"`
	// PeakGoroutines is the most goroutines seen at the start, once a second while
	// viewers run, and at the end.
	PeakGoroutines int `json:"peakGoroutines"`
}

// NoCPUReason: the platform has no process CPU reading in internal/procstat.
const NoCPUReason = "no CPU reading on this platform: process CPU time is read through getrusage(2) on linux and darwin only"

// ResourcesMethod joins Method in a report that carries client resources.
const ResourcesMethod = "Client resources: what the whipbench process itself spent, publisher, viewers and reassembly together — not the server. cpuSeconds is user plus system CPU time (getrusage(2), linux and darwin; absent elsewhere, with cpuReason) from the run's start to its end; cpuUtilisationPercent is 100 × cpuSeconds / (wall time × the client's CPUs), so 100% is every core busy for the whole run. peakGoroutines is the most goroutines seen at the start, once a second while viewers run and at the end. A client near its ceiling delays and drops packets itself, and its figures then describe the client as much as the server."

// ResourceWatch measures a run's client resources. Observe and Finish are not safe for
// concurrent use: one goroutine observes, then the run finishes, as with StepWatch. A nil
// *ResourceWatch ignores Observe and finishes as nil.
type ResourceWatch struct {
	start      time.Time
	cpu0       time.Duration
	cpuOK      bool
	peak       int
	cpu        func() (time.Duration, bool)
	goroutines func() int
}

// NewResourceWatch starts measuring at start, a time.Now reading taken just before.
func NewResourceWatch(start time.Time) *ResourceWatch {
	return newResourceWatch(start, procstat.CPU, procstat.Goroutines)
}

func newResourceWatch(start time.Time, cpu func() (time.Duration, bool), goroutines func() int) *ResourceWatch {
	w := &ResourceWatch{start: start, cpu: cpu, goroutines: goroutines}
	w.cpu0, w.cpuOK = cpu()
	w.Observe()
	return w
}

// Observe samples the goroutine count; the run's timeline calls it once a second.
func (w *ResourceWatch) Observe() {
	if w == nil {
		return
	}
	w.peak = max(w.peak, w.goroutines())
}

// Finish observes once more and returns the run's resources, with end its finish time.
func (w *ResourceWatch) Finish(end time.Time) *Resources {
	if w == nil {
		return nil
	}
	w.Observe()
	r := &Resources{PeakGoroutines: w.peak}
	cpu1, ok := w.cpu()
	wall := end.Sub(w.start)
	switch {
	case !w.cpuOK || !ok:
		r.CPUReason = NoCPUReason
	default:
		s := max(cpu1-w.cpu0, 0).Seconds()
		r.CPUSeconds = &s
		if cores := runtime.NumCPU(); wall > 0 && cores > 0 {
			pc := 100 * s / (wall.Seconds() * float64(cores))
			r.CPUUtilisationPercent = &pc
		}
	}
	return r
}

// withResources records the run's client resources and their Method line.
func withResources(r *Report, res *Resources) {
	if res == nil {
		return
	}
	c := *res
	r.Client.Resources = &c
	r.Method = append(slices.Clone(r.Method), ResourcesMethod)
}

// resourcesRow is the Markdown header row for the client's resources, empty without them.
func resourcesRow(r *Resources) string {
	if r == nil {
		return ""
	}
	cpu := "CPU " + r.CPUReason
	if r.CPUSeconds != nil {
		cpu = fmt.Sprintf("%.1f CPU s", *r.CPUSeconds)
		if r.CPUUtilisationPercent != nil {
			cpu += fmt.Sprintf(", %.0f%% of all cores", *r.CPUUtilisationPercent)
		}
	}
	return fmt.Sprintf("| client resources | %s, peak %d goroutines (the whole process, not the server) |\n", esc(cpu), r.PeakGoroutines)
}
