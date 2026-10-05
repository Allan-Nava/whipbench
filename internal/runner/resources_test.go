package runner_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/Allan-Nava/whipbench/internal/procstat"
	"github.com/Allan-Nava/whipbench/internal/report"
	"github.com/Allan-Nava/whipbench/internal/testserver"
)

// WB-17: a run's report says what the client itself spent, sampled by the timeline.
func TestReportCarriesClientResources(t *testing.T) {
	t.Parallel()
	rep := run(t, testserver.Options{}, base("vp8", 3), "")
	r := rep.Client.Resources
	if r == nil {
		t.Fatal("no client resources in the report")
	}
	// Three viewers each hold a peer connection with goroutines of its own.
	if r.PeakGoroutines < 3*2 {
		t.Errorf("peak goroutines %d with 3 viewers and a publisher", r.PeakGoroutines)
	}
	if procstat.CPUAvailable {
		if r.CPUSeconds == nil || *r.CPUSeconds <= 0 || r.CPUUtilisationPercent == nil ||
			*r.CPUUtilisationPercent <= 0 || *r.CPUUtilisationPercent > 100 {
			t.Errorf("cpu %v s, %v%%", r.CPUSeconds, r.CPUUtilisationPercent)
		}
	} else if r.CPUSeconds != nil || r.CPUReason != report.NoCPUReason {
		t.Errorf("cpu without a reading: %+v", r)
	}
	if !slices.Contains(rep.Method, report.ResourcesMethod) {
		t.Error("no client resources Method line")
	}
	if !strings.Contains(rep.Markdown(), "| client resources | ") {
		t.Error("no client resources row in the Markdown")
	}
}
