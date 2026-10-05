package procstat

import (
	"testing"
	"time"
)

func TestCPUAdvancesWithWork(t *testing.T) {
	before, ok := CPU()
	if ok != CPUAvailable {
		t.Fatalf("CPU() ok = %v, CPUAvailable = %v", ok, CPUAvailable)
	}
	if !ok {
		t.Skip("no CPU reading on this platform")
	}
	x := uint64(1)
	for deadline := time.Now().Add(50 * time.Millisecond); time.Now().Before(deadline); {
		for range 1000 {
			x = x*6364136223846793005 + 1442695040888963407
		}
	}
	after, _ := CPU()
	if after <= before {
		t.Errorf("CPU did not advance over 50 ms of work: %v then %v (%d)", before, after, x)
	}
}

func TestGoroutinesAndHeap(t *testing.T) {
	n := Goroutines()
	stop := make(chan struct{})
	for range 10 {
		go func() { <-stop }()
	}
	if got := Goroutines(); got < n+10 {
		t.Errorf("goroutines %d after starting 10 more than %d", got, n)
	}
	close(stop)
	if HeapBytes() == 0 {
		t.Error("heap in use is 0 in a running process")
	}
}
