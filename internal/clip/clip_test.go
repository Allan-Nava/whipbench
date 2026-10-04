package clip_test

import (
	"testing"

	"github.com/Allan-Nava/whipbench"
	"github.com/Allan-Nava/whipbench/internal/clip"
)

func TestEmbeddedClips(t *testing.T) {
	for _, tc := range []struct {
		codec string
		data  []byte
	}{{clip.VP8, whipbench.ClipVP8}, {clip.H264, whipbench.ClipH264}} {
		c, err := clip.Load(tc.codec, tc.data)
		if err != nil {
			t.Fatalf("%s: %v", tc.codec, err)
		}
		if got := len(c.Frames); got != 120 {
			t.Errorf("%s: %d frames, want 120 (4 s at 30 fps)", tc.codec, got)
		}
		if c.Ticks != 3000 {
			t.Errorf("%s: %d ticks per frame, want 3000", tc.codec, c.Ticks)
		}
		if got := c.KeyframeInterval(); got != 30 {
			t.Errorf("%s: keyframe every %d frames, want 30", tc.codec, got)
		}
		if len(c.Frames)%c.KeyframeInterval() != 0 {
			t.Errorf("%s: the loop does not close on a keyframe boundary", tc.codec)
		}
	}
}

func TestTimestampsContinueAcrossLoops(t *testing.T) {
	c := &clip.Clip{Ticks: 3000, Frames: make([]clip.Frame, 120)}
	base := uint32(4294960000) // close to the wrap
	for k := uint64(1); k < 500; k++ {
		d := c.Timestamp(base, k) - c.Timestamp(base, k-1)
		if d != 3000 {
			t.Fatalf("frame %d: step %d, want 3000 (loop point at 120 must be seamless)", k, d)
		}
	}
	c.Frames[0].Key = true
	if !c.Frame(120).Key || c.Frame(121).Key {
		t.Fatal("frame 120 must be frame 0 of the second pass")
	}
}

func TestSplitAnnexB(t *testing.T) {
	in := []byte{0, 0, 0, 1, 0x67, 1, 2, 0, 0, 1, 0x68, 3, 0, 0, 0, 1, 0x65, 4, 5}
	got := clip.SplitAnnexB(in)
	if len(got) != 3 || got[0][0] != 0x67 || len(got[0]) != 3 || got[1][0] != 0x68 || len(got[1]) != 2 || got[2][0] != 0x65 {
		t.Fatalf("split = %x", got)
	}
}

func TestParseH264AccessUnits(t *testing.T) {
	// SPS, PPS, IDR slice (first_mb 0), then a P slice (first_mb 0), then a second
	// slice of the same picture (first_mb != 0: first bit 0) — two pictures.
	in := []byte{
		0, 0, 0, 1, 0x67, 0x42,
		0, 0, 0, 1, 0x68, 0xce,
		0, 0, 0, 1, 0x65, 0x88,
		0, 0, 0, 1, 0x41, 0x9a,
		0, 0, 0, 1, 0x41, 0x40,
	}
	c, err := clip.ParseH264(in, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Frames) != 2 || !c.Frames[0].Key || c.Frames[1].Key {
		t.Fatalf("frames = %+v", c.Frames)
	}
}

func TestRejectsClipWithoutLeadingKeyframe(t *testing.T) {
	if _, err := clip.ParseH264([]byte{0, 0, 0, 1, 0x41, 0x9a}, 30); err == nil {
		t.Fatal("a clip that starts on a P frame must be rejected")
	}
}

func TestLoadAtRate(t *testing.T) {
	c, err := clip.LoadAt(clip.H264, whipbench.ClipH264, 25)
	if err != nil || c.Ticks != 3600 {
		t.Fatalf("h264 at 25 fps: %v ticks, %v; want 3600 — the flag overrides the SPS", c, err)
	}
	if _, err := clip.LoadAt(clip.VP8, whipbench.ClipVP8, 25); err == nil {
		t.Error("vp8 with a frame rate: no error — an IVF file states its own")
	}
	if _, err := clip.LoadAt(clip.H264, whipbench.ClipH264, 7); err == nil {
		t.Error("h264 at 7 fps: no error — it does not divide the 90 kHz clock")
	}
	// An Annex-B stream without an SPS has no rate of its own: it needs the flag.
	idr := []byte{0, 0, 0, 1, 0x65, 0x88, 0x84}
	if _, err := clip.ParseH264(idr, 0); err == nil {
		t.Error("no SPS, no fps: no error")
	}
	if c, err := clip.ParseH264(idr, 30); err != nil || c.Ticks != 3000 {
		t.Errorf("no SPS, fps 30: %v, %v", c, err)
	}
}
