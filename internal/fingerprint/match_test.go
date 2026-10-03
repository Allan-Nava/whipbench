package fingerprint

import (
	"testing"
	"time"
)

const (
	matchFrames = 10
	matchTicks  = 3000
)

// matchT is after every send in these tests, so Latest returns the highest recorded k ≡ i.
var matchT = at(100000)

// newMatch returns a matcher over its own log of 10 frames per loop, k = 0..39 recorded
// at 33·k ms.
func newMatch() (*Matcher, *SendLog) {
	l := logAt(matchFrames, 0, 39)
	return NewMatcher(l, matchTicks), l
}

// anchored is newMatch with the anchor (35, 1000) set by a first Match(5, 1000, T).
func anchored(t *testing.T) (*Matcher, *SendLog) {
	t.Helper()
	m, l := newMatch()
	if v, _ := m.Match(5, 1000, matchT); v != Sampled {
		t.Fatalf("anchoring match: verdict %d, want Sampled", v)
	}
	wantAnchor(t, m, 35, 1000)
	return m, l
}

func wantAnchor(t *testing.T, m *Matcher, k uint64, ts uint32) {
	t.Helper()
	if !m.have || m.kA != k || m.tsA != ts {
		t.Fatalf("anchor = (%v, %d, %d), want (true, %d, %d)", m.have, m.kA, m.tsA, k, ts)
	}
}

func wantVerdict(t *testing.T, m *Matcher, i int, ts uint32, t1 time.Time, want Verdict, wantD time.Duration) {
	t.Helper()
	v, d := m.Match(i, ts, t1)
	if v != want || d != wantD {
		t.Fatalf("Match(%d, %d, …) = %d, %v; want %d, %v", i, ts, v, d, want, wantD)
	}
}

// sample is the duration a Sampled match of frame k must report at T.
func sample(k int) time.Duration {
	return matchT.Sub(at(33 * k))
}

func TestMatchFirstIsKept(t *testing.T) {
	m, _ := newMatch()
	if d := sample(35); d != 98845*time.Millisecond {
		t.Fatalf("test arithmetic: T − t0[35] = %v, want 98.845s", d)
	}
	wantVerdict(t, m, 5, 1000, matchT, Sampled, sample(35))
	wantAnchor(t, m, 35, 1000)
}

func TestMatchAliased(t *testing.T) {
	m, l := anchored(t)
	recordAt(l, 40, 49)
	// kL 49 = 35 + 4 + 10: one whole loop too new.
	wantVerdict(t, m, 9, 1000+4*matchTicks, matchT, Aliased, 0)
	wantAnchor(t, m, 35, 1000)
	wantVerdict(t, m, 9, 1000+4*matchTicks, matchT, Aliased, 0)
	wantAnchor(t, m, 35, 1000)
}

func TestMatchTwoLoops(t *testing.T) {
	m, l := anchored(t)
	recordAt(l, 40, 59)
	// kL 57 = 35 + 2 + 20: two whole loops too new.
	wantVerdict(t, m, 7, 1000+2*matchTicks, matchT, Aliased, 0)
	wantAnchor(t, m, 35, 1000)
}

func TestMatchNoEvidence(t *testing.T) {
	m, _ := anchored(t)
	// 4500 is not a whole number of ticks: no evidence either way.
	wantVerdict(t, m, 6, 1000+4500, matchT, Sampled, sample(36))
	wantAnchor(t, m, 36, 5500)
}

func TestMatchNotAWholeLoop(t *testing.T) {
	m, _ := anchored(t)
	// kL 39, expected 38: diff 1 is not a whole loop.
	wantVerdict(t, m, 9, 1000+3*matchTicks, matchT, Sampled, sample(39))
	wantAnchor(t, m, 39, 10000)
}

func TestMatchTimestampWrap(t *testing.T) {
	w := uint32(0xFFFFF000)

	m, _ := newMatch()
	wantVerdict(t, m, 5, w, matchT, Sampled, sample(35))
	if w+6000 != 0x770 {
		t.Fatalf("test arithmetic: w+6000 = %#x, want 0x770", w+6000)
	}
	// Δ 2 across the wrap, kL 37, diff 0.
	wantVerdict(t, m, 7, w+6000, matchT, Sampled, sample(37))
	wantAnchor(t, m, 37, 0x770)

	m2, l2 := newMatch()
	wantVerdict(t, m2, 5, w, matchT, Sampled, sample(35))
	recordAt(l2, 40, 47)
	// kL 47, diff 10: one whole loop too new, across the wrap.
	wantVerdict(t, m2, 7, w+6000, matchT, Aliased, 0)
	wantAnchor(t, m2, 35, w)
}

func TestMatchNotLogged(t *testing.T) {
	m, _ := newMatch()
	// k ≡ 3 is first sent at 99 ms.
	wantVerdict(t, m, 3, 0, at(50), NotLogged, 0)
	if m.have {
		t.Fatal("a NotLogged match set the anchor")
	}
}

func TestMatchSampleIsMonotonic(t *testing.T) {
	l := NewSendLog(10)
	t0 := time.Now()
	l.Record(0, t0)
	m := NewMatcher(l, matchTicks)
	wantVerdict(t, m, 0, 0, t0.Add(5*time.Millisecond), Sampled, 5*time.Millisecond)
}
