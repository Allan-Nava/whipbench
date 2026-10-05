package clocksync

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"testing"
	"time"
)

// responder serves on 127.0.0.1:0 with a clock ahead of this one by ahead, and returns
// its address.
func responder(t *testing.T, ahead time.Duration) net.Addr {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, conn, func() time.Time { return time.Now().Add(ahead) }) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("Serve: %v", err)
		}
		_ = conn.Close()
	})
	return conn.LocalAddr()
}

func client(t *testing.T) net.PacketConn {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestMeasureFindsTheOffset(t *testing.T) {
	t.Parallel()
	peer := responder(t, 250*time.Millisecond)
	conn := client(t)
	before := time.Now()
	p, err := Measure(context.Background(), conn, peer, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if p.RTT <= 0 || p.RTT > 50*time.Millisecond {
		t.Fatalf("loopback round trip %v", p.RTT)
	}
	if d := (p.Offset - 250*time.Millisecond).Abs(); d > p.RTT/2+time.Microsecond {
		t.Errorf("offset %v, want 250ms ± %v", p.Offset, p.RTT/2)
	}
	if p.At.Before(before) || p.At.After(time.Now()) {
		t.Errorf("point at %v, outside the call", p.At)
	}
	// A second call on the same socket works too: the first one's reader has stopped.
	if _, err := Measure(context.Background(), conn, peer, 4, time.Millisecond); err != nil {
		t.Fatalf("second Measure: %v", err)
	}
}

func TestMeasureWithoutAnAnswer(t *testing.T) {
	t.Parallel()
	silent := client(t) // a socket nobody serves on
	conn := client(t)
	start := time.Now()
	_, err := Measure(context.Background(), conn, silent.LocalAddr(), 2, time.Millisecond)
	if !errors.Is(err, ErrNoAnswer) {
		t.Fatalf("err %v, want ErrNoAnswer", err)
	}
	if d := time.Since(start); d < AnswerTimeout || d > AnswerTimeout+time.Second {
		t.Errorf("gave up after %v, want about %v", d, AnswerTimeout)
	}
}

func TestMeasureStopsWithItsContext(t *testing.T) {
	t.Parallel()
	silent := client(t)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := Measure(ctx, client(t), silent.LocalAddr(), 2, time.Millisecond); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err %v, want the context's", err)
	}
}

// exchange sends b to the responder at peer and reports whether anything came back.
func exchange(t *testing.T, peer net.Addr, b []byte) ([]byte, bool) {
	t.Helper()
	conn := client(t)
	if _, err := conn.WriteTo(b, peer); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	buf := make([]byte, 64)
	n, _, err := conn.ReadFrom(buf)
	if err != nil {
		return nil, false
	}
	return buf[:n], true
}

func TestServeDropsWhatIsNotARequest(t *testing.T) {
	t.Parallel()
	peer := responder(t, 0)
	good := request(7, 123)
	long := append(append([]byte(nil), good...), 0)
	badMagic := append([]byte(nil), good...)
	copy(badMagic, "XXXX")
	answer := append([]byte(nil), good...) // a response: t2 and t3 in the padding
	binary.BigEndian.PutUint64(answer[16:], 1)
	for name, b := range map[string][]byte{
		"empty": {}, "short": good[:31], "long": long, "magic only": []byte(Magic),
		"bad magic": badMagic, "an answer": answer,
	} {
		if got, ok := exchange(t, peer, b); ok {
			t.Errorf("%s: answered with %d bytes", name, len(got))
		}
	}
	got, ok := exchange(t, peer, good)
	if !ok || len(got) != PacketSize || string(got[:16]) != string(good[:16]) {
		t.Fatalf("a good request: answered %v with %x", ok, got)
	}
	if t2, t3 := binary.BigEndian.Uint64(got[16:]), binary.BigEndian.Uint64(got[24:]); t2 == 0 || t3 < t2 {
		t.Errorf("t2 %d, t3 %d", t2, t3)
	}
}

// A fake peer answers each probe twice: first with a stale nonce and a wild offset, then
// correctly. Measure must keep the correct one.
func TestMeasureIgnoresAStaleAnswer(t *testing.T) {
	t.Parallel()
	fake := client(t)
	go func() {
		buf := make([]byte, 64)
		for {
			n, from, err := fake.ReadFrom(buf)
			if err != nil {
				return
			}
			if n != PacketSize {
				continue
			}
			now := time.Now().UnixNano()
			stale := make([]byte, PacketSize)
			copy(stale, buf[:16])
			binary.BigEndian.PutUint32(stale[4:], binary.BigEndian.Uint32(buf[4:])+1000)
			binary.BigEndian.PutUint64(stale[16:], uint64(now+int64(time.Hour)))
			binary.BigEndian.PutUint64(stale[24:], uint64(now+int64(time.Hour)))
			_, _ = fake.WriteTo(stale, from)
			wrongT1 := append([]byte(nil), stale...)
			copy(wrongT1[4:8], buf[4:8])
			binary.BigEndian.PutUint64(wrongT1[8:], 1)
			_, _ = fake.WriteTo(wrongT1, from)
			good := make([]byte, PacketSize)
			copy(good, buf[:16])
			binary.BigEndian.PutUint64(good[16:], uint64(now))
			binary.BigEndian.PutUint64(good[24:], uint64(now))
			_, _ = fake.WriteTo(good, from)
		}
	}()
	p, err := Measure(context.Background(), client(t), fake.LocalAddr(), 4, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if p.Offset.Abs() > p.RTT/2+time.Microsecond {
		t.Errorf("offset %v ± %v: a stale answer was taken", p.Offset, p.RTT/2)
	}
}

func TestModel(t *testing.T) {
	t.Parallel()
	t0 := time.Now()
	m := Model{Points: []Point{
		{At: t0, Offset: 10 * time.Millisecond, RTT: 2 * time.Millisecond},
		{At: t0.Add(30 * time.Second), Offset: 13 * time.Millisecond, RTT: 6 * time.Millisecond},
		{At: t0.Add(60 * time.Second), Offset: 7 * time.Millisecond, RTT: 4 * time.Millisecond},
	}}
	for _, c := range []struct {
		at   time.Duration
		want time.Duration
	}{
		{-time.Minute, 10 * time.Millisecond}, // clamped to the first point
		{0, 10 * time.Millisecond},
		{10 * time.Second, 11 * time.Millisecond},
		{30 * time.Second, 13 * time.Millisecond},
		{45 * time.Second, 10 * time.Millisecond},
		{60 * time.Second, 7 * time.Millisecond},
		{time.Hour, 7 * time.Millisecond}, // clamped to the last point
	} {
		if got := m.OffsetAt(t0.Add(c.at)); (got - c.want).Abs() > time.Microsecond {
			t.Errorf("OffsetAt(+%v) = %v, want %v", c.at, got, c.want)
		}
	}
	if u := m.Uncertainty(); u != 3*time.Millisecond {
		t.Errorf("Uncertainty %v, want 3ms (the largest RTT/2)", u)
	}
	if (Model{}).OffsetAt(t0) != 0 || (Model{}).Uncertainty() != 0 {
		t.Error("an empty model must be zero")
	}
	one := Model{Points: m.Points[1:2]}
	if one.OffsetAt(t0) != 13*time.Millisecond || one.OffsetAt(t0.Add(time.Hour)) != 13*time.Millisecond {
		t.Error("one point is held on both sides")
	}
}

func TestScrubDropsTheAddress(t *testing.T) {
	t.Parallel()
	err := &net.OpError{Op: "write", Net: "udp", Addr: &net.UDPAddr{IP: net.IPv4(192, 0, 2, 7), Port: 7444}, Err: errors.New("no route to host")}
	if got := scrub(err).Error(); got != "no route to host" {
		t.Errorf("scrub: %q", got)
	}
}
