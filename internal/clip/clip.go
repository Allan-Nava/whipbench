// Package clip turns a pre-encoded elementary stream into a list of frames the
// publisher can loop, and gives each frame of the loop its RTP timestamp.
//
// Nothing is decoded or encoded: the frames go on the wire byte for byte as the
// encoder wrote them. That is what lets one machine publish without a codec, and it
// is also the clip's main limit — a pre-encoded stream cannot answer a PLI or FIR
// with a fresh keyframe, so a viewer that joins waits for the next keyframe in the
// loop (at most one GOP, 1 s for the clips in testdata/).
package clip

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

// ClockRate is the RTP clock of every video codec WebRTC carries (RFC 7741, RFC 6184).
const ClockRate = 90000

// Codec names, as the CLI and the scenario file spell them.
const (
	VP8  = "vp8"
	H264 = "h264"
)

// Frame is one encoded picture: a VP8 frame, or an H.264 access unit in Annex-B
// form (start codes included, so the payloader can split it into NAL units).
type Frame struct {
	Data []byte
	Key  bool
}

// Clip is a loopable sequence of frames at a constant frame rate.
type Clip struct {
	Codec string
	// Ticks is the frame duration in 90 kHz RTP clock ticks (3000 at 30 fps).
	Ticks  uint32
	Frames []Frame
}

// FrameDuration is the wall-clock time between two frames.
func (c *Clip) FrameDuration() time.Duration {
	return time.Duration(c.Ticks) * time.Second / ClockRate
}

// Duration is one pass of the loop.
func (c *Clip) Duration() time.Duration {
	return time.Duration(len(c.Frames)) * c.FrameDuration()
}

// Frame returns the k-th frame of the endless loop (k counts from 0 across loops).
func (c *Clip) Frame(k uint64) Frame {
	return c.Frames[k%uint64(len(c.Frames))]
}

// Timestamp is the RTP timestamp of the k-th frame of the loop, from base.
//
// The loop is seamless on the wire: frame k is always k frame durations after frame
// 0, whichever pass it belongs to, so a viewer sees one continuous stream and its
// jitter and keyframe-interval maths never meet a discontinuity at the loop point.
// The uint32 arithmetic wraps exactly as RTP timestamps do (RFC 3550 §5.1).
func (c *Clip) Timestamp(base uint32, k uint64) uint32 {
	return base + uint32(k)*c.Ticks //nolint:gosec // wrap-around is the RTP semantics
}

// KeyframeInterval is the spacing of keyframes in the clip, in frames, or 0 when it
// is not constant. A clip whose length is not a multiple of it would show a short
// GOP at every loop point.
func (c *Clip) KeyframeInterval() int {
	prev, gap := -1, 0
	for i, f := range c.Frames {
		if !f.Key {
			continue
		}
		if prev >= 0 {
			if gap != 0 && i-prev != gap {
				return 0
			}
			gap = i - prev
		}
		prev = i
	}
	return gap
}

func (c *Clip) validate() error {
	if len(c.Frames) == 0 {
		return errors.New("clip has no frames")
	}
	if !c.Frames[0].Key {
		return errors.New("clip must start with a keyframe, or the first pass of the loop is undecodable")
	}
	if c.Ticks == 0 {
		return errors.New("clip frame duration is zero")
	}
	return nil
}

// ParseIVF reads a VP8 IVF file: a 32-byte header ("DKIF", version, header size,
// FourCC, width, height, timebase rate and scale, frame count), then frames of a
// 12-byte header (size, 64-bit pts in timebase units) and the payload. It is parsed
// here rather than with pion's ivfreader because the frame duration is derived from
// the pts, and the derivation has to be the one this comment states: one pts unit
// is scale/rate seconds. The pts must be evenly spaced.
func ParseIVF(data []byte) (*Clip, error) {
	if len(data) < 32 || string(data[0:4]) != "DKIF" {
		return nil, errors.New("ivf: not an IVF file")
	}
	if string(data[8:12]) != "VP80" {
		return nil, fmt.Errorf("ivf: FourCC %q, want VP80", data[8:12])
	}
	hdrSize := int(binary.LittleEndian.Uint16(data[6:8]))
	rate := uint64(binary.LittleEndian.Uint32(data[16:20]))
	scale := uint64(binary.LittleEndian.Uint32(data[20:24]))
	if rate == 0 || scale == 0 || hdrSize < 32 || hdrSize > len(data) {
		return nil, errors.New("ivf: bad header")
	}
	c := &Clip{Codec: VP8}
	var pts []uint64
	for off := hdrSize; off < len(data); {
		if off+12 > len(data) {
			return nil, fmt.Errorf("ivf frame %d: truncated header", len(c.Frames))
		}
		size := int(binary.LittleEndian.Uint32(data[off : off+4]))
		p := binary.LittleEndian.Uint64(data[off+4 : off+12])
		off += 12
		if size == 0 || off+size > len(data) {
			return nil, fmt.Errorf("ivf frame %d: bad size %d", len(c.Frames), size)
		}
		frame := data[off : off+size]
		off += size
		// VP8 frame tag, RFC 6386 §9.1: bit 0 of the first byte is 0 for a key frame.
		c.Frames = append(c.Frames, Frame{Data: frame, Key: frame[0]&0x01 == 0})
		pts = append(pts, p)
	}
	if len(pts) < 2 {
		return nil, errors.New("ivf: need at least two frames to know the frame rate")
	}
	step := pts[1] - pts[0]
	for i := 2; i < len(pts); i++ {
		if pts[i]-pts[i-1] != step {
			return nil, fmt.Errorf("ivf: frame %d is not evenly spaced (constant frame rate required)", i)
		}
	}
	// step units of scale/rate seconds = step*scale*90000/rate ticks.
	ticks := step * scale * ClockRate / rate
	if ticks == 0 || ticks > 1<<31 || step*scale*ClockRate%rate != 0 {
		return nil, fmt.Errorf("ivf: frame duration is not a whole number of 90 kHz ticks")
	}
	c.Ticks = uint32(ticks)
	return c, c.validate()
}

// ParseH264 reads an H.264 Annex-B elementary stream. The stream has no container
// timing: at fps 0 the frame rate is the one its first SPS declares in the VUI
// (sps.go), and a positive fps overrides it. Access units are delimited the way
// §7.4.1.2.3 of H.264 says: a non-VCL NAL (AUD, SPS, PPS, SEI) after a slice, or a
// slice whose first_mb_in_slice is 0, starts a new picture.
func ParseH264(data []byte, fps int) (*Clip, error) {
	nals := SplitAnnexB(data)
	var ticks uint32
	switch {
	case fps > 0:
		if ClockRate%fps != 0 {
			return nil, fmt.Errorf("h264: frame rate %d does not divide the 90 kHz clock", fps)
		}
		ticks = uint32(ClockRate / fps) //nolint:gosec // fps > 0
	case fps == 0:
		t, err := spsTicks(nals)
		if err != nil {
			return nil, err
		}
		ticks = t
	default:
		return nil, fmt.Errorf("h264: frame rate %d is not a frame rate", fps)
	}
	c := &Clip{Codec: H264, Ticks: ticks}
	var cur []byte
	curVCL, curKey := false, false
	flush := func() {
		if curVCL {
			c.Frames = append(c.Frames, Frame{Data: cur, Key: curKey})
		}
		cur, curVCL, curKey = nil, false, false
	}
	for _, nal := range nals {
		if len(nal) == 0 {
			continue
		}
		t := nal[0] & 0x1F
		switch {
		case t >= 6 && t <= 9: // SEI, SPS, PPS, AUD
			if curVCL {
				flush()
			}
		case t == 1 || t == 5:
			// first_mb_in_slice is ue(v): it is 0 exactly when its first bit is 1.
			if curVCL && len(nal) > 1 && nal[1]&0x80 != 0 {
				flush()
			}
			curVCL = true
			if t == 5 {
				curKey = true
			}
		}
		cur = append(cur, 0, 0, 0, 1)
		cur = append(cur, nal...)
	}
	flush()
	return c, c.validate()
}

// SplitAnnexB returns the NAL units of an Annex-B stream, start codes removed.
func SplitAnnexB(b []byte) [][]byte {
	var out [][]byte
	start := -1
	i := 0
	for i+2 < len(b) {
		if b[i] == 0 && b[i+1] == 0 && b[i+2] == 1 {
			if start >= 0 {
				end := i
				for end > start && b[end-1] == 0 { // trailing zero of a 4-byte start code
					end--
				}
				out = append(out, b[start:end])
			}
			i += 3
			start = i
			continue
		}
		i++
	}
	if start >= 0 && start < len(b) {
		out = append(out, b[start:])
	}
	return out
}

// Load parses an embedded or on-disk clip by codec, at the rate the clip declares: the
// IVF header's time base, or an H.264 stream's SPS.
func Load(codec string, data []byte) (*Clip, error) {
	return LoadAt(codec, data, 0)
}

// LoadAt is Load with an H.264 frame rate that overrides the SPS's, or 0 to use it. An
// IVF file states its own rate, so fps applies to H.264 alone.
func LoadAt(codec string, data []byte, fps int) (*Clip, error) {
	if fps != 0 && codec != H264 {
		return nil, fmt.Errorf("a frame rate applies to an h264 clip; a %s clip states its own", codec)
	}
	switch codec {
	case VP8:
		return ParseIVF(data)
	case H264:
		return ParseH264(data, fps)
	default:
		return nil, fmt.Errorf("unknown codec %q (want %s or %s)", codec, VP8, H264)
	}
}
