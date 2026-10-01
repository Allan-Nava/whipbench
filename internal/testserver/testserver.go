// Package testserver is a minimal WHIP/WHEP relay built on pion, for tests: one
// publisher on POST /whip, any number of viewers on POST /whep, sessions ended by
// DELETE on the Location each answer returns. It forwards every RTP packet of the
// publisher's video track to every viewer with its sequence number and timestamp
// unchanged, the way a selective forwarding unit does.
//
// Header extensions are renegotiated per leg, so the relay re-maps the one it knows
// — abs-capture-time — from the publisher's id to each viewer's, and drops the rest
// (they belong to the publisher leg's transport). Options make it behave like the
// servers a benchmark has to survive: one that strips the extension, one that
// refuses viewers beyond a limit, one that drops packets.
//
// It is not a production server and never listens on anything but what the test
// hands it (an httptest server on loopback).
package testserver

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Allan-Nava/whipbench/internal/rtc"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

// Options shape the relay's behaviour.
type Options struct {
	RTC rtc.Options
	// StripExtensions forwards packets with no header extension at all.
	StripExtensions bool
	// MaxViewers refuses viewers beyond this many with 503; 0 is unlimited.
	MaxViewers int
	// DropEvery drops every n-th forwarded packet on every viewer leg; 0 drops none.
	DropEvery int
	// Token, when set, is the bearer every request must carry.
	Token string
}

// Server is the relay.
type Server struct {
	opt Options
	api *webrtc.API

	mu       sync.RWMutex
	pub      *webrtc.PeerConnection
	pubExt   uint8
	codec    *webrtc.RTPCodecCapability
	viewers  map[string]*viewerLeg
	nextID   atomic.Int64
	accepted atomic.Int64
	ready    chan struct{}
	once     sync.Once
	closed   bool
}

type viewerLeg struct {
	pc    *webrtc.PeerConnection
	track *webrtc.TrackLocalStaticRTP
	ext   uint8
	n     atomic.Uint64
}

// New returns a relay.
func New(opt Options) (*Server, error) {
	api, err := rtc.NewAPI(opt.RTC)
	if err != nil {
		return nil, err
	}
	return &Server{opt: opt, api: api, viewers: map[string]*viewerLeg{}, ready: make(chan struct{})}, nil
}

// Ready is closed once the publisher's track has arrived.
func (s *Server) Ready() <-chan struct{} { return s.ready }

// Viewers is the number of viewer sessions currently open.
func (s *Server) Viewers() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.viewers)
}

// Close ends every session.
func (s *Server) Close() {
	s.mu.Lock()
	s.closed = true
	pub := s.pub
	vs := s.viewers
	s.viewers = map[string]*viewerLeg{}
	s.pub = nil
	s.mu.Unlock()
	if pub != nil {
		_ = pub.Close()
	}
	for _, v := range vs {
		_ = v.pc.Close()
	}
}

// ServeHTTP implements POST /whip, POST /whep, DELETE /session/{id}.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.opt.Token != "" && r.Header.Get("Authorization") != "Bearer "+s.opt.Token {
		http.Error(w, "unauthorised", http.StatusUnauthorized)
		return
	}
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/whip":
		s.whip(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/whep":
		s.whep(w, r)
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/session/"):
		s.end(w, strings.TrimPrefix(r.URL.Path, "/session/"))
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func readOffer(w http.ResponseWriter, r *http.Request) (string, bool) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/sdp") {
		http.Error(w, "want application/sdp", http.StatusUnsupportedMediaType)
		return "", false
	}
	b, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return "", false
	}
	return string(b), true
}

func (s *Server) answer(pc *webrtc.PeerConnection, offer string) (string, error) {
	if err := pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: offer}); err != nil {
		return "", err
	}
	ans, err := pc.CreateAnswer(nil)
	if err != nil {
		return "", err
	}
	gathered := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(ans); err != nil {
		return "", err
	}
	<-gathered
	return pc.LocalDescription().SDP, nil
}

func (s *Server) whip(w http.ResponseWriter, r *http.Request) {
	offer, ok := readOffer(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	if s.pub != nil || s.closed {
		s.mu.Unlock()
		http.Error(w, "a publisher is already connected", http.StatusConflict)
		return
	}
	pc, err := s.api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		s.mu.Unlock()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.pub = pc
	s.mu.Unlock()

	if _, err := pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo,
		webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	pc.OnTrack(func(t *webrtc.TrackRemote, recv *webrtc.RTPReceiver) {
		c := t.Codec().RTPCodecCapability
		s.mu.Lock()
		s.codec = &c
		s.pubExt = rtc.ExtensionID(recv.GetParameters().HeaderExtensions, rtc.AbsCaptureTimeURI)
		s.mu.Unlock()
		s.once.Do(func() { close(s.ready) })
		go drain(recv)
		s.forward(t)
	})
	ans, err := s.answer(pc, offer)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/sdp")
	w.Header().Set("Location", "/session/publisher")
	w.WriteHeader(http.StatusCreated)
	_, _ = io.WriteString(w, ans)
}

func (s *Server) forward(t *webrtc.TrackRemote) {
	for {
		pkt, _, err := t.ReadRTP()
		if err != nil {
			return
		}
		s.mu.RLock()
		pubExt := s.pubExt
		var stamp []byte
		if pubExt != 0 && !s.opt.StripExtensions {
			stamp = pkt.GetExtension(pubExt)
		}
		for _, v := range s.viewers {
			n := v.n.Add(1)
			if s.opt.DropEvery > 0 && n%uint64(s.opt.DropEvery) == 0 { //nolint:gosec // positive
				continue
			}
			// A fresh header per leg: the leg's interceptors append their own
			// extensions, and a shared slice would race between legs.
			out := &rtp.Packet{Header: rtp.Header{
				Version: 2, Marker: pkt.Marker, SequenceNumber: pkt.SequenceNumber, Timestamp: pkt.Timestamp,
			}, Payload: pkt.Payload}
			if stamp != nil && v.ext != 0 {
				_ = out.SetExtension(v.ext, append([]byte(nil), stamp...))
			}
			_ = v.track.WriteRTP(out)
		}
		s.mu.RUnlock()
	}
}

func (s *Server) whep(w http.ResponseWriter, r *http.Request) {
	offer, ok := readOffer(w, r)
	if !ok {
		return
	}
	s.mu.RLock()
	codec := s.codec
	s.mu.RUnlock()
	if codec == nil {
		http.Error(w, "no stream is being published", http.StatusNotFound)
		return
	}
	if s.opt.MaxViewers > 0 && s.accepted.Add(1) > int64(s.opt.MaxViewers) {
		http.Error(w, "viewer limit reached", http.StatusServiceUnavailable)
		return
	}
	pc, err := s.api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	track, err := webrtc.NewTrackLocalStaticRTP(*codec, "video", "relay")
	if err != nil {
		_ = pc.Close()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sender, err := pc.AddTrack(track)
	if err != nil {
		_ = pc.Close()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	go func() {
		buf := make([]byte, 1500)
		for {
			if _, _, err := sender.Read(buf); err != nil {
				return
			}
		}
	}()
	ans, err := s.answer(pc, offer)
	if err != nil {
		_ = pc.Close()
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	id := fmt.Sprintf("v%d", s.nextID.Add(1))
	leg := &viewerLeg{pc: pc, track: track, ext: rtc.ExtensionID(sender.GetParameters().HeaderExtensions, rtc.AbsCaptureTimeURI)}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		_ = pc.Close()
		http.Error(w, "closed", http.StatusServiceUnavailable)
		return
	}
	s.viewers[id] = leg
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/sdp")
	w.Header().Set("Location", "/session/"+id)
	w.WriteHeader(http.StatusCreated)
	_, _ = io.WriteString(w, ans)
}

func (s *Server) end(w http.ResponseWriter, id string) {
	s.mu.Lock()
	var pc *webrtc.PeerConnection
	if id == "publisher" && s.pub != nil {
		pc, s.pub = s.pub, nil
	} else if v, ok := s.viewers[id]; ok {
		pc = v.pc
		delete(s.viewers, id)
	}
	s.mu.Unlock()
	if pc == nil {
		http.Error(w, "no such session", http.StatusNotFound)
		return
	}
	_ = pc.Close()
	w.WriteHeader(http.StatusOK)
}

func drain(r *webrtc.RTPReceiver) {
	buf := make([]byte, 1500)
	for {
		if _, _, err := r.Read(buf); err != nil {
			return
		}
	}
}
