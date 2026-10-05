// Package rtpstats computes what a viewer reports about the RTP stream it received:
// loss from sequence numbers, interarrival jitter, keyframes and the gaps between
// them, and stalls. It is pure arithmetic over (sequence number, timestamp, arrival
// time) so it can be tested on synthetic sequences without a network.
//
// The definitions follow RFC 3550 so the numbers mean what an operator expects:
//
//   - expected = highest extended sequence number − first extended sequence number + 1
//     (Appendix A.1, A.3), lost = expected − received, with duplicates not counted as
//     received. Lost can therefore not go negative, unlike the RFC's cumulative
//     count, which counts duplicates.
//   - a packet that arrives behind later ones — reordered, or a NACK retransmission on
//     its original sequence number, RTX not being negotiated — is received like any
//     other, so a gap filled late stops counting as lost and lost is what never
//     arrived (WB-41). tooLate is a packet more than 65,535 sequence numbers behind the
//     highest, too old for the window to tell from a duplicate; a retransmission is
//     never that far behind.
//   - jitter is the interarrival jitter of §6.4.1 and Appendix A.8:
//     D(i,j) = (Rj − Ri) − (Sj − Si) in RTP clock units, J += (|D| − J)/16, updated on
//     every packet in arrival order.
package rtpstats

import (
	"math"
	"time"
)

// Stream accumulates the statistics of one received RTP stream. Not safe for
// concurrent use: one viewer's read loop owns it.
type Stream struct {
	clockRate float64
	stall     time.Duration

	started       bool
	cycles        int64 // multiple of 65536
	maxSeq        uint16
	baseExt       int64
	maxExt        int64
	received      uint64
	duplicates    uint64
	tooLate       uint64
	seen          [65536 / 64]uint64 // bitmap of sequence numbers in the window (maxExt-65535, maxExt]
	jitter        float64            // RTP clock units
	lastArrivalTS float64            // arrival time in RTP clock units
	lastTS        uint32
	firstArrival  time.Duration
	lastArrival   time.Duration
	stalls        int
	stalled       time.Duration
	longestGap    time.Duration

	keys keyframes
}

// New returns a Stream for a codec clocked at clockRate Hz. A gap of at least
// stall between two consecutive packets counts as a stall.
func New(clockRate int, stall time.Duration) *Stream {
	return &Stream{clockRate: float64(clockRate), stall: stall}
}

// Packet is the part of an RTP packet the statistics need.
type Packet struct {
	Seq       uint16
	Timestamp uint32
	Marker    bool
	// KeyStart is true when the packet starts a keyframe (see KeyframeStart).
	KeyStart bool
}

// Add records one packet. arrival is measured on a monotonic clock from any fixed
// origin; only differences are used.
func (s *Stream) Add(p Packet, arrival time.Duration) {
	dup, late := s.extend(p.Seq)
	if late {
		s.tooLate++
		return
	}
	if dup {
		s.duplicates++
		return
	}
	s.received++

	// Interarrival jitter, RFC 3550 A.8.
	arr := arrival.Seconds() * s.clockRate
	if s.received > 1 {
		d := (arr - s.lastArrivalTS) - float64(int32(p.Timestamp-s.lastTS)) //nolint:gosec // signed RTP difference
		s.jitter += (math.Abs(d) - s.jitter) / 16
		gap := arrival - s.lastArrival
		if gap > s.longestGap {
			s.longestGap = gap
		}
		if s.stall > 0 && gap >= s.stall {
			s.stalls++
			s.stalled += gap
		}
	} else {
		s.firstArrival = arrival
	}
	s.lastArrivalTS, s.lastTS, s.lastArrival = arr, p.Timestamp, arrival

	s.keys.add(p, arrival, s.clockRate)
}

// extend maps a 16-bit sequence number to the extended one (RFC 3550 A.1) and says
// whether it was already seen, or is too old for the window to tell.
func (s *Stream) extend(seq uint16) (dup, late bool) {
	if !s.started {
		s.started = true
		s.maxSeq = seq
		s.baseExt = int64(seq)
		s.maxExt = int64(seq)
		s.mark(seq)
		return false, false
	}
	delta := int16(seq - s.maxSeq) //nolint:gosec // signed sequence difference
	if delta > 0 {
		if seq < s.maxSeq {
			s.cycles += 65536
		}
		ext := s.cycles + int64(seq)
		// The bits for the numbers between the old maximum and the new one belonged to
		// the previous cycle; clear them before reusing them.
		if ext-s.maxExt >= 65536 {
			s.seen = [65536 / 64]uint64{}
		} else {
			for e := s.maxExt + 1; e < ext; e++ {
				s.unmark(uint16(e)) //nolint:gosec // low 16 bits on purpose
			}
		}
		s.maxSeq = seq
		s.maxExt = ext
		s.mark(seq)
		return false, false
	}
	// Reordered, late or duplicate: same cycle as the maximum, or the one before.
	ext := s.cycles + int64(seq)
	if seq > s.maxSeq {
		ext -= 65536
	}
	if ext <= s.maxExt-65536 {
		return false, true
	}
	// Within the window each bit stands for exactly one extended number.
	if s.isMarked(seq) {
		return true, false
	}
	if ext < s.baseExt {
		s.baseExt = ext // older than the first packet seen: the stream started earlier
	}
	s.mark(seq)
	return false, false
}

func (s *Stream) mark(seq uint16)          { s.seen[seq/64] |= 1 << (seq % 64) }
func (s *Stream) unmark(seq uint16)        { s.seen[seq/64] &^= 1 << (seq % 64) }
func (s *Stream) isMarked(seq uint16) bool { return s.seen[seq/64]&(1<<(seq%64)) != 0 }

// Summary is the stream's statistics at the end of a run.
type Summary struct {
	Received   uint64 `json:"received"`
	Expected   uint64 `json:"expected"`
	Lost       uint64 `json:"lost"`
	Duplicates uint64 `json:"duplicates"`
	TooLate    uint64 `json:"tooLate"`
	// LossPercent is Lost/Expected·100.
	LossPercent float64 `json:"lossPercent"`
	JitterMs    float64 `json:"jitterMs"`
	// ReceivingSeconds is the time from the first packet to the last.
	ReceivingSeconds float64 `json:"receivingSeconds"`
	Stalls           int     `json:"stalls"`
	StalledMs        float64 `json:"stalledMs"`
	LongestGapMs     float64 `json:"longestGapMs"`
	Keyframes        int     `json:"keyframes"`
	// KeyframeInterval is over the spacing of consecutive keyframes, in seconds of
	// RTP time — the GOP the stream carries, independent of network timing.
	KeyframeIntervalMinS  float64 `json:"keyframeIntervalMinS"`
	KeyframeIntervalMeanS float64 `json:"keyframeIntervalMeanS"`
	KeyframeIntervalMaxS  float64 `json:"keyframeIntervalMaxS"`
}

// Summary returns the statistics so far.
func (s *Stream) Summary() Summary {
	out := Summary{
		Received: s.received, Duplicates: s.duplicates, TooLate: s.tooLate,
		JitterMs:     s.jitter / s.clockRate * 1000,
		Stalls:       s.stalls,
		StalledMs:    ms(s.stalled),
		LongestGapMs: ms(s.longestGap),
		Keyframes:    s.keys.count,
	}
	if s.started {
		out.Expected = uint64(s.maxExt - s.baseExt + 1) //nolint:gosec // maxExt >= baseExt
		if out.Expected > s.received {
			out.Lost = out.Expected - s.received
		}
		out.LossPercent = float64(out.Lost) / float64(out.Expected) * 100
		out.ReceivingSeconds = (s.lastArrival - s.firstArrival).Seconds()
	}
	if n := len(s.keys.intervals); n > 0 {
		lo, hi, sum := math.Inf(1), math.Inf(-1), 0.0
		for _, v := range s.keys.intervals {
			lo, hi, sum = math.Min(lo, v), math.Max(hi, v), sum+v
		}
		out.KeyframeIntervalMinS, out.KeyframeIntervalMaxS, out.KeyframeIntervalMeanS = lo, hi, sum/float64(n)
	}
	return out
}

// FirstKeyframe is the arrival time of the last packet of the first keyframe
// received complete, and whether there was one.
func (s *Stream) FirstKeyframe() (time.Duration, bool) {
	return s.keys.firstComplete, s.keys.haveComplete
}

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

// keyframes tracks keyframe starts, their spacing, and the first complete one.
//
// A keyframe is complete when the marker packet of its timestamp arrives and every
// sequence number from the last key-start packet of that timestamp to the marker
// was received — the closest a viewer without a decoder can get to "the first
// frame it could decode".
type keyframes struct {
	count     int
	lastKeyTS uint32
	haveLast  bool
	intervals []float64

	pending      bool
	pendTS       uint32
	pendStart    uint16
	pendCount    int
	haveComplete bool

	firstComplete time.Duration
}

func (k *keyframes) add(p Packet, arrival time.Duration, clockRate float64) {
	if p.KeyStart {
		if !k.haveLast || p.Timestamp != k.lastKeyTS {
			if k.haveLast {
				k.intervals = append(k.intervals, float64(int32(p.Timestamp-k.lastKeyTS))/clockRate) //nolint:gosec // signed RTP difference
			}
			k.count++
			k.lastKeyTS, k.haveLast = p.Timestamp, true
		}
		k.pending, k.pendTS, k.pendStart, k.pendCount = true, p.Timestamp, p.Seq, 0
	}
	if !k.pending || p.Timestamp != k.pendTS {
		return
	}
	k.pendCount++
	if p.Marker {
		if int(p.Seq-k.pendStart)+1 == k.pendCount && !k.haveComplete {
			k.haveComplete, k.firstComplete = true, arrival
		}
		k.pending = false
	}
}

// KeyframeStart reports whether an RTP payload starts a keyframe.
//
// VP8 (RFC 7741 §4.2, §4.3): the payload descriptor's S bit is set and its partition
// index is 0, and bit 0 of the first byte of the VP8 payload header is 0.
//
// H.264 (RFC 6184 §5.6–5.8): a single NAL unit, a STAP-A carrying one, or the first
// fragment of an FU-A, whose type is IDR (5) or SPS (7). SPS counts because servers
// and encoders send it, in its own packet or aggregated, ahead of the IDR slice.
func KeyframeStart(codec string, payload []byte) bool {
	switch codec {
	case "vp8":
		return vp8KeyStart(payload)
	case "h264":
		return h264KeyStart(payload)
	}
	return false
}

func vp8KeyStart(p []byte) bool {
	if len(p) < 1 {
		return false
	}
	b0 := p[0]
	if b0&0x10 == 0 || b0&0x07 != 0 { // S bit, partition index
		return false
	}
	i := 1
	if b0&0x80 != 0 { // X: extension byte follows
		if len(p) <= i {
			return false
		}
		x := p[i]
		i++
		if x&0x80 != 0 { // I: picture ID, 7 or 15 bits
			if len(p) <= i {
				return false
			}
			if p[i]&0x80 != 0 {
				i += 2
			} else {
				i++
			}
		}
		if x&0x40 != 0 { // L: TL0PICIDX
			i++
		}
		if x&0x30 != 0 { // T or K: TID/Y/KEYIDX byte
			i++
		}
	}
	return len(p) > i && p[i]&0x01 == 0
}

func h264KeyStart(p []byte) bool {
	if len(p) < 1 {
		return false
	}
	isKey := func(t byte) bool { return t == 5 || t == 7 }
	switch t := p[0] & 0x1F; {
	case t >= 1 && t <= 23:
		return isKey(t)
	case t == 24: // STAP-A: 16-bit size, NAL, repeated
		for i := 1; i+2 < len(p); {
			n := int(p[i])<<8 | int(p[i+1])
			i += 2
			if n == 0 || i+n > len(p) {
				return false
			}
			if isKey(p[i] & 0x1F) {
				return true
			}
			i += n
		}
	case t == 28: // FU-A: indicator, header (S E R type)
		return len(p) > 1 && p[1]&0x80 != 0 && isKey(p[1]&0x1F)
	}
	return false
}
