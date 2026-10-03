package report

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Allan-Nava/whipbench/internal/stats"
	"github.com/Allan-Nava/whipbench/internal/viewer"
)

// Markdown renders the report for a person. An invalid run prints its verdict in
// place of the aggregate table: the numbers stay in the JSON, marked invalid, for
// anyone debugging the run, but the page meant for reading does not show them.
func (r *Report) Markdown() string {
	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }
	title := r.Scenario.Name
	if title == "" {
		title = "whipbench " + r.Command
	}
	w("# %s\n\n", esc(title))
	w("| | |\n|---|---|\n")
	w("| whipbench | %s |\n", esc(r.Version))
	w("| started | %s |\n", r.StartedAt.Format("2006-01-02 15:04:05Z"))
	w("| duration | %.1f s |\n", r.FinishedAt.Sub(r.StartedAt).Seconds())
	if r.Server.WHIPHost != "" {
		w("| WHIP host | `%s` |\n", esc(r.Server.WHIPHost))
	}
	w("| WHEP host | `%s` |\n", esc(r.Server.WHEPHost))
	s := r.Scenario
	offset := ""
	if s.RampOffsetSeed != nil {
		offset = fmt.Sprintf(", ramp offset seed %d, up to %gs", *s.RampOffsetSeed, s.RampOffsetMaxSeconds)
	}
	w("| scenario | %d viewers, ramp %gs%s, hold %gs, warmup %gs, join timeout %gs, codec %s |\n",
		s.Viewers, s.RampSeconds, offset, s.HoldSeconds, s.WarmupSeconds, s.JoinTimeoutSeconds, s.Codec)
	w("| client | %s/%s, %d CPUs, %s |\n\n", r.Client.OS, r.Client.Arch, r.Client.CPUs, r.Client.Go)

	a := r.Aggregate
	w("## Verdict\n\n**%s.**\n\n", esc(a.Verdict))
	w("%d viewers asked for, %d joined, %d failed (%s%%), %d dropped after joining.\n\n",
		a.Viewers, a.Joined, a.Failed, trim(a.FailedPercent), a.Dropped)

	if a.Valid {
		b := a.FingerprintDelay()
		w("## Aggregate\n\n")
		w("| metric | n | p50 | p95 | p99 | min | max |\n|---|---:|---:|---:|---:|---:|---:|\n")
		row := func(name string, sm stats.Summary, unit string) {
			if sm.N == 0 {
				w("| %s | 0 | — | — | — | — | — |\n", name)
				return
			}
			f := func(v float64) string { return fmt.Sprintf("%.1f%s", v, unit) }
			if unit == " s" || unit == "%" {
				f = func(v float64) string { return fmt.Sprintf("%.2f%s", v, unit) }
			}
			w("| %s | %d | %s | %s | %s | %s | %s |\n", name, sm.N, f(sm.P50), f(sm.P95), f(sm.P99), f(sm.Min), f(sm.Max))
		}
		if b.Available && b.Ms != nil {
			row("one-way delay (fingerprint, per frame)", *b.Ms, " ms")
		}
		row("join: first keyframe", a.FirstKeyframeMs, " ms")
		row("join: first RTP packet", a.FirstRTPMs, " ms")
		row("signalling (POST → answer)", a.SignallingMs, " ms")
		if a.PacketTransit.Available && a.PacketTransit.Ms != nil {
			row("packet transit (stamped packets)", *a.PacketTransit.Ms, " ms")
		}
		row("loss per viewer", a.LossPercent, "%")
		row("jitter per viewer", a.JitterMs, " ms")
		row("keyframe interval per viewer", a.KeyframeS, " s")
		w("\n")
		if !b.Available {
			w("**One-way delay: unavailable** — %s.\n\n", esc(b.Reason))
		} else {
			loop := ""
			if b.LoopMinMs != nil {
				loop = fmt.Sprintf(", sent in %.0f ms at the fastest", *b.LoopMinMs)
			}
			w("One-way delay (fingerprint): %d samples from %d complete frames on %d viewers (frame end: %s); %d incomplete, %d unmatched, %d invalid; %d duplicate clip frames never sampled; loop %d frames%s.\n\n",
				b.Samples, b.CompleteFrames, b.Viewers, frameEnds(b.ViewersByFrameEnd), b.IncompleteFrames, b.UnmatchedFrames, b.Invalid, b.DuplicateFrames, b.LoopFrames, loop)
			if b.Reason != "" {
				w("One-way delay %s.\n\n", esc(b.Reason))
			}
		}
		if !a.PacketTransit.Available {
			w("**Packet transit: unavailable** — %s.\n\n", esc(a.PacketTransit.Reason))
		} else if a.PacketTransit.Reason != "" {
			w("Packet transit %s.\n\n", esc(a.PacketTransit.Reason))
		}
		w("Packets: %d received, %d lost (%.3f%% of expected), %d stalls across all viewers.\n\n",
			a.PacketsReceived, a.PacketsLost, a.LossTotal, a.Stalls)
	}

	if p := r.Publisher; p != nil {
		w("## Publisher\n\n")
		stamp := "yes"
		if !p.Stamped {
			stamp = "no — the WHIP answer did not accept abs-capture-time"
		}
		w("codec %s, connected in %.0f ms, %d frames and %d packets sent over %.1f s (%d loops), schedule slips %d, send-time stamp: %s.",
			p.Codec, p.ConnectMs, p.FramesSent, p.PacketsSent, p.SendingSeconds, p.Loops, p.ScheduleSlips, stamp)
		if p.Error != "" {
			w(" Error: %s (`%s`).", esc(p.Error), p.ErrorKind)
		}
		w("\n\n")
	}

	if len(r.Errors) > 0 {
		w("## Errors\n\n| kind | count |\n|---|---:|\n")
		keys := make([]string, 0, len(r.Errors))
		for k := range r.Errors {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			w("| `%s` | %d |\n", esc(k), r.Errors[k])
		}
		w("\n")
	}

	w("## Viewers\n\n")
	w("| id | start | joined | first RTP | first keyframe | received | lost | loss | jitter | keyframe | stalls | delay p50 | delay p99 | transit p50 | transit p99 | error |\n")
	w("|---:|---:|:---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|\n")
	for _, v := range r.Viewers {
		joined := "no"
		if v.Joined {
			joined = "yes"
		}
		d50, d99 := "n/a", "n/a"
		for _, d := range v.OneWayDelay {
			if d.Source != viewer.SourceFingerprint {
				continue
			}
			if d.Available && d.Ms != nil {
				d50, d99 = fmt.Sprintf("%.1f ms", d.Ms.P50), fmt.Sprintf("%.1f ms", d.Ms.P99)
			}
			break
		}
		pt50, pt99 := "n/a", "n/a"
		if v.PacketTransit.Available && v.PacketTransit.Ms != nil {
			pt50, pt99 = fmt.Sprintf("%.1f ms", v.PacketTransit.Ms.P50), fmt.Sprintf("%.1f ms", v.PacketTransit.Ms.P99)
		}
		kf := "—"
		if v.RTP.Keyframes > 1 {
			kf = fmt.Sprintf("%.2f s", v.RTP.KeyframeIntervalMeanS)
		}
		w("| %d | %.0f ms | %s | %s | %s | %d | %d | %.2f%% | %.2f ms | %s | %d | %s | %s | %s | %s | %s |\n",
			v.ID, v.StartOffsetMs, joined, msOrDash(v.FirstRTPMs), msOrDash(v.FirstKeyframeMs),
			v.RTP.Received, v.RTP.Lost, v.RTP.LossPercent, v.RTP.JitterMs, kf, v.RTP.Stalls, d50, d99, pt50, pt99, esc(v.ErrorKind))
	}
	w("\n## Method\n\n")
	for _, m := range r.Method {
		w("- %s\n", esc(m))
	}
	return b.String()
}

func msOrDash(v *float64) string {
	if v == nil {
		return "—"
	}
	return fmt.Sprintf("%.0f ms", *v)
}

// esc keeps a value from breaking a table row.
func esc(s string) string {
	return strings.NewReplacer("|", `\|`, "\n", " ").Replace(s)
}

// frameEnds renders the viewers by frame end in key order, "marker 4, timestamp 1".
func frameEnds(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s %d", k, m[k])
	}
	return strings.Join(parts, ", ")
}
