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
// UnmatchedFrames + the frames whose fingerprint is a clip duplicate, which are never
// sampled because their bytes cannot say which clip frame was sent.
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
	// Ms summarises Hist; nil whenever the block is unavailable, so a zero is never
	// mistaken for a measurement.
	Ms *stats.Summary `json:"ms,omitempty"`
	// Hist holds the samples in ms; exported so the report and its tests can pool it.
	Hist *stats.Histogram `json:"-"`
}
