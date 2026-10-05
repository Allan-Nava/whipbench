//go:build !linux && !darwin

package procstat

import "time"

// cpuAvailable is false: the release builds linux and darwin only, and reading process
// CPU elsewhere (GetProcessTimes on windows) would need golang.org/x/sys or a syscall
// table of its own. A caller reports the figure unavailable, never 0.
const cpuAvailable = false

func cpu() (time.Duration, bool) { return 0, false }
