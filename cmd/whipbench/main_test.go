package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Allan-Nava/whipbench/internal/report"
	"github.com/Allan-Nava/whipbench/internal/rtc"
	"github.com/Allan-Nava/whipbench/internal/stats"
	"github.com/Allan-Nava/whipbench/internal/testserver"
)

func TestUsage(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run(nil, &out, &errb); code != exitUsage {
		t.Fatalf("no args: exit %d", code)
	}
	if code := run([]string{"frobnicate"}, &out, &errb); code != exitUsage {
		t.Fatalf("unknown command: exit %d", code)
	}
	if code := run([]string{"view"}, &out, &errb); code != exitUsage || !strings.Contains(errb.String(), "whep is required") {
		t.Fatalf("view without --whep: exit %d, %q", code, errb.String())
	}
	if code := run([]string{"run"}, &out, &errb); code != exitUsage {
		t.Fatalf("run without a file: exit %d", code)
	}
	out.Reset()
	if code := run([]string{"version"}, &out, &errb); code != exitOK || !strings.HasPrefix(out.String(), "whipbench ") {
		t.Fatalf("version: exit %d, %q", code, out.String())
	}
}

func TestBearerEnvMustBeSet(t *testing.T) {
	t.Setenv("WHIPBENCH_TEST_EMPTY", "")
	var out, errb bytes.Buffer
	code := run([]string{"view", "--whep", "http://127.0.0.1:1/whep", "--bearer-env", "WHIPBENCH_TEST_EMPTY"}, &out, &errb)
	if code != exitUsage {
		t.Fatalf("exit %d, %q", code, errb.String())
	}
}

// The whole CLI path: a scenario file, a relay, a report on disk with no token.
func TestRunWritesReports(t *testing.T) {
	srv, err := testserver.New(testserver.Options{RTC: rtc.Options{LoopbackOnly: true}, Token: "CLISECRET"})
	if err != nil {
		t.Fatal(err)
	}
	hs := httptest.NewServer(srv)
	defer func() { hs.Close(); srv.Close() }()
	t.Setenv("WHIPBENCH_TEST_TOKEN", "CLISECRET")

	dir := t.TempDir()
	scen := filepath.Join(dir, "s.json")
	body := fmt.Sprintf(`{"name":"cli test","whip":%q,"whep":%q,"viewers":2,"rampSeconds":0.5,"holdSeconds":2,
		"warmupSeconds":0.5,"bearerEnv":"WHIPBENCH_TEST_TOKEN","loopbackOnly":true}`,
		hs.URL+"/whip?key=CLIKEY", hs.URL+"/whep?key=CLIKEY")
	if err := os.WriteFile(scen, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	code := run([]string{"run", scen, "--out", filepath.Join(dir, "out"), "--ramp-offset-seed", "7", "--ramp-offset-max", "200ms"}, &out, &errb)
	if code != exitOK {
		t.Fatalf("exit %d\nstdout %s\nstderr %s", code, out.String(), errb.String())
	}
	files, _ := filepath.Glob(filepath.Join(dir, "out", "whipbench-*-cli-test.*"))
	if len(files) != 2 {
		t.Fatalf("report files %v", files)
	}
	for _, f := range files {
		b, _ := os.ReadFile(f) //nolint:gosec // test
		for _, bad := range []string{"CLISECRET", "CLIKEY", "key="} {
			if bytes.Contains(b, []byte(bad)) {
				t.Errorf("%s contains %q", f, bad)
			}
		}
		if strings.HasSuffix(f, ".json") {
			var r struct {
				Aggregate struct{ Valid bool } `json:"aggregate"`
				Scenario  struct {
					BearerEnv            string
					RampOffsetSeed       *int64
					RampOffsetMaxSeconds float64
				}
				Viewers []struct{ RampOffsetMs *float64 }
			}
			if err := json.Unmarshal(b, &r); err != nil || !r.Aggregate.Valid || r.Scenario.BearerEnv != "WHIPBENCH_TEST_TOKEN" {
				t.Errorf("json report: %v %+v", err, r)
			}
			if r.Scenario.RampOffsetSeed == nil || *r.Scenario.RampOffsetSeed != 7 || r.Scenario.RampOffsetMaxSeconds != 0.2 {
				t.Errorf("the flags did not reach the report's scenario: %+v", r.Scenario)
			}
			for i, v := range r.Viewers {
				if v.RampOffsetMs == nil || *v.RampOffsetMs < 0 || *v.RampOffsetMs >= 200 {
					t.Errorf("viewer %d: ramp offset %v", i, v.RampOffsetMs)
				}
			}
		}
	}
	for _, bad := range []string{"CLISECRET", "CLIKEY"} {
		if strings.Contains(out.String()+errb.String(), bad) {
			t.Errorf("console output contains %q", bad)
		}
	}
}

// WB-8's flags are checked like the scenario keys they set.
func TestRampOffsetFlags(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"view", "--whep", "http://127.0.0.1:1/whep", "--ramp-offset-max", "1s"}, "needs rampOffsetSeed"},
		{[]string{"view", "--whep", "http://127.0.0.1:1/whep", "--ramp-offset-seed", "7", "--ramp-offset-max", "-1s"}, "rampOffsetMaxSeconds"},
		{[]string{"view", "--whep", "http://127.0.0.1:1/whep", "--ramp-offset-seed", "seven"}, "ramp-offset-seed"},
	} {
		var out, errb bytes.Buffer
		if code := run(tc.args, &out, &errb); code != exitUsage || !strings.Contains(errb.String(), tc.want) {
			t.Errorf("%v: exit %d, stderr %q, want exit %d mentioning %q", tc.args, code, errb.String(), exitUsage, tc.want)
		}
	}
	dir := t.TempDir()
	scen := filepath.Join(dir, "s.json")
	if err := os.WriteFile(scen, []byte(`{"whep":"http://127.0.0.1:1/whep","viewers":1,"holdSeconds":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := run([]string{"run", scen, "--ramp-offset-max", "1s"}, &out, &errb); code != exitUsage || !strings.Contains(errb.String(), "needs rampOffsetSeed") {
		t.Errorf("run --ramp-offset-max without a seed: exit %d, %q", code, errb.String())
	}
}

// The headline leads with one-way delay, the figure servers are ranked by, and keeps the
// join, loss and packet transit line as it was.
func TestHeadline(t *testing.T) {
	a := report.Aggregate{
		OneWayDelay:   []report.OneWayDelay{{Source: "fingerprint", Available: true, Ms: &stats.Summary{P50: 12.3, P99: 45.6}}},
		PacketTransit: report.PacketTransit{Reason: "not negotiated"},
	}
	want := "one-way delay (fingerprint) p50 12.3 ms, p99 45.6 ms\njoin (first keyframe) p50 0 ms, p95 0 ms; loss 0.000%; packet transit unavailable: not negotiated\n"
	if got := headline(a); got != want {
		t.Errorf("available:\ngot  %q\nwant %q", got, want)
	}
	a.OneWayDelay = []report.OneWayDelay{{Source: "fingerprint", Reason: report.NoSendLogReason}}
	got := headline(a)
	if !strings.HasPrefix(got, "one-way delay unavailable: no send log") {
		t.Errorf("unavailable: %q", got)
	}
	if i, j := strings.Index(got, "one-way delay"), strings.Index(got, "packet transit"); j < i {
		t.Errorf("packet transit before one-way delay: %q", got)
	}
}
