// Package procstat reads what the whipbench process itself is spending — CPU time,
// goroutines, heap — so that a saturated client is visible next to the server's figures
// (WB-17). It is the standard library only, and every reading describes the whole
// process: publisher, viewers and reassembly together, never the server.
//
// Each function reads the current value when called; nothing samples in the background.
// /metrics calls them at scrape time, and the report's resource summary once a second.
package procstat

import (
	"runtime"
	rtmetrics "runtime/metrics"
	"time"
)

// heapInUse are the runtime/metrics classes whose sum is the heap in use — the bytes in
// spans holding objects, live or not yet swept, which is MemStats.HeapInuse. Reading them
// does not stop the world, which runtime.ReadMemStats does on every call.
var heapInUse = [...]string{"/memory/classes/heap/objects:bytes", "/memory/classes/heap/unused:bytes"}

// CPU returns the CPU time this process has used since it started, user plus system, and
// false where the platform gives no reading (CPUAvailable says which platforms do).
func CPU() (time.Duration, bool) { return cpu() }

// CPUAvailable reports whether CPU returns a reading on this platform: linux and darwin,
// through getrusage(2).
const CPUAvailable = cpuAvailable

// Goroutines returns the number of goroutines that exist now.
func Goroutines() int { return runtime.NumGoroutine() }

// HeapBytes returns the bytes of heap in use now.
func HeapBytes() uint64 {
	var s [len(heapInUse)]rtmetrics.Sample
	for i, name := range heapInUse {
		s[i].Name = name
	}
	rtmetrics.Read(s[:])
	var n uint64
	for _, v := range s {
		if v.Value.Kind() == rtmetrics.KindUint64 {
			n += v.Value.Uint64()
		}
	}
	return n
}
