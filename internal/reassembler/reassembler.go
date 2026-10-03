// Package reassembler turns one viewer's RTP packets, each with its arrival instant, into
// complete frames. There is one Reassembler per viewer, fed from its read loop.
//
// It is not pion's samplebuilder, for three reasons (D4): the samplebuilder pops a frame
// only when the next one arrives, so the frame's own arrival is lost behind its
// successor's; it records no arrival at all; and it gives up on a frame by sequence count,
// not by time. A one-way delay needs the instant the frame was whole at the viewer, so
// this package keeps the arrival of every packet and decides completeness itself.
//
// A frame — the packets sharing one RTP timestamp — is complete when every sequence
// number between its first and last packet has arrived, its head is proven (the packet
// just before its first carries an earlier timestamp, so nothing of the frame was lost
// ahead of it) and its end is known. A frame still incomplete Window after its first
// packet arrived is given up and counted, except a viewer's first frame, whose head it can
// never prove.
//
// The end of a frame is known by one of two rules (D5), reported by FrameEnd: the marker
// bit on its last packet, or — for a sender or relay that sets no marker — the arrival of
// the next sequence number under a later timestamp. The reassembler starts on the marker
// rule and switches to the timestamp rule, for good, the first time a frame ends without
// one. A frame's arrival is that of its last packet: the instant it became whole.
//
// Each Frame also carries the instant the reassembler found it complete, which can be
// later than its arrival when a reordered packet fills a gap. Nothing reads it yet; it is
// kept for WB-41 (D6).
package reassembler

import (
	"fmt"
	"sort"
	"time"

	"github.com/Allan-Nava/whipbench/internal/clip"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
)

// Window is how long a frame may stay incomplete after its first packet arrived. The
// one-way delay Method line in internal/report states it; S7 observes whether it
// outlasts pion's NACK retries.
const Window = time.Second

// Frame-end rules, as the report's frameEnd spells them (D5). EndMarker means frames end
// on the RTP marker bit; EndTimestamp means the sender sets none, so a frame ends when the
// next sequence number arrives under a later timestamp.
const (
	EndMarker    = "marker"
	EndTimestamp = "timestamp"
)

// Frame is one complete frame, with the two instants a delay is measured against.
type Frame struct {
	Timestamp uint32
	Payload   []byte    // depacketised: the VP8 frame, or H.264 Annex-B (4-byte start codes); nil when Rejected
	Rejected  bool      // the depacketiser refused it (STAP-B, MTAP, FU-B…): a server re-packetised it
	Arrival   time.Time // t1: the arrival of the frame's last packet, its own arrival (D5)
	Completed time.Time // when the reassembler found it complete; unused until WB-41 (D6)
}

// seen is what the reassembler remembers of every recent packet, closed frames included:
// enough to prove the head of the frame after it, and the end of the frame before it.
type seen struct {
	ts uint32
	at time.Time
}

// packet is one packet of a pending frame. The payload is a copy: pion reuses the read
// buffer, so a slice of it would be overwritten before the frame completes.
type packet struct {
	payload []byte
	marker  bool
	arrival time.Time
}

// pending is a frame still missing a packet, its head proof or its end.
type pending struct {
	pkts   map[uint64]packet
	lo, hi uint64
	first  time.Time
}

// Reassembler is one viewer's frame reassembly. Not safe for concurrent use: the
// viewer's read loop is its only caller.
type Reassembler struct {
	newDep func() rtp.Depacketizer

	haveSeq bool
	maxExt  uint64

	seen    map[uint64]seen
	pending map[uint32]*pending
	closed  map[uint32]time.Time

	earliest     uint32
	haveEarliest bool

	end        string
	incomplete uint64
	lastPrune  time.Time
}

// New returns a reassembler for codec, clip.VP8 or clip.H264. Any other codec is an
// error: without a depacketiser no frame can be compared with the clip.
func New(codec string) (*Reassembler, error) {
	var newDep func() rtp.Depacketizer
	switch codec {
	case clip.VP8:
		newDep = func() rtp.Depacketizer { return &codecs.VP8Packet{} }
	case clip.H264:
		newDep = func() rtp.Depacketizer { return &codecs.H264Packet{} }
	default:
		return nil, fmt.Errorf("reassembler: no depacketiser for codec %q", codec)
	}
	return &Reassembler{
		newDep:  newDep,
		seen:    make(map[uint64]seen),
		pending: make(map[uint32]*pending),
		closed:  make(map[uint32]time.Time),
	}, nil
}

// Push adds one packet that arrived at arrival and returns the frames it completed, in
// sequence order, or nil. arrival is the reassembler's only clock, so a test can inject
// it. The packet's payload is copied; p may be reused once Push returns.
func (r *Reassembler) Push(p *rtp.Packet, arrival time.Time) []Frame {
	r.expire(arrival)

	ext := r.extend(p.SequenceNumber)
	if _, dup := r.seen[ext]; dup {
		return nil // the first copy wins
	}
	ts := p.Timestamp
	r.seen[ext] = seen{ts: ts, at: arrival}
	if !r.haveEarliest || int32(ts-r.earliest) < 0 { //nolint:gosec // RTP timestamp difference, wraps by design
		r.earliest, r.haveEarliest = ts, true
	}

	if _, ok := r.closed[ts]; !ok {
		f, ok := r.pending[ts]
		if !ok {
			f = &pending{pkts: make(map[uint64]packet), lo: ext, hi: ext, first: arrival}
			r.pending[ts] = f
		}
		f.pkts[ext] = packet{
			payload: append([]byte(nil), p.Payload...),
			marker:  p.Marker,
			arrival: arrival,
		}
		if ext < f.lo {
			f.lo = ext
		}
		if ext > f.hi {
			f.hi = ext
		}
	}

	out := r.complete(arrival)

	if arrival.Sub(r.lastPrune) >= Window {
		r.prune(arrival)
	}
	return out
}

// Incomplete returns how many frames were given up after Window, the viewer's first frame
// excepted. Frames still pending when the viewer stops are not counted.
func (r *Reassembler) Incomplete() uint64 { return r.incomplete }

// FrameEnd returns the frame-end rule in force, EndMarker or EndTimestamp, or "" until
// the first frame completes.
func (r *Reassembler) FrameEnd() string { return r.end }

// expire gives up every pending frame whose first packet arrived Window or more ago.
func (r *Reassembler) expire(arrival time.Time) {
	for ts, f := range r.pending {
		if arrival.Sub(f.first) < Window {
			continue
		}
		delete(r.pending, ts)
		r.closed[ts] = arrival
		if ts != r.earliest {
			r.incomplete++
		}
	}
}

// extend maps a 16-bit sequence number onto a 64-bit line, so that wrap-around never
// reorders it. The first packet starts at 1<<32, far from zero in both directions.
func (r *Reassembler) extend(seq uint16) uint64 {
	if !r.haveSeq {
		r.haveSeq = true
		r.maxExt = uint64(seq) + 1<<32
		return r.maxExt
	}
	ext := uint64(int64(r.maxExt) + int64(int16(seq-uint16(r.maxExt)))) //nolint:gosec // extended sequence numbers stay near 1<<32, far from either bound
	if ext > r.maxExt {
		r.maxExt = ext
	}
	return ext
}

// complete closes and depacketises every pending frame that is now whole, in ascending
// order of its first sequence number.
func (r *Reassembler) complete(arrival time.Time) []Frame {
	order := make([]uint32, 0, len(r.pending))
	for ts := range r.pending {
		order = append(order, ts)
	}
	sort.Slice(order, func(i, j int) bool { return r.pending[order[i]].lo < r.pending[order[j]].lo })

	var out []Frame
	for _, ts := range order {
		f := r.pending[ts]
		if !r.whole(ts, f) {
			continue
		}
		delete(r.pending, ts)
		r.closed[ts] = arrival
		out = append(out, r.depacketise(ts, f, arrival))
	}
	return out
}

// whole reports whether f holds every packet, its head is proven and its end is known. It
// sets the frame-end rule as a side effect: EndMarker on a marker, EndTimestamp — for
// good — on a frame that ended without one.
func (r *Reassembler) whole(ts uint32, f *pending) bool {
	if uint64(len(f.pkts)) != f.hi-f.lo+1 {
		return false
	}
	head, ok := r.seen[f.lo-1]
	if !ok || int32(head.ts-ts) >= 0 { //nolint:gosec // RTP timestamp difference, wraps by design
		return false
	}

	next, ok := r.seen[f.hi+1]
	nextIsLater := ok && int32(next.ts-ts) > 0 //nolint:gosec // RTP timestamp difference, wraps by design
	if r.end == EndTimestamp {
		return nextIsLater // a marker no longer ends a frame, nor stops one ending
	}
	if f.pkts[f.hi].marker {
		r.end = EndMarker
		return true
	}
	if nextIsLater && !hasMarker(f) {
		r.end = EndTimestamp
		return true
	}
	return false
}

func hasMarker(f *pending) bool {
	for _, p := range f.pkts {
		if p.marker {
			return true
		}
	}
	return false
}

// depacketise runs f's payloads, lo to hi, through one fresh depacketiser, so that no
// fragment of an earlier frame can leak into this one.
func (r *Reassembler) depacketise(ts uint32, f *pending, arrival time.Time) Frame {
	fr := Frame{Timestamp: ts, Arrival: f.pkts[f.hi].arrival, Completed: arrival}
	dep := r.newDep()
	var payload []byte
	for ext := f.lo; ext <= f.hi; ext++ {
		b, err := dep.Unmarshal(f.pkts[ext].payload)
		if err != nil {
			fr.Rejected = true
			return fr
		}
		payload = append(payload, b...)
	}
	fr.Payload = payload
	return fr
}

// prune forgets seen packets and closed frames older than 2·Window, which bounds the
// reassembler's state however long the viewer runs.
func (r *Reassembler) prune(arrival time.Time) {
	for ext, s := range r.seen {
		if arrival.Sub(s.at) > 2*Window {
			delete(r.seen, ext)
		}
	}
	for ts, at := range r.closed {
		if arrival.Sub(at) > 2*Window {
			delete(r.closed, ts)
		}
	}
	r.lastPrune = arrival
}
