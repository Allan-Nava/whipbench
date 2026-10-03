package fingerprint

import "time"

// Verdict is what a viewer does with one matched frame.
type Verdict int

const (
	Sampled   Verdict = iota + 1 // t1 − t0 is a sample
	Aliased                      // invalid: the RTP timestamps prove the match whole loops too new (D3)
	NotLogged                    // invalid: no send of that clip index at or before t1 in the log
)

// Matcher is one viewer's matching state: the log it reads and the anchor (kA, tsA) of
// its last sampled match. A clip index alone cannot tell loops apart, so the anchor lets
// the RTP timestamp step between two matches say how many frames apart they really are;
// when the log's latest send is whole loops further on than that, the match is aliased
// and is not a sample (D3). Not safe for concurrent use; one per viewer.
type Matcher struct {
	log    *SendLog
	ticks  int64 // RTP timestamp ticks per frame
	frames int64 // n, frames per loop
	have   bool  // an anchor has been set
	kA     uint64
	tsA    uint32
}

// NewMatcher returns a matcher with no anchor that reads log, for a clip whose frames are
// ticks RTP timestamp units apart.
func NewMatcher(log *SendLog, ticks uint32) *Matcher {
	return &Matcher{log: log, ticks: int64(ticks), frames: int64(log.Frames())}
}

// Match decides what a frame of clip index i, carrying RTP timestamp ts and received at
// t1, contributes. A Sampled verdict comes with t1 − t0, which is monotonic whenever both
// times carry a monotonic reading, as time.Now() values do; the other verdicts carry 0.
// Only a sampled match moves the anchor, so one aliased frame cannot poison the next.
func (m *Matcher) Match(i int, ts uint32, t1 time.Time) (Verdict, time.Duration) {
	kL, t0, ok := m.log.Latest(i, t1)
	if !ok {
		return NotLogged, 0
	}
	if m.have {
		dts := int64(int32(ts - m.tsA)) //nolint:gosec // the int32 cast is the RTP wrap-around
		if dts%m.ticks == 0 {
			delta := dts / m.ticks
			diff := int64(kL) - (int64(m.kA) + delta) //nolint:gosec // frame indices stay far below 2^63
			if diff >= m.frames && diff%m.frames == 0 {
				return Aliased, 0
			}
		}
	}
	m.have, m.kA, m.tsA = true, kL, ts
	return Sampled, t1.Sub(t0)
}
