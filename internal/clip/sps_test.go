package clip

import (
	"errors"
	"testing"
)

// writer builds an SPS bit by bit, the inverse of bits, so each test states the fields
// it sets and the parse is checked against a stream nothing else wrote.
type writer struct {
	b   []byte
	pos int
}

func (w *writer) u(n int, v uint32) {
	for i := n - 1; i >= 0; i-- {
		if w.pos%8 == 0 {
			w.b = append(w.b, 0)
		}
		w.b[len(w.b)-1] |= byte(v>>i&1) << (7 - w.pos%8)
		w.pos++
	}
}

func (w *writer) ue(v uint32) {
	x := uint64(v) + 1
	n := 0
	for y := x; y > 1; y >>= 1 {
		n++
	}
	w.u(n, 0)
	w.u(n+1, uint32(x)) //nolint:gosec // x <= 2^32
}

func (w *writer) se(v int32) {
	if v > 0 {
		w.ue(uint32(2*v - 1)) //nolint:gosec // test values are small
	} else {
		w.ue(uint32(-2 * v)) //nolint:gosec // test values are small
	}
}

// escape inserts the emulation prevention byte an encoder writes before 00, 01, 02, 03
// after two zero bytes.
func escape(b []byte) []byte {
	var out []byte
	zeros := 0
	for _, c := range b {
		if zeros >= 2 && c <= 3 {
			out = append(out, 3)
			zeros = 0
		}
		if c == 0 {
			zeros++
		} else {
			zeros = 0
		}
		out = append(out, c)
	}
	return out
}

type spsOpts struct {
	profile       uint32
	scaling       bool // a scaling matrix with one list present (high profiles)
	poc           uint32
	vui, timing   bool
	sar255        bool // Extended_SAR: 32 more bits before timing_info
	units, scale  uint32
	frameCropping bool
}

func buildSPS(o spsOpts) []byte {
	w := &writer{}
	w.u(8, o.profile)
	w.u(8, 0)
	w.u(8, 30)
	w.ue(0)
	if o.profile == 100 {
		w.ue(1) // chroma_format_idc 4:2:0
		w.ue(0)
		w.ue(0)
		w.u(1, 0)
		if o.scaling {
			w.u(1, 1)
			for i := 0; i < 8; i++ {
				if i == 0 {
					w.u(1, 1)
					for j := 0; j < 16; j++ {
						w.se(1)
					}
				} else {
					w.u(1, 0)
				}
			}
		} else {
			w.u(1, 0)
		}
	}
	w.ue(0) // log2_max_frame_num_minus4
	w.ue(o.poc)
	switch o.poc {
	case 0:
		w.ue(2)
	case 1:
		w.u(1, 0)
		w.se(-3)
		w.se(2)
		w.ue(2)
		w.se(1)
		w.se(-1)
	}
	w.ue(1)
	w.u(1, 0)
	w.ue(39) // 640 px
	w.ue(22) // 368 px
	w.u(1, 1)
	w.u(1, 1)
	if o.frameCropping {
		w.u(1, 1)
		w.ue(0)
		w.ue(0)
		w.ue(0)
		w.ue(4)
	} else {
		w.u(1, 0)
	}
	if !o.vui {
		w.u(1, 0)
		w.u(1, 1) // rbsp_stop_one_bit
		return escape(w.b)
	}
	w.u(1, 1)
	if o.sar255 {
		w.u(1, 1)
		w.u(8, 255)
		w.u(16, 1)
		w.u(16, 1)
	} else {
		w.u(1, 0)
	}
	w.u(1, 0)
	w.u(1, 1) // video_signal_type
	w.u(3, 5)
	w.u(1, 0)
	w.u(1, 1)
	w.u(24, 0x010101)
	w.u(1, 0)
	if !o.timing {
		w.u(1, 0)
	} else {
		w.u(1, 1)
		w.u(32, o.units)
		w.u(32, o.scale)
		w.u(1, 1)
	}
	w.u(1, 1)
	return escape(w.b)
}

func TestSPSTiming(t *testing.T) {
	for _, tc := range []struct {
		name  string
		o     spsOpts
		ticks uint32
	}{
		{"baseline 30 fps", spsOpts{profile: 66, poc: 2, vui: true, timing: true, units: 1, scale: 60}, 3000},
		{"high, scaling matrix, 25 fps", spsOpts{profile: 100, scaling: true, poc: 0, vui: true, timing: true, units: 1, scale: 50}, 3600},
		{"poc type 1, cropping, Extended_SAR, 29.97 fps", spsOpts{profile: 77, poc: 1, frameCropping: true, sar255: true, vui: true, timing: true, units: 1001, scale: 60000}, 3003},
		{"60 fps", spsOpts{profile: 66, poc: 2, vui: true, timing: true, units: 1, scale: 120}, 1500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sps := buildSPS(tc.o)
			got, err := spsTicks([][]byte{{0x09, 0xf0}, append([]byte{0x67}, sps...)})
			if err != nil || got != tc.ticks {
				t.Fatalf("spsTicks = %d, %v; want %d", got, err, tc.ticks)
			}
		})
	}
}

func TestSPSTimingAbsentOrUnusable(t *testing.T) {
	for _, tc := range []struct {
		name string
		nals [][]byte
		want error
	}{
		{"no VUI", [][]byte{append([]byte{0x67}, buildSPS(spsOpts{profile: 66, poc: 2})...)}, errNoTiming},
		{"VUI without timing_info", [][]byte{append([]byte{0x67}, buildSPS(spsOpts{profile: 66, poc: 2, vui: true})...)}, errNoTiming},
		{"cut short", [][]byte{append([]byte{0x67}, buildSPS(spsOpts{profile: 66, poc: 2, vui: true, timing: true, units: 1, scale: 60})[:6]...)}, errShortSPS},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := spsTicks(tc.nals); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
	if _, err := spsTicks([][]byte{{0x65, 0x88}}); err == nil {
		t.Error("a stream without an SPS: no error")
	}
	// 7 fps is not a whole number of 90 kHz ticks per frame (12857.14…).
	if _, err := ticksOf(1, 14); err == nil {
		t.Error("7 fps: no error")
	}
	if _, err := ticksOf(0, 60); err == nil {
		t.Error("num_units_in_tick 0: no error")
	}
}

func TestUnescape(t *testing.T) {
	in := []byte{0x00, 0x00, 0x03, 0x01, 0x00, 0x00, 0x03, 0x00, 0x05}
	want := []byte{0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x05}
	if got := unescape(in); string(got) != string(want) {
		t.Fatalf("unescape = % x, want % x", got, want)
	}
	// A time_scale of 0x00000100 escapes inside the SPS; the parse must undo it.
	sps := buildSPS(spsOpts{profile: 66, poc: 2, vui: true, timing: true, units: 1, scale: 0x100})
	units, scale, err := spsTiming(sps)
	if err != nil || units != 1 || scale != 0x100 {
		t.Fatalf("spsTiming = %d/%d, %v; want 1/256", units, scale, err)
	}
}
