package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExposition(t *testing.T) {
	l := &Live{}
	l.ViewersTarget.Store(10)
	l.ViewersJoined.Add(3)
	l.Packet(1000)
	l.Packet(500)
	l.Delay(3)    // ≤ 5 ms bucket
	l.Delay(40)   // ≤ 50 ms
	l.Delay(9000) // only +Inf
	l.Delay(-1)   // ignored: a negative delay is a clock problem, not a sample

	rec := httptest.NewRecorder()
	l.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	body := rec.Body.String()
	for _, want := range []string{
		"whipbench_viewers_target 10\n",
		"whipbench_viewers_joined_total 3\n",
		"whipbench_rtp_packets_received_total 2\n",
		"whipbench_rtp_bytes_received_total 1500\n",
		`whipbench_one_way_delay_seconds_bucket{le="0.002"} 0` + "\n",
		`whipbench_one_way_delay_seconds_bucket{le="0.005"} 1` + "\n",
		`whipbench_one_way_delay_seconds_bucket{le="0.05"} 2` + "\n",
		`whipbench_one_way_delay_seconds_bucket{le="5"} 2` + "\n",
		`whipbench_one_way_delay_seconds_bucket{le="+Inf"} 3` + "\n",
		"whipbench_one_way_delay_seconds_count 3\n",
		"whipbench_one_way_delay_seconds_sum 9.043\n",
		"# TYPE whipbench_one_way_delay_seconds histogram\n",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in\n%s", want, body)
		}
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain; version=0.0.4") {
		t.Errorf("content type %q", ct)
	}
}

func TestNilLiveIsANoOp(t *testing.T) {
	var l *Live
	l.Packet(10)
	l.Delay(1)
}
