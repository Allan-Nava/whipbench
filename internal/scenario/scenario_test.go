package scenario

import (
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
