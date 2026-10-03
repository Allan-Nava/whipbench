package fingerprint

import (
	"sync"
	"time"
)

// sendLogLoops is how many whole passes of the clip a SendLog remembers (D2). The ring is
// bounded so a long run costs no more memory than a short one; a send older than that
// can no longer be returned, so it can never be matched.
const sendLogLoops = 4

// SendLog is the publisher's send time t0 for each absolute frame index k, over the last
// four loops (D2). One goroutine records; any number look up. The ring is indexed by k,
// not by time, so a slip in the publisher's pacing moves t0 but never which slot a frame
// lands in, and a lookup by clip index stays exact across a slip.
type SendLog struct {
	mu       sync.RWMutex
	frames   uint64      // n, frames per loop of the clip
	k        []uint64    // k+1 held in each slot; 0 marks an empty slot
	t0       []time.Time // send time of the k in the same slot, monotonic reading kept
	next     uint64      // one past the highest k recorded
	loopMin  time.Duration
	haveLoop bool
}

// NewSendLog returns an empty log for a clip of frames frames per loop, with room for
// four loops. A clip always has frames, so frames ≤ 0 is a programming error and panics
// rather than returning an error every caller would have to ignore.
func NewSendLog(frames int) *SendLog {
	if frames <= 0 {
		panic("fingerprint: a send log needs at least one frame per loop")
	}
	n := uint64(frames)
	return &SendLog{
		frames: n,
		k:      make([]uint64, sendLogLoops*n),
		t0:     make([]time.Time, sendLogLoops*n),
	}
}

// Record logs that frame k was sent at t0. A nil log records nothing, so a publisher
// that keeps no log calls Record unconditionally and stays as it is. t0 is stored as
// given, monotonic reading included, so that a viewer's t1.Sub(t0) is immune to a
// wall-clock step. When k−n is still in the ring, the loop it closes updates LoopMin.
func (l *SendLog) Record(k uint64, t0 time.Time) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	size := uint64(len(l.k))
	s := k % size
	l.k[s] = k + 1
	l.t0[s] = t0
	if k >= l.frames {
		p := (k - l.frames) % size
		if l.k[p] == k-l.frames+1 {
			d := t0.Sub(l.t0[p])
			if !l.haveLoop || d < l.loopMin {
				l.loopMin = d
				l.haveLoop = true
			}
		}
	}
	if k+1 > l.next {
		l.next = k + 1
	}
}

// Latest returns the most recent send of clip index i that happened at or before t1:
// its absolute frame index k and its send time t0. A viewer cannot have received a
// frame before it was sent, so a later send of the same index is never a candidate,
// and a send that has rotated out of the ring is never returned. ok is false when i is
// not a clip index, nothing was recorded, or no logged send of i qualifies.
func (l *SendLog) Latest(i int, t1 time.Time) (k uint64, t0 time.Time, ok bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	n := int64(l.frames) //nolint:gosec // frames came from a positive int
	if i < 0 || int64(i) >= n || l.next == 0 {
		return 0, time.Time{}, false
	}
	h := int64(l.next - 1) //nolint:gosec // a frame index stays far below 2^63
	ii := int64(i)
	if h < ii {
		return 0, time.Time{}, false
	}
	size := uint64(len(l.k))
	oldest := h - int64(size) //nolint:gosec // the ring is a few hundred slots
	for c := h - (h-ii)%n; c >= ii && c > oldest; c -= n {
		ck := uint64(c) //nolint:gosec // c ≥ i ≥ 0 here
		s := ck % size
		if l.k[s] == ck+1 && !l.t0[s].After(t1) {
			return ck, l.t0[s], true
		}
	}
	return 0, time.Time{}, false
}

// Frames is n, the frames per loop the log was built for.
func (l *SendLog) Frames() int {
	return int(l.frames) //nolint:gosec // frames came from a positive int
}

// Recorded is one past the highest k recorded: the number of frames logged when the
// publisher logs every k from 0, as it does.
func (l *SendLog) Recorded() uint64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.next
}

// LoopMin is the shortest time recorded between a frame's send and the send of the same
// clip index one loop later. A slip lengthens only the loops that span it, so the minimum
// is the clip's own loop period as the publisher paced it. ok is false until one loop has
// closed.
func (l *SendLog) LoopMin() (time.Duration, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.loopMin, l.haveLoop
}
