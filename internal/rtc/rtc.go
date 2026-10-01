// Package rtc builds the pion API every whipbench peer uses, so the publisher, the
// viewers and the test server negotiate the same codecs and the same header
// extension.
package rtc

import (
	"strings"

	"github.com/pion/ice/v4"
	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v4"
)

// AbsCaptureTimeURI is the RTP header extension whipbench stamps send times into:
// http://www.webrtc.org/experiments/rtp-hdrext/abs-capture-time, a 64-bit NTP
// timestamp (UQ32.32) in a one-byte-header extension element. pion/rtp implements
// its payload as rtp.AbsCaptureTimeExtension.
const AbsCaptureTimeURI = "http://www.webrtc.org/experiments/rtp-hdrext/abs-capture-time"

// Options changes how a peer gathers ICE candidates.
type Options struct {
	// LoopbackOnly gathers host candidates on the loopback interface alone. The
	// tests use it so a round trip touches no other interface; it also suits a
	// server on the same machine.
	LoopbackOnly bool
	// IncludeLoopback adds loopback candidates to the usual ones, for a server in a
	// local container that advertises 127.0.0.1.
	IncludeLoopback bool
}

// Codec capabilities. RTX is deliberately not registered: a retransmission then
// arrives with its original sequence number on the original stream (NACK is still
// negotiated), so the loss a viewer reports is what was never recovered, counted
// on one sequence space.
var (
	feedback = []webrtc.RTCPFeedback{
		{Type: "goog-remb"}, {Type: "ccm", Parameter: "fir"}, {Type: "nack"}, {Type: "nack", Parameter: "pli"},
	}
	VP8 = webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8, ClockRate: 90000, RTCPFeedback: feedback},
		PayloadType:        96,
	}
	H264 = webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType: webrtc.MimeTypeH264, ClockRate: 90000, RTCPFeedback: feedback,
			SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
		},
		PayloadType: 102,
	}
	h264Baseline = webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType: webrtc.MimeTypeH264, ClockRate: 90000, RTCPFeedback: feedback,
			SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f",
		},
		PayloadType: 104,
	}
)

// CodecFor returns the parameters of a codec by whipbench name ("vp8", "h264").
func CodecFor(name string) (webrtc.RTPCodecParameters, bool) {
	switch name {
	case "vp8":
		return VP8, true
	case "h264":
		return H264, true
	}
	return webrtc.RTPCodecParameters{}, false
}

// CodecName maps a negotiated MIME type back to the whipbench name, or "".
func CodecName(mime string) string {
	switch strings.ToLower(mime) {
	case strings.ToLower(webrtc.MimeTypeVP8):
		return "vp8"
	case strings.ToLower(webrtc.MimeTypeH264):
		return "h264"
	}
	return ""
}

// NewAPI returns a pion API that offers or accepts the given codecs (all of them
// when none is named), the abs-capture-time extension, and pion's default
// interceptors (NACK, RTCP reports, TWCC).
func NewAPI(opt Options, codecs ...string) (*webrtc.API, error) {
	me := &webrtc.MediaEngine{}
	var params []webrtc.RTPCodecParameters
	if len(codecs) == 0 {
		params = []webrtc.RTPCodecParameters{VP8, H264, h264Baseline}
	}
	for _, c := range codecs {
		p, ok := CodecFor(c)
		if !ok {
			continue
		}
		params = append(params, p)
		if c == "h264" {
			params = append(params, h264Baseline)
		}
	}
	for _, p := range params {
		if err := me.RegisterCodec(p, webrtc.RTPCodecTypeVideo); err != nil {
			return nil, err
		}
	}
	if err := me.RegisterHeaderExtension(webrtc.RTPHeaderExtensionCapability{URI: AbsCaptureTimeURI}, webrtc.RTPCodecTypeVideo); err != nil {
		return nil, err
	}
	ir := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(me, ir); err != nil {
		return nil, err
	}
	se := webrtc.SettingEngine{}
	se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	switch {
	case opt.LoopbackOnly:
		se.SetIncludeLoopbackCandidate(true)
		se.SetInterfaceFilter(func(name string) bool { return strings.HasPrefix(name, "lo") })
		se.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
	case opt.IncludeLoopback:
		se.SetIncludeLoopbackCandidate(true)
	}
	return webrtc.NewAPI(webrtc.WithMediaEngine(me), webrtc.WithInterceptorRegistry(ir), webrtc.WithSettingEngine(se)), nil
}

// ExtensionID returns the negotiated id of uri among exts, or 0 when the remote
// peer did not accept it.
func ExtensionID(exts []webrtc.RTPHeaderExtensionParameter, uri string) uint8 {
	for _, h := range exts {
		if h.URI == uri && h.ID > 0 && h.ID < 256 {
			return uint8(h.ID)
		}
	}
	return 0
}
