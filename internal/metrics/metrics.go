// Package metrics keeps the live counters of a run and serves them in the
// Prometheus text exposition format (version 0.0.4) at /metrics.
//
// It is written by hand rather than with the Prometheus client library: a dozen
// counters and two histograms do not justify the dependency tree, and the format is
// a few lines of text. Every series is prefixed whipbench_.
//
// One-way delay (WB-42) is the one labelled series: whipbench_one_way_delay_seconds
// carries source, the way the report's blocks do, so a second source (WB-39's stamp)
// adds a label value instead of changing what an existing series means. There is no
// per-viewer label.
//
// Three series describe the client rather than the server (WB-17): its CPU time,
// goroutines and heap, read from internal/procstat when /metrics is scraped, never on a
// ticker. They cover the whole process — publisher, viewers and reassembly — so a client
// that has run out of CPU shows here before its figures are taken for the server's.
package metrics

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/Allan-Nava/whipbench/internal/procstat"
)

// TransitBucketsMs are the upper bounds of the packet transit histogram.
var TransitBucketsMs = []float64{1, 2, 5, 10, 20, 50, 100, 200, 500, 1000, 2000, 5000}

// OneWayDelayBucketsMs are the upper bounds of the one-way delay histogram: the same
// 1-2-5 ladder as TransitBucketsMs, from 0.1 ms to 1 s. A frame's delay on one machine
// is under a millisecond at the median, and a frame still incomplete 1 s after its
// first packet is never sampled, so above 1 s there is only +Inf.
var OneWayDelayBucketsMs = [...]float64{0.1, 0.2, 0.5, 1, 2, 5, 10, 20, 50, 100, 200, 500, 1000}

// OneWayDelaySources are the values the source label may take, in exposition order:
// "fingerprint" is WB-38's send instant (viewer.SourceFingerprint, which this package
// cannot import). A sample from any other source is dropped, so the label's cardinality
// is fixed here; WB-39 adds "stamp".
var OneWayDelaySources = [...]string{"fingerprint"}

// delayHist is one source's one-way delay histogram.
type delayHist struct {
	buckets [len(OneWayDelayBucketsMs)]atomic.Uint64
	count   atomic.Uint64
	sumNs   atomic.Uint64
}

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

	oneWayDelay [len(OneWayDelaySources)]delayHist
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

// OneWayDelay records one valid one-way delay sample from source. The viewer calls it
// exactly where its report block takes the sample, never for an invalid or unmatched
// frame; a negative delay or an unknown source is dropped.
func (l *Live) OneWayDelay(source string, d time.Duration) {
	if l == nil || d < 0 {
		return
	}
	for s, name := range OneWayDelaySources {
		if name != source {
			continue
		}
		h := &l.oneWayDelay[s]
		ms := float64(d) / float64(time.Millisecond)
		for i, b := range OneWayDelayBucketsMs {
			if ms <= b {
				h.buckets[i].Add(1)
				break
			}
		}
		h.count.Add(1)
		h.sumNs.Add(uint64(d)) //nolint:gosec // non-negative
		return
	}
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

	// Declared whether or not this process can observe it: a `view` has no send log, so
	// its count stays 0, as packet transit's does when the extension is not negotiated.
	const d = "whipbench_one_way_delay_seconds"
	fmt.Fprintf(w, "# HELP %s One-way delay per frame: first-packet send to last-packet arrival, one sample per valid match, by source of the send instant (fingerprint: the publisher's send log, found by the frame's fingerprint). Network plus server plus both stacks, not glass-to-glass; observed only by a run that publishes.\n# TYPE %s histogram\n", d, d)
	for s, src := range OneWayDelaySources {
		dh := &l.oneWayDelay[s]
		var cum uint64
		for i, b := range OneWayDelayBucketsMs {
			cum += dh.buckets[i].Load()
			fmt.Fprintf(w, "%s_bucket{source=%q,le=\"%s\"} %d\n", d, src, strconv.FormatFloat(b/1000, 'g', -1, 64), cum)
		}
		n := dh.count.Load()
		fmt.Fprintf(w, "%s_bucket{source=%q,le=\"+Inf\"} %d\n%s_sum{source=%q} %s\n%s_count{source=%q} %d\n", d, src, n, d, src,
			strconv.FormatFloat(float64(dh.sumNs.Load())/1e9, 'g', -1, 64), d, src, n)
	}

	writeClient(w)
}

// writeClient renders the client's own series, read now. Where the platform has no CPU
// reading the counter is declared with no sample, so a dashboard shows no data rather
// than a 0 that would read as an idle client.
func writeClient(w io.Writer) {
	const c = "whipbench_client_cpu_seconds_total"
	fmt.Fprintf(w, "# HELP %s CPU time this whipbench process has used, user plus system, since it started: publisher, viewers and reassembly together, not the server. No sample where the platform gives no reading.\n# TYPE %s counter\n", c, c)
	if d, ok := procstat.CPU(); ok {
		fmt.Fprintf(w, "%s %s\n", c, strconv.FormatFloat(d.Seconds(), 'g', -1, 64))
	}
	fmt.Fprintf(w, "# HELP whipbench_client_goroutines Goroutines in this whipbench process now.\n# TYPE whipbench_client_goroutines gauge\nwhipbench_client_goroutines %d\n", procstat.Goroutines())
	fmt.Fprintf(w, "# HELP whipbench_client_heap_bytes Heap in use by this whipbench process now: bytes in spans holding objects, live or not yet swept (runtime/metrics, no stop-the-world).\n# TYPE whipbench_client_heap_bytes gauge\nwhipbench_client_heap_bytes %d\n", procstat.HeapBytes())
}
