package fingerprint

import (
	"sync"
	"testing"
	"time"
)

// base is the origin every test time is measured from; it carries a monotonic reading.
var base = time.Now()

// at is base plus ms milliseconds.
func at(ms int) time.Time {
	return base.Add(time.Duration(ms) * time.Millisecond)
}

// logAt returns a log of n frames per loop with k = from..to recorded at 33·k ms.
func logAt(n int, from, to uint64) *SendLog {
	l := NewSendLog(n)
	recordAt(l, from, to)
	return l
}

func recordAt(l *SendLog, from, to uint64) {
	for k := from; k <= to; k++ {
		l.Record(k, at(33*int(k))) //nolint:gosec // test indices are tiny
	}
}

func wantLatest(t *testing.T, l *SendLog, i int, t1 time.Time, want uint64) {
	t.Helper()
	k, t0, ok := l.Latest(i, t1)
	if !ok {
		t.Fatalf("Latest(%d, %v): not found, want k %d", i, t1.Sub(base), want)
	}
	if k != want {
		t.Fatalf("Latest(%d, %v): k %d, want %d", i, t1.Sub(base), k, want)
	}
	if !t0.Equal(at(33 * int(want))) { //nolint:gosec // test indices are tiny
		t.Fatalf("Latest(%d, …): t0 %v, want %v", i, t0.Sub(base), at(33*int(want)).Sub(base)) //nolint:gosec // idem
	}
}

func wantNone(t *testing.T, l *SendLog, i int, t1 time.Time) {
	t.Helper()
	if k, _, ok := l.Latest(i, t1); ok {
		t.Fatalf("Latest(%d, %v): k %d, want not found", i, t1.Sub(base), k)
	}
}

func TestSendLogLatest(t *testing.T) {
	l := logAt(3, 0, 8)
	wantLatest(t, l, 1, at(140), 4)
	wantLatest(t, l, 1, at(300), 7)
	wantLatest(t, l, 2, at(70), 2)
	wantLatest(t, l, 0, at(0), 0) // an equal t0 counts
}

func TestSendLogNeverReturnsAnUnloggedSend(t *testing.T) {
	l := logAt(3, 0, 8)
	wantNone(t, l, 2, at(65)) // k 2 is sent at 66 ms, nothing earlier for i 2
}

func TestSendLogRingOverwrites(t *testing.T) {
	l := logAt(3, 0, 14) // 12 slots: k 0..2 overwritten by k 12..14
	wantNone(t, l, 0, at(10))
	wantLatest(t, l, 0, at(100), 3)
	wantLatest(t, l, 0, at(1000), 12)
}

func TestSendLogBadIndex(t *testing.T) {
	l := logAt(3, 0, 0)
	wantNone(t, l, -1, at(1000))
	wantNone(t, l, 3, at(1000))
	wantNone(t, NewSendLog(3), 0, at(1000))
}

func TestSendLogNilRecordIsANoOp(t *testing.T) {
	var l *SendLog
	l.Record(0, time.Now())
}

func TestSendLogNewPanicsOnZero(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewSendLog(0) did not panic")
		}
	}()
	NewSendLog(0)
}

func wantLoopMin(t *testing.T, l *SendLog, want time.Duration, wantOK bool) {
	t.Helper()
	d, ok := l.LoopMin()
	if d != want || ok != wantOK {
		t.Fatalf("LoopMin() = %v, %v; want %v, %v", d, ok, want, wantOK)
	}
}

func TestSendLogLoopMin(t *testing.T) {
	l := NewSendLog(4)
	for k := 0; k <= 3; k++ {
		l.Record(uint64(k), at(10*k)) //nolint:gosec // test indices are tiny
	}
	wantLoopMin(t, l, 0, false)
	l.Record(4, at(50))
	wantLoopMin(t, l, 50*time.Millisecond, true)
	l.Record(5, at(55))
	wantLoopMin(t, l, 45*time.Millisecond, true)
}

func TestSendLogSlip(t *testing.T) {
	l := NewSendLog(4)
	for k := 0; k <= 11; k++ {
		ms := 10 * k
		if k >= 6 {
			ms += 600
		}
		l.Record(uint64(k), at(ms)) //nolint:gosec // test indices are tiny
	}
	if got := l.Recorded(); got != 12 {
		t.Fatalf("Recorded() = %d, want 12", got)
	}
	// The loops that span the slip take 640 ms; those that do not take 40 ms.
	wantLoopMin(t, l, 40*time.Millisecond, true)
	// The index follows k, not time: k 10 was sent at 700 ms.
	k, t0, ok := l.Latest(2, at(700))
	if !ok || k != 10 || !t0.Equal(at(700)) {
		t.Fatalf("Latest(2, 700 ms) = %d, %v, %v; want 10, 700ms, true", k, t0.Sub(base), ok)
	}
}

func TestSendLogConcurrent(t *testing.T) {
	const n = 120
	l := NewSendLog(n)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for k := uint64(0); k < 2400; k++ {
			l.Record(k, time.Now())
		}
	}()
	errs := make(chan string, 8)
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5000; j++ {
				i := j % n
				t1 := time.Now()
				k, t0, ok := l.Latest(i, t1)
				if !ok {
					continue
				}
				if k%n != uint64(i) || t0.After(t1) { //nolint:gosec // i is in [0, n)
					errs <- "a lookup returned a send of another index or after t1"
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
}
