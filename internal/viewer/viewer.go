// Package viewer is one WHEP viewer: it negotiates a receive-only video session,
// reads every RTP packet, and reports join time, loss, jitter, keyframe spacing,
// stalls and — when the stamp survives the server — packet transit.
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

	"github.com/Allan-Nava/whipbench/internal/fingerprint"
	"github.com/Allan-Nava/whipbench/internal/metrics"
	"github.com/Allan-Nava/whipbench/internal/reassembler"
	"github.com/Allan-Nava/whipbench/internal/rtc"
	"github.com/Allan-Nava/whipbench/internal/rtpstats"
	"github.com/Allan-Nava/whipbench/internal/stats"
	"github.com/Allan-Nava/whipbench/internal/whip"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

// MaxPlausibleTransit bounds a packet transit sample. Anything above it, or below
// zero, says the clocks disagree or the stamp was rewritten, not that the network
// took that long.
const MaxPlausibleTransit = 60 * time.Second

// MaxInvalidStampShare is the share of stamped packets that may give an
// implausible transit before the viewer reports packet transit as unavailable.
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
	// Frames and SendLog, both set by run, from the clip it publishes, turn on one-way
	// delay; view leaves them nil and the report says why.
	Frames  *fingerprint.Table
	SendLog *fingerprint.SendLog
}

// PacketTransit is a viewer's per-packet arrival minus send stamp, or why it could not be measured.
type PacketTransit struct {
	Available bool `json:"available"`
	// Reason says why packet transit is unavailable; empty when it is available.
	Reason string `json:"reason,omitempty"`
	// Negotiated: the server's WHEP answer accepted abs-capture-time.
	Negotiated bool `json:"negotiated"`
	// Stamped is the number of packets that carried a stamp; Invalid those whose
	// transit was negative or above MaxPlausibleTransit (excluded from the summary).
	Stamped uint64         `json:"stamped"`
	Invalid uint64         `json:"invalid"`
	Ms      *stats.Summary `json:"ms,omitempty"`

	hist *stats.Histogram
}

// Histogram is the viewer's packet transit histogram, for merging into a run aggregate;
// nil when packet transit is unavailable.
func (l *PacketTransit) Histogram() *stats.Histogram {
	if !l.Available {
		return nil
	}
	return l.hist
}

// Result is one viewer's report.
type Result struct {
	ID int `json:"id"`
	// StartOffsetMs is when the viewer started, after the warmup: its slot on the
	// ramp plus RampOffsetMs.
	StartOffsetMs float64 `json:"startOffsetMs"`
	// RampOffsetMs is the seeded offset included in StartOffsetMs (WB-8); absent
	// when the scenario has no rampOffsetSeed.
	RampOffsetMs *float64 `json:"rampOffsetMs,omitempty"`
	Joined       bool     `json:"joined"`
	Codec        string   `json:"codec,omitempty"`

	SignallingMs    *float64 `json:"signallingMs,omitempty"`
	ICEConnectedMs  *float64 `json:"iceConnectedMs,omitempty"`
	FirstRTPMs      *float64 `json:"firstRtpMs,omitempty"`
	FirstKeyframeMs *float64 `json:"firstKeyframeMs,omitempty"`

	RTP           rtpstats.Summary `json:"rtp"`
	OneWayDelay   []OneWayDelay    `json:"oneWayDelay"`
	PacketTransit PacketTransit    `json:"packetTransit"`

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
	transit   PacketTransit
	extID     uint8
	joinedSet bool
	owdOn     bool
	owd       OneWayDelay
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
		res.PacketTransit = finishPacketTransit(pr.transit)
		if pr.owdOn {
			res.OneWayDelay = []OneWayDelay{pr.owd}
		}
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
		pr.transit.Negotiated = pr.extID != 0
		pr.transit.hist = stats.NewHistogram()
		if cfg.Frames != nil && cfg.SendLog != nil {
			pr.owdOn = true
			pr.owd = OneWayDelay{Source: SourceFingerprint, Hist: stats.NewHistogram()}
		}
		pr.mu.Unlock()
		go drainRTCP(receiver)
		defer close(pr.readDone)
		readLoop(track, pr, live, cfg.Frames, cfg.SendLog)
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

func readLoop(track *webrtc.TrackRemote, pr *progress, live *metrics.Live, frames *fingerprint.Table, log *fingerprint.SendLog) {
	var rs *reassembler.Reassembler
	var m *fingerprint.Matcher
	pr.mu.Lock()
	codec, extID, st, origin := pr.codec, pr.extID, pr.stream, pr.origin
	if pr.owdOn {
		var err error
		if rs, err = reassembler.New(codec); err != nil {
			pr.owd.Reason = fmt.Sprintf("codec %q has no fingerprint", codec)
			rs = nil
		} else {
			m = fingerprint.NewMatcher(log, frames.Ticks())
		}
	}
	pr.mu.Unlock()
	var lastInc uint64
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
				pr.transit.Stamped++
				d := now.Sub(ac.CaptureTime())
				if d < 0 || d > MaxPlausibleTransit {
					pr.transit.Invalid++
				} else {
					v := float64(d) / float64(time.Millisecond)
					pr.transit.hist.Add(v)
					live.PacketTransit(v)
				}
			}
		}
		pr.mu.Unlock()

		// One-way delay: reassembly and hashing stay outside pr.mu.
		if rs == nil {
			continue
		}
		done := rs.Push(pkt, now)
		if len(done) == 0 && rs.Incomplete() == lastInc {
			continue
		}
		outs := make([]outcome, len(done))
		ds := make([]time.Duration, len(done))
		for j, f := range done {
			outs[j], ds[j] = classify(f, codec, frames, m)
		}
		pr.mu.Lock()
		for j, o := range outs {
			pr.owd.CompleteFrames++
			switch o {
			case sampled:
				pr.owd.Hist.Add(float64(ds[j]) / float64(time.Millisecond))
			case invalid:
				pr.owd.Invalid++
			case unmatched:
				pr.owd.UnmatchedFrames++
			case duplicate: // complete, never sampled: CompleteFrames is all it adds
			}
		}
		pr.owd.IncompleteFrames = rs.Incomplete()
		pr.owd.FrameEnd = rs.FrameEnd()
		pr.mu.Unlock()
		lastInc = rs.Incomplete()
	}
}

func finishPacketTransit(l PacketTransit) PacketTransit {
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

// outcome is what one complete frame contributes to the viewer's block.
type outcome int

const (
	sampled   outcome = iota
	invalid           // aliased or not logged (P6)
	unmatched         // rejected, nothing to hash, or not in the clip
	duplicate         // a clip duplicate: complete, never sampled
)

// classify hashes and matches one complete frame. It runs outside pr.mu.
func classify(f reassembler.Frame, codec string, frames *fingerprint.Table, m *fingerprint.Matcher) (outcome, time.Duration) {
	if f.Rejected {
		return unmatched, 0
	}
	fp, ok := fingerprint.Of(codec, f.Payload)
	if !ok {
		return unmatched, 0
	}
	i, st := frames.Lookup(fp)
	switch st {
	case fingerprint.Unique:
		if v, d := m.Match(i, f.Timestamp, f.Arrival); v == fingerprint.Sampled {
			return sampled, d
		}
		return invalid, 0
	case fingerprint.Duplicate:
		return duplicate, 0
	default:
		return unmatched, 0
	}
}
