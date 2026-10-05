package rtc

import (
	"sync/atomic"

	"github.com/pion/interceptor"
	"github.com/pion/rtcp"
)

// NACKCounter counts the RTCP NACK messages (generic NACK, RFC 4585 §6.2.1) a peer
// writes — for a viewer, the NACKs it sent (WB-41). Pass it in Options.Interceptors.
//
// pion's own stats interceptor cannot give this number. It counts a NACK only when the
// NACK passes through the RTCP writer it wraps (interceptor v0.1.49,
// pkg/stats/interceptor.go:200-210 and stats_recorder.go:187-193), but the chain binds
// writers in registration order, each wrapping the one before (chain.go:28-34), and
// RegisterDefaultInterceptors registers the NACK generator before the stats interceptor
// (webrtc v4.2.22, interceptor.go:55 and :69). The generator keeps the writer it was
// bound with and writes its NACKs there (pkg/nack/generator_interceptor.go:82-95, :227),
// beneath the stats interceptor, which therefore never sees one. NACKCounter is
// registered ahead of the defaults, so the generator's writer is its own.
//
// One message can ask for several packets, and pion's generator asks again every 100 ms
// for a packet still missing, so the count is messages sent, as WebRTC's nackCount is,
// not packets requested. The counter is shared by every peer connection built from the
// API it was passed to; a viewer builds one of each.
type NACKCounter struct{ n atomic.Uint64 }

// Sent returns the NACK messages written so far.
func (c *NACKCounter) Sent() uint64 { return c.n.Load() }

// NewInterceptor implements interceptor.Factory.
func (c *NACKCounter) NewInterceptor(string) (interceptor.Interceptor, error) {
	return &nackCount{c: c}, nil
}

type nackCount struct {
	interceptor.NoOp
	c *NACKCounter
}

// BindRTCPWriter counts every TransportLayerNack in each batch on its way out.
func (i *nackCount) BindRTCPWriter(writer interceptor.RTCPWriter) interceptor.RTCPWriter {
	return interceptor.RTCPWriterFunc(func(pkts []rtcp.Packet, attrs interceptor.Attributes) (int, error) {
		for _, p := range pkts {
			if _, ok := p.(*rtcp.TransportLayerNack); ok {
				i.c.n.Add(1)
			}
		}
		return writer.Write(pkts, attrs)
	})
}
