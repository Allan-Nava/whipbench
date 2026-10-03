// Package fingerprint identifies a received video frame by its content, so a viewer
// can pair it with the moment the publisher sent it and measure one-way delay.
//
// One-way delay is the time from a frame's first-packet send to last-packet arrival
// (t1 − t0): t0 is when the publisher hands the frame's first RTP packet to the stack,
// t1 is when the viewer receives the last packet of the frame. The two clocks are the
// same process's, so no clock offset enters the number.
//
// The publisher logs t0 per absolute frame index k — k counts every frame sent since
// the stream started, across loops of the clip — before the frame's first packet is
// written, so the logged time never trails the packet on the wire.
//
// The fingerprint of a frame is the first 64 bits, big-endian, of the SHA-256 of its
// hashed bytes: the whole frame for VP8; for H.264, the VCL NAL units only (types 1-5,
// header byte included, start codes removed, in order), because a relay may add, drop or
// repeat parameter sets and SEI around an unchanged picture. SHA-256 rather than a
// seeded hash keeps a fingerprint the same in every process.
//
// A fingerprint shared by two or more clip frames cannot say which frame arrived, so it
// is never sampled: the frame was sent, but contributes no delay.
//
// A match is the latest send of the frame's clip index i = k mod N (N the clip's frame
// count) at or before t1. A match is invalid when the RTP timestamps prove it whole loops
// too new — the received frame belongs to an earlier loop than the send found — or when
// the log holds no send of that index at or before t1.
package fingerprint

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/Allan-Nava/whipbench/internal/clip"
)

// Of returns the fingerprint of one frame of the given codec (clip.VP8 or clip.H264):
// the first 64 bits, big-endian, of the SHA-256 of its hashed bytes. ok is false for an
// unknown codec and for an H.264 frame with no VCL NAL unit.
func Of(codec string, frame []byte) (fp uint64, ok bool) {
	h := sha256.New()
	switch codec {
	case clip.VP8:
		h.Write(frame)
	case clip.H264:
		kept := false
		for _, unit := range clip.SplitAnnexB(frame) {
			if len(unit) == 0 || !isVCL(unit[0]&0x1F) {
				continue
			}
			h.Write(unit)
			kept = true
		}
		if !kept {
			return 0, false
		}
	default:
		return 0, false
	}
	var sum [sha256.Size]byte
	h.Sum(sum[:0])
	return binary.BigEndian.Uint64(sum[:8]), true
}

// isVCL says which H.264 NAL unit types carry the coded picture (D1). It is the one
// line H1's check against H.264 Table 7-1 may change.
func isVCL(nalType byte) bool { return nalType >= 1 && nalType <= 5 }

// Status says what a fingerprint is in the clip.
type Status int

const (
	Unknown   Status = iota // in no clip frame: the frame counts as unmatched
	Unique                  // in exactly one clip frame: matched against the send log
	Duplicate               // in two or more clip frames: sent, never sampled (Q7)
)

// Table maps every clip frame's fingerprint to its index i in the loop. It is built
// once per run and only read afterwards, so every viewer shares one without a lock.
type Table struct {
	codec     string
	ticks     uint32
	frames    int
	index     map[uint64]int
	dups      map[uint64]struct{}
	dupFrames int
}

// NewTable fingerprints every frame of c. It fails when the clip has no frames or a
// frame has nothing to hash, because a table that silently skipped a frame would turn
// that frame's every arrival into an unmatched one.
func NewTable(c *clip.Clip) (*Table, error) {
	if c == nil || len(c.Frames) == 0 {
		return nil, errors.New("fingerprint: the clip has no frames")
	}
	// A matcher divides by Ticks (match.go); clip.Load rejects 0 already, a hand-built clip may not.
	if c.Ticks == 0 {
		return nil, errors.New("fingerprint: the clip has no frame duration (Ticks is 0)")
	}
	fps := make([]uint64, len(c.Frames))
	seen := make(map[uint64]int, len(c.Frames))
	for i, f := range c.Frames {
		fp, ok := Of(c.Codec, f.Data)
		if !ok {
			return nil, fmt.Errorf("fingerprint: clip frame %d has nothing to hash", i)
		}
		fps[i] = fp
		seen[fp]++
	}
	t := &Table{
		codec:  c.Codec,
		ticks:  c.Ticks,
		frames: len(c.Frames),
		index:  make(map[uint64]int, len(c.Frames)),
		dups:   make(map[uint64]struct{}),
	}
	for i, fp := range fps {
		if seen[fp] > 1 {
			t.dups[fp] = struct{}{}
			t.dupFrames++
			continue
		}
		t.index[fp] = i
	}
	return t, nil
}

// Lookup says where fp sits in the clip: its index i and Unique, or -1 and Duplicate
// when several clip frames share it, or -1 and Unknown when none has it.
func (t *Table) Lookup(fp uint64) (i int, st Status) {
	if i, ok := t.index[fp]; ok {
		return i, Unique
	}
	if _, ok := t.dups[fp]; ok {
		return -1, Duplicate
	}
	return -1, Unknown
}

// Frames is N, the clip's frame count: the modulus that turns an absolute frame index
// k into the clip index i.
func (t *Table) Frames() int { return t.frames }

// Ticks is the clip's RTP timestamp step per frame, which lets a matcher tell from the
// timestamps how many whole loops apart two frames are.
func (t *Table) Ticks() uint32 { return t.ticks }

// Codec is the clip's codec, clip.VP8 or clip.H264, which a viewer checks against its
// track's before it fingerprints anything.
func (t *Table) Codec() string { return t.codec }

// DuplicateFrames counts the clip frames whose fingerprint is not unique: frames that
// are sent but never sampled, reported so a thin sample is explained.
func (t *Table) DuplicateFrames() int { return t.dupFrames }
