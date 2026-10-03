package reassembler_test

import (
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"

	"github.com/Allan-Nava/whipbench"
	"github.com/Allan-Nava/whipbench/internal/clip"
	"github.com/Allan-Nava/whipbench/internal/fingerprint"
	"github.com/Allan-Nava/whipbench/internal/publisher"
	"github.com/Allan-Nava/whipbench/internal/reassembler"
)

// TestRoundTrip sends two loops of each embedded clip through the publisher's own
// payloader and back through the reassembler, offline, and checks that every frame that
// comes out fingerprints to the clip frame it was cut from. It is the one test that holds
// the whole chain — payloader, depacketiser, VCL hash, clip table — to the clips the
// binary actually streams, so a change to any link that breaks matching fails here
// before it reaches a live run.
func TestRoundTrip(t *testing.T) {
	cases := []struct {
		name  string
		codec string
		data  []byte
		pay   func() rtp.Payloader
	}{
		{"H264", clip.H264, whipbench.ClipH264, func() rtp.Payloader { return &codecs.H264Payloader{} }},
		{"VP8", clip.VP8, whipbench.ClipVP8, func() rtp.Payloader { return &codecs.VP8Payloader{EnablePictureID: true} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := clip.Load(tc.codec, tc.data)
			if err != nil {
				t.Fatal(err)
			}
			table, err := fingerprint.NewTable(c)
			if err != nil {
				t.Fatal(err)
			}
			r, err := reassembler.New(tc.codec)
			if err != nil {
				t.Fatal(err)
			}
			pay := tc.pay() // one payloader for the whole run, as the publisher keeps one per Stream

			n := len(c.Frames)
			seq := uint16(65000)       // wraps during the run
			base := uint32(0xFFFFF000) // wraps during the run
			start := time.Unix(1000, 0)

			var frames []reassembler.Frame
			stapA := 0
			// 2N + 1 frames: frame 0 is the viewer's first frame and never comes out (P4).
			for k := 0; k <= 2*n; k++ {
				ts := c.Timestamp(base, uint64(k))
				payloads := pay.Payload(publisher.MTU, c.Frame(uint64(k)).Data)
				for j, payload := range payloads {
					if tc.codec == clip.H264 && len(payload) > 0 && payload[0]&0x1F == 24 {
						stapA++
					}
					p := &rtp.Packet{
						Header: rtp.Header{
							Version:        2,
							SequenceNumber: seq,
							Timestamp:      ts,
							Marker:         j == len(payloads)-1,
						},
						Payload: payload,
					}
					seq++
					arrival := start.Add(time.Duration(k)*c.FrameDuration() + time.Duration(j)*time.Microsecond)
					frames = append(frames, r.Push(p, arrival)...)
				}
			}

			if len(frames) != 2*n {
				t.Fatalf("%d frames came out, want %d", len(frames), 2*n)
			}
			matched := 0
			for _, f := range frames {
				if f.Rejected {
					t.Errorf("ts %d: rejected by the depacketiser", f.Timestamp)
					continue
				}
				fp, ok := fingerprint.Of(tc.codec, f.Payload)
				if !ok {
					t.Errorf("ts %d: no fingerprint", f.Timestamp)
					continue
				}
				want := int((f.Timestamp-base)/c.Ticks) % n // uint32 subtraction handles the wrap
				i, st := table.Lookup(fp)
				if i != want || st != fingerprint.Unique {
					t.Errorf("ts %d: Lookup = (%d, %v), want (%d, Unique)", f.Timestamp, i, st, want)
					continue
				}
				matched++
			}
			if got := r.Incomplete(); got != 0 {
				t.Errorf("Incomplete() = %d, want 0", got)
			}
			if tc.codec == clip.H264 && stapA != 9 {
				t.Errorf("%d STAP-A packets, want 9 (one per keyframe, k = 0, 30, …, 240)", stapA)
			}
			t.Logf("%s: %d of %d frames matched; %d STAP-A", tc.name, matched, len(frames), stapA)
		})
	}
}
