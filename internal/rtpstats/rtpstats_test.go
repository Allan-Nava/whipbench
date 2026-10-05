package rtpstats

import (
	"math"
	"testing"
	"time"

	"github.com/Allan-Nava/whipbench"
	"github.com/Allan-Nava/whipbench/internal/clip"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
)

const clock = 90000

// at returns the arrival time at which a packet with RTP timestamp ts would arrive
// with zero jitter (transit constant), plus extra.
func at(ts uint32, extra time.Duration) time.Duration {
	return time.Duration(float64(ts)/clock*float64(time.Second)) + extra
}

func feed(s *Stream, seqs []uint16) {
	for _, q := range seqs {
		ts := uint32(q) * 3000
		s.Add(Packet{Seq: q, Timestamp: ts}, at(ts, 0))
	}
}

func seqRange(from uint16, n int) []uint16 {
	out := make([]uint16, n)
	for i := range out {
		out[i] = from + uint16(i) //nolint:gosec // test
	}
	return out
}

func TestNoLossInOrder(t *testing.T) {
	s := New(clock, 0)
	feed(s, seqRange(100, 1000))
	got := s.Summary()
	if got.Expected != 1000 || got.Received != 1000 || got.Lost != 0 || got.JitterMs > 1e-5 { // ns rounding of the synthetic arrivals
		t.Fatalf("summary = %+v", got)
	}
}

func TestLossFromGaps(t *testing.T) {
	s := New(clock, 0)
	var seqs []uint16
	for _, q := range seqRange(0, 1000) {
		if q%100 == 50 { // drop ten
			continue
		}
		seqs = append(seqs, q)
	}
	feed(s, seqs)
	got := s.Summary()
	if got.Expected != 1000 || got.Lost != 10 || math.Abs(got.LossPercent-1) > 1e-12 {
		t.Fatalf("summary = %+v", got)
	}
}

func TestSequenceWrap(t *testing.T) {
	s := New(clock, 0)
	seqs := seqRange(65500, 200) // crosses 65535 → 0
	seqs = append(seqs[:10], seqs[11:]...)
	for _, q := range seqs {
		s.Add(Packet{Seq: q}, 0)
	}
	got := s.Summary()
	if got.Expected != 200 || got.Lost != 1 {
		t.Fatalf("across the wrap: %+v", got)
	}
}

func TestReorderIsNotLoss(t *testing.T) {
	s := New(clock, 0)
	seqs := seqRange(65530, 20)
	for i := 0; i+1 < len(seqs); i += 2 {
		seqs[i], seqs[i+1] = seqs[i+1], seqs[i]
	}
	for _, q := range seqs {
		s.Add(Packet{Seq: q}, 0)
	}
	got := s.Summary()
	if got.Expected != 20 || got.Lost != 0 || got.Duplicates != 0 {
		t.Fatalf("reordered pairs: %+v", got)
	}
}

// WB-41: a NACK retransmission, with RTX not negotiated, arrives on its original sequence
// number well after the packets behind it. Lost counts it while it is missing and not
// once it has arrived: it is received, never tooLate, and lost is what never arrived.
func TestRetransmissionFillsAGap(t *testing.T) {
	s := New(clock, 0)
	feed(s, []uint16{100, 101, 103, 104, 105})
	if got := s.Summary(); got.Lost != 1 || got.Received != 5 {
		t.Fatalf("before the retransmission: %+v", got)
	}
	s.Add(Packet{Seq: 102, Timestamp: 102 * 3000}, at(105*3000, 120*time.Millisecond))
	feed(s, []uint16{106})
	got := s.Summary()
	if got.Lost != 0 || got.Received != 7 || got.Expected != 7 || got.TooLate != 0 || got.Duplicates != 0 {
		t.Fatalf("after the retransmission: %+v", got)
	}
	// A retransmission the stream already has — a NACK answered twice — is a duplicate.
	s.Add(Packet{Seq: 102, Timestamp: 102 * 3000}, at(106*3000, 0))
	if got := s.Summary(); got.Duplicates != 1 || got.Received != 7 || got.Lost != 0 {
		t.Fatalf("a second copy: %+v", got)
	}
}

func TestDuplicatesAreNotReceivedTwice(t *testing.T) {
	s := New(clock, 0)
	seqs := append(seqRange(0, 100), 10, 20, 30, 40, 50)
	for _, q := range seqs {
		s.Add(Packet{Seq: q}, 0)
	}
	got := s.Summary()
	if got.Received != 100 || got.Duplicates != 5 || got.Lost != 0 {
		t.Fatalf("duplicates: %+v", got)
	}
}

func TestWindowClearsOnAdvance(t *testing.T) {
	// The same 16-bit number one full cycle later is a new packet, not a duplicate.
	s := New(clock, 0)
	for _, q := range seqRange(0, 70000%65536) {
		s.Add(Packet{Seq: q}, 0)
	}
	for _, q := range seqRange(70000%65536, 65536) {
		s.Add(Packet{Seq: q}, 0)
	}
	got := s.Summary()
	if got.Duplicates != 0 || got.Lost != 0 || got.Expected != 70000 {
		t.Fatalf("two cycles: %+v", got)
	}
}

// The RFC 3550 A.8 recurrence, written independently of the implementation.
func referenceJitter(arrivals []float64, ts []float64) float64 {
	j := 0.0
	for i := 1; i < len(ts); i++ {
		d := (arrivals[i] - arrivals[i-1]) - (ts[i] - ts[i-1])
		j += (math.Abs(d) - j) / 16
	}
	return j
}

func TestJitterMatchesRFC3550(t *testing.T) {
	s := New(clock, 0)
	var arr, tss []float64
	delays := []time.Duration{0, 4, 1, 9, 0, 2, 15, 3, 3, 0, 7} // ms
	for i := 0; i < 300; i++ {
		ts := uint32(i) * 3000 //nolint:gosec // test
		d := delays[i%len(delays)] * time.Millisecond
		a := at(ts, d)
		s.Add(Packet{Seq: uint16(i), Timestamp: ts}, a) //nolint:gosec // test
		arr = append(arr, a.Seconds()*clock)
		tss = append(tss, float64(ts))
	}
	want := referenceJitter(arr, tss) / clock * 1000
	if got := s.Summary().JitterMs; math.Abs(got-want) > 1e-6 {
		t.Fatalf("jitter = %.6f ms, reference %.6f ms", got, want)
	}
}

func TestJitterConvergesToAlternatingDelay(t *testing.T) {
	// Arrivals alternate 0 and 10 ms late: |D| is 10 ms on every packet, so J → 10 ms.
	s := New(clock, 0)
	for i := 0; i < 2000; i++ {
		ts := uint32(i) * 3000                                                                       //nolint:gosec // test
		s.Add(Packet{Seq: uint16(i), Timestamp: ts}, at(ts, time.Duration(i%2)*10*time.Millisecond)) //nolint:gosec // test
	}
	if got := s.Summary().JitterMs; math.Abs(got-10) > 0.01 {
		t.Fatalf("jitter = %.4f ms, want 10", got)
	}
}

func TestJitterIgnoresConstantTransit(t *testing.T) {
	s := New(clock, 0)
	for i := 0; i < 100; i++ {
		ts := uint32(i) * 3000                                                     //nolint:gosec // test
		s.Add(Packet{Seq: uint16(i), Timestamp: ts}, at(ts, 250*time.Millisecond)) //nolint:gosec // test
	}
	if got := s.Summary().JitterMs; got > 1e-5 { // ns rounding only
		t.Fatalf("a constant delay is not jitter, got %v ms", got)
	}
}

func TestStalls(t *testing.T) {
	s := New(clock, 500*time.Millisecond)
	s.Add(Packet{Seq: 0}, 0)
	s.Add(Packet{Seq: 1}, 100*time.Millisecond)
	s.Add(Packet{Seq: 2}, 700*time.Millisecond) // 600 ms gap
	s.Add(Packet{Seq: 3}, 750*time.Millisecond)
	got := s.Summary()
	if got.Stalls != 1 || got.StalledMs != 600 || got.LongestGapMs != 600 || got.ReceivingSeconds != 0.75 {
		t.Fatalf("stalls: %+v", got)
	}
}

// frames builds packets for n frames of 3 packets each, a keyframe every gop frames.
func frames(n, gop int, drop func(seq uint16) bool) []Packet {
	var out []Packet
	seq := uint16(0)
	for f := 0; f < n; f++ {
		ts := uint32(f) * 3000 //nolint:gosec // test
		for i := 0; i < 3; i++ {
			p := Packet{Seq: seq, Timestamp: ts, Marker: i == 2, KeyStart: i == 0 && f%gop == 0}
			if drop == nil || !drop(seq) {
				out = append(out, p)
			}
			seq++
		}
	}
	return out
}

func TestKeyframeIntervalAndFirstComplete(t *testing.T) {
	s := New(clock, 0)
	for i, p := range frames(120, 30, nil) {
		s.Add(p, time.Duration(i)*time.Millisecond)
	}
	got := s.Summary()
	if got.Keyframes != 4 || got.KeyframeIntervalMinS != 1 || got.KeyframeIntervalMaxS != 1 || got.KeyframeIntervalMeanS != 1 {
		t.Fatalf("keyframes: %+v", got)
	}
	first, ok := s.FirstKeyframe()
	if !ok || first != 2*time.Millisecond { // packet index 2 is the first marker
		t.Fatalf("first complete keyframe at %v (%v)", first, ok)
	}
}

func TestFirstKeyframeSkipsAnIncompleteOne(t *testing.T) {
	s := New(clock, 0)
	// Lose the middle packet of the first keyframe: the first complete one is the
	// next keyframe, frame 30, whose marker is sequence number 30*3+2.
	pkts := frames(60, 30, func(q uint16) bool { return q == 1 })
	var want time.Duration
	for i, p := range pkts {
		a := time.Duration(i) * time.Millisecond
		if p.Seq == 92 {
			want = a
		}
		s.Add(p, a)
	}
	first, ok := s.FirstKeyframe()
	if !ok || first != want {
		t.Fatalf("first complete keyframe at %v, want %v", first, want)
	}
}

// Packetise the real clips with pion's payloaders and check that exactly the first
// packet of each keyframe is recognised, and nothing else.
func TestKeyframeStartOnRealPayloads(t *testing.T) {
	for _, tc := range []struct {
		codec string
		data  []byte
		pay   rtp.Payloader
	}{
		{clip.VP8, whipbench.ClipVP8, &codecs.VP8Payloader{EnablePictureID: true}},
		{clip.H264, whipbench.ClipH264, &codecs.H264Payloader{}},
	} {
		c, err := clip.Load(tc.codec, tc.data)
		if err != nil {
			t.Fatal(err)
		}
		starts := 0
		for i, f := range c.Frames {
			for j, p := range tc.pay.Payload(1200, f.Data) {
				got := KeyframeStart(tc.codec, p)
				if got && !f.Key {
					t.Fatalf("%s frame %d packet %d: delta frame taken for a keyframe", tc.codec, i, j)
				}
				if got {
					starts++
				}
				if f.Key && j == 0 && !got {
					t.Fatalf("%s frame %d: first packet of a keyframe not recognised", tc.codec, i)
				}
			}
		}
		if starts < 4 {
			t.Fatalf("%s: %d key-start packets, want at least 4", tc.codec, starts)
		}
	}
}
