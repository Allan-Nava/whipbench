package reassembler

import (
	"bytes"
	"testing"
	"time"

	"github.com/pion/rtp"
)

var base = time.Unix(1000, 0)

// at is the arrival instant ms milliseconds after base.
func at(ms float64) time.Time { return base.Add(time.Duration(ms * float64(time.Millisecond))) }

func pk(seq uint16, ts uint32, marker bool, payload ...byte) *rtp.Packet {
	return &rtp.Packet{
		Header:  rtp.Header{Version: 2, SequenceNumber: seq, Timestamp: ts, Marker: marker},
		Payload: payload,
	}
}

// s is a VP8 payload whose descriptor has the S bit: a frame's first packet.
func s(b byte) []byte { return []byte{0x10, b} }

// c is a VP8 continuation payload.
func c(b byte) []byte { return []byte{0x00, b} }

func newR(t *testing.T, codec string) *Reassembler {
	t.Helper()
	r, err := New(codec)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// prime pushes a first frame that never completes and is never counted; its seq 9 proves
// the next frame's head.
func prime(r *Reassembler) { r.Push(pk(9, 0, true, s('p')...), at(0)) }

func push(t *testing.T, r *Reassembler, p *rtp.Packet, ms float64) []Frame {
	t.Helper()
	return r.Push(p, at(ms))
}

func none(t *testing.T, got []Frame, what string) {
	t.Helper()
	if got != nil {
		t.Fatalf("%s: got %d frames, want nil", what, len(got))
	}
}

func one(t *testing.T, got []Frame, payload string, what string) Frame {
	t.Helper()
	if len(got) != 1 {
		t.Fatalf("%s: got %d frames, want 1", what, len(got))
	}
	if string(got[0].Payload) != payload {
		t.Fatalf("%s: payload %q, want %q", what, got[0].Payload, payload)
	}
	return got[0]
}

func TestCompleteOnMarker(t *testing.T) {
	r := newR(t, "vp8")
	prime(r)
	none(t, push(t, r, pk(10, 3000, false, s('a')...), 10), "push 10")
	none(t, push(t, r, pk(11, 3000, false, c('b')...), 11), "push 11")
	f := one(t, push(t, r, pk(12, 3000, true, c('c')...), 12), "abc", "push 12")
	if f.Timestamp != 3000 || f.Rejected || !f.Arrival.Equal(at(12)) || !f.Completed.Equal(at(12)) {
		t.Fatalf("frame %+v", f)
	}
	if r.FrameEnd() != EndMarker {
		t.Fatalf("FrameEnd() = %q, want %q", r.FrameEnd(), EndMarker)
	}
}

func TestFirstCopyWins(t *testing.T) {
	r := newR(t, "vp8")
	prime(r)
	none(t, push(t, r, pk(10, 3000, false, s('a')...), 10), "push 10")
	none(t, push(t, r, pk(12, 3000, true, c('c')...), 12), "push 12")
	none(t, push(t, r, pk(12, 3000, true, c('X')...), 13), "duplicate 12")
	f := one(t, push(t, r, pk(11, 3000, false, c('b')...), 14), "abc", "push 11")
	if !f.Arrival.Equal(at(12)) || !f.Completed.Equal(at(14)) {
		t.Fatalf("Arrival %v Completed %v, want at(12), at(14)", f.Arrival, f.Completed)
	}
	none(t, push(t, r, pk(12, 3000, true, c('Y')...), 15), "duplicate 12 after completion")
}

// WB-41: a frame is completed late when one of its own packets arrives after its marker
// packet; a frame that waits only for the packet proving its head — the frame before's —
// is not, though its Completed is later than its Arrival. First is the arrival of the
// first packet to arrive, whichever its sequence number.
func TestLateCompleted(t *testing.T) {
	r := newR(t, "vp8")
	prime(r)
	none(t, push(t, r, pk(10, 3000, false, s('a')...), 10), "push 10")
	f := one(t, push(t, r, pk(11, 3000, true, c('b')...), 11), "ab", "push 11")
	if f.Late || !f.First.Equal(at(10)) || r.LateCompleted() != 0 {
		t.Fatalf("an in-order frame: %+v, LateCompleted %d", f, r.LateCompleted())
	}

	// Frame 6000: 13 then the marker 14, and 12 — its middle — retransmitted after both.
	none(t, push(t, r, pk(13, 6000, false, c('d')...), 40), "push 13")
	none(t, push(t, r, pk(14, 6000, true, c('e')...), 41), "push 14")
	f = one(t, push(t, r, pk(12, 6000, false, s('c')...), 140), "cde", "retransmitted 12")
	if !f.Late || !f.Arrival.Equal(at(41)) || !f.Completed.Equal(at(140)) || !f.First.Equal(at(40)) {
		t.Fatalf("a gap filled after the marker: %+v", f)
	}
	if r.LateCompleted() != 1 {
		t.Fatalf("LateCompleted() = %d, want 1", r.LateCompleted())
	}

	// Frame 9000's marker 16 arrives after frame 12000 (17, 18): 9000's last packet is its
	// marker, so it is not late; 12000 waited only for its head proof, so it is not either.
	none(t, push(t, r, pk(15, 9000, false, s('f')...), 70), "push 15")
	none(t, push(t, r, pk(17, 12000, false, s('h')...), 72), "push 17")
	none(t, push(t, r, pk(18, 12000, true, c('i')...), 73), "push 18")
	got := push(t, r, pk(16, 9000, true, c('g')...), 90)
	if len(got) != 2 || got[0].Late || got[1].Late || !got[1].Completed.After(got[1].Arrival) {
		t.Fatalf("frames %+v", got)
	}
	if r.LateCompleted() != 1 {
		t.Fatalf("LateCompleted() = %d after a late head proof, want still 1", r.LateCompleted())
	}
}

func TestSequenceWrap(t *testing.T) {
	r := newR(t, "vp8")
	none(t, push(t, r, pk(65534, 0, true, s('p')...), 0), "push 65534")
	none(t, push(t, r, pk(65535, 3000, false, s('a')...), 10), "push 65535")
	none(t, push(t, r, pk(0, 3000, false, c('b')...), 11), "push 0")
	one(t, push(t, r, pk(1, 3000, true, c('c')...), 12), "abc", "push 1")
}

func TestLostHeadNeverCompletes(t *testing.T) {
	r := newR(t, "vp8")
	prime(r)
	none(t, push(t, r, pk(11, 3000, false, c('b')...), 10), "push 11")
	none(t, push(t, r, pk(12, 3000, true, c('c')...), 11), "push 12")
	none(t, push(t, r, pk(13, 6000, false, s('d')...), 20), "push 13")
	one(t, push(t, r, pk(14, 6000, true, c('e')...), 21), "de", "push 14")
	if n := r.Incomplete(); n != 0 {
		t.Fatalf("Incomplete() = %d before the window, want 0", n)
	}
	one(t, push(t, r, pk(15, 9000, true, s('f')...), 1010), "f", "push 15")
	if n := r.Incomplete(); n != 1 {
		t.Fatalf("Incomplete() = %d after the window, want 1", n)
	}
}

func TestHeadWithLaterTimestamp(t *testing.T) {
	r := newR(t, "vp8")
	none(t, push(t, r, pk(9, 6000, true, s('p')...), 0), "push 9")
	none(t, push(t, r, pk(10, 3000, false, s('a')...), 1), "push 10")
	none(t, push(t, r, pk(11, 3000, false, c('b')...), 2), "push 11")
	none(t, push(t, r, pk(12, 3000, true, c('c')...), 3), "push 12")
}

func TestIncompleteAfterWindow(t *testing.T) {
	r := newR(t, "vp8")
	prime(r)
	none(t, push(t, r, pk(10, 3000, false, s('a')...), 5), "push 10")
	none(t, push(t, r, pk(12, 3000, true, c('c')...), 7), "push 12")
	one(t, push(t, r, pk(13, 6000, true, s('d')...), 1004), "d", "push 13")
	if n := r.Incomplete(); n != 0 {
		t.Fatalf("Incomplete() = %d at 999 ms, want 0", n)
	}
	one(t, push(t, r, pk(14, 9000, true, s('e')...), 1005), "e", "push 14")
	if n := r.Incomplete(); n != 1 {
		t.Fatalf("Incomplete() = %d at 1000 ms, want 1", n)
	}
	none(t, push(t, r, pk(11, 3000, false, c('b')...), 1006), "late 11")
	if n := r.Incomplete(); n != 1 {
		t.Fatalf("Incomplete() = %d after the late packet, want 1", n)
	}
	one(t, push(t, r, pk(15, 12000, true, s('f')...), 2100), "f", "push 15")
	if n := r.Incomplete(); n != 1 {
		t.Fatalf("Incomplete() = %d at 2100 ms, want 1", n)
	}
}

func TestStaleFUBytesNeverHashed(t *testing.T) {
	r := newR(t, "h264")
	r.Push(pk(9, 0, true, 0x41, 0x01), at(0))
	none(t, push(t, r, pk(10, 3000, false, 0x7C, 0x85, 'x'), 1), "FU-A start")
	none(t, push(t, r, pk(12, 3000, true, 0x7C, 0x45, 'z'), 3), "FU-A end")
	got := push(t, r, pk(13, 6000, true, 0x41, 'b'), 4)
	if len(got) != 1 {
		t.Fatalf("got %d frames, want 1", len(got))
	}
	if want := []byte{0, 0, 0, 1, 0x41, 'b'}; !bytes.Equal(got[0].Payload, want) || got[0].Timestamp != 6000 {
		t.Fatalf("frame ts %d payload % x, want ts 6000 payload % x", got[0].Timestamp, got[0].Payload, want)
	}
}

func TestMarkerlessSwitch(t *testing.T) {
	r := newR(t, "vp8")
	prime(r)
	none(t, push(t, r, pk(10, 3000, false, s('a')...), 1), "push 10")
	one(t, push(t, r, pk(11, 3000, true, c('b')...), 2), "ab", "push 11")
	if r.FrameEnd() != EndMarker {
		t.Fatalf("FrameEnd() = %q after A, want %q", r.FrameEnd(), EndMarker)
	}
	none(t, push(t, r, pk(12, 6000, false, s('c')...), 20), "push 12")
	none(t, push(t, r, pk(13, 6000, false, c('d')...), 21), "push 13")
	b := one(t, push(t, r, pk(14, 9000, false, s('e')...), 30), "cd", "push 14")
	if !b.Arrival.Equal(at(21)) {
		t.Fatalf("B Arrival %v, want at(21)", b.Arrival)
	}
	if r.FrameEnd() != EndTimestamp {
		t.Fatalf("FrameEnd() = %q after B, want %q", r.FrameEnd(), EndTimestamp)
	}
	none(t, push(t, r, pk(15, 9000, true, c('f')...), 31), "push 15 (marker ignored)")
	cf := one(t, push(t, r, pk(16, 12000, false, s('g')...), 40), "ef", "push 16")
	if !cf.Arrival.Equal(at(31)) {
		t.Fatalf("C Arrival %v, want at(31)", cf.Arrival)
	}
}

func TestLostMarkerDoesNotSwitch(t *testing.T) {
	r := newR(t, "vp8")
	prime(r)
	none(t, push(t, r, pk(10, 3000, false, s('a')...), 1), "push 10")
	one(t, push(t, r, pk(11, 3000, true, c('b')...), 2), "ab", "push 11")
	none(t, push(t, r, pk(12, 6000, false, s('c')...), 3), "push 12")
	none(t, push(t, r, pk(14, 9000, false, s('e')...), 5), "push 14")
	none(t, push(t, r, pk(15, 9000, true, c('f')...), 6), "push 15")
	none(t, push(t, r, pk(16, 12000, false, s('g')...), 8), "push 16")
	one(t, push(t, r, pk(17, 12000, true, c('h')...), 9), "gh", "push 17")
	one(t, push(t, r, pk(18, 15000, true, s('i')...), 1010), "i", "push 18")
	if n := r.Incomplete(); n != 2 {
		t.Fatalf("Incomplete() = %d, want 2", n)
	}
	if r.FrameEnd() != EndMarker {
		t.Fatalf("FrameEnd() = %q, want %q", r.FrameEnd(), EndMarker)
	}
}

func TestRejected(t *testing.T) {
	r := newR(t, "h264")
	r.Push(pk(9, 0, true, 0x41, 0x01), at(0))
	for i, p := range []*rtp.Packet{
		pk(10, 3000, true, 0x19, 0x00, 0x01), // STAP-B
		pk(11, 6000, true, 0x1A, 0x00, 0x01), // MTAP16
		pk(12, 9000, true, 0x1D, 0x00, 0x01), // FU-B
	} {
		got := r.Push(p, at(float64(i+1)))
		if len(got) != 1 || !got[0].Rejected || got[0].Payload != nil {
			t.Fatalf("seq %d: got %+v, want one Rejected frame with nil Payload", p.SequenceNumber, got)
		}
	}
}

func TestFirstFrameNotCounted(t *testing.T) {
	r := newR(t, "vp8")
	none(t, push(t, r, pk(10, 3000, false, s('a')...), 0), "push 10")
	none(t, push(t, r, pk(11, 3000, true, c('b')...), 1), "push 11")
	one(t, push(t, r, pk(12, 6000, true, s('c')...), 1500), "c", "push 12")
	if n := r.Incomplete(); n != 0 {
		t.Fatalf("Incomplete() = %d, want 0", n)
	}
}

func TestPendingAtStopNotCounted(t *testing.T) {
	r := newR(t, "vp8")
	prime(r)
	none(t, push(t, r, pk(10, 3000, false, s('a')...), 1), "push 10")
	if n := r.Incomplete(); n != 0 {
		t.Fatalf("Incomplete() = %d, want 0", n)
	}
}

func TestStateIsBounded(t *testing.T) {
	r := newR(t, "vp8")
	var out, maxSeen, maxClosed, maxPending int
	for k := 0; k < 1800; k++ {
		for j := 0; j < 3; j++ {
			if k%10 == 5 && j == 1 {
				continue // lost; its sequence number is still consumed
			}
			payload := c(byte(k))
			if j == 0 {
				payload = s(byte(k))
			}
			seq := uint16(3*k + j)                           //nolint:gosec // wraps by design
			p := pk(seq, uint32(3000*k), j == 2, payload...) //nolint:gosec // k < 1800
			out += len(r.Push(p, at(float64(k)*1000/30+float64(j)*0.1)))
			maxSeen = max(maxSeen, len(r.seen))
			maxClosed = max(maxClosed, len(r.closed))
			maxPending = max(maxPending, len(r.pending))
		}
	}
	if out != 1619 {
		t.Fatalf("%d frames out, want 1619", out)
	}
	if n := r.Incomplete(); n != 177 {
		t.Fatalf("Incomplete() = %d, want 177", n)
	}
	if maxSeen >= 400 || maxClosed >= 200 || maxPending >= 40 {
		t.Fatalf("state maxima seen %d closed %d pending %d, want < 400, < 200, < 40", maxSeen, maxClosed, maxPending)
	}
	t.Logf("state maxima: seen %d, closed %d, pending %d", maxSeen, maxClosed, maxPending)
}

func TestNewCodec(t *testing.T) {
	if _, err := New("opus"); err == nil {
		t.Fatal(`New("opus"): no error`)
	}
	for _, codec := range []string{"vp8", "h264"} {
		if _, err := New(codec); err != nil {
			t.Fatalf("New(%q): %v", codec, err)
		}
	}
}
