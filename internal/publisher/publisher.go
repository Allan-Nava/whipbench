// Package publisher is the WHIP side: it publishes a pre-encoded clip in a loop,
// paced at the clip's frame rate, and stamps every RTP packet with the wall-clock
// time it was handed to the stack.
//
// The stamp goes in the abs-capture-time header extension when the server accepts
// it in its SDP answer. With a pre-encoded clip the moment of "capture" is the
// moment of sending, so the field carries the send time; a viewer subtracts it from
// its arrival time to get the one-way delay through the network and the server.
package publisher

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/Allan-Nava/whipbench/internal/clip"
	"github.com/Allan-Nava/whipbench/internal/metrics"
	"github.com/Allan-Nava/whipbench/internal/rtc"
	"github.com/Allan-Nava/whipbench/internal/whip"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"
)

// MTU is the RTP packet payload budget: small enough for any path a WebRTC
// connection takes once SRTP, UDP, IP and a TURN header are added.
const MTU = 1200

// Config is one publisher.
type Config struct {
	WHIP   string
	Bearer string
	Clip   *clip.Clip
	RTC    rtc.Options
	HTTP   *http.Client
	Live   *metrics.Live
	// ConnectTimeout bounds the POST plus ICE and DTLS; 0 means 10 s.
	ConnectTimeout time.Duration
	// NoStamp disables the send-time stamp, to measure what it costs or to check a
	// server's behaviour without it.
	NoStamp bool
}

// Result is what the publisher reports at the end.
type Result struct {
	Codec string `json:"codec"`
	// Stamped says whether the server accepted abs-capture-time, so packets carried
	// the send time.
	Stamped          bool    `json:"stamped"`
	ConnectMs        float64 `json:"connectMs"`
	FramesSent       uint64  `json:"framesSent"`
	PacketsSent      uint64  `json:"packetsSent"`
	BytesSent        uint64  `json:"bytesSent"`
	Loops            uint64  `json:"loops"`
	ScheduleSlips    int     `json:"scheduleSlips"`
	SendingSeconds   float64 `json:"sendingSeconds"`
	Error            string  `json:"error,omitempty"`
	ErrorKind        string  `json:"errorKind,omitempty"`
	DisconnectedLate bool    `json:"disconnectedLate,omitempty"`
}

// Publisher is a running WHIP session.
type Publisher struct {
	cfg     Config
	pc      *webrtc.PeerConnection
	track   *webrtc.TrackLocalStaticRTP
	session *whip.Session
	extID   uint8
	res     Result
}

func fail(res *Result, kind string, err error) error {
	res.ErrorKind = kind
	var we *whip.Error
	if errors.As(err, &we) {
		res.ErrorKind = we.Kind
	}
	res.Error = err.Error()
	return err
}

// Connect performs the WHIP exchange and waits for the connection to be up.
func Connect(ctx context.Context, cfg Config) (*Publisher, error) {
	p := &Publisher{cfg: cfg, res: Result{Codec: cfg.Clip.Codec}}
	if cfg.HTTP == nil {
		p.cfg.HTTP = &http.Client{Timeout: 15 * time.Second}
	}
	if p.cfg.ConnectTimeout == 0 {
		p.cfg.ConnectTimeout = 10 * time.Second
	}
	codec, ok := rtc.CodecFor(cfg.Clip.Codec)
	if !ok {
		return p, fail(&p.res, "config", fmt.Errorf("no codec %q", cfg.Clip.Codec))
	}
	api, err := rtc.NewAPI(cfg.RTC, cfg.Clip.Codec)
	if err != nil {
		return p, fail(&p.res, "webrtc", err)
	}
	pc, err := api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return p, fail(&p.res, "webrtc", err)
	}
	p.pc = pc
	track, err := webrtc.NewTrackLocalStaticRTP(codec.RTPCodecCapability, "video", "whipbench")
	if err != nil {
		return p, p.abort(fail(&p.res, "webrtc", err))
	}
	p.track = track
	tr, err := pc.AddTransceiverFromTrack(track, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendonly})
	if err != nil {
		return p, p.abort(fail(&p.res, "webrtc", err))
	}
	// RTCP has to be read for the interceptors (NACK responder, reports) to work.
	go func() {
		buf := make([]byte, 1500)
		for {
			if _, _, err := tr.Sender().Read(buf); err != nil {
				return
			}
		}
	}()
	connected := make(chan struct{})
	failed := make(chan struct{})
	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		switch s {
		case webrtc.PeerConnectionStateConnected:
			closeOnce(connected)
		case webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed:
			closeOnce(failed)
		}
	})

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		return p, p.abort(fail(&p.res, "webrtc", err))
	}
	gathered := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(offer); err != nil {
		return p, p.abort(fail(&p.res, "webrtc", err))
	}
	<-gathered

	cctx, cancel := context.WithTimeout(ctx, p.cfg.ConnectTimeout)
	defer cancel()
	t0 := time.Now()
	sess, err := whip.Offer(cctx, p.cfg.HTTP, cfg.WHIP, pc.LocalDescription().SDP, cfg.Bearer)
	if err != nil {
		return p, p.abort(fail(&p.res, "whip", err))
	}
	p.session = sess
	if err := pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: sess.Answer}); err != nil {
		return p, p.abort(fail(&p.res, "bad_answer", err))
	}
	if !cfg.NoStamp {
		p.extID = rtc.ExtensionID(tr.Sender().GetParameters().HeaderExtensions, rtc.AbsCaptureTimeURI)
	}
	p.res.Stamped = p.extID != 0
	select {
	case <-connected:
	case <-failed:
		return p, p.abort(fail(&p.res, "ice_failed", errors.New("the connection to the server failed (ICE or DTLS)")))
	case <-cctx.Done():
		return p, p.abort(fail(&p.res, "connect_timeout", fmt.Errorf("not connected after %s", p.cfg.ConnectTimeout)))
	}
	p.res.ConnectMs = float64(time.Since(t0)) / float64(time.Millisecond)
	return p, nil
}

func closeOnce(c chan struct{}) {
	select {
	case <-c:
	default:
		close(c)
	}
}

func (p *Publisher) abort(err error) error {
	if p.session != nil {
		dctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = whip.Delete(dctx, p.cfg.HTTP, p.session, p.cfg.Bearer)
		cancel()
	}
	if p.pc != nil {
		_ = p.pc.Close()
	}
	return err
}

// Stamped says whether packets carry the send-time stamp.
func (p *Publisher) Stamped() bool { return p.extID != 0 }

// Stream sends the clip in a loop until ctx is done, then ends the session.
//
// Frame k is due k frame durations after the first. The packets of a frame are
// written back to back, each stamped just before it is written. When the sender
// falls more than half a second behind (a suspended laptop, a starved CPU) the
// schedule restarts from now rather than bursting to catch up; RTP timestamps keep
// counting frames, so a slip shows up as jitter at the viewers and is counted here.
func (p *Publisher) Stream(ctx context.Context) Result {
	c := p.cfg.Clip
	var pay rtp.Payloader
	switch c.Codec {
	case clip.VP8:
		pay = &codecs.VP8Payloader{EnablePictureID: true}
	default:
		pay = &codecs.H264Payloader{}
	}
	seq := uint16(rand.UintN(1 << 16)) //nolint:gosec // RFC 3550 §5.1: random initial values
	base := rand.Uint32()              //nolint:gosec // idem
	fd := c.FrameDuration()
	start := time.Now()
	timer := time.NewTimer(0)
	defer timer.Stop()
	disconnected := make(chan struct{})
	p.pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		// Disconnected can recover; Failed cannot.
		if s == webrtc.PeerConnectionStateFailed {
			closeOnce(disconnected)
		}
	})
	origin := start
	var k uint64
loop:
	for ; ; k++ {
		due := origin.Add(time.Duration(k) * fd)
		if lag := time.Since(due); lag > 500*time.Millisecond {
			p.res.ScheduleSlips++
			origin = time.Now().Add(-time.Duration(k) * fd)
			due = time.Now()
		}
		timer.Reset(time.Until(due))
		select {
		case <-ctx.Done():
			break loop
		case <-disconnected:
			p.res.DisconnectedLate = true
			p.res.ErrorKind, p.res.Error = "disconnected", "the server connection dropped while publishing"
			break loop
		case <-timer.C:
		}
		f := c.Frame(k)
		ts := c.Timestamp(base, k)
		payloads := pay.Payload(MTU, f.Data)
		for i, pl := range payloads {
			pkt := &rtp.Packet{
				Header:  rtp.Header{Version: 2, SequenceNumber: seq, Timestamp: ts, Marker: i == len(payloads)-1},
				Payload: pl,
			}
			seq++
			if p.extID != 0 {
				ext, _ := rtp.NewAbsCaptureTimeExtension(time.Now()).Marshal()
				_ = pkt.SetExtension(p.extID, ext)
			}
			if err := p.track.WriteRTP(pkt); err != nil {
				continue // a closing connection; the state change ends the loop
			}
			p.res.PacketsSent++
			p.res.BytesSent += uint64(len(pl))
			if p.cfg.Live != nil {
				p.cfg.Live.PacketsSent.Add(1)
			}
		}
		p.res.FramesSent++
		if p.cfg.Live != nil {
			p.cfg.Live.FramesSent.Add(1)
		}
	}
	p.res.Loops = k / uint64(len(c.Frames))
	p.res.SendingSeconds = time.Since(start).Seconds()
	_ = p.abort(nil)
	return p.res
}

// Result returns the result so far (for a publisher that failed to connect).
func (p *Publisher) Result() Result { return p.res }
