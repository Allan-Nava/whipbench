//go:build linux || darwin

package procstat

import (
	"syscall"
	"time"
)

const cpuAvailable = true

// cpu is getrusage(RUSAGE_SELF): every thread of the process, user plus system time.
func cpu() (time.Duration, bool) {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0, false
	}
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano()), true
}
