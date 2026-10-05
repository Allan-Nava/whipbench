// Package clocksync measures the offset between two hosts' wall clocks over UDP, so a
// split run — publisher on one host, viewers on another — knows the clock it runs with
// (WB-3; WB-1, D6).
//
// The exchange is request and response, no connection. The viewer (the client) sends a
// 32-byte request carrying a nonce and t1, its wall clock in Unix nanoseconds; the
// publisher (the responder) answers with a packet of the same size carrying the nonce, t1
// echoed untouched, t2 (its wall clock at receipt) and t3 (its wall clock at send). The
// answer is never larger than the request, so the responder amplifies nothing.
//
// Per probe, with t4 the client's arrival reading:
//
//	rtt    = (t4mono − t1mono) − (t3 − t2)
//	offset = ((t2 − t1w) + (t3 − t4w)) / 2,  t4w = t1w + (t4mono − t1mono)
//
// The round trip is on the client's monotonic clock; the offset uses wall readings, but
// t4w is derived from t1w and the monotonic elapsed time, so a wall-clock step during one
// probe cannot corrupt it. The sign is the responder's clock minus the client's. Of the
// probes that answer, the one with the smallest round trip is kept, and its offset is
// known to within rtt/2 — the exchange cannot tell an asymmetric path from an offset.
package clocksync

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"
)

// Magic opens every request and response.
const Magic = "WBC1"

// PacketSize is the size of a request and of a response; anything else is dropped.
const PacketSize = 32

// DefaultProbes and DefaultSpacing are what Measure uses when given zero.
const (
	DefaultProbes  = 16
	DefaultSpacing = 10 * time.Millisecond
)

// AnswerTimeout is how long Measure waits after its last probe for answers still in flight.
const AnswerTimeout = time.Second

// ErrNoAnswer: no probe was answered within AnswerTimeout of the last one.
var ErrNoAnswer = errors.New("clocksync: the clock peer did not answer")

// Point is one measurement: the best probe of one Measure call.
type Point struct {
	// At is the client's reading at the probe's midpoint, t1 plus half the elapsed time.
	At time.Time
	// Offset is the responder's clock minus the client's.
	Offset time.Duration
	// RTT is the probe's round trip less the responder's own processing time.
	RTT time.Duration
}

// request encodes a request: magic, nonce, t1, then 16 zero bytes.
func request(nonce uint32, t1 int64) []byte {
	b := make([]byte, PacketSize)
	copy(b, Magic)
	binary.BigEndian.PutUint32(b[4:], nonce)
	binary.BigEndian.PutUint64(b[8:], uint64(t1)) //nolint:gosec // a Unix-ns reading, round-tripped bit for bit
	return b
}

// isRequest: exactly PacketSize bytes, the magic, and zero padding — which also keeps a
// responder from answering another responder's answer, whose t2 and t3 are not zero.
func isRequest(b []byte) bool {
	if len(b) != PacketSize || string(b[:4]) != Magic {
		return false
	}
	for _, c := range b[16:] {
		if c != 0 {
			return false
		}
	}
	return true
}

// Serve answers requests on conn until ctx is done, then returns nil. now is the
// responder's clock — time.Now in production; tests pass a clock with an offset.
// Anything that is not a well-formed request is dropped without an answer.
func Serve(ctx context.Context, conn net.PacketConn, now func() time.Time) error {
	stop := context.AfterFunc(ctx, func() { _ = conn.SetReadDeadline(time.Unix(1, 0)) })
	defer stop()
	buf := make([]byte, 64)
	for {
		n, from, err := conn.ReadFrom(buf)
		t2 := now().UnixNano()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("clocksync: %w", scrub(err))
		}
		if !isRequest(buf[:n]) {
			continue
		}
		// Magic, nonce and t1 go back untouched, then t2 and t3.
		out := make([]byte, PacketSize)
		copy(out, buf[:16])
		binary.BigEndian.PutUint64(out[16:], uint64(t2))               //nolint:gosec // as in request
		binary.BigEndian.PutUint64(out[24:], uint64(now().UnixNano())) //nolint:gosec // as in request
		_, _ = conn.WriteTo(out, from)
	}
}

type sent struct {
	t1 time.Time // with its monotonic reading
	w  int64     // t1's wall clock in Unix ns, as sent
}

type arrival struct {
	b  []byte
	t4 time.Time
}

// Measure sends probes requests to peer, spacing apart (DefaultProbes and DefaultSpacing
// when zero), matches answers by nonce and echoed t1, and returns the point of the
// answer with the smallest round trip. It returns once every probe has answered, or
// AnswerTimeout after the last one; no answer at all is ErrNoAnswer. Errors never carry
// the peer's address. conn must not be read by anything else while Measure runs.
func Measure(ctx context.Context, conn net.PacketConn, peer net.Addr, probes int, spacing time.Duration) (Point, error) {
	if probes <= 0 {
		probes = DefaultProbes
	}
	if spacing <= 0 {
		spacing = DefaultSpacing
	}
	var seed [4]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return Point{}, fmt.Errorf("clocksync: %w", err)
	}
	base := binary.BigEndian.Uint32(seed[:])

	// The reader stamps t4 as soon as an answer arrives and hands it over; it stops when
	// the read deadline passes, which ending this call sets to the past.
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		return Point{}, fmt.Errorf("clocksync: %w", scrub(err))
	}
	arrivals := make(chan arrival, probes)
	quit, readerDone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(readerDone)
		buf := make([]byte, 64)
		for {
			n, _, err := conn.ReadFrom(buf)
			t4 := time.Now()
			if err != nil {
				return
			}
			if n != PacketSize || string(buf[:4]) != Magic {
				continue
			}
			select {
			case arrivals <- arrival{b: append([]byte(nil), buf[:n]...), t4: t4}:
			case <-quit:
				return
			}
		}
	}()
	defer func() {
		close(quit)
		_ = conn.SetReadDeadline(time.Unix(1, 0))
		<-readerDone
	}()

	pending := make(map[uint32]sent, probes)
	var best Point
	answered := 0
	take := func(a arrival) {
		nonce := binary.BigEndian.Uint32(a.b[4:])
		s, ok := pending[nonce]
		if !ok || int64(binary.BigEndian.Uint64(a.b[8:])) != s.w { //nolint:gosec // as in request
			return // stale, duplicate or forged: not one of this call's probes
		}
		delete(pending, nonce)
		t2 := int64(binary.BigEndian.Uint64(a.b[16:])) //nolint:gosec // as in request
		t3 := int64(binary.BigEndian.Uint64(a.b[24:])) //nolint:gosec // as in request
		elapsed := a.t4.Sub(s.t1)
		rtt := elapsed - time.Duration(t3-t2)
		if rtt < 0 || t3 < t2 {
			return // the responder's clock went backwards between t2 and t3
		}
		t4w := s.w + int64(elapsed)
		p := Point{At: s.t1.Add(elapsed / 2), Offset: time.Duration(((t2 - s.w) + (t3 - t4w)) / 2), RTT: rtt}
		if answered == 0 || p.RTT < best.RTT {
			best = p
		}
		answered++
	}

	tick := time.NewTicker(spacing)
	defer tick.Stop()
	var deadline <-chan time.Time
	for i := 0; ; {
		if i < probes {
			nonce := base + uint32(i) //nolint:gosec // i < probes, a small int
			t1 := time.Now()
			s := sent{t1: t1, w: t1.UnixNano()}
			pending[nonce] = s
			if _, err := conn.WriteTo(request(nonce, s.w), peer); err != nil {
				return Point{}, fmt.Errorf("clocksync: %w", scrub(err))
			}
			i++
			if i == probes {
				deadline = time.After(AnswerTimeout)
			}
		}
		for waiting := true; waiting; {
			select {
			case <-ctx.Done():
				if answered > 0 {
					return best, nil
				}
				return Point{}, ctx.Err()
			case a := <-arrivals:
				take(a)
				if i == probes && len(pending) == 0 {
					return best, nil
				}
			case <-tick.C:
				if i < probes {
					waiting = false
				}
			case <-deadline:
				if answered == 0 {
					return Point{}, ErrNoAnswer
				}
				return best, nil
			}
		}
	}
}

// scrub drops the addresses a *net.OpError prints, keeping the cause, so no error from
// this package names the peer.
func scrub(err error) error {
	var oe *net.OpError
	if errors.As(err, &oe) && oe.Err != nil {
		return oe.Err
	}
	return err
}
