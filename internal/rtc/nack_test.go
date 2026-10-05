package rtc

import (
	"testing"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

func TestNACKCounterCountsNACKMessagesOnly(t *testing.T) {
	c := &NACKCounter{}
	i, err := c.NewInterceptor("")
	if err != nil {
		t.Fatal(err)
	}
	var passed int
	w := i.BindRTCPWriter(interceptor.RTCPWriterFunc(func(pkts []rtcp.Packet, _ interceptor.Attributes) (int, error) {
		passed += len(pkts)
		return 0, nil
	}))
	nack := &rtcp.TransportLayerNack{MediaSSRC: 1, Nacks: rtcp.NackPairsFromSequenceNumbers([]uint16{3, 4, 9})}
	if _, err := w.Write([]rtcp.Packet{nack, &rtcp.PictureLossIndication{MediaSSRC: 1}}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]rtcp.Packet{nack}, nil); err != nil {
		t.Fatal(err)
	}
	// Two messages, though the first asked for three packets; the PLI is not a NACK.
	if c.Sent() != 2 || passed != 3 {
		t.Fatalf("Sent() = %d, passed %d; want 2 and 3", c.Sent(), passed)
	}
}

// The counter must sit where pion's NACK generator writes: registered ahead of the
// defaults, as NewAPI does, it sees a NACK the generator sends for a gap.
func TestNACKCounterSeesTheDefaultGenerator(t *testing.T) {
	c := &NACKCounter{}
	ir := &interceptor.Registry{}
	ir.Add(c)
	if err := webrtc.RegisterDefaultInterceptors(&webrtc.MediaEngine{}, ir); err != nil {
		t.Fatal(err)
	}
	chain, err := ir.Build("test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = chain.Close() }()
	wrote := make(chan struct{}, 16)
	chain.BindRTCPWriter(interceptor.RTCPWriterFunc(func(pkts []rtcp.Packet, _ interceptor.Attributes) (int, error) {
		for _, p := range pkts {
			if _, ok := p.(*rtcp.TransportLayerNack); ok {
				wrote <- struct{}{}
			}
		}
		return 0, nil
	}))
	info := &interceptor.StreamInfo{SSRC: 7, ClockRate: 90000, MimeType: webrtc.MimeTypeVP8,
		RTCPFeedback: []interceptor.RTCPFeedback{{Type: "nack"}}}
	seqs := []uint16{1, 2, 4, 5} // 3 is missing
	n := 0
	reader := chain.BindRemoteStream(info, interceptor.RTPReaderFunc(func(b []byte, a interceptor.Attributes) (int, interceptor.Attributes, error) {
		p := rtp.Packet{Header: rtp.Header{Version: 2, SSRC: 7, SequenceNumber: seqs[n%len(seqs)], PayloadType: 96}, Payload: []byte{0}}
		n++
		m, err := p.MarshalTo(b)
		return m, a, err
	}))
	buf := make([]byte, 1500)
	for range seqs {
		if _, _, err := reader.Read(buf, interceptor.Attributes{}); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-wrote:
	case <-time.After(2 * time.Second):
		t.Fatal("the NACK generator sent no NACK for the gap")
	}
	if c.Sent() == 0 {
		t.Fatal("a NACK reached the transport without passing the counter")
	}
}
