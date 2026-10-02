// Package metrics keeps the live counters of a run and serves them in the
// Prometheus text exposition format (version 0.0.4) at /metrics.
//
// It is written by hand rather than with the Prometheus client library: a dozen
// counters and one histogram do not justify the dependency tree, and the format is
// a few lines of text. Every series is prefixed whipbench_.
package metrics

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"sync/atomic"
)

// TransitBucketsMs are the upper bounds of the packet transit histogram.
var TransitBucketsMs = []float64{1, 2, 5, 10, 20, 50, 100, 200, 500, 1000, 2000, 5000}

// Live is the shared state of a run. All methods are safe for concurrent use; the
// zero value is ready, and a nil *Live ignores every call, so code that records
// metrics need not check whether anyone is serving them.
type Live struct {
	ViewersTarget  atomic.Int64
	ViewersStarted atomic.Int64
	ViewersJoined  atomic.Int64
	ViewersFailed  atomic.Int64
	ViewersActive  atomic.Int64

	PacketsReceived atomic.Uint64
	BytesReceived   atomic.Uint64
	PacketsSent     atomic.Uint64
	FramesSent      atomic.Uint64

	transitBuckets [12]atomic.Uint64 // len(TransitBucketsMs)
	transitCount   atomic.Uint64
	transitSumUs   atomic.Uint64
}

// Packet records one received packet of size n bytes.
func (l *Live) Packet(n int) {
	if l == nil {
		return
	}
	l.PacketsReceived.Add(1)
	l.BytesReceived.Add(uint64(max(n, 0))) //nolint:gosec // non-negative
}

// PacketTransit records one packet transit sample, in milliseconds.
func (l *Live) PacketTransit(ms float64) {
	if l == nil || ms < 0 || math.IsNaN(ms) {
		return
	}
	for i, b := range TransitBucketsMs {
		if ms <= b {
			l.transitBuckets[i].Add(1)
			break
		}
	}
	l.transitCount.Add(1)
	l.transitSumUs.Add(uint64(ms * 1000))
}

// Handler serves /metrics.
func (l *Live) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		l.Write(w)
	})
	return mux
}

// Write renders every series.
func (l *Live) Write(w io.Writer) {
	gauge := func(name, help string, v int64) {
		fmt.Fprintf(w, "# HELP whipbench_%s %s\n# TYPE whipbench_%s gauge\nwhipbench_%s %d\n", name, help, name, name, v)
	}
	counter := func(name, help string, v uint64) {
		fmt.Fprintf(w, "# HELP whipbench_%s %s\n# TYPE whipbench_%s counter\nwhipbench_%s %d\n", name, help, name, name, v)
	}
	gauge("viewers_target", "Viewers the scenario asks for.", l.ViewersTarget.Load())
	counter("viewers_started_total", "Viewers that have sent their WHEP offer.", uint64(max(l.ViewersStarted.Load(), 0))) //nolint:gosec // non-negative
	counter("viewers_joined_total", "Viewers that received a complete keyframe.", uint64(max(l.ViewersJoined.Load(), 0))) //nolint:gosec // non-negative
	counter("viewers_failed_total", "Viewers that failed to join or errored.", uint64(max(l.ViewersFailed.Load(), 0)))    //nolint:gosec // non-negative
	gauge("viewers_active", "Viewers currently receiving.", l.ViewersActive.Load())
	counter("rtp_packets_received_total", "RTP packets received by all viewers.", l.PacketsReceived.Load())
	counter("rtp_bytes_received_total", "RTP payload bytes received by all viewers.", l.BytesReceived.Load())
	counter("publisher_rtp_packets_sent_total", "RTP packets the publisher has written.", l.PacketsSent.Load())
	counter("publisher_frames_sent_total", "Frames the publisher has written.", l.FramesSent.Load())

	const h = "whipbench_packet_transit_seconds"
	fmt.Fprintf(w, "# HELP %s Packet transit: arrival time minus the send-time stamp, per stamped packet (network plus server, not glass-to-glass).\n# TYPE %s histogram\n", h, h)
	var cum uint64
	for i, b := range TransitBucketsMs {
		cum += l.transitBuckets[i].Load()
		fmt.Fprintf(w, "%s_bucket{le=\"%s\"} %d\n", h, strconv.FormatFloat(b/1000, 'g', -1, 64), cum)
	}
	n := l.transitCount.Load()
	fmt.Fprintf(w, "%s_bucket{le=\"+Inf\"} %d\n%s_sum %s\n%s_count %d\n", h, n, h,
		strconv.FormatFloat(float64(l.transitSumUs.Load())/1e6, 'g', -1, 64), h, n)
}
