// Package viewer is one WHEP viewer: it negotiates a receive-only video session,
// reads every RTP packet, and reports join time, loss, jitter, keyframe spacing,
// stalls and — when the stamp survives the server — one-way delay.
//
// Join time is measured from the moment the WHEP POST is sent (after ICE
// gathering, which for host candidates takes milliseconds and is not counted) to:
//
//   - first RTP packet: the first media packet of any kind arrives;
//   - first keyframe: the last packet of the first keyframe received complete
//     arrives. The viewer has no decoder, so this is not "first decoded frame"; it
//     is the earliest moment a decoder could have produced one. A viewer has
//     joined when it gets there before the join timeout.
package viewer

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Allan-Nava/whipbench/internal/metrics"
	"github.com/Allan-Nava/whipbench/internal/rtc"
	"github.com/Allan-Nava/whipbench/internal/rtpstats"
	"github.com/Allan-Nava/whipbench/internal/stats"
	"github.com/Allan-Nava/whipbench/internal/whip"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

// MaxPlausibleDelay bounds a one-way delay sample. Anything above it, or below
// zero, says the clocks disagree or the stamp was rewritten, not that the network
// took that long.
const MaxPlausibleDelay = 60 * time.Second

// MaxInvalidStampShare is the share of stamped packets that may give an
// implausible delay before the viewer reports latency as unavailable.
const MaxInvalidStampShare = 0.01

// Config is one viewer.
type Config struct {
	WHEP   string
	Bearer string
	RTC    rtc.Options
	HTTP   *http.Client
	// JoinTimeout bounds POST → first complete keyframe; 0 means 10 s.
	JoinTimeout time.Duration
	// Stall is the packet gap counted as a stall; 0 means 500 ms.
	Stall time.Duration
	Live  *metrics.Live
}

// Latency is the one-way delay a viewer measured, or why it could not.
type Latency struct {
	Available bool `json:"available"`
	// Reason says why latency is unavailable; empty when it is available.
	Reason string `json:"reason,omitempty"`
	// Negotiated: the server's WHEP answer accepted abs-capture-time.
	Negotiated bool `json:"negotiated"`
	// Stamped is the number of packets that carried a stamp; Invalid those whose
	// delay was negative or above MaxPlausibleDelay (excluded from the summary).
	Stamped uint64         `json:"stamped"`
	Invalid uint64         `json:"invalid"`
	Ms      *stats.Summary `json:"ms,omitempty"`

	hist *stats.Histogram
}

// Histogram is the viewer's delay histogram, for merging into a run aggregate;
// nil when latency is unavailable.
func (l *Latency) Histogram() *stats.Histogram {
	if !l.Available {
		return nil
	}
	return l.hist
}

// Result is one viewer's report.
type Result struct {
	ID            int     `json:"id"`
	StartOffsetMs float64 `json:"startOffsetMs"`
	Joined        bool    `json:"joined"`
	Codec         string  `json:"codec,omitempty"`

	SignallingMs    *float64 `json:"signallingMs,omitempty"`
	ICEConnectedMs  *float64 `json:"iceConnectedMs,omitempty"`
	FirstRTPMs      *float64 `json:"firstRtpMs,omitempty"`
	FirstKeyframeMs *float64 `json:"firstKeyframeMs,omitempty"`

	RTP     rtpstats.Summary `json:"rtp"`
	Latency Latency          `json:"latency"`

	// DroppedAfterJoin: the connection failed after the viewer had joined.
	DroppedAfterJoin bool   `json:"droppedAfterJoin,omitempty"`
	ErrorKind        string `json:"errorKind,omitempty"`
	Error            string `json:"error,omitempty"`
}

type progress struct {
	mu        sync.Mutex
	iceAt     time.Duration
	firstRTP  time.Duration
	firstKey  time.Duration
	haveICE   bool
	haveRTP   bool
	haveKey   bool
	origin    time.Time // the WHEP POST: every progress time is measured from it
	started   bool      // a track's read loop is running
	codec     string
	joined    chan struct{}
	failed    chan struct{}
	readDone  chan struct{}
	stream    *rtpstats.Stream
	latency   Latency
	extID     uint8
	joinedSet bool
}

func msPtr(d time.Duration) *float64 {
	v := float64(d) / float64(time.Millisecond)
	return &v
}

// Run runs one viewer until ctx is done, and returns its result. It never returns
// an error: a viewer that fails is a result with ErrorKind set.
func Run(ctx context.Context, id int, cfg Config) (res Result) {
	res.ID = id
	if cfg.HTTP == nil {
		cfg.HTTP = &http.Client{Timeout: 15 * time.Second}
	}
	if cfg.JoinTimeout == 0 {
		cfg.JoinTimeout = 10 * time.Second
	}
	if cfg.Stall == 0 {
		cfg.Stall = 500 * time.Millisecond
	}
	live := cfg.Live
	failWith := func(kind string, err error) Result {
		var we *whip.Error
		if errors.As(err, &we) {
			kind = we.Kind
			if we.Status != 0 {
				kind = fmt.Sprintf("http_%d", we.Status)
			}
		}
		res.ErrorKind, res.Error = kind, err.Error()
		if live != nil {
			live.ViewersFailed.Add(1)
		}
		return res
	}

	api, err := rtc.NewAPI(cfg.RTC)
	if err != nil {
		return failWith("webrtc", err)
	}
	pc, err := api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return failWith("webrtc", err)
	}
	var sess *whip.Session
	pr := &progress{joined: make(chan struct{}), failed: make(chan struct{}), readDone: make(chan struct{})}
	defer func() {
		dctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = whip.Delete(dctx, cfg.HTTP, sess, cfg.Bearer)
		cancel()
		_ = pc.Close()
		pr.mu.Lock()
		started := pr.started
		pr.mu.Unlock()
		if started {
			select {
			case <-pr.readDone:
			case <-time.After(3 * time.Second):
			}
		}
		pr.mu.Lock()
		defer pr.mu.Unlock()
		res.Codec = pr.codec
		if pr.haveICE {
			res.ICEConnectedMs = msPtr(pr.iceAt)
		}
		if pr.haveRTP {
			res.FirstRTPMs = msPtr(pr.firstRTP)
		}
		if pr.haveKey {
			res.FirstKeyframeMs = msPtr(pr.firstKey)
		}
		if pr.stream != nil {
			res.RTP = pr.stream.Summary()
		}
		res.Latency = finishLatency(pr.latency)
	}()

	if _, err := pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo,
		webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly}); err != nil {
		return failWith("webrtc", err)
	}
	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		switch s {
		case webrtc.PeerConnectionStateConnected:
			pr.mu.Lock()
			if !pr.haveICE {
				pr.haveICE, pr.iceAt = true, time.Since(pr.origin)
			}
			pr.mu.Unlock()
		case webrtc.PeerConnectionStateFailed:
			closeOnce(pr.failed)
		}
	})
	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		pr.mu.Lock()
		if pr.started { // one video track is all a viewer asks for
			pr.mu.Unlock()
			return
		}
		pr.started = true
		pr.codec = rtc.CodecName(track.Codec().MimeType)
		pr.stream = rtpstats.New(int(track.Codec().ClockRate), cfg.Stall)
		pr.extID = rtc.ExtensionID(receiver.GetParameters().HeaderExtensions, rtc.AbsCaptureTimeURI)
		pr.latency.Negotiated = pr.extID != 0
		pr.latency.hist = stats.NewHistogram()
		pr.mu.Unlock()
		go drainRTCP(receiver)
		defer close(pr.readDone)
		readLoop(track, pr, live)
	})

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		return failWith("webrtc", err)
	}
	gathered := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(offer); err != nil {
		return failWith("webrtc", err)
	}
	select {
	case <-gathered:
	case <-ctx.Done():
		return failWith("not_started", errors.New("the run ended before ICE gathering finished"))
	}

	// The join clock starts here, at the POST.
	pr.mu.Lock()
	pr.origin = time.Now()
	origin := pr.origin
	pr.mu.Unlock()
	if live != nil {
		live.ViewersStarted.Add(1)
	}
	jctx, cancel := context.WithTimeout(ctx, cfg.JoinTimeout)
	defer cancel()
	sess, err = whip.Offer(jctx, cfg.HTTP, cfg.WHEP, pc.LocalDescription().SDP, cfg.Bearer)
	if err != nil {
		return failWith("whep", err)
	}
	res.SignallingMs = msPtr(time.Since(origin))
	if err := pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: sess.Answer}); err != nil {
		return failWith("bad_answer", fmt.Errorf("the WHEP answer was refused: %w", err))
	}

	select {
	case <-pr.joined:
	case <-pr.failed:
		return failWith("ice_failed", errors.New("the connection failed before the first keyframe (ICE or DTLS)"))
	case <-jctx.Done():
		pr.mu.Lock()
		kind, msg := "no_keyframe", "media arrived but no complete keyframe"
		switch {
		case !pr.haveICE:
			kind, msg = "ice_timeout", "the connection was not established"
		case !pr.haveRTP:
			kind, msg = "no_media", "connected, but no RTP arrived"
		}
		pr.mu.Unlock()
		if ctx.Err() != nil {
			return failWith("run_ended", fmt.Errorf("the run ended before the viewer joined (%s)", msg))
		}
		return failWith(kind, fmt.Errorf("%s within the %s join timeout", msg, cfg.JoinTimeout))
	}
	res.Joined = true
	if live != nil {
		live.ViewersJoined.Add(1)
		live.ViewersActive.Add(1)
		defer live.ViewersActive.Add(-1)
	}
	select {
	case <-ctx.Done():
	case <-pr.failed:
		res.DroppedAfterJoin = true
		res.ErrorKind, res.Error = "dropped", "the connection failed after the viewer had joined"
	}
	return res
}

func readLoop(track *webrtc.TrackRemote, pr *progress, live *metrics.Live) {
	pr.mu.Lock()
	codec, extID, st, origin := pr.codec, pr.extID, pr.stream, pr.origin
	pr.mu.Unlock()
	var ac rtp.AbsCaptureTimeExtension
	for {
		pkt, _, err := track.ReadRTP()
		if err != nil {
			return
		}
		now := time.Now()
		arrival := now.Sub(origin)
		live.Packet(len(pkt.Payload))

		pr.mu.Lock()
		st.Add(rtpstats.Packet{
			Seq: pkt.SequenceNumber, Timestamp: pkt.Timestamp, Marker: pkt.Marker,
			KeyStart: rtpstats.KeyframeStart(codec, pkt.Payload),
		}, arrival)
		if !pr.haveRTP {
			pr.haveRTP, pr.firstRTP = true, arrival
		}
		if !pr.joinedSet {
			if at, ok := st.FirstKeyframe(); ok {
				pr.haveKey, pr.firstKey, pr.joinedSet = true, at, true
				close(pr.joined)
			}
		}
		if extID != 0 {
			if raw := pkt.GetExtension(extID); len(raw) >= 8 && ac.Unmarshal(raw) == nil {
				pr.latency.Stamped++
				d := now.Sub(ac.CaptureTime())
				if d < 0 || d > MaxPlausibleDelay {
					pr.latency.Invalid++
				} else {
					v := float64(d) / float64(time.Millisecond)
					pr.latency.hist.Add(v)
					live.Delay(v)
				}
			}
		}
		pr.mu.Unlock()
	}
}

func finishLatency(l Latency) Latency {
	switch {
	case !l.Negotiated:
		l.Reason = "not negotiated: the WHEP answer did not accept abs-capture-time"
	case l.Stamped == 0:
		l.Reason = "no stamps arrived: the server strips or rewrites the header extension, or the publisher did not stamp"
	case float64(l.Invalid) > MaxInvalidStampShare*float64(l.Stamped):
		l.Reason = fmt.Sprintf("%d of %d stamps gave a negative or implausible delay: the clocks are not synchronised, or the server rewrites the extension", l.Invalid, l.Stamped)
	case l.hist == nil || l.hist.Count() == 0:
		l.Reason = "no valid samples"
	default:
		l.Available = true
		s := l.hist.Summary()
		l.Ms = &s
	}
	return l
}

func drainRTCP(r *webrtc.RTPReceiver) {
	buf := make([]byte, 1500)
	for {
		if _, _, err := r.Read(buf); err != nil {
			return
		}
	}
}

func closeOnce(c chan struct{}) {
	select {
	case <-c:
	default:
		close(c)
	}
}
