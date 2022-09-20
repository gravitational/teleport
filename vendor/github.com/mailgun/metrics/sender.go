package metrics

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/mailgun/timetools"
)

type sender interface {
	Close() error
	Write(data []byte) (int, error)
}

type UDPSender struct {
	// underlying connection
	c *net.UDPConn
	// resolved udp address
	ra *net.UDPAddr
}

func (s *UDPSender) Write(data []byte) (int, error) {
	// no need for locking here, as the underlying fdNet
	// already serialized writes
	n, err := s.c.WriteToUDP([]byte(data), s.ra)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return n, errors.New("Wrote no bytes")
	}
	return n, nil
}

func (s *UDPSender) Close() error {
	return s.c.Close()
}

func newUDPSender(addr string) (sender, error) {
	c, err := net.ListenPacket("udp", ":0")
	if err != nil {
		return nil, err
	}

	ra, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}

	return &UDPSender{ra: ra, c: c.(*net.UDPConn)}, nil
}

type bufSender struct {
	m           *sync.Mutex
	s           sender
	buf         *bytes.Buffer
	lastFlush   time.Time
	clock       timetools.TimeProvider
	maxBytes    int
	flushPeriod time.Duration
}

func (s *bufSender) flush() {
	for {
		s.clock.Sleep(s.flushPeriod)
		s.Write(nil)
	}
}

func (s *bufSender) Close() error {
	return s.s.Close()
}

func (s *bufSender) timeToFlush() bool {
	return s.clock.UtcNow().After(s.lastFlush.Add(s.flushPeriod))
}

func (s *bufSender) Write(data []byte) (int, error) {
	s.m.Lock()
	defer s.m.Unlock()

	if len(data)+1 > s.maxBytes {
		return 0, fmt.Errorf("datagram is too large")
	}

	// if we are approaching the limit, flush
	if s.buf.Len() != 0 && (s.buf.Len()+len(data)+1 > s.maxBytes || s.timeToFlush()) {
		s.lastFlush = s.clock.UtcNow()
		s.buf.WriteTo(s.s)
		// always truncate regardless off errors
		s.buf.Truncate(0)
	}

	if len(data) != 0 {
		s.buf.Write(data)
		s.buf.WriteRune('\n')
	}

	// Never return errors, otherwise WriterTo will be confused.
	// we must discard errors anyways
	return len(data), nil
}

func newBufSender(s sender, maxBytes int, flushPeriod time.Duration) (sender, error) {
	b := &bytes.Buffer{}
	b.Grow(maxBytes)
	clock := &timetools.RealTime{}
	sdr := &bufSender{
		m:           &sync.Mutex{},
		s:           s,
		clock:       clock,
		buf:         b,
		maxBytes:    maxBytes,
		flushPeriod: flushPeriod,
		lastFlush:   clock.UtcNow(),
	}
	go sdr.flush()
	return sdr, nil
}
