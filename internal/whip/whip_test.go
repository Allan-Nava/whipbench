package whip

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const answer = "v=0\r\no=- 1 1 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n"

func TestOfferResolvesLocationAndSendsBearer(t *testing.T) {
	var gotAuth, gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusOK)
			return
		}
		gotAuth, gotCT = r.Header.Get("Authorization"), r.Header.Get("Content-Type")
		w.Header().Set("Location", "../session/abc")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(answer))
	}))
	defer srv.Close()
	s, err := Offer(context.Background(), srv.Client(), srv.URL+"/live/whep", "v=0", "tok")
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer tok" || gotCT != "application/sdp" {
		t.Fatalf("auth %q content-type %q", gotAuth, gotCT)
	}
	if s.Resource != srv.URL+"/session/abc" || s.Answer != answer {
		t.Fatalf("session %+v", s)
	}
	if err := Delete(context.Background(), srv.Client(), s, "tok"); err != nil {
		t.Fatal(err)
	}
}

func TestErrorsNeverCarryThePathOrQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "denied SECRET", http.StatusForbidden)
	}))
	defer srv.Close()
	_, err := Offer(context.Background(), srv.Client(), srv.URL+"/live/KEY/whep?token=SECRET", "v=0", "BEARER")
	var we *Error
	if !errors.As(err, &we) || we.Kind != "http_status" || we.Status != 403 {
		t.Fatalf("err = %#v", err)
	}
	// A transport error: net/http would put the whole URL in the message.
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	_ = ln.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err2 := Offer(ctx, http.DefaultClient, "http://"+addr+"/live/KEY/whep?token=SECRET", "v=0", "BEARER")
	for _, e := range []error{err, err2} {
		if e == nil {
			t.Fatal("expected an error")
		}
		for _, bad := range []string{"SECRET", "KEY", "BEARER", "token", "/live"} {
			if strings.Contains(e.Error(), bad) {
				t.Errorf("error %q contains %q", e, bad)
			}
		}
	}
}

func TestNotSDP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("<html>"))
	}))
	defer srv.Close()
	_, err := Offer(context.Background(), srv.Client(), srv.URL, "v=0", "")
	var we *Error
	if !errors.As(err, &we) || we.Kind != "bad_answer" {
		t.Fatalf("err = %v", err)
	}
}

func TestHost(t *testing.T) {
	for in, want := range map[string]string{
		"https://a.example.test:8443/x/y?token=1": "a.example.test:8443",
		"http://u:p@[::1]:8889/whep":              "[::1]:8889",
		"not a url":                               "invalid-url",
		"":                                        "invalid-url",
	} {
		if got := Host(in); got != want {
			t.Errorf("Host(%q) = %q, want %q", in, got, want)
		}
	}
}
