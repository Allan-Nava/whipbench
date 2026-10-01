// Package scenario is the declarative description of a run, in the spirit of a k6
// options block: what to publish, where, how many viewers, how fast they arrive
// and how long they stay. A scenario file is JSON; unknown fields are an error, so
// a typo cannot silently fall back to a default.
//
//	{
//	  "name": "mediamtx-local-50",
//	  "whip": "http://127.0.0.1:8889/bench/whip",
//	  "whep": "http://127.0.0.1:8889/bench/whep",
//	  "codec": "vp8",
//	  "viewers": 50,
//	  "rampSeconds": 10,
//	  "holdSeconds": 30
//	}
package scenario

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"
)

// Scenario is one run. The JSON names are the file format; keep them stable.
type Scenario struct {
	Name string `json:"name,omitempty"`
	// WHIP is the publisher's endpoint. Empty means no publisher: the viewers watch
	// a stream someone else publishes (what `whipbench view` does).
	WHIP string `json:"whip,omitempty"`
	WHEP string `json:"whep"`
	// Codec of the embedded clip to publish: "vp8" (default) or "h264".
	Codec string `json:"codec,omitempty"`
	// Viewers is how many WHEP viewers to start.
	Viewers int `json:"viewers"`
	// RampSeconds spreads the viewers' starts evenly over this long; 0 starts them
	// all at once.
	RampSeconds float64 `json:"rampSeconds"`
	// HoldSeconds is how long every viewer stays connected after the ramp ends.
	HoldSeconds float64 `json:"holdSeconds"`
	// WarmupSeconds is the time between the publisher connecting and the first
	// viewer, so the server has a stream to offer; default 2.
	WarmupSeconds float64 `json:"warmupSeconds,omitempty"`
	// JoinTimeoutSeconds bounds POST → first complete keyframe; default 10.
	JoinTimeoutSeconds float64 `json:"joinTimeoutSeconds,omitempty"`
	// StallMs is the packet gap counted as a stall; default 500.
	StallMs float64 `json:"stallMs,omitempty"`
	// BearerEnv names an environment variable holding a bearer token for both
	// endpoints. The token itself never appears in a scenario or a report.
	BearerEnv string `json:"bearerEnv,omitempty"`
	// Metrics is a listen address for the Prometheus endpoint, e.g.
	// "127.0.0.1:9464"; empty disables it.
	Metrics string `json:"metrics,omitempty"`
	// IncludeLoopback adds loopback ICE candidates, for a server in a local
	// container that advertises 127.0.0.1.
	IncludeLoopback bool `json:"includeLoopback,omitempty"`
	// LoopbackOnly gathers ICE candidates on the loopback interface alone, for a
	// server on the same machine; the end-to-end tests use it.
	LoopbackOnly bool `json:"loopbackOnly,omitempty"`
}

// Defaults.
const (
	DefaultWarmup      = 2.0
	DefaultJoinTimeout = 10.0
	DefaultStallMs     = 500.0
	MaxViewers         = 10000
)

// Load reads and validates a scenario file.
func Load(path string) (Scenario, error) {
	b, err := os.ReadFile(path) //nolint:gosec // the user names the file
	if err != nil {
		return Scenario{}, err
	}
	return Parse(b)
}

// Parse decodes and validates a scenario.
func Parse(b []byte) (Scenario, error) {
	var s Scenario
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return Scenario{}, fmt.Errorf("scenario: %w", err)
	}
	if dec.More() {
		return Scenario{}, errors.New("scenario: trailing data after the JSON object")
	}
	s.applyDefaults()
	return s, s.Validate()
}

func (s *Scenario) applyDefaults() {
	if s.Codec == "" {
		s.Codec = "vp8"
	}
	if s.WarmupSeconds == 0 && s.WHIP != "" {
		s.WarmupSeconds = DefaultWarmup
	}
	if s.JoinTimeoutSeconds == 0 {
		s.JoinTimeoutSeconds = DefaultJoinTimeout
	}
	if s.StallMs == 0 {
		s.StallMs = DefaultStallMs
	}
}

// Normalise fills in the defaults; Parse does it already.
func (s *Scenario) Normalise() { s.applyDefaults() }

func checkURL(field, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("scenario: %s must be an http(s) URL", field) // the URL itself is not echoed
	}
	return nil
}

// Validate reports the first thing wrong with the scenario.
func (s Scenario) Validate() error {
	if s.WHEP == "" {
		return errors.New("scenario: whep is required")
	}
	if err := checkURL("whep", s.WHEP); err != nil {
		return err
	}
	if s.WHIP != "" {
		if err := checkURL("whip", s.WHIP); err != nil {
			return err
		}
	}
	switch s.Codec {
	case "vp8", "h264":
	default:
		return fmt.Errorf("scenario: codec %q (want vp8 or h264)", s.Codec)
	}
	switch {
	case s.Viewers < 1 || s.Viewers > MaxViewers:
		return fmt.Errorf("scenario: viewers must be 1..%d", MaxViewers)
	case s.RampSeconds < 0:
		return errors.New("scenario: rampSeconds must not be negative")
	case s.HoldSeconds <= 0:
		return errors.New("scenario: holdSeconds must be positive")
	case s.WarmupSeconds < 0:
		return errors.New("scenario: warmupSeconds must not be negative")
	case s.JoinTimeoutSeconds <= 0:
		return errors.New("scenario: joinTimeoutSeconds must be positive")
	case s.StallMs <= 0:
		return errors.New("scenario: stallMs must be positive")
	}
	return nil
}

func secs(v float64) time.Duration { return time.Duration(v * float64(time.Second)) }

// Ramp, Hold, Warmup, JoinTimeout and Stall are the durations of the scenario.
func (s Scenario) Ramp() time.Duration        { return secs(s.RampSeconds) }
func (s Scenario) Hold() time.Duration        { return secs(s.HoldSeconds) }
func (s Scenario) Warmup() time.Duration      { return secs(s.WarmupSeconds) }
func (s Scenario) JoinTimeout() time.Duration { return secs(s.JoinTimeoutSeconds) }
func (s Scenario) Stall() time.Duration {
	return time.Duration(s.StallMs * float64(time.Millisecond))
}

// Starts returns when each viewer starts, relative to the end of the warmup: viewer
// i of n at i·ramp/n, so the starts are evenly spaced, the first is at 0 and the
// last is one step before the ramp ends. With no ramp every viewer starts at 0.
func Starts(n int, ramp time.Duration) []time.Duration {
	out := make([]time.Duration, n)
	if n == 0 || ramp <= 0 {
		return out
	}
	for i := range out {
		out[i] = time.Duration(int64(ramp) * int64(i) / int64(n))
	}
	return out
}

// Duration is the whole run after the warmup: every viewer starts within the ramp,
// and all stop together HoldSeconds after it.
func (s Scenario) Duration() time.Duration { return s.Ramp() + s.Hold() }

// Redacted is the scenario as a report records it: endpoints reduced to their
// host, the metrics address kept (it is the client's own), the bearer variable's
// name kept and its value never read here.
func (s Scenario) Redacted(host func(string) string) Scenario {
	r := s
	r.WHEP = host(s.WHEP)
	if s.WHIP != "" {
		r.WHIP = host(s.WHIP)
	}
	return r
}
