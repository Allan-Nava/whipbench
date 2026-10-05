package runner_test

// WB-42: the live one-way delay series counts exactly the samples the report pools, and
// a run without a send log declares it with none.

import (
	"bytes"
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Allan-Nava/whipbench"
	"github.com/Allan-Nava/whipbench/internal/clip"
	"github.com/Allan-Nava/whipbench/internal/metrics"
	"github.com/Allan-Nava/whipbench/internal/runner"
	"github.com/Allan-Nava/whipbench/internal/scenario"
	"github.com/Allan-Nava/whipbench/internal/testserver"
)

const fingerprintCount = `whipbench_one_way_delay_seconds_count{source="fingerprint"} `

func exposition(l *metrics.Live) string {
	var b bytes.Buffer
	l.Write(&b)
	return b.String()
}

func TestOneWayDelaySeriesCountsTheReportsSamples(t *testing.T) {
	t.Parallel()
	srv, err := testserver.New(testserver.Options{RTC: loop})
	if err != nil {
		t.Fatal(err)
	}
	hs := httptest.NewServer(srv)
	defer func() { hs.Close(); srv.Close() }()
	sc := base("vp8", 3)
	sc.WHIP, sc.WHEP = hs.URL+"/whip", hs.URL+"/whep"
	c, err := clip.Load("vp8", whipbench.ClipVP8)
	if err != nil {
		t.Fatal(err)
	}
	live := &metrics.Live{}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rep, err := runner.Run(ctx, runner.WithLive(runner.Options{Command: "run", Scenario: sc, Clip: c, RTC: loop}, live))
	if err != nil {
		t.Fatal(err)
	}
	a := rep.Aggregate
	if !a.Valid || a.Joined != 3 {
		t.Fatalf("aggregate: %+v", a)
	}
	var perViewer uint64
	for _, v := range rep.Viewers {
		for _, b := range v.OneWayDelay {
			perViewer += b.Samples
		}
	}
	d := a.FingerprintDelay()
	if perViewer == 0 || d.Samples != perViewer {
		t.Fatalf("samples: %d pooled, %d over the viewers' blocks", d.Samples, perViewer)
	}
	body := exposition(live)
	if want := fmt.Sprintf("%s%d\n", fingerprintCount, perViewer); !strings.Contains(body, want) {
		t.Errorf("the live series must count every sample the report has, %d:\n%s", perViewer, body)
	}
}

func TestViewDeclaresOneWayDelayWithNoSample(t *testing.T) {
	t.Parallel()
	whep := published(t)
	sc := scenario.Scenario{Name: "split", Codec: "vp8", Viewers: 2, HoldSeconds: 2, JoinTimeoutSeconds: 5, WHEP: whep}
	live := &metrics.Live{}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rep, err := runner.Run(ctx, runner.WithLive(runner.Options{Command: "view", Scenario: sc, RTC: loop}, live))
	if err != nil {
		t.Fatal(err)
	}
	if rep.Aggregate.Joined != 2 || live.PacketsReceived.Load() == 0 {
		t.Fatalf("the viewers received nothing: %+v", rep.Aggregate)
	}
	if body := exposition(live); !strings.Contains(body, fingerprintCount+"0\n") {
		t.Errorf("a view has no send log: the series is declared with no sample:\n%s", body)
	}
}
