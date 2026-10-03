package fingerprint

import (
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"github.com/Allan-Nava/whipbench"
	"github.com/Allan-Nava/whipbench/internal/clip"
)

func load(t *testing.T, codec string) *clip.Clip {
	t.Helper()
	data := whipbench.ClipVP8
	if codec == clip.H264 {
		data = whipbench.ClipH264
	}
	c, err := clip.Load(codec, data)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func prefix(b []byte) uint64 {
	sum := sha256.Sum256(b)
	return binary.BigEndian.Uint64(sum[:8])
}

func TestFingerprintVP8WholeFrame(t *testing.T) {
	// The SHA-256 test vector: proves SHA-256, the first 64 bits, big-endian.
	fp, ok := Of("vp8", []byte("abc"))
	if !ok || fp != 0xba7816bf8f01cfea {
		t.Fatalf(`Of("vp8", "abc") = %#x, %v; want 0xba7816bf8f01cfea, true`, fp, ok)
	}
	other, ok := Of("vp8", []byte("abd"))
	if !ok || other == fp {
		t.Fatalf(`Of("vp8", "abd") = %#x, %v; want a different value, true`, other, ok)
	}
}

func TestFingerprintH264VCLOnly(t *testing.T) {
	a := []byte{
		0, 0, 0, 1, 0x67, 0xAA,
		0, 0, 0, 1, 0x68, 0xBB,
		0, 0, 1, 0x06, 0xCC,
		0, 0, 0, 1, 0x65, 0xDD, 0xEE,
		0, 0, 1, 0x41, 0xFF,
	}
	b := []byte{0, 0, 1, 0x65, 0xDD, 0xEE, 0, 0, 0, 1, 0x41, 0xFF}
	fa, okA := Of("h264", a)
	fb, okB := Of("h264", b)
	if !okA || !okB || fa != fb {
		t.Fatalf("A = %#x, %v; B = %#x, %v; want equal, both ok", fa, okA, fb, okB)
	}
	if want := prefix([]byte{0x65, 0xDD, 0xEE, 0x41, 0xFF}); fb != want {
		t.Fatalf("B = %#x; want %#x, the SHA-256 prefix of the VCL bytes", fb, want)
	}

	// The NAL header byte is hashed.
	c := append([]byte(nil), b...)
	c[3] = 0x45
	if fc, ok := Of("h264", c); !ok || fc == fb {
		t.Fatalf("header 0x45 = %#x, %v; want a value other than %#x, true", fc, ok, fb)
	}

	// No VCL unit at all.
	if fp, ok := Of("h264", []byte{0, 0, 0, 1, 0x67, 0xAA, 0, 0, 0, 1, 0x68, 0xBB}); fp != 0 || ok {
		t.Fatalf("no VCL = %#x, %v; want 0, false", fp, ok)
	}

	// Every embedded clip frame, rebuilt from its VCL units alone, fingerprints the same.
	cl := load(t, clip.H264)
	for i, f := range cl.Frames {
		var vcl []byte
		for _, u := range clip.SplitAnnexB(f.Data) {
			if len(u) > 0 && isVCL(u[0]&0x1F) {
				vcl = append(vcl, 0, 0, 1)
				vcl = append(vcl, u...)
			}
		}
		want, okW := Of("h264", f.Data)
		got, okG := Of("h264", vcl)
		if !okW || !okG || got != want {
			t.Fatalf("frame %d: VCL-only %#x, %v; full %#x, %v", i, got, okG, want, okW)
		}
	}
}

func TestFingerprintStableAcrossProcesses(t *testing.T) {
	// Golden values on the embedded clips. A per-process seed, as hash/maphash has, would
	// break them; a regenerated clip (scripts/make-clips.sh) changes them, and then these
	// are updated with it.
	cases := []struct {
		codec string
		want  [2]uint64
	}{
		{clip.VP8, [2]uint64{0xcc7dc033284a903a, 0x624f3ffa2b4897e8}},
		{clip.H264, [2]uint64{0x5000846a47ceb02b, 0x25780bb4ae13506e}},
	}
	for _, tc := range cases {
		c := load(t, tc.codec)
		for i, want := range tc.want {
			if got, ok := Of(tc.codec, c.Frames[i].Data); !ok || got != want {
				t.Errorf("%s frame %d = %#x, %v; want %#x, true", tc.codec, i, got, ok, want)
			}
		}
	}
}

func TestFingerprintUnknownCodec(t *testing.T) {
	if fp, ok := Of("opus", []byte{1}); fp != 0 || ok {
		t.Fatalf(`Of("opus") = %#x, %v; want 0, false`, fp, ok)
	}
}

func TestFingerprintIsVCL(t *testing.T) {
	for _, n := range []byte{1, 2, 3, 4, 5} {
		if !isVCL(n) {
			t.Errorf("isVCL(%d) = false; want true", n)
		}
	}
	for _, n := range []byte{0, 6, 7, 8, 9, 24, 28} {
		if isVCL(n) {
			t.Errorf("isVCL(%d) = true; want false", n)
		}
	}
}

func TestTableEmbeddedClips(t *testing.T) {
	for _, codec := range []string{clip.VP8, clip.H264} {
		c := load(t, codec)
		tb, err := NewTable(c)
		if err != nil {
			t.Fatal(err)
		}
		if tb.Frames() != 120 || tb.Ticks() != 3000 || tb.Codec() != codec || tb.DuplicateFrames() != 0 {
			t.Fatalf("%s: Frames %d, Ticks %d, Codec %q, DuplicateFrames %d; want 120, 3000, %q, 0",
				codec, tb.Frames(), tb.Ticks(), tb.Codec(), tb.DuplicateFrames(), codec)
		}
		for i, f := range c.Frames {
			fp, _ := Of(codec, f.Data)
			if gi, st := tb.Lookup(fp); gi != i || st != Unique {
				t.Fatalf("%s frame %d: Lookup = %d, %v; want %d, Unique", codec, i, gi, st, i)
			}
		}
	}
}

func TestTableDuplicates(t *testing.T) {
	c := &clip.Clip{Codec: "vp8", Ticks: 3000}
	for _, d := range []string{"f0", "f1", "f2", "f1", "f4"} {
		c.Frames = append(c.Frames, clip.Frame{Data: []byte(d)})
	}
	tb, err := NewTable(c)
	if err != nil {
		t.Fatal(err)
	}
	if n := tb.DuplicateFrames(); n != 2 {
		t.Fatalf("DuplicateFrames = %d; want 2", n)
	}
	f1, _ := Of("vp8", []byte("f1"))
	if i, st := tb.Lookup(f1); i != -1 || st != Duplicate {
		t.Fatalf(`Lookup("f1") = %d, %v; want -1, Duplicate`, i, st)
	}
	f4, _ := Of("vp8", []byte("f4"))
	if i, st := tb.Lookup(f4); i != 4 || st != Unique {
		t.Fatalf(`Lookup("f4") = %d, %v; want 4, Unique`, i, st)
	}
	if i, st := tb.Lookup(42); i != -1 || st != Unknown {
		t.Fatalf("Lookup(42) = %d, %v; want -1, Unknown", i, st)
	}
}

func TestTableErrors(t *testing.T) {
	if _, err := NewTable(nil); err == nil {
		t.Error("NewTable(nil): no error")
	}
	if _, err := NewTable(&clip.Clip{Codec: "vp8", Ticks: 3000}); err == nil {
		t.Error("NewTable(no frames): no error")
	}
	if _, err := NewTable(&clip.Clip{Codec: "vp8", Frames: []clip.Frame{{Data: []byte{1}}}}); err == nil || err.Error() != "fingerprint: the clip has no frame duration (Ticks is 0)" {
		t.Errorf("NewTable(Ticks 0) = %v; want the frame-duration error", err)
	}
	bad := &clip.Clip{Codec: "h264", Ticks: 3000, Frames: []clip.Frame{{Data: []byte{0, 0, 1, 0x67, 0xAA}}}}
	_, err := NewTable(bad)
	if err == nil {
		t.Fatal("NewTable(no VCL frame): no error")
	}
	if want := "fingerprint: clip frame 0 has nothing to hash"; err.Error() != want {
		t.Fatalf("NewTable(no VCL frame) = %q; want %q", err, want)
	}
}
