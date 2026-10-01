// Package version carries the whipbench version, stamped into every report.
package version

import "runtime/debug"

// Version is set at build time by the release workflow:
//
//	-ldflags "-X github.com/Allan-Nava/whipbench/internal/version.Version=v0.0.1"
//
// A plain `go build` leaves it at "dev"; `go install …@v0.0.1` recovers the module
// version from the build info instead.
var Version = "dev"

// String returns the version a report records. A development build names the
// commit it was built from, and says when the tree had uncommitted changes, so a
// result in evals/ can be traced to the code that produced it.
func String() string {
	if Version != "dev" {
		return Version
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return Version
	}
	if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	rev, dirty := "", false
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if len(rev) >= 12 {
		v := Version + "+" + rev[:12]
		if dirty {
			v += ".dirty"
		}
		return v
	}
	return Version
}
