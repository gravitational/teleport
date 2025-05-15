package listener

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
)

// InMemoryListener is a in-memory implementation of a net.Listener.
type InMemoryListener struct {
	connCh    chan net.Conn
	closeCh   chan struct{}
	closeOnce sync.Once
}

// Accept implements net.Listener.
func (m *InMemoryListener) Accept() (net.Conn, error) {
	select {
	case <-m.closeCh:
		return nil, io.EOF
	default:
	}

	for {
		select {
		case conn := <-m.connCh:
			return conn, nil
		case <-m.closeCh:
			return nil, io.EOF
		}
	}
}

// Addr implements net.Listener.
func (m *InMemoryListener) Addr() net.Addr {
	return defaultMemoryAddr
}

// Close implements net.Listener.
func (m *InMemoryListener) Close() error {
	m.closeOnce.Do(func() { close(m.closeCh) })
	return nil
}

// Dial dials the memory listener, creating a new net.Conn.
//
// This function satisfies net.Dialer.DialContext signature.
func (m *InMemoryListener) DialContext(ctx context.Context, _ string, _ string) (net.Conn, error) {
	select {
	case <-m.closeCh:
		return nil, ErrListenerClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	serverConn, clientConn := net.Pipe()

	select {
	case m.connCh <- serverConn:
	case <-ctx.Done():
		// In this case the connection was not accepted in time by the server
		// and the dial context is done. To avoid having the server using an
		// orphned connection we should close it.
		_ = serverConn.Close()
		return nil, ctx.Err()
	}
	return clientConn, nil
}

// ErrListenerClosed is the error returned by dial when the listener is closed.
var ErrListenerClosed = errors.New("in-memory listener closed")

// InNewMemoryListener initializes a new in-memory listener.
func InNewMemoryListener() *InMemoryListener {
	return &InMemoryListener{
		connCh:  make(chan net.Conn),
		closeCh: make(chan struct{}),
	}
}

var _ net.Listener = (*InMemoryListener)(nil)

type memoryAddr string

func (m memoryAddr) Network() string { return string(m) }
func (m memoryAddr) String() string  { return string(m) }

var defaultMemoryAddr = memoryAddr("memory")

var _ net.Addr = (*memoryAddr)(nil)
