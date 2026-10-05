package clocksync

import "time"

// Model is a run's points, in the order they were measured, and the offset between them:
// piecewise-linear from one point to the next, so a clock that slews between two points is
// followed rather than averaged, and held at the first or last point outside them.
type Model struct {
	Points []Point
}

// OffsetAt is the responder's clock minus the client's at t, a client reading. With no
// points it is zero; callers check len(Points) first, as a missing offset is never a 0.
func (m Model) OffsetAt(t time.Time) time.Duration {
	ps := m.Points
	if len(ps) == 0 {
		return 0
	}
	if !t.After(ps[0].At) {
		return ps[0].Offset
	}
	for i := 1; i < len(ps); i++ {
		a, b := ps[i-1], ps[i]
		if t.After(b.At) {
			continue
		}
		span := b.At.Sub(a.At)
		if span <= 0 {
			return b.Offset
		}
		frac := float64(t.Sub(a.At)) / float64(span)
		return a.Offset + time.Duration(frac*float64(b.Offset-a.Offset))
	}
	return ps[len(ps)-1].Offset
}

// Uncertainty is the largest RTT/2 over the points: each offset is known to within half
// its round trip, and the interpolation between two points is no better than the worse.
func (m Model) Uncertainty() time.Duration {
	var u time.Duration
	for _, p := range m.Points {
		u = max(u, p.RTT/2)
	}
	return u
}
