package quic

import (
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/quic-go/quic-go"

	apiclient "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/utils"
)

func newStreamConn(stream quic.Stream, localAddr, remoteAddr *apiclient.NetAddr) net.Conn {
	return &streamConn{
		stream:     stream,
		localAddr:  &utils.NetAddr{Addr: localAddr.GetAddr(), AddrNetwork: localAddr.GetNetwork()},
		remoteAddr: &utils.NetAddr{Addr: remoteAddr.GetAddr(), AddrNetwork: remoteAddr.GetNetwork()},
	}
}

type streamConn struct {
	stream     quic.Stream
	localAddr  net.Addr
	remoteAddr net.Addr

	closeOnce sync.Once
	closed    atomic.Bool

	deadlineMu sync.RWMutex
	writeMu    sync.RWMutex
}

var _ net.Conn = (*streamConn)(nil)

// LocalAddr implements [net.Conn].
func (c *streamConn) LocalAddr() net.Addr {
	return c.localAddr
}

// RemoteAddr implements [net.Conn].
func (c *streamConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

// Close implements [net.Conn].
func (c *streamConn) Close() error {
	err := net.ErrClosed
	c.closeOnce.Do(func() { err = c.close() })
	return err
}

var aLongTimeAgo = time.Unix(0, 1)

func (c *streamConn) close() error {
	c.closed.Store(true)

	c.stream.CancelRead(quic.StreamErrorCode(0))

	c.deadlineMu.Lock()
	defer c.deadlineMu.Unlock()
	_ = c.stream.SetWriteDeadline(aLongTimeAgo)

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	err := c.stream.Close()
	if err != nil && strings.HasPrefix(err.Error(), "close called for canceled stream ") {
		return nil
	}

	return trace.Wrap(err)
}

// Read implements [net.Conn].
func (c *streamConn) Read(b []byte) (n int, err error) {
	if c.closed.Load() {
		return 0, net.ErrClosed
	}
	n, err = c.stream.Read(b)
	return n, err
}

// SetDeadline implements [net.Conn].
func (c *streamConn) SetDeadline(t time.Time) error {
	c.deadlineMu.RLock()
	defer c.deadlineMu.RUnlock()
	if c.closed.Load() {
		return net.ErrClosed
	}
	return c.stream.SetDeadline(t)
}

// SetReadDeadline implements [net.Conn].
func (c *streamConn) SetReadDeadline(t time.Time) error {
	c.deadlineMu.RLock()
	defer c.deadlineMu.RUnlock()
	if c.closed.Load() {
		return net.ErrClosed
	}
	return c.stream.SetReadDeadline(t)
}

// SetWriteDeadline implements [net.Conn].
func (c *streamConn) SetWriteDeadline(t time.Time) error {
	c.deadlineMu.RLock()
	defer c.deadlineMu.RUnlock()
	if c.closed.Load() {
		return net.ErrClosed
	}
	return c.stream.SetWriteDeadline(t)
}

// Write implements [net.Conn].
func (c *streamConn) Write(b []byte) (n int, err error) {
	c.writeMu.RLock()
	defer c.writeMu.RUnlock()
	if c.closed.Load() {
		return 0, net.ErrClosed
	}
	n, err = c.stream.Write(b)
	return n, err
}
