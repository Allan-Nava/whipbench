package viewer

import "github.com/Allan-Nava/whipbench/internal/stats"

// SourceFingerprint is the source of WB-38's send instant: the publisher's log, found by
// the frame's fingerprint. WB-39 adds "stamp" beside it under the same key (D7).
const SourceFingerprint = "fingerprint"

// OneWayDelay is one viewer's one-way delay from one source (D7). The viewer fills the
// counts, FrameEnd and Hist, and a Reason only when it could not measure at all (S6); the
// report decides Available, Samples, Ms and every other Reason, so a block built anywhere
// is finished one way.
//
// The counts account for every complete frame: CompleteFrames = Samples + Invalid +
// UnmatchedFrames + ExcludedFrames + the frames whose fingerprint is a clip duplicate,
// which are never sampled because their bytes cannot say which clip frame was sent.
// LateCompletedFrames is a subset of CompleteFrames, inside the window or not (WB-41).
// Every sample is also recorded in exactly one of KeyHist and DeltaHist, by the kind of
// the clip frame it matched, so Keyframes.Samples + DeltaFrames.Samples = Samples (WB-44).
type OneWayDelay struct {
	// Source names where the send instant came from; SourceFingerprint for WB-38.
	Source string `json:"source"`
	// Available: the block carries at least one valid sample, summarised in Ms.
	Available bool `json:"available"`
	// Reason says why there is no sample; empty when the block is available.
	Reason string `json:"reason,omitempty"`
	// FrameEnd is how the viewer decided a frame was over: "marker" when the stream
	// carries markers, "timestamp" when only the next RTP timestamp ends a frame (D5).
	FrameEnd string `json:"frameEnd,omitempty"`
	// CompleteFrames were reassembled whole and hashed; IncompleteFrames were still
	// missing packets when they timed out, and were never hashed.
	CompleteFrames   uint64 `json:"completeFrames"`
	IncompleteFrames uint64 `json:"incompleteFrames"`
	// Samples is the number of valid delays, Hist's count once the report finishes the
	// block; Invalid matches gave no sample, and UnmatchedFrames had no clip frame.
	Samples         uint64 `json:"samples"`
	Invalid         uint64 `json:"invalid"`
	UnmatchedFrames uint64 `json:"unmatchedFrames"`
	// ExcludedFrames were complete but their first packet arrived inside the scenario's
	// excludeFirstSeconds of the viewer's first RTP packet, so they were never sampled
	// (WB-41). LateCompletedFrames are complete frames one of whose packets arrived after
	// their last: a gap filled after the end of the frame had arrived, by a NACK
	// retransmission or by reordering. Their samples still end at the last packet.
	ExcludedFrames      uint64 `json:"excludedFrames"`
	LateCompletedFrames uint64 `json:"lateCompletedFrames"`
	// Ms summarises Hist; nil whenever the block is unavailable, so a zero is never
	// mistaken for a measurement.
	Ms *stats.Summary `json:"ms,omitempty"`
	// Keyframes and DeltaFrames split the samples by the kind of clip frame each matched
	// (WB-44): a keyframe spans many packets and a delta frame few, so their
	// first-to-last spread differs. They describe Ms and carry no comparability of their own.
	Keyframes   FrameKindDelay `json:"keyframes"`
	DeltaFrames FrameKindDelay `json:"deltaFrames"`
	// UncertaintyMs, Comparable and NotComparableReason are set by the report from its
	// clock (WB-40, D8), as in the pooled block; the uncertainty only when available.
	UncertaintyMs       *float64 `json:"uncertaintyMs,omitempty"`
	Comparable          bool     `json:"comparable"`
	NotComparableReason string   `json:"notComparableReason,omitempty"`
	// Hist holds the samples in ms; exported so the report and its tests can pool it.
	// KeyHist and DeltaHist hold the same samples split by frame kind.
	Hist      *stats.Histogram `json:"-"`
	KeyHist   *stats.Histogram `json:"-"`
	DeltaHist *stats.Histogram `json:"-"`
}

// FrameKindDelay is the part of a one-way delay block drawn from one kind of clip frame,
// keyframes or delta frames (WB-44). The report fills it from the block's KeyHist or
// DeltaHist; Ms is nil whenever the subset has no sample or the block is unavailable,
// so a zero is never mistaken for a measurement.
type FrameKindDelay struct {
	Samples uint64         `json:"samples"`
	Ms      *stats.Summary `json:"ms,omitempty"`
}
