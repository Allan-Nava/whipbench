package metrics

import (
	"bytes"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestExposition(t *testing.T) {
	l := &Live{}
	l.ViewersTarget.Store(10)
	l.ViewersJoined.Add(3)
	l.Packet(1000)
	l.Packet(500)
	l.PacketTransit(3)    // ≤ 5 ms bucket
	l.PacketTransit(40)   // ≤ 50 ms
	l.PacketTransit(9000) // only +Inf
	l.PacketTransit(-1)   // ignored: a negative transit is a clock problem, not a sample

	rec := httptest.NewRecorder()
	l.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	body := rec.Body.String()
	for _, want := range []string{
		"whipbench_viewers_target 10\n",
		"whipbench_viewers_joined_total 3\n",
		"whipbench_rtp_packets_received_total 2\n",
		"whipbench_rtp_bytes_received_total 1500\n",
		`whipbench_packet_transit_seconds_bucket{le="0.002"} 0` + "\n",
		`whipbench_packet_transit_seconds_bucket{le="0.005"} 1` + "\n",
		`whipbench_packet_transit_seconds_bucket{le="0.05"} 2` + "\n",
		`whipbench_packet_transit_seconds_bucket{le="5"} 2` + "\n",
		`whipbench_packet_transit_seconds_bucket{le="+Inf"} 3` + "\n",
		"whipbench_packet_transit_seconds_count 3\n",
		"whipbench_packet_transit_seconds_sum 9.043\n",
		"# TYPE whipbench_packet_transit_seconds histogram\n",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in\n%s", want, body)
		}
	}
	for _, v := range unlabelledOneWayDelay(body) {
		t.Errorf("one-way delay only as whipbench_one_way_delay_seconds with a known source label (fingerprint now, stamp with WB-39): an unlabelled series would change meaning when the second source lands: %q", v)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain; version=0.0.4") {
		t.Errorf("content type %q", ct)
	}
}

func TestNilLiveIsANoOp(t *testing.T) {
	var l *Live
	l.Packet(10)
	l.PacketTransit(1)
	l.OneWayDelay("fingerprint", time.Millisecond)
}

// WB-42: the one-way delay histogram, labelled by source, declared before any sample.
func TestOneWayDelaySeries(t *testing.T) {
	var empty bytes.Buffer
	(&Live{}).Write(&empty)
	for _, want := range []string{
		"# TYPE whipbench_one_way_delay_seconds histogram\n",
		`whipbench_one_way_delay_seconds_bucket{source="fingerprint",le="0.0001"} 0` + "\n",
		`whipbench_one_way_delay_seconds_bucket{source="fingerprint",le="+Inf"} 0` + "\n",
		`whipbench_one_way_delay_seconds_count{source="fingerprint"} 0` + "\n",
		`whipbench_one_way_delay_seconds_sum{source="fingerprint"} 0` + "\n",
	} {
		if !strings.Contains(empty.String(), want) {
			t.Errorf("a process with no sample (a `view`) still declares %q:\n%s", want, empty.String())
		}
	}

	l := &Live{}
	l.OneWayDelay("fingerprint", 400*time.Microsecond) // ≤ 0.5 ms
	l.OneWayDelay("fingerprint", 3*time.Millisecond)   // ≤ 5 ms
	l.OneWayDelay("fingerprint", 2*time.Second)        // only +Inf
	l.OneWayDelay("fingerprint", -time.Millisecond)    // dropped: a sample is never negative
	l.OneWayDelay("stamp", time.Millisecond)           // dropped: not a source yet (WB-39)
	l.OneWayDelay("", time.Millisecond)                // dropped: no unlabelled sample
	var b bytes.Buffer
	l.Write(&b)
	body := b.String()
	for _, want := range []string{
		`whipbench_one_way_delay_seconds_bucket{source="fingerprint",le="0.0002"} 0` + "\n",
		`whipbench_one_way_delay_seconds_bucket{source="fingerprint",le="0.0005"} 1` + "\n",
		`whipbench_one_way_delay_seconds_bucket{source="fingerprint",le="0.005"} 2` + "\n",
		`whipbench_one_way_delay_seconds_bucket{source="fingerprint",le="1"} 2` + "\n",
		`whipbench_one_way_delay_seconds_bucket{source="fingerprint",le="+Inf"} 3` + "\n",
		`whipbench_one_way_delay_seconds_count{source="fingerprint"} 3` + "\n",
		`whipbench_one_way_delay_seconds_sum{source="fingerprint"} 2.0034` + "\n",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in\n%s", want, body)
		}
	}
	if strings.Contains(body, `source="stamp"`) || strings.Contains(body, `source=""`) {
		t.Errorf("a source outside OneWayDelaySources reached the exposition:\n%s", body)
	}
	if n := strings.Count(body, `whipbench_one_way_delay_seconds_bucket{source="fingerprint",`); n != len(OneWayDelayBucketsMs)+1 {
		t.Errorf("%d fingerprint buckets, want %d", n, len(OneWayDelayBucketsMs)+1)
	}
	if v := unlabelledOneWayDelay(body); len(v) != 0 {
		t.Errorf("unlabelled one-way delay lines: %q", v)
	}
	// The packet transit series is untouched by one-way delay samples.
	if !strings.Contains(body, "whipbench_packet_transit_seconds_count 0\n") {
		t.Errorf("packet transit moved:\n%s", body)
	}
}

// The guard refuses what D8 kept shut: a one-way delay sample without a source label,
// under the histogram's name or any other.
func TestGuardRefusesUnlabelledOneWayDelay(t *testing.T) {
	for _, bad := range []string{
		"whipbench_one_way_delay_seconds_count 3\n",
		`whipbench_one_way_delay_seconds_bucket{le="0.001"} 1` + "\n",
		`whipbench_one_way_delay_seconds_sum{source="stamp"} 1` + "\n",
		`whipbench_one_way_delay_seconds_count{viewer="1",source="fingerprint"} 1` + "\n",
		"whipbench_one_way_delay_ms 3\n",
	} {
		if len(unlabelledOneWayDelay(bad)) == 0 {
			t.Errorf("the guard let %q through", bad)
		}
	}
}

// unlabelledOneWayDelay returns every sample line naming one-way delay that is not
// whipbench_one_way_delay_seconds{_bucket,_sum,_count} with a source label from
// OneWayDelaySources and no label other than source and, on a bucket, le.
func unlabelledOneWayDelay(body string) []string {
	var bad []string
	for _, line := range strings.Split(body, "\n") {
		if !strings.Contains(line, "one_way_delay") || strings.HasPrefix(line, "#") {
			continue
		}
		name, rest, ok := strings.Cut(line, "{")
		labels, _, closed := strings.Cut(rest, "}")
		suffix, isSeries := strings.CutPrefix(name, "whipbench_one_way_delay_seconds_")
		if !ok || !closed || !isSeries || !slices.Contains([]string{"bucket", "sum", "count"}, suffix) {
			bad = append(bad, line)
			continue
		}
		var src string
		good := true
		for _, kv := range strings.Split(labels, ",") {
			k, v, _ := strings.Cut(kv, "=")
			switch {
			case k == "source":
				src = strings.Trim(v, `"`)
			case k == "le" && suffix == "bucket":
			default:
				good = false
			}
		}
		if !good || !slices.Contains(OneWayDelaySources[:], src) {
			bad = append(bad, line)
		}
	}
	return bad
}
