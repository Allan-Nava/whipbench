// Package report turns a run into a JSON document and a Markdown rendering of it,
// comparable across servers because every number carries its definition.
//
// Three rules are enforced here, not left to the caller:
//
//   - No endpoint is recorded beyond its host. Paths and query strings are where
//     servers put stream keys and tokens, so a report — which is meant to be shared —
//     never contains them.
//   - The no-verdict rule: when more than 10% of the viewers failed to join, no
//     aggregate is valid. The numbers of the viewers that did join describe a
//     different, smaller run than the one asked for, and quoting them as the result
//     would flatter the server.
//   - Comparability (WB-40): every report says its topology and how its clocks were
//     put on one time base, every one-way delay block says whether it may be ranked
//     against another report's and why not, and Rankable is the one ranking rule.
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
	Interrupted bool      `json:"interrupted,omitempty"`
	StartedAt   time.Time `json:"startedAt"`
	FinishedAt  time.Time `json:"finishedAt"`
	Server      Server    `json:"server"`
	Client      Client    `json:"client"`
	// Topology and Clock say what makes this report's delays comparable with
	// another's (WB-40, D6); neither names a host.
	Topology  string            `json:"topology"`
	Clock     Clock             `json:"clock"`
	Scenario  scenario.Scenario `json:"scenario"`
	Publisher *publisher.Result `json:"publisher,omitempty"`
	Aggregate Aggregate         `json:"aggregate"`
	Errors    map[string]int    `json:"errors"`
	Timeline  []Second          `json:"timeline,omitempty"`
	Viewers   []viewer.Result   `json:"viewers"`
	Method    []string          `json:"method"`
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

	// OneWayDelay is the headline: one pooled block per source, the fingerprint first.
	OneWayDelay   []OneWayDelay `json:"oneWayDelay"`
	PacketTransit PacketTransit `json:"packetTransit"`
}

// OneWayDelay is the pooled one-way delay of one source over the viewers that joined.
// Its counts are the sums of the joined viewers' blocks; Samples and Ms pool every
// valid sample of the viewers whose block is available, as packet transit does.
type OneWayDelay struct {
	Source    string `json:"source"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
	// Viewers is the number of joined viewers whose samples are pooled, and
	// ViewersByFrameEnd splits them by how each found the end of a frame.
	Viewers           int            `json:"viewers"`
	ViewersByFrameEnd map[string]int `json:"viewersByFrameEnd"` // never nil
	CompleteFrames    uint64         `json:"completeFrames"`
	IncompleteFrames  uint64         `json:"incompleteFrames"`
	Samples           uint64         `json:"samples"`
	Invalid           uint64         `json:"invalid"`
	UnmatchedFrames   uint64         `json:"unmatchedFrames"`
	// DuplicateFrames is the number of clip frames whose bytes repeat in the clip and
	// so are never sampled; LoopFrames the clip's length in frames; LoopMinMs the
	// shortest time the publisher took to send one loop, absent until it sent one.
	DuplicateFrames int            `json:"duplicateFrames"`
	LoopFrames      int            `json:"loopFrames"`
	LoopMinMs       *float64       `json:"loopMinMs,omitempty"`
	Ms              *stats.Summary `json:"ms,omitempty"`
	// UncertaintyMs is the clock's uncertainty, present only on an available block;
	// Comparable and NotComparableReason are the block's verdict under the report's
	// clock (D8), and only a comparable block may be ranked (Rankable).
	UncertaintyMs       *float64 `json:"uncertaintyMs,omitempty"`
	Comparable          bool     `json:"comparable"`
	NotComparableReason string   `json:"notComparableReason,omitempty"`
}

// Fingerprint is what run's clip table and send log add to the pooled block; nil when this
// process published nothing (view, or a run without a WHIP endpoint).
type Fingerprint struct {
	DuplicateFrames int
	LoopFrames      int
	LoopMin         time.Duration // 0 until the log has seen a whole loop
}

// NoSendLogReason: without a send log in this process there is no t0 to subtract (Q2).
const NoSendLogReason = "no send log: the send log lives in the `run` process that publishes the clip, and this process published nothing"

// NoTrackReason: the viewer never received a video track, so it reassembled nothing.
const NoTrackReason = "no video track reached this viewer"

// FingerprintDelay returns the pooled fingerprint block; Build always produces one. An
// Aggregate built by hand without one gets {Source: "fingerprint", Reason: NoSendLogReason}.
func (a Aggregate) FingerprintDelay() OneWayDelay {
	for _, b := range a.OneWayDelay {
		if b.Source == viewer.SourceFingerprint {
			return b
		}
	}
	return OneWayDelay{Source: viewer.SourceFingerprint, Reason: NoSendLogReason, ViewersByFrameEnd: map[string]int{}}
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
	// Fingerprint carries the clip table's and the send log's figures; runner sets it
	// when this process publishes (S6). Nil means there is no send log to match against.
	Fingerprint *Fingerprint
	// Topology is TopologySingleProcess or TopologySplit; empty is read as split, which
	// claims no shared clock.
	Topology string
	// Clock is how the two ends were put on one time base: runner sets MonotonicClock
	// when it publishes, and ExchangeClock when a split run was given a clock peer (WB-3).
	// Nil means the publisher's clock was not measured — method none, nothing comparable.
	Clock *Clock
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
		Topology:    in.Topology,
		Clock:       Clock{Method: ClockNone},
	}
	if r.Topology == "" {
		r.Topology = TopologySplit
	}
	if in.Clock != nil {
		r.Clock = *in.Clock
	}
	if in.Scenario.WHIP != "" {
		r.Server.WHIPHost = whip.Host(in.Scenario.WHIP)
	}
	if in.Scenario.RampOffsetSeed != nil {
		r.Method = append(slices.Clone(Method), RampOffsetMethod)
	}
	if r.Clock.Method == ClockExchange || r.Clock.Reason != "" {
		r.Method = append(slices.Clone(r.Method), ClockExchangeMethod)
	}
	if in.Publisher != nil {
		p := *in.Publisher
		r.Publisher = &p
		if p.ErrorKind != "" {
			r.Errors["publisher_"+p.ErrorKind]++
		}
	}
	sort.Slice(r.Viewers, func(i, j int) bool { return r.Viewers[i].ID < r.Viewers[j].ID })
	for i := range r.Viewers {
		r.Viewers[i].OneWayDelay = normaliseOneWayDelay(r.Viewers[i].OneWayDelay, in.Fingerprint)
		for j, o := range r.Viewers[i].OneWayDelay {
			r.Viewers[i].OneWayDelay[j] = judgeViewer(o, r.Clock)
		}
	}
	for _, v := range r.Viewers {
		if v.ErrorKind != "" {
			r.Errors[v.ErrorKind]++
		}
	}
	r.Aggregate = aggregate(in.Scenario.Viewers, r.Viewers, r.Publisher, in.Fingerprint)
	for i, o := range r.Aggregate.OneWayDelay {
		r.Aggregate.OneWayDelay[i] = judge(o, r.Clock)
	}
	if in.Interrupted && r.Aggregate.Valid {
		r.Aggregate.Valid = false
		r.Aggregate.Verdict = "no verdict: the run was interrupted before its scheduled end"
	}
	return r
}

// normaliseOneWayDelay gives every viewer exactly its blocks, each finished; a viewer with
// none gets one unavailable fingerprint block saying why.
func normaliseOneWayDelay(bs []viewer.OneWayDelay, fp *Fingerprint) []viewer.OneWayDelay {
	bs = slices.Clone(bs)
	if len(bs) == 0 {
		reason := NoTrackReason
		if fp == nil {
			reason = NoSendLogReason
		}
		bs = []viewer.OneWayDelay{{Source: viewer.SourceFingerprint, Reason: reason}}
	}
	for i := range bs {
		bs[i] = finishOneWayDelay(bs[i])
	}
	return bs
}

// finishOneWayDelay decides Available, Reason, Samples and Ms from the counts; idempotent.
// A block without Hist keeps the Samples it was given but is treated as having none, so a
// summary is never invented for it.
func finishOneWayDelay(o viewer.OneWayDelay) viewer.OneWayDelay {
	if o.Source == "" {
		o.Source = viewer.SourceFingerprint
	}
	if o.Hist != nil {
		o.Samples = o.Hist.Count()
	}
	if o.Reason != "" {
		o.Available, o.Ms = false, nil
		return o
	}
	if o.Samples == 0 || o.Hist == nil {
		o.Available, o.Ms = false, nil
		o.Reason = fmt.Sprintf("no valid sample: %d complete frames (%d unmatched, %d invalid), %d incomplete",
			o.CompleteFrames, o.UnmatchedFrames, o.Invalid, o.IncompleteFrames)
		return o
	}
	o.Available = true
	sm := o.Hist.Summary()
	o.Ms = &sm
	return o
}

// fingerprintBlock is a viewer's first fingerprint block; a viewer without one is
// finished as an empty block, so it is counted among those that had no sample.
func fingerprintBlock(bs []viewer.OneWayDelay) viewer.OneWayDelay {
	for _, b := range bs {
		if b.Source == viewer.SourceFingerprint {
			return b
		}
	}
	return finishOneWayDelay(viewer.OneWayDelay{})
}

func aggregate(target int, vs []viewer.Result, pub *publisher.Result, fp *Fingerprint) Aggregate {
	a := Aggregate{Viewers: max(target, len(vs))}
	var sig, rtp1, key, loss, jit, kfi []float64
	hist := stats.NewHistogram()
	var noTransit []string
	delay := OneWayDelay{Source: viewer.SourceFingerprint, ViewersByFrameEnd: map[string]int{}}
	delayHist := stats.NewHistogram()
	var noDelay []string
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
		b := fingerprintBlock(v.OneWayDelay)
		delay.CompleteFrames += b.CompleteFrames
		delay.IncompleteFrames += b.IncompleteFrames
		delay.Invalid += b.Invalid
		delay.UnmatchedFrames += b.UnmatchedFrames
		if b.Available && b.Hist != nil {
			delayHist.Merge(b.Hist)
			delay.Viewers++
			delay.ViewersByFrameEnd[b.FrameEnd]++
		} else {
			noDelay = append(noDelay, b.Reason)
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

	delay.Samples = delayHist.Count()
	if fp != nil {
		delay.DuplicateFrames, delay.LoopFrames = fp.DuplicateFrames, fp.LoopFrames
		if fp.LoopMin > 0 {
			ms := float64(fp.LoopMin) / float64(time.Millisecond)
			delay.LoopMinMs = &ms
		}
	}
	switch {
	case fp == nil:
		delay.Reason = NoSendLogReason
	case delay.Samples > 0:
		delay.Available = true
		s := delayHist.Summary()
		delay.Ms = &s
		if n := len(noDelay); n > 0 {
			delay.Reason = fmt.Sprintf("pooled over %d of %d joined viewers; the other %d had none (%s)", delay.Viewers, a.Joined, n, mostCommon(noDelay))
		}
	case a.Joined == 0:
		delay.Reason = "no viewer joined"
	default:
		delay.Reason = mostCommon(noDelay)
	}
	a.OneWayDelay = []OneWayDelay{delay}

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
	OneWayDelayMethod,
	ComparabilityMethod,
	"Packet transit: the publisher stamps each packet's wall-clock send time in the abs-capture-time RTP header extension; a viewer's sample is its arrival time minus the stamp. It is network plus server forwarding plus both clients' stacks, per packet rather than per frame, on one host or synchronised clocks — not glass-to-glass. When the server does not negotiate or forward the extension, packet transit is reported unavailable, never estimated.",
	"Percentiles are nearest-rank. Join, loss and jitter summaries take one value per joined viewer; packet transit pools every valid sample of every viewer that has it, in a histogram with 1% buckets.",
	"No-verdict rule: when more than 10% of the viewers failed to join, the aggregate is not valid and must not be quoted.",
}

// RampOffsetMethod joins Method in a report whose scenario sets rampOffsetSeed.
const RampOffsetMethod = "Viewer starts: viewer i of n starts i·ramp/n after the warmup, plus a seeded offset drawn uniformly from [0, rampOffsetMaxSeconds) — the i-th output of SplitMix64 seeded with rampOffsetSeed — so that join time samples the GOP evenly whatever the ramp step. Each viewer's offset is recorded as rampOffsetMs, and the same seed gives the same offsets."

// OneWayDelayMethod is the definition every report carries (P1, P2), placed in Method
// before packet transit because it is the headline.
const OneWayDelayMethod = "One-way delay (source fingerprint): per frame, first-packet send to last-packet arrival, on the monotonic clock of the one process that runs both ends. The publisher logs t0 just before it hands a frame's first packet to the stack, by absolute frame index. Each viewer reassembles frames by RTP timestamp; t1 is the arrival of the frame's last packet — the marker packet, or, on a stream without markers, the last before the next timestamp (frameEnd). A complete frame is hashed — the first 64 bits of SHA-256 over the whole VP8 frame, or over the H.264 VCL NAL units — and matched to the latest send of that clip frame at or before t1; the sample is t1 − t0. A frame still incomplete 1 s after its first packet counts in incompleteFrames and is never hashed; a viewer's first frame and the frames still pending when it stops are not counted. A frame the depacketiser rejects or that is not in the clip counts in unmatchedFrames; a frame whose bytes repeat in the clip (duplicateFrames) is never sampled; a match the RTP timestamps prove whole loops too new, or one with no logged send at or before t1, is invalid. loopMinMs is the shortest time the publisher took to send loopFrames frames: a delay longer than that is caught only by that RTP timestamp check, and a viewer's first match is taken as it is. Retransmitted packets count like any other. Network plus server forwarding plus both clients' stacks — not glass-to-glass; a `view` run has no send log and reports it unavailable. The CPU cost of reassembling and hashing every frame on every viewer is not measured."

// ClockExchangeMethod joins Method in a report whose run was given a clock peer (WB-3).
const ClockExchangeMethod = "Clock exchange: the viewer's host sends the publisher 16 UDP probes 10 ms apart, each carrying its wall clock t1; the publisher answers each with t1, its wall clock at receipt t2 and at send t3, in a packet no larger than the probe. Per probe, rtt = (t4 − t1) − (t3 − t2) on the viewer's monotonic clock, and offset = ((t2 − t1) + (t3 − t4)) / 2, with t4 the wall reading t1 plus the monotonic elapsed time, so a step during one probe cannot corrupt it; the offset is the publisher's clock minus the viewer's. The probe with the smallest rtt gives the point, known to within rtt/2: the exchange cannot tell an asymmetric path from an offset. Points are taken before the first viewer starts, every 30 s while viewers run and after they stop; a point with no answer within 1 s of its last probe is skipped. offsetMs is the first point's offset, uncertaintyMs the largest rtt/2 over the points, and between points the offset is piecewise-linear, held at the first and last outside them. It is a wall-clock method: a detected step makes a figure not comparable. With no point at all the method is none, with the reason. No figure applies the offset yet."

// ComparabilityMethod is what makes two reports' delays comparable (WB-40, D6 and D8).
const ComparabilityMethod = "Comparability: topology is single-process when this process published and viewed, split otherwise. The clock block says how both ends share a time base — monotonic in a single-process run, offset 0 ± 0 ms by construction; exchange in a split run given a clock peer, with the offset and uncertainty it measured; none in a split run without one, or whose peer never answered, with no offset or uncertainty recorded — and stepDetected is set when, between two once-a-second observations, wall-clock and monotonic elapsed time differ by more than 0.1 ms plus a 500 ppm slew. A source block is comparable only when it is available, the clock was measured, its uncertainty is 1 ms or less and, for a wall-clock method, no step was detected; an unavailable block carries no uncertainty. Two reports rank on a source only when both blocks are comparable, both aggregates valid, the clip (codec, loop frames) and every scenario key the same but the endpoint hosts, the name, the bearer variable and the metrics address. Sources are never averaged and there is no merged best source."
