// Package report turns a run into a JSON document and a Markdown rendering of it,
// comparable across servers because every number carries its definition.
//
// Two rules are enforced here, not left to the caller:
//
//   - No endpoint is recorded beyond its host. Paths and query strings are where
//     servers put stream keys and tokens, so a report — which is meant to be shared —
//     never contains them.
//   - The no-verdict rule: when more than 10% of the viewers failed to join, no
//     aggregate is valid. The numbers of the viewers that did join describe a
//     different, smaller run than the one asked for, and quoting them as the result
//     would flatter the server.
package report

import (
	"encoding/json"
	"fmt"
	"math"
	"runtime"
	"slices"
	"sort"
	"time"

	"github.com/Allan-Nava/whipbench/internal/publisher"
	"github.com/Allan-Nava/whipbench/internal/scenario"
	"github.com/Allan-Nava/whipbench/internal/stats"
	"github.com/Allan-Nava/whipbench/internal/viewer"
	"github.com/Allan-Nava/whipbench/internal/whip"
)

// Schema identifies the report format; bump it when a field changes meaning.
const Schema = "whipbench.report/v0"

// NoVerdictThreshold is the share of viewers that may fail to join before the run
// has no verdict. Strictly more than this fails the run.
const NoVerdictThreshold = 0.10

// Report is one run.
type Report struct {
	Schema  string `json:"schema"`
	Version string `json:"whipbenchVersion"`
	Command string `json:"command"`
	// Interrupted: the run was stopped before its scheduled end.
	Interrupted bool              `json:"interrupted,omitempty"`
	StartedAt   time.Time         `json:"startedAt"`
	FinishedAt  time.Time         `json:"finishedAt"`
	Server      Server            `json:"server"`
	Client      Client            `json:"client"`
	Scenario    scenario.Scenario `json:"scenario"`
	Publisher   *publisher.Result `json:"publisher,omitempty"`
	Aggregate   Aggregate         `json:"aggregate"`
	Errors      map[string]int    `json:"errors"`
	Timeline    []Second          `json:"timeline,omitempty"`
	Viewers     []viewer.Result   `json:"viewers"`
	Method      []string          `json:"method"`
}

// Server is what a report records about the system under test: hosts only.
type Server struct {
	WHIPHost string `json:"whipHost,omitempty"`
	WHEPHost string `json:"whepHost"`
}

// Client is the machine that ran whipbench — no hostname, no user.
type Client struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
	CPUs int    `json:"cpus"`
	Go   string `json:"go"`
}

// Second is one second of the run: viewers receiving, and packets they received.
type Second struct {
	T       int    `json:"t"`
	Active  int64  `json:"active"`
	Packets uint64 `json:"packets"`
}

// Aggregate is the run's result across viewers.
type Aggregate struct {
	// Valid is false under the no-verdict rule, or when the publisher failed;
	// Verdict says why. Consumers must not quote the summaries of an invalid run.
	Valid   bool   `json:"valid"`
	Verdict string `json:"verdict"`

	Viewers       int     `json:"viewers"`
	Joined        int     `json:"joined"`
	Failed        int     `json:"failed"`
	FailedPercent float64 `json:"failedPercent"`
	Dropped       int     `json:"droppedAfterJoin"`

	// Summaries over the viewers that joined, one value per viewer.
	SignallingMs    stats.Summary `json:"signallingMs"`
	FirstRTPMs      stats.Summary `json:"joinFirstRtpMs"`
	FirstKeyframeMs stats.Summary `json:"joinFirstKeyframeMs"`
	LossPercent     stats.Summary `json:"lossPercent"`
	JitterMs        stats.Summary `json:"jitterMs"`
	KeyframeS       stats.Summary `json:"keyframeIntervalMeanS"`
	Stalls          int           `json:"stalls"`

	PacketsReceived uint64  `json:"packetsReceived"`
	PacketsLost     uint64  `json:"packetsLost"`
	LossTotal       float64 `json:"lossPercentTotal"`

	PacketTransit PacketTransit `json:"packetTransit"`
}

// PacketTransit is the pooled packet transit of every stamped packet of every
// viewer that had it.
type PacketTransit struct {
	Available bool           `json:"available"`
	Reason    string         `json:"reason,omitempty"`
	Viewers   int            `json:"viewers"`
	Ms        *stats.Summary `json:"ms,omitempty"`
}

// Input is everything a report is built from.
type Input struct {
	Command    string
	Version    string
	StartedAt  time.Time
	FinishedAt time.Time
	Scenario   scenario.Scenario
	Publisher  *publisher.Result
	Viewers    []viewer.Result
	Timeline   []Second
	// Interrupted: the run was stopped (Ctrl-C) before its scheduled end.
	Interrupted bool
}

// Build assembles a report and applies the no-verdict rule.
func Build(in Input) *Report {
	r := &Report{
		Schema: Schema, Version: in.Version, Command: in.Command,
		StartedAt: in.StartedAt.UTC(), FinishedAt: in.FinishedAt.UTC(),
		Server:   Server{WHEPHost: whip.Host(in.Scenario.WHEP)},
		Client:   Client{OS: runtime.GOOS, Arch: runtime.GOARCH, CPUs: runtime.NumCPU(), Go: runtime.Version()},
		Scenario: in.Scenario.Redacted(whip.Host),
		Viewers:  in.Viewers,
		Timeline: in.Timeline,
		Errors:   map[string]int{},
		Method:   Method,

		Interrupted: in.Interrupted,
	}
	if in.Scenario.WHIP != "" {
		r.Server.WHIPHost = whip.Host(in.Scenario.WHIP)
	}
	if in.Scenario.RampOffsetSeed != nil {
		r.Method = append(slices.Clone(Method), RampOffsetMethod)
	}
	if in.Publisher != nil {
		p := *in.Publisher
		r.Publisher = &p
		if p.ErrorKind != "" {
			r.Errors["publisher_"+p.ErrorKind]++
		}
	}
	sort.Slice(r.Viewers, func(i, j int) bool { return r.Viewers[i].ID < r.Viewers[j].ID })
	for _, v := range r.Viewers {
		if v.ErrorKind != "" {
			r.Errors[v.ErrorKind]++
		}
	}
	r.Aggregate = aggregate(in.Scenario.Viewers, r.Viewers, r.Publisher)
	if in.Interrupted && r.Aggregate.Valid {
		r.Aggregate.Valid = false
		r.Aggregate.Verdict = "no verdict: the run was interrupted before its scheduled end"
	}
	return r
}

func aggregate(target int, vs []viewer.Result, pub *publisher.Result) Aggregate {
	a := Aggregate{Viewers: max(target, len(vs))}
	var sig, rtp1, key, loss, jit, kfi []float64
	hist := stats.NewHistogram()
	var noTransit []string
	for _, v := range vs {
		if !v.Joined {
			continue
		}
		a.Joined++
		if v.DroppedAfterJoin {
			a.Dropped++
		}
		if v.SignallingMs != nil {
			sig = append(sig, *v.SignallingMs)
		}
		if v.FirstRTPMs != nil {
			rtp1 = append(rtp1, *v.FirstRTPMs)
		}
		if v.FirstKeyframeMs != nil {
			key = append(key, *v.FirstKeyframeMs)
		}
		loss = append(loss, v.RTP.LossPercent)
		jit = append(jit, v.RTP.JitterMs)
		if v.RTP.Keyframes > 1 {
			kfi = append(kfi, v.RTP.KeyframeIntervalMeanS)
		}
		a.Stalls += v.RTP.Stalls
		a.PacketsReceived += v.RTP.Received
		a.PacketsLost += v.RTP.Lost
		if h := v.PacketTransit.Histogram(); h != nil {
			hist.Merge(h)
			a.PacketTransit.Viewers++
		} else {
			noTransit = append(noTransit, v.PacketTransit.Reason)
		}
	}
	a.Failed = a.Viewers - a.Joined
	if a.Viewers > 0 {
		a.FailedPercent = float64(a.Failed) / float64(a.Viewers) * 100
	}
	if exp := a.PacketsReceived + a.PacketsLost; exp > 0 {
		a.LossTotal = float64(a.PacketsLost) / float64(exp) * 100
	}
	a.SignallingMs, a.FirstRTPMs, a.FirstKeyframeMs = stats.Summarise(sig), stats.Summarise(rtp1), stats.Summarise(key)
	a.LossPercent, a.JitterMs, a.KeyframeS = stats.Summarise(loss), stats.Summarise(jit), stats.Summarise(kfi)

	switch {
	case hist.Count() > 0:
		a.PacketTransit.Available = true
		s := hist.Summary()
		a.PacketTransit.Ms = &s
		if n := len(noTransit); n > 0 {
			a.PacketTransit.Reason = fmt.Sprintf("pooled over %d of %d joined viewers; the other %d had none (%s)", a.PacketTransit.Viewers, a.Joined, n, mostCommon(noTransit))
		}
	case a.Joined == 0:
		a.PacketTransit.Reason = "no viewer joined"
	default:
		a.PacketTransit.Reason = mostCommon(noTransit)
	}

	a.Valid, a.Verdict = verdict(a, pub)
	return a
}

// verdict applies the no-verdict rule.
func verdict(a Aggregate, pub *publisher.Result) (bool, string) {
	if pub != nil && pub.ErrorKind != "" && pub.FramesSent == 0 {
		return false, "no verdict: the publisher never streamed (" + pub.ErrorKind + ")"
	}
	if a.Viewers == 0 {
		return false, "no verdict: no viewers"
	}
	if a.Joined == 0 {
		return false, fmt.Sprintf("no verdict: none of the %d viewers joined", a.Viewers)
	}
	if float64(a.Failed) > NoVerdictThreshold*float64(a.Viewers) {
		return false, fmt.Sprintf("no verdict: %d of %d viewers (%s%%) failed to join, above the %d%% the rule allows",
			a.Failed, a.Viewers, trim(a.FailedPercent), int(NoVerdictThreshold*100))
	}
	return true, fmt.Sprintf("valid: %d of %d viewers joined", a.Joined, a.Viewers)
}

func mostCommon(reasons []string) string {
	if len(reasons) == 0 {
		return ""
	}
	n := map[string]int{}
	best := ""
	for _, r := range reasons {
		n[r]++
		if n[r] > n[best] || (n[r] == n[best] && r < best) {
			best = r
		}
	}
	return best
}

func trim(v float64) string {
	if v == math.Trunc(v) {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%.1f", v)
}

// JSON renders the report, indented.
func (r *Report) JSON() ([]byte, error) {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// Method is the definitions a report carries with it, so a number read out of
// context still says what it measured.
var Method = []string{
	"Join time is measured from the WHEP POST (after host-candidate ICE gathering). firstRtp: the first RTP packet arrives. firstKeyframe: the last packet of the first keyframe received complete arrives — no decoder runs, so this is when a frame could first be decoded, not when one was. A viewer has joined when it reaches firstKeyframe within the join timeout.",
	"Loss: expected = highest − first extended sequence number + 1 (RFC 3550 A.1), lost = expected − received, duplicates not counted. Measured on the stream the viewer receives, after NACK recovery; RTX is not negotiated, so a retransmission counts as received.",
	"Jitter: RFC 3550 §6.4.1 interarrival jitter, J += (|D| − J)/16 per packet, in milliseconds.",
	"Keyframe interval: spacing of keyframe starts in RTP time, i.e. the GOP the server delivers. The publisher's clip has a fixed 1 s GOP and cannot answer PLI, so a joining viewer waits for the next keyframe in the loop.",
	"Packet transit: the publisher stamps each packet's wall-clock send time in the abs-capture-time RTP header extension; a viewer's sample is its arrival time minus the stamp. It is network plus server forwarding plus both clients' stacks, per packet rather than per frame, on one host or synchronised clocks — not glass-to-glass. When the server does not negotiate or forward the extension, packet transit is reported unavailable, never estimated.",
	"Percentiles are nearest-rank. Join, loss and jitter summaries take one value per joined viewer; packet transit pools every valid sample of every viewer that has it, in a histogram with 1% buckets.",
	"No-verdict rule: when more than 10% of the viewers failed to join, the aggregate is not valid and must not be quoted.",
}

// RampOffsetMethod joins Method in a report whose scenario sets rampOffsetSeed.
const RampOffsetMethod = "Viewer starts: viewer i of n starts i·ramp/n after the warmup, plus a seeded offset drawn uniformly from [0, rampOffsetMaxSeconds) — the i-th output of SplitMix64 seeded with rampOffsetSeed — so that join time samples the GOP evenly whatever the ramp step. Each viewer's offset is recorded as rampOffsetMs, and the same seed gives the same offsets."
