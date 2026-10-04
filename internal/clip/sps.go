package clip

import (
	"errors"
	"fmt"
)

// An Annex-B stream carries no container timing; its frame rate, when the encoder wrote
// one, is in the sequence parameter set's VUI (H.264 §7.3.2.1.1, Annex E.1.1):
// time_scale / num_units_in_tick is the rate of the clock ticks, and a frame lasts two
// of them (E.2.1, the field-based count). The parse below reads just far enough to
// reach timing_info, skipping every field before it.

// errNoTiming is a stream whose SPS carries no timing_info: its rate must be given.
var errNoTiming = errors.New("h264: the SPS carries no frame rate (VUI timing_info); give it with --fps")

// spsTicks is the frame duration in 90 kHz ticks that the first SPS in nals declares.
func spsTicks(nals [][]byte) (uint32, error) {
	for _, nal := range nals {
		if len(nal) > 1 && nal[0]&0x1F == 7 {
			units, scale, err := spsTiming(nal[1:])
			if err != nil {
				return 0, err
			}
			return ticksOf(units, scale)
		}
	}
	return 0, errors.New("h264: no SPS in the stream to read the frame rate from; give it with --fps")
}

// ticksOf turns num_units_in_tick and time_scale into whole 90 kHz ticks per frame:
// 2·units/scale seconds, so 60/1 is 3000 (30 fps) and 60000/1001 is 3003 (29.97).
func ticksOf(units, scale uint32) (uint32, error) {
	if units == 0 || scale == 0 {
		return 0, fmt.Errorf("h264: SPS timing %d/%d is not a frame rate; give it with --fps", scale, units)
	}
	num := uint64(ClockRate) * 2 * uint64(units)
	if num%uint64(scale) != 0 || num/uint64(scale) == 0 || num/uint64(scale) > 1<<32-1 {
		return 0, fmt.Errorf("h264: SPS frame rate %d/%d is not a whole number of 90 kHz ticks; give it with --fps", scale, 2*uint64(units))
	}
	return uint32(num / uint64(scale)), nil //nolint:gosec // bounded above
}

// spsTiming reads num_units_in_tick and time_scale from an SPS RBSP (the NAL header
// byte removed), or errNoTiming when the VUI or its timing_info is absent.
func spsTiming(payload []byte) (units, scale uint32, err error) {
	r := &bits{b: unescape(payload)}
	profile := r.u(8)
	r.u(8) // constraint flags, reserved bits
	r.u(8) // level_idc
	r.ue() // seq_parameter_set_id
	switch profile {
	case 100, 110, 122, 244, 44, 83, 86, 118, 128, 138, 139, 134, 135:
		chroma := r.ue()
		if chroma == 3 {
			r.u(1) // separate_colour_plane_flag
		}
		r.ue()           // bit_depth_luma_minus8
		r.ue()           // bit_depth_chroma_minus8
		r.u(1)           // qpprime_y_zero_transform_bypass_flag
		if r.u(1) == 1 { // seq_scaling_matrix_present_flag
			lists := 8
			if chroma == 3 {
				lists = 12
			}
			for i := 0; i < lists; i++ {
				if r.u(1) == 1 {
					size := 16
					if i >= 6 {
						size = 64
					}
					r.scalingList(size)
				}
			}
		}
	}
	r.ue()          // log2_max_frame_num_minus4
	switch r.ue() { // pic_order_cnt_type
	case 0:
		r.ue() // log2_max_pic_order_cnt_lsb_minus4
	case 1:
		r.u(1) // delta_pic_order_always_zero_flag
		r.se() // offset_for_non_ref_pic
		r.se() // offset_for_top_to_bottom_field
		n := r.ue()
		for i := uint32(0); i < n && r.err == nil; i++ {
			r.se()
		}
	}
	r.ue()           // max_num_ref_frames
	r.u(1)           // gaps_in_frame_num_value_allowed_flag
	r.ue()           // pic_width_in_mbs_minus1
	r.ue()           // pic_height_in_map_units_minus1
	if r.u(1) == 0 { // frame_mbs_only_flag
		r.u(1) // mb_adaptive_frame_field_flag
	}
	r.u(1)           // direct_8x8_inference_flag
	if r.u(1) == 1 { // frame_cropping_flag
		r.ue()
		r.ue()
		r.ue()
		r.ue()
	}
	if r.u(1) == 0 { // vui_parameters_present_flag
		return 0, 0, r.or(errNoTiming)
	}
	if r.u(1) == 1 { // aspect_ratio_info_present_flag
		if r.u(8) == 255 { // Extended_SAR
			r.u(16)
			r.u(16)
		}
	}
	if r.u(1) == 1 { // overscan_info_present_flag
		r.u(1)
	}
	if r.u(1) == 1 { // video_signal_type_present_flag
		r.u(3)
		r.u(1)
		if r.u(1) == 1 { // colour_description_present_flag
			r.u(24)
		}
	}
	if r.u(1) == 1 { // chroma_loc_info_present_flag
		r.ue()
		r.ue()
	}
	if r.u(1) == 0 { // timing_info_present_flag
		return 0, 0, r.or(errNoTiming)
	}
	units, scale = r.u(32), r.u(32)
	if r.err != nil {
		return 0, 0, r.err
	}
	return units, scale, nil
}

// unescape drops the emulation prevention byte of every 00 00 03 (§7.4.1).
func unescape(b []byte) []byte {
	out := make([]byte, 0, len(b))
	zeros := 0
	for _, c := range b {
		if zeros >= 2 && c == 3 {
			zeros = 0
			continue
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

// bits reads an RBSP most significant bit first. Reading past the end sets err once and
// returns zeros after, so a parse reads straight through and checks err at the end.
type bits struct {
	b   []byte
	pos int
	err error
}

var errShortSPS = errors.New("h264: the SPS ends before its timing_info")

func (r *bits) bit() uint32 {
	if r.pos >= len(r.b)*8 {
		if r.err == nil {
			r.err = errShortSPS
		}
		return 0
	}
	v := uint32(r.b[r.pos/8]>>(7-r.pos%8)) & 1
	r.pos++
	return v
}

func (r *bits) u(n int) uint32 {
	var v uint32
	for i := 0; i < n; i++ {
		v = v<<1 | r.bit()
	}
	return v
}

// ue is Exp-Golomb (§9.1): up to 31 leading zeros, as a 32-bit value can hold.
func (r *bits) ue() uint32 {
	zeros := 0
	for r.bit() == 0 {
		if r.err != nil {
			return 0
		}
		if zeros++; zeros > 31 {
			r.err = errors.New("h264: an Exp-Golomb code in the SPS is too long")
			return 0
		}
	}
	return (uint32(1)<<zeros - 1) + r.u(zeros)
}

func (r *bits) se() int32 {
	k := r.ue()
	if k%2 == 1 {
		return int32((k + 1) / 2) //nolint:gosec // k < 2^32
	}
	return -int32(k / 2) //nolint:gosec // k < 2^32
}

// scalingList skips one scaling_list() (§7.3.2.1.1.1): delta_scale is read while the
// next scale is not 0.
func (r *bits) scalingList(size int) {
	last, next := int32(8), int32(8)
	for j := 0; j < size && r.err == nil; j++ {
		if next != 0 {
			next = (last + r.se() + 256) % 256
		}
		if next != 0 {
			last = next
		}
	}
}

// or is err when nothing went wrong reading, the read error otherwise.
func (r *bits) or(err error) error {
	if r.err != nil {
		return r.err
	}
	return err
}
