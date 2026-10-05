package scenario

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestStartsAreEvenlySpaced(t *testing.T) {
	got := Starts(4, 10*time.Second)
	want := []time.Duration{0, 2500 * time.Millisecond, 5 * time.Second, 7500 * time.Millisecond}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("starts = %v, want %v", got, want)
		}
	}
	for i, d := range Starts(50, 10*time.Second) {
		if d < 0 || d >= 10*time.Second {
			t.Fatalf("viewer %d starts at %v, outside the ramp", i, d)
		}
		if i > 0 && d-Starts(50, 10*time.Second)[i-1] != 200*time.Millisecond {
			t.Fatalf("viewer %d: step is not ramp/n", i)
		}
	}
}

func TestNoRampStartsEveryoneAtOnce(t *testing.T) {
	for _, d := range Starts(10, 0) {
		if d != 0 {
			t.Fatal("with rampSeconds 0 every viewer starts at 0")
		}
	}
	if len(Starts(0, time.Second)) != 0 {
		t.Fatal("no viewers, no starts")
	}
}

func TestParseDefaultsAndDuration(t *testing.T) {
	s, err := Parse([]byte(`{"whip":"http://127.0.0.1:8889/a/whip","whep":"http://127.0.0.1:8889/a/whep","viewers":5,"rampSeconds":10,"holdSeconds":30}`))
	if err != nil {
		t.Fatal(err)
	}
	if s.Codec != "vp8" || s.WarmupSeconds != 2 || s.JoinTimeoutSeconds != 10 || s.StallMs != 500 {
		t.Fatalf("defaults: %+v", s)
	}
	if s.Duration() != 40*time.Second {
		t.Fatalf("duration %v, want ramp+hold = 40s", s.Duration())
	}
	// WB-41: the sample window defaults to 5 s, is filled in so the report records it,
	// and leaves the warmup alone.
	if s.ExcludeFirstSeconds == nil || *s.ExcludeFirstSeconds != 5 || s.ExcludeFirst() != 5*time.Second {
		t.Fatalf("excludeFirstSeconds default: %v", s.ExcludeFirstSeconds)
	}
	if b, _ := json.Marshal(s); !strings.Contains(string(b), `"excludeFirstSeconds":5`) {
		t.Fatalf("the default window is not recorded: %s", b)
	}
}

// WB-41: an explicit 0 survives the default and samples from the first frame; a fraction
// is kept as given.
func TestExcludeFirstSecondsZeroAndFraction(t *testing.T) {
	for in, want := range map[string]time.Duration{"0": 0, "0.5": 500 * time.Millisecond, "12": 12 * time.Second} {
		s, err := Parse([]byte(`{"whep":"http://h/w","viewers":1,"holdSeconds":1,"excludeFirstSeconds":` + in + `}`))
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if s.ExcludeFirst() != want {
			t.Errorf("excludeFirstSeconds %s: window %v, want %v", in, s.ExcludeFirst(), want)
		}
		if b, _ := json.Marshal(s); !strings.Contains(string(b), `"excludeFirstSeconds":`+in) {
			t.Errorf("excludeFirstSeconds %s is not recorded: %s", in, b)
		}
	}
	if (Scenario{}).ExcludeFirst() != 5*time.Second {
		t.Error("a scenario that skipped Normalise must still get the default window")
	}
}

func TestParseRejects(t *testing.T) {
	for _, tc := range []struct{ name, json, want string }{
		{"unknown field", `{"whep":"http://h/w","viewers":1,"holdSeconds":1,"viewer":3}`, "unknown field"},
		{"no whep", `{"viewers":1,"holdSeconds":1}`, "whep is required"},
		{"bad scheme", `{"whep":"rtmp://h/w","viewers":1,"holdSeconds":1}`, "http(s) URL"},
		{"zero viewers", `{"whep":"http://h/w","viewers":0,"holdSeconds":1}`, "viewers"},
		{"no hold", `{"whep":"http://h/w","viewers":1}`, "holdSeconds"},
		{"negative ramp", `{"whep":"http://h/w","viewers":1,"holdSeconds":1,"rampSeconds":-1}`, "rampSeconds"},
		{"codec", `{"whep":"http://h/w","viewers":1,"holdSeconds":1,"codec":"av1"}`, "codec"},
		{"trailing", `{"whep":"http://h/w","viewers":1,"holdSeconds":1} {}`, "trailing"},
		{"negative window", `{"whep":"http://h/w","viewers":1,"holdSeconds":1,"excludeFirstSeconds":-1}`, "excludeFirstSeconds must not be negative"},
		{"huge window", `{"whep":"http://h/w","viewers":1,"holdSeconds":1,"excludeFirstSeconds":3601}`, "excludeFirstSeconds must be at most 3600"},
	} {
		_, err := Parse([]byte(tc.json))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want it to mention %q", tc.name, err, tc.want)
		}
	}
}

func TestValidationErrorDoesNotEchoTheURL(t *testing.T) {
	_, err := Parse([]byte(`{"whep":"ftp://h/w?token=SECRET","viewers":1,"holdSeconds":1}`))
	if err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("err = %v", err)
	}
}

func seed(v int64) *int64 { return &v }

func TestRampOffsetsSameSeedSameOffsets(t *testing.T) {
	a, b := RampOffsets(50, 42, time.Second), RampOffsets(50, 42, time.Second)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("offset %d: %v then %v from the same seed", i, a[i], b[i])
		}
	}
	// Offset i does not depend on how many viewers there are.
	for i, d := range RampOffsets(10, 42, time.Second) {
		if d != a[i] {
			t.Fatalf("offset %d: %v with 10 viewers, %v with 50", i, d, a[i])
		}
	}
	same := 0
	for i, d := range RampOffsets(50, 43, time.Second) {
		if d == a[i] {
			same++
		}
	}
	if same > 1 {
		t.Fatalf("seeds 42 and 43 share %d of 50 offsets", same)
	}
	// The generator is written down (SplitMix64), so a seed in a report gives the
	// same offsets on any machine and any Go version. Pin the first values.
	want := []time.Duration{741564878, 159910392, 278601130}
	for i, w := range want {
		if a[i] != w {
			t.Fatalf("seed 42: offsets %v, want %v — the generator changed, and old reports no longer reproduce", a[:3], want)
		}
	}
}

func TestRampOffsetsStayWithinTheBound(t *testing.T) {
	for _, max := range []time.Duration{time.Second, 7 * time.Millisecond, 3 * time.Second, time.Nanosecond} {
		for _, s := range []int64{0, 1, -1, 42, 1 << 62} {
			for i, d := range RampOffsets(200, s, max) {
				if d < 0 || d >= max {
					t.Fatalf("seed %d, bound %v: offset %d is %v", s, max, i, d)
				}
			}
		}
	}
	// Evenly, not merely within: 1000 offsets over a 1 s GOP, ten 100 ms bins.
	var bins [10]int
	for _, d := range RampOffsets(1000, 7, time.Second) {
		bins[d/(100*time.Millisecond)]++
	}
	for i, n := range bins {
		if n < 60 || n > 140 {
			t.Fatalf("bin %d holds %d of 1000 offsets: %v", i, n, bins)
		}
	}
	if len(RampOffsets(0, 1, time.Second)) != 0 {
		t.Fatal("no viewers, no offsets")
	}
}

func TestScheduleWithoutTheSeedIsTheRamp(t *testing.T) {
	s, err := Parse([]byte(`{"whep":"http://h/w","viewers":10,"rampSeconds":10,"holdSeconds":30}`))
	if err != nil {
		t.Fatal(err)
	}
	if s.RampOffsetSeed != nil || s.RampOffsetMaxSeconds != 0 {
		t.Fatalf("a ramp offset nobody asked for: %+v", s)
	}
	starts, offsets := s.Schedule()
	if offsets != nil {
		t.Fatalf("offsets %v without a seed", offsets)
	}
	want := Starts(10, 10*time.Second)
	for i := range want {
		if starts[i] != want[i] {
			t.Fatalf("starts %v, want the plain ramp %v", starts, want)
		}
	}
	if s.Duration() != 40*time.Second {
		t.Fatalf("duration %v, want ramp+hold = 40s", s.Duration())
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "rampOffset") {
		t.Fatalf("a scenario without the key records one: %s", b)
	}
}

func TestScheduleWithTheSeed(t *testing.T) {
	s, err := Parse([]byte(`{"whep":"http://h/w","viewers":10,"rampSeconds":10,"holdSeconds":30,"rampOffsetSeed":0}`))
	if err != nil {
		t.Fatal(err)
	}
	if s.RampOffsetSeed == nil || *s.RampOffsetSeed != 0 || s.RampOffsetMaxSeconds != DefaultRampOffsetMax {
		t.Fatalf("seed 0 is a seed, and the bound defaults to the clip's GOP: %+v", s)
	}
	starts, offsets := s.Schedule()
	ramp, want := Starts(10, 10*time.Second), RampOffsets(10, 0, time.Second)
	for i := range ramp {
		if offsets[i] != want[i] || starts[i] != ramp[i]+want[i] {
			t.Fatalf("viewer %d: start %v offset %v, want %v + %v", i, starts[i], offsets[i], ramp[i], want[i])
		}
	}
	if s.Duration() != 41*time.Second {
		t.Fatalf("duration %v, want ramp + offset bound + hold = 41s", s.Duration())
	}
	s.RampOffsetSeed, s.RampOffsetMaxSeconds = seed(5), 0.25
	if _, offsets := s.Schedule(); offsets[3] != RampOffsets(10, 5, 250*time.Millisecond)[3] {
		t.Fatal("the schedule does not use the scenario's seed and bound")
	}
}

func TestParseRejectsARampOffsetBoundWithoutItsSeed(t *testing.T) {
	for _, tc := range []struct{ name, json, want string }{
		{"bound without seed", `{"whep":"http://h/w","viewers":1,"holdSeconds":1,"rampOffsetMaxSeconds":1}`, "needs rampOffsetSeed"},
		{"negative bound", `{"whep":"http://h/w","viewers":1,"holdSeconds":1,"rampOffsetSeed":1,"rampOffsetMaxSeconds":-1}`, "rampOffsetMaxSeconds"},
		{"fractional seed", `{"whep":"http://h/w","viewers":1,"holdSeconds":1,"rampOffsetSeed":1.5}`, "rampOffsetSeed"},
	} {
		_, err := Parse([]byte(tc.json))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want it to mention %q", tc.name, err, tc.want)
		}
	}
}

// The published SplitMix64 vectors: seed 0, and seed 1234567 from Vigna's reference.
func TestSplitMix64ReferenceVectors(t *testing.T) {
	for seed, want := range map[uint64][]uint64{
		0:       {0xe220a8397b1dcdaf, 0x6e789e6aa1b965f4, 0x06c45d188009454f},
		1234567: {6457827717110365317, 3203168211198807973, 9817491932198370423},
	} {
		state := seed
		for i, w := range want {
			if got := splitmix64(&state); got != w {
				t.Fatalf("seed %d, output %d: %#x, want %#x", seed, i, got, w)
			}
		}
	}
}
