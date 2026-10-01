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

	"github.com/Allan-Nava/whipbench/internal/rtc"
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
	code := run([]string{"run", scen, "--out", filepath.Join(dir, "out")}, &out, &errb)
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
				Scenario  struct{ BearerEnv string }
			}
			if err := json.Unmarshal(b, &r); err != nil || !r.Aggregate.Valid || r.Scenario.BearerEnv != "WHIPBENCH_TEST_TOKEN" {
				t.Errorf("json report: %v %+v", err, r)
			}
		}
	}
	for _, bad := range []string{"CLISECRET", "CLIKEY"} {
		if strings.Contains(out.String()+errb.String(), bad) {
			t.Errorf("console output contains %q", bad)
		}
	}
}
